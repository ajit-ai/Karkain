package sema

import (
	"fmt"
	"strings"

	"karkain/pkg/jit"
	"karkain/pkg/parser"
)

// ============================================================
// Phase 36: FFI Semantic Analysis & AST Processing
// Handles extern "C" block declarations, type checking,
// and symbol resolution for foreign function calls
// ============================================================

// FFIBlock represents a parsed extern "C" declaration block
type FFIBlock struct {
	Functions []FFIFuncDecl
	Line      int
}

// FFIFuncDecl represents a single foreign function declaration
type FFIFuncDecl struct {
	Name     string
	RetType  string
	Params   []FFIParam
	ABI      string // "C", "stdcall", "fastcall"
	IsVarArg bool
}

// FFIParam represents a parameter in a foreign function
type FFIParam struct {
	Name string
	Type string
}

// FFIChecker performs semantic analysis on FFI declarations
type FFIChecker struct {
	Blocks     []*FFIBlock
	Registered map[string]*FFIFuncDecl
	Registry   *jit.FFIRegistry
	Errors     []string
	Warnings   []string
}

// NewFFIChecker creates a new FFI semantic checker
func NewFFIChecker() *FFIChecker {
	return &FFIChecker{
		Registered: make(map[string]*FFIFuncDecl),
		Registry:   jit.NewFFIRegistry(),
	}
}

// ============================================================
// AST Processing: extern "C" blocks
// ============================================================

// ProcessProgram scans a program for FFI declarations
func (fc *FFIChecker) ProcessProgram(prog *parser.Program) error {
	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *parser.CImportBlock:
			block, err := fc.ParseCImportBlock(n.Content)
			if err != nil {
				fc.Errors = append(fc.Errors, err.Error())
				continue
			}
			fc.Blocks = append(fc.Blocks, block)
			for _, fn := range block.Functions {
				fc.Registered[fn.Name] = &fn
			}
		}
	}
	if len(fc.Errors) > 0 {
		return fmt.Errorf("ffi: %d error(s) in FFI declarations", len(fc.Errors))
	}
	return nil
}

// ParseCImportBlock parses the content of a C import block into structured declarations
func (fc *FFIChecker) ParseCImportBlock(content string) (*FFIBlock, error) {
	block := &FFIBlock{}

	content = strings.TrimSpace(content)
	if content == "" {
		return block, nil
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		decl, err := fc.ParseFuncDecl(line)
		if err != nil {
			fc.Warnings = append(fc.Warnings, err.Error())
			continue
		}
		if decl != nil {
			block.Functions = append(block.Functions, *decl)
		}
	}

	return block, nil
}

