package codegen

import (
	"fmt"
	"karkain/pkg/diagnostics"
)

// Linker transforms object files into final executables or linked artifacts
type Linker struct {
	objects    []*Object
	symbols    map[string]*Symbol
	entryPoint string
	diagnostics []diagnostics.Diagnostic
}

// NewLinker creates a new linker instance
func NewLinker() *Linker {
	return &Linker{
		objects:      make([]*Object, 0),
		symbols:      make(map[string]*Symbol),
		diagnostics: make([]diagnostics.Diagnostic, 0),
	}
}

// AddObject adds an object file to the linker
func (l *Linker) AddObject(obj *Object) error {
	// Validate symbol names don't collide with runtime symbols
	if err := ValidateSymbolNames(obj.Symbols); err != nil {
		l.addDiagnostic(diagnostics.ErrorDiagnostic(obj.Name, 1, 1, diagnostics.CodeCodegen, 
			fmt.Sprintf("symbol validation failed: %v", err)))
		return err
	}
	
	l.objects = append(l.objects, obj)
	
	// Collect symbols from the object
	for _, sym := range obj.Symbols {
		if existing, exists := l.symbols[sym.Name]; exists {
			// Handle symbol collision
			if existing.Binding == SymbolBindingGlobal && sym.Binding == SymbolBindingGlobal {
				l.addDiagnostic(diagnostics.ErrorDiagnostic(obj.Name, sym.SourceLine, 1, diagnostics.CodeCodegen,
					fmt.Sprintf("duplicate global symbol '%s'", sym.Name)))
				return fmt.Errorf("duplicate global symbol '%s'", sym.Name)
			}
		} else {
			l.symbols[sym.Name] = sym
		}
	}
	
	return nil
}

// SetEntryPoint sets the entry point symbol for the executable
func (l *Linker) SetEntryPoint(symbol string) {
	l.entryPoint = symbol
}

// Link performs the linking process
func (l *Linker) Link() (*Executable, error) {
	// 1. Collect all objects
	if len(l.objects) == 0 {
		return nil, fmt.Errorf("no objects to link")
	}
	
	// 2. Resolve symbols
	if err := l.resolveSymbols(); err != nil {
		return nil, err
	}
	
	// 3. Layout sections
	sections, err := l.layoutSections()
	if err != nil {
		return nil, err
	}
	
	// 4. Apply relocations
	if err := l.applyRelocations(sections); err != nil {
		return nil, err
	}
	
	// 5. Handle entry point
	entryAddr, err := l.resolveEntryPoint()
	if err != nil {
		return nil, err
	}
	
	// 6. Generate final executable
	executable := &Executable{
		Sections:    sections,
		EntryPoint:  entryAddr,
		Symbols:     l.symbols,
		EntryPointName: l.entryPoint,
	}
	
	return executable, nil
}

// resolveSymbols resolves all symbol references
func (l *Linker) resolveSymbols() error {
	// Check that entry point exists
	if l.entryPoint != "" {
		if _, exists := l.symbols[l.entryPoint]; !exists {
			l.addDiagnostic(diagnostics.ErrorDiagnostic("", 1, 1, diagnostics.CodeCodegen,
				fmt.Sprintf("entry point symbol '%s' not found", l.entryPoint)))
			return fmt.Errorf("entry point symbol '%s' not found", l.entryPoint)
		}
	}
	
	// Check for undefined symbols in relocations
	for _, obj := range l.objects {
		for _, reloc := range obj.Relocations {
			if _, exists := l.symbols[reloc.Symbol]; !exists {
				l.addDiagnostic(diagnostics.ErrorDiagnostic(obj.Name, 1, 1, diagnostics.CodeCodegen,
					fmt.Sprintf("undefined symbol '%s' referenced in relocation", reloc.Symbol)))
				return fmt.Errorf("undefined symbol '%s'", reloc.Symbol)
			}
		}
	}
	
	return nil
}

// layoutSections calculates section layout and addresses
func (l *Linker) layoutSections() ([]*Section, error) {
	var layout []*Section
	currentAddr := uint64(0x1000) // Start at page boundary
	
	// Collect unique sections from all objects
	sectionMap := make(map[string]*Section)
	
	for _, obj := range l.objects {
		for _, sect := range obj.Sections {
			if existing, exists := sectionMap[sect.Name]; exists {
				// Merge section data
				existing.Data = append(existing.Data, sect.Data...)
				existing.Size += uint64(len(sect.Data))
			} else {
				newSection := &Section{
					Name:  sect.Name,
					Type:  sect.Type,
					Data:  make([]byte, len(sect.Data)),
					Align: sect.Align,
					Size:  uint64(len(sect.Data)),
				}
				copy(newSection.Data, sect.Data)
				sectionMap[sect.Name] = newSection
			}
		}
	}
	
	// Layout sections with proper alignment
	for _, sectionType := range []SectionType{SectionTypeText, SectionTypeROData, SectionTypeData, SectionTypeBSS} {
		for _, sect := range sectionMap {
			if sect.Type == sectionType {
				// Align to section alignment
				if currentAddr%uint64(sect.Align) != 0 {
					currentAddr = ((currentAddr / uint64(sect.Align)) + 1) * uint64(sect.Align)
				}
				
				sect.Address = currentAddr
				sect.Size = uint64(len(sect.Data))
				currentAddr += sect.Size
				
				layout = append(layout, sect)
			}
		}
	}
	
	// Update symbol addresses based on section layout
	for _, sym := range l.symbols {
		if sym.Section != "" {
			if sect, exists := sectionMap[sym.Section]; exists {
				sym.Value = sect.Address + sym.Value
			}
		}
	}
	
	return layout, nil
}

