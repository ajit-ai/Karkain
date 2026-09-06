package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFormatCommand_Directory formats a directory containing multiple .kark
// files and verifies that all files are processed.
func TestFormatCommand_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two .kark files
	files := map[string]string{
		"main.kark": "func  main(){\n   print(1)\n}\n",
		"lib.kark":  "func   add(a,b){\n  return   a+b\n}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	result := FormatCommand(tmpDir, false)
	if result.ExitCode != ExitSuccess {
		t.Errorf("FormatCommand(dir) = exit %d, message: %s", result.ExitCode, result.Message)
	}
	if result.Message == "" {
		t.Error("expected non-empty message")
	}

	// Verify files were formatted
	for name := range files {
		content, err := os.ReadFile(filepath.Join(tmpDir, name))
		if err != nil {
			t.Fatal(err)
		}
		src := string(content)
		if src == "" {
			t.Errorf("file %s is empty after formatting", name)
		}
		// The formatter canonicalizes inter-token spacing, so keywords
		// like "func" should be followed by exactly one space before the name
		if name == "main.kark" {
			if !strings.Contains(src, "func main()") {
				t.Errorf("file %s not properly formatted: %s", name, src)
			}
		}
		if name == "lib.kark" {
			if !strings.Contains(src, "func add(a, b)") {
				t.Errorf("file %s not properly formatted: %s", name, src)
			}
		}
	}
}

// TestFormatCommand_DirectoryCheck verifies that --check on a directory reports
// unformatted files without modifying them.
func TestFormatCommand_DirectoryCheck(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an unformatted file
	unformatted := "func  main(){\n   print(1)\n}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "test.kark"), []byte(unformatted), 0644); err != nil {
		t.Fatal(err)
	}

	result := FormatCommand(tmpDir, true)
	if result.ExitCode != ExitFailure {
		t.Errorf("FormatCommand(dir, check) = exit %d, want %d", result.ExitCode, ExitFailure)
	}

	// Verify file was NOT modified
	content, err := os.ReadFile(filepath.Join(tmpDir, "test.kark"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != unformatted {
		t.Error("file should not be modified in check-only mode")
	}
}

// TestFormatCommand_DirectoryAllFormatted verifies that --check on a directory
// with all files formatted returns success.
func TestFormatCommand_DirectoryAllFormatted(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a properly formatted file
	formatted := "func main() {\n    print(1)\n}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "test.kark"), []byte(formatted), 0644); err != nil {
		t.Fatal(err)
	}

	result := FormatCommand(tmpDir, true)
	if result.ExitCode != ExitSuccess {
		t.Errorf("FormatCommand(dir, check) = exit %d, want %d, message: %s", result.ExitCode, ExitSuccess, result.Message)
	}
}

// TestFormatCommand_DirectoryNoKarkFiles verifies that formatting a directory
// with no .kark files returns success.
func TestFormatCommand_DirectoryNoKarkFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a non-.kark file
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	result := FormatCommand(tmpDir, false)
	if result.ExitCode != ExitSuccess {
		t.Errorf("FormatCommand(dir) = exit %d, want %d", result.ExitCode, ExitSuccess)
	}
}

// TestCanonicalize_Exported verifies the exported Canonicalize function works
// the same as the internal formatter.
func TestCanonicalize_Exported(t *testing.T) {
	src := "func  add( a,b ){\n  return   a+b\n}\n"
	formatted, danger, err := Canonicalize(src)
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	if danger {
		t.Fatal("unexpected danger flag")
	}
	// Verify canonical spacing
	if formatted == src {
		t.Error("expected formatting to change source")
	}
}

// TestExitLintCode verifies the ExitLint constant value.
func TestExitLintCode(t *testing.T) {
	if ExitLint != 7 {
		t.Errorf("ExitLint = %d, want 7", ExitLint)
	}
}
