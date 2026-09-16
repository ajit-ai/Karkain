package cli

import (
	"strings"
	"testing"
)

// TestPhase124_TargetCommand_Catalog asserts `karkain target` exposes the Phase
// 124 compute-target catalog (CPU, SIMD, WASM, GPU, NPU, quantum) alongside the
// existing host/target listing, and that the legacy substrings remain intact.
func TestPhase124_TargetCommand_Catalog(t *testing.T) {
	out := captureStdout(t, func() {
		if res := TargetCommand(); res.ExitCode != ExitSuccess {
			t.Errorf("target: got %d", res.ExitCode)
		}
	})
	for _, want := range []string{
		"native", "c23", "wasm32-wasi", "Default: native",
		"Compute targets (Phase 124 experimental):",
		"cpu", "simd", "wasm32-wasi",
		"gpu-experimental", "npu-experimental", "quantum-experimental",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("target output missing %q:\n%s", want, out)
		}
	}
}

// TestPhase124_TargetCommand_Detail asserts the capability view for a known
// compute target and the deterministic usage error for an unknown one.
func TestPhase124_TargetCommand_Detail(t *testing.T) {
	out := captureStdout(t, func() {
		if res := TargetCommand("gpu-experimental"); res.ExitCode != ExitSuccess {
			t.Errorf("target gpu-experimental: got %d", res.ExitCode)
		}
	})
	for _, want := range []string{
		"Compute target: gpu-experimental",
		"Family:         gpu",
		"Maturity:       experimental",
		"Memory model:   host-device",
		"Capabilities:   ",
		"KIR classes:    ",
		"Tensor ops:     ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("gpu detail missing %q:\n%s", want, out)
		}
	}

	out = captureStdout(t, func() {
		if res := TargetCommand("npu-experimental"); res.ExitCode != ExitSuccess {
			t.Errorf("target npu-experimental: got %d", res.ExitCode)
		}
	})
	for _, want := range []string{"Compute target: npu-experimental", "device-only"} {
		if !strings.Contains(out, want) {
			t.Errorf("npu detail missing %q:\n%s", want, out)
		}
	}

	out = captureStdout(t, func() {
		if res := TargetCommand("generative-ai-9000"); res.ExitCode != ExitUsage {
			t.Errorf("unknown compute target: got %d, want ExitUsage", res.ExitCode)
		}
	})
	if !strings.Contains(out, "unknown compute target") {
		t.Errorf("unknown target must report a friendly error:\n%s", out)
	}
}

// TestPhase124_TargetCLI_E2E drives the real binary: the catalog line and one
// capability detail through `karkain target <name>`.
func TestPhase124_TargetCLI_E2E(t *testing.T) {
	bin := buildKarkain(t)
	root := t.TempDir()

	out, err := runBin(t, bin, root, "target")
	if err != nil {
		t.Fatalf("karkain target failed: %v\n%s", err, out)
	}
	for _, want := range []string{"Compute targets (Phase 124 experimental):", "gpu-experimental", "quantum-experimental"} {
		if !strings.Contains(out, want) {
			t.Errorf("catalog missing %q:\n%s", want, out)
		}
	}

	out, err = runBin(t, bin, root, "target", "npu-experimental")
	if err != nil {
		t.Fatalf("karkain target npu-experimental failed: %v\n%s", err, out)
	}
	for _, want := range []string{"Compute target: npu-experimental", "device-only", "synchronous single-invocation inference"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail missing %q:\n%s", want, out)
		}
	}

	out, err = runBin(t, bin, root, "target", "quantum-experimental")
	if err != nil {
		t.Fatalf("karkain target quantum-experimental failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Maturity:       research") {
		t.Errorf("quantum maturity must be research:\n%s", out)
	}
}