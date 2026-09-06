package codegen

import (
	"fmt"
	"karkain/pkg/parser"
)

// DebugInfoBuilder builds debug information during code generation
type DebugInfoBuilder struct {
	debugInfo     *DebugInfo
	sourceFiles   map[string]*SourceFile
	currentFile   string
	currentLine   int
	currentFunc   string
	addressCounter uint64
}

// NewDebugInfoBuilder creates a new debug information builder
func NewDebugInfoBuilder() *DebugInfoBuilder {
	return &DebugInfoBuilder{
		debugInfo: &DebugInfo{
			SourceFiles:  make([]*SourceFile, 0),
			LineInfo:     make([]*LineInfo, 0),
			FunctionInfo: make([]*FunctionInfo, 0),
			Variables:    make([]*VariableInfo, 0),
		},
		sourceFiles:   make(map[string]*SourceFile),
		addressCounter: 0,
	}
}

// SetSourceFile sets the current source file
func (dib *DebugInfoBuilder) SetSourceFile(path string) {
	dib.currentFile = path
	
	// Create source file entry if not exists
	if _, exists := dib.sourceFiles[path]; !exists {
		file := dib.debugInfo.AddSourceFile(path, "", nil)
		dib.sourceFiles[path] = file
	}
}

// SetCurrentLine sets the current source line
func (dib *DebugInfoBuilder) SetCurrentLine(line int) {
	dib.currentLine = line
}

// SetCurrentFunction sets the current function being generated
func (dib *DebugInfoBuilder) SetCurrentFunction(name string) {
	dib.currentFunc = name
}

// NextAddress increments and returns the next address
func (dib *DebugInfoBuilder) NextAddress() uint64 {
	addr := dib.addressCounter
	dib.addressCounter += 4 // Assume 4-byte instruction size
	return addr
}

// AddLineMapping adds a source line to address mapping
func (dib *DebugInfoBuilder) AddLineMapping(address uint64, length uint32) {
	fileIndex := uint32(0)
	if file, exists := dib.sourceFiles[dib.currentFile]; exists {
		// Find file index
		for i, f := range dib.debugInfo.SourceFiles {
			if f == file {
				fileIndex = uint32(i)
				break
			}
		}
	}
	
	dib.debugInfo.AddLineInfo(address, fileIndex, uint32(dib.currentLine), 0, length, dib.currentFunc)
}

// AddFunctionDebug adds debug information for a function
func (dib *DebugInfoBuilder) AddFunctionDebug(name string, address, size uint64, startLine, endLine int) {
	fileIndex := uint32(0)
	if file, exists := dib.sourceFiles[dib.currentFile]; exists {
		for i, f := range dib.debugInfo.SourceFiles {
			if f == file {
				fileIndex = uint32(i)
				break
			}
		}
	}
	
	dib.debugInfo.AddFunctionInfo(name, address, size, fileIndex, uint32(startLine), uint32(endLine))
}

// AddVariableDebug adds debug information for a variable
func (dib *DebugInfoBuilder) AddVariableDebug(name, varType string, address uint64, line int, isLocal bool) {
	fileIndex := uint32(0)
	if file, exists := dib.sourceFiles[dib.currentFile]; exists {
		for i, f := range dib.debugInfo.SourceFiles {
			if f == file {
				fileIndex = uint32(i)
				break
			}
		}
	}
	
	dib.debugInfo.AddVariableInfo(name, varType, address, fileIndex, uint32(line), isLocal, dib.currentFunc)
}

// GetDebugInfo returns the built debug information
func (dib *DebugInfoBuilder) GetDebugInfo() *DebugInfo {
	return dib.debugInfo
}

