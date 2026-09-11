package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/codegen"
)

// Phase 112 gate: std.testing module — assertion helpers and counts usable
// from .kark programs. Tested by building and running a real example on BOTH
// engines (the test-driver synthesis does not support imports, so stdlib
// modules are validated through the build+run path, mirroring phase109).

func phase112BuildAndRun(t *testing.T, engine, example string) string {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "main.exe")

	switch engine {
	case "go":
		res := BuildCommandIncremental(example, exe, codegen.Config{}, false, filepath.Join(dir, ".cache"))
		if res.ExitCode != ExitSuccess {
			t.Fatalf("Go build failed: [%d] %s", res.ExitCode, res.Message)
		}
	case "kcc":
		var buildOut strings.Builder
		res := KCCBuildCommand(&buildOut, example, exe, codegen.Config{}, false)
		if res.ExitCode != ExitSuccess {
			t.Fatalf("kcc build failed: [%d] %s — %s", res.ExitCode, res.Message, buildOut.String())
		}
	default:
		t.Fatalf("unknown engine %q", engine)
	}

	run := exec.Command(exe)
	run.Dir = dir
	var stdout strings.Builder
	var stderr strings.Builder
	run.Stdout = &stdout
	run.Stderr = &stderr
	if err := run.Run(); err != nil {
		t.Fatalf("run failed (engine=%s): %v — %s", engine, err, stderr.String())
	}
	return strings.ReplaceAll(stdout.String(), "\r\n", "\n")
}

func TestPhase112_StdTesting_RunBothEngines(t *testing.T) {
	skipIfNoCompiler(t)
	root := repoRoot(t)
	example := filepath.Join(root, "examples", "testing", "main.kark")
	if _, err := os.Stat(example); err != nil {
		t.Fatalf("std.testing example missing: %s", example)
	}
	want := "checks: 3 passed; 2 failed"

	for _, engine := range []string{"go", "kcc"} {
		t.Run("engine="+engine, func(t *testing.T) {
			got := phase112BuildAndRun(t, engine, example)
			if !strings.Contains(got, want) {
				t.Errorf("engine %s: output missing %q\n%s", engine, want, got)
			}
		})
	}
}
