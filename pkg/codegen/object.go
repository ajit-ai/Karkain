package codegen

import (
	"fmt"
)

// Object represents a native object file with sections, symbols, relocations, and debug info
type Object struct {
	Name        string
	Sections    []*Section
	Symbols     []*Symbol
	Relocations []*Relocation
	DebugInfo   *DebugInfo
}

// Section represents a section in the object file (code, data, bss, etc.)
type Section struct {
	Name    string
	Type    SectionType
	Data    []byte
	Address uint64
	Align   uint32
	Size    uint64 // Size of the section in bytes
}

// SectionType defines the type of a section
type SectionType int

const (
	SectionTypeText   SectionType = iota // Executable code
	SectionTypeData                      // Initialized data
	SectionTypeBSS                       // Uninitialized data
	SectionTypeROData                    // Read-only data
	SectionTypeDebug                     // Debug information
)

// Symbol represents a symbol in the object file
type Symbol struct {
	Name       string
	Type       SymbolType
	Binding    SymbolBinding
	Section    string
	Value      uint64 // Address/offset within section
	Size       uint64
	SourceFile string // Source file where symbol is defined
	SourceLine int    // Line number where symbol is defined
}

// SymbolType defines the type of a symbol
type SymbolType int

const (
	SymbolTypeNone     SymbolType = iota
	SymbolTypeObject              // Data object
	SymbolTypeFunction            // Function entry point
	SymbolTypeSection             // Section name
	SymbolTypeFile                // Source file name
)

// SymbolBinding defines the binding/visibility of a symbol
type SymbolBinding int

const (
	SymbolBindingLocal  SymbolBinding = iota // Local symbol
	SymbolBindingGlobal                      // Global symbol
	SymbolBindingWeak                        // Weak symbol
)

// Relocation represents a relocation entry for address fixups
type Relocation struct {
	Offset  uint64 // Offset within section where relocation applies
	Type    RelocType
	Symbol  string // Symbol name to relocate against
	Addend  int64  // Addend to add to symbol value
	Section string // Section where relocation applies
}

// RelocType defines the type of relocation
type RelocType int

const (
	RelocTypeNone    RelocType = iota
	RelocTypeAddr32            // 32-bit absolute address
	RelocTypeAddr64            // 64-bit absolute address
	RelocTypePCRel32           // 32-bit PC-relative
	RelocTypePCRel64           // 64-bit PC-relative
	RelocTypePLT32             // 32-bit PLT relative
	RelocTypeGOT32             // 32-bit GOT relative
)

// DebugInfo contains source-level debug information
type DebugInfo struct {
	SourceFiles  []*SourceFile
	LineInfo     []*LineInfo
	FunctionInfo []*FunctionInfo
	Variables    []*VariableInfo
}

// SourceFile represents a source file in debug information
type SourceFile struct {
	Path      string
	Directory string
	Checksum  []byte // Optional checksum for file contents
}

// LineInfo maps source locations to generated addresses
type LineInfo struct {
	Address   uint64
	FileIndex uint32
	Line      uint32
	Column    uint32
	Length    uint32 // Length of the instruction/source span
	Function  string // Function containing this line
}

// FunctionInfo contains debug information for a function
type FunctionInfo struct {
	Name       string
	Address    uint64
	Size       uint64
	FileIndex  uint32
	StartLine  uint32
	EndLine    uint32
	Parameters []*VariableInfo
}

// VariableInfo contains debug information for a variable
type VariableInfo struct {
	Name      string
	Type      string
	Address   uint64 // Stack offset or address
	FileIndex uint32
	Line      uint32
	IsLocal   bool
	Function  string // Containing function if local
}

// NewObject creates a new empty object file
func NewObject(name string) *Object {
	return &Object{
		Name:        name,
		Sections:    make([]*Section, 0),
		Symbols:     make([]*Symbol, 0),
		Relocations: make([]*Relocation, 0),
		DebugInfo: &DebugInfo{
			SourceFiles:  make([]*SourceFile, 0),
			LineInfo:     make([]*LineInfo, 0),
			FunctionInfo: make([]*FunctionInfo, 0),
			Variables:    make([]*VariableInfo, 0),
		},
	}
}

// AddSection adds a section to the object file
func (obj *Object) AddSection(name string, sectionType SectionType, data []byte, align uint32) *Section {
	section := &Section{
		Name:  name,
		Type:  sectionType,
		Data:  data,
		Align: align,
		Size:  uint64(len(data)),
	}
	obj.Sections = append(obj.Sections, section)
	return section
}

