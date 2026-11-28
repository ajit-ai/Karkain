// v1.2.34 follow-up: dynamic-library boundary. The decision is that no target
// supports dynamic loading from a .kark program; static `import "C"` FFI stays
// the only interop path. Baseline: docs/audit/V1.2.34-DYNLIB-BOUNDARY.md.
package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karkain/pkg/jit"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// The JIT FFI registry is a simulation. This layer proves both halves: the
// simulation PATH works (positive control), while real dynamic loading does
// not. Without the positive control, a registry that failed at every step
// would satisfy the negative assertions and prove nothing.
func TestPhaseV1234D_JITDynamicLoadingIsStubbed(t *testing.T) {
	reg := jit.NewFFIRegistry()
	lib, err := reg.LoadLibrary("boundary", "libdoesnotexist.so")
	if err != nil {
		t.Fatalf("LoadLibrary must not fail in simulation mode: %v", err)
	}
	if lib.Handle != 0 {
		t.Errorf("simulated library handle = %d, want 0 (no OS handle is opened)", lib.Handle)
	}

	// Positive control: a manually registered symbol resolves.
	lib.RegisterSymbol("registered", 0, jit.CTypeI32, nil)
	sym, err := lib.ResolveSymbol("registered")
	if err != nil || sym == nil {
		t.Fatalf("positive control failed: manually registered symbol did not resolve: sym=%v err=%v", sym, err)
	}

	// The boundary itself: a symbol that is not registered can NEVER be
	// resolved, because dynamicSymbol is a stub that always errors.
	if _, err := lib.ResolveSymbol("dlopen"); err == nil {
		t.Fatal("ResolveSymbol('dlopen') succeeded -- dynamic symbol resolution is no longer a stub")
	} else if !strings.Contains(err.Error(), "dynamic symbol resolution not available") {
		t.Errorf("unexpected resolution error: %v", err)
	}
}

func v1234dRun(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, v1226Prepare(t), args...)
	var out, errText strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errText
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("command timed out: %v", args)
	}
	return out.String(), errText.String(), err
}

func TestPhaseV1234D_NoLanguageDynlibBuiltin(t *testing.T) {
	// A direct dlopen call must be an unknown function on BOTH engines: there
	// is no .kark-reachable dynamic loader. The symbol name must appear in the
	// diagnostic so a generic failure cannot be mistaken for this boundary.
	path := v1226Source(t, "dynlib_undefined", "func main() { print(dlopen(\"libx\")) }\n")
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out, errText, err := v1234dRun(t, "check", path, "--engine", engine)
			if err == nil {
				t.Fatalf("%s check accepted dlopen: stdout=%q stderr=%q", engine, out, errText)
			}
			all := out + errText
			if !strings.Contains(all, "dlopen") {
				t.Errorf("%s refusal did not name the symbol: %q", engine, all)
			}
			if !strings.Contains(all, "K002") && !strings.Contains(all, "K101") {
				t.Errorf("%s refusal did not carry an undefined-function code: %q", engine, all)
			}
		})
	}
}

func TestPhaseV1234D_NativeDynlibRefused(t *testing.T) {
	path := v1226Source(t, "dynlib_native", "func main() { print(dlopen(\"libx\")) }\n")
	image := filepath.Join(filepath.Dir(path), "dynlib.exe")
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out, errText, err := v1234dRun(t, "build", path, "--engine", engine,
				"--target", "native-x86_64-windows", "-o", image)
			combined := out + errText
			if err == nil {
				t.Fatalf("%s native build accepted a dynamic load: %q", engine, combined)
			}
			if !strings.Contains(combined, "dlopen") {
				t.Errorf("%s native refusal did not name the construct: %q", engine, combined)
			}
			if _, statErr := os.Stat(image); statErr == nil {
				t.Error("a refused native build must not write an image")
			}
		})
	}
}

// The positive control for the language layer: the SUPPORTED interop surface
// is static C FFI, and it must still be accepted and emitted as a C
// declaration. Without this, "dynamic loading is unsupported" could be read
// as "no interop works at all".
// Positive control at the SYNTAX level: the supported interop surface
// (`import "C"` blocks) is still accepted. This deliberately does NOT claim
// that a typed C call links -- a measured, separate defect is recorded in the
// baseline. It only prevents "dynamic loading is unsupported" from being read
// as "the parser rejects all interop".
func TestPhaseV1234D_StaticCInteropSyntaxAccepted(t *testing.T) {
	src := "import \"C\" {\n    func abs(x int) int\n}\nfunc main() { print(1) }\n"
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("static C import syntax was rejected: %v", p.Errors)
	}
	if prog == nil {
		t.Fatal("ParseProgram returned nil for a static C import")
	}
}
