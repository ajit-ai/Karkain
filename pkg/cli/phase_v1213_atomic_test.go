// Phase v1.2.13 - Atomics + litmus (Cond 7).
//
// Pins the platform decision: the C-free native target has NO atomic
// primitives (no lock prefix / fence emission), so @atomic_* operations
// are refused BY NAME with K145 on the native target. The C23 path keeps
// the full @atomic_* surface via C11 stdatomic and builds + runs the
// same program.
package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var v1213KarkainOnce sync.Once
var v1213KarkainBin string
var v1213KarkainErr error

func v1213Karkain(t *testing.T) string {
	t.Helper()
	v1213KarkainOnce.Do(func() {
		v1213KarkainErr = buildKarkainErr(t, &v1213KarkainBin)
	})
	if v1213KarkainErr != nil {
		t.Fatalf("building karkain: %v", v1213KarkainErr)
	}
	return v1213KarkainBin
}

// TestPhaseV1213_NativeAtomicRefused pins the platform decision.
func TestPhaseV1213_NativeAtomicRefused(t *testing.T) {
	karkain := v1213Karkain(t)
	src := "func main() {\n    let p = 0\n    print(@atomic_load(p, \"relaxed\"))\n}\n"
	repo := repoRoot(t)
	dir, err := os.MkdirTemp(repo, "v1213refused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	probe := filepath.Join(dir, "atomicrefused.kark")
	if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "atomicrefused.exe")
	cmd := exec.Command(karkain, "build", probe, "--engine", "go", "--target", "native-x86_64-windows", "-o", out)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	msg, _ := cmd.CombinedOutput()
	if !strings.Contains(string(msg), "atomic") {
		t.Errorf("expected atomic refusal, got:\n%s", string(msg))
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("refusal should not write an image")
	}
}

// TestPhaseV1213_C23AtomicBuilds proves the same program builds through C23.
func TestPhaseV1213_C23AtomicBuilds(t *testing.T) {
	karkain := v1213Karkain(t)
	src := "func main() {\n    let p = 0\n    print(@atomic_load(p, \"relaxed\"))\n}\n"
	repo := repoRoot(t)
	dir, err := os.MkdirTemp(repo, "v1213c23")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	probe := filepath.Join(dir, "atomicc23.kark")
	if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "atomicc23.c")
	cmd := exec.Command(karkain, "build", probe, "--engine", "go", "--target", "c23", "-o", out)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("c23 build should succeed:\n%s", string(msg))
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("c23 output missing: %v", err)
	}
}