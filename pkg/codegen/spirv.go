package codegen

import (
	"fmt"
	"karkain/pkg/parser"
)

// SPIR-V constants
const (
	SPIRV_MAGIC     uint32 = 0x07230203
	SPIRV_VERSION   uint32 = 0x00010500 // SPIR-V 1.5
	SPIRV_GENERATOR uint32 = 0x00000000
	SPIRV_SCHEMA    uint32 = 0

	// Opcodes
	OpCapability     uint32 = 17
	OpMemoryModel    uint32 = 14
	OpEntryPoint     uint32 = 15
	OpName           uint32 = 5
	OpDecorate       uint32 = 32
	OpTypeVoid       uint32 = 19
	OpTypeFloat      uint32 = 22
	OpTypeInt        uint32 = 21
	OpTypePointer    uint32 = 32
	OpTypeRuntimeArray uint32 = 29
	OpTypeFunction   uint32 = 33
	OpVariable       uint32 = 59
	OpFunction       uint32 = 54
	OpFunctionEnd    uint32 = 56
	OpAccessChain    uint32 = 65
	OpLoad           uint32 = 61
	OpStore          uint32 = 62
	OpIAdd           uint32 = 128
	OpIMul           uint32 = 129
	OpSDiv           uint32 = 132

	// Storage classes
	StorageClassUniformConstant uint32 = 0
	StorageClassInput           uint32 = 1
	StorageClassUniform         uint32 = 2
	StorageClassStorageBuffer   uint32 = 3
	StorageClassFunction        uint32 = 7

	// Execution model
	ExecutionModelGLCompute uint32 = 5

	// Built-in decorations
	DecorationBuiltIn         uint32 = 11
	BuiltInGlobalInvocationId uint32 = 28
)

// SPIRVGenerator generates binary SPIR-V bytecode for Vulkan compute pipelines
type SPIRVGenerator struct {
	words    []uint32
	bound    uint32
	typeIDs  map[string]uint32
	errors   []string
}

// NewSPIRVGenerator creates a new SPIR-V generator
func NewSPIRVGenerator() *SPIRVGenerator {
	return &SPIRVGenerator{
		words:   make([]uint32, 0, 256),
		bound:   1,
		typeIDs: make(map[string]uint32),
	}
}

func (g *SPIRVGenerator) nextID() uint32 {
	id := g.bound
	g.bound++
	return id
}

func (g *SPIRVGenerator) emitWord(word uint32) {
	g.words = append(g.words, word)
}

func (g *SPIRVGenerator) emitInstruction(opcode uint32, operands ...uint32) {
	wordCount := uint32(len(operands) + 1)
	g.emitWord((wordCount << 16) | (opcode & 0xFFFF))
	for _, op := range operands {
		g.emitWord(op)
	}
}

// GenerateSPIRV emits binary SPIR-V bytecode for a kernel declaration
func (g *SPIRVGenerator) GenerateSPIRV(kernel *parser.KernelDeclStmt) ([]uint32, error) {
	g.words = g.words[:0]
	g.bound = 1
	g.typeIDs = make(map[string]uint32)
	g.errors = nil

	// Emit header with placeholder bound
	g.words = append(g.words, SPIRV_MAGIC, SPIRV_VERSION, SPIRV_GENERATOR, 0, SPIRV_SCHEMA)

	g.emitCapabilities()
	g.emitMemoryModel()
	g.emitEntryPoint(kernel)
	g.emitTypeDeclarations(kernel)
	g.emitFunction(kernel)

	// Update header with final bound
	g.words[3] = g.bound

	if len(g.errors) > 0 {
		return nil, fmt.Errorf("SPIR-V codegen errors: %v", g.errors)
	}

	return g.words, nil
}

func (g *SPIRVGenerator) emitCapabilities() {
	g.emitInstruction(OpCapability, 1) // Shader capability
}

func (g *SPIRVGenerator) emitMemoryModel() {
	g.emitInstruction(OpMemoryModel, 0, 1) // Logical, GLSL450
}

func (g *SPIRVGenerator) emitEntryPoint(kernel *parser.KernelDeclStmt) {
	mainID := g.nextID()
	g.emitInstruction(OpEntryPoint, ExecutionModelGLCompute, mainID)
	// Emit name string (packed as words)
	g.emitString(kernel.Name)
}

func (g *SPIRVGenerator) emitTypeDeclarations(kernel *parser.KernelDeclStmt) {
	// Void type
	voidID := g.nextID()
	g.typeIDs["void"] = voidID
	g.emitInstruction(OpTypeVoid, voidID)

	// Float type (f32)
	floatID := g.nextID()
	g.typeIDs["float"] = floatID
	g.emitInstruction(OpTypeFloat, floatID, 32)

	// Int type (i32)
	intID := g.nextID()
	g.typeIDs["int"] = intID
	g.emitInstruction(OpTypeInt, intID, 32, 1) // Signed 32-bit

	// Runtime array types for parameters
	for _, p := range kernel.Params {
		elemTypeID := g.getTypeID(p.Type)
		arrayTypeID := g.nextID()
		g.typeIDs["array_"+p.Type] = arrayTypeID
		g.emitInstruction(OpTypeRuntimeArray, arrayTypeID, elemTypeID)

		// Storage buffer pointer type
		ptrTypeID := g.nextID()
		g.typeIDs["ptr_"+p.Type] = ptrTypeID
		g.emitInstruction(OpTypePointer, ptrTypeID, StorageClassStorageBuffer, arrayTypeID)
	}
}

func (g *SPIRVGenerator) emitFunction(kernel *parser.KernelDeclStmt) {
	voidID := g.typeIDs["void"]

	// Function type
	funcTypeID := g.nextID()
	g.emitInstruction(OpTypeFunction, funcTypeID, voidID)

	// Function declaration
	mainID := g.nextID()
	g.emitInstruction(OpFunction, voidID, mainID, 0, funcTypeID) // 0 = None decoration

	// Function body (empty for now)
	g.emitInstruction(OpFunctionEnd)
}

func (g *SPIRVGenerator) getTypeID(kType string) uint32 {
	switch kType {
	case "float", "float64":
		return g.typeIDs["float"]
	case "int":
		return g.typeIDs["int"]
	default:
		return g.typeIDs["float"] // Default to float
	}
}

func (g *SPIRVGenerator) emitString(s string) {
	// Encode string as null-terminated UTF-8 packed into 32-bit words
	bytes := []byte(s)
	bytes = append(bytes, 0) // Null terminator

	// Pad to 4-byte boundary
	for len(bytes)%4 != 0 {
		bytes = append(bytes, 0)
	}

	// Pack bytes into words
	for i := 0; i < len(bytes); i += 4 {
		word := uint32(bytes[i])
		if i+1 < len(bytes) {
			word |= uint32(bytes[i+1]) << 8
		}
		if i+2 < len(bytes) {
			word |= uint32(bytes[i+2]) << 16
		}
		if i+3 < len(bytes) {
			word |= uint32(bytes[i+3]) << 24
		}
		g.emitWord(word)
	}
}

// GetBinary returns the raw bytes of the SPIR-V module
func (g *SPIRVGenerator) GetBinary(words []uint32) []byte {
	bytes := make([]byte, len(words)*4)
	for i, word := range words {
		bytes[i*4] = byte(word)
		bytes[i*4+1] = byte(word >> 8)
		bytes[i*4+2] = byte(word >> 16)
		bytes[i*4+3] = byte(word >> 24)
	}
	return bytes
}
