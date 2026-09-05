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

func TestCLI_Workspace_GraphRemoveLintE2E(t *testing.T) {
	bin := buildKarkain(t)
	// Graph exercises cross-member edges; lint uses a dep-free workspace so no
	// member references an out-of-scope sibling function.
	graphRoot := buildWorkspaceDir(t, []string{"lib", "app"}, map[string][]string{"lib": {}, "app": {"lib"}})
	lintRoot := buildWorkspaceDir(t, []string{"app"}, map[string][]string{"app": {}})

	exitCode := func(root string, args ...string) int {
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

	// graph shows both members in dependency order with the app->lib edge
	out, err := runBin(t, bin, graphRoot, "workspace", "graph")
	if err != nil {
		t.Fatalf("workspace graph failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "member lib") || !strings.Contains(out, "member app -> lib") {
		t.Errorf("workspace graph output unexpected:\n%s", out)
	}

	// lint passes for the single-member workspace
	if got := exitCode(lintRoot, "workspace", "lint"); got != ExitSuccess {
		t.Errorf("workspace lint: got %d, want %d", got, ExitSuccess)
	}
	// pkg workspace parity
	out, err = runBin(t, bin, lintRoot, "pkg", "workspace", "lint")
	if err != nil {
		t.Fatalf("pkg workspace lint failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "LINT  [0] app") {
		t.Errorf("pkg workspace lint output unexpected:\n%s", out)
	}

	// remove app from the graph workspace, then list shows only lib
	if got := exitCode(graphRoot, "workspace", "remove", "app"); got != ExitSuccess {
		t.Errorf("workspace remove app: got %d, want %d", got, ExitSuccess)
	}
	out, err = runBin(t, bin, graphRoot, "workspace", "list")
	if err != nil {
		t.Fatalf("workspace list after remove failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "app") || !strings.Contains(out, "lib") {
		t.Errorf("list after remove unexpected:\n%s", out)
	}
	// removing a non-member -> package exit
	if got := exitCode(graphRoot, "workspace", "remove", "nope"); got != ExitPackage {
		t.Errorf("workspace remove non-member: got %d, want %d", got, ExitPackage)
	}
}

func TestCLI_Workspace_LintFailureExit(t *testing.T) {
	bin := buildKarkain(t)
	root := buildWorkspaceDir(t, []string{"app"}, map[string][]string{"app": {}})
	writeFile(t, filepath.Join(root, "app", "src", "main.kark"),
		"func main() { print(1) } extrajunk\n")

	_, err := runBin(t, bin, root, "workspace", "lint")
	if err == nil {
		t.Fatal("workspace lint with syntax error should fail")
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != ExitCompile {
		t.Errorf("workspace lint failure: got %d, want %d", ee.ExitCode(), ExitCompile)
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
