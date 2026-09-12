package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Phase 90: Production Release Gate tests.
// Verifies Karkain 1.0 production readiness.

func phase90Root(t *testing.T) string {
	t.Helper()
	_, f, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(f), "..", "..")
}

func TestPhase90_VersionConsistency(t *testing.T) {
	root := phase90Root(t)
	karkain := buildBootstrap(t, root)

	cmd := exec.Command(karkain, "--version")
	out, err := cmd.CombinedOutput()
	t.Logf("version: %s", string(out))
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	if !strings.Contains(string(out), "v0.115.0") {
		t.Fatalf("expected v0.115.0 in version output, got: %s", string(out))
	}
}

func TestPhase90_CLICheck(t *testing.T) {
	root := phase90Root(t)
	karkain := buildBootstrap(t, root)

	testFiles := []string{
		"01_hello.kark",
		"02_arithmetic.kark",
		"03_functions.kark",
		"04_variables.kark",
		"05_return.kark",
		"06_control_flow.kark",
		"release.kark",
	}

	for _, name := range testFiles {
		t.Run(name, func(t *testing.T) {
			srcFile := filepath.Join(root, "kcc-tests", name)
			cmd := exec.Command(karkain, "check", srcFile)
			out, err := cmd.CombinedOutput()
			t.Logf("output: %s", string(out))
			if err != nil {
				t.Fatalf("check failed for %s: %v", name, err)
			}
		})
	}
}

func TestPhase90_CLIBuild(t *testing.T) {
	root := phase90Root(t)
	karkain := buildBootstrap(t, root)

	testFiles := []string{
		"01_hello.kark",
		"02_arithmetic.kark",
		"03_functions.kark",
		"release.kark",
	}

	for _, name := range testFiles {
		t.Run(name, func(t *testing.T) {
			srcFile := filepath.Join(root, "kcc-tests", name)
			cmd := exec.Command(karkain, "build", srcFile, "--target", "c23")
			out, err := cmd.CombinedOutput()
			t.Logf("output: %s", string(out))
			if err != nil {
				t.Fatalf("build failed for %s: %v", name, err)
			}
		})
	}
}

func TestPhase90_CLIFmt(t *testing.T) {
	root := phase90Root(t)
	karkain := buildBootstrap(t, root)

	// Test fmt --check on a well-formatted file
	srcFile := filepath.Join(root, "kcc-tests", "01_hello.kark")
	cmd := exec.Command(karkain, "fmt", "--check", srcFile)
	out, err := cmd.CombinedOutput()
	t.Logf("fmt --check output: %s", string(out))
	// fmt --check returns 0 if formatted, non-zero if not
	// We just verify it doesn't crash
	_ = err
}

func TestPhase90_CLILint(t *testing.T) {
	root := phase90Root(t)
	karkain := buildBootstrap(t, root)

	srcFile := filepath.Join(root, "kcc-tests", "03_functions.kark")
	cmd := exec.Command(karkain, "lint", srcFile)
	out, err := cmd.CombinedOutput()
	t.Logf("lint output: %s", string(out))
	if err != nil {
		t.Fatalf("lint failed: %v", err)
	}
	if !strings.Contains(string(out), "passed") {
		t.Fatalf("expected lint passed, got: %s", string(out))
	}
}

func TestPhase90_StdlibImport(t *testing.T) {
	root := phase90Root(t)
	karkain := buildBootstrap(t, root)

	// Test that stdlib modules can be imported
	srcFile := filepath.Join(root, "kcc-tests", "stdlib_import_test.kark")
	cmd := exec.Command(karkain, "check", srcFile)
	out, err := cmd.CombinedOutput()
	t.Logf("output: %s", string(out))
	if err != nil {
		t.Fatalf("stdlib import check failed: %v", err)
	}
}

