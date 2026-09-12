package codegen

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// DwarfSectionNames lists the DWARF debug sections the emitter produces.
var DwarfSectionNames = []string{".debug_info", ".debug_abbrev", ".debug_str", ".debug_line"}

const (
	dwarfDebugInfoName   = ".debug_info"
	dwarfDebugAbbrevName = ".debug_abbrev"
	dwarfDebugStrName    = ".debug_str"
	dwarfDebugLineName   = ".debug_line"

	dwarfAddressSize = 8
	dwarfVersion     = 4

	dwTagCompileUnit uint16 = 0x11
	dwTagSubprogram  uint16 = 0x2e
	dwTagBaseType    uint16 = 0x24
	dwTagVariable    uint16 = 0x34

	dwChildrenNo  byte = 0
	dwChildrenYes byte = 1

	dwAtProducer uint16 = 0x25
	dwAtLanguage uint16 = 0x13
	dwAtName     uint16 = 0x03
	dwAtStmtList uint16 = 0x10
	dwAtLowPC    uint16 = 0x11
	dwAtHighPC   uint16 = 0x12
	dwAtDeclFile uint16 = 0x3a
	dwAtDeclLine uint16 = 0x3b
	dwAtType     uint16 = 0x49
	dwAtExternal uint16 = 0x3e
	dwAtEncoding uint16 = 0x27
	dwAtByteSize uint16 = 0x0b

	dwFormAddr        uint16 = 0x01
	dwFormData1       uint16 = 0x0b
	dwFormData2       uint16 = 0x05
	dwFormData4       uint16 = 0x06
	dwFormData8       uint16 = 0x07
	dwFormStrp        uint16 = 0x0e
	dwFormFlagPresent uint16 = 0x19
	dwFormRef4        uint16 = 0x14
	dwFormSecOffset   uint16 = 0x17

	dwLangC99 uint16 = 0x0001

	dwAteSigned  byte = 0x05
	dwAteFloat   byte = 0x04
	dwAteBoolean byte = 0x02
	dwAteAddress byte = 0x01

	dwLnsCopy             byte = 0x01
	dwLnsAdvancePC        byte = 0x02
	dwLnsAdvanceLine      byte = 0x03
	dwLnsSetFile          byte = 0x04
	dwLnsSetColumn        byte = 0x05
	dwLnsNegateStmt       byte = 0x06
	dwLnsSetBasicBlock    byte = 0x07
	dwLnsConstAddPC       byte = 0x08
	dwLnsFixedAdvancePC   byte = 0x09
	dwLnsSetPrologueEnd   byte = 0x0a
	dwLnsSetEpilogueBegin byte = 0x0b
	dwLnsSetISA           byte = 0x0c

	dwLneEndSequence byte = 0x01
	dwLneSetAddress  byte = 0x02
	dwLineExtOp      byte = 0x00

	dwLineOperandBase   uint8 = 13
	dwLineRange         uint8 = 14
	dwLineMinInst       uint8 = 1
	dwLineDefaultIsStmt uint8 = 1
	dwDwarfProducer           = "karkain-compiler v0.117.0 (Phase 104 DWARF)"
)

// DwarfEmitter emits DWARF 4 debug sections from the native debug model.
type DwarfEmitter struct{}

var dwLineBase int8 = -5

// NewDwarfEmitter creates a DWARF emitter.
func NewDwarfEmitter() *DwarfEmitter {
	return &DwarfEmitter{}
}

