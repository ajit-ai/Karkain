package codegen

import (
	"strings"
	"testing"
)

func TestObjectCreation(t *testing.T) {
	obj := NewObject("test")
	if obj == nil {
		t.Fatal("NewObject returned nil")
	}
	if obj.Name != "test" {
		t.Errorf("expected name 'test', got '%s'", obj.Name)
	}
	if len(obj.Sections) != 0 {
		t.Errorf("expected no sections, got %d", len(obj.Sections))
	}
	if len(obj.Symbols) != 0 {
		t.Errorf("expected no symbols, got %d", len(obj.Symbols))
	}
	if obj.DebugInfo == nil {
		t.Error("expected debug info to be initialized")
	}
}

func TestAddSection(t *testing.T) {
	obj := NewObject("test")

	section := obj.AddSection(".text", SectionTypeText, []byte{0x90, 0x90}, 16)
	if section == nil {
		t.Fatal("AddSection returned nil")
	}
	if section.Name != ".text" {
		t.Errorf("expected section name '.text', got '%s'", section.Name)
	}
	if section.Type != SectionTypeText {
		t.Errorf("expected section type Text, got %d", section.Type)
	}
	if section.Align != 16 {
		t.Errorf("expected alignment 16, got %d", section.Align)
	}
	if len(obj.Sections) != 1 {
		t.Errorf("expected 1 section, got %d", len(obj.Sections))
	}
}

func TestAddSymbol(t *testing.T) {
	obj := NewObject("test")

	symbol := obj.AddSymbol("main", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0x1000, 0x50)
	if symbol == nil {
		t.Fatal("AddSymbol returned nil")
	}
	if symbol.Name != "main" {
		t.Errorf("expected symbol name 'main', got '%s'", symbol.Name)
	}
	if symbol.Type != SymbolTypeFunction {
		t.Errorf("expected symbol type Function, got %d", symbol.Type)
	}
	if symbol.Binding != SymbolBindingGlobal {
		t.Errorf("expected symbol binding Global, got %d", symbol.Binding)
	}
	if symbol.Section != ".text" {
		t.Errorf("expected section '.text', got '%s'", symbol.Section)
	}
	if symbol.Value != 0x1000 {
		t.Errorf("expected value 0x1000, got 0x%x", symbol.Value)
	}
	if symbol.Size != 0x50 {
		t.Errorf("expected size 0x50, got 0x%x", symbol.Size)
	}
	if len(obj.Symbols) != 1 {
		t.Errorf("expected 1 symbol, got %d", len(obj.Symbols))
	}
}

func TestAddRelocation(t *testing.T) {
	obj := NewObject("test")

	reloc := obj.AddRelocation(0x100, RelocTypeAddr32, "main", 0, ".text")
	if reloc == nil {
		t.Fatal("AddRelocation returned nil")
	}
	if reloc.Offset != 0x100 {
		t.Errorf("expected offset 0x100, got 0x%x", reloc.Offset)
	}
	if reloc.Type != RelocTypeAddr32 {
		t.Errorf("expected relocation type Addr32, got %d", reloc.Type)
	}
	if reloc.Symbol != "main" {
		t.Errorf("expected symbol 'main', got '%s'", reloc.Symbol)
	}
	if reloc.Addend != 0 {
		t.Errorf("expected addend 0, got %d", reloc.Addend)
	}
	if reloc.Section != ".text" {
		t.Errorf("expected section '.text', got '%s'", reloc.Section)
	}
	if len(obj.Relocations) != 1 {
		t.Errorf("expected 1 relocation, got %d", len(obj.Relocations))
	}
}

func TestFindSymbol(t *testing.T) {
	obj := NewObject("test")
	obj.AddSymbol("main", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0x1000, 0x50)

	sym := obj.FindSymbol("main")
	if sym == nil {
		t.Fatal("FindSymbol returned nil for existing symbol")
	}
	if sym.Name != "main" {
		t.Errorf("expected to find 'main', got '%s'", sym.Name)
	}

	// Test non-existent symbol
	sym = obj.FindSymbol("nonexistent")
	if sym != nil {
		t.Error("expected nil for non-existent symbol")
	}
}

