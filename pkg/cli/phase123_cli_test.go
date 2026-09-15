package cli

// Phase 123 — Language Core Completion: enum/ADT codegen + match parity.
//
// Phase 123 Part A completes the enum (algebraic data type) surface on the
// self-hosted kcc engine with byte-identical Go↔kcc behavior:
//
//   - enum declarations with unit and payload variants (`enum Color { Red,
//     Green, Blue }` / `enum Shape { Circle(r), Rect(w), Unit }`);
//   - `EnumName.Variant` construction in expression position, equality
//     comparison, and tag-only match-arm patterns (`Shape.Circle => ...`);
//   - check-time enum validation in the self-hosted checker (K106 for an
//     unknown enum type, K113 for an unknown variant), covering BOTH
//     expression construction and match-arm patterns;
//   - the same `expected '=>' in match arm` parse error as the Go parser when
//     a match-arm pattern is followed by anything other than `=>` (Go does not
//     support payload destructuring `Shape.Circle(r)` in patterns);
//   - parser-level enum-variant registration so `EnumName.Variant` parses as a
//     dedicated EnumVariantExpr node, not a generic member access;
//   - NODE_ENUM_VARIANT_EXPR does not collide with NODE_CONST_DECL (both being
//     94 was corrupted KIR rendering of `EnumName.Variant` expressions).
//
// The pinned golden examples (examples/01-fundamentals/13_enums.kark and
// 14_adt_match.kark) are byte-identical on both engines and are additionally
// pinned in the Phase 114 corpus gate; this suite focuses on the language-core
// fabric: the new kcc-only enum/match checks and the parse-error parity that
// the corpus goldens do not exercise.
//
// Deliberate strictness divergence (documented, NOT a defect): the kcc checker
// rejects unknown enum variants (K113) and undeclared enum types used in
// EnumName.Variant construction or match-arm patterns (K106) at CHECK time,
// while the Go resolver defers such validation to the C compiler (Go check
// passes, the build fails on an undeclared tag constant). kcc also reports an
// undeclared dotted base (`MissingKind.Purple`) as K102 at check time, matching
// Go's K002. Both engines ultimately reject all four negative fixtures.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// phase123EnumFixtures maps the Phase 123 negative fixtures (created under
// examples/type_errors/) to the kcc check-time error code each must produce.
var phase123EnumFixtures = []struct {
	name string
	code string
	want string
}{
	{"err15_unknown_enum_variant_match", "K113", "Color.Purple"},
	{"err16_undefined_enum_match_arm", "K106", "Missing"},
	{"err17_undefined_enum_expr", "K102", "MissingKind"},
	{"err18_unknown_enum_variant_expr", "K113", "Color.Purple"},
}

// TestPhase123_EnumMatchChecker parses and type-checks every enum-positive and
// enum-negative fixture through the self-hosted checker, asserting the exact
// K1XX code and the offending name in the diagnostic.
func TestPhase123_EnumMatchChecker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping self-hosted Phase 123 gate in short mode")
	}
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	hasGCC(t)
	root := repoRoot(t)

	// Positive enum programs must pass check and never report a diagnostic.
	positives := []string{
		filepath.Join(root, "examples", "01-fundamentals", "13_enums.kark"),
		filepath.Join(root, "examples", "01-fundamentals", "14_adt_match.kark"),
	}
	for _, file := range positives {
		var buf bytes.Buffer
		res := KCCCheckCommand(&buf, file, false)
		if res.ExitCode != ExitSuccess {
			t.Errorf("kcc check rejected valid enum program %s (exit %d):\n%s",
				file, res.ExitCode, strings.TrimSpace(res.Message))
		} else if !strings.Contains(res.Message, "[ok]") {
			t.Errorf("kcc check of %s succeeded without the [ok] marker", file)
		}
	}

	// Negative fixtures must exit 3, mention the expected code + name, and
	// never be reported as [ok].
	for _, fx := range phase123EnumFixtures {
		file := filepath.Join(root, "examples", "type_errors", fx.name, "main.kark")
		var buf bytes.Buffer
		res := KCCCheckCommand(&buf, file, false)
		out := res.Message
		if res.ExitCode != ExitCompile {
			t.Errorf("fixture %s: want ExitCompile, got %d:\n%s", fx.name, res.ExitCode, strings.TrimSpace(out))
		}
		if strings.Contains(out, "[ok]") {
			t.Errorf("fixture %s: rejected program still reported as [ok]", fx.name)
		}
		if !strings.Contains(out, fx.code) {
			t.Errorf("fixture %s: output lacks expected error code %s:\n%s", fx.name, fx.code, strings.TrimSpace(out))
		}
		if !strings.Contains(out, fx.want) {
			t.Errorf("fixture %s: output lacks expected offending name %q:\n%s", fx.name, fx.want, strings.TrimSpace(out))
		}
	}
}

