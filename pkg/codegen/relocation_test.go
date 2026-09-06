package codegen

import (
	"testing"
)

func TestReloc_Addr32(t *testing.T) {
	rm := NewRelocationManager()
	sym := &Symbol{Name: "target", Type: SymbolTypeFunction, Binding: SymbolBindingGlobal, Section: ".text", Value: 0x1234, Size: 4}
	rm.RegisterSymbol(sym)

	sec := &Section{Name: ".text", Type: SectionTypeText, Data: make([]byte, 8), Address: 0x1000, Align: 4, Size: 8}
	rm.AddRelocation(0, RelocTypeAddr32, "target", 0, ".text")

	if err := rm.ApplyRelocations(sec); err != nil {
		t.Fatalf("ApplyRelocations failed: %v", err)
	}
	// Little-endian 32-bit: 0x1234 → 0x34 0x12 0x00 0x00
	if sec.Data[0] != 0x34 || sec.Data[1] != 0x12 || sec.Data[2] != 0x00 || sec.Data[3] != 0x00 {
		t.Errorf("Addr32 data wrong: %v", sec.Data[:4])
	}
}

func TestReloc_Addr32WithAddend(t *testing.T) {
	rm := NewRelocationManager()
	rm.RegisterSymbol(&Symbol{Name: "data", Type: SymbolTypeObject, Binding: SymbolBindingGlobal, Section: ".data", Value: 0x2000, Size: 0})

	sec := &Section{Name: ".data", Type: SectionTypeData, Data: make([]byte, 4), Address: 0x3000, Align: 4, Size: 4}
	rm.AddRelocation(0, RelocTypeAddr32, "data", 0x10, ".data")

	if err := rm.ApplyRelocations(sec); err != nil {
		t.Fatalf("ApplyRelocations failed: %v", err)
	}
	// 0x2000 + 0x10 = 0x2010 → 0x10 0x20 0x00 0x00
	if sec.Data[0] != 0x10 || sec.Data[1] != 0x20 {
		t.Errorf("Addr32+addend data wrong: %v", sec.Data[:4])
	}
}

func TestReloc_Addr64(t *testing.T) {
	rm := NewRelocationManager()
	rm.RegisterSymbol(&Symbol{Name: "big", Type: SymbolTypeObject, Binding: SymbolBindingGlobal, Section: ".data", Value: 0x0123456789ABCDEF, Size: 0})

	sec := &Section{Name: ".data", Type: SectionTypeData, Data: make([]byte, 8), Address: 0x3000, Align: 8, Size: 8}
	rm.AddRelocation(0, RelocTypeAddr64, "big", 0, ".data")

	if err := rm.ApplyRelocations(sec); err != nil {
		t.Fatalf("ApplyRelocations failed: %v", err)
	}
	expected := []byte{0xEF, 0xCD, 0xAB, 0x89, 0x67, 0x45, 0x23, 0x01}
	for i, b := range expected {
		if sec.Data[i] != b {
			t.Errorf("Addr64 byte %d: expected 0x%02x, got 0x%02x", i, b, sec.Data[i])
		}
	}
}

func TestReloc_PCRel32(t *testing.T) {
	rm := NewRelocationManager()
	rm.RegisterSymbol(&Symbol{Name: "func", Type: SymbolTypeFunction, Binding: SymbolBindingGlobal, Section: ".text", Value: 0x1020, Size: 16})

	sec := &Section{Name: ".text", Type: SectionTypeText, Data: make([]byte, 8), Address: 0x1000, Align: 4, Size: 8}
	// Reloc at offset 4: next instruction addr = 0x1000 + 4 + 4 = 0x1008
	rm.AddRelocation(4, RelocTypePCRel32, "func", 0, ".text")

	if err := rm.ApplyRelocations(sec); err != nil {
		t.Fatalf("ApplyRelocations failed: %v", err)
	}
	// value = 0x1020 - 0x1008 = 0x18 (little-endian 32-bit at offset 4)
	if sec.Data[4] != 0x18 || sec.Data[5] != 0x00 || sec.Data[6] != 0x00 || sec.Data[7] != 0x00 {
		t.Errorf("PCRel32 data wrong at offset 4: %v", sec.Data[4:8])
	}
}

func TestReloc_PCRel32Negative(t *testing.T) {
	rm := NewRelocationManager()
	rm.RegisterSymbol(&Symbol{Name: "backward", Type: SymbolTypeFunction, Binding: SymbolBindingGlobal, Section: ".text", Value: 0x1000, Size: 16})

	sec := &Section{Name: ".text", Type: SectionTypeText, Data: make([]byte, 8), Address: 0x1000, Align: 4, Size: 8}
	// Reloc at offset 4: next instruction = 0x1008; backward = 0x1000; value = 0x1000 - 0x1008 = -8
	rm.AddRelocation(4, RelocTypePCRel32, "backward", 0, ".text")

	if err := rm.ApplyRelocations(sec); err != nil {
		t.Fatalf("ApplyRelocations failed: %v", err)
	}
	// -8 as uint32 = 0xFFFFFFF8: 0xF8 0xFF 0xFF 0xFF
	if sec.Data[4] != 0xF8 || sec.Data[5] != 0xFF || sec.Data[6] != 0xFF || sec.Data[7] != 0xFF {
		t.Errorf("PCRel32 negative data wrong: %v", sec.Data[4:8])
	}
}

