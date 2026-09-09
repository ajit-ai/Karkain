package codegen

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// ParsedSubprogram is a decoded DW_TAG_subprogram entry.
type ParsedSubprogram struct {
	Name          string
	LowPC, HighPC uint64
	File, Line    uint32
	Size          uint64
}

// ParsedVariable is a decoded DW_TAG_variable entry.
type ParsedVariable struct {
	Name     string
	File     uint32
	Line     uint32
	Function string
}

// ParsedLineRow is one materialized row of the .debug_line program.
type ParsedLineRow struct {
	Address     uint64
	File        uint32
	Line        uint32
	Column      uint32
	IsStmt      bool
	EndSequence bool
}

// ParsedDWARF is the decoded view of the DWARF debug sections.
type ParsedDWARF struct {
	Version   uint16
	AddrSize  byte
	UnitName  string
	Producer  string
	Language  uint32
	TextLow   uint64
	Funcs     []ParsedSubprogram
	Variables []ParsedVariable
	LineRows  []ParsedLineRow
}

// ParseDWARF decodes the four DWARF 4 debug sections emitted by DwarfEmitter.
// Any of the sections may be nil, in which case the corresponding data is left
// empty (only .debug_info is required).
func ParseDWARF(infoData, abbrevData, strData, lineData []byte) (*ParsedDWARF, error) {
	if len(infoData) < 11 {
		return nil, fmt.Errorf("dwarf: .debug_info section too short (%d bytes)", len(infoData))
	}
	unitLength := binary.LittleEndian.Uint32(infoData[0:])
	version := binary.LittleEndian.Uint16(infoData[4:])
	addrSize := infoData[10]
	if version != dwarfVersion {
		return nil, fmt.Errorf("dwarf: unsupported version %d (want %d)", version, dwarfVersion)
	}
	unitEnd := 4 + int(unitLength)
	if unitEnd > len(infoData) {
		return nil, fmt.Errorf("dwarf: .debug_info unit length %d exceeds section size %d", unitLength, len(infoData))
	}

	result := &ParsedDWARF{
		Version:   version,
		AddrSize:  addrSize,
		Funcs:     make([]ParsedSubprogram, 0),
		Variables: make([]ParsedVariable, 0),
	}

	dies, err := parseDIEs(infoData[11:unitEnd], abbrevData, strData, addrSize)
	if err != nil {
		return nil, err
	}
	walkParsedDIEs(result, dies)

	if len(lineData) >= 4 {
		rows, err := parseLineRows(lineData, addrSize)
		if err != nil {
			return nil, fmt.Errorf("dwarf: .debug_line: %v", err)
		}
		result.LineRows = rows
	}
	return result, nil
}

func walkParsedDIEs(p *ParsedDWARF, dies []*dwarfDie) {
	for _, d := range dies {
		switch d.tag {
		case dwTagCompileUnit:
			p.UnitName = d.attrs.strs[dwAtName]
			p.Producer = d.attrs.strs[dwAtProducer]
			p.Language = uint32(d.attrs.uints[dwAtLanguage])
			p.TextLow = d.attrs.uints[dwAtLowPC]
			walkParsedDIEs(p, d.children)
		case dwTagSubprogram:
			name := d.attrs.strs[dwAtName]
			low := d.attrs.uints[dwAtLowPC]
			high := d.attrs.uints[dwAtHighPC]
			size, highPC := high, low+high
			if high > low {
				size = high - low
			}
			p.Funcs = append(p.Funcs, ParsedSubprogram{
				Name:   name,
				LowPC:  low,
				HighPC: highPC,
				File:   uint32(d.attrs.uints[dwAtDeclFile]),
				Line:   uint32(d.attrs.uints[dwAtDeclLine]),
				Size:   size,
			})
			for _, child := range d.children {
				if child.tag == dwTagVariable {
					p.Variables = append(p.Variables, ParsedVariable{
						Name:     child.attrs.strs[dwAtName],
						File:     uint32(child.attrs.uints[dwAtDeclFile]),
						Line:     uint32(child.attrs.uints[dwAtDeclLine]),
						Function: name,
					})
				}
			}
		case dwTagVariable:
			p.Variables = append(p.Variables, ParsedVariable{
				Name: d.attrs.strs[dwAtName],
				File: uint32(d.attrs.uints[dwAtDeclFile]),
				Line: uint32(d.attrs.uints[dwAtDeclLine]),
			})
		}
	}
}

