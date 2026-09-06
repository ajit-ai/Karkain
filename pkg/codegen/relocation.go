package codegen

import (
	"fmt"
)

// RelocationManager manages relocations during code generation
type RelocationManager struct {
	relocations []*Relocation
	symbols     map[string]*Symbol
}

// NewRelocationManager creates a new relocation manager
func NewRelocationManager() *RelocationManager {
	return &RelocationManager{
		relocations: make([]*Relocation, 0),
		symbols:     make(map[string]*Symbol),
	}
}

// AddRelocation adds a relocation entry
func (rm *RelocationManager) AddRelocation(offset uint64, relocType RelocType, symbol string, addend int64, section string) {
	reloc := &Relocation{
		Offset:  offset,
		Type:    relocType,
		Symbol:  symbol,
		Addend:  addend,
		Section: section,
	}
	rm.relocations = append(rm.relocations, reloc)
}

// RegisterSymbol registers a symbol for relocation resolution
func (rm *RelocationManager) RegisterSymbol(symbol *Symbol) {
	rm.symbols[symbol.Name] = symbol
}

// ResolveSymbol resolves a symbol to its final address
func (rm *RelocationManager) ResolveSymbol(name string) (uint64, error) {
	sym, exists := rm.symbols[name]
	if !exists {
		return 0, fmt.Errorf("undefined symbol '%s' for relocation", name)
	}
	return sym.Value, nil
}

// ApplyRelocations applies relocations to section data
func (rm *RelocationManager) ApplyRelocations(section *Section) error {
	data := section.Data
	
	for _, reloc := range rm.relocations {
		if reloc.Section != section.Name {
			continue
		}
		
		symbolAddr, err := rm.ResolveSymbol(reloc.Symbol)
		if err != nil {
			return fmt.Errorf("failed to resolve symbol '%s' at offset 0x%x: %w", 
				reloc.Symbol, reloc.Offset, err)
		}
		
		// Apply relocation based on type
		switch reloc.Type {
		case RelocTypeAddr32:
			if reloc.Offset+4 > uint64(len(data)) {
				return fmt.Errorf("relocation offset 0x%x exceeds section size", reloc.Offset)
			}
			// Write 32-bit absolute address
			value := uint32(symbolAddr + uint64(reloc.Addend))
			data[reloc.Offset] = byte(value)
			data[reloc.Offset+1] = byte(value >> 8)
			data[reloc.Offset+2] = byte(value >> 16)
			data[reloc.Offset+3] = byte(value >> 24)
			
		case RelocTypeAddr64:
			if reloc.Offset+8 > uint64(len(data)) {
				return fmt.Errorf("relocation offset 0x%x exceeds section size", reloc.Offset)
			}
			// Write 64-bit absolute address
			value := symbolAddr + uint64(reloc.Addend)
			data[reloc.Offset] = byte(value)
			data[reloc.Offset+1] = byte(value >> 8)
			data[reloc.Offset+2] = byte(value >> 16)
			data[reloc.Offset+3] = byte(value >> 24)
			data[reloc.Offset+4] = byte(value >> 32)
			data[reloc.Offset+5] = byte(value >> 40)
			data[reloc.Offset+6] = byte(value >> 48)
			data[reloc.Offset+7] = byte(value >> 56)
			
		case RelocTypePCRel32:
			if reloc.Offset+4 > uint64(len(data)) {
				return fmt.Errorf("relocation offset 0x%x exceeds section size", reloc.Offset)
			}
			// Calculate PC-relative offset (next instruction address - symbol address)
			nextInstrAddr := section.Address + reloc.Offset + 4
			value := int64(symbolAddr) - int64(nextInstrAddr) + reloc.Addend
			if value < -2147483648 || value > 2147483647 {
				return fmt.Errorf("PC-relative relocation overflow for symbol '%s'", reloc.Symbol)
			}
			// Write 32-bit PC-relative offset
			value32 := uint32(value)
			data[reloc.Offset] = byte(value32)
			data[reloc.Offset+1] = byte(value32 >> 8)
			data[reloc.Offset+2] = byte(value32 >> 16)
			data[reloc.Offset+3] = byte(value32 >> 24)
			
		case RelocTypePCRel64:
			if reloc.Offset+8 > uint64(len(data)) {
				return fmt.Errorf("relocation offset 0x%x exceeds section size", reloc.Offset)
			}
			// Calculate PC-relative offset
			nextInstrAddr := section.Address + reloc.Offset + 8
			value := int64(symbolAddr) - int64(nextInstrAddr) + reloc.Addend
			// Write 64-bit PC-relative offset
			data[reloc.Offset] = byte(value)
			data[reloc.Offset+1] = byte(value >> 8)
			data[reloc.Offset+2] = byte(value >> 16)
			data[reloc.Offset+3] = byte(value >> 24)
			data[reloc.Offset+4] = byte(value >> 32)
			data[reloc.Offset+5] = byte(value >> 40)
			data[reloc.Offset+6] = byte(value >> 48)
			data[reloc.Offset+7] = byte(value >> 56)
			
		case RelocTypePLT32:
			// PLT entry - similar to PC-relative but for PLT entries
			if reloc.Offset+4 > uint64(len(data)) {
				return fmt.Errorf("relocation offset 0x%x exceeds section size", reloc.Offset)
			}
			// For now, treat as PC-relative (PLT resolution happens at link time)
			nextInstrAddr := section.Address + reloc.Offset + 4
			value := int64(symbolAddr) - int64(nextInstrAddr) + reloc.Addend
			value32 := uint32(value)
			data[reloc.Offset] = byte(value32)
			data[reloc.Offset+1] = byte(value32 >> 8)
			data[reloc.Offset+2] = byte(value32 >> 16)
			data[reloc.Offset+3] = byte(value32 >> 24)
			
		case RelocTypeGOT32:
			// GOT entry - offset to GOT entry for symbol
			if reloc.Offset+4 > uint64(len(data)) {
				return fmt.Errorf("relocation offset 0x%x exceeds section size", reloc.Offset)
			}
			// For now, treat as absolute (GOT resolution happens at link time)
			value := uint32(symbolAddr + uint64(reloc.Addend))
			data[reloc.Offset] = byte(value)
			data[reloc.Offset+1] = byte(value >> 8)
			data[reloc.Offset+2] = byte(value >> 16)
			data[reloc.Offset+3] = byte(value >> 24)
			
		default:
			return fmt.Errorf("unsupported relocation type %d for symbol '%s'", reloc.Type, reloc.Symbol)
		}
	}
	
	section.Data = data
	return nil
}

