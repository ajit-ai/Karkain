package cli

// Phase 118 — Beta 1 external validation & release-candidate readiness gate.
//
// This suite is the automated half of the Phase 118 RC checklist
// (docs/source/status/rc-checklist.rst). It validates the external developer
// journey from a fresh binary: version identity, build identity, first
// program, multi-file project, workspace dependency, CLI/help contract,
// release/issue/documentation metadata, and representative examples.
//
// It deliberately avoids duplicating the Phase 114/115/116/117 gates; the
// regression baselines those suites enforce remain the ground truth for the
// stable core.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runBinEnv runs the karkain binary with args in workdir, forcing the Go
// engine (deterministic, no kcc rebuild) unless overridden by extraEnv.
func runBinEnv(t *testing.T, bin, workdir string, extraEnv []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = workdir
	cmd.Env = envWithEngineGo()
	for _, e := range extraEnv {
		cmd.Env = append(cmd.Env, e)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// envWithEngineGo returns the current environment with KARKAIN_ENGINE=go.
func envWithEngineGo() []string {
	out := make([]string, 0, 8)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "KARKAIN_ENGINE=") {
			continue
		}
		out = append(out, e)
	}
	return append(out, "KARKAIN_ENGINE=go")
}

// writeFileAt writes data to path, used to build repro programs in temp dirs.
func writeFileAt(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestPhase118_RCReadiness assembles the whole Phase 118 gate.
func TestPhase118_RCReadiness(t *testing.T) {
	bin := buildPreviewBinary(t)
	t.Run("VersionIdentity", func(t *testing.T) { testPhase118VersionIdentity(t, bin) })
	t.Run("BuildIdentity", func(t *testing.T) { testPhase118BuildIdentity(t, bin) })
	t.Run("HelpAndCLIContract", func(t *testing.T) { testPhase118HelpContract(t, bin) })
	t.Run("FirstProgram", func(t *testing.T) { testPhase118FirstProgram(t, bin) })
	t.Run("MultiFileProject", func(t *testing.T) { testPhase118MultiFileProject(t, bin) })
	t.Run("WorkspaceDependency", func(t *testing.T) { testPhase118WorkspaceDependency(t, bin) })
	t.Run("ExamplesSpotCheck", func(t *testing.T) { testPhase118ExamplesSpotCheck(t, bin) })
	t.Run("ReleaseAndDocsMetadata", func(t *testing.T) { testPhase118ReleaseMetadata(t) })
	t.Run("DocsNavigation", func(t *testing.T) { testPhase118DocsNavigation(t) })
}

func testPhase118VersionIdentity(t *testing.T, bin string) {
	out, err := runBin(t, bin, t.TempDir(), "--version")
	if err != nil {
		t.Fatalf("karkain --version failed: %v\n%s", err, out)
	}
	for _, want := range []string{"Karkain Compiler v1.1.0", "Stable Build"} {
		if !strings.Contains(out, want) {
			t.Errorf("--version does not contain %q: %s", want, out)
		}
	}
	if strings.Contains(out, "Developer Preview") {
		t.Errorf("--version still reports Developer Preview: %s", out)
	}
	vf, rerr := os.ReadFile(filepath.Join(repoRoot(t), "VERSION"))
	if rerr != nil {
		t.Fatalf("read VERSION: %v", rerr)
	}
	if !strings.Contains(string(vf), "1.1.0") {
		t.Errorf("VERSION does not match 1.1.0: %s", vf)
	}
}

func testPhase118BuildIdentity(t *testing.T, bin string) {
	out, err := runBin(t, bin, t.TempDir(), "config")
	if err != nil {
		t.Fatalf("karkain config failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "engine:") {
		t.Errorf("karkain config must expose the engine for reproducibility:\n%s", out)
	}
	if !strings.Contains(out, "target:") {
		t.Errorf("karkain config must expose the target:\n%s", out)
	}

	tout, terr := runBin(t, bin, t.TempDir(), "target")
	if terr != nil {
		t.Fatalf("karkain target failed: %v\n%s", terr, tout)
	}
	for _, want := range []string{"Host:", "x86_64-windows", "native", "c23", "wasm32-wasi"} {
		if !strings.Contains(tout, want) {
			t.Errorf("karkain target missing %q:\n%s", want, tout)
		}
	}
}

func testPhase118HelpContract(t *testing.T, bin string) {
	out, err := runBin(t, bin, t.TempDir(), "--help")
	if err != nil {
		t.Fatalf("karkain --help failed: %v\n%s", err, out)
	}
	for _, cmd := range []string{
		"run", "build", "transpile", "check", "test", "bench", "prof",
		"lint", "explain", "fmt", "clean", "target", "config", "lsp",
		"workspace", "ide", "new", "pkg",
	} {
		if !strings.Contains(out, cmd) {
			t.Errorf("--help does not mention command %q", cmd)
		}
	}
	for _, legacy := range []string{"Developer Preview", "1.0", "v0.115.0"} {
		if strings.Contains(out, legacy) {
			t.Errorf("--help contains stale text %q", legacy)
		}
	}
}

func testPhase118FirstProgram(t *testing.T, bin string) {
	dir := t.TempDir()
	hello := filepath.Join(dir, "hello.kark")
	writeFileAt(t, hello, "func main() {\n    print(\"Hello, Karkain!\")\n}\n")

	if out, err := runBinEnv(t, bin, dir, nil, "check", hello); err != nil {
		t.Fatalf("check hello.kark failed: %v\n%s", err, out)
	}
	if out, err := runBinEnv(t, bin, dir, nil, "build", hello); err != nil {
		t.Fatalf("build hello.kark failed: %v\n%s", err, out)
	}
	out, err := runBinEnv(t, bin, dir, nil, "run", hello)
	if err != nil {
		t.Fatalf("run hello.kark failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Hello, Karkain!") {
		t.Errorf("unexpected first-program output: %s", out)
	}
}

func testPhase118MultiFileProject(t *testing.T, bin string) {
	dir := t.TempDir()
	writeFileAt(t, filepath.Join(dir, "math.kark"),
		"public func twice(n) {\n    return n * 2\n}\n")
	writeFileAt(t, filepath.Join(dir, "main.kark"),
		"import math\n\nfunc main() {\n    print(math.twice(21))\n}\n")

	if out, err := runBinEnv(t, bin, dir, nil, "check", filepath.Join(dir, "main.kark")); err != nil {
		t.Fatalf("check multi-file project failed: %v\n%s", err, out)
	}
	out, err := runBinEnv(t, bin, dir, nil, "run", filepath.Join(dir, "main.kark"))
	if err != nil {
		t.Fatalf("run multi-file project failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("multi-file project expected 42, got: %s", out)
	}
}

func testPhase118WorkspaceDependency(t *testing.T, bin string) {
	src := filepath.Join(repoRoot(t), "examples", "workspace")
	ws := filepath.Join(t.TempDir(), "ws")
	if err := copyDirRecursive(src, ws); err != nil {
		t.Fatalf("copy workspace example: %v", err)
	}

	if out, err := runBinEnv(t, bin, ws, nil, "workspace", "build"); err != nil {
		t.Fatalf("workspace build failed: %v\n%s", err, out)
	}
	out, err := runBinEnv(t, bin, ws, nil, "workspace", "run")
	if err != nil {
		t.Fatalf("workspace run failed: %v\n%s", err, out)
	}
	for _, want := range []string{"hi from library api", "hi from app"} {
		if !strings.Contains(out, want) {
			t.Errorf("workspace run missing %q in output:\n%s", want, out)
		}
	}
}

func testPhase118ExamplesSpotCheck(t *testing.T, bin string) {
	root := repoRoot(t)
	dir := t.TempDir()

	hello := filepath.Join(root, "examples", "01-fundamentals", "01_hello_world.kark")
	out, err := runBinEnv(t, bin, dir, nil, "run", hello)
	if err != nil {
		t.Fatalf("run 01_hello_world.kark failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Hello, Karkain!") {
		t.Errorf("02 golden mismatch: %s", out)
	}

	moduleSys := filepath.Join(root, "examples", "module_system", "main.kark")
	out, err = runBinEnv(t, bin, filepath.Dir(moduleSys), nil, "run", "main.kark")
	if err != nil {
		t.Fatalf("run module_system/main.kark failed: %v\n%s", err, out)
	}
	for _, want := range []string{"42", "9"} {
		if !strings.Contains(out, want) {
			t.Errorf("module_system golden missing %q: %s", want, out)
		}
	}
}

func testPhase118ReleaseMetadata(t *testing.T) {
	root := repoRoot(t)
	files := []string{
		"SECURITY.md",
		"CONTRIBUTING.md",
		"CODE_OF_CONDUCT.md",
		"LICENSE",
		".github/ISSUE_TEMPLATE/bug_report.yml",
		".github/ISSUE_TEMPLATE/feature_request.yml",
		".github/ISSUE_TEMPLATE/documentation.yml",
		".github/ISSUE_TEMPLATE/config.yml",
		"docs/source/status/scope.rst",
		"docs/source/status/compatibility.rst",
		"docs/source/status/migration-beta1.rst",
		"docs/source/status/rc-checklist.rst",
		"docs/source/reference/stable-api.rst",
		"docs/source/development/feature-freeze.rst",
		"docs/source/development/release.rst",
		"docs/source/development/reporting-bugs.rst",
		"docs/source/getting-started/first-project.rst",
		"docs/source/getting-started/workspace.rst",
		"examples/workspace/karkain.workspace.json",
		"examples/workspace/app/karkain.toml",
		"examples/workspace/library/src/api.kark",
		"scripts/install.ps1",
		"scripts/install.sh",
		"scripts/verify-install.ps1",
		"scripts/verify-rc-journey.ps1",
	}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("missing release/issue metadata file %s: %v", f, err)
		}
	}

	gi, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	for _, pat := range []string{"/releases/", "/.build/", "/artifacts/"} {
		if !strings.Contains(string(gi), pat) {
			t.Errorf(".gitignore must cover release hygiene pattern %s", pat)
		}
	}
}

func testPhase118DocsNavigation(t *testing.T) {
	root := repoRoot(t)
	want := map[string][]string{
		"docs/source/status/index.rst":         {"scope", "compatibility", "rc-checklist", "migration-beta1"},
		"docs/source/reference/index.rst":      {"stable-api"},
		"docs/source/development/index.rst":    {"feature-freeze", "release", "reporting-bugs"},
		"docs/source/getting-started/index.rst": {"first-project", "workspace"},
	}
	for f, needles := range want {
		data, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Errorf("read %s: %v", f, err)
			continue
		}
		for _, n := range needles {
			if !strings.Contains(string(data), n) {
				t.Errorf("%s must reference new page %q in its toctree/navigation", f, n)
			}
		}
	}
}

// copyDirRecursive copies src into dst (both directories).
func copyDirRecursive(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		return os.WriteFile(target, data, 0644)
	})
}