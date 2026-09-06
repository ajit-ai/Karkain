package cli

import (
	"karkain/pkg/codegen"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCommand_NativeLink(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_native.kark")
	content := `func main() {
    let x = 42
    println(x)
}`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Run build with native-link target
	cfg := codegen.Config{
		Target:      "native-link",
		CompileOnly: true,
	}

	result := BuildCommand(testFile, "", cfg, false)
	if result.ExitCode != ExitSuccess {
		t.Errorf("BuildCommand with native-link target failed: %s", result.Message)
	}
	if !strings.Contains(result.Message, "Native build") {
		t.Errorf("expected 'Native build' in message, got: %s", result.Message)
	}
}

func TestBuildCommand_NativeLinkVerbose(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_native_verbose.kark")
	content := `func main() {
    let x = 42
    println(x)
}`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Run build with native-link target and verbose
	cfg := codegen.Config{
		Target:      "native-link",
		CompileOnly: true,
	}

	result := BuildCommand(testFile, "", cfg, true)
	if result.ExitCode != ExitSuccess {
		t.Errorf("BuildCommand with native-link verbose failed: %s", result.Message)
	}
}

func TestRunCommand_NativeLink(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_native_run.kark")
	content := `func main() {
    let x = 42
    println(x)
}`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Run with native-link target
	cfg := codegen.Config{
		Target: "native-link",
	}

	result := RunCommand(testFile, cfg, false)
	if result.ExitCode != ExitSuccess {
		t.Errorf("RunCommand with native-link target failed: %s", result.Message)
	}
}

func TestValidateTarget_NativeLink(t *testing.T) {
	// native-link should be a valid target
	if err := ValidateTarget("native-link"); err != nil {
		t.Errorf("ValidateTarget('native-link') = %v, want nil", err)
	}
}

func TestValidateTarget_Invalid(t *testing.T) {
	// invalid target should fail
	if err := ValidateTarget("invalid-target"); err == nil {
		t.Error("ValidateTarget('invalid-target') = nil, want error")
	}
}
