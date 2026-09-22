package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/target"
)

// Phase 139 — Cross-Compilation Expansion: aarch64-windows, riscv64-linux and
// macOS triples. Parse/canonicalization is pinned in pkg/target; this gate
// proves the CLI search/error paths: a missing cross-linker is a
// deterministic ToolchainError (exit 6, searched-listing, no artifact), a
// present one yields a real artifact — never a silent host fallback — and
// cross-run stays refused with the build-only hint.

func TestPhase139_NewTriplesNormalize(t *testing.T) {
	cases := map[string]string{
		"aarch64-windows":           "aarch64-windows",
		"riscv64-linux":             "riscv64-linux",
		"riscv64-unknown-linux-gnu": "riscv64-linux",
		"x86_64-macos":              "x86_64-macos",
		"x86_64-apple-macosx":       "x86_64-macos",
		"x86_64-darwin":             "x86_64-macos",
		"aarch64-macos":             "aarch64-macos",
		"aarch64-apple-macosx":      "aarch64-macos",
	}
	for in, want := range cases {
		got, err := NormalizeTarget(in)
		if err != nil {
			t.Errorf("NormalizeTarget(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeTarget(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPhase139_InvalidCombosRejected(t *testing.T) {
	bin := buildKarkain(t)
	hello := copyExampleToTemp(t, "hello")
	for triple, frag := range map[string]string{
		"riscv64-windows":  "only modeled for Linux",
		"x86_64-macos-gnu": "macOS builds use clang",
		"wasm32-macos":     "x86_64 and aarch64",
		"s390x-linux":      "unsupported architecture",
	} {
		out, err := runBin(t, bin, t.TempDir(), "build", hello,
			"--engine", "go", "--target", triple, "-o", filepath.Join(t.TempDir(), "out"))
		if err == nil {
			t.Errorf("build --target %s unexpectedly succeeded:\n%s", triple, out)
			continue
		}
		if code := exitCodeOf(t, err); code != ExitUsage {
			t.Errorf("build --target %s exit = %d, want usage %d\n%s", triple, code, ExitUsage, out)
		}
		if !strings.Contains(out, "unsupported target '"+triple+"'") || !strings.Contains(out, frag) {
			t.Errorf("build --target %s lacks usage diagnostic (%q):\n%s", triple, frag, out)
		}
	}
}

func TestPhase139_CrossBuildMissingToolchain(t *testing.T) {
	bin := buildKarkain(t)
	hello := copyExampleToTemp(t, "hello")
	t.Setenv("CC", "")

	for triple, searched := range map[string][]string{
		"aarch64-windows": {"aarch64-w64-mingw32-gcc"},
		"riscv64-linux":   {"riscv64-linux-gnu-gcc"},
		"x86_64-macos":    {"clang --target=x86_64-apple-macosx"},
		"aarch64-macos":   {"clang --target=aarch64-apple-macosx"},
	} {
		outPath := filepath.Join(t.TempDir(), "out")
		out, err := runBin(t, bin, t.TempDir(), "build", hello,
			"--engine", "go", "--target", triple, "-o", outPath)
		if err == nil {
			// A real cross-linker is present on this host: success is
			// correct, and the artifact must exist (no silent fallback
			// can produce a missing artifact).
			if _, serr := os.Stat(outPath); serr != nil {
				t.Errorf("build --target %s succeeded but no artifact at %s", triple, outPath)
			}
			continue
		}
		if code := exitCodeOf(t, err); code != ExitEnv {
			t.Errorf("build --target %s exit = %d, want %d\n%s", triple, code, ExitEnv, out)
		}
		for _, frag := range append([]string{
			"no cross-linker available for target '" + triple + "'",
			"on host '" + target.Host().String() + "'",
			"host toolchain can neither link nor execute",
		}, searched...) {
			if !strings.Contains(out, frag) {
				t.Errorf("build --target %s output lacks %q:\n%s", triple, frag, out)
			}
		}
		if _, serr := os.Stat(outPath); !os.IsNotExist(serr) {
			t.Errorf("build --target %s must not create an artifact on failure (%s present)", triple, outPath)
		}
	}
}

func TestPhase139_CrossRunForeignRefused(t *testing.T) {
	bin := buildKarkain(t)
	hello := copyExampleToTemp(t, "hello")

	// None of the new triples is the host machine on any supported host
	// (riscv64/macos hosts are not modeled), so run is always refused.
	out, err := runBin(t, bin, t.TempDir(), "run", hello,
		"--engine", "go", "--target", "riscv64-linux")
	if err == nil {
		t.Fatalf("run --target riscv64-linux unexpectedly succeeded:\n%s", out)
	}
	if code := exitCodeOf(t, err); code != ExitEnv {
		t.Errorf("foreign run exit = %d, want %d\n%s", code, ExitEnv, out)
	}
	if !strings.Contains(out, "cannot run a riscv64-linux binary on the host") ||
		!strings.Contains(out, "cross-run") {
		t.Errorf("foreign run diagnostic missing expected text:\n%s", out)
	}
}

func TestPhase139_TargetListsNewTriples(t *testing.T) {
	bin := buildKarkain(t)
	out, err := runBin(t, bin, t.TempDir(), "target")
	if err != nil {
		t.Fatalf("karkain target: %v\n%s", err, out)
	}
	for _, frag := range []string{
		"aarch64-windows", "riscv64-linux", "x86_64-macos", "aarch64-macos",
		"riscv64", "Mach-O",
	} {
		if !strings.Contains(out, frag) {
			t.Errorf("karkain target output lacks %q:\n%s", frag, out)
		}
	}
}

// --- WASM GC types (structs) end-to-end -------------------------------------

func TestPhase139_WasmStructE2E(t *testing.T) {
	wt := findWasmtime()
	if wt == "" {
		t.Skip("wasmtime not found")
	}
	bin := buildKarkain(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "structs.kark")
	prog := "type Point struct { x int; y int }\n" +
		"func main() {\n" +
		"    let p = Point { y: 20, x: 10 }\n" +
		"    print(p.x)\n" +
		"    print(p.y)\n" +
		"    p.y = 30\n" +
		"    print(p.x + p.y)\n" +
		"}\n"
	if err := os.WriteFile(src, []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}
	outWasm := filepath.Join(dir, "structs.wasm")
	out, err := runBin(t, bin, dir, "build", "--engine", "go",
		"--target", "wasm32-wasi", "-o", outWasm, src)
	if err != nil {
		t.Fatalf("wasm build of struct program failed: %v\n%s", err, out)
	}
	run := exec.Command(wt, "run", outWasm)
	run.Dir = dir
	runOut, runErr := run.CombinedOutput()
	if runErr != nil {
		t.Fatalf("wasmtime run failed: %v\n%s", runErr, runOut)
	}
	if want := "10\n20\n40\n"; string(runOut) != want {
		t.Fatalf("struct output = %q, want %q", runOut, want)
	}
}

// --- wit command ------------------------------------------------------------

const phase139SampleWIT = `package karkain:geometry;

interface shapes {
  record point {
    x: s32,
    y: s32,
  }
  enum color { red, green, blue }
  area: func(s: string) -> f64;
}

world app {
  import shapes;
}
`

func TestPhase139_WitCommand(t *testing.T) {
	bin := buildKarkain(t)
	dir := t.TempDir()
	witFile := filepath.Join(dir, "shapes.wit")
	if err := os.WriteFile(witFile, []byte(phase139SampleWIT), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runBin(t, bin, dir, "wit", witFile)
	if err != nil {
		t.Fatalf("karkain wit: %v\n%s", err, out)
	}
	for _, frag := range []string{
		"package karkain:geometry;",
		"record point {",
		"area: func(s: string) -> f64;",
		"import shapes;",
	} {
		if !strings.Contains(out, frag) {
			t.Errorf("wit summary lacks %q:\n%s", frag, out)
		}
	}

	stubs, err := runBin(t, bin, dir, "wit", "--stubs", witFile)
	if err != nil {
		t.Fatalf("karkain wit --stubs: %v\n%s", err, stubs)
	}
	for _, frag := range []string{
		"type point struct { x int; y int }",
		"enum color { red, green, blue }",
	} {
		if !strings.Contains(stubs, frag) {
			t.Errorf("wit stubs lack %q:\n%s", frag, stubs)
		}
	}

	badFile := filepath.Join(dir, "bad.wit")
	if err := os.WriteFile(badFile, []byte("package karkain:bad; bogus foo;"), 0o644); err != nil {
		t.Fatal(err)
	}
	badOut, err := runBin(t, bin, dir, "wit", badFile)
	if err == nil {
		t.Fatalf("malformed WIT unexpectedly accepted:\n%s", badOut)
	}
	if code := exitCodeOf(t, err); code != ExitCompile {
		t.Errorf("malformed WIT exit = %d, want %d\n%s", code, ExitCompile, badOut)
	}

	missOut, err := runBin(t, bin, dir, "wit", filepath.Join(dir, "missing.wit"))
	if err == nil {
		t.Fatalf("missing WIT file unexpectedly accepted:\n%s", missOut)
	}
	if code := exitCodeOf(t, err); code != ExitUsage {
		t.Errorf("missing WIT exit = %d, want %d\n%s", code, ExitUsage, missOut)
	}
}
