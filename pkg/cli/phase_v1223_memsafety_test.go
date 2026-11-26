// v1.2.23 - memory safety Phase 1: linear types + K160 on both engines.
//
// A `linear type` binding must be consumed exactly once. The Go borrow checker
// enforces this per binding/per scope; the self-hosted (kcc) checker mirrors
// the rule and emits the same K160 diagnostic. Diagnostics surface through
// `karkain build` (the borrow checker runs on build/run/dbg, not on check).
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

var v1223Once sync.Once
var v1223Bin string
var v1223Err error

func v1223Prepare(t *testing.T) string {
	t.Helper()
	v1223Once.Do(func() {
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
		v1223Err = buildKarkainErr(t, &v1223Bin)
	})
	if v1223Err != nil {
		t.Fatalf("building karkain: %v", v1223Err)
	}
	return v1223Bin
}

func v1223Source(t *testing.T, name, body string) string {
	t.Helper()
	dir, err := os.MkdirTemp(repoRoot(t), "v1223")
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

// v1223Build runs `karkain build` for the given engine and returns exit code,
// stdout and stderr.
func v1223Build(t *testing.T, engine, path string) (int, string, string) {
	t.Helper()
	karkain := v1223Prepare(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "build", path, "--engine", engine)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}
	if ctx.Err() != nil {
		t.Fatalf("%s timed out: %v\nstdout:\n%s\nstderr:\n%s", engine, ctx.Err(), stdout.String(), stderr.String())
	}
	return code, stdout.String(), stderr.String()
}

func v1223Combined(code int, stdout, stderr string) string {
	return stdout + "\n" + stderr
}

const v1223LinearType = `linear type Handle { id int }
`

// TestPhaseV1223_UsedExactlyOnce passes on both engines.
func TestPhaseV1223_UsedExactlyOnce(t *testing.T) {
	src := v1223LinearType + `func main() {
    let h = Handle{id: 7}
    print(h.id)
}
`
	path := v1223Source(t, "once", src)
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			code, stdout, stderr := v1223Build(t, engine, path)
			if code != 0 {
				t.Fatalf("expected success, got exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			if !strings.Contains(stdout+stderr, "[ok]") && !strings.Contains(stdout+stderr, "Build successful") {
				t.Fatalf("no success marker\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
			}
		})
	}
}

// TestPhaseV1223_UsedTwice is rejected with K160 on both engines.
func TestPhaseV1223_UsedTwice(t *testing.T) {
	src := v1223LinearType + `func main() {
    let h = Handle{id: 7}
    print(h.id)
    print(h.id)
}
`
	path := v1223Source(t, "twice", src)
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			code, stdout, stderr := v1223Build(t, engine, path)
			if code == 0 {
				t.Fatalf("expected failure, got success\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
			}
			out := v1223Combined(code, stdout, stderr)
			if !strings.Contains(out, "K160") {
				t.Fatalf("expected K160, got:\n%s", out)
			}
		})
	}
}

// TestPhaseV1223_NeverUsed is rejected with K160 on both engines.
func TestPhaseV1223_NeverUsed(t *testing.T) {
	src := v1223LinearType + `func main() {
    let h = Handle{id: 7}
}
`
	path := v1223Source(t, "never", src)
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			code, stdout, stderr := v1223Build(t, engine, path)
			if code == 0 {
				t.Fatalf("expected failure, got success\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
			}
			out := v1223Combined(code, stdout, stderr)
			if !strings.Contains(out, "K160") {
				t.Fatalf("expected K160, got:\n%s", out)
			}
		})
	}
}

// TestPhaseV1223_NonMainLocal exercises a linear binding in a non-main function.
func TestPhaseV1223_NonMainLocal(t *testing.T) {
	src := v1223LinearType + `func f() {
    let h = Handle{id: 7}
    print(h.id)
    print(h.id)
}
func main() { f() }
`
	path := v1223Source(t, "nonmain", src)
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			code, stdout, stderr := v1223Build(t, engine, path)
			if code == 0 {
				t.Fatalf("expected failure, got success\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
			}
			out := v1223Combined(code, stdout, stderr)
			if !strings.Contains(out, "K160") {
				t.Fatalf("expected K160, got:\n%s", out)
			}
		})
	}
}

// TestPhaseV1223_MoveConsumes exercises move as consumption (Go engine;
// kcc has no move codegen yet, so it is verified on Go only).
func TestPhaseV1223_MoveConsumes(t *testing.T) {
	src := v1223LinearType + `func main() {
    let h = Handle{id: 7}
    let h2 = move(h)
    print(h.id)
    print(h2.id)
}
`
	path := v1223Source(t, "move", src)
	for _, engine := range []string{"go"} {
		t.Run(engine, func(t *testing.T) {
			code, stdout, stderr := v1223Build(t, engine, path)
			if code == 0 {
				t.Fatalf("expected failure (use after move), got success\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
			}
		})
	}
}

// TestPhaseV1223_MoveOnce passes on the Go engine.
func TestPhaseV1223_MoveOnce(t *testing.T) {
	src := v1223LinearType + `func main() {
    let h = Handle{id: 7}
    let h2 = move(h)
    print(h2.id)
}
`
	path := v1223Source(t, "moveonce", src)
	for _, engine := range []string{"go"} {
		t.Run(engine, func(t *testing.T) {
			code, stdout, stderr := v1223Build(t, engine, path)
			if code != 0 {
				t.Fatalf("expected success, got exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
		})
	}
}

// TestPhaseV1223_OptionMatchExhaustive passes on both engines.
func TestPhaseV1223_OptionMatchExhaustive(t *testing.T) {
	src := `func main() {
    let o = Some(42)
    match o {
        Some(v) => { print(v) }
        None => { print(0) }
    }
}
`
	path := v1223Source(t, "opt", src)
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			code, stdout, stderr := v1223Build(t, engine, path)
			if code != 0 {
				t.Fatalf("expected success, got exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
		})
	}
}

// TestPhaseV1223_OptionMatchNonExhaustive rejected on the Go engine (kcc
// does not yet perform exhaustiveness checking).
func TestPhaseV1223_OptionMatchNonExhaustive(t *testing.T) {
	src := `func main() {
    let o = Some(42)
    match o {
        Some(v) => { print(v) }
    }
}
`
	path := v1223Source(t, "optnex", src)
	for _, engine := range []string{"go"} {
		t.Run(engine, func(t *testing.T) {
			code, stdout, stderr := v1223Build(t, engine, path)
			if code == 0 {
				t.Fatalf("expected failure, got success\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
			}
			out := v1223Combined(code, stdout, stderr)
			if !strings.Contains(out, "non-exhaustive match on Option") {
				t.Fatalf("expected non-exhaustive diagnostic, got:\n%s", out)
			}
		})
	}
}


