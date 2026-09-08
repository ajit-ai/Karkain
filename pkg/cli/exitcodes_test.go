package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateTarget(t *testing.T) {
	valid := []string{"native", "c23", "wasm32-wasi"}
	for _, v := range valid {
		if err := ValidateTarget(v); err != nil {
			t.Errorf("ValidateTarget(%q) = %v, want nil", v, err)
		}
	}
	invalid := []string{"x86_64", "bogus", "spirv", "Native"}
	for _, v := range invalid {
		if err := ValidateTarget(v); err == nil {
			t.Errorf("ValidateTarget(%q) = nil, want error", v)
		}
	}
}

func TestTargetError_Message(t *testing.T) {
	err := &TargetError{Target: "bogus"}
	if err == nil {
		t.Fatal("TargetError returned nil")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("TargetError message missing target: %v", err)
	}
	for _, v := range []string{"native", "c23", "wasm32-wasi"} {
		if !strings.Contains(err.Error(), v) {
			t.Errorf("TargetError message missing valid target %q: %v", v, err)
		}
	}
}

func TestClassifyCompileError(t *testing.T) {
	envErr := errors.New("no supported C compiler found")
	if got := classifyCompileError(envErr); got != ExitEnv {
		t.Errorf("classifyCompileError(env) = %d, want %d", got, ExitEnv)
	}
	for _, e := range []error{
		errors.New("parse failed"),
		errors.New("codegen error: no target"),
		errors.New("gcc: fatal error"),
	} {
		if got := classifyCompileError(e); got != ExitCompile {
			t.Errorf("classifyCompileError(%q) = %d, want %d", e, got, ExitCompile)
		}
	}
}

func TestExitCodeConstants(t *testing.T) {
	want := map[int]bool{
		ExitSuccess: true,
		ExitFailure: true,
		ExitUsage:   true,
		ExitCompile: true,
		ExitTest:    true,
		ExitPackage: true,
		ExitEnv:     true,
	}
	for code := 0; code <= 6; code++ {
		if !want[code] {
			t.Errorf("exit code %d is not assigned (scheme must map 0..6)", code)
		}
	}
}

func TestCLI_ExitCodes_E2E(t *testing.T) {
	// Phase 97: kcc is the default engine. These tests assert the Go engine's
	// exit-code contract (usage=2, compile=3), so pin the Go engine explicitly.
	t.Setenv("KARKAIN_ENGINE", "go")
	bin := buildKarkain(t)
	root := t.TempDir()

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

	// CLI usage errors -> 2.
	if got := exitCode("--bogus-flag"); got != ExitUsage {
		t.Errorf("unknown flag: got %d, want %d", got, ExitUsage)
	}
	if got := exitCode("frobnicate"); got != ExitUsage {
		t.Errorf("unknown command: got %d, want %d", got, ExitUsage)
	}
	if got := exitCode("run", "--target", "x86_64", filepath.Join(root, "a.kark")); got != ExitUsage {
		t.Errorf("invalid target: got %d, want %d", got, ExitUsage)
	}
	badExt := filepath.Join(root, "a.txt")
	os.WriteFile(badExt, []byte("print(1)"), 0644)
	if got := exitCode("check", badExt); got != ExitUsage {
		t.Errorf("invalid extension: got %d, want %d", got, ExitUsage)
	}

	// Parse failure -> 3.
	parseErr := filepath.Join(root, "bad.kark")
	os.WriteFile(parseErr, []byte("func main() { print(1) } extrajunk"), 0644)
	if got := exitCode("check", parseErr); got != ExitCompile {
		t.Errorf("parse error: got %d, want %d", got, ExitCompile)
	}

	// Name-resolution failure (duplicate top-level def) -> 3.
	depthErr := filepath.Join(root, "dup.kark")
	os.WriteFile(depthErr, []byte("func foo() { return 1 }\nfunc foo() { return 2 }\n"), 0644)
	if got := exitCode("check", depthErr); got != ExitCompile {
		t.Errorf("name-resolution error: got %d, want %d", got, ExitCompile)
	}

	// Package operational failure (not in a project) -> 5.
	if got := exitCode("pkg", "add", "libdep"); got != ExitPackage {
		t.Errorf("pkg not-in-project: got %d, want %d", got, ExitPackage)
	}
	if got := exitCode("pkg", "unknownsub"); got != ExitUsage {
		t.Errorf("pkg unknown subcommand: got %d, want %d", got, ExitUsage)
	}

	// Package success -> 0.
	if got := exitCode("new", "sample"); got != ExitSuccess {
		t.Errorf("pkg new: got %d, want %d", got, ExitSuccess)
	}
	// Compile failure (no gcc -> env class 6) still routes through ExitEnv
	// when codegen is reached; guard on compilation pipelines via gcc check.
}

func TestCLI_TestFailure_ExitCode_E2E(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
	// Phase 97: pin the Go engine so this asserts the Go runner's exit-4
	// contract deterministically (kcc's test-runner exit path is covered by the
	// phase95/96 parity gates instead).
	t.Setenv("KARKAIN_ENGINE", "go")
	bin := buildKarkain(t)
	root := t.TempDir()

	failTest := filepath.Join(root, "failing_test.kark")
	os.WriteFile(failTest, []byte("func test_ok() { assert(1 == 1) }\nfunc test_bad() { assert(1 == 2) }\n"), 0644)

	_, err := runBin(t, bin, root, "test", failTest)
	if err == nil {
		t.Fatal("expected test command to fail")
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("unexpected error type: %v", err)
	}
	if got := ee.ExitCode(); got != ExitTest {
		t.Errorf("failing test: got %d, want %d", got, ExitTest)
	}
}
