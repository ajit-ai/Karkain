// v1.2.27 coverage instrumentation gate. `karkain coverage` reports FUNCTION
// coverage from an opt-in runtime; the gate pins the report, its determinism,
// the program's unchanged stdout, and the absence of instrumentation bytes in
// an ordinary (non-coverage) build.
package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func v127Run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, v1226Prepare(t), args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("command timed out: %v", args)
	}
	return stdout.String(), stderr.String(), err
}

const v127Program = "func helper(x int) int { return x + 1 }\n" +
	"func unused() int { return 7 }\n" +
	"func main() { print(helper(41)) }\n"

func TestPhaseV127Coverage_ReportAndDeterminism(t *testing.T) {
	path := v1226Source(t, "coverage", v127Program)
	var first string
	for i := 0; i < 2; i++ {
		out, errText, err := v127Run(t, "coverage", path)
		if err != nil {
			t.Fatalf("coverage run failed: %v\nstdout=%q\nstderr=%q", err, out, errText)
		}
		norm := v1226CNormalize(out)
		if i == 0 {
			first = norm
		} else if norm != first {
			t.Errorf("coverage report is not deterministic:\n%s\n---\n%s", first, norm)
		}
		if !strings.Contains(norm, "42\n") {
			t.Errorf("program stdout was not preserved: %q", norm)
		}
		if !strings.Contains(norm, "covered   helper (1 calls)") ||
			!strings.Contains(norm, "uncovered unused") ||
			!strings.Contains(norm, "covered   main (1 calls)") ||
			!strings.Contains(norm, "functions: 2/3 (66%)") {
			t.Errorf("coverage report missing an expected line:\n%s", norm)
		}
	}
}

func TestPhaseV127Coverage_OrdinaryBuildHasNoInstrumentation(t *testing.T) {
	path := v1226Source(t, "nocov", "func main() { print(1) }\n")
	out, errText, err := v127Run(t, "build", path, "--engine", "go")
	if err != nil {
		t.Fatalf("build failed: %v\nstdout=%q\nstderr=%q", err, out, errText)
	}
	data, readErr := os.ReadFile(filepath.Join(filepath.Dir(path), "nocov.c"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.Contains(string(data), "karkain_cov_hit") || strings.Contains(string(data), "karkain_cov_table") {
		t.Fatal("ordinary build carried coverage instrumentation")
	}
}
