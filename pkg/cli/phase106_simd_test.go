package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/codegen"
)

// Phase 106 gate: SIMD & Vector Types end-to-end. The [N]f32-style variable
// annotations select lane-vector types and @simd_* ops perform real elementwise
// vector arithmetic; the gate proves the executable output matches a scalar
// oracle and that the generated assembly actually contains the AVX/AVX2 vector
// instructions.

const phase106ExampleDir = "phase106"

func TestPhase106_SimdE2E_RunsClean(t *testing.T) {
	skipIfNoCompiler(t)

	dir := t.TempDir()
	copyTree(t, filepath.Join("..", "..", "examples", phase106ExampleDir), dir)
	exePath := filepath.Join(dir, "main_106.exe")

	res := BuildCommandIncremental(filepath.Join(dir, "main.kark"), exePath, codegen.Config{}, false, filepath.Join(dir, ".karkain-cache"))
	if res.ExitCode != ExitSuccess {
		t.Fatalf("SIMD build failed: %q (exit %d)", res.Message, res.ExitCode)
	}
	if _, err := os.Stat(exePath); err != nil {
		t.Fatalf("expected executable at %s: %v", exePath, err)
	}

	// The generated C must contain lane-typed declarations and vector ops.
	c, err := os.ReadFile(filepath.Join(dir, "main.c"))
	if err != nil {
		t.Fatalf("expected generated main.c: %v", err)
	}
	for _, want := range []string{
		"karkain_f32x8 a = karkain_simd_splat_f32x8(((float)(2.0)))",
		"karkain_simd_add_f32x8(a, b)",
		"make_float(karkain_simd_sum_f32x8(sum))",
		"karkain_f64x4",
		"karkain_i32x8",
	} {
		if !strings.Contains(string(c), want) {
			t.Errorf("generated C missing %q", want)
		}
	}

	out := runExe(t, exePath)
	want := "40\n-8\n48\n12\n5\n56\n40\n"
	if out != want {
		t.Fatalf("SIMD executable output = %q, want %q", out, want)
	}
}

// TestPhase106_SimdScalarOracleDifferential builds the same lane arithmetic both
// as SIMD vectors and as a plain scalar loop and requires byte-identical output.
func TestPhase106_SimdScalarOracleDifferential(t *testing.T) {
	skipIfNoCompiler(t)

	src := `func loopOracle(sa, sb) {
    let ss = 0.0
    let sm = 0.0
    let sd = 0.0
    let i = 0
    while(i < 8) {
        ss = ss + (sa + sb)
        sm = sm + (sa * sb)
        sd = sd + (sb / sa)
        i = i + 1
    }
    print(ss)
    print(sm)
    print(sd)
}
func main() {
    let a [8]f32 = @simd_splat(2.0, 8)
    let b [8]f32 = @simd_splat(1.0, 8)
    let s [8]f32 = @simd_add(a, b)
    let m [8]f32 = @simd_mul(a, b)
    let d [8]f32 = @simd_div(b, a)
    print(@simd_sum(s))
    print(@simd_sum(m))
    print(@simd_sum(d))
    loopOracle(2.0, 1.0)
}`
	dir := t.TempDir()
	kark := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(kark, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	exePath := filepath.Join(dir, "main_106.exe")
	res := BuildCommandIncremental(kark, exePath, codegen.Config{}, false, filepath.Join(dir, ".karkain-cache"))
	if res.ExitCode != ExitSuccess {
		t.Fatalf("differential build failed: %q (exit %d)", res.Message, res.ExitCode)
	}
	out := runExe(t, exePath)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 output lines, got %d: %q", len(lines), out)
	}
	// The first three lines must be identical to the scalar-oracle lines.
	for i := 0; i < 3; i++ {
		if lines[i] != lines[i+3] {
			t.Errorf("SIMD line %d (%s) != scalar line %d (%s)", i+1, lines[i], i+4, lines[i+3])
		}
	}
}

// TestPhase106_AvxInstructionEmissionProbe is the roadmap assembly gate: an
// 8-wide f32 lane add must lower to the AVX/AVX2 vector instruction family
// (vaddps). The probe compiles the generated C under the pipeline's own flags
// (-O0 and the auto-appended -mavx, which is what x86 SIMD programs are built
// with) and inspects the assembly. It skips gracefully when the host C compiler
// cannot target AVX (e.g. non-x86 hosts) — the correctness gates above still run.
func TestPhase106_AvxInstructionEmissionProbe(t *testing.T) {
	skipIfNoCompiler(t)
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("probe needs gcc; skipping")
	}

	dir := t.TempDir()
	copyTree(t, filepath.Join("..", "..", "examples", phase106ExampleDir), dir)
	exe := filepath.Join(dir, "main_106.exe")
	res := BuildCommandIncremental(filepath.Join(dir, "main.kark"), exe, codegen.Config{}, false, filepath.Join(dir, ".karkain-cache"))
	if res.ExitCode != ExitSuccess {
		t.Fatalf("SIMD build failed: %q", res.Message)
	}
	cFile := filepath.Join(dir, "main.c")
	if _, err := os.Stat(cFile); err != nil {
		t.Fatalf("generated main.c missing: %v", err)
	}
	asmPath := filepath.Join(dir, "main.s")
	cc := exec.Command("gcc", "-S", "-O0", "-mavx", cFile, "-o", asmPath)
	if out, cerr := cc.CombinedOutput(); cerr != nil {
		t.Skipf("gcc -S -O0 -mavx failed (%v): %s", cerr, out)
	}
	asm, err := os.ReadFile(asmPath)
	if err != nil {
		t.Fatalf("read assembly: %v", err)
	}
	found := false
	for _, op := range []string{"vaddps", "vsubps", "vmulps", "vdivps"} {
		if strings.Contains(string(asm), op) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no AVX/AVX2 vector arithmetic instruction (vaddps family) in assembly; lane add did not vectorize")
	}
}