type abbrevAttr struct {
	at, form uint16
}

type dwarfAbbrev struct {
	tag      uint16
	children byte
	attrs    []abbrevAttr
}

type dwarfAttrs struct {
	uints map[uint16]uint64
	strs  map[uint16]string
}

type dwarfDie struct {
	offset   uint64
	tag      uint16
	attrs    dwarfAttrs
	children []*dwarfDie
}

func parseDIEs(data, abbrevData, strData []byte, addrSize byte) ([]*dwarfDie, error) {
	abbrevs, err := parseAbbrevs(abbrevData)
	if err != nil {
		return nil, err
	}
	pos := 0
	return parseDIELevel(data, &pos, abbrevs, strData, addrSize)
}

func parseAbbrevs(data []byte) (map[uint64]dwarfAbbrev, error) {
	abbrevs := make(map[uint64]dwarfAbbrev)
	pos := 0
	for pos < len(data) {
		code, n := readUleb(data[pos:])
		pos += n
		if code == 0 {
			return abbrevs, nil
		}
		tag, n := readUleb(data[pos:])
		pos += n
		if pos >= len(data) {
			return nil, fmt.Errorf("dwarf: truncated .debug_abbrev")
		}
		children := data[pos]
		pos++
		attrs := make([]abbrevAttr, 0)
		for pos < len(data) {
			at, n := readUleb(data[pos:])
			pos += n
			form, n := readUleb(data[pos:])
			pos += n
			if at == 0 && form == 0 {
				break
			}
			attrs = append(attrs, abbrevAttr{at: uint16(at), form: uint16(form)})
		}
		abbrevs[code] = dwarfAbbrev{tag: uint16(tag), children: children, attrs: attrs}
	}
	if len(abbrevs) == 0 {
		return nil, fmt.Errorf("dwarf: empty .debug_abbrev")
	}
	return abbrevs, nil
}

func parseDIELevel(data []byte, pos *int, abbrevs map[uint64]dwarfAbbrev, strData []byte, addrSize byte) ([]*dwarfDie, error) {
	out := make([]*dwarfDie, 0)
	for *pos < len(data) {
		off := uint64(*pos)
		code, n := readUleb(data[*pos:])
		*pos += n
		if code == 0 {
			return out, nil
		}
		abbrev, ok := abbrevs[code]
		if !ok {
			return nil, fmt.Errorf("dwarf: undefined abbreviation code %d", code)
		}
		attrs := dwarfAttrs{uints: make(map[uint16]uint64), strs: make(map[uint16]string)}
		for _, aa := range abbrev.attrs {
			if err := readAttrValue(data, pos, aa, &attrs, strData, addrSize); err != nil {
				return nil, err
			}
		}
		die := &dwarfDie{offset: off, tag: abbrev.tag, attrs: attrs}
		if abbrev.children == dwChildrenYes {
			children, err := parseDIELevel(data, pos, abbrevs, strData, addrSize)
			if err != nil {
				return nil, err
			}
			die.children = children
		}
		out = append(out, die)
	}
	return out, nil
}

