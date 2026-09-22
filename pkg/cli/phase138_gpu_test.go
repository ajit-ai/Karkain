package cli

import (
	"strings"
	"testing"

	"karkain/pkg/codegen"
)

// Phase 138 — Accelerator Kernel Surface v1 (GPU/NPU)
// Tests that @target(gpu) functions generate valid WGSL compute shaders
// with compile-only guarantee (no hardware/SDK required).

func TestPhase138_GPUGWGSLGeneration(t *testing.T) {
	// Test that @target(gpu) functions generate valid WGSL shaders
	source := `
@target(gpu)
func vector_add(a float, b float) float {
	return a + b
}
`
	cfg := codegen.Config{CompileOnly: true, Verbose: false}
	prog, diags := parseSourceWithErrors("", source, nil, false)
	if len(diags) > 0 {
		t.Fatalf("failed to parse test source: %v", diags)
	}

	gen := codegen.New(cfg)
	if err := gen.GenerateAndCompile(prog, "test.kark"); err != nil {
		t.Fatalf("GPU codegen failed: %v", err)
	}

	// Check that GPU generator has the function
	if gen.HasGPUFunction("vector_add") {
		shader := gen.GetGPUFunction("vector_add")
		if shader == "" {
			t.Errorf("GPU function shader is empty")
		}
		if !strings.Contains(shader, "@compute") {
			t.Errorf("WGSL shader missing @compute attribute")
		}
		if !strings.Contains(shader, "fn vector_add") {
			t.Errorf("WGSL shader missing function declaration")
		}
	} else {
		t.Errorf("GPU function not found in generator")
	}
}

func TestPhase138_GPUGTargetValidation(t *testing.T) {
	// Test that invalid @target(...) names are rejected
	source := `
@target(gpu_invalid)
func test() int {
	return 42
}
`
	cfg := codegen.Config{CompileOnly: true}
	prog, diags := parseSourceWithErrors("", source, nil, false)
	if len(diags) > 0 {
		t.Fatalf("failed to parse test source: %v", diags)
	}

	gen := codegen.New(cfg)
	if err := gen.GenerateAndCompile(prog, "test.kark"); err == nil {
		t.Errorf("Expected error for invalid target, got nil")
	}
}

func TestPhase138_GPUGCompileOnly(t *testing.T) {
	// Test that GPU codegen works in compile-only mode without hardware
	source := `
@target(gpu)
func matmul_kernel() float {
	return 1.0
}
`
	cfg := codegen.Config{CompileOnly: true}
	prog, diags := parseSourceWithErrors("", source, nil, false)
	if len(diags) > 0 {
		t.Fatalf("failed to parse test source: %v", diags)
	}

	gen := codegen.New(cfg)
	// Should succeed in compile-only mode even without GPU hardware
	if err := gen.GenerateAndCompile(prog, "test.kark"); err != nil {
		t.Errorf("Compile-only GPU codegen failed: %v", err)
	}
}

func TestPhase138_GPUGMultipleFunctions(t *testing.T) {
	// Test that multiple @target(gpu) functions are handled correctly
	source := `
@target(gpu)
func kernel_a() float {
	return 1.0
}

@target(gpu)
func kernel_b() float {
	return 2.0
}
`
	cfg := codegen.Config{CompileOnly: true}
	prog, diags := parseSourceWithErrors("", source, nil, false)
	if len(diags) > 0 {
		t.Fatalf("failed to parse test source: %v", diags)
	}

	gen := codegen.New(cfg)
	if err := gen.GenerateAndCompile(prog, "test.kark"); err != nil {
		t.Errorf("Multi-function GPU codegen failed: %v", err)
	}

	// Check that both functions were processed
	if !gen.HasGPUFunction("kernel_a") {
		t.Errorf("GPU function kernel_a not found")
	}
	if !gen.HasGPUFunction("kernel_b") {
		t.Errorf("GPU function kernel_b not found")
	}
}

func TestPhase138_GPUGMixedTargets(t *testing.T) {
	// Test that @target(gpu) and regular (CPU) functions coexist
	source := `
@target(gpu)
func gpu_kernel() float {
	return 1.0
}

func cpu_function() int {
	return 42
}
`
	cfg := codegen.Config{CompileOnly: true}
	prog, diags := parseSourceWithErrors("", source, nil, false)
	if len(diags) > 0 {
		t.Fatalf("failed to parse test source: %v", diags)
	}

	gen := codegen.New(cfg)
	if err := gen.GenerateAndCompile(prog, "test.kark"); err != nil {
		t.Errorf("Mixed target codegen failed: %v", err)
	}

	// GPU function should be in GPU generator
	if !gen.HasGPUFunction("gpu_kernel") {
		t.Errorf("GPU function not found in generator")
	}
}
