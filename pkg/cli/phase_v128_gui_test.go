// v1.2.28 first slice - GUI platform decision.
//
// The C-free native target has no windowing backend (its only syscalls are
// write and exit), so gui_available is refused BY NAME with K145 and no image
// is written. The C23 path keeps the std.gui capability surface: the honest
// answer today is false, because no GUI runtime is linked yet. This is a
// capability decision, not a claim that a GUI works.
package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func v128Run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, v1226Prepare(t), args...)
	var out, errText strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errText
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("command timed out: %v", args)
	}
	return out.String(), errText.String(), err
}

func TestPhaseV128_NativeGUIRefused(t *testing.T) {
	path := v1226Source(t, "guirefused", "func main() { print(gui_available()) }\n")
	out := filepath.Join(filepath.Dir(path), "gui.exe")
	stdout, stderr, err := v128Run(t, "build", path, "--engine", "go",
		"--target", "native-x86_64-windows", "-o", out)
	combined := stdout + stderr
	if err == nil {
		t.Fatalf("native target accepted gui_available: %q", combined)
	}
	if !strings.Contains(combined, "K145") || !strings.Contains(combined, "windowing backend") {
		t.Errorf("refusal did not name the GUI capability boundary: %q", combined)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Error("a refused native build must not write an image")
	}
}

func TestPhaseV128_C23CapabilityBothEngines(t *testing.T) {
	path := v1226Source(t, "guic23", "func main() { print(gui_available()) }\n")
	for _, eng := range []string{"go", "kcc"} {
		out, errText, err := v128Run(t, "run", path, "--engine", eng)
		if err != nil {
			t.Fatalf("%s run failed: %v\nstdout=%q\nstderr=%q", eng, err, out, errText)
		}
		if got := v1226CNormalize(out); got != "false\n" {
			t.Errorf("%s stdout = %q, want false", eng, got)
		}
	}
}

func TestPhaseV128_UnknownFunctionNotMisreported(t *testing.T) {
	path := v1226Source(t, "guiunknown", "func main() { print(definitely_not_gui()) }\n")
	out := filepath.Join(filepath.Dir(path), "unknown.exe")
	stdout, stderr, err := v128Run(t, "build", path, "--engine", "go",
		"--target", "native-x86_64-windows", "-o", out)
	combined := stdout + stderr
	if err == nil {
		t.Fatalf("unknown function unexpectedly built: %q", combined)
	}
	if strings.Contains(combined, "windowing backend") || strings.Contains(combined, "gui_available") {
		t.Errorf("unknown function was misreported as a GUI boundary: %q", combined)
	}
}