// Emit builds the four DWARF 4 debug sections (.debug_info, .debug_abbrev,
// .debug_str, .debug_line) for the given debug info. textBase is the linked
// .text base address; textSize is the .text byte size.
func (de *DwarfEmitter) Emit(dbg *DebugInfo, textBase, textSize uint64) ([]*Section, error) {
	if dbg == nil {
		return nil, fmt.Errorf("dwarf: no debug info available")
	}
	sourceName := "source.kark"
	if len(dbg.SourceFiles) > 0 && dbg.SourceFiles[0].Path != "" {
		sourceName = dbg.SourceFiles[0].Path
	}

	strData, strOffsets := de.buildStrings(dbg, sourceName)
	abbrevData := de.buildAbbrev()
	infoData, err := de.buildInfo(dbg, strOffsets, sourceName, textBase, textSize)
	if err != nil {
		return nil, err
	}
	lineData, err := de.buildLine(dbg, textBase)
	if err != nil {
		return nil, err
	}

	return []*Section{
		{Name: dwarfDebugAbbrevName, Type: SectionTypeDebug, Data: abbrevData, Align: 1, Size: uint64(len(abbrevData))},
		{Name: dwarfDebugStrName, Type: SectionTypeDebug, Data: strData, Align: 1, Size: uint64(len(strData))},
		{Name: dwarfDebugInfoName, Type: SectionTypeDebug, Data: infoData, Align: 1, Size: uint64(len(infoData))},
		{Name: dwarfDebugLineName, Type: SectionTypeDebug, Data: lineData, Align: 1, Size: uint64(len(lineData))},
	}, nil
}

// buildStrings collects the unique strings referenced by the DWARF sections.
func (de *DwarfEmitter) buildStrings(dbg *DebugInfo, sourceName string) ([]byte, map[string]uint32) {
	set := make(map[string]bool)
	ordered := make([]string, 0)
	add := func(s string) {
		if s == "" {
			s = "void"
		}
		if !set[s] {
			set[s] = true
			ordered = append(ordered, s)
		}
	}
	add(dwDwarfProducer)
	add(sourceName)
	for _, fn := range dbg.FunctionInfo {
		add(fn.Name)
	}
	for _, v := range dbg.Variables {
		add(v.Name)
		add(v.Type)
	}
	for _, name := range []string{"int", "float", "bool", "string", "void"} {
		add(name)
	}
	sort.Strings(ordered)

	data := make([]byte, 0)
	offsets := make(map[string]uint32, len(ordered))
	for _, s := range ordered {
		offsets[s] = uint32(len(data))
		data = append(data, []byte(s)...)
		data = append(data, 0)
	}
	return data, offsets
}

// buildAbbrev emits the abbreviation table for the four DIE kinds used.
func (de *DwarfEmitter) buildAbbrev() []byte {
	t := make([]byte, 0)
	attr := func(at, form uint16) {
		t = append(t, uleb(uint64(at))...)
		t = append(t, uleb(uint64(form))...)
	}
	endAttr := func() {
		t = append(t, 0, 0)
	}

	t = append(t, 1)
	t = append(t, uleb(uint64(dwTagCompileUnit))...)
	t = append(t, dwChildrenYes)
	attr(dwAtProducer, dwFormStrp)
	attr(dwAtLanguage, dwFormData4)
	attr(dwAtName, dwFormStrp)
	attr(dwAtStmtList, dwFormSecOffset)
	attr(dwAtLowPC, dwFormAddr)
	attr(dwAtHighPC, dwFormData4)
	endAttr()

	t = append(t, 2)
	t = append(t, uleb(uint64(dwTagSubprogram))...)
	t = append(t, dwChildrenYes)
	attr(dwAtName, dwFormStrp)
	attr(dwAtDeclFile, dwFormData4)
	attr(dwAtDeclLine, dwFormData4)
	attr(dwAtType, dwFormRef4)
	attr(dwAtExternal, dwFormFlagPresent)
	attr(dwAtLowPC, dwFormAddr)
	attr(dwAtHighPC, dwFormData4)
	endAttr()

	t = append(t, 3)
	t = append(t, uleb(uint64(dwTagBaseType))...)
	t = append(t, dwChildrenNo)
	attr(dwAtName, dwFormStrp)
	attr(dwAtEncoding, dwFormData1)
	attr(dwAtByteSize, dwFormData1)
	endAttr()

	t = append(t, 4)
	t = append(t, uleb(uint64(dwTagVariable))...)
	t = append(t, dwChildrenNo)
	attr(dwAtName, dwFormStrp)
	attr(dwAtDeclFile, dwFormData4)
	attr(dwAtDeclLine, dwFormData4)
	attr(dwAtType, dwFormRef4)
	endAttr()

	t = append(t, 0)
	return t
}

