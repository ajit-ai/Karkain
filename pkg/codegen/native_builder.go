package codegen

import (
	"fmt"
	"karkain/pkg/diagnostics"
	"karkain/pkg/parser"
)

// NativeBuildResult holds the result of a native build operation
type NativeBuildResult struct {
	Object      *Object
	Executable  *Executable
	Diagnostics []diagnostics.Diagnostic
	DebugInfo   *DebugInfo
	SourceMap   *SourceAddressMap
}

// NativeBuilder orchestrates the Phase-84 native build pipeline:
// AST → Object → Linker → Executable
type NativeBuilder struct {
	generator *NativeGenerator
	linker    *Linker
	diags     *CodegenDiagnostics
}

// NewNativeBuilder creates a new native builder
func NewNativeBuilder() *NativeBuilder {
	return &NativeBuilder{
		generator: NewNativeGenerator(),
		linker:    NewLinker(),
		diags:     NewCodegenDiagnostics(""),
	}
}

// Build compiles a Karkain program through the native pipeline:
// 1. Generate Object from AST (sections, symbols, debug info)
// 2. Link the Object to produce an Executable
// 3. Return the complete build result
func (nb *NativeBuilder) Build(prog *parser.Program, sourceFile string) (*NativeBuildResult, error) {
	nb.diags = NewCodegenDiagnostics(sourceFile)

	// Step 1: Generate Object from AST
	obj, objDiags, err := nb.generator.GenerateObject(prog, sourceFile)
	if err != nil {
		nb.diags.AddError(1, 1, diagnostics.CodeCodegen,
			fmt.Sprintf("object generation failed: %v", err))
		return &NativeBuildResult{
			Object:      obj,
			Diagnostics: nb.diags.GetDiagnostics(),
		}, err
	}

	// Merge object generation diagnostics
	for _, d := range objDiags {
		nb.diags.AddError(d.Line, d.Column, diagnostics.Code(d.Code), d.Message)
	}

	// Step 2: Link the Object (create fresh linker for each build)
	linker := NewLinker()
	linker.SetEntryPoint("main")
	if err := linker.AddObject(obj); err != nil {
		nb.diags.AddError(1, 1, diagnostics.CodeCodegen,
			fmt.Sprintf("linker add object failed: %v", err))
		return &NativeBuildResult{
			Object:      obj,
			Diagnostics: nb.diags.GetDiagnostics(),
		}, err
	}

	exec, err := linker.Link()
	if err != nil {
		// Merge linker diagnostics
		for _, d := range linker.GetDiagnostics() {
			nb.diags.AddError(d.Line, d.Column, diagnostics.Code(d.Code), d.Message)
		}
		return &NativeBuildResult{
			Object:      obj,
			Diagnostics: nb.diags.GetDiagnostics(),
		}, err
	}

	// Attach debug info to executable
	exec.DebugInfo = obj.DebugInfo

	// Relocate debug info addresses to match linked section layout
	relocateDebugAddresses(obj.DebugInfo, exec.Sections)

	// Step 3: Build source-address map from debug info
	var sourceMap *SourceAddressMap
	if obj.DebugInfo != nil {
		sourceMap = NewSourceAddressMap()
		sourceMap.BuildFromDebugInfo(obj.DebugInfo)
	}

	return &NativeBuildResult{
		Object:      obj,
		Executable:  exec,
		Diagnostics: nb.diags.GetDiagnostics(),
		DebugInfo:   obj.DebugInfo,
		SourceMap:   sourceMap,
	}, nil
}

// relocateDebugAddresses offsets debug info addresses by the .text section
// base address so source-to-address mappings reflect actual linked addresses.
func relocateDebugAddresses(dbg *DebugInfo, sections []*Section) {
	if dbg == nil {
		return
	}
	var textBase uint64
	for _, sect := range sections {
		if sect.Type == SectionTypeText {
			textBase = sect.Address
			break
		}
	}
	if textBase == 0 {
		return
	}
	for _, fi := range dbg.FunctionInfo {
		fi.Address += textBase
	}
	for _, li := range dbg.LineInfo {
		li.Address += textBase
	}
	for _, vi := range dbg.Variables {
		vi.Address += textBase
	}
}

// BuildMultiObject links multiple objects into a single executable
func (nb *NativeBuilder) BuildMultiObject(objects []*Object, entryPoint string) (*NativeBuildResult, error) {
	nb.diags = NewCodegenDiagnostics("multi-object")

	linker := NewLinker()
	linker.SetEntryPoint(entryPoint)

	for _, obj := range objects {
		if err := linker.AddObject(obj); err != nil {
			nb.diags.AddError(1, 1, diagnostics.CodeCodegen,
				fmt.Sprintf("linker add object failed: %v", err))
			return &NativeBuildResult{
				Diagnostics: nb.diags.GetDiagnostics(),
			}, err
		}
	}

	exec, err := linker.Link()
	if err != nil {
		for _, d := range linker.GetDiagnostics() {
			nb.diags.AddError(d.Line, d.Column, diagnostics.Code(d.Code), d.Message)
		}
		return &NativeBuildResult{
			Diagnostics: nb.diags.GetDiagnostics(),
		}, err
	}

	// Build combined source-address map
	var sourceMap *SourceAddressMap
	if len(objects) > 0 && objects[0].DebugInfo != nil {
		sourceMap = NewSourceAddressMap()
		sourceMap.BuildFromDebugInfo(objects[0].DebugInfo)
		exec.DebugInfo = objects[0].DebugInfo
	}

	return &NativeBuildResult{
		Executable:  exec,
		Diagnostics: nb.diags.GetDiagnostics(),
		SourceMap:   sourceMap,
	}, nil
}

// GetDiagnostics returns all collected diagnostics
func (nb *NativeBuilder) GetDiagnostics() []diagnostics.Diagnostic {
	return nb.diags.GetDiagnostics()
}

// HasErrors returns true if any errors were collected
func (nb *NativeBuilder) HasErrors() bool {
	return nb.diags.HasErrors()
}

// NativeBuildConfig controls the native build process
type NativeBuildConfig struct {
	SourceFile string
	EntryPoint string // defaults to "main"
	Verbose    bool
}

// DefaultNativeBuildConfig returns a config with sensible defaults
func DefaultNativeBuildConfig(sourceFile string) NativeBuildConfig {
	return NativeBuildConfig{
		SourceFile: sourceFile,
		EntryPoint: "main",
	}
}
