package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 103: Module System v2 — export sets, qualified-name resolution and
// the public/private module contract on both engines.
//
//   - examples/module_system/  is the acceptance module: main.kark imports the
//     sibling math module and calls its public exports with qualified names.
//     The module program must run identically on the Go engine and the
//     self-hosted kcc engine (which now parses `public` and lowers qualified
//     calls onto the flat user-function namespace).
//   - examples/module_system_errors/ holds cross-module violations that the Go
//     engine's resolver must reject with exit 3 (private export, missing
//     import, undefined module function, cross-module name collision).

// moduleSystemGolden is the expected output of the module acceptance program.
const moduleSystemGolden = "42\n9\n"

// runPhase103Engine drives the module acceptance program through one engine.
func runPhase103Engine(t *testing.T, karkain, engine string) {
	t.Helper()
	root := repoRoot(t)
	mainFile := filepath.Join(root, "examples", "module_system", "main.kark")
	cmd := exec.Command(karkain, "run", mainFile, "--engine", engine)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("module run (engine=%s): exit err %v\n%s", engine, err, string(out))
	}
	got := stripKCCBuildBanner(t, string(out))
	got = strings.ReplaceAll(got, "\r\n", "\n")
	if got != moduleSystemGolden {
		t.Fatalf("module run (engine=%s): output mismatch\nwant:\n%q\ngot:\n%q", engine, moduleSystemGolden, got)
	}
}

// TestPhase103_ModuleSystem_GoGolden runs the module program through the Go
// engine and pins its output (public exports + qualified calls resolve and
// emit the bare user symbol).
func TestPhase103_ModuleSystem_GoGolden(t *testing.T) {
	root := repoRoot(t)
	runPhase103Engine(t, buildBootstrap(t, root), "go")
}

// TestPhase103_ModuleSystem_KCCGolden runs the same program through the
// self-hosted engine; the kcc lexer/parser accept `public` and lower qualified
// module calls so generated C links against the real module functions.
func TestPhase103_ModuleSystem_KCCGolden(t *testing.T) {
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	hasGCC(t)
	root := repoRoot(t)
	runPhase103Engine(t, buildBootstrap(t, root), "kcc")
}

// TestPhase103_ModuleSystem_KCCAccept verifies the self-hosted check path
// accepts a clean module program that uses the new `public` export modifier.
func TestPhase103_ModuleSystem_KCCAccept(t *testing.T) {
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	root := repoRoot(t)
	karkain := buildBootstrap(t, root)
	mainFile := filepath.Join(root, "examples", "module_system", "main.kark")
	cmd := exec.Command(karkain, "check", mainFile, "--engine", "kcc")
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=kcc")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kcc check of module program failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "[ok]") {
		t.Errorf("kcc check did not report [ok]:\n%s", string(out))
	}
}

// phase103Rejections maps each cross-module violation fixture to the diagnostic
// fragment `karkain check` must produce (Go engine), with exit 3.
var phase103Rejections = []struct {
	name string
	want string
}{
	{"main_private.kark", "private"},
	{"main_unimported.kark", "is not imported; add 'import lib'"},
	{"main_wrongmodule.kark", "function 'nothere' is not defined"},
	{"main_duplicate.kark", "duplicate function definition 'helper'"},
}

// TestPhase103_ModuleSystem_Rejections drives the Go engine's check command
// over every cross-module violation fixture and asserts exit 3 + the expected
// diagnostic.
func TestPhase103_ModuleSystem_Rejections(t *testing.T) {
	root := repoRoot(t)
	karkain := buildBootstrap(t, root)
	for _, fx := range phase103Rejections {
		f := filepath.Join(root, "examples", "module_system_errors", fx.name)
		cmd := exec.Command(karkain, "check", f, "--engine", "go")
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("%s: want exit 3, got success\n%s", fx.name, string(out))
			continue
		}
		var code int
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("%s: unexpected error: %v", fx.name, err)
		}
		if code != 3 {
			t.Errorf("%s: want exit 3, got %d:\n%s", fx.name, code, string(out))
		}
		if !strings.Contains(string(out), fx.want) {
			t.Errorf("%s: output lacks %q:\n%s", fx.name, fx.want, string(out))
		}
	}
}

// TestPhase103_CompilerSourcesStillClean re-runs the Phase 99/102 self-hosted
// gate after the parser changes (public modifier, qualified-call lowering): the
// assembled src/compiler sources must still parse and type-check clean under
// kcc — the compiler compiles itself.
func TestPhase103_CompilerSourcesStillClean(t *testing.T) {
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	root := repoRoot(t)
	karkain := buildBootstrap(t, root)
	mainFile := filepath.Join(root, "src", "compiler", "main.kark")
	if _, err := os.Stat(mainFile); err != nil {
		t.Skip("src/compiler not present")
	}
	cmd := exec.Command(karkain, "check", mainFile, "--engine", "kcc")
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=kcc")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("self-hosted check of compiler sources failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "[ok]") {
		t.Errorf("self-hosted check did not report [ok]:\n%s", string(out))
	}
}