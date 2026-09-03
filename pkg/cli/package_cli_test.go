package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildKarkain compiles the karkain CLI binary into a temp dir and returns
// its path. It is used by integration tests that exercise real dispatch.
func buildKarkain(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "karkain")
	if isWindows() {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "karkain/cmd/karkain")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build karkain: %v\n%s", err, out)
	}
	return bin
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// pkg/cli -> repo root is two up
	return filepath.Dir(filepath.Dir(dir))
}

func isWindows() bool { return os.PathSeparator == '\\' }

// runBin runs the karkain binary with args in a working dir and returns output.
func runBin(t *testing.T, bin, workdir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = workdir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestCLI_NewListTreeFetch_E2E(t *testing.T) {
	bin := buildKarkain(t)
	root := t.TempDir()

	// karkain new myapp
	out, err := runBin(t, bin, root, "new", "myapp")
	if err != nil {
		t.Fatalf("karkain new failed: %v\n%s", err, out)
	}
	appDir := filepath.Join(root, "myapp")
	if _, serr := os.Stat(filepath.Join(appDir, "karkain.toml")); os.IsNotExist(serr) {
		t.Fatalf("karkain new should create karkain.toml: %v", serr)
	}

	// Add a local dependency: create deps/lib inside the project, then
	// declare it as a local path dep.
	depDir := filepath.Join(root, "libdep")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}
	libManifest := `name = "libdep"
version = "0.1.0"
`
	if err := os.WriteFile(filepath.Join(depDir, "karkain.toml"), []byte(libManifest), 0644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(depDir, "lib.kark"), []byte("func libfoo() { return 1 }\n"), 0644)

	// karkain add <path> --source local --url <path>
	// (top-level add forwards to pkg add)
	out, err = runBin(t, bin, appDir, "add", "libdep", "--source", "local", "--url", filepath.Join(root, "libdep"))
	if err != nil {
		t.Fatalf("karkain add failed: %v\n%s", err, out)
	}

	// karkain update (re-resolve + write lock)
	out, err = runBin(t, bin, appDir, "update")
	if err != nil {
		t.Fatalf("karkain update failed: %v\n%s", err, out)
	}
	if _, lerr := os.Stat(filepath.Join(appDir, "karkain.lock")); os.IsNotExist(lerr) {
		t.Fatalf("karkain update should write karkain.lock: %v", lerr)
	}

	// karkain fetch (fetch locked deps)
	out, err = runBin(t, bin, appDir, "fetch")
	if err != nil {
		t.Fatalf("karkain fetch failed: %v\n%s", err, out)
	}

	// karkain list
	out, err = runBin(t, bin, appDir, "list")
	if err != nil {
		t.Fatalf("karkain list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "libdep") {
		t.Errorf("karkain list should mention libdep, got:\n%s", out)
	}

	// karkain tree
	out, err = runBin(t, bin, appDir, "tree")
	if err != nil {
		t.Fatalf("karkain tree failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "libdep") {
		t.Errorf("karkain tree should mention libdep, got:\n%s", out)
	}

	// remove
	out, err = runBin(t, bin, appDir, "remove", "libdep")
	if err != nil {
		t.Fatalf("karkain remove failed: %v\n%s", err, out)
	}
}

func TestCLI_HelpListsNewCommands(t *testing.T) {
	bin := buildKarkain(t)
	out, err := runBin(t, bin, t.TempDir(), "--help")
	if err != nil {
		t.Fatalf("karkain --help failed: %v", err)
	}
	for _, want := range []string{"karkain new", "karkain list", "karkain tree", "karkain update", "karkain remove"} {
		if !strings.Contains(out, want) {
			t.Errorf("--help should mention %q", want)
		}
	}
}
