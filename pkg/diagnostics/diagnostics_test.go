package diagnostics

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSourceMap_JSONGeneration(t *testing.T) {
	sm := NewSourceMap("example.kar")
	sm.AddMapping(1, 0, 3, 5, "main")
	sm.AddMapping(2, 10, 4, 8, "add")
	sm.AddMapping(5, 0, 10, 0, "multiply")

	data, err := sm.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	// Verify valid JSON
	var parsed SourceMap
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if parsed.SourceFile != "example.kar" {
		t.Errorf("expected sourceFile 'example.kar', got '%s'", parsed.SourceFile)
	}
	if len(parsed.Mappings) != 3 {
		t.Fatalf("expected 3 mappings, got %d", len(parsed.Mappings))
	}

	// Verify first mapping
	m0 := parsed.Mappings[0]
	if m0.GeneratedLine != 1 || m0.GeneratedColumn != 0 {
		t.Errorf("mapping 0: expected generated (1,0), got (%d,%d)", m0.GeneratedLine, m0.GeneratedColumn)
	}
	if m0.OriginalLine != 3 || m0.OriginalColumn != 5 {
		t.Errorf("mapping 0: expected original (3,5), got (%d,%d)", m0.OriginalLine, m0.OriginalColumn)
	}
	if m0.SymbolName != "main" {
		t.Errorf("mapping 0: expected symbol 'main', got '%s'", m0.SymbolName)
	}

	// Verify JSON contains expected keys
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"sourceFile"`) {
		t.Error("JSON missing sourceFile field")
	}
	if !strings.Contains(jsonStr, `"mappings"`) {
		t.Error("JSON missing mappings field")
	}
}

func TestReporter_FormatError(t *testing.T) {
	source := `func add(a int, b int) int {
    return a + b
}

func main() {
    let x = add(1, 2)
    let y = sub(3, 4)
}`
	reporter := NewReporter(source, "main.kar")

	output := reporter.Report(SeverityError, 7, 12, "undefined function 'sub'")

	// Verify file path in output
	if !strings.Contains(output, "main.kar:7:12") {
		t.Errorf("output missing file path and location, got:\n%s", output)
	}

	// Verify line numbers are shown
	if !strings.Contains(output, "6 |") || !strings.Contains(output, "7 |") {
		t.Errorf("output missing line numbers, got:\n%s", output)
	}

	// Verify caret marker is present
	if !strings.Contains(output, "^") {
		t.Errorf("output missing caret marker, got:\n%s", output)
	}

	// Verify the error message is included
	if !strings.Contains(output, "undefined function 'sub'") {
		t.Errorf("output missing error message, got:\n%s", output)
	}
}

func TestReporter_SeverityLevels(t *testing.T) {
	source := `let x = 5`
	reporter := NewReporter(source, "test.kar")

	errorOutput := reporter.Report(SeverityError, 1, 5, "type mismatch")
	warningOutput := reporter.Report(SeverityWarning, 1, 5, "unused variable")
	infoOutput := reporter.Report(SeverityInfo, 1, 5, "hint available")

	if !strings.HasPrefix(errorOutput, "error:") {
		t.Errorf("error should start with 'error:', got:\n%s", errorOutput)
	}
	if !strings.HasPrefix(warningOutput, "warning:") {
		t.Errorf("warning should start with 'warning:', got:\n%s", warningOutput)
	}
	if !strings.HasPrefix(infoOutput, "info:") {
		t.Errorf("info should start with 'info:', got:\n%s", infoOutput)
	}
}