func readAttrValue(data []byte, pos *int, aa abbrevAttr, attrs *dwarfAttrs, strData []byte, addrSize byte) error {
	switch aa.form {
	case dwFormStrp:
		if *pos+4 > len(data) {
			return fmt.Errorf("dwarf: truncated string reference")
		}
		off := int(binary.LittleEndian.Uint32(data[*pos:]))
		*pos += 4
		attrs.strs[aa.at] = readCString(strData, off)
	case dwFormAddr:
		v, err := readAddr(data, pos, addrSize)
		if err != nil {
			return err
		}
		attrs.uints[aa.at] = v
	case dwFormData1:
		if *pos+1 > len(data) {
			return fmt.Errorf("dwarf: truncated attribute")
		}
		attrs.uints[aa.at] = uint64(data[*pos])
		*pos++
	case dwFormData2:
		if *pos+2 > len(data) {
			return fmt.Errorf("dwarf: truncated attribute")
		}
		attrs.uints[aa.at] = uint64(binary.LittleEndian.Uint16(data[*pos:]))
		*pos += 2
	case dwFormData4, dwFormSecOffset, dwFormRef4:
		if *pos+4 > len(data) {
			return fmt.Errorf("dwarf: truncated attribute")
		}
		attrs.uints[aa.at] = uint64(binary.LittleEndian.Uint32(data[*pos:]))
		*pos += 4
	case dwFormData8:
		if *pos+8 > len(data) {
			return fmt.Errorf("dwarf: truncated attribute")
		}
		attrs.uints[aa.at] = binary.LittleEndian.Uint64(data[*pos:])
		*pos += 8
	case dwFormFlagPresent:
		attrs.uints[aa.at] = 1
	default:
		return fmt.Errorf("dwarf: unsupported attribute form 0x%x", aa.form)
	}
	return nil
}

func parseLineRows(lineData []byte, addrSize byte) ([]ParsedLineRow, error) {
	if len(lineData) < 4 {
		return nil, fmt.Errorf(".debug_line too short")
	}
	unitLength := int(binary.LittleEndian.Uint32(lineData))
	unitEnd := 4 + unitLength
	if unitEnd > len(lineData) {
		return nil, fmt.Errorf("unit length %d exceeds section size %d", unitLength, len(lineData))
	}
	pos := 4
	if pos+2 > unitEnd {
		return nil, fmt.Errorf("truncated line unit header")
	}
	pos += 2
	if pos+4 > unitEnd {
		return nil, fmt.Errorf("truncated line unit header")
	}
	headerLength := int(binary.LittleEndian.Uint32(lineData[pos:]))
	pos += 4
	hdrEnd := pos + headerLength
	if hdrEnd > unitEnd {
		return nil, fmt.Errorf("header length %d exceeds unit", headerLength)
	}

	minInst := uint64(lineData[pos])
	pos++
	defaultIsStmt := lineData[pos] == 1
	pos++
	lineBase := int8(lineData[pos])
	pos++
	lineRange := int(lineData[pos])
	pos++
	opcodeBase := int(lineData[pos])
	pos++
	pos += opcodeBase - 1
	if pos > hdrEnd {
		return nil, fmt.Errorf("truncated opcode lengths")
	}

	dirCount, n := readUleb(lineData[pos:])
	pos += n
	for i := uint64(0); i < dirCount; i++ {
		pos = skipCString(lineData, pos)
		if pos > hdrEnd {
			return nil, fmt.Errorf("truncated directory table")
		}
	}
	fileCount, n := readUleb(lineData[pos:])
	pos += n
	fileTable := make([]string, 0, fileCount)
	for i := uint64(0); i < fileCount; i++ {
		name, end := readCString2(lineData, pos)
		pos = end
		_, n := readUleb(lineData[pos:])
		pos += n
		_, n = readUleb(lineData[pos:])
		pos += n
		_, n = readUleb(lineData[pos:])
		pos += n
		if pos > hdrEnd {
			return nil, fmt.Errorf("truncated file table")
		}
		fileTable = append(fileTable, name)
	}
	_ = fileTable
	pos = hdrEnd

	rows := make([]ParsedLineRow, 0)
	materialize := func(rows *[]ParsedLineRow, state *parsedLineState, end bool) {
		*rows = append(*rows, ParsedLineRow{
			Address:     state.address,
			File:        state.file,
			Line:        state.line,
			Column:      state.column,
			IsStmt:      state.isStmt,
			EndSequence: end,
		})
	}
	state := parsedLineState{file: 1, line: 1, isStmt: defaultIsStmt}

	for pos < unitEnd {
		op := lineData[pos]
		if op == dwLineExtOp {
			extLen, n := readUleb(lineData[pos+1:])
			opStart := pos + 1 + n
			opEnd := opStart + int(extLen)
			if opEnd > unitEnd {
				return nil, fmt.Errorf("truncated extended opcode")
			}
			extOp := lineData[opStart]
			switch extOp {
			case dwLneSetAddress:
				addrPos := opStart + 1
				var addr uint64
				for i := 0; i < int(addrSize) && addrPos < opEnd; i++ {
					addr |= uint64(lineData[addrPos]) << (8 * i)
					addrPos++
				}
				state.address = addr
			case dwLneEndSequence:
				materialize(&rows, &state, true)
				state = parsedLineState{file: 1, line: 1, isStmt: defaultIsStmt}
			}
			pos = opEnd
			continue
		}
		if op < byte(opcodeBase) {
			switch op {
			case dwLnsCopy:
				materialize(&rows, &state, false)
			case dwLnsAdvancePC:
				delta, n := readUleb(lineData[pos+1:])
				pos += 1 + n
				state.address += delta * minInst
				continue
			case dwLnsAdvanceLine:
				delta, n := readSleb(lineData[pos+1:])
				pos += 1 + n
				state.line = uint32(int64(state.line) + delta)
				continue
			case dwLnsSetFile:
				f, n := readUleb(lineData[pos+1:])
				pos += 1 + n
				state.file = uint32(f)
				continue
			case dwLnsSetColumn:
				c, n := readUleb(lineData[pos+1:])
				pos += 1 + n
				state.column = uint32(c)
				continue
			case dwLnsNegateStmt:
				state.isStmt = !state.isStmt
				pos++
				continue
			case dwLnsSetBasicBlock:
				pos++
				continue
			case dwLnsConstAddPC:
				state.address += uint64((255-opcodeBase)/lineRange) * minInst
				pos++
				continue
			case dwLnsFixedAdvancePC:
				if pos+3 > unitEnd {
					return nil, fmt.Errorf("truncated fixed advance")
				}
				state.address += uint64(binary.LittleEndian.Uint16(lineData[pos+1:]))
				pos += 3
				continue
			case dwLnsSetPrologueEnd, dwLnsSetEpilogueBegin:
				pos++
				continue
			case dwLnsSetISA:
				_, n := readUleb(lineData[pos+1:])
				pos += 1 + n
				continue
			default:
				return nil, fmt.Errorf("unsupported standard opcode %d", op)
			}
			pos++
			continue
		}
		adj := (int(op) - opcodeBase)
		state.line = uint32(int64(state.line) + int64(adj/lineRange) + int64(lineBase))
		state.address += uint64(adj%lineRange) * minInst
		materialize(&rows, &state, false)
		pos++
	}
	return rows, nil
}

