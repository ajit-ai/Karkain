package codegen

import (
	"fmt"
	"karkain/pkg/parser"
	"strings"
)

// SymbolTable manages symbols during code generation
type SymbolTable struct {
	symbols     map[string]*Symbol
	externals   map[string]bool
	scopes      []*Scope
	currentScope *Scope
}

// Scope represents a lexical scope for symbol visibility
type Scope struct {
	parent  *Scope
	symbols map[string]*Symbol
	name    string
	level   int
}

// NewSymbolTable creates a new symbol table
func NewSymbolTable() *SymbolTable {
	globalScope := &Scope{
		symbols: make(map[string]*Symbol),
		name:    "global",
		level:   0,
	}
	return &SymbolTable{
		symbols:   make(map[string]*Symbol),
		externals: make(map[string]bool),
		scopes:    []*Scope{globalScope},
		currentScope: globalScope,
	}
}

// EnterScope enters a new lexical scope
func (st *SymbolTable) EnterScope(name string) {
	newScope := &Scope{
		parent:  st.currentScope,
		symbols: make(map[string]*Symbol),
		name:    name,
		level:   st.currentScope.level + 1,
	}
	st.scopes = append(st.scopes, newScope)
	st.currentScope = newScope
}

// ExitScope exits the current scope
func (st *SymbolTable) ExitScope() {
	if len(st.scopes) > 1 {
		st.scopes = st.scopes[:len(st.scopes)-1]
		st.currentScope = st.scopes[len(st.scopes)-1]
	}
}

// DefineSymbol defines a symbol in the current scope
func (st *SymbolTable) DefineSymbol(name string, symbolType SymbolType, binding SymbolBinding, section string, value uint64, size uint64) error {
	// Check if symbol already exists in current scope
	if _, exists := st.currentScope.symbols[name]; exists {
		return fmt.Errorf("symbol '%s' already defined in current scope", name)
	}

	symbol := &Symbol{
		Name:    name,
		Type:    symbolType,
		Binding: binding,
		Section: section,
		Value:   value,
		Size:    size,
	}

	st.currentScope.symbols[name] = symbol
	st.symbols[name] = symbol
	return nil
}

// LookupSymbol looks up a symbol by name, searching from current scope outward
func (st *SymbolTable) LookupSymbol(name string) *Symbol {
	for scope := st.currentScope; scope != nil; scope = scope.parent {
		if sym, exists := scope.symbols[name]; exists {
			return sym
		}
	}
	return nil
}

// DefineExternal marks a symbol as external (defined elsewhere)
func (st *SymbolTable) DefineExternal(name string) {
	st.externals[name] = true
}

// IsExternal checks if a symbol is external
func (st *SymbolTable) IsExternal(name string) bool {
	return st.externals[name]
}

// CollectSymbolsFromAST collects symbols from an AST for object generation
func (st *SymbolTable) CollectSymbolsFromAST(prog *parser.Program, object *Object) error {
	for _, stmt := range prog.Statements {
		switch node := stmt.(type) {
		case *parser.FuncDecl:
			// Define function symbol using namespaced name
			cName := userFuncC(node.Name)
			symbolType := SymbolTypeFunction
			binding := SymbolBindingGlobal
			
			// main and getArgs are special - they keep canonical names
			if node.Name == "main" || node.Name == "getArgs" {
				cName = node.Name
			}
			
			err := st.DefineSymbol(cName, symbolType, binding, ".text", 0, 0)
			if err != nil {
				return fmt.Errorf("failed to define function symbol '%s': %w", cName, err)
			}
			
			// Add to object symbols
			object.AddSymbol(cName, symbolType, binding, ".text", 0, 0)
			
			// Collect function parameters as local symbols
			st.EnterScope(node.Name)
			for _, param := range node.Params {
				paramSymName := param
				err := st.DefineSymbol(paramSymName, SymbolTypeObject, SymbolBindingLocal, "", 0, 0)
				if err != nil {
					return fmt.Errorf("failed to define parameter symbol '%s': %w", paramSymName, err)
				}
			}
			
			// Collect local variables from function body
			st.collectSymbolsFromBlock(node.Body, object, node.Name)
			st.ExitScope()
			
		case *parser.StructDeclStmt:
			// Define struct as a type symbol
			cName := node.Name
			err := st.DefineSymbol(cName, SymbolTypeObject, SymbolBindingGlobal, ".rodata", 0, 0)
			if err != nil {
				return fmt.Errorf("failed to define struct symbol '%s': %w", cName, err)
			}
			object.AddSymbol(cName, SymbolTypeObject, SymbolBindingGlobal, ".rodata", 0, 0)
			
		case *parser.EnumDecl:
			// Define enum as a type symbol
			cName := node.Name
			err := st.DefineSymbol(cName, SymbolTypeObject, SymbolBindingGlobal, ".rodata", 0, 0)
			if err != nil {
				return fmt.Errorf("failed to define enum symbol '%s': %w", cName, err)
			}
			object.AddSymbol(cName, SymbolTypeObject, SymbolBindingGlobal, ".rodata", 0, 0)
			
		case *parser.VarDeclStmt:
			// Define variable symbol
			varSymName := node.Name
			varSymType := SymbolTypeObject
			varBinding := SymbolBindingLocal
			varSection := ""
			
			// Global variables (top-level) get global binding
			if st.currentScope.name == "global" {
				varBinding = SymbolBindingGlobal
				varSection = ".data"
				// Use namespaced name for globals
				varSymName = userFuncC(node.Name)
			}
			
			err := st.DefineSymbol(varSymName, varSymType, varBinding, varSection, 0, 0)
			if err != nil {
				return fmt.Errorf("failed to define variable symbol '%s': %w", varSymName, err)
			}
			object.AddSymbol(varSymName, varSymType, varBinding, varSection, 0, 0)
		}
	}
	return nil
}