// CollectDebugInfoFromAST collects debug information from an AST
func (dib *DebugInfoBuilder) CollectDebugInfoFromAST(prog *parser.Program, sourceFile string) error {
	dib.SetSourceFile(sourceFile)
	
	for _, stmt := range prog.Statements {
		switch node := stmt.(type) {
		case *parser.FuncDecl:
			// Collect function debug info
			funcAddr := dib.NextAddress()
			startLine := node.Line
			endLine := startLine // Will be updated as we process body
			
			dib.SetCurrentFunction(node.Name)
			dib.SetCurrentLine(startLine)
			
			// Collect function parameters
			for _, param := range node.Params {
				dib.AddVariableDebug(param, "param", dib.NextAddress(), startLine, true)
			}
			
			// Process function body to find end line and collect locals
			bodyEndLine := dib.collectDebugFromBlock(node.Body, startLine)
			endLine = bodyEndLine
			
			// Estimate function size (rough approximation)
			funcSize := dib.addressCounter - funcAddr
			
			dib.AddFunctionDebug(node.Name, funcAddr, funcSize, startLine, endLine)
			
		case *parser.StructDeclStmt:
			// Struct debug info (line number)
			dib.SetCurrentLine(node.Line)
			dib.AddLineMapping(dib.NextAddress(), 0)
			
		case *parser.EnumDecl:
			// Enum debug info
			dib.SetCurrentLine(node.Line)
			dib.AddLineMapping(dib.NextAddress(), 0)
			
		case *parser.VarDeclStmt:
			// Variable debug info
			dib.SetCurrentLine(node.Line)
			varAddr := dib.NextAddress()
			dib.AddVariableDebug(node.Name, node.Type, varAddr, node.Line, dib.currentFunc != "")
			dib.AddLineMapping(varAddr, 0)
		}
	}
	
	return nil
}

// collectDebugFromBlock collects debug information from a block of statements
func (dib *DebugInfoBuilder) collectDebugFromBlock(stmts []parser.Node, startLine int) int {
	maxLine := startLine
	
	for _, stmt := range stmts {
		switch node := stmt.(type) {
		case *parser.VarDeclStmt:
			dib.SetCurrentLine(node.Line)
			if node.Line > maxLine {
				maxLine = node.Line
			}
			varAddr := dib.NextAddress()
			dib.AddVariableDebug(node.Name, node.Type, varAddr, node.Line, true)
			dib.AddLineMapping(varAddr, 0)
			
		case *parser.ReturnStmt:
			dib.SetCurrentLine(node.Line)
			if node.Line > maxLine {
				maxLine = node.Line
			}
			dib.AddLineMapping(dib.NextAddress(), 0)
			
		case *parser.IfStmt:
			// Process if body
			ifBodyEnd := dib.collectDebugFromBlock(node.Consequence, node.Line)
			if ifBodyEnd > maxLine {
				maxLine = ifBodyEnd
			}
			
			// Process else body
			if len(node.Alternative) > 0 {
				elseBodyEnd := dib.collectDebugFromBlock(node.Alternative, node.Line)
				if elseBodyEnd > maxLine {
					maxLine = elseBodyEnd
				}
			}
			
		case *parser.WhileStmt:
			bodyEnd := dib.collectDebugFromBlock(node.Body, node.Line)
			if bodyEnd > maxLine {
				maxLine = bodyEnd
			}
			
		case *parser.ForStmt:
			bodyEnd := dib.collectDebugFromBlock(node.Body, node.Line)
			if bodyEnd > maxLine {
				maxLine = bodyEnd
			}
			
		case *parser.ExprStmt:
			dib.SetCurrentLine(node.Line)
			if node.Line > maxLine {
				maxLine = node.Line
			}
			dib.AddLineMapping(dib.NextAddress(), 0)
			
		case *parser.PrintStmt:
			dib.SetCurrentLine(node.Line)
			if node.Line > maxLine {
				maxLine = node.Line
			}
			dib.AddLineMapping(dib.NextAddress(), 0)
		}
	}
	
	return maxLine
}