// GetRelocations returns all relocations
func (rm *RelocationManager) GetRelocations() []*Relocation {
	return rm.relocations
}

// GetRelocationsForSection returns relocations for a specific section
func (rm *RelocationManager) GetRelocationsForSection(sectionName string) []*Relocation {
	var sectionRelocs []*Relocation
	for _, reloc := range rm.relocations {
		if reloc.Section == sectionName {
			sectionRelocs = append(sectionRelocs, reloc)
		}
	}
	return sectionRelocs
}

// RelocationForFunctionCall creates a relocation for a function call
func (rm *RelocationManager) RelocationForFunctionCall(offset uint64, functionName string, section string) {
	rm.AddRelocation(offset, RelocTypePCRel32, functionName, 0, section)
}

// RelocationForGlobalVariable creates a relocation for a global variable access
func (rm *RelocationManager) RelocationForGlobalVariable(offset uint64, varName string, section string) {
	rm.AddRelocation(offset, RelocTypeAddr64, varName, 0, section)
}

// RelocationForStringLiteral creates a relocation for a string literal
func (rm *RelocationManager) RelocationForStringLiteral(offset uint64, stringLabel string, section string) {
	rm.AddRelocation(offset, RelocTypeAddr64, stringLabel, 0, section)
}

// ValidateRelocations checks for relocation errors
func (rm *RelocationManager) ValidateRelocations() error {
	// Check for undefined symbols
	for _, reloc := range rm.relocations {
		if _, exists := rm.symbols[reloc.Symbol]; !exists {
			return fmt.Errorf("relocation references undefined symbol '%s'", reloc.Symbol)
		}
	}
	return nil
}

// CalculateRelocationSize calculates the size of a relocation entry
func CalculateRelocationSize(relocType RelocType) uint32 {
	switch relocType {
	case RelocTypeAddr32, RelocTypePCRel32, RelocTypePLT32, RelocTypeGOT32:
		return 4
	case RelocTypeAddr64, RelocTypePCRel64:
		return 8
	default:
		return 0
	}
}

// GetRelocationName returns the string name of a relocation type
func GetRelocationName(relocType RelocType) string {
	switch relocType {
	case RelocTypeNone:
		return "NONE"
	case RelocTypeAddr32:
		return "ADDR32"
	case RelocTypeAddr64:
		return "ADDR64"
	case RelocTypePCRel32:
		return "PCREL32"
	case RelocTypePCRel64:
		return "PCREL64"
	case RelocTypePLT32:
		return "PLT32"
	case RelocTypeGOT32:
		return "GOT32"
	default:
		return "UNKNOWN"
	}
}
