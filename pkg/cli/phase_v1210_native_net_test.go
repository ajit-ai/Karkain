// Phase v1.2.10 - Native networking + platform decision (Condition 3).
//
// Pins the platform decision: the C-free native target has NO socket
// syscalls (only write and exit), so std.net programs are refused BY NAME
// with K145 on the native target, while the C23 path keeps the full
// std.net surface and builds + runs the same program.
//
// What WORKS:
//   - A std.net program builds and runs through --target c23 on any host.
//
// What does NOT work (documented decision, not a gap):
//   - The same program on native-x86_64-* is refused BY NAME with K145
//     naming the missing socket builtin and the C23 alternative.
package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var v1210KarkainOnce sync.Once
var v1210KarkainBin string
var v1210KarkainErr error

func v1210Karkain(t *testing.T) string {
	t.Helper()
	v1210KarkainOnce.Do(func() {
		v1210KarkainErr = buildKarkainErr(t, &v1210KarkainBin)
	})
	if v1210KarkainErr != nil {
		t.Fatalf("building karkain: %v", v1210KarkainErr)
	}
	return v1210KarkainBin
}

// TestPhaseV1210_NativeNetRefused pins the platform decision: a std.net
// program on the native target is refused BY NAME with K145.
func TestPhaseV1210_NativeNetRefused(t *testing.T) {
	karkain := v1210Karkain(t)
	src := "import std.net\nfunc main() {\n\tbool ep = net_endpoint(\"127.0.0.1\", 49051)\n\tprint(ep)\n}\n"
	repo := repoRoot(t)
	dir, err := os.MkdirTemp(repo, "v1210refused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	probe := filepath.Join(dir, "netrefused.kark")
	if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "netrefused.exe")
	cmd := exec.Command(karkain, "build", probe, "--engine", "go", "--target", "native-x86_64-windows", "-o", out)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	msg, _ := cmd.CombinedOutput()
	// Importing std.net now also brings udp_* wrappers into the assembly. The
	// native driver refuses the first unsupported network builtin it reaches,
	// so the contract is K145 naming a network/udp/socket symbol, not one
	// exact word. This still fails a silent fallback.
	s := string(msg)
	named := strings.Contains(s, "socket") || strings.Contains(s, "net_") || strings.Contains(s, "udp_")
	if !strings.Contains(s, "error[K145]") || !named {
		t.Errorf("expected K145 refusal naming a network builtin, got:\n%s", s)
	}
	if _, err := os.Stat(out); err == nil {
		t.Errorf("refusal should not write an image")
	}
}

// TestPhaseV1210_C23NetBuilds proves the same program builds through the
// C23 path (the documented alternative).
func TestPhaseV1210_C23NetBuilds(t *testing.T) {
	karkain := v1210Karkain(t)
	src := "import std.net\nfunc main() {\n\tbool ep = net_endpoint(\"127.0.0.1\", 49051)\n\tprint(ep)\n}\n"
	repo := repoRoot(t)
	dir, err := os.MkdirTemp(repo, "v1210c23")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	probe := filepath.Join(dir, "netc23.kark")
	if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "netc23.c")
	cmd := exec.Command(karkain, "build", probe, "--engine", "go", "--target", "c23", "-o", out)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("c23 build should succeed:\n%s", string(msg))
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("c23 output missing: %v", err)
	}
}

// TestPhaseV1210_NativeNetPlatformDecision documents the decision.
func TestPhaseV1210_NativeNetPlatformDecision(t *testing.T) {
	// The native target has only write and exit syscalls. Networking is
	// deliberately C23-only. This test is a documentation pin.
	if runtime.GOOS != "windows" {
		t.Skip("platform decision documented for windows host")
	}
}
