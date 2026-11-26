// v1.2.25 slice 2 - arena builtins on both engines.
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

var v1225bOnce sync.Once
var v1225bBin string
var v1225bErr error

func v1225bPrepare(t *testing.T) string {
	t.Helper()
	v1225bOnce.Do(func() {
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
		v1225bErr = buildKarkainErr(t, &v1225bBin)
	})
	if v1225bErr != nil {
		t.Fatalf("building karkain: %v", v1225bErr)
	}
	return v1225bBin
}

func v1225bSource(t *testing.T, name, body string) string {
	t.Helper()
	dir, err := os.MkdirTemp(repoRoot(t), "v1225b")
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

func v1225bRun(t *testing.T, engine, path string) string {
	t.Helper()
	karkain := v1225bPrepare(t)
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

const v1225bProgram = `func main() {
    let a = arena_new(64)
    let p = arena_alloc(a, 8)
    print(arena_capacity(a))
    print(arena_used(a))
    arena_reset(a)
    print(arena_used(a))
    arena_free(a)
    print(1)
}
`

func TestPhaseV1225B_ArenaBothEngines(t *testing.T) {
	path := v1225bSource(t, "arena", v1225bProgram)
	var first string
	for i, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out := v1225bRun(t, engine, path)
			if i == 0 {
				first = out
			} else if strings.TrimSpace(out) != strings.TrimSpace(first) {
				t.Errorf("engine stdout differs:\ngo:\n%s\n%s:\n%s", first, engine, out)
			}
			lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
			want := []string{"64", "16", "0", "1"}
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