type parsedLineState struct {
	address uint64
	file    uint32
	line    uint32
	column  uint32
	isStmt  bool
}

func readUleb(data []byte) (uint64, int) {
	var result uint64
	var shift uint
	for i, b := range data {
		result |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return result, i + 1
		}
		shift += 7
	}
	return result, len(data)
}

func readSleb(data []byte) (int64, int) {
	var result int64
	var shift uint
	for i, b := range data {
		result |= int64(b&0x7f) << shift
		shift += 7
		if b&0x80 == 0 {
			if b&0x40 != 0 {
				result |= ^int64(0) << shift
			}
			return result, i + 1
		}
	}
	return result, len(data)
}

func readAddr(data []byte, pos *int, size byte) (uint64, error) {
	if *pos+int(size) > len(data) || size > 8 {
		return 0, fmt.Errorf("dwarf: truncated address")
	}
	var v uint64
	for i := 0; i < int(size); i++ {
		v |= uint64(data[*pos+i]) << (8 * i)
	}
	*pos += int(size)
	return v, nil
}

func readCString(data []byte, off int) string {
	if off < 0 || off >= len(data) {
		return ""
	}
	end := off
	for end < len(data) && data[end] != 0 {
		end++
	}
	return string(data[off:end])
}

func readCString2(data []byte, pos int) (string, int) {
	end := pos
	for end < len(data) && data[end] != 0 {
		end++
	}
	name := string(data[pos:end])
	if end < len(data) {
		end++
	}
	return name, end
}

