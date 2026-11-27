// v1.2.26 S4b-2: kcc trait/impl parity via the Go-side staging bridge.
//
// The self-hosted parser has no trait/impl AST nodes, so the Go assembler
// lowers trait syntax before kcc sees it: trait declarations become blank
// lines and impl methods are lifted to ordinary top-level functions. The Go
// engine keeps its native declarations and completeness validation. The gate
// pins both-engine observable parity and the bridge's line-preservation.
package cli

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func v1226FRun(t *testing.T, engine, path string, args ...string) (string, string, error) {
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

const v1226FTraitProgram = "trait Numeric { fn add(x int) int; }\n" +
	"impl Numeric for int { func add(x int) int { return x + 1 } }\n" +
	"func main() { print(add(41)) }\n"

func TestPhaseV1226F_TraitParityBothEngines(t *testing.T) {
	path := v1226Source(t, "traitparity", v1226FTraitProgram)
	for _, eng := range []string{"go", "kcc"} {
		out, errText, err := v1226FRun(t, eng, path, "run")
		if err != nil {
			t.Fatalf("%s run failed: %v\nstdout=%q\nstderr=%q", eng, err, out, errText)
		}
		if got := v1226CNormalize(out); got != "42\n" {
			t.Errorf("%s stdout = %q, want 42", eng, got)
		}
	}
}

func TestPhaseV1226F_BridgePreservesLinesAndLiftsMethods(t *testing.T) {
	src := v1226FTraitProgram
	got := stripTraitImplForKCC(src)
	if len(strings.Split(got, "\n")) != len(strings.Split(src, "\n")) {
		t.Fatalf("bridge changed the line count: %q", got)
	}
	if !strings.Contains(got, "func add(x int) int { return x + 1 }") {
		t.Errorf("impl method was not lifted to top level: %q", got)
	}
	if strings.Contains(got, "trait") || strings.Contains(got, "impl Numeric") {
		t.Errorf("trait/impl declarations survived the bridge: %q", got)
	}
}

func TestPhaseV1226F_MissingMethodRejectedOnGoEngine(t *testing.T) {
	path := v1226Source(t, "traitbad", "trait Numeric { fn add(x int) int; }\nimpl Numeric for int { }\nfunc main() { print(1) }\n")
	out, errText, err := v1226FRun(t, "go", path, "check")
	if err == nil {
		t.Fatalf("incomplete impl accepted; stdout=%q", out)
	}
	if combined := out + errText; !strings.Contains(combined, "missing method 'add'") {
		t.Fatalf("diagnostic did not name the missing method: %q", combined)
	}
}
