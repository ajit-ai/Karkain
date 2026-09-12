package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Phase 115 — Developer Preview Readiness gate.
//
// This suite is the automated half of docs/release/developer-preview-
// checklist.md. It verifies the public contract an external developer
// relies on: version identity, repository-integrity files, documentation
// tree completeness, honest status vocabulary, and a fresh-checkout
// build+run simulation.

// buildPreviewBinary builds the karkain CLI once for the whole suite.
func buildPreviewBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "karkain"+exeSuffix())
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/karkain")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build karkain: %v\n%s", err, out)
	}
	return bin
}

func exeSuffix() string {
	if os.PathSeparator == '\\' {
		return ".exe"
	}
	return ""
}

// TestPhase115_VersionCommand verifies the Developer Preview build identity.
func TestPhase115_VersionCommand(t *testing.T) {
	bin := buildPreviewBinary(t)
	out, err := runBin(t, bin, t.TempDir(), "--version")
	if err != nil {
		t.Fatalf("karkain --version failed: %v\n%s", err, out)
	}
	for _, want := range []string{"Karkain Compiler v0.117.0", "Beta 1 Build"} {
		if !strings.Contains(out, want) {
			t.Errorf("--version does not contain %q: %s", want, out)
		}
	}
}

// TestPhase115_HelpListsCommands verifies --help surfaces every implemented
// top-level command family.
func TestPhase115_HelpListsCommands(t *testing.T) {
	bin := buildPreviewBinary(t)
	out, err := runBin(t, bin, t.TempDir(), "--help")
	if err != nil {
		t.Fatalf("karkain --help failed: %v\n%s", err, out)
	}
	for _, cmd := range []string{
		"run", "build", "transpile", "check", "test", "bench", "prof",
		"lint", "explain", "fmt", "clean", "target", "config", "lsp",
		"workspace", "ide", "new",
	} {
		if !strings.Contains(out, "  "+cmd) && !strings.Contains(out, cmd) {
			t.Errorf("--help does not mention command %q", cmd)
		}
	}
}

// TestPhase115_RepoSkeleton verifies the Developer Preview integrity files
// exist at the repository root and carry the expected content markers.
func TestPhase115_RepoSkeleton(t *testing.T) {
	root := repoRoot(t)
	files := map[string]string{
		"README.md":          "Beta 1",
		"LICENSE":            "MIT License",
		"CONTRIBUTING.md":    "Contributing to Karkain",
		"CODE_OF_CONDUCT.md": "Contributor Covenant",
		"VERSION":            "0.117.0",
	}
	for name, marker := range files {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Errorf("missing required repository file %s: %v", name, err)
			continue
		}
		if !strings.Contains(string(data), marker) {
			t.Errorf("%s does not contain expected marker %q", name, marker)
		}
	}

	// Every relative link in README.md must resolve to an existing file.
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	for _, m := range markdownLinkRe.FindAllStringSubmatch(string(readme), -1) {
		target := m[1]
		if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
			continue
		}
		target = strings.SplitN(target, "#", 2)[0]
		if target == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, target)); err != nil {
			t.Errorf("README link to non-existent path: %s", target)
		}
	}
}

// TestPhase115_DocTree verifies the required documentation pages exist in
// the RST source tree (mirror of the CI html check).
func TestPhase115_DocTree(t *testing.T) {
	root := filepath.Join(repoRoot(t), "docs", "source")
	pages := []string{
		"index.rst",
		"getting-started/index.rst",
		"getting-started/installation.rst",
		"language/index.rst",
		"stdlib/index.rst",
		"tools/index.rst",
		"targets/index.rst",
		"compiler/index.rst",
		"status/index.rst",
		"status/implemented.rst",
		"reference/index.rst",
		"reference/example-matrix.rst",
		"development/index.rst",
		"development/developer-preview.rst",
		"examples/index.rst",
		"release-notes.rst",
	}
	for _, p := range pages {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("missing required doc page %s: %v", p, err)
		}
	}
	status, err := os.ReadFile(filepath.Join(root, "status", "index.rst"))
	if err != nil || !strings.Contains(string(status), "Beta 1") {
		t.Errorf("status/index.rst does not carry the Beta 1 status")
	}
}

// TestPhase115_ExampleInventory verifies the example corpus is present and
// the verify script exists (Phase 114 dependency for the Developer Preview).
func TestPhase115_ExampleInventory(t *testing.T) {
	root := repoRoot(t)
	for _, p := range []string{
		"examples/EXAMPLES.md",
		"examples/01-fundamentals/01_hello_world.kark",
		"examples/15-developer-tools/02_assertions_test.kark",
		"scripts/verify-examples.ps1",
		"pkg/cli/phase114_examples_test.go",
	} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("missing example-corpus artifact %s: %v", p, err)
		}
	}
}

// gitAvailable reports whether git is on PATH (needed for the fresh-checkout
// simulation).
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

var markdownLinkRe = regexp.MustCompile(`\]\(([^)]+)\)`)

// TestPhase115_FreshCheckout simulates a brand-new developer pulling the
// repository, building it, and running the documented first-program workflow.
func TestPhase115_FreshCheckout(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git is not on PATH; skipping fresh-checkout simulation")
	}
	skipIfNoCompiler(t)

	parent := t.TempDir()
	clone := filepath.Join(parent, "Karkain")
	cmd := exec.Command("git", "clone", "--quiet", repoRoot(t), clone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, out)
	}

	bin := exeIn(clone)
	build := exec.Command("go", "build", "-o", bin, "./cmd/karkain")
	build.Dir = clone
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fresh-checkout build failed: %v\n%s", out, err)
	}

	workdir := filepath.Join(clone, "scratch")
	if err := os.MkdirAll(workdir, 0755); err != nil {
		t.Fatal(err)
	}

	prog := "func main() {\n    println(\"Hello, Karkain!\")\n}\n"
	if err := os.WriteFile(filepath.Join(workdir, "hello.kark"), []byte(prog), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := runBin(t, bin, workdir, "run", "hello.kark", "--engine", "go")
	if err != nil {
		t.Fatalf("fresh run failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Hello, Karkain!") {
		t.Errorf("fresh run did not print Hello, Karkain!: %s", out)
	}

	out, err = runBin(t, bin, workdir, "check", "hello.kark")
	if err != nil {
		t.Fatalf("fresh check failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "[ok]") {
		t.Errorf("fresh check did not report [ok]: %s", out)
	}

	out, err = runBin(t, bin, workdir, "build", "hello.kark")
	if err != nil {
		t.Fatalf("fresh build failed: %v\n%s", err, out)
	}

	testFile := "func double(x) {\n    return x * 2\n}\n\n" +
		"func test_double() {\n    assert_eq(double(21), 42)\n}\n"
	if err := os.WriteFile(filepath.Join(workdir, "hello_test.kark"), []byte(testFile), 0644); err != nil {
		t.Fatal(err)
	}
	out, err = runBin(t, bin, workdir, "test", "hello_test.kark", "--engine", "go")
	if err != nil {
		t.Fatalf("fresh test failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "1 passed") {
		t.Errorf("fresh test did not report 1 passed: %s", out)
	}
}

func exeIn(dir string) string {
	return filepath.Join(dir, "karkain"+exeSuffix())
}
