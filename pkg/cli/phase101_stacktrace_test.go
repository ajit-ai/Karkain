package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"karkain/pkg/codegen"
)

// Phase 101 gate: the runtime debugger foundation. When a program fails at
// runtime, both engines must report the source-located header (Phase 100) and a
// "  stack:" chain naming the Karkain functions active at the failure, each with
// a source file and line. Go engine and self-hosted kcc must agree on the chain.

var stackFrameRe = regexp.MustCompile(`(?m)^    (\w+) \(([^:]+):(\d+)\)\r?$`)

// wantStackChain encodes the expected frames (name, line) for the
// stack_chain.kark fixture: inner fails at line 2 after being called from outer.
var wantStackChain = []struct {
	name string
	line string
}{
	{"inner", "2"},
	{"outer", "5"},
	{"main", "9"},
}

// TestPhase101_StackChainParity runs the call-chain fixture on both engines and
// requires identical frame chains plus the ExitFailure exit code.
func TestPhase101_StackChainParity(t *testing.T) {
	fixture := filepath.Join(runtimeErrorsDir(t), "stack_chain.kark")

	prog, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	tmpProg := filepath.Join(tmpDir, "stack_chain.kark")
	if err := os.WriteFile(tmpProg, prog, 0o644); err != nil {
		t.Fatal(err)
	}

	var goOut, goErr bytes.Buffer
	goRes := RunCommand(tmpProg, codegen.Config{Stdout: &goOut, Stderr: &goErr}, false)
	if goRes.ExitCode != ExitFailure {
		t.Fatalf("Go engine: exit=%d want ExitFailure(%d) (out=%q err=%q)",
			goRes.ExitCode, ExitFailure, goOut.String(), goErr.String())
	}
	goDiag := runtimeErrorLocRe.FindStringSubmatch(goErr.String())
	if goDiag == nil {
		t.Fatalf("Go engine: no source-located runtime diagnostic in %q", goErr.String())
	}
	goFrames := stackFrameRe.FindAllStringSubmatch(goErr.String(), -1)
	if goFrames == nil {
		t.Fatalf("Go engine: no stack chain in %q", goErr.String())
	}

	kccRes := KCCRunCommand(nil, fixture, codegen.Config{}, false)
	if kccRes.ExitCode != ExitFailure {
		t.Fatalf("kcc engine: exit=%d want ExitFailure(%d) (msg=%q)",
			kccRes.ExitCode, ExitFailure, kccRes.Message)
	}
	kccDiag := runtimeErrorLocRe.FindStringSubmatch(kccRes.Message)
	if kccDiag == nil {
		t.Fatalf("kcc engine: no source-located runtime diagnostic in %q", kccRes.Message)
	}
	kccFrames := stackFrameRe.FindAllStringSubmatch(kccRes.Message, -1)
	if kccFrames == nil {
		t.Fatalf("kcc engine: no stack chain in %q", kccRes.Message)
	}

	// Header parity: failing location must be stack_chain.kark line 2 on both.
	if filepath.Base(goDiag[1]) != "stack_chain.kark" || goDiag[2] != "2" {
		t.Errorf("Go engine: wrong header location %q", goErr.String())
	}
	if filepath.Base(kccDiag[1]) != "stack_chain.kark" || kccDiag[2] != "2" {
		t.Errorf("kcc engine: wrong header location %q", kccRes.Message)
	}

	// Chain parity: identical frames on both engines.
	if len(goFrames) != len(kccFrames) {
		t.Fatalf("frame count parity: Go=%d kcc=%d (Go %q kcc %q)",
			len(goFrames), len(kccFrames), goErr.String(), kccRes.Message)
	}
	if len(goFrames) != len(wantStackChain) {
		t.Fatalf("frame count=%d want %d (Go %q kcc %q)",
			len(goFrames), len(wantStackChain), goErr.String(), kccRes.Message)
	}
	for i, want := range wantStackChain {
		for _, eng := range []struct {
			name string
			fs   [][]string
		}{
			{"Go", goFrames},
			{"kcc", kccFrames},
		} {
			f := eng.fs[i]
			if f[1] != want.name {
				t.Errorf("%s engine: frame %d name=%q want %q", eng.name, i, f[1], want.name)
			}
			if filepath.Base(f[2]) != "stack_chain.kark" {
				t.Errorf("%s engine: frame %d file=%q", eng.name, i, f[2])
			}
			if f[3] != want.line {
				t.Errorf("%s engine: frame %d line=%s want %s", eng.name, i, f[3], want.line)
			}
		}
	}
}

// TestPhase101_NormalRunHasNoStack verifies successful programs never emit a
// stack chain — the instrumentation must be invisible on the happy path.
func TestPhase101_NormalRunHasNoStack(t *testing.T) {
	fixture := filepath.Join(runtimeErrorsDir(t), "multi_ok.kark")

	prog, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	tmpProg := filepath.Join(tmpDir, "multi_ok.kark")
	if err := os.WriteFile(tmpProg, prog, 0o644); err != nil {
		t.Fatal(err)
	}

	var goOut, goErr bytes.Buffer
	goRes := RunCommand(tmpProg, codegen.Config{Stdout: &goOut, Stderr: &goErr}, false)
	if goRes.ExitCode != ExitSuccess {
		t.Fatalf("Go engine: exit=%d want 0 (err=%q)", goRes.ExitCode, goErr.String())
	}
	if strings.Contains(goErr.String(), "  stack:") {
		t.Errorf("Go engine: spurious stack on success in %q", goErr.String())
	}

	kccRes := KCCRunCommand(nil, fixture, codegen.Config{}, false)
	if kccRes.ExitCode != ExitSuccess {
		t.Fatalf("kcc engine: exit=%d want 0 (msg=%q)", kccRes.ExitCode, kccRes.Message)
	}
	if strings.Contains(kccRes.Message, "  stack:") {
		t.Errorf("kcc engine: spurious stack on success in %q", kccRes.Message)
	}
}