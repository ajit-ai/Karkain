package codegen

import (
	"strings"
	"testing"
)

func TestLinker_MinimalProgramLinks(t *testing.T) {
	obj := NewObject("minimal.kark")
	textData := []byte{0x55, 0x48, 0x89, 0xE5, 0xB8, 0x01, 0x00, 0x00, 0x00, 0x5D, 0xC3}
	obj.AddSection(".text", SectionTypeText, textData, 16)
	obj.AddSymbol("main", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0, uint64(len(textData)))

	linker := NewLinker()
	linker.SetEntryPoint("main")
	if err := linker.AddObject(obj); err != nil {
		t.Fatalf("AddObject failed: %v", err)
	}

	exec, err := linker.Link()
	if err != nil {
		t.Fatalf("Link failed: %v", err)
	}
	if exec == nil {
		t.Fatal("Link returned nil executable")
	}
	if exec.EntryPoint == 0 {
		t.Error("entry point should not be 0")
	}
	textSection := exec.GetSection(".text")
	if textSection == nil {
		t.Fatal("no .text section in executable")
	}
	if len(textSection.Data) != len(textData) {
		t.Errorf(".text data length: expected %d, got %d", len(textData), len(textSection.Data))
	}
	if err := exec.Validate(); err != nil {
		t.Errorf("Validate failed: %v", err)
	}
}

func TestLinker_UndefinedSymbolDiagnostic(t *testing.T) {
	obj := NewObject("bad.kark")
	obj.AddSection(".text", SectionTypeText, []byte{0x90}, 16)
	obj.AddRelocation(0, RelocTypeAddr32, "ghost", 0, ".text")
	obj.AddSymbol("main", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0, 1)

	linker := NewLinker()
	linker.SetEntryPoint("main")
	if err := linker.AddObject(obj); err != nil {
		t.Fatalf("AddObject failed: %v", err)
	}

	_, err := linker.Link()
	if err == nil {
		t.Fatal("expected Link error for undefined symbol")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error should mention undefined symbol: %v", err)
	}

	diags := linker.GetDiagnostics()
	if len(diags) == 0 {
		t.Fatal("expected at least one diagnostic")
	}
	found := false
	for _, d := range diags {
		if d.IsError() && strings.Contains(d.Message, "ghost") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error diagnostic mentioning undefined symbol")
	}
}

func TestLinker_DuplicateGlobalDiagnostic(t *testing.T) {
	obj1 := NewObject("a.kark")
	obj1.AddSymbol("foo", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0, 8)

	obj2 := NewObject("b.kark")
	obj2.AddSymbol("foo", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0, 8)

	linker := NewLinker()
	if err := linker.AddObject(obj1); err != nil {
		t.Fatalf("AddObject 1 failed: %v", err)
	}
	err := linker.AddObject(obj2)
	if err == nil {
		t.Fatal("expected error for duplicate global symbol")
	}
	diags := linker.GetDiagnostics()
	found := false
	for _, d := range diags {
		if d.IsError() && strings.Contains(d.Message, "foo") {
			found = true
		}
	}
	if !found {
		t.Error("expected error diagnostic for duplicate global symbol")
	}
}

func TestLinker_MissingEntryPointDiagnostic(t *testing.T) {
	obj := NewObject("no_main.kark")
	obj.AddSection(".text", SectionTypeText, []byte{0x90}, 16)
	obj.AddSymbol("other", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0, 1)

	linker := NewLinker()
	linker.SetEntryPoint("main")
	if err := linker.AddObject(obj); err != nil {
		t.Fatalf("AddObject failed: %v", err)
	}

	_, err := linker.Link()
	if err == nil {
		t.Fatal("expected error for missing entry point")
	}
	diags := linker.GetDiagnostics()
	found := false
	for _, d := range diags {
		if d.IsError() && strings.Contains(d.Message, "main") {
			found = true
		}
	}
	if !found {
		t.Error("expected error diagnostic for missing entry point")
	}
}

func TestLinker_RelocationApplied(t *testing.T) {
	// Object with .data (8 zero bytes) + reloc ADDR32 referencing symbol in .text
	obj := NewObject("reloc.kark")
	textData := []byte{0x55, 0x48, 0x89, 0xE5, 0xC3}
	obj.AddSection(".text", SectionTypeText, textData, 16)
	obj.AddSymbol("helper", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0, uint64(len(textData)))

	dataData := make([]byte, 8)
	obj.AddSection(".data", SectionTypeData, dataData, 8)
	// Relocation at offset 0 of .data referencing helper
	obj.AddRelocation(0, RelocTypeAddr32, "helper", 0, ".data")

	linker := NewLinker()
	linker.SetEntryPoint("main")
	// Need main symbol for entry point
	obj.AddSymbol("main", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0, uint64(len(textData)))
	if err := linker.AddObject(obj); err != nil {
		t.Fatalf("AddObject failed: %v", err)
	}

	exec, err := linker.Link()
	if err != nil {
		t.Fatalf("Link failed: %v", err)
	}
	// .data should have the absolute address of helper written (little-endian 32-bit)
	dataSection := exec.GetSection(".data")
	if dataSection == nil {
		t.Fatal("no .data section")
	}
	addr := uint32(exec.GetSymbol("helper").Value)
	expected0 := byte(addr)
	expected1 := byte(addr >> 8)
	if dataSection.Data[0] != expected0 || dataSection.Data[1] != expected1 {
		t.Errorf("relocation not applied: data[0:2] = 0x%02x%02x, expected 0x%02x%02x",
			dataSection.Data[1], dataSection.Data[0], expected1, expected0)
	}
}

func TestMinimalLinker_LinkSingleObject(t *testing.T) {
	obj := NewObject("mini.kark")
	textData := []byte{0xC3}
	obj.AddSection(".text", SectionTypeText, textData, 16)
	obj.AddSymbol("main", SymbolTypeFunction, SymbolBindingGlobal, ".text", 0, 1)

	ml := NewMinimalLinker()
	exec, err := ml.LinkSingleObject(obj)
	if err != nil {
		t.Fatalf("LinkSingleObject failed: %v", err)
	}
	if exec.EntryPointName != "main" {
		t.Errorf("expected entry point 'main', got '%s'", exec.EntryPointName)
	}
}

func TestExecutable_Size(t *testing.T) {
	exec := &Executable{
		Sections: []*Section{
			{Name: ".text", Data: make([]byte, 100), Size: 100},
			{Name: ".data", Data: make([]byte, 50), Size: 50},
		},
	}
	if exec.Size() != 150 {
		t.Errorf("expected size 150, got %d", exec.Size())
	}
}

func TestExecutable_Validate_EntryOutOfBounds(t *testing.T) {
	exec := &Executable{
		Sections: []*Section{
			{Name: ".text", Address: 0x1000, Size: 32, Data: make([]byte, 32)},
		},
		EntryPoint: 0x2000, // outside .text
		Symbols:    map[string]*Symbol{"main": {Name: "main", Section: ".text", Value: 0x1000}},
	}
	err := exec.Validate()
	if err == nil {
		t.Fatal("expected error for entry point outside .text")
	}
	if !strings.Contains(err.Error(), "outside") {
		t.Errorf("error should mention 'outside': %v", err)
	}
}

func TestExecutable_Validate_NoTextSection(t *testing.T) {
	exec := &Executable{
		Sections:   []*Section{},
		EntryPoint: 0x1000,
		Symbols:    map[string]*Symbol{},
	}
	err := exec.Validate()
	if err == nil {
		t.Fatal("expected error for missing .text section with entry point")
	}
}
