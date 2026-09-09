package codegen

import (
	"karkain/pkg/parser"
	"strings"
	"testing"
)

const dwarfTestTextBase = uint64(0x1000)

// dwarfTestProgram builds a two-function program: main with a local var and a
// return; helper with a parameter and a return.
func dwarfTestProgram() *parser.Program {
	return &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{
				Name:   "main",
				Params: []string{},
				Line:   1,
				Body: []parser.Node{
					&parser.VarDeclStmt{
						Name:  "x",
						Type:  "int",
						Value: &parser.IntLiteral{Value: "42"},
						Line:  2,
					},
					&parser.ReturnStmt{
						Value: &parser.IntLiteral{Value: "0"},
						Line:  3,
					},
				},
			},
			&parser.FuncDecl{
				Name:   "helper",
				Params: []string{"a"},
				Line:   5,
				Body: []parser.Node{
					&parser.ReturnStmt{
						Value: &parser.BinaryExpr{
							Left:     &parser.Identifier{Name: "a"},
							Operator: "*",
							Right:    &parser.IntLiteral{Value: "2"},
						},
						Line: 6,
					},
				},
			},
		},
	}
}

// collectTestDebugInfo builds relocated debug info for the test program.
func collectTestDebugInfo(t *testing.T) []*Section {
	t.Helper()
	builder := NewNativeBuilder()
	result, err := builder.Build(dwarfTestProgram(), "test.kark")
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if result.Executable == nil {
		t.Fatal("expected non-nil Executable")
	}
	if !result.Executable.HasDebugSections() {
		t.Fatal("expected DWARF sections on executable")
	}
	return result.Executable.GetDebugSections()
}

func sectionsData(sections []*Section) (info, abbrev, str, line []byte) {
	for _, sect := range sections {
		switch sect.Name {
		case dwarfDebugInfoName:
			info = sect.Data
		case dwarfDebugAbbrevName:
			abbrev = sect.Data
		case dwarfDebugStrName:
			str = sect.Data
		case dwarfDebugLineName:
			line = sect.Data
		}
	}
	return info, abbrev, str, line
}

func TestDwarfEmitter_SectionsProduced(t *testing.T) {
	sections := collectTestDebugInfo(t)
	if len(sections) != 4 {
		t.Fatalf("expected 4 debug sections, got %d", len(sections))
	}
	for _, sect := range sections {
		if sect.Type != SectionTypeDebug {
			t.Errorf("section %s has type %v, want SectionTypeDebug", sect.Name, sect.Type)
		}
		if len(sect.Data) == 0 {
			t.Errorf("section %s is empty", sect.Name)
		}
	}
	info, abbrev, str, line := sectionsData(sections)
	if info == nil || abbrev == nil || str == nil || line == nil {
		t.Fatal("expected all four DWARF sections")
	}
}

func TestDwarfEmitter_SubprogramRoundTrip(t *testing.T) {
	sections := collectTestDebugInfo(t)
	info, abbrev, str, line := sectionsData(sections)

	dw, err := ParseDWARF(info, abbrev, str, line)
	if err != nil {
		t.Fatalf("ParseDWARF failed: %v", err)
	}
	if dw.Version != 4 {
		t.Errorf("version = %d, want 4", dw.Version)
	}
	if dw.AddrSize != 8 {
		t.Errorf("addr size = %d, want 8", dw.AddrSize)
	}
	if dw.UnitName != "test.kark" {
		t.Errorf("unit name = %q, want %q", dw.UnitName, "test.kark")
	}
	if !strings.Contains(dw.Producer, "karkain-compiler") {
		t.Errorf("producer %q missing compiler prefix", dw.Producer)
	}

	byName := make(map[string]ParsedSubprogram)
	for _, fn := range dw.Funcs {
		byName[fn.Name] = fn
	}
	main, ok := byName["main"]
	if !ok {
		t.Fatal("expected 'main' subprogram")
	}
	if main.LowPC != dwarfTestTextBase {
		t.Errorf("main low_pc = 0x%x, want 0x%x", main.LowPC, dwarfTestTextBase)
	}
	if main.Size == 0 {
		t.Error("main size = 0, want non-zero")
	}
	if main.Line != 1 {
		t.Errorf("main decl line = %d, want 1", main.Line)
	}
	if main.File != 1 {
		t.Errorf("main decl file = %d, want 1", main.File)
	}
	helper, ok := byName["helper"]
	if !ok {
		t.Fatal("expected 'helper' subprogram")
	}
	if helper.Line != 5 {
		t.Errorf("helper decl line = %d, want 5", helper.Line)
	}
	if helper.LowPC <= main.LowPC {
		t.Errorf("helper low_pc 0x%x not greater than main low_pc 0x%x", helper.LowPC, main.LowPC)
	}
}

