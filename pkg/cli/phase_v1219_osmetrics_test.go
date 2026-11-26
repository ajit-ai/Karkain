// v1.2.19 - OS metric and process enumeration on both engines (C23 path).
//
// The gate asserts predicates, not host-specific numbers: PIDs, CPU counts,
// memory sizes and uptime differ between runs and machines. It also checks
// that the process list contains the process's own PID, which is the one
// deterministic property of os_process_list().
//
// Native/WASM remain excluded by existing socket/stdlib boundaries.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var v1219Once sync.Once
var v1219Bin string
var v1219Err error

func v1219Prepare(t *testing.T) string {
	t.Helper()
	v1219Once.Do(func() {
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
		v1219Err = buildKarkainErr(t, &v1219Bin)
	})
	if v1219Err != nil {
		t.Fatalf("building karkain: %v", v1219Err)
	}
	return v1219Bin
}

func v1219Source(t *testing.T, name, src string) string {
	t.Helper()
	dir, err := os.MkdirTemp(repoRoot(t), "v1219")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, name+".kark")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

const v1219MetricsProgram = `import std.os

func main() {
    print(os_current_pid())
    print(os_cpu_total())
    print(os_memory_total_mb())
    print(os_memory_available_mb())
    print(os_uptime())
    print(len(os_processes()))
    let list = os_processes()
    let self = str(os_current_pid()) + ":"
    let found = 0
    let i = 0
    while (i < len(list)) {
        if (contains(list[i], self)) {
            found = 1
        }
        i = i + 1
    }
    print(found)
    print(os_error())
}
`

func v1219Run(t *testing.T, engine, path string) string {
	t.Helper()
	karkain := v1219Prepare(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "run", path, "--engine", engine)
	// Keep compiler diagnostics separate from program stdout. On Linux CI the
	// generated C emits a warning; CombinedOutput would mix that warning into
	// the metric lines and turn a compiler message into a false gate failure.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s run timed out: %v\nstdout:\n%s\nstderr:\n%s", engine, ctx.Err(), stdout.String(), stderr.String())
	}
	if err != nil {
		t.Fatalf("%s run failed: %v\nstdout:\n%s\nstderr:\n%s", engine, err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func v1219Int(t *testing.T, line string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(strings.TrimSpace(line), 10, 64)
	if err != nil {
		t.Fatalf("not an integer %q: %v", line, err)
	}
	return n
}

// TestPhaseV1219_MetricsBothEngines proves real OS values and a self-containing
// process list on both front ends. Exact values are deliberately not golden.
func TestPhaseV1219_MetricsBothEngines(t *testing.T) {
	path := v1219Source(t, "metrics", v1219MetricsProgram)
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out := v1219Run(t, engine, path)
			// Linux CI emits gcc warnings through the CLI's inherited stdio
			// before the program's eight metric lines. Find the first run of
			// seven consecutive integers; the eighth line is os_error() and
			// may legitimately be empty, so strings.Fields cannot be used.
			raw := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
			start := -1
			for i := 0; i+7 < len(raw); i++ {
				ok := true
				for j := 0; j < 7; j++ {
					if _, err := strconv.ParseInt(strings.TrimSpace(raw[i+j]), 10, 64); err != nil {
						ok = false
						break
					}
				}
				if ok {
					start = i
					break
				}
			}
			if start < 0 {
				t.Fatalf("could not locate the 8 metric lines in output:\n%s", out)
			}
			lines := raw[start : start+8]
			pid := v1219Int(t, lines[0])
			cpus := v1219Int(t, lines[1])
			total := v1219Int(t, lines[2])
			avail := v1219Int(t, lines[3])
			uptime := v1219Int(t, lines[4])
			count := v1219Int(t, lines[5])
			found := v1219Int(t, lines[6])
			if pid <= 0 {
				t.Errorf("pid = %d, want > 0", pid)
			}
			if cpus < 1 {
				t.Errorf("cpu count = %d, want >= 1", cpus)
			}
			if total <= 0 {
				t.Errorf("total memory = %d, want > 0", total)
			}
			if avail < 0 || avail > total {
				t.Errorf("available memory %d outside [0,%d]", avail, total)
			}
			if uptime <= 0 {
				t.Errorf("uptime = %d, want > 0", uptime)
			}
			if count <= 0 || count > 64 {
				t.Errorf("process list length = %d, want 1..64", count)
			}
			if found != 1 {
				t.Errorf("process list does not contain own pid %d:\n%s", pid, out)
			}
			if strings.TrimSpace(lines[7]) != "" {
				t.Errorf("os_error() = %q, want empty:\n%s", lines[7], out)
			}
		})
	}
}

// TestPhaseV1219_RuntimeWiring pins that a metric-free program does not emit
// the OS runtime. Importing std.os is enough to pull it in, because the module
// wrappers themselves call os_* builtins.
func TestPhaseV1219_RuntimeWiring(t *testing.T) {
	karkain := v1219Prepare(t)
	dir, err := os.MkdirTemp(repoRoot(t), "v1219wire")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	cases := map[string]string{
		"metric": v1219MetricsProgram,
		"plain":  "func main() { print(\"plain\") }\n",
	}
	for name, src := range cases {
		path := filepath.Join(dir, name+".kark")
		out := filepath.Join(dir, name+".c")
		if err := os.WriteFile(path, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		cmd := exec.CommandContext(ctx, karkain, "build", path, "--engine", "go", "--target", "c23", "-o", out)
		if msg, err := cmd.CombinedOutput(); err != nil {
			cancel()
			t.Fatalf("%s c23 build failed: %v\n%s", name, err, string(msg))
		}
		cancel()
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		hasRuntime := strings.Contains(string(data), "karkain_os_pid(")
		if name == "metric" && !hasRuntime {
			t.Errorf("metric program did not emit the OS runtime")
		}
		if name == "plain" && hasRuntime {
			t.Errorf("metric-free program emitted the OS runtime")
		}
	}
}

// TestPhaseV1219_NativeRefused pins the platform boundary: the C-free native
// target has no OS process/metric syscalls and must refuse by name with no
// image, rather than silently mis-lowering the builtin.
func TestPhaseV1219_NativeRefused(t *testing.T) {
	karkain := v1219Prepare(t)
	src := "func main() { print(os_pid()) }\n"
	path := v1219Source(t, "native", fmt.Sprintf("%s", src))
	out := filepath.Join(filepath.Dir(path), "native.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "build", path, "--engine", "go",
		"--target", "native-x86_64-windows", "-o", out)
	msg, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("native build unexpectedly succeeded:\n%s", string(msg))
	}
	if !strings.Contains(string(msg), "os_pid") {
		t.Errorf("refusal must name the missing builtin, got:\n%s", string(msg))
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Errorf("native refusal should not write an image")
	}
}