// TestPhase123_MatchArmParseErrorParity verifies the `=>` guard added to the
// kcc parseMatchExpr mirrors the Go parser exactly: a match-arm pattern
// followed by anything other than `=>` (here, payload destructuring — not part
// of the language) is rejected with the identical diagnostic on both engines.
func TestPhase123_MatchArmParseErrorParity(t *testing.T) {
	src := `enum Shape { Circle(r), Rect(w), Unit }

func area(s) {
    return match s {
        Shape.Circle(r) => 10,
        Shape.Rect => 20,
        _ => 30,
    }
}

func main() {
    let s = Shape.Circle(5)
    print(area(s))
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	const want = "expected '=>' in match arm, got '('"

	// Both engines route syntax errors through the Go-side Phase 105 preflight
	// (projectSyntaxDiagnostics), which renders to stderr — so drive the real
	// binary with CombinedOutput, exactly like `karkain check`, for each leg.
	bin := buildPreviewBinary(t)
	root := repoRoot(t)

	runEngine := func(engine string) (string, error) {
		cmd := exec.Command(bin, "check", path)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	for _, engine := range []string{"go", "kcc"} {
		out, err := runEngine(engine)
		if err == nil {
			t.Fatalf("%s engine check should fail for destructuring pattern", engine)
		}
		if !strings.Contains(out, want) {
			t.Fatalf("%s engine output lacks %q:\n%s", engine, want, out)
		}
	}
}

// TestPhase123_EnumVariantExpressionKIR verifies the EnumVariantExpr node
// renders correctly in KIR after the NODE_ENUM_VARIANT_EXPR=94/95 collision
// fix: construction must render as `(enum_variant EnumName Variant ...)`, not
// as a corrupted `(expr <ConstDecl>)` node.
func TestPhase123_EnumVariantExpressionKIR(t *testing.T) {
	bin := buildPreviewBinary(t)
	root := repoRoot(t)
	dir := t.TempDir()
	fixture := filepath.Join(dir, "main.kark")
	src := `enum Color { Red, Green, Blue }

func main() {
    let a = Color.Red
    let c = Color.Blue
    print(a)
    print(c)
}
`
	if err := os.WriteFile(fixture, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runBin(t, bin, root, "kir", fixture)
	if err != nil {
		t.Fatalf("karkain kir failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "enum Color (Red:") {
		t.Errorf("KIR missing enum declaration header:\n%s", out)
	}
	if !strings.Contains(out, "let a = (enum_variant Color Red)") {
		t.Errorf("KIR missing EnumVariantExpr construction for Color.Red:\n%s", out)
	}
	if !strings.Contains(out, "let c = (enum_variant Color Blue)") {
		t.Errorf("KIR missing EnumVariantExpr construction for Color.Blue:\n%s", out)
	}
	if strings.Contains(out, "<ConstDecl>") {
		t.Errorf("KIR corrupted EnumVariantExpr as const-decl reference:\n%s", out)
	}
}

// ── Phase 123 Part B — WASM Stabilization & Production Foundation ────────────

// wasmExample holds a WASM corpus entry: source path relative to
// examples/wasm and expected stdout lines (separated by newline).
type wasmExample struct {
	relDir  string // subdirectory under examples/wasm ("" for root hello.kark)
	wantOut string
}

var phase123WasmCorpus = []wasmExample{
	{"", "hello wasmtime\n42\ndone\n"},
	{"functions", "5\n14\n"},
	{"control_flow", "30\n120\n"},
	{"data", "5\n30\n"},
	{"strings_builtin", "12\n"},
}

// buildAndRunWasm compiles a .kark file to .wasm, runs it with wasmtime,
// and returns the stdout string.  Returns error on build or run failure.
func buildAndRunWasm(t *testing.T, srcFile string, args ...string) (string, error) {
	t.Helper()
	tmpDir := t.TempDir()
	outWasm := filepath.Join(tmpDir, "out.wasm")
	cmd := exec.Command("go", "run", "../../cmd/karkain", "build",
		"--target", "wasm32-wasi", "-o", outWasm, srcFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build failed: %v\n%s", err, out)
	}

	wt := findWasmtime()
	if wt == "" {
		t.Skip("wasmtime not found; install from https://wasmtime.dev")
	}
	runArgs := append([]string{"run", outWasm}, args...)
	run := exec.Command(wt, runArgs...)
	run.Dir = tmpDir
	var stdout, stderr bytes.Buffer
	run.Stdout = &stdout
	run.Stderr = &stderr
	err := run.Run()
	if stderr.Len() > 0 {
		t.Logf("wasmtime stderr: %s", strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), err
}

// TestPhase123_WasmExampleCorpus builds and runs all five Phase 123 WASM
// examples through the real pipeline (karkain build --target wasm32-wasi
// + wasmtime run) and asserts the expected stdout.
func TestPhase123_WasmExampleCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WASM corpus in short mode")
	}
	wt := findWasmtime()
	if wt == "" {
		t.Skip("wasmtime not found")
	}
	root := repoRoot(t)
	examplesDir := filepath.Join(root, "examples", "wasm")
	if _, err := os.Stat(examplesDir); err != nil {
		t.Skip("examples/wasm/ missing")
	}

	for _, ex := range phase123WasmCorpus {
		name := ex.relDir
		if name == "" {
			name = "hello"
		}
		t.Run(name, func(t *testing.T) {
			srcFile := filepath.Join(examplesDir, ex.relDir, "main.kark")
			if ex.relDir == "" {
				srcFile = filepath.Join(examplesDir, "hello.kark")
			}
			if _, err := os.Stat(srcFile); err != nil {
				t.Skipf("source missing: %v", err)
			}
			got, err := buildAndRunWasm(t, srcFile)
			if err != nil {
				t.Fatalf("wasmtime run failed: %v\nstdout: %s", err, got)
			}
			if got != ex.wantOut {
				t.Errorf("stdout mismatch\ngot:  %q\nwant: %q", got, ex.wantOut)
			}
		})
	}
}

// TestPhase123_WasmExitCodeContract verifies that `return N` in a .kark
// program translates to wasmtime exit code N for a set of representative
// values.
func TestPhase123_WasmExitCodeContract(t *testing.T) {
	wt := findWasmtime()
	if wt == "" {
		t.Skip("wasmtime not found")
	}

	cases := []struct {
		code string
		want int
	}{
		{"func main() { return 0 }", 0},
		{"func main() { return 1 }", 1},
		{"func main() { return 3 }", 3},
		{"func main() { return 42 }", 42},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("exit_%d", tc.want), func(t *testing.T) {
			dir := t.TempDir()
			srcFile := filepath.Join(dir, "main.kark")
			if err := os.WriteFile(srcFile, []byte(tc.code), 0o644); err != nil {
				t.Fatal(err)
			}
			tmpDir := t.TempDir()
			outWasm := filepath.Join(tmpDir, "out.wasm")
			cmd := exec.Command("go", "run", "../../cmd/karkain", "build",
				"--target", "wasm32-wasi", "-o", outWasm, srcFile)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("build failed: %v\n%s", err, out)
			}
			run := exec.Command(wt, "run", outWasm)
			run.Dir = tmpDir
			err := run.Run()
			got := exitCode(err)
			if got != tc.want {
				t.Errorf("exit code: got %d, want %d (err=%v)", got, tc.want, err)
			}
		})
	}
}

// TestPhase123_WasmRuntimeError verifies that a runtime error (division
// by zero) produces "runtime error:" on stderr and exit code 1.
func TestPhase123_WasmRuntimeError(t *testing.T) {
	wt := findWasmtime()
	if wt == "" {
		t.Skip("wasmtime not found")
	}
	src := `func main() {
    let x = 10
    let y = 0
    let z = x / y
    print(z)
}
`
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(srcFile, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	outWasm := filepath.Join(tmpDir, "out.wasm")
	cmd := exec.Command("go", "run", "../../cmd/karkain", "build",
		"--target", "wasm32-wasi", "-o", outWasm, srcFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	run := exec.Command(wt, "run", outWasm)
	run.Dir = tmpDir
	var stderr bytes.Buffer
	run.Stderr = &stderr
	err := run.Run()
	got := exitCode(err)
	if got != 1 {
		t.Errorf("exit code: got %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "runtime error:") {
		t.Errorf("stderr missing 'runtime error:': %q", stderr.String())
	}
}

// TestPhase123_WasmGetArgs verifies that getArgs() returns an array
// of the CLI arguments.
func TestPhase123_WasmGetArgs(t *testing.T) {
	wt := findWasmtime()
	if wt == "" {
		t.Skip("wasmtime not found")
	}
	src := `func main() {
    let args = getArgs()
    print(len(args))
    for (let i = 0; i < len(args); i++) {
        print(args[i])
    }
}
`
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(srcFile, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	outWasm := filepath.Join(tmpDir, "out.wasm")
	cmd := exec.Command("go", "run", "../../cmd/karkain", "build",
		"--target", "wasm32-wasi", "-o", outWasm, srcFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	run := exec.Command(wt, "run", outWasm, "alpha", "beta", "42")
	run.Dir = tmpDir
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("wasmtime run failed: %v\n%s", err, out)
	}
	got := string(out)
	// Expected: argc=4, then 4 argument strings
	if !strings.Contains(got, "4") {
		t.Errorf("expected argc 4 in output:\n%s", got)
	}
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") || !strings.Contains(got, "42") {
		t.Errorf("missing expected arguments in output:\n%s", got)
	}
}

// TestPhase123_WasmNegativeFeatures verifies that unsupported language
// features — structs, maps, and the module system (import) — are rejected
// at WASM build time with a deterministic K108 diagnostic that identifies
// the unsupported feature and the selected target.  All failures exit 3 and
// are never reported as [ok].  The stdlib import fixture lives inside the
// repository tree so stdlib discovery succeeds and the WASM backend's own
// modules gate fires (rather than an out-of-tree module-resolution failure).
func TestPhase123_WasmNegativeFeatures(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string // diagnostic fragment expected on failure
	}{
		{"struct", `func main() { let s = Point{ x: 1, y: 2 }; print(s.x) }`, "K108"},
		{"map", `func main() { let m = identity({ "a": 1 }); print(m["a"]) }
func identity(x) { return x }`, "K108"},
		{"import", "import std.io\nfunc main() { print(\"hi\") }", "K108"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			// The stdlib import fixture must live inside the repository tree so
			// stdlib discovery succeeds and the WASM backend's own K108 modules
			// gate fires (an out-of-tree temp file fails earlier at module
			// resolution with "module not found", which is still deterministic
			// but does not exercise the target-specific diagnostic).
			if tc.name == "import" {
				root := repoRoot(t)
				dir = t.TempDir()
				probe := filepath.Join(root, "examples", "wasm", ".tmp-import-probe")
				if err := os.RemoveAll(probe); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(probe, 0o755); err != nil {
					t.Fatal(err)
				}
				defer os.RemoveAll(probe)
				dir = probe
			}
			srcFile := filepath.Join(dir, "main.kark")
			if err := os.WriteFile(srcFile, []byte(tc.src), 0o644); err != nil {
				t.Fatal(err)
			}
			tmpDir := t.TempDir()
			outWasm := filepath.Join(tmpDir, "out.wasm")
			cmd := exec.Command("go", "run", "../../cmd/karkain", "build",
				"--target", "wasm32-wasi", "-o", outWasm, srcFile)
			out, err := cmd.CombinedOutput()
			sout := string(out)
			if err == nil {
				t.Fatalf("expected build failure for %s, got success:\n%s", tc.name, sout)
			}
			if !strings.Contains(sout, tc.want) {
				t.Errorf("expected diagnostic %q for %s, got:\n%s", tc.want, tc.name, sout)
			}
			if strings.Contains(sout, "[ok]") {
				t.Errorf("rejected %s program still reported [ok]:\n%s", tc.name, sout)
			}
		})
	}
}

// TestPhase123_WasmDeterminism builds the same source twice and asserts
// byte-identical .wasm output.
func TestPhase123_WasmDeterminism(t *testing.T) {
	wt := findWasmtime()
	if wt == "" {
		t.Skip("wasmtime not found")
	}
	src := `func main() { let s = 0; for (let i = 1; i <= 100; i++) { s = s + i }; print(s) }
`
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(srcFile, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	build := func() []byte {
		tmpDir := t.TempDir()
		outWasm := filepath.Join(tmpDir, "out.wasm")
		cmd := exec.Command("go", "run", "../../cmd/karkain", "build",
			"--target", "wasm32-wasi", "-o", outWasm, srcFile)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build failed: %v\n%s", err, out)
		}
		b, err := os.ReadFile(outWasm)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	a := build()
	b := build()
	if !bytes.Equal(a, b) {
		t.Fatalf("non-deterministic wasm output: len=%d vs %d", len(a), len(b))
	}
	t.Logf("deterministic wasm: %d bytes", len(a))
}

// exitCode extracts the exit code from an exec.ExitError, defaulting to -1.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
}