func TestDwarfEmitter_VariablesRoundTrip(t *testing.T) {
	sections := collectTestDebugInfo(t)
	info, abbrev, str, line := sectionsData(sections)

	dw, err := ParseDWARF(info, abbrev, str, line)
	if err != nil {
		t.Fatalf("ParseDWARF failed: %v", err)
	}
	var foundX, foundA bool
	for _, v := range dw.Variables {
		switch v.Name {
		case "x":
			foundX = v.Line == 2 && v.Function == "main"
		case "a":
			foundA = v.Line == 5 && v.Function == "helper"
		}
	}
	if !foundX {
		t.Error("expected variable x (line 2, function main)")
	}
	if !foundA {
		t.Error("expected variable a (line 5, function helper)")
	}
}

func TestDwarfEmitter_LineProgramRoundTrip(t *testing.T) {
	sections := collectTestDebugInfo(t)
	info, abbrev, str, line := sectionsData(sections)

	dw, err := ParseDWARF(info, abbrev, str, line)
	if err != nil {
		t.Fatalf("ParseDWARF failed: %v", err)
	}
	if len(dw.LineRows) == 0 {
		t.Fatal("expected line rows")
	}
	want := []struct {
		addr uint64
		line uint32
	}{
		{dwarfTestTextBase, 1},
		{dwarfTestTextBase + 4, 2},
		{dwarfTestTextBase + 8, 3},
		{dwarfTestTextBase + 0x0c, 5},
		{dwarfTestTextBase + 0x14, 6},
	}
	for _, w := range want {
		if !lineRowExists(dw.LineRows, w.addr, w.line) {
			t.Errorf("missing line row addr=0x%x line=%d", w.addr, w.line)
		}
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
}

func lineRowExists(rows []ParsedLineRow, addr uint64, line uint32) bool {
	for _, r := range rows {
		if r.Address == addr && r.Line == line && !r.EndSequence {
			return true
		}
	}
	return false
}

func TestDwarfEmitter_TextDump(t *testing.T) {
	sections := collectTestDebugInfo(t)
	text, err := DwarfTextDump(sections)
	if err != nil {
		t.Fatalf("DwarfTextDump failed: %v", err)
	}
	for _, want := range []string{
		"DW_TAG_compile_unit",
		"DW_TAG_subprogram",
		"DW_AT_low_pc",
		"main",
		"helper",
		"test.kark",
		"0x1000",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text dump missing %q", want)
		}
	}
}

func TestDwarfEmitter_NilDebugInfo(t *testing.T) {
	emitter := NewDwarfEmitter()
	if _, err := emitter.Emit(nil, dwarfTestTextBase, 0); err == nil {
		t.Error("expected error for nil debug info")
	}
}

func TestParseDWARF_Errors(t *testing.T) {
	if _, err := ParseDWARF([]byte{0, 1}, nil, nil, nil); err == nil {
		t.Error("expected error for short .debug_info")
	}
	badVersion := make([]byte, 11)
	badVersion[4] = 3
	if _, err := ParseDWARF(badVersion, nil, nil, nil); err == nil {
		t.Error("expected error for unsupported version")
	}
}

func TestDwarfEmitter_AttachedByNativeBuilder(t *testing.T) {
	sections := collectTestDebugInfo(t)
	var found bool
	for _, sect := range sections {
		if sect.Name == ".debug_line" {
			found = true
		}
	}
	if !found {
		t.Error("expected .debug_line section")
	}
}