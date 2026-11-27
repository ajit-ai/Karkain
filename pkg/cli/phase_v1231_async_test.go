// v1.2.31 async/await gate: both engines execute the same program through the
// existing spawn/join runtime, with exact stdout. async f(args) lowers to
// spawn(f, args) and await identifier lowers to join(identifier).
package cli

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func v1231Run(t *testing.T, engine, path string) (string, string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, v1226Prepare(t), "run", path, "--engine", engine)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s run timed out", engine)
	}
	return stdout.String(), stderr.String(), err
}

func TestPhaseV1231_AsyncAwaitBothEngines(t *testing.T) {
	path := v1226Source(t, "asyncawait",
		"func double(x) { return x * 2 }\nfunc main() { let t = async double(21); print(await t) }\n")
	for _, eng := range []string{"go", "kcc"} {
		out, errText, err := v1231Run(t, eng, path)
		if err != nil {
			t.Fatalf("%s run failed: %v\nstdout=%q\nstderr=%q", eng, err, out, errText)
		}
		if got := v1226CNormalize(out); got != "42\n" {
			t.Errorf("%s stdout = %q, want 42", eng, got)
		}
	}
}