// ParseFuncDecl parses a single function declaration line
// e.g.: fn printf(fmt: *i8, args: ...) -> i32;
func (fc *FFIChecker) ParseFuncDecl(line string) (*FFIFuncDecl, error) {
	line = strings.TrimSuffix(line, ";")
	line = strings.TrimSpace(line)

	if !strings.HasPrefix(line, "fn ") {
		return nil, fmt.Errorf("expected 'fn' keyword, got: %s", line)
	}

	line = strings.TrimPrefix(line, "fn ")
	decl := &FFIFuncDecl{ABI: "C"}

	// Extract function name
	parenIdx := strings.Index(line, "(")
	if parenIdx < 0 {
		return nil, fmt.Errorf("missing parameter list: %s", line)
	}

	decl.Name = strings.TrimSpace(line[:parenIdx])
	rest := line[parenIdx+1:]

	// Extract parameters
	closeParen := strings.LastIndex(rest, ")")
	if closeParen < 0 {
		return nil, fmt.Errorf("unclosed parameter list: %s", line)
	}

	paramStr := rest[:closeParen]
	afterParen := strings.TrimSpace(rest[closeParen+1:])

	// Check for varargs
	if strings.Contains(paramStr, "...") {
		decl.IsVarArg = true
		// Remove the entire varargs parameter (e.g., "args: ..." or just "...")
		paramStr = strings.ReplaceAll(paramStr, "...", "")
		// Remove any trailing comma + whitespace after removing varargs
		paramStr = strings.TrimRight(paramStr, " ,\t")
		// Remove dangling parameter name before comma (e.g., "args: " → "")
		// If paramStr ends with ":" or name + ":" it was a varargs param
		paramStr = strings.TrimSpace(paramStr)
		if idx := strings.LastIndex(paramStr, ","); idx >= 0 {
			left := strings.TrimSpace(paramStr[:idx])
			right := strings.TrimSpace(paramStr[idx+1:])
			if right == "" || strings.Contains(right, ":") && strings.TrimSpace(strings.SplitN(right, ":", 2)[1]) == "" {
				paramStr = left
			}
		}
	}

	// Parse parameters
	if strings.TrimSpace(paramStr) != "" {
		params := strings.Split(paramStr, ",")
		for _, p := range params {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			param, err := fc.ParseParam(p)
			if err != nil {
				fc.Warnings = append(fc.Warnings, fmt.Sprintf("param parse error: %v", err))
				continue
			}
			decl.Params = append(decl.Params, *param)
		}
	}

	// Parse return type
	if strings.HasPrefix(afterParen, "->") {
		retType := strings.TrimSpace(strings.TrimPrefix(afterParen, "->"))
		decl.RetType = retType
	} else {
		decl.RetType = "void"
	}

	return decl, nil
}

// ParseParam parses a single parameter declaration
// e.g.: "fmt: *i8" or "size: u64"
func (fc *FFIChecker) ParseParam(s string) (*FFIParam, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty parameter")
	}

	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid parameter format: %s", s)
	}

	return &FFIParam{
		Name: strings.TrimSpace(parts[0]),
		Type: strings.TrimSpace(parts[1]),
	}, nil
}

// ============================================================
// Type Validation
// ============================================================

// ValidateTypes checks that all declared types are valid C types
func (fc *FFIChecker) ValidateTypes() error {
	validTypes := map[string]bool{
		"void": true, "i8": true, "i16": true, "i32": true, "i64": true,
		"u8": true, "u16": true, "u32": true, "u64": true,
		"f32": true, "f64": true, "bool": true,
		"*void": true, "*i8": true, "*u8": true,
		"*i32": true, "*u32": true, "*i64": true, "*u64": true,
		"*f32": true, "*f64": true, "*char": true, "*byte": true,
		"string": true,
	}

	for _, block := range fc.Blocks {
		for _, fn := range block.Functions {
			if !validTypes[fn.RetType] {
				fc.Errors = append(fc.Errors,
					fmt.Sprintf("invalid return type '%s' in '%s'", fn.RetType, fn.Name))
			}
			for _, param := range fn.Params {
				if !validTypes[param.Type] {
					fc.Errors = append(fc.Errors,
						fmt.Sprintf("invalid parameter type '%s' in '%s.%s'", param.Type, fn.Name, param.Name))
				}
			}
		}
	}

	if len(fc.Errors) > 0 {
		return fmt.Errorf("ffi: %d type validation error(s)", len(fc.Errors))
	}
	return nil
}

// ============================================================
// Symbol Resolution
// ============================================================

// ResolveSymbols attempts to resolve all declared functions in the registry
func (fc *FFIChecker) ResolveSymbols(libName string) error {
	lib, err := fc.Registry.LoadLibrary(libName, "")
	if err != nil {
		return err
	}

	for _, block := range fc.Blocks {
		for _, fn := range block.Functions {
			_, err := lib.ResolveSymbol(fn.Name)
			if err != nil {
				fc.Warnings = append(fc.Warnings,
					fmt.Sprintf("symbol '%s' not dynamically resolved (will use simulation)", fn.Name))
			}
		}
	}
	return nil
}

// ============================================================
// Call Validation
// ============================================================