type dwarfBaseType struct {
	name     string
	encoding byte
	size     byte
}

func dwarfBaseTypeFor(t string) dwarfBaseType {
	switch t {
	case "int":
		return dwarfBaseType{name: "int", encoding: dwAteSigned, size: 8}
	case "float", "float64":
		return dwarfBaseType{name: "float", encoding: dwAteFloat, size: 8}
	case "bool":
		return dwarfBaseType{name: "bool", encoding: dwAteBoolean, size: 1}
	case "string":
		return dwarfBaseType{name: "string", encoding: dwAteAddress, size: 8}
	default:
		return dwarfBaseType{name: "void", encoding: dwAteSigned, size: 0}
	}
}

// buildInfo emits the .debug_info section: one compile unit whose children are
// one subprogram per function (with its local variables nested) followed by the
// base-type DIEs referenced from DW_AT_type attributes.
func (de *DwarfEmitter) buildInfo(dbg *DebugInfo, strOffsets map[string]uint32, sourceName string, textBase, textSize uint64) ([]byte, error) {
	funcs := sortedFunctions(dbg.FunctionInfo)
	localVars := make(map[string][]*VariableInfo)
	var globalVars []*VariableInfo
	for _, v := range dbg.Variables {
		if v.Function != "" && v.IsLocal {
			localVars[v.Function] = append(localVars[v.Function], v)
		} else {
			globalVars = append(globalVars, v)
		}
	}

	baseTypes := make([]dwarfBaseType, 0)
	addBase := func(name string) {
		bt := dwarfBaseTypeFor(name)
		for _, existing := range baseTypes {
			if existing.name == bt.name {
				return
			}
		}
		baseTypes = append(baseTypes, bt)
	}
	for _, v := range dbg.Variables {
		addBase(v.Type)
	}
	addBase("int")
	addBase("void")

	cuSize := textSize
	if cuSize == 0 {
		var maxEnd uint64
		for _, fn := range funcs {
			if fn.Address+fn.Size > maxEnd {
				maxEnd = fn.Address + fn.Size
			}
		}
		if maxEnd > textBase {
			cuSize = maxEnd - textBase
		}
	}

	fnOffsets := make(map[string]uint64, len(funcs))
	useOffset := make(map[*VariableInfo]uint64)
	baseOffsets := make(map[string]uint64, len(baseTypes))

	buf := make([]byte, 0)
	// Compile unit header.
	buf = binary.LittleEndian.AppendUint32(buf, 0) // length, patched below
	buf = binary.LittleEndian.AppendUint16(buf, dwarfVersion)
	buf = binary.LittleEndian.AppendUint32(buf, 0) // abbrev offset
	buf = append(buf, dwarfAddressSize)

	// Compile unit DIE (= abbreviation code 1).
	buf = append(buf, 1)
	buf = appendDWAttr(buf, dwarfAddString(strOffsets, dwDwarfProducer))
	buf = appendDWAttr(buf, uint32(dwLangC99))
	buf = appendDWAttr(buf, dwarfAddString(strOffsets, sourceName))
	buf = appendDWAttr(buf, 0)
	buf = appendDWAddr(buf, textBase)
	buf = appendDWAttr(buf, uint32(cuSize))

	// Subprogram DIEs (code 2).
	for _, fn := range funcs {
		fnOffsets[fn.Name] = uint64(len(buf))
		buf = append(buf, 2)
		buf = appendDWAttr(buf, dwarfAddString(strOffsets, fn.Name))
		buf = appendDWAttr(buf, fn.FileIndex+1)
		buf = appendDWAttr(buf, clampU32(fn.StartLine))
		retType := "void"
		if fn.Name == "main" {
			retType = "int"
		}
		buf = appendDWAttr(buf, uint32(baseOffsets[retType]))
		// DW_AT_external (flag_present): no bytes
		buf = appendDWAddr(buf, fn.Address)
		buf = appendDWAttr(buf, uint32(fn.Size))
		for _, v := range localVars[fn.Name] {
			useOffset[v] = uint64(len(buf))
			buf = appendDWVariable(buf, strOffsets, baseOffsets, v)
		}
		buf = append(buf, 0) // closes the subprogram's children
	}
	for _, v := range globalVars {
		useOffset[v] = uint64(len(buf))
		buf = appendDWVariable(buf, strOffsets, baseOffsets, v)
	}
	for _, bt := range baseTypes {
		baseOffsets[bt.name] = uint64(len(buf))
		buf = append(buf, 3)
		buf = appendDWAttr(buf, dwarfAddString(strOffsets, bt.name))
		buf = append(buf, bt.encoding, bt.size)
	}
	buf = append(buf, 0) // closes the compile unit's children

	unitLength := uint32(len(buf) - 4)
	binary.LittleEndian.PutUint32(buf[0:], unitLength)
	return buf, nil
}

