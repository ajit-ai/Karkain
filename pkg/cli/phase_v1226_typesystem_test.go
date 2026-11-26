// v1.2.26 slice 1 (S1) - type system foundation: mutability tracking in AST.
//
// Additive, non-breaking: `let mut x = ...` records Mut=true on VarDeclStmt;
// `let x = ...` keeps Mut=false. Existing programs keep compiling and keep
// their meaning. Type-value foundation (pkg/sema.Type) ships alongside as
// scaffolding; full inference/interfaces/const folding are later slices.
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

var v1226Once sync.Once
var v1226Bin string
var v1226Err error

func v1226Prepare(t *testing.T) string {
	t.Helper()
	v1226Once.Do(func() {
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
		v1226Err = buildKarkainErr(t, &v1226Bin)
	})
	if v1226Err != nil {
		t.Fatalf("building karkain: %v", v1226Err)
	}
	return v1226Bin
}

func v1226Source(t *testing.T, name, body string) string {
	t.Helper()
	dir, err := os.MkdirTemp(repoRoot(t), "v1226")
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

func v1226Run(t *testing.T, engine, path string) string {
	t.Helper()
	karkain := v1226Prepare(t)
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

const v1226MutProgram = `func main() {
    let mut x = 5
    x = 6
    print(x)
}
`

const v1226PlainProgram = `func main() {
    let x = 5
    x = 6
    print(x)
}
`

func TestPhaseV1226_MutBothEngines(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"mut", v1226MutProgram},
		{"plain", v1226PlainProgram},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := v1226Source(t, tc.name, tc.src)
			var first string
			for i, engine := range []string{"go", "kcc"} {
				t.Run(engine, func(t *testing.T) {
					out := v1226Run(t, engine, path)
					if i == 0 {
						first = out
					} else if strings.TrimSpace(out) != strings.TrimSpace(first) {
						t.Errorf("engine stdout differs:\ngo:\n%s\n%s:\n%s", first, engine, out)
					}
					if strings.TrimSpace(out) != "6" {
						t.Errorf("expected 6, got %q", strings.TrimSpace(out))
					}
				})
			}
		})
	}
}