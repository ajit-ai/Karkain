// v1.2.24 slice 2 - RC builtins on both engines.
package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var v1224bOnce sync.Once
var v1224bBin string
var v1224bErr error

func v1224bPrepare(t *testing.T) string {
	t.Helper()
	v1224bOnce.Do(func() {
		for _, dir := range []string{
			`C:\Program Files\Git\cmd`,
			`C:\Windows\System32\WindowsPowerShell\v1.0`,
			`C:\msys64\mingw64\bin`,
		} {
			if st, err := os.Stat(dir); err == nil && st.IsDir() {
				parts := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
				found := false
				for _, p := range parts {
					if strings.EqualFold(p, dir) {
						found = true
						break
					}
				}
				if !found {
					_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
				}
			}
		}
		v1224bErr = buildKarkainErr(t, &v1224bBin)
	})
	if v1224bErr != nil {
		t.Fatalf("building karkain: %v", v1224bErr)
	}
	return v1224bBin
}

func v1224bSource(t *testing.T, name, body string) string {
	t.Helper()
	dir, err := os.MkdirTemp(repoRoot(t), "v1224b")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, name+".kark")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func v1224bRun(t *testing.T, engine, path string) string {
	t.Helper()
	karkain := v1224bPrepare(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "run", path, "--engine", engine)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s timed out: %v\nstdout:\n%s\nstderr:\n%s", engine, ctx.Err(), stdout.String(), stderr.String())
	}
	if err != nil {
		t.Fatalf("%s failed: %v\nstdout:\n%s\nstderr:\n%s", engine, err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

const v1224bProgram = `func main() {
    let b = rc_new(42)
    print(rc_get(b))
    print(rc_strong_count(b))
    print(rc_retain(b))
    print(rc_strong_count(b))
    print(rc_release(b))
    print(rc_release(b))
}
`

func TestPhaseV1224B_RCBothEngines(t *testing.T) {
	path := v1224bSource(t, "rc", v1224bProgram)
	var first string
	for i, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out := v1224bRun(t, engine, path)
			if i == 0 {
				first = out
			} else if strings.TrimSpace(out) != strings.TrimSpace(first) {
				t.Errorf("engine stdout differs:\ngo:\n%s\n%s:\n%s", first, engine, out)
			}
			lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
			want := []string{"42", "1", "2", "2", "1", "0"}
			if len(lines) < len(want) {
				t.Fatalf("got %d lines:\n%s", len(lines), out)
			}
			for j, w := range want {
				if strings.TrimSpace(lines[j]) != w {
					t.Errorf("line %d = %q, want %q", j, lines[j], w)
				}
			}
		})
	}
}

// Freshness gate check (Go side).
func TestPhaseV1224B_Freshness(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "src", "compiler", "rc_runtime.kark")); err != nil {
		t.Fatal(err)
	}
}