// SourceAddressMap provides bidirectional mapping between source and addresses
type SourceAddressMap struct {
	sourceToAddr map[string]map[int]uint64  // file -> line -> address
	addrToSource map[uint64]*SourceLocation  // address -> source location
}

// SourceLocation represents a source location
type SourceLocation struct {
	File     string
	Line     int
	Column   int
	Function string
}

// NewSourceAddressMap creates a new source-address mapping
func NewSourceAddressMap() *SourceAddressMap {
	return &SourceAddressMap{
		sourceToAddr: make(map[string]map[int]uint64),
		addrToSource: make(map[uint64]*SourceLocation),
	}
}

// AddMapping adds a source-to-address mapping
func (sam *SourceAddressMap) AddMapping(file string, line int, address uint64, function string) {
	if sam.sourceToAddr[file] == nil {
		sam.sourceToAddr[file] = make(map[int]uint64)
	}
	sam.sourceToAddr[file][line] = address
	
	sam.addrToSource[address] = &SourceLocation{
		File:     file,
		Line:     line,
		Function: function,
	}
}

// GetAddressForSource returns the address for a source location
func (sam *SourceAddressMap) GetAddressForSource(file string, line int) (uint64, bool) {
	if fileMap, exists := sam.sourceToAddr[file]; exists {
		if addr, exists := fileMap[line]; exists {
			return addr, true
		}
	}
	return 0, false
}

// GetSourceForAddress returns the source location for an address
func (sam *SourceAddressMap) GetSourceForAddress(address uint64) (*SourceLocation, bool) {
	loc, exists := sam.addrToSource[address]
	return loc, exists
}

// BuildFromDebugInfo builds a source-address map from debug information
func (sam *SourceAddressMap) BuildFromDebugInfo(debugInfo *DebugInfo) {
	// Add mappings from LineInfo entries
	for _, lineInfo := range debugInfo.LineInfo {
		if int(lineInfo.FileIndex) < len(debugInfo.SourceFiles) {
			file := debugInfo.SourceFiles[lineInfo.FileIndex].Path
			sam.AddMapping(file, int(lineInfo.Line), lineInfo.Address, lineInfo.Function)
		}
	}
	
	// Also add mappings from FunctionInfo entries (function start lines)
	for _, funcInfo := range debugInfo.FunctionInfo {
		if int(funcInfo.FileIndex) < len(debugInfo.SourceFiles) {
			file := debugInfo.SourceFiles[funcInfo.FileIndex].Path
			sam.AddMapping(file, int(funcInfo.StartLine), funcInfo.Address, funcInfo.Name)
		}
	}
}

// FormatAddress formats an address as a hexadecimal string
func FormatAddress(addr uint64) string {
	return fmt.Sprintf("0x%x", addr)
}

// FormatSourceLocation formats a source location as "file:line:column"
func FormatSourceLocation(loc *SourceLocation) string {
	if loc == nil {
		return "<unknown>"
	}
	return fmt.Sprintf("%s:%d", loc.File, loc.Line)
}

// GenerateDebugMap generates a debug map from source to addresses
func GenerateDebugMap(debugInfo *DebugInfo) map[string]map[int]uint64 {
	sam := NewSourceAddressMap()
	sam.BuildFromDebugInfo(debugInfo)
	return sam.sourceToAddr
}

// LookupAddressBySource looks up an address by source location
func LookupAddressBySource(debugInfo *DebugInfo, file string, line int) (uint64, bool) {
	sam := NewSourceAddressMap()
	sam.BuildFromDebugInfo(debugInfo)
	return sam.GetAddressForSource(file, line)
}

// LookupSourceByAddress looks up a source location by address
func LookupSourceByAddress(debugInfo *DebugInfo, address uint64) (*SourceLocation, bool) {
	sam := NewSourceAddressMap()
	sam.BuildFromDebugInfo(debugInfo)
	return sam.GetSourceForAddress(address)
}