func TestFindSection(t *testing.T) {
	obj := NewObject("test")
	obj.AddSection(".text", SectionTypeText, []byte{0x90}, 16)

	sect := obj.FindSection(".text")
	if sect == nil {
		t.Fatal("FindSection returned nil for existing section")
	}
	if sect.Name != ".text" {
		t.Errorf("expected to find '.text', got '%s'", sect.Name)
	}

	// Test non-existent section
	sect = obj.FindSection(".data")
	if sect != nil {
		t.Error("expected nil for non-existent section")
	}
}

func TestDebugInfo(t *testing.T) {
	obj := NewObject("test")

	file := obj.DebugInfo.AddSourceFile("test.kark", "", nil)
	if file == nil {
		t.Fatal("AddSourceFile returned nil")
	}
	if file.Path != "test.kark" {
		t.Errorf("expected file path 'test.kark', got '%s'", file.Path)
	}

	lineInfo := obj.DebugInfo.AddLineInfo(0x1000, 0, 10, 5, 4, "main")
	if lineInfo == nil {
		t.Fatal("AddLineInfo returned nil")
	}
	if lineInfo.Address != 0x1000 {
		t.Errorf("expected address 0x1000, got 0x%x", lineInfo.Address)
	}
	if lineInfo.Line != 10 {
		t.Errorf("expected line 10, got %d", lineInfo.Line)
	}

	funcInfo := obj.DebugInfo.AddFunctionInfo("main", 0x1000, 0x50, 0, 1, 20)
	if funcInfo == nil {
		t.Fatal("AddFunctionInfo returned nil")
	}
	if funcInfo.Name != "main" {
		t.Errorf("expected function name 'main', got '%s'", funcInfo.Name)
	}

	varInfo := obj.DebugInfo.AddVariableInfo("x", "int", 0x1050, 0, 5, true, "main")
	if varInfo == nil {
		t.Fatal("AddVariableInfo returned nil")
	}
	if varInfo.Name != "x" {
		t.Errorf("expected variable name 'x', got '%s'", varInfo.Name)
	}
}

func TestSectionTypes(t *testing.T) {
	obj := NewObject("test")

	obj.AddSection(".text", SectionTypeText, []byte{}, 16)
	obj.AddSection(".data", SectionTypeData, []byte{}, 8)
	obj.AddSection(".bss", SectionTypeBSS, []byte{}, 4)
	obj.AddSection(".rodata", SectionTypeROData, []byte{}, 8)

	if len(obj.Sections) != 4 {
		t.Errorf("expected 4 sections, got %d", len(obj.Sections))
	}

	types := []SectionType{SectionTypeText, SectionTypeData, SectionTypeBSS, SectionTypeROData}
	for i, sect := range obj.Sections {
		if sect.Type != types[i] {
			t.Errorf("section %d: expected type %d, got %d", i, types[i], sect.Type)
		}
	}
}

func TestSymbolTypes(t *testing.T) {
	obj := NewObject("test")

	obj.AddSymbol("func1", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0, 0)
	obj.AddSymbol("var1", SymbolTypeObject, SymbolBindingLocal, "", 0, 0)
	obj.AddSymbol("section1", SymbolTypeSection, SymbolBindingGlobal, ".data", 0, 0)

	if len(obj.Symbols) != 3 {
		t.Errorf("expected 3 symbols, got %d", len(obj.Symbols))
	}

	bindings := []SymbolBinding{SymbolBindingGlobal, SymbolBindingLocal, SymbolBindingGlobal}
	for i, sym := range obj.Symbols {
		if sym.Binding != bindings[i] {
			t.Errorf("symbol %d: expected binding %d, got %d", i, bindings[i], sym.Binding)
		}
	}
}

func TestObjectString(t *testing.T) {
	obj := NewObject("test")
	obj.AddSection(".text", SectionTypeText, []byte{0x90, 0x90}, 16)
	obj.AddSymbol("main", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0x1000, 0x50)
	obj.AddRelocation(0x100, RelocTypeAddr32, "main", 0, ".text")

	str := obj.String()
	if str == "" {
		t.Error("Object.String() returned empty string")
	}

	// Check that key information is present
	checks := []string{"Object: test", "Sections (1)", "Symbols (1)", "Relocations (1)", "Debug Info"}
	for _, check := range checks {
		if !contains(str, check) {
			t.Errorf("Object.String() missing expected text: %s", check)
		}
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