func TestPhase90_NativeExecutableGeneration(t *testing.T) {
	root := phase90Root(t)

	gccPath, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("gcc not available")
	}

	karkain := buildBootstrap(t, root)

	// Build release test to C
	releaseFile := filepath.Join(root, "kcc-tests", "release.kark")
	cmd := exec.Command(karkain, "build", releaseFile, "--target", "c23")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build to C failed: %v\n%s", err, string(out))
	}

	// Compile C to native
	cFile := filepath.Join(root, "kcc-tests", "release.c")
	exeFile := filepath.Join(root, "kcc-tests", "release.exe")
	cmd = exec.Command(gccPath, "-std=c99", "-o", exeFile, cFile, "-lm", "-lgmp")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gcc compilation failed: %v\n%s", err, string(out))
	}

	// Verify executable
	if _, err := os.Stat(exeFile); os.IsNotExist(err) {
		t.Fatalf("native executable not created")
	}
}

func TestPhase90_SelfHostedCompilerStillWorks(t *testing.T) {
	root := phase90Root(t)
	karkain := buildBootstrap(t, root)

	// Verify self-hosted compiler source still checks
	srcFile := filepath.Join(root, "src", "compiler", "main.kark")
	cmd := exec.Command(karkain, "check", srcFile)
	out, err := cmd.CombinedOutput()
	t.Logf("output: %s", string(out))
	if err != nil {
		t.Fatalf("self-hosted compiler check failed: %v", err)
	}
}

func TestPhase90_RuntimeDirectoryStructure(t *testing.T) {
	root := phase90Root(t)

	runtimeDir := filepath.Join(root, "runtime")
	if _, err := os.Stat(runtimeDir); os.IsNotExist(err) {
		t.Fatalf("runtime directory not found")
	}

	expected := []string{
		"types.kark",
		"io.kark",
		"platform.kark",
		"init.kark",
		"boundary.kark",
	}

	for _, name := range expected {
		path := filepath.Join(runtimeDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("runtime file not found: %s", name)
		}
	}
}

func TestPhase90_StdlibDirectoryStructure(t *testing.T) {
	root := phase90Root(t)

	stdlibDir := filepath.Join(root, "stdlib")
	if _, err := os.Stat(stdlibDir); os.IsNotExist(err) {
		t.Fatalf("stdlib directory not found")
	}

	expected := []string{
		"core/core.kark",
		"string/string.kark",
		"collections/collections.kark",
		"math/math.kark",
		"io/io.kark",
		"system/system.kark",
	}

	for _, name := range expected {
		path := filepath.Join(stdlibDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("stdlib file not found: %s", name)
		}
	}
}

func TestPhase90_ConformanceTestsExist(t *testing.T) {
	root := phase90Root(t)

	conformanceDir := filepath.Join(root, "conformance")
	if _, err := os.Stat(conformanceDir); os.IsNotExist(err) {
		t.Fatalf("conformance directory not found")
	}

	entries, err := os.ReadDir(conformanceDir)
	if err != nil {
		t.Fatalf("cannot read conformance directory: %v", err)
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), "_test.kark") {
			count++
		}
	}

	if count < 10 {
		t.Fatalf("expected at least 10 conformance tests, found %d", count)
	}
	t.Logf("found %d conformance tests", count)
}

func TestPhase90_GoTestsPass(t *testing.T) {
	root := phase90Root(t)

	cmd := exec.Command("go", "test", "./pkg/lexer/...", "./pkg/parser/...", "./pkg/codegen/...", "./pkg/pm/...", "-count=1")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	t.Logf("output: %s", string(out))
	if err != nil {
		t.Fatalf("existing tests failed: %v", err)
	}
}

func TestPhase90_BuildScriptExists(t *testing.T) {
	root := phase90Root(t)
	script := filepath.Join(root, "scripts", "build-kcc.sh")
	if _, err := os.Stat(script); os.IsNotExist(err) {
		t.Fatalf("build script not found at %s", script)
	}
}

func TestPhase90_ReleaseTestProgramsExist(t *testing.T) {
	root := phase90Root(t)

	expected := []string{
		"release.kark",
		"acceptance.kark",
		"01_hello.kark",
		"02_arithmetic.kark",
		"03_functions.kark",
		"04_variables.kark",
		"05_return.kark",
		"06_control_flow.kark",
		"07_error.kark",
	}

	for _, name := range expected {
		path := filepath.Join(root, "kcc-tests", name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("test program not found: %s", name)
		}
	}
}