func appendDWVariable(buf []byte, strOffsets map[string]uint32, baseOffsets map[string]uint64, v *VariableInfo) []byte {
	buf = append(buf, 4)
	buf = appendDWAttr(buf, dwarfAddString(strOffsets, v.Name))
	buf = appendDWAttr(buf, v.FileIndex+1)
	buf = appendDWAttr(buf, clampU32(v.Line))
	bt := dwarfBaseTypeFor(v.Type)
	buf = appendDWAttr(buf, uint32(baseOffsets[bt.name]))
	return buf
}

// buildLine emits the .debug_line section (version 4) for the debug info.
func (de *DwarfEmitter) buildLine(dbg *DebugInfo, textBase uint64) ([]byte, error) {
	rows := de.lineRows(dbg, textBase)
	if len(rows) == 0 {
		rows = []lineRow{{address: textBase, file: 0, line: 1}}
		rows = append(rows, lineRow{address: textBase, endSequence: true})
	}

	sourceName := "source.kark"
	if len(dbg.SourceFiles) > 0 && dbg.SourceFiles[0].Path != "" {
		sourceName = dbg.SourceFiles[0].Path
	}

	hdr := make([]byte, 0, 40)
	hdr = append(hdr, 0, 0, 0, 0)
	hdr = appendU16(hdr, dwarfVersion)
	hdr = append(hdr, 0, 0, 0, 0)
	hdr = append(hdr, dwLineMinInst)
	hdr = append(hdr, dwLineDefaultIsStmt)
	hdr = append(hdr, byte(dwLineBase))
	hdr = append(hdr, dwLineRange)
	hdr = append(hdr, dwLineOperandBase)
	hdr = append(hdr, 0, 1, 1, 1, 1, 0, 0, 0, 1, 0, 0, 1)
	hdr = append(hdr, 1)
	hdr = append(hdr, '.', 0)
	hdr = append(hdr, 1)
	hdr = append(hdr, []byte(sourceName)...)
	hdr = append(hdr, 0)
	hdr = append(hdr, uleb(0)...)
	hdr = append(hdr, uleb(0)...)
	hdr = append(hdr, uleb(0)...)

	headerLength := len(hdr) - 10
	binary.LittleEndian.PutUint32(hdr[6:], uint32(headerLength))

	prog := de.emitLineProgram(rows)
	unitLength := uint32(len(hdr) - 4 + len(prog))
	binary.LittleEndian.PutUint32(hdr[0:], unitLength)

	return append(hdr, prog...), nil
}

type lineRow struct {
	address     uint64
	file        uint32
	line        uint32
	endSequence bool
}

