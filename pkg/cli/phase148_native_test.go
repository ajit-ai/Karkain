package cli

// Phase 148A — `native-x86_64-linux` CLI surface (Go engine): target
// registration, build-writes-image, run-refusal off Linux, incremental
// refusal, target listing. Execution goldens live in pkg/native
// (Linux-run, structural elsewhere); kcc parity is Phase 151, so every
// subtest below pins --engine go explicitly.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func write148Probe(t *testing.T, name, src string) string {
	t.Helper()
	probe := filepath.Join(t.TempDir(), name+".kark")
	if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
		t.Fatalf("writing probe: %v", err)
	}
	return probe
}

func elfMagic(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading image: %v", err)
	}
	if len(b) < 4 || b[0] != 0x7F || b[1] != 'E' || b[2] != 'L' || b[3] != 'F' {
		t.Fatalf("not an ELF image: %q", path)
	}
}

// TestPhase148_BuildWritesImage proves `build --target
// native-x86_64-linux` routes to the C-free backend (no gcc involved)
// and writes a real ELF image, for straight-line and control-flow
// programs alike. Runs on every host (emission is pure Go).
func TestPhase148_BuildWritesImage(t *testing.T) {
	karkain := phase130Karkain(t)
	straight := write148Probe(t, "straight", "func main() {\n\tprint(40 + 2)\n}\n")
	ctrl := write148Probe(t, "ctrl",
		"func main() {\n\tlet s = 0\n\tlet i = 0\n\twhile (i < 10) {\n\t\ti = i + 1\n\t\ts = s + i\n\t}\n\tprint(s)\n}\n")
	for name, probe := range map[string]string{"straight": straight, "ctrl": ctrl} {
		out := filepath.Join(t.TempDir(), name+".elf")
		cmd := exec.Command(karkain, "build", probe, "--engine", "go", "--target", "native-x86_64-linux", "-o", out)
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
		if msg, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: build should succeed:\n%s", name, string(msg))
		}
		elfMagic(t, out)
	}
}

// TestPhase148_RunRefusedOffLinux pins the honest boundary: running an
// ELF image off linux/amd64 is ExitEnv with the build-only hint (the
// Phase-111 rule), never silent host fallback. Linux runners exercise
// the real execution path in pkg/native instead.
func TestPhase148_RunRefusedOffLinux(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write148Probe(t, "norun", "func main() {\n\tprint(1)\n}\n")
	cmd := exec.Command(karkain, "run", probe, "--engine", "go", "--target", "native-x86_64-linux")
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	out, err := cmd.CombinedOutput()
	msg := string(out)
	if isLinuxAmd64() {
		if err != nil {
			t.Fatalf("on linux/amd64 run should succeed:\n%s", msg)
		}
		if strings.TrimSpace(msg) != "1" {
			t.Errorf("golden mismatch: want 1, got %q", msg)
		}
		return
	}
	if err == nil {
		t.Fatalf("off-Linux run should be refused, but succeeded:\n%s", msg)
	}
	if !strings.Contains(msg, "build only") && !strings.Contains(msg, "cross-run") {
		t.Errorf("want build-only hint, got:\n%s", msg)
	}
}

// TestPhase148_IncrementalRefused pins the documented boundary:
// --incremental serves the C pipeline, so native targets are a loud
// usage refusal (native-split caching is post-148 work).
func TestPhase148_IncrementalRefused(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write148Probe(t, "incr", "func main() {\n\tprint(1)\n}\n")
	cmd := exec.Command(karkain, "build", probe, "--engine", "go", "--target", "native-x86_64-linux", "--incremental")
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("incremental+native should be refused, but succeeded:\n%s", string(out))
	}
	if !strings.Contains(string(out), "--incremental") {
		t.Errorf("refusal must name --incremental, got:\n%s", string(out))
	}
}

// TestPhase148_TargetListed guards the registry contract: ValidateTarget
// accepts, NormalizeTarget passes through, and `karkain target` shows
// the row with its run-requirement note.
func TestPhase148_TargetListed(t *testing.T) {
	if err := ValidateTarget("native-x86_64-linux"); err != nil {
		t.Fatalf("ValidateTarget rejected: %v", err)
	}
	if got, err := NormalizeTarget("native-x86_64-linux"); err != nil || got != "native-x86_64-linux" {
		t.Fatalf("NormalizeTarget = %q, err=%v", got, err)
	}
	karkain := phase130Karkain(t)
	out, err := exec.Command(karkain, "target").CombinedOutput()
	if err != nil {
		t.Fatalf("target failed:\n%s", string(out))
	}
	msg := string(out)
	if !strings.Contains(msg, "native-x86_64-linux") {
		t.Errorf("target listing missing native-x86_64-linux:\n%s", msg)
	}
	if !strings.Contains(msg, "linux/amd64") {
		t.Errorf("target row must state the run requirement:\n%s", msg)
	}
}

// TestPhase148_NativeRejectedAsTriple guards the routing shape: the
// value is an exact-match alias (host, non-triple), never parsed as a
// conventional cross triple.
func TestPhase148_NativeRejectedAsTriple(t *testing.T) {
	if _, isTriple := SelectedTarget("native-x86_64-linux"); isTriple {
		t.Fatalf("native-x86_64-linux must not parse as a cross triple")
	}
}

func isLinuxAmd64() bool {
	return runtime.GOOS == "linux" && runtime.GOARCH == "amd64"
}
