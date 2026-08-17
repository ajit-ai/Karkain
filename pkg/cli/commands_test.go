package cli

import (
	"karkain/pkg/codegen"
	"karkain/pkg/parser"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_CheckCommand(t *testing.T) {
	// Test valid .kar file
	validFile := filepath.Join(t.TempDir(), "valid.kar")
	err := os.WriteFile(validFile, []byte(`func main() {
    print("hello")
}`), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	result := CheckCommand(validFile, false)
	if result.ExitCode != 0 {
		t.Errorf("CheckCommand on valid file should pass, got exit code %d: %s", result.ExitCode, result.Message)
	}

	// Test invalid .kar file (syntax error)
	invalidFile := filepath.Join(t.TempDir(), "invalid.kar")
	err = os.WriteFile(invalidFile, []byte(`{{{`), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	result = CheckCommand(invalidFile, false)
	if result.ExitCode == 0 {
		t.Error("CheckCommand on invalid file should fail, got exit code 0")
	}

	// Test non-existent file
	result = CheckCommand(filepath.Join(t.TempDir(), "nonexistent.kar"), false)
	if result.ExitCode == 0 {
		t.Error("CheckCommand on non-existent file should fail")
	}
}

func TestCLI_BuildAndRunCommands(t *testing.T) {
	// Create a simple valid .kar file
	testDir := t.TempDir()
	simpleFile := filepath.Join(testDir, "simple.kar")
	err := os.WriteFile(simpleFile, []byte(`func add(a, b) {
    return a + b
}

func main() {
    let result = add(2, 3)
    print(result)
}`), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test build command (compile only, no execution)
	buildCfg := codegen.NewConfig()
	buildCfg.CompileOnly = true
	result := BuildCommand(simpleFile, "", buildCfg, false)
	if result.ExitCode != 0 {
		t.Errorf("BuildCommand should succeed, got exit code %d: %s", result.ExitCode, result.Message)
	}

	// Verify .c file was generated
	cFile := strings.TrimSuffix(simpleFile, filepath.Ext(simpleFile)) + ".c"
	if _, err := os.Stat(cFile); os.IsNotExist(err) {
		t.Error("BuildCommand should generate .c file")
	}

	// Test build command with custom output path
	outputExe := filepath.Join(testDir, "myapp")
	result = BuildCommand(simpleFile, outputExe, buildCfg, false)
	if result.ExitCode != 0 {
		t.Errorf("BuildCommand with output path should succeed, got exit code %d: %s", result.ExitCode, result.Message)
	}
}

func TestCLI_TestRunner(t *testing.T) {
	// Create a test file with test_ prefixed functions
	testDir := t.TempDir()
	testFile := filepath.Join(testDir, "math_test.kar")
	err := os.WriteFile(testFile, []byte(`func test_addition() {
    let result = 2 + 3
    print(result)
}

func test_subtraction() {
    let result = 5 - 3
    print(result)
}`), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := codegen.NewConfig()
	cfg.CompileOnly = true
	result := TestCommand(testFile, cfg, true)
	if result.ExitCode != 0 {
		t.Errorf("TestCommand should discover and run tests, got exit code %d: %s", result.ExitCode, result.Message)
	}
}

func TestCLI_TestRunner_NoTests(t *testing.T) {
	// Create a file with no test_ functions
	testDir := t.TempDir()
	noTestFile := filepath.Join(testDir, "nontest.kar")
	err := os.WriteFile(noTestFile, []byte(`func main() {
    print("no tests here")
}`), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := codegen.NewConfig()
	result := TestCommand(noTestFile, cfg, false)
	if result.ExitCode != 0 {
		t.Errorf("TestCommand with no tests should still pass, got exit code %d: %s", result.ExitCode, result.Message)
	}
}

func TestCLI_ValidateKarFile(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"test.kar", false},
		{"path/to/file.kar", false},
		{"", true},
		{"file.txt", true},
		{"file.go", true},
	}

	for _, tt := range tests {
		err := ValidateKarFile(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateKarFile(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
		}
	}
}

func TestCLI_CheckCommand_InvalidExtension(t *testing.T) {
	result := CheckCommand("test.go", false)
	if result.ExitCode == 0 {
		t.Error("CheckCommand on non-.kar file should fail")
	}
	if !strings.Contains(result.Message, ".kar") {
		t.Errorf("Expected error about .kar extension, got: %s", result.Message)
	}
}

func TestCLI_RunCommand_InvalidFile(t *testing.T) {
	result := RunCommand("nonexistent.kar", codegen.NewConfig(), false)
	if result.ExitCode == 0 {
		t.Error("RunCommand on non-existent file should fail")
	}
}

func TestCLI_BuildCommand_InvalidFile(t *testing.T) {
	result := BuildCommand("nonexistent.kar", "", codegen.NewConfig(), false)
	if result.ExitCode == 0 {
		t.Error("BuildCommand on non-existent file should fail")
	}
}

func TestCLI_HasKernelDecl(t *testing.T) {
	// Test with no kernels
	prog := &parser.Program{Statements: []parser.Node{}}
	if HasKernelDecl(prog) {
		t.Error("HasKernelDecl should return false for program without kernels")
	}
}
