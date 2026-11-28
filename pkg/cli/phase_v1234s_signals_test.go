// v1.2.34 follow-up: portable signal control. The C runtime, generated kcc
// mirror, both-engine builtin tables and std.process wrappers must agree.
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

const v1234sProgram = `import std.process
func main() {
    print(proc_ignore_signal(2))
    print(proc_raise_signal(2))
    print(proc_default_signal(2))
}
`

const v1234sInvalid = `import std.process
func main() {
    print(proc_ignore_signal(9))
}
`

func v1234sSource(t *testing.T, name, body string) string {
	t.Helper()
	dir, err := os.MkdirTemp(repoRoot(t), "v1234s")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, name+".kark")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func v1234sRun(t *testing.T, engine, src string, target ...string) (string, string, int) {
	t.Helper()
	cliBin, kccBin := a1Prepare(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	args := []string{"run", src, "--engine", engine}
	if len(target) > 0 {
		args = append(args, "--target", target[0])
	}
	cmd := exec.CommandContext(ctx, cliBin, args...)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine, "KARKAIN_KCC="+kccBin)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s timed out", engine)
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

func TestPhaseV1234S_SignalControlBothEngines(t *testing.T) {
	path := v1234sSource(t, "signals", v1234sProgram)
	var first string
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out, errOut, code := v1234sRun(t, engine, path)
			if code != 0 {
				t.Fatalf("%s exit = %d\nstdout:\n%s\nstderr:\n%s", engine, code, out, errOut)
			}
			got := strings.ReplaceAll(out, "\r\n", "\n")
			if got != "0\n0\n0\n" {
				t.Errorf("%s stdout = %q, want %q", engine, got, "0\n0\n0\n")
			}
			if first == "" {
				first = got
			} else if got != first {
				t.Errorf("engine stdout differs: %q vs %q", first, got)
			}
		})
	}
}

func TestPhaseV1234S_UnsupportedSignalIsNotSilentlyAccepted(t *testing.T) {
	path := v1234sSource(t, "bad_signal", v1234sInvalid)
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out, _, code := v1234sRun(t, engine, path)
			if code != 0 {
				t.Fatalf("%s exit = %d, want 0 (runtime reports -1)", engine, code)
			}
			if got := strings.ReplaceAll(out, "\r\n", "\n"); got != "-1\n" {
				t.Errorf("%s stdout = %q, want %q", engine, got, "-1\n")
			}
		})
	}
}

func TestPhaseV1234S_NativeRefusedByBothEngines(t *testing.T) {
	path := v1234sSource(t, "native_refusal", v1234sProgram)
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out, errOut, code := v1234sRun(t, engine, path, a1HostTarget())
			if code == 0 {
				t.Errorf("%s native accepted signal builtins: %q", engine, out)
			}
			all := out + errOut
			if !strings.Contains(all, "K145") && !strings.Contains(all, "K108") {
				t.Errorf("%s native refusal did not name K145/K108: %s", engine, all)
			}
		})
	}
}
