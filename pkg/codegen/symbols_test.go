package codegen

import (
	"strings"
	"testing"

	"karkain/pkg/parser"
)

func TestSymbolTable_DefineAndLookup(t *testing.T) {
	st := NewSymbolTable()
	err := st.DefineSymbol("karkain_user_helper", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0x1000, 0x20)
	if err != nil {
		t.Fatalf("DefineSymbol failed: %v", err)
	}
	sym := st.LookupSymbol("karkain_user_helper")
	if sym == nil {
		t.Fatal("LookupSymbol returned nil for defined symbol")
	}
	if sym.Name != "karkain_user_helper" {
		t.Errorf("expected name 'karkain_user_helper', got '%s'", sym.Name)
	}
	if sym.Binding != SymbolBindingGlobal {
		t.Errorf("expected global binding, got %d", sym.Binding)
	}
	if sym.Value != 0x1000 {
		t.Errorf("expected value 0x1000, got 0x%x", sym.Value)
	}
}

func TestSymbolTable_DuplicateRejects(t *testing.T) {
	st := NewSymbolTable()
	if err := st.DefineSymbol("x", SymbolTypeObject, SymbolBindingLocal, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := st.DefineSymbol("x", SymbolTypeObject, SymbolBindingLocal, "", 0, 0); err == nil {
		t.Fatal("expected error on duplicate definition")
	}
}

func TestSymbolTable_ScopedShadowing(t *testing.T) {
	st := NewSymbolTable()
	st.DefineSymbol("count", SymbolTypeObject, SymbolBindingGlobal, ".data", 0, 0)
	st.EnterScope("inner")
	st.DefineSymbol("count", SymbolTypeObject, SymbolBindingLocal, "", 0, 0)
	inner := st.LookupSymbol("count")
	if inner == nil || inner.Binding != SymbolBindingLocal {
		t.Fatal("inner scope should shadow outer 'count'")
	}
	st.ExitScope()
	outer := st.LookupSymbol("count")
	if outer == nil || outer.Binding != SymbolBindingGlobal {
		t.Fatal("after ExitScope, should resolve to outer global 'count'")
	}
}

func TestSymbolTable_External(t *testing.T) {
	st := NewSymbolTable()
	st.DefineExternal("karkain_user_foo")
	if !st.IsExternal("karkain_user_foo") {
		t.Error("expected IsExternal true for marked symbol")
	}
	if st.IsExternal("karkain_user_bar") {
		t.Error("expected IsExternal false for unmarked symbol")
	}
}

func TestCollectSymbolsFromAST(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{
				Name:   "main",
				Params: []string{},
				Body:   []parser.Node{},
			},
			&parser.FuncDecl{
				Name:   "abs",
				Params: []string{"x"},
				Body:   []parser.Node{},
			},
			&parser.StructDeclStmt{
				Name:   "Point",
				Fields: []parser.StructField{{Name: "x", Type: "int"}, {Name: "y", Type: "int"}},
			},
		},
	}

	obj := NewObject("test.kark")
	st := NewSymbolTable()
	if err := st.CollectSymbolsFromAST(prog, obj); err != nil {
		t.Fatalf("CollectSymbolsFromAST failed: %v", err)
	}

	// main keeps canonical name; abs gets namespaced
	names := make(map[string]bool)
	for _, sym := range obj.Symbols {
		names[sym.Name] = true
	}
	if !names["main"] {
		t.Error("expected symbol 'main' (canonical)")
	}
	if !names["karkain_user_abs"] {
		t.Error("expected symbol 'karkain_user_abs' (namespaced)")
	}
	if !names["Point"] {
		t.Error("expected symbol 'Point'")
	}
}

func TestMangleAndDemangle(t *testing.T) {
	mangled := MangleSymbol("foo", true)
	if !strings.HasPrefix(mangled, "karkain_user_") {
		t.Errorf("expected karkain_user_ prefix, got '%s'", mangled)
	}
	demangled := DemangleSymbol(mangled)
	if demangled != "foo" {
		t.Errorf("expected demangled 'foo', got '%s'", demangled)
	}
	// Non-prefixed name passes through
	if DemangleSymbol("main") != "main" {
		t.Error("DemangleSymbol should pass through non-prefixed names")
	}
}

func TestValidateSymbolNames_RejectsRuntime(t *testing.T) {
	syms := []*Symbol{
		{Name: "main", Type: SymbolTypeFunction, Binding: SymbolBindingGlobal, Section: ".text", Value: 0, Size: 0},
		{Name: "karkain_user_safe", Type: SymbolTypeFunction, Binding: SymbolBindingGlobal, Section: ".text", Value: 0, Size: 0},
		{Name: "__karkain_runtime_bad", Type: SymbolTypeFunction, Binding: SymbolBindingGlobal, Section: ".text", Value: 0, Size: 0},
	}
	err := ValidateSymbolNames(syms)
	if err == nil {
		t.Fatal("expected error for __karkain_runtime_bad")
	}
	if !strings.Contains(err.Error(), "__karkain_runtime_bad") {
		t.Errorf("error should mention bad symbol name: %v", err)
	}
}

func TestGetGlobalSymbols(t *testing.T) {
	st := NewSymbolTable()
	st.DefineSymbol("karkain_user_global_fn", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0, 0)
	st.EnterScope("fn")
	st.DefineSymbol("local_var", SymbolTypeObject, SymbolBindingLocal, "", 0, 0)
	st.ExitScope()

	globals := st.GetGlobalSymbols()
	if len(globals) != 1 {
		t.Fatalf("expected 1 global, got %d", len(globals))
	}
	if globals[0].Name != "karkain_user_global_fn" {
		t.Errorf("expected global symbol 'karkain_user_global_fn', got '%s'", globals[0].Name)
	}
}