// collectSymbolsFromBlock collects symbols from a block of statements
func (st *SymbolTable) collectSymbolsFromBlock(stmts []parser.Node, object *Object, functionName string) {
	for _, stmt := range stmts {
		switch node := stmt.(type) {
		case *parser.VarDeclStmt:
			// Local variable in function
			varSymName := node.Name
			err := st.DefineSymbol(varSymName, SymbolTypeObject, SymbolBindingLocal, "", 0, 0)
			if err == nil {
				object.AddSymbol(varSymName, SymbolTypeObject, SymbolBindingLocal, "", 0, 0)
			}
			
		case *parser.IfStmt:
			// Enter new scope for if body
			st.EnterScope("if")
			st.collectSymbolsFromBlock(node.Consequence, object, functionName)
			st.ExitScope()
			
			// Handle else branch
			if len(node.Alternative) > 0 {
				st.EnterScope("else")
				st.collectSymbolsFromBlock(node.Alternative, object, functionName)
				st.ExitScope()
			}
			
		case *parser.WhileStmt:
			// Enter new scope for while body
			st.EnterScope("while")
			st.collectSymbolsFromBlock(node.Body, object, functionName)
			st.ExitScope()
			
		case *parser.ForStmt:
			// Enter new scope for for body
			st.EnterScope("for")
			st.collectSymbolsFromBlock(node.Body, object, functionName)
			st.ExitScope()
		}
	}
}

// MangleSymbol applies Karkain symbol mangling rules
func MangleSymbol(name string, isFunction bool) string {
	if !isFunction {
		return userFuncC(name)
	}
	return userFuncC(name)
}

// DemangleSymbol attempts to demangle a Karkain symbol name
func DemangleSymbol(mangled string) string {
	// Remove karkain_user_ prefix if present
	if strings.HasPrefix(mangled, "karkain_user_") {
		return strings.TrimPrefix(mangled, "karkain_user_")
	}
	return mangled
}

// ValidateSymbolNames ensures symbols don't collide with runtime symbols
func ValidateSymbolNames(symbols []*Symbol) error {
	runtimePrefixes := []string{"karkain_", "__karkain"}
	
	for _, sym := range symbols {
		// Skip main and getArgs as they are special
		if sym.Name == "main" || sym.Name == "getArgs" {
			continue
		}
		
		// Check if symbol collides with runtime symbols
		for _, prefix := range runtimePrefixes {
			if strings.HasPrefix(sym.Name, prefix) && !strings.HasPrefix(sym.Name, "karkain_user_") {
				return fmt.Errorf("symbol '%s' collides with runtime symbol prefix '%s'", sym.Name, prefix)
			}
		}
	}
	return nil
}

// GetGlobalSymbols returns all global symbols
func (st *SymbolTable) GetGlobalSymbols() []*Symbol {
	var globals []*Symbol
	for _, sym := range st.symbols {
		if sym.Binding == SymbolBindingGlobal {
			globals = append(globals, sym)
		}
	}
	return globals
}

// GetLocalSymbols returns all local symbols in the current scope
func (st *SymbolTable) GetLocalSymbols() []*Symbol {
	var locals []*Symbol
	for _, sym := range st.currentScope.symbols {
		if sym.Binding == SymbolBindingLocal {
			locals = append(locals, sym)
		}
	}
	return locals
}