// ValidateCall checks that a function call matches a declared FFI function
func (fc *FFIChecker) ValidateCall(funcName string, numArgs int) error {
	decl, ok := fc.Registered[funcName]
	if !ok {
		return fmt.Errorf("ffi: function '%s' not declared in extern \"C\" block", funcName)
	}

	if !decl.IsVarArg && len(decl.Params) != numArgs {
		return fmt.Errorf("ffi: '%s' expects %d args, got %d", funcName, len(decl.Params), numArgs)
	}

	if !decl.IsVarArg && len(decl.Params) < numArgs {
		return fmt.Errorf("ffi: '%s' accepts max %d args, got %d", funcName, len(decl.Params), numArgs)
	}

	return nil
}

// ValidateCallTypes checks that argument types match the declaration
func (fc *FFIChecker) ValidateCallTypes(funcName string, argTypes []string) error {
	decl, ok := fc.Registered[funcName]
	if !ok {
		return fmt.Errorf("ffi: function '%s' not declared", funcName)
	}

	if !decl.IsVarArg && len(argTypes) < len(decl.Params) {
		return fmt.Errorf("ffi: '%s' expects at least %d args, got %d", funcName, len(decl.Params), len(argTypes))
	}

	for i, argType := range argTypes {
		if i >= len(decl.Params) {
			if decl.IsVarArg {
				break
			}
			return fmt.Errorf("ffi: too many arguments for '%s'", funcName)
		}
		expected := decl.Params[i].Type
		if !typesCompatible(expected, argType) {
			return fmt.Errorf("ffi: argument %d of '%s': expected '%s', got '%s'",
				i, funcName, expected, argType)
		}
	}

	return nil
}

// ============================================================
// Code Generation Helpers
// ============================================================

// GenerateFFIBridge generates the JIT bridge code for a function declaration
func (fc *FFIChecker) GenerateFFIBridge(fn *FFIFuncDecl) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("// FFI Bridge: %s\n", fn.Name))
	b.WriteString(fmt.Sprintf("// RetType: %s\n", fn.RetType))
	b.WriteString(fmt.Sprintf("// Params: %d\n", len(fn.Params)))
	for i, p := range fn.Params {
		b.WriteString(fmt.Sprintf("//   [%d] %s: %s\n", i, p.Name, p.Type))
	}
	if fn.IsVarArg {
		b.WriteString("// VarArgs: true\n")
	}
	return b.String()
}

// ============================================================
// Type Compatibility
// ============================================================

func typesCompatible(expected, actual string) bool {
	if expected == actual {
		return true
	}

	// Pointer compatibility
	if strings.HasPrefix(expected, "*") && strings.HasPrefix(actual, "*") {
		return true
	}

	// Integer compatibility
	intTypes := map[string]bool{"i8": true, "i16": true, "i32": true, "i64": true,
		"u8": true, "u16": true, "u32": true, "u64": true}
	if intTypes[expected] && intTypes[actual] {
		return true
	}

	// Float compatibility
	floatTypes := map[string]bool{"f32": true, "f64": true}
	if floatTypes[expected] && floatTypes[actual] {
		return true
	}

	return false
}

// ============================================================
// GetDeclarations returns all registered FFI declarations
func (fc *FFIChecker) GetDeclarations() map[string]*FFIFuncDecl {
	return fc.Registered
}

// HasDeclaration checks if a function is declared
func (fc *FFIChecker) HasDeclaration(name string) bool {
	_, ok := fc.Registered[name]
	return ok
}

// GetReturnType returns the return type for a declared function
func (fc *FFIChecker) GetReturnType(name string) string {
	if decl, ok := fc.Registered[name]; ok {
		return decl.RetType
	}
	return ""
}

// GetParamTypes returns the parameter types for a declared function
func (fc *FFIChecker) GetParamTypes(name string) []string {
	if decl, ok := fc.Registered[name]; ok {
		types := make([]string, len(decl.Params))
		for i, p := range decl.Params {
			types[i] = p.Type
		}
		return types
	}
	return nil
}