func TestReloc_PCRel64(t *testing.T) {
	rm := NewRelocationManager()
	rm.RegisterSymbol(&Symbol{Name: "far", Type: SymbolTypeFunction, Binding: SymbolBindingGlobal, Section: ".text", Value: 0x1020, Size: 16})

	sec := &Section{Name: ".text", Type: SectionTypeText, Data: make([]byte, 8), Address: 0x1000, Align: 8, Size: 8}
	rm.AddRelocation(0, RelocTypePCRel64, "far", 0, ".text")

	if err := rm.ApplyRelocations(sec); err != nil {
		t.Fatalf("ApplyRelocations failed: %v", err)
	}
	// value = 0x1020 - (0x1000 + 0 + 8) = 0x18
	expected := []byte{0x18, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	for i, b := range expected {
		if sec.Data[i] != b {
			t.Errorf("PCRel64 byte %d: expected 0x%02x, got 0x%02x", i, b, sec.Data[i])
		}
	}
}

func TestReloc_BoundsExceed(t *testing.T) {
	rm := NewRelocationManager()
	rm.RegisterSymbol(&Symbol{Name: "x", Type: SymbolTypeObject, Binding: SymbolBindingGlobal, Section: ".data", Value: 0x1000, Size: 0})

	sec := &Section{Name: ".data", Type: SectionTypeData, Data: make([]byte, 2), Address: 0x1000, Align: 4, Size: 2}
	rm.AddRelocation(0, RelocTypeAddr32, "x", 0, ".data") // needs 4 bytes, only 2 available

	if err := rm.ApplyRelocations(sec); err == nil {
		t.Fatal("expected error for relocation exceeding section bounds")
	}
}

func TestReloc_UndefinedSymbol(t *testing.T) {
	rm := NewRelocationManager()
	// No symbols registered

	sec := &Section{Name: ".text", Type: SectionTypeText, Data: make([]byte, 4), Address: 0x1000, Align: 4, Size: 4}
	rm.AddRelocation(0, RelocTypeAddr32, "ghost", 0, ".text")

	if err := rm.ApplyRelocations(sec); err == nil {
		t.Fatal("expected error for undefined symbol")
	}
}

func TestReloc_ValidateFindsUndefined(t *testing.T) {
	rm := NewRelocationManager()
	rm.AddRelocation(0, RelocTypeAddr32, "missing", 0, ".text")

	if err := rm.ValidateRelocations(); err == nil {
		t.Fatal("expected error from ValidateRelocations for undefined symbol")
	}
}

func TestReloc_NameAndSize(t *testing.T) {
	tests := []struct {
		rt     RelocType
		name   string
		size   uint32
	}{
		{RelocTypeNone, "NONE", 0},
		{RelocTypeAddr32, "ADDR32", 4},
		{RelocTypeAddr64, "ADDR64", 8},
		{RelocTypePCRel32, "PCREL32", 4},
		{RelocTypePCRel64, "PCREL64", 8},
		{RelocTypePLT32, "PLT32", 4},
		{RelocTypeGOT32, "GOT32", 4},
		{RelocType(99), "UNKNOWN", 0},
	}
	for _, tt := range tests {
		if got := GetRelocationName(tt.rt); got != tt.name {
			t.Errorf("GetRelocationName(%d): expected %q, got %q", tt.rt, tt.name, got)
		}
		if got := CalculateRelocationSize(tt.rt); got != tt.size {
			t.Errorf("CalculateRelocationSize(%d): expected %d, got %d", tt.rt, tt.size, got)
		}
	}
}

func TestReloc_FunctionCall(t *testing.T) {
	rm := NewRelocationManager()
	rm.RegisterSymbol(&Symbol{Name: "karkain_user_helper", Type: SymbolTypeFunction, Binding: SymbolBindingGlobal, Section: ".text", Value: 0x1020, Size: 0})

	rm.RelocationForFunctionCall(8, "karkain_user_helper", ".text")

	sec := &Section{Name: ".text", Type: SectionTypeText, Data: make([]byte, 12), Address: 0x1000, Align: 4, Size: 12}
	if err := rm.ApplyRelocations(sec); err != nil {
		t.Fatalf("ApplyRelocations failed: %v", err)
	}
	// PCRel32 at offset 8: next = 0x1000 + 8 + 4 = 0x100C; sym = 0x1020; value = 0x1020 - 0x100C = 0x14
	if sec.Data[8] != 0x14 {
		t.Errorf("function call reloc wrong: got 0x%02x", sec.Data[8])
	}
}
