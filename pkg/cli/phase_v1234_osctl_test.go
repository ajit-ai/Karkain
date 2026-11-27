// v1.2.34 takeover: portable OS/process control builtins.
//
// The inherited WIP had the C runtime and generated mirror but no builtin
// registration on either engine, so stdlib wrappers were undefined (Go K002 /
// kcc K101) and would have recursed if registered under their public names.
// This gate drives the real CLI on both engines with an internal osctl_ name
// behind unchanged public stdlib wrappers.
package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const v1234Program = `import std.env
import std.filesystem
func main() {
    let s = env_set("KARKAIN_V1234", "alive")
    print(s)
    print(env_get("KARKAIN_V1234"))
    let p = "V1234_OSCTL_DIR"
    print(fs_mkdir(p))
    print(fs_exists(p))
    print(fs_is_dir(p))
    print(fs_remove(p))
    print(fs_exists(p))
}
`

const v1234Want = "0\nalive\n0\n1\n1\n0\n0\n"

func v1234Source(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(repoRoot(t), "v1234")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(path, []byte(v1234Program), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func v1234Run(t *testing.T, engine string, args ...string) (string, string, int) {
	t.Helper()
	cli, _ := a1Prepare(t) // fresh CLI and fresh kcc from the working tree
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, args...)
	cmd.Env = append(os.Environ(), "KARKAIN_KCC=", "KARKAIN_ENGINE="+engine)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s timed out", args)
	}
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("%v: %v", args, err)
		}
	}
	return out.String(), errOut.String(), code
}

// v1234WithKCC sets KARKAIN_KCC for a kcc-engine invocation. a1Prepare builds
// the binary once; its path is needed explicitly because v1234Run clears the
// inherited value.
func v1234WithKCC(t *testing.T, path string, args ...string) (string, string, int) {
	t.Helper()
	cli, kcc := a1Prepare(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, args...)
	cmd.Env = append(os.Environ(), "KARKAIN_KCC="+kcc, "KARKAIN_ENGINE=kcc")
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s timed out", args)
	}
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("%v: %v", args, err)
		}
	}
	return out.String(), errOut.String(), code
}

func TestPhaseV1234_OsctlBothEngines(t *testing.T) {
	path := v1234Source(t)
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(filepath.Dir(path), "V1234_OSCTL_DIR")) })

	goOut, goErr, goCode := v1234Run(t, "go", "run", path, "--engine", "go")
	if goCode != 0 {
		t.Fatalf("go engine exit = %d\nstdout:\n%s\nstderr:\n%s", goCode, goOut, goErr)
	}
	kccOut, kccErr, kccCode := v1234WithKCC(t, path, "run", path, "--engine", "kcc")
	if kccCode != 0 {
		t.Fatalf("kcc engine exit = %d\nstdout:\n%s\nstderr:\n%s", kccCode, kccOut, kccErr)
	}
	norm := func(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }
	if got := norm(goOut); got != v1234Want {
		t.Errorf("go stdout = %q, want %q", got, v1234Want)
	}
	if got := norm(kccOut); got != v1234Want {
		t.Errorf("kcc stdout = %q, want %q", got, v1234Want)
	}
}

func TestPhaseV1234_NativeRefusedByBothEngines(t *testing.T) {
	path := v1234Source(t)
	// Use a target the CURRENT host can execute: otherwise the CLI's
	// cross-run guard refuses first with a build-only hint (measured on the
	// Linux CI leg) and the native backend is never asked about osctl_*.
	target := a1HostTarget()
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			var out, errOut string
			var code int
			if engine == "kcc" {
				out, errOut, code = v1234WithKCC(t, path, "run", path, "--engine", "kcc", "--target", target)
			} else {
				out, errOut, code = v1234Run(t, "go", "run", path, "--engine", "go", "--target", target)
			}
			if code == 0 {
				t.Errorf("%s native path accepted osctl builtins\nstdout:\n%s", engine, out)
			}
			if !strings.Contains(out+errOut, "K145") && !strings.Contains(out+errOut, "K108") {
				t.Errorf("%s native refusal did not name K145/K108:\nstdout:\n%s\nstderr:\n%s", engine, out, errOut)
			}
		})
	}
}