// AddSymbol adds a symbol to the object file
func (obj *Object) AddSymbol(name string, symbolType SymbolType, binding SymbolBinding, section string, value uint64, size uint64) *Symbol {
	symbol := &Symbol{
		Name:    name,
		Type:    symbolType,
		Binding: binding,
		Section: section,
		Value:   value,
		Size:    size,
	}
	obj.Symbols = append(obj.Symbols, symbol)
	return symbol
}

// AddRelocation adds a relocation to the object file
func (obj *Object) AddRelocation(offset uint64, relocType RelocType, symbol string, addend int64, section string) *Relocation {
	reloc := &Relocation{
		Offset:  offset,
		Type:    relocType,
		Symbol:  symbol,
		Addend:  addend,
		Section: section,
	}
	obj.Relocations = append(obj.Relocations, reloc)
	return reloc
}

// AddSourceFile adds a source file to debug information
func (dbg *DebugInfo) AddSourceFile(path, directory string, checksum []byte) *SourceFile {
	file := &SourceFile{
		Path:      path,
		Directory: directory,
		Checksum:  checksum,
	}
	dbg.SourceFiles = append(dbg.SourceFiles, file)
	return file
}

// AddLineInfo adds a line mapping to debug information
func (dbg *DebugInfo) AddLineInfo(address uint64, fileIndex uint32, line, column, length uint32, function string) *LineInfo {
	info := &LineInfo{
		Address:   address,
		FileIndex: fileIndex,
		Line:      line,
		Column:    column,
		Length:    length,
		Function:  function,
	}
	dbg.LineInfo = append(dbg.LineInfo, info)
	return info
}

// AddFunctionInfo adds function debug information
func (dbg *DebugInfo) AddFunctionInfo(name string, address, size uint64, fileIndex uint32, startLine, endLine uint32) *FunctionInfo {
	info := &FunctionInfo{
		Name:      name,
		Address:   address,
		Size:      size,
		FileIndex: fileIndex,
		StartLine: startLine,
		EndLine:   endLine,
	}
	dbg.FunctionInfo = append(dbg.FunctionInfo, info)
	return info
}

// AddVariableInfo adds variable debug information
func (dbg *DebugInfo) AddVariableInfo(name, varType string, address uint64, fileIndex uint32, line uint32, isLocal bool, function string) *VariableInfo {
	info := &VariableInfo{
		Name:      name,
		Type:      varType,
		Address:   address,
		FileIndex: fileIndex,
		Line:      line,
		IsLocal:   isLocal,
		Function:  function,
	}
	dbg.Variables = append(dbg.Variables, info)
	return info
}

// FindSymbol finds a symbol by name
func (obj *Object) FindSymbol(name string) *Symbol {
	for _, sym := range obj.Symbols {
		if sym.Name == name {
			return sym
		}
	}
	return nil
}

// FindSection finds a section by name
func (obj *Object) FindSection(name string) *Section {
	for _, sect := range obj.Sections {
		if sect.Name == name {
			return sect
		}
	}
	return nil
}

// String returns a string representation of the object file
func (obj *Object) String() string {
	var result string
	result += fmt.Sprintf("Object: %s\n", obj.Name)
	result += fmt.Sprintf("Sections (%d):\n", len(obj.Sections))
	for _, sect := range obj.Sections {
		result += fmt.Sprintf("  %s: type=%d, size=%d, align=%d\n", sect.Name, sect.Type, len(sect.Data), sect.Align)
	}
	result += fmt.Sprintf("Symbols (%d):\n", len(obj.Symbols))
	for _, sym := range obj.Symbols {
		result += fmt.Sprintf("  %s: type=%d, binding=%d, section=%s, value=0x%x, size=%d\n",
			sym.Name, sym.Type, sym.Binding, sym.Section, sym.Value, sym.Size)
	}
	result += fmt.Sprintf("Relocations (%d):\n", len(obj.Relocations))
	for _, reloc := range obj.Relocations {
		result += fmt.Sprintf("  offset=0x%x, type=%d, symbol=%s, addend=%d, section=%s\n",
			reloc.Offset, reloc.Type, reloc.Symbol, reloc.Addend, reloc.Section)
	}
	result += fmt.Sprintf("Debug Info: %d files, %d lines, %d functions, %d variables\n",
		len(obj.DebugInfo.SourceFiles), len(obj.DebugInfo.LineInfo),
		len(obj.DebugInfo.FunctionInfo), len(obj.DebugInfo.Variables))
	return result
}
