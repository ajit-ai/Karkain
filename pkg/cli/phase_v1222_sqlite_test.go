// v1.2.22 - real SQLite driver on both engines.
//
// The vendored SQLite amalgamation is compiled as a separate C source by the
// CLI. All databases are local (":memory:" or t.TempDir()); no external server.
// Native/WASM remain excluded by existing FFI/socket boundaries.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var v1222Once sync.Once
var v1222Bin string
var v1222Err error

func v1222Prepare(t *testing.T) string {
	t.Helper()
	v1222Once.Do(func() {
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
		v1222Err = buildKarkainErr(t, &v1222Bin)
	})
	if v1222Err != nil {
		t.Fatalf("building karkain: %v", v1222Err)
	}
	return v1222Bin
}

func v1222Source(t *testing.T, name, body string) string {
	t.Helper()
	dir, err := os.MkdirTemp(repoRoot(t), "v1222")
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

func v1222Run(t *testing.T, engine, path string) string {
	t.Helper()
	karkain := v1222Prepare(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
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

func v1222Lines(out string) []string {
	return strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
}

const v1222MemoryProgram = `import std.db
func main() {
    let h = db_sqlite_open(":memory:")
    print(h > 0)
    print(db_sqlite_exec(h, "CREATE TABLE t (id INTEGER, name TEXT)"))
    print(db_sqlite_exec(h, "INSERT INTO t VALUES (1, 'alice')"))
    let rows = db_sqlite_query(h, "SELECT id, name FROM t")
    print(len(rows))
    print(rows[0][0])
    print(rows[0][1])
    print(db_sqlite_close(h))
    print(db_sqlite_error())
}
`

// TestPhaseV1222_InMemoryBothEngines proves a real SQLite CREATE/INSERT/SELECT
// round trip through the vendored amalgamation on both front ends.
func TestPhaseV1222_InMemoryBothEngines(t *testing.T) {
	path := v1222Source(t, "memory", v1222MemoryProgram)
	var first string
	for i, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out := v1222Run(t, engine, path)
			if i == 0 {
				first = out
			} else if out != first {
				t.Errorf("engine stdout differs:\ngo:\n%s\n%s:\n%s", first, engine, out)
			}
			lines := v1222Lines(out)
			want := []string{"1", "0", "0", "1", "1", "alice", "0", ""}
			if len(lines) < len(want) {
				t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(want), out)
			}
			for j, w := range want {
				if strings.TrimSpace(lines[j]) != w {
					t.Errorf("line %d = %q, want %q", j, lines[j], w)
				}
			}
		})
	}
}

func v1222KarkString(s string) string {
	s = filepath.ToSlash(s)
	return strings.ReplaceAll(s, `\`, `\\`)
}

// TestPhaseV1222_FileReopen proves persistence through a temp-file database.
func TestPhaseV1222_FileReopen(t *testing.T) {
	dir, err := os.MkdirTemp(repoRoot(t), "v1222file")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			// Each engine gets its own file so a prior CREATE TABLE cannot
			// turn the second engine into an insert-only run.
			dbPath := filepath.Join(dir, engine+".db")
			src := fmt.Sprintf(`import std.db
func main() {
    let p = "%s"
    let h = db_sqlite_open(p)
    print(h > 0)
    print(db_sqlite_exec(h, "CREATE TABLE t (name TEXT)"))
    print(db_sqlite_exec(h, "INSERT INTO t VALUES ('persist')"))
    print(db_sqlite_close(h))
    let h2 = db_sqlite_open(p)
    let rows = db_sqlite_query(h2, "SELECT name FROM t")
    print(len(rows))
    print(rows[0][0])
    print(db_sqlite_close(h2))
    print(db_sqlite_error())
}
`, v1222KarkString(dbPath))
			path := v1222Source(t, "file", src)
			out := v1222Run(t, engine, path)
			lines := v1222Lines(out)
			want := []string{"1", "0", "0", "0", "1", "persist", "0", ""}
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

// TestPhaseV1222_DeterministicErrors pins sqlite:-prefixed diagnostics.
func TestPhaseV1222_DeterministicErrors(t *testing.T) {
	path := v1222Source(t, "errors", `import std.db
func main() {
    let h = db_sqlite_open(":memory:")
    print(db_sqlite_exec(h, "SELECT bad"))
    print(db_sqlite_error())
    print(len(db_sqlite_query(h, "SELECT * FROM missing")))
    print(db_sqlite_error())
    print(db_sqlite_close(99))
    print(db_sqlite_error())
}
`)
	var first string
	for i, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out := v1222Run(t, engine, path)
			if i == 0 {
				first = out
			} else if out != first {
				t.Errorf("engine error text differs:\n%s", out)
			}
			lines := v1222Lines(out)
			if len(lines) < 6 {
				t.Fatalf("got %d lines:\n%s", len(lines), out)
			}
			if strings.TrimSpace(lines[0]) == "0" {
				t.Errorf("bad SQL unexpectedly succeeded:\n%s", out)
			}
			if !strings.HasPrefix(strings.TrimSpace(lines[1]), "sqlite:") {
				t.Errorf("missing sqlite: error:\n%s", out)
			}
			if strings.TrimSpace(lines[2]) != "0" {
				t.Errorf("failed query must return empty array:\n%s", out)
			}
			if !strings.HasPrefix(strings.TrimSpace(lines[3]), "sqlite:") {
				t.Errorf("missing query error:\n%s", out)
			}
			if strings.TrimSpace(lines[4]) != "-1" {
				t.Errorf("invalid close = %q, want -1", lines[4])
			}
			if !strings.HasPrefix(strings.TrimSpace(lines[5]), "sqlite:") {
				t.Errorf("missing close error:\n%s", out)
			}
		})
	}
}

// TestPhaseV1222_RuntimeWiring proves a DB-free program emits no binding.
func TestPhaseV1222_RuntimeWiring(t *testing.T) {
	karkain := v1222Prepare(t)
	dir, err := os.MkdirTemp(repoRoot(t), "v1222wire")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "plain.kark")
	outC := filepath.Join(dir, "plain.c")
	if err := os.WriteFile(path, []byte("func main() { print(\"plain\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "build", path, "--engine", "go", "--target", "c23", "-o", outC)
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, string(msg))
	}
	data, err := os.ReadFile(outC)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "karkain_sqlite_open(") {
		t.Errorf("DB-free program emitted SQLite binding")
	}
}

// TestPhaseV1222_NativeRefused pins the C23-only boundary.
func TestPhaseV1222_NativeRefused(t *testing.T) {
	karkain := v1222Prepare(t)
	path := v1222Source(t, "native", `func main() { let h = sqlite_open(":memory:") }`)
	out := filepath.Join(filepath.Dir(path), "native.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "build", path, "--engine", "go",
		"--target", "native-x86_64-windows", "-o", out)
	msg, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("native build unexpectedly succeeded:\n%s", string(msg))
	}
	if !strings.Contains(string(msg), "sqlite_") {
		t.Errorf("refusal must name a sqlite_* builtin:\n%s", string(msg))
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Errorf("native refusal should not write an image")
	}
}