func skipCString(data []byte, pos int) int {
	for pos < len(data) && data[pos] != 0 {
		pos++
	}
	if pos < len(data) {
		pos++
	}
	return pos
}

// DwarfTextDump renders the DWARF sections in a readelf --debug-dump style.
// It requires .debug_info; the other sections may be nil.
func DwarfTextDump(sections []*Section) (string, error) {
	var info, abbrev, str, line []byte
	for _, sect := range sections {
		switch sect.Name {
		case dwarfDebugInfoName:
			info = sect.Data
		case dwarfDebugAbbrevName:
			abbrev = sect.Data
		case dwarfDebugStrName:
			str = sect.Data
		case dwarfDebugLineName:
			line = sect.Data
		}
	}
	if len(info) == 0 {
		return "", fmt.Errorf("dwarf: no .debug_info section")
	}
	p, err := ParseDWARF(info, abbrev, str, line)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("Contents of the .debug_info section:\n")
	sb.WriteString("  Compilation unit @ offset 0x0:\n")
	sb.WriteString(fmt.Sprintf("    Length:      0x%x\n", len(info)-4))
	sb.WriteString(fmt.Sprintf("    Version:     %d\n", p.Version))
	sb.WriteString("    Abbrev Offset: 0x0\n")
	sb.WriteString(fmt.Sprintf("    Pointer Size: %d\n", p.AddrSize))
	sb.WriteString("  <0> DW_TAG_compile_unit\n")
	sb.WriteString(fmt.Sprintf("        DW_AT_producer    %s\n", p.Producer))
	sb.WriteString(fmt.Sprintf("        DW_AT_language    %d\n", p.Language))
	sb.WriteString(fmt.Sprintf("        DW_AT_name        %s\n", p.UnitName))
	sb.WriteString(fmt.Sprintf("        DW_AT_stmt_list   0x%x\n", uint64(0)))
	sb.WriteString(fmt.Sprintf("        DW_AT_low_pc      0x%x\n", p.TextLow))
	for _, fn := range p.Funcs {
		sb.WriteString(fmt.Sprintf("  <%d> DW_TAG_subprogram\n", 41))
		sb.WriteString(fmt.Sprintf("        DW_AT_name        %s\n", fn.Name))
		sb.WriteString(fmt.Sprintf("        DW_AT_decl_file   %d\n", fn.File))
		sb.WriteString(fmt.Sprintf("        DW_AT_decl_line   %d\n", fn.Line))
		sb.WriteString("        DW_AT_external    true\n")
		sb.WriteString(fmt.Sprintf("        DW_AT_low_pc      0x%x\n", fn.LowPC))
		sb.WriteString(fmt.Sprintf("        DW_AT_high_pc     0x%x (size)\n", fn.Size))
	}
	for _, v := range p.Variables {
		sb.WriteString(fmt.Sprintf("  <%d> DW_TAG_variable\n", 42))
		sb.WriteString(fmt.Sprintf("        DW_AT_name        %s\n", v.Name))
		sb.WriteString(fmt.Sprintf("        DW_AT_decl_file   %d\n", v.File))
		sb.WriteString(fmt.Sprintf("        DW_AT_decl_line   %d\n", v.Line))
	}

	sb.WriteString("\nContents of the .debug_line section:\n")
	sb.WriteString("  Address          File Line Column IsStmt EndSeq\n")
	for _, r := range p.LineRows {
		isStmt := "-"
		if r.IsStmt {
			isStmt = "Y"
		}
		endSeq := "-"
		if r.EndSequence {
			endSeq = "Y"
		}
		sb.WriteString(fmt.Sprintf("  0x%-16x %-4d %-4d %-6d %-6s %s\n", r.Address, r.File, r.Line, r.Column, isStmt, endSeq))
	}
	return sb.String(), nil
}