// v1.2.26 slice 6 (S6) - generics type-argument inference on both engines.
// v1 generics require explicit [T] at call sites; S6 infers T when the
// parameter's declared type is the type parameter and the corresponding
// argument is a literal or an annotated binding. Both engines use the same
// narrow rule: an unannotated binding still produces the explicit-args
// diagnostic (K002 on Go, K115 on kcc).
package cli

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func v1226DRun(t *testing.T, engine, path string) (string, string, error) {
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

func TestPhaseV1226D_InferredTypeArguments(t *testing.T) {
	path := v1226Source(t, "infer", "func id[T](x T) { return x }\nfunc main() { print(id(42)); print(id(\"hi\")) }\n")
	const want = "42\nhi\n"
	var first string
	for i, eng := range []string{"go", "kcc"} {
		out, errText, err := v1226DRun(t, eng, path)
		if err != nil {
			t.Fatalf("%s run failed: %v\nstdout=%q\nstderr=%q", eng, err, out, errText)
		}
		got := v1226CNormalize(out)
		if i == 0 {
			first = got
		} else if got != first {
			t.Errorf("engine stdout differs: go=%q %s=%q", first, eng, got)
		}
		if got != want {
			t.Errorf("%s stdout = %q, want %q", eng, got, want)
		}
	}
}

func TestPhaseV1226D_UnannotatedBindingStillNeedsExplicitArgs(t *testing.T) {
	path := v1226Source(t, "noinfer", "func id[T](x T) { return x }\nfunc main() { let a = 2; print(id(a)) }\n")
	for _, eng := range []string{"go", "kcc"} {
		out, errText, err := v1226DRun(t, eng, path)
		if err == nil {
			t.Errorf("%s accepted id(a) with an unannotated binding; stdout=%q", eng, out)
		}
		if combined := out + errText; !strings.Contains(combined, "requires explicit type arguments") {
			t.Errorf("%s did not name the explicit-args requirement: %q", eng, combined)
		}
	}
}

func TestPhaseV1226D_ExplicitTypeArgsStillWork(t *testing.T) {
	path := v1226Source(t, "explicit", "func id[T](x T) { return x }\nfunc main() { print(id[int](7)) }\n")
	for _, eng := range []string{"go", "kcc"} {
		out, errText, err := v1226DRun(t, eng, path)
		if err != nil {
			t.Fatalf("%s run failed: %v\nstdout=%q\nstderr=%q", eng, err, out, errText)
		}
		if got := v1226CNormalize(out); got != "7\n" {
			t.Errorf("%s stdout = %q, want 7", eng, got)
		}
	}
}
