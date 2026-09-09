package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 102: Karkain Language Foundation Completion. The examples under
// examples/language_foundation are the acceptance corpus for the baseline
// language surface. Every example must run identically on BOTH engines:
//
//   - the Go front end (karkain run --engine go)
//   - the self-hosted kcc engine (karkain run --engine kcc)
//
// and produce the exact golden output recorded here. The multi-file cases
// (modules/, application/) additionally exercise sibling-module assembly on
// both engines.

// phase102Case pairs an example file with its golden output.
type phase102Case struct {
	file string
	want string
}

// phase102Cases returns the full foundation corpus with golden outputs.
func phase102Cases(t *testing.T) []phase102Case {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "examples", "language_foundation")
	return []phase102Case{
		{filepath.Join(dir, "01_variables.kark"), "Karkain\n30\n3.14159\ntrue\n200\n5\n2.5\n"},
		{filepath.Join(dir, "02_functions.kark"), "30\n-10\nvalue is 7\n70\n10\n"},
		{filepath.Join(dir, "03_recursion.kark"), "3628800\n1\n55\n"},
		{filepath.Join(dir, "04_arrays.kark"), "5\n1\n99\n6\n3\n[2, 3]\n115\n"},
		{filepath.Join(dir, "05_strings.kark"), "Hello, World!\n13\nH\nW\nello\n1\n1\n42\n99\n3.5\n"},
		{filepath.Join(dir, "06_control_flow.kark"), "0\n1\n2\n3\n4\n0\n1\n2\n0\n1\n2\n1\n2\n4\n5\nbig\nelif\n"},
		{filepath.Join(dir, "07_structs.kark"), "Alice\n30\n31\n32\n0\n0\n"},
		{filepath.Join(dir, "08_maps.kark"), "Karkain\n6\n7\n0\n2\n1\n0\n1\n0\n"},
		{filepath.Join(dir, "09_types.kark"), "14\n6\n40\n2\n2\n4.5\n4\n3\n1024\n9\n10\n"},
		{filepath.Join(dir, "10_match.kark"), "300\noverflow\n7\nthree\n"},
		{filepath.Join(dir, "methods.kark"), "5\n15\n15\n"},
		{filepath.Join(dir, "modules", "main.kark"), "42\n81\nmodule math\n"},
		{filepath.Join(dir, "application", "main.kark"), "Ada: 1000\nAda: 1250\n62\n100\n"},
	}
}

// runPhase102Cases drives every example through one engine binary.
func runPhase102Cases(t *testing.T, karkain, engine string, cases []phase102Case) {
	t.Helper()
	for _, c := range cases {
		cmd := exec.Command(karkain, "run", c.file, "--engine", engine)
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("%s (engine=%s): exit err %v\n%s", filepath.Base(c.file), engine, err, string(out))
			continue
		}
		got := string(out)
		// The kcc engine prints a "[ok] path -> path.c23" build banner to
		// stdout ahead of the program output; strip it before comparing.
		got = stripKCCBuildBanner(t, got)
		// Normalize Windows CRLF line endings.
		got = strings.ReplaceAll(got, "\r\n", "\n")
		if got != c.want {
			t.Errorf("%s (engine=%s): output mismatch\nwant:\n%q\ngot:\n%q", filepath.Base(c.file), engine, c.want, got)
		}
	}
}

// stripKCCBuildBanner removes the self-hosted engine's `[ok] <src> -> <dst>
// (c23)` build banner line from run output.
func stripKCCBuildBanner(t *testing.T, out string) string {
	t.Helper()
	lines := strings.Split(out, "\n")
	kept := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.HasPrefix(ln, "[ok] ") {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

// TestPhase102_FoundationGolden_Go pins the Go engine's output for every
// foundation example, including the multi-file module and application cases.
func TestPhase102_FoundationGolden_Go(t *testing.T) {
	root := repoRoot(t)
	karkain := buildBootstrap(t, root)
	runPhase102Cases(t, karkain, "go", phase102Cases(t))
}

// TestPhase102_FoundationGolden_KCC runs the same corpus through the
// self-hosted engine (isolated build) and asserts identical output.
func TestPhase102_FoundationGolden_KCC(t *testing.T) {
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	hasGCC(t)
	root := repoRoot(t)
	karkain := buildBootstrap(t, root)
	runPhase102Cases(t, karkain, "kcc", phase102Cases(t))
}

// TestPhase102_CompilerSourcesSelfCheck re-verifies the Phase 99 self-hosted
// gate after the language-foundation parser/codegen work: the assembled
// src/compiler sources must still parse and type-check clean under kcc — the
// compiler compiles itself (check path only, which stays fast and low-memory).
func TestPhase102_CompilerSourcesSelfCheck(t *testing.T) {
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