// v1.2.30 first slice: the new FreeBSD/Android targets are modeled but have
// no guaranteed cross linker. The gate pins the Phase-111 rule: an explicit
// foreign target must fail with a ToolchainError naming what was searched —
// never a silent host fallback and never a wrong-architecture artifact.
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

func v1230Run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, v1226Prepare(t), args...)
	cmd.Env = append(os.Environ(), "CC=")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("command timed out: %v", args)
	}
	return stdout.String(), stderr.String(), err
}

func TestPhaseV1230_ForeignTargetsFailDeterministically(t *testing.T) {
	path := v1226Source(t, "platform", "func main() { print(1) }\n")
	for _, target := range []string{"x86_64-freebsd", "aarch64-android"} {
		t.Run(target, func(t *testing.T) {
			out := filepath.Join(filepath.Dir(path), strings.ReplaceAll(target, "-", "_")+".exe")
			stdout, stderr, err := v1230Run(t, "build", path, "--engine", "go", "--target", target, "-o", out)
			combined := stdout + stderr
			if err == nil {
				t.Fatalf("%s built without a cross toolchain: %q", target, combined)
			}
			if !strings.Contains(combined, "toolchain") && !strings.Contains(combined, "CC=") {
				t.Errorf("%s did not report a missing toolchain: %q", target, combined)
			}
			if _, statErr := os.Stat(out); statErr == nil {
				t.Errorf("%s failure still wrote an artifact", target)
			}
		})
	}
}
