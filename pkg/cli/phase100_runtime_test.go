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

// Phase 100 parity gate: the runtime failure model must behave identically on
// the Go engine and the self-hosted kcc engine. Both engines must (a) stop
// silently returning zero for invalid divisions/modulo/index accesses, (b)
// report a source-located `runtime error: <kind> at <file>:<line>` diagnostic,
// and (c) exit with the documented program-failure code (ExitFailure).

// runtimeErrorsDir locates the examples/runtime_errors fixture corpus.
func runtimeErrorsDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.Abs(filepath.Join(wd, "..", "..", "examples", "runtime_errors"))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

var runtimeErrorLocRe = regexp.MustCompile(`runtime error: [^\n]* at ([^:]+):(\d+)`)

// TestPhase100_RuntimeErrorParity runs every negative fixture through both
// engines and requires identical diagnostic kind and source location plus the
// ExitFailure exit code on each engine.
func TestPhase100_RuntimeErrorParity(t *testing.T) {
	dir := runtimeErrorsDir(t)
	cases := []struct {
		file string
		kind string
	}{
		{"div_by_zero.kark", "integer division by zero"},
		{"mod_by_zero.kark", "integer modulo by zero"},
		{"float_div_by_zero.kark", "division by zero"},
		{"float_mod_by_zero.kark", "modulo by zero"},
		{"array_oob_read.kark", "array index out of range"},
		{"array_oob_write.kark", "array index out of range"},
		{"string_oob.kark", "string index out of range"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			fixture := filepath.Join(dir, tc.file)

			// Go engine (run from a temp copy so artifacts never land in the repo).
			prog, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatal(err)
			}
			tmpDir := t.TempDir()
			tmpProg := filepath.Join(tmpDir, tc.file)
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
			if !strings.Contains(goErr.String(), tc.kind) {
				t.Errorf("Go engine: diagnostic kind mismatch in %q", goErr.String())
			}

			// Self-hosted kcc engine.
			kccRes := KCCRunCommand(nil, fixture, codegen.Config{}, false)
			if kccRes.ExitCode != ExitFailure {
				t.Fatalf("kcc engine: exit=%d want ExitFailure(%d) (msg=%q)",
					kccRes.ExitCode, ExitFailure, kccRes.Message)
			}
			kccDiag := runtimeErrorLocRe.FindStringSubmatch(kccRes.Message)
			if kccDiag == nil {
				t.Fatalf("kcc engine: no source-located runtime diagnostic in %q", kccRes.Message)
			}
			if !strings.Contains(kccRes.Message, tc.kind) {
				t.Errorf("kcc engine: diagnostic kind mismatch in %q", kccRes.Message)
			}

			// Parity: identical file and line on both engines.
			if filepath.Base(goDiag[1]) != filepath.Base(kccDiag[1]) {
				t.Errorf("file parity: Go=%q kcc=%q", goDiag[1], kccDiag[1])
			}
			if goDiag[2] != kccDiag[2] {
				t.Errorf("line parity: Go=%s kcc=%s", goDiag[2], kccDiag[2])
			}
		})
	}
}

// TestPhase100_ValidArithmeticParity verifies the positive control: division,
// modulo and index operations that are in range keep their exact results and
// success exit on both engines (the checked helpers must not over-report).
func TestPhase100_ValidArithmeticParity(t *testing.T) {
	dir := runtimeErrorsDir(t)
	fixture := filepath.Join(dir, "multi_ok.kark")

	prog, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	tmpProg := filepath.Join(tmpDir, filepath.Base(fixture))
	if err := os.WriteFile(tmpProg, prog, 0o644); err != nil {
		t.Fatal(err)
	}

	var goOut, goErr bytes.Buffer
	goRes := RunCommand(tmpProg, codegen.Config{Stdout: &goOut, Stderr: &goErr}, false)
	if goRes.ExitCode != ExitSuccess {
		t.Fatalf("Go engine: exit=%d want 0 (err=%q)", goRes.ExitCode, goErr.String())
	}
	goStdout := strings.ReplaceAll(goOut.String(), "\r\n", "\n")
	if goStdout != "3\n1\n99\ne\n" {
		t.Errorf("Go engine: unexpected output %q", goStdout)
	}

	kccRes := KCCRunCommand(nil, fixture, codegen.Config{}, false)
	if kccRes.ExitCode != ExitSuccess {
		t.Fatalf("kcc engine: exit=%d want 0 (msg=%q)", kccRes.ExitCode, kccRes.Message)
	}
	kccOutput := strings.ReplaceAll(kccRes.Message, "\r\n", "\n")
	if !strings.Contains(kccOutput, "3\n1\n99\ne") {
		t.Errorf("kcc engine: unexpected output %q", kccOutput)
	}
	if strings.Contains(kccOutput, "runtime error") {
		t.Errorf("kcc engine: spurious runtime error in %q", kccOutput)
	}
}
