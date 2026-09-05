package cli

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_Workspace_FamilyE2E(t *testing.T) {
	bin := buildKarkain(t)
	// Single-member workspace keeps check/build semantics independent of the
	// cross-member dep resolution path.
	root := buildWorkspaceDir(t, []string{"app"}, map[string][]string{"app": {}})

	exitCode := func(args ...string) int {
		t.Helper()
		_, err := runBin(t, bin, root, args...)
		if err == nil {
			return 0
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		t.Fatalf("unexpected error type: %v", err)
		return -1
	}

	// list
	out, err := runBin(t, bin, root, "workspace", "list")
	if err != nil {
		t.Fatalf("workspace list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "app") || !strings.Contains(out, "[0]") {
		t.Errorf("workspace list output unexpected:\n%s", out)
	}

	// check
	if got := exitCode("workspace", "check"); got != ExitSuccess {
		t.Errorf("workspace check: got %d, want %d", got, ExitSuccess)
	}

	// clean removes a generated artifact (sibling .c of the entry .kark source)
	writeFile(t, filepath.Join(root, "app", "src", "main.c"), "int main(void){}\n")
	if got := exitCode("workspace", "clean"); got != ExitSuccess {
		t.Errorf("workspace clean: got %d, want %d", got, ExitSuccess)
	}
	if exists(t, filepath.Join(root, "app", "src", "main.c")) {
		t.Error("workspace clean should remove main.c")
	}
	if !exists(t, filepath.Join(root, "app", "src", "main.kark")) {
		t.Error("workspace clean must preserve sources")
	}

	// unknown subcommand -> usage
	if got := exitCode("workspace", "frobnicate"); got != ExitUsage {
		t.Errorf("workspace unknown: got %d, want %d", got, ExitUsage)
	}
	// pkg workspace parity: unknown too
	if got := exitCode("pkg", "workspace", "frobnicate"); got != ExitUsage {
		t.Errorf("pkg workspace unknown: got %d, want %d", got, ExitUsage)
	}
}

func TestCLI_Workspace_BuildE2E(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
	bin := buildKarkain(t)
	root := buildWorkspaceDir(t, []string{"app"}, map[string][]string{"app": {}})

	out, err := runBin(t, bin, root, "workspace", "build")
	if err != nil {
		t.Fatalf("workspace build failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "BUILD [0] app") {
		t.Errorf("expected BUILD line, got:\n%s", out)
	}
}