// lineRows flattens the debug model into an ordered line-row list: a start row
// per function, one row per line mapping, and an end-sequence marker per
// function plus one final marker.
func (de *DwarfEmitter) lineRows(dbg *DebugInfo, textBase uint64) []lineRow {
	rows := make([]lineRow, 0)
	funcs := sortedFunctions(dbg.FunctionInfo)
	for _, fn := range funcs {
		rows = append(rows, lineRow{
			address: fn.Address,
			file:    fn.FileIndex,
			line:    clampU32(fn.StartLine),
		})
	}
	for _, li := range dbg.LineInfo {
		rows = append(rows, lineRow{
			address: li.Address,
			file:    li.FileIndex,
			line:    clampU32(li.Line),
		})
	}
	for _, fn := range funcs {
		rows = append(rows, lineRow{
			address:     fn.Address + fn.Size,
			file:        fn.FileIndex,
			line:        clampU32(fn.EndLine),
			endSequence: true,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].address == rows[j].address {
			return !rows[i].endSequence && rows[j].endSequence
		}
		return rows[i].address < rows[j].address
	})

	dedup := make([]lineRow, 0, len(rows))
	seen := make(map[lineRow]bool)
	for _, r := range rows {
		key := lineRow{address: r.address, file: r.file, line: r.line, endSequence: r.endSequence}
		if seen[key] {
			continue
		}
		seen[key] = true
		dedup = append(dedup, r)
	}
	return dedup
}

type lineState struct {
	address uint64
	file    uint32
	line    uint32
	column  uint32
	isStmt  bool
}

// emitLineProgram encodes the DWARF line-number state machine program.
func (de *DwarfEmitter) emitLineProgram(rows []lineRow) []byte {
	prog := make([]byte, 0)
	state := lineState{address: 0, file: 1, line: 1, isStmt: true}

	for _, r := range rows {
		if r.endSequence {
			prog = de.advanceAddress(prog, &state, r.address)
			prog = append(prog, dwLineExtOp, 0x01, dwLneEndSequence)
			state = lineState{address: 0, file: 1, line: 1, isStmt: true}
			continue
		}
		dwFile := r.file + 1
		if dwFile != state.file {
			prog = append(prog, dwLnsSetFile)
			prog = append(prog, uleb(uint64(dwFile))...)
			state.file = dwFile
		}
		if int64(r.line) != int64(state.line) {
			prog = append(prog, dwLnsAdvanceLine)
			prog = append(prog, sleb(int64(r.line)-int64(state.line))...)
			state.line = r.line
		}
		prog = de.advanceAddress(prog, &state, r.address)
		prog = append(prog, dwLnsCopy)
	}
	return prog
}

// advanceAddress advances the line state machine to target using
// DW_LNS_const_add_pc and DW_LNS_advance_pc.
func (de *DwarfEmitter) advanceAddress(prog []byte, state *lineState, target uint64) []byte {
	if target <= state.address {
		return prog
	}
	const addPC = uint64((255 - int(dwLineOperandBase)) / int(dwLineRange))
	for state.address < target {
		delta := target - state.address
		if delta >= addPC {
			prog = append(prog, dwLnsConstAddPC)
			state.address += addPC
			continue
		}
		prog = append(prog, dwLnsAdvancePC)
		prog = append(prog, uleb(delta)...)
		state.address += delta
	}
	return prog
}

func sortedFunctions(fns []*FunctionInfo) []*FunctionInfo {
	out := make([]*FunctionInfo, len(fns))
	copy(out, fns)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Address == out[j].Address {
			return out[i].Name < out[j].Name
		}
		return out[i].Address < out[j].Address
	})
	return out
}

func clampU32(v uint32) uint32 {
	if v == 0 {
		return 1
	}
	return v
}

func appendDWAttr(buf []byte, v uint32) []byte {
	return binary.LittleEndian.AppendUint32(buf, v)
}

func appendDWAddr(buf []byte, v uint64) []byte {
	return binary.LittleEndian.AppendUint64(buf, v)
}

func appendU16(buf []byte, v uint16) []byte {
	return binary.LittleEndian.AppendUint16(buf, v)
}

func dwarfAddString(offsets map[string]uint32, s string) uint32 {
	return offsets[s]
}

// uleb encodes v as an unsigned LEB128 value.
func uleb(v uint64) []byte {
	var out []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if v == 0 {
			return out
		}
	}
}

// sleb encodes v as a signed LEB128 value.
func sleb(v int64) []byte {
	var out []byte
	for {
		b := byte(v & 0x7f)
		sign := b & 0x40
		v >>= 7
		if (v == 0 && sign == 0) || (v == -1 && sign != 0) {
			out = append(out, b)
			return out
		}
		out = append(out, b|0x80)
	}
}