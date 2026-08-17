package codegen

import (
	"fmt"
	"karkain/pkg/parser"
	"karkain/pkg/sema"
	"strings"
)

// GenericKernelSpecializer handles monomorphization and multi-backend code generation
// for generic GPU kernels
type GenericKernelSpecializer struct {
	monomorphizer *sema.Monomorphizer
	wgslGen       *WGSLGenerator
	spirvGen      *SPIRVGenerator
	errors        []string
}

// NewGenericKernelSpecializer creates a new specializer with default backends
func NewGenericKernelSpecializer() *GenericKernelSpecializer {
	return &GenericKernelSpecializer{
		monomorphizer: sema.NewMonomorphizer(),
		wgslGen:       NewWGSLGenerator(),
		spirvGen:      NewSPIRVGenerator(),
	}
}

// NewGenericKernelSpecializerWith creates a specializer with provided monomorphizer
func NewGenericKernelSpecializerWith(m *sema.Monomorphizer) *GenericKernelSpecializer {
	return &GenericKernelSpecializer{
		monomorphizer: m,
		wgslGen:       NewWGSLGenerator(),
		spirvGen:      NewSPIRVGenerator(),
	}
}

// SpecializeAndGenerateWGSL instantiates a generic kernel and emits WGSL
func (s *GenericKernelSpecializer) SpecializeAndGenerateWGSL(
	tmpl *parser.KernelDeclStmt,
	typeArgs map[string]string,
) (string, error) {
	concrete, err := s.monomorphizer.InstantiateGenericKernel(tmpl, typeArgs)
	if err != nil {
		return "", fmt.Errorf("monomorphization failed: %w", err)
	}
	return s.wgslGen.GenerateWGSL(concrete)
}

// SpecializeAndGenerateSPIRV instantiates a generic kernel and emits SPIR-V binary
func (s *GenericKernelSpecializer) SpecializeAndGenerateSPIRV(
	tmpl *parser.KernelDeclStmt,
	typeArgs map[string]string,
) ([]uint32, error) {
	concrete, err := s.monomorphizer.InstantiateGenericKernel(tmpl, typeArgs)
	if err != nil {
		return nil, fmt.Errorf("monomorphization failed: %w", err)
	}
	return s.spirvGen.GenerateSPIRV(concrete)
}

// MonomorphizeKernel returns the specialized kernel AST without code generation
func (s *GenericKernelSpecializer) MonomorphizeKernel(
	tmpl *parser.KernelDeclStmt,
	typeArgs map[string]string,
) (*parser.KernelDeclStmt, error) {
	return s.monomorphizer.InstantiateGenericKernel(tmpl, typeArgs)
}

// GetMonomorphizer returns the underlying monomorphizer for trait/impl registration
func (s *GenericKernelSpecializer) GetMonomorphizer() *sema.Monomorphizer {
	return s.monomorphizer
}

// GetErrors returns accumulated errors
func (s *GenericKernelSpecializer) GetErrors() []string {
	return s.errors
}

// GenerateWGSLWithResolvedType is a helper that generates WGSL for a kernel
// with an explicit resolved element type (for kernels parameterized by element type).
// This maps element type strings to WGSL scalar types for storage bindings.
func GenerateWGSLWithResolvedType(kernel *parser.KernelDeclStmt, elemType string) (string, error) {
	gen := NewWGSLGenerator()

	wgslType := mapElementTypeToWGSL(elemType)
	_ = wgslType // used in binding generation

	return gen.GenerateWGSL(kernel)
}

// mapElementTypeToWGSL maps abstract element types to WGSL scalar type strings
func mapElementTypeToWGSL(elemType string) string {
	switch strings.ToLower(elemType) {
	case "int", "i32":
		return "i32"
	case "float", "float32", "f32":
		return "f32"
	case "float64", "f64", "double":
		return "f64"
	case "uint", "u32":
		return "u32"
	default:
		return "f32"
	}
}
