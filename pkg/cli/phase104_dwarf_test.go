package cli

import (
	"strings"
	"testing"

	"karkain/pkg/codegen"
)

const phase104Source = `func factorial(n int) int {
	if n <= 1 {
		return 1
	}
	return n * factorial(n - 1)
}
func main() {
	let value = factorial(5)
	return value
}
`

func phase104Program(t *testing.T, source, file string) (*codegen.NativeBuildResult, string) {
	t.Helper()
	prog := parseSource(source, false)
	builder := codegen.NewNativeBuilder()
	result, err := builder.Build(prog, file)
	if err != nil {
		t.Fatalf("NativeBuilder.Build failed: %v", err)
	}
	if result.Executable == nil {
		t.Fatal("expected non-nil Executable")
	}
	return result, source
}

func TestPhase104_NativeExecutableCarriesDWARF(t *testing.T) {
	result, _ := phase104Program(t, phase104Source, "phase104_source.kark")
	exec := result.Executable
	if !exec.HasDebugSections() {
		t.Fatal("expected DWARF debug sections on native executable")
	}
	sections := exec.GetDebugSections()
	if len(sections) != 4 {
		t.Fatalf("expected 4 debug sections, got %d", len(sections))
	}
	for _, sect := range sections {
		if len(sect.Data) == 0 {
			t.Errorf("debug section %s is empty", sect.Name)
		}
	}
	for _, want := range codegen.DwarfSectionNames {
		if exec.GetSection(want) == nil {
			t.Errorf("missing expected DWARF section %q", want)
		}
	}
}

func TestPhase104_DWARFRoundTrip(t *testing.T) {
	result, _ := phase104Program(t, phase104Source, "phase104_source.kark")
	sections := result.Executable.GetDebugSections()
	info, abbrev, str, line := splitDebugSections(sections)

	dw, err := codegen.ParseDWARF(info, abbrev, str, line)
	if err != nil {
		t.Fatalf("ParseDWARF failed: %v", err)
	}
	if dw.Version != 4 {
		t.Errorf("version = %d, want 4", dw.Version)
	}
	if dw.UnitName != "phase104_source.kark" {
		t.Errorf("unit name = %q, want %q", dw.UnitName, "phase104_source.kark")
	}
	if len(dw.Funcs) < 2 {
		t.Fatalf("expected at least 2 subprograms, got %d", len(dw.Funcs))
	}
	byName := make(map[string]codegen.ParsedSubprogram)
	for _, fn := range dw.Funcs {
		byName[fn.Name] = fn
	}
	for _, name := range []string{"factorial", "main"} {
		fn, ok := byName[name]
		if !ok {
			t.Errorf("missing subprogram %q", name)
			continue
		}
		if fn.Size == 0 {
			t.Errorf("subprogram %q has zero size", name)
		}
		if fn.Line == 0 {
			t.Errorf("subprogram %q has zero decl line", name)
		}
	}
	if !(byName["main"].LowPC >= byName["factorial"].LowPC && byName["main"].LowPC > 0) {
		t.Errorf("main low_pc 0x%x should follow factorial low_pc 0x%x",
			byName["main"].LowPC, byName["factorial"].LowPC)
	}

	var foundN, foundValue bool
	for _, v := range dw.Variables {
		if v.Name == "n" && v.Function == "factorial" {
			foundN = true
		}
		if v.Name == "value" && v.Function == "main" {
			foundValue = true
		}
	}
	if !foundN {
		t.Error("expected parameter variable 'n' on factorial")
	}
	if !foundValue {
		t.Error("expected variable 'value' on main")
	}

	if len(dw.LineRows) == 0 {
		t.Fatal("expected line rows")
	}
	var endSeqs int
	for _, r := range dw.LineRows {
		if r.EndSequence {
			endSeqs++
		}
	}
	if endSeqs < 2 {
		t.Errorf("expected at least 2 end-sequence rows, got %d", endSeqs)
	}
	for _, r := range dw.LineRows {
		if r.File == 0 {
			t.Errorf("line row with file 0 at addr 0x%x", r.Address)
		}
		if !r.EndSequence && r.Line == 0 {
			t.Errorf("line row with line 0 at addr 0x%x", r.Address)
		}
	}
}

func TestPhase104_DWARFTextDump(t *testing.T) {
	result, _ := phase104Program(t, phase104Source, "phase104_source.kark")
	text, err := codegen.DwarfTextDump(result.Executable.GetDebugSections())
	if err != nil {
		t.Fatalf("DwarfTextDump failed: %v", err)
	}
	for _, want := range []string{
		"DW_TAG_compile_unit",
		"DW_TAG_subprogram",
		"DW_AT_producer",
		"DW_AT_low_pc",
		"factorial",
		"main",
		"phase104_source.kark",
		"Contents of the .debug_line section",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text dump missing %q", want)
		}
	}
}

// splitDebugSections returns the four debug section payloads by name.
func splitDebugSections(sections []*codegen.Section) (info, abbrev, str, line []byte) {
	for _, sect := range sections {
		switch sect.Name {
		case ".debug_info":
			info = sect.Data
		case ".debug_abbrev":
			abbrev = sect.Data
		case ".debug_str":
			str = sect.Data
		case ".debug_line":
			line = sect.Data
		}
	}
	return info, abbrev, str, line
}