// applyRelocations applies all relocations
func (l *Linker) applyRelocations(sections []*Section) error {
	// Build section map for lookup
	sectionMap := make(map[string]*Section)
	for _, sect := range sections {
		sectionMap[sect.Name] = sect
	}
	
	// Create relocation manager
	relocManager := NewRelocationManager()
	
	// Register all symbols
	for _, sym := range l.symbols {
		relocManager.RegisterSymbol(sym)
	}
	
	// Process relocations from all objects
	for _, obj := range l.objects {
		for _, reloc := range obj.Relocations {
			relocManager.AddRelocation(reloc.Offset, reloc.Type, reloc.Symbol, reloc.Addend, reloc.Section)
		}
	}
	
	// Validate relocations
	if err := relocManager.ValidateRelocations(); err != nil {
		return err
	}
	
	// Apply relocations to each section
	for _, sect := range sections {
		// Set section address in relocation manager
		sect.Address = sectionMap[sect.Name].Address
		
		if err := relocManager.ApplyRelocations(sect); err != nil {
			l.addDiagnostic(diagnostics.ErrorDiagnostic("", 1, 1, diagnostics.CodeCodegen,
				fmt.Sprintf("failed to apply relocations to section '%s': %v", sect.Name, err)))
			return err
		}
	}
	
	return nil
}

// resolveEntryPoint resolves the entry point address
func (l *Linker) resolveEntryPoint() (uint64, error) {
	if l.entryPoint == "" {
		return 0, nil // No entry point specified
	}
	
	sym, exists := l.symbols[l.entryPoint]
	if !exists {
		return 0, fmt.Errorf("entry point symbol '%s' not found", l.entryPoint)
	}
	
	return sym.Value, nil
}

// addDiagnostic adds a diagnostic using Phase-83 framework
func (l *Linker) addDiagnostic(diag diagnostics.Diagnostic) {
	l.diagnostics = append(l.diagnostics, diag)
}

// GetDiagnostics returns all diagnostics from the linking process
func (l *Linker) GetDiagnostics() []diagnostics.Diagnostic {
	return l.diagnostics
}

// Executable represents the final linked executable
type Executable struct {
	Sections      []*Section
	EntryPoint    uint64
	Symbols       map[string]*Symbol
	EntryPointName string
	DebugInfo     *DebugInfo
}

// GetSection returns a section by name
func (exec *Executable) GetSection(name string) *Section {
	for _, sect := range exec.Sections {
		if sect.Name == name {
			return sect
		}
	}
	return nil
}

// GetSymbol returns a symbol by name
func (exec *Executable) GetSymbol(name string) *Symbol {
	return exec.Symbols[name]
}

// Size returns the total size of the executable
func (exec *Executable) Size() uint64 {
	var total uint64
	for _, sect := range exec.Sections {
		total += sect.Size
	}
	return total
}

// Validate validates the executable structure
func (exec *Executable) Validate() error {
	// Check that entry point is within code section
	if exec.EntryPoint > 0 {
		textSection := exec.GetSection(".text")
		if textSection == nil {
			return fmt.Errorf("no .text section found but entry point specified")
		}
		if exec.EntryPoint < textSection.Address || exec.EntryPoint >= textSection.Address+textSection.Size {
			return fmt.Errorf("entry point 0x%x outside .text section bounds", exec.EntryPoint)
		}
	}
	
	// Check that all symbols are within their sections
	for _, sym := range exec.Symbols {
		if sym.Section != "" {
			sect := exec.GetSection(sym.Section)
			if sect == nil {
				return fmt.Errorf("symbol '%s' references non-existent section '%s'", sym.Name, sym.Section)
			}
			if sym.Value < sect.Address || sym.Value >= sect.Address+sect.Size {
				return fmt.Errorf("symbol '%s' value 0x%x outside section '%s' bounds", 
					sym.Name, sym.Value, sym.Section)
			}
		}
	}
	
	return nil
}

// MinimalLinker provides a simplified linking interface for basic use cases
type MinimalLinker struct {
	*Linker
}

// NewMinimalLinker creates a minimal linker with default settings
func NewMinimalLinker() *MinimalLinker {
	return &MinimalLinker{
		Linker: NewLinker(),
	}
}

// LinkSingleObject links a single object file into an executable
func (ml *MinimalLinker) LinkSingleObject(obj *Object) (*Executable, error) {
	ml.SetEntryPoint("main")
	
	if err := ml.AddObject(obj); err != nil {
		return nil, err
	}
	
	return ml.Link()
}

// LinkMultipleObjects links multiple object files into an executable
func (ml *MinimalLinker) LinkMultipleObjects(objects []*Object) (*Executable, error) {
	ml.SetEntryPoint("main")
	
	for _, obj := range objects {
		if err := ml.AddObject(obj); err != nil {
			return nil, err
		}
	}
	
	return ml.Link()
}
