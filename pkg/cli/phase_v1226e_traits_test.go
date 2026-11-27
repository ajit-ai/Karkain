// v1.2.26 slice 4b first slice (S4b-1): trait/impl declarations on the Go
// engine only, mirroring the generics v1 rollout (146a Go, 146c kcc parity).
// The self-hosted compiler has no trait/impl AST nodes yet, so this gate is
// deliberately Go-only. The boundary is documented, not asserted by driving
// kcc: an earlier attempt to check trait syntax through kcc hung the child
// process pipe (the documented killed-child trap), which is not evidence.
package cli

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func v1226ERun(t *testing.T, engine, path string, args ...string) (string, string, error) {
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

const v1226EGood = "trait Numeric { fn add(x int) int; }\n" +
	"impl Numeric for int { func add(x int) int { return x + 1 } }\n" +
	"func main() { print(add(41)) }\n"

func TestPhaseV1226E_TraitImplGoEngine(t *testing.T) {
	path := v1226Source(t, "traitgood", v1226EGood)
	out, errText, err := v1226ERun(t, "go", path, "run")
	if err != nil {
		t.Fatalf("go run failed: %v\nstdout=%q\nstderr=%q", err, out, errText)
	}
	if got := v1226CNormalize(out); got != "42\n" {
		t.Fatalf("go stdout = %q, want 42", got)
	}
}

func TestPhaseV1226E_MissingMethodRejected(t *testing.T) {
	path := v1226Source(t, "traitbad", "trait Numeric { fn add(x int) int; }\nimpl Numeric for int { }\nfunc main() { print(1) }\n")
	out, errText, err := v1226ERun(t, "go", path, "check")
	if err == nil {
		t.Fatalf("incomplete impl accepted; stdout=%q", out)
	}
	if combined := out + errText; !strings.Contains(combined, "missing method 'add'") {
		t.Fatalf("diagnostic did not name the missing method: %q", combined)
	}
}

