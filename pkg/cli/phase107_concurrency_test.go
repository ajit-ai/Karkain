package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/codegen"
)

// Phase 107 gate: Concurrency Runtime end-to-end. examples/concurrency/
// pipeline exercises spawn+join, a producer task streaming over a channel
// with deterministic close, and a serialized actor handler — all through the
// real Go-engine front end, the Phase 107 codegen wrappers, the embedded C
// runtime and the host C compiler.

const phase107ExampleDir = "concurrency/pipeline"

func TestPhase107_ConcurrencyE2E_RunsClean(t *testing.T) {
	skipIfNoCompiler(t)

	dir := t.TempDir()
	copyTree(t, filepath.Join("..", "..", "examples", phase107ExampleDir), dir)
	exePath := filepath.Join(dir, "main_107.exe")

	res := BuildCommandIncremental(filepath.Join(dir, "main.kark"), exePath, codegen.Config{}, false, filepath.Join(dir, ".karkain-cache"))
	if res.ExitCode != ExitSuccess {
		t.Fatalf("concurrency build failed: %q (exit %d)", res.Message, res.ExitCode)
	}
	if _, err := os.Stat(exePath); err != nil {
		t.Fatalf("expected executable at %s: %v", exePath, err)
	}

	c, err := os.ReadFile(filepath.Join(dir, "main.c"))
	if err != nil {
		t.Fatalf("expected generated main.c: %v", err)
	}
	for _, marker := range []string{
		"karkain_sched_spawn",
		"karkain_conc_channel(make_int(-1))",
		"karkain_channel_send",
		"karkain_conc_recv",
		"karkain_conc_actor",
		"karkain_run_0",
		"karkain_actor_dispatch",
	} {
		if !strings.Contains(string(c), marker) {
			t.Errorf("generated C missing %q", marker)
		}
	}

	out := runExe(t, exePath)
	want := "144\n10\n20\n30\n0\n6\n"
	if out != want {
		t.Fatalf("concurrency executable output = %q, want %q", out, want)
	}
}

// TestPhase107_ConcurrencyDeterminism proves the scheduler reproduces the same
// join/actor/channel results across repeated fresh processes (no ordering or
// memory bugs steal a status or drop a message).
func TestPhase107_ConcurrencyDeterminism(t *testing.T) {
	skipIfNoCompiler(t)

	dir := t.TempDir()
	copyTree(t, filepath.Join("..", "..", "examples", phase107ExampleDir), dir)
	exePath := filepath.Join(dir, "main_107.exe")

	res := BuildCommandIncremental(filepath.Join(dir, "main.kark"), exePath, codegen.Config{}, false, filepath.Join(dir, ".karkain-cache"))
	if res.ExitCode != ExitSuccess {
		t.Fatalf("concurrency build failed: %q (exit %d)", res.Message, res.ExitCode)
	}

	first := runExe(t, exePath)
	if first == "" {
		t.Fatal("no output")
	}
	for i := 2; i <= 5; i++ {
		if got := runExe(t, exePath); got != first {
			t.Fatalf("run %d output %q != first run %q", i, got, first)
		}
	}
}