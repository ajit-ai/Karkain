// v1.2.26 slice 8 (S8) - const folding parity in the self-hosted engine.
// The Go engine's S5 folds constant integer +,-,* into make_int at codegen
// time; S8 makes kcc emit the same folded form. Division and modulo are
// deliberately NOT folded by either engine so a zero divisor still reaches
// the checked runtime helper and reports a source-located error.
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

func v1226CRun(t *testing.T, engine, path string, args ...string) (string, string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	argv := append([]string{args[0], path, "--engine", engine}, args[1:]...)
	cmd := exec.CommandContext(ctx, v1226Prepare(t), argv...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s %s timed out", engine, args[0])
	}
	return stdout.String(), stderr.String(), err
}

func v1226CNormalize(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func TestPhaseV1226C_FoldedValuesMatchBothEngines(t *testing.T) {
	path := v1226Source(t, "cfold", "func main() { print(2 + 3 * 4); print(10 - 3); print(7 * 3) }\n")
	const want = "14\n7\n21\n"
	var goOut string
	for i, eng := range []string{"go", "kcc"} {
		out, errText, err := v1226CRun(t, eng, path, "run")
		if err != nil {
			t.Fatalf("%s run failed: %v\nstdout=%q\nstderr=%q", eng, err, out, errText)
		}
		got := v1226CNormalize(out)
		if i == 0 {
			goOut = got
		} else if got != goOut {
			t.Errorf("engine stdout differs: go=%q %s=%q", goOut, eng, got)
		}
		if got != want {
			t.Errorf("%s stdout = %q, want %q", eng, got, want)
		}
	}
}

func TestPhaseV1226C_KCCEmitsFoldedConstant(t *testing.T) {
	path := v1226Source(t, "cfoldc", "func main() { print(2 + 3 * 4) }\n")
	// The CLI's kcc engine stages its generated C in a sandbox, so drive the
	// self-hosted compiler directly: kcc build writes <file>.c23 beside the
	// source, which is the artifact whose emitted form this layer inspects.
	kcc := phase95KCC(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, kcc, "build", path)
	cmd.Dir = filepath.Dir(path)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("kcc build failed: %v\nstdout=%q\nstderr=%q", err, stdout.String(), stderr.String())
	}
	found := false
	entries, readErr := os.ReadDir(filepath.Dir(path))
	if readErr != nil {
		t.Fatal(readErr)
	}
	var sample string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".c") && !strings.HasSuffix(e.Name(), ".c23") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(filepath.Dir(path), e.Name()))
		if readErr != nil {
			continue
		}
		if len(sample) == 0 {
			sample = string(data)
		}
		if strings.Contains(string(data), "make_int(14)") {
			found = true
		}
	}
	if !found {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		if len(sample) > 1200 {
			sample = sample[:1200]
		}
		t.Fatalf("no generated C contains make_int(14); kcc did not fold 2 + 3 * 4 (dir %s: %v)\n--- generated C sample ---\n%s", filepath.Dir(path), names, sample)
	}
}

func TestPhaseV1226C_DivisionByZeroStillChecked(t *testing.T) {
	path := v1226Source(t, "divzero", "func main() { print(1 / 0) }\n")
	for _, eng := range []string{"go", "kcc"} {
		out, errText, err := v1226CRun(t, eng, path, "run")
		if err == nil {
			t.Errorf("%s accepted 1/0; stdout=%q", eng, out)
		}
		if combined := out + errText; !strings.Contains(combined, "division by zero") {
			t.Errorf("%s did not report a checked division error: %q", eng, combined)
		}
	}
}
