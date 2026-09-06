package codegen

import (
	"testing"

	"karkain/pkg/parser"
)

func TestSourceAddressMap_ForwardLookup(t *testing.T) {
	sam := NewSourceAddressMap()
	sam.AddMapping("main.kark", 10, 0x401020, "main")
	sam.AddMapping("main.kark", 12, 0x401024, "main")
	sam.AddMapping("util.kark", 5, 0x401030, "helper")

	addr, ok := sam.GetAddressForSource("main.kark", 10)
	if !ok || addr != 0x401020 {
		t.Errorf("expected 0x401020, got 0x%x (found=%v)", addr, ok)
	}
	addr, ok = sam.GetAddressForSource("main.kark", 12)
	if !ok || addr != 0x401024 {
		t.Errorf("expected 0x401024, got 0x%x (found=%v)", addr, ok)
	}
	addr, ok = sam.GetAddressForSource("util.kark", 5)
	if !ok || addr != 0x401030 {
		t.Errorf("expected 0x401030, got 0x%x (found=%v)", addr, ok)
	}
}

func TestSourceAddressMap_ReverseLookup(t *testing.T) {
	sam := NewSourceAddressMap()
	sam.AddMapping("main.kark", 10, 0x401020, "main")

	loc, ok := sam.GetSourceForAddress(0x401020)
	if !ok {
		t.Fatal("expected to find source for address 0x401020")
	}
	if loc.File != "main.kark" {
		t.Errorf("expected file 'main.kark', got '%s'", loc.File)
	}
	if loc.Line != 10 {
		t.Errorf("expected line 10, got %d", loc.Line)
	}
	if loc.Function != "main" {
		t.Errorf("expected function 'main', got '%s'", loc.Function)
	}
}

func TestSourceAddressMap_MissingSource(t *testing.T) {
	sam := NewSourceAddressMap()
	_, ok := sam.GetAddressForSource("missing.kark", 1)
	if ok {
		t.Error("expected false for missing source location")
	}
}

func TestSourceAddressMap_MissingAddress(t *testing.T) {
	sam := NewSourceAddressMap()
	_, ok := sam.GetSourceForAddress(0xDEAD)
	if ok {
		t.Error("expected false for missing address")
	}
}

func TestBuildFromDebugInfo(t *testing.T) {
	dbg := &DebugInfo{
		SourceFiles: []*SourceFile{
			{Path: "main.kark", Directory: "."},
			{Path: "lib.kark", Directory: "./lib"},
		},
		LineInfo: []*LineInfo{
			{Address: 0x1000, FileIndex: 0, Line: 5, Column: 1, Length: 4, Function: "main"},
			{Address: 0x1004, FileIndex: 0, Line: 6, Column: 1, Length: 4, Function: "main"},
			{Address: 0x2000, FileIndex: 1, Line: 3, Column: 1, Length: 8, Function: "helper"},
		},
	}

	sam := NewSourceAddressMap()
	sam.BuildFromDebugInfo(dbg)

	// Forward
	addr, ok := sam.GetAddressForSource("main.kark", 5)
	if !ok || addr != 0x1000 {
		t.Errorf("expected 0x1000 for main.kark:5, got 0x%x", addr)
	}
	addr, ok = sam.GetAddressForSource("lib.kark", 3)
	if !ok || addr != 0x2000 {
		t.Errorf("expected 0x2000 for lib.kark:3, got 0x%x", addr)
	}

	// Reverse
	loc, ok := sam.GetSourceForAddress(0x1004)
	if !ok || loc.Function != "main" || loc.Line != 6 {
		t.Errorf("unexpected reverse result: %+v (ok=%v)", loc, ok)
	}
}

func TestCollectDebugInfoFromAST(t *testing.T) {
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.FuncDecl{
				Name:   "main",
				Params: []string{},
				Body: []parser.Node{
					&parser.VarDeclStmt{
						Name:  "x",
						Type:  "int",
						Value: &parser.IntLiteral{Value: "5"},
					},
					&parser.ReturnStmt{
						Value: &parser.IntLiteral{Value: "0"},
					},
				},
			},
		},
	}

	dib := NewDebugInfoBuilder()
	if err := dib.CollectDebugInfoFromAST(prog, "test.kark"); err != nil {
		t.Fatalf("CollectDebugInfoFromAST failed: %v", err)
	}

	dbi := dib.GetDebugInfo()

	// Source file registered
	if len(dbi.SourceFiles) != 1 || dbi.SourceFiles[0].Path != "test.kark" {
		t.Errorf("expected source file 'test.kark', got %v", dbi.SourceFiles)
	}

	// Function info populated
	if len(dbi.FunctionInfo) == 0 {
		t.Fatal("expected at least one FunctionInfo")
	}
	fn := dbi.FunctionInfo[0]
	if fn.Name != "main" {
		t.Errorf("expected function 'main', got '%s'", fn.Name)
	}

	// Variable info populated
	foundX := false
	for _, v := range dbi.Variables {
		if v.Name == "x" {
			foundX = true
			if !v.IsLocal {
				t.Error("expected variable 'x' to be local")
			}
			if v.Function != "main" {
				t.Errorf("expected variable 'x' in function 'main', got '%s'", v.Function)
			}
		}
	}
	if !foundX {
		t.Error("expected variable 'x' in Variables")
	}

	// LineInfo populated
	if len(dbi.LineInfo) == 0 {
		t.Error("expected at least one LineInfo entry")
	}
}

func TestLookupHelpers(t *testing.T) {
	dbg := &DebugInfo{
		SourceFiles: []*SourceFile{{Path: "main.kark"}},
		LineInfo: []*LineInfo{
			{Address: 0x401000, FileIndex: 0, Line: 10, Column: 5, Length: 4, Function: "main"},
		},
	}

	addr, ok := LookupAddressBySource(dbg, "main.kark", 10)
	if !ok || addr != 0x401000 {
		t.Errorf("LookupAddressBySource: expected 0x401000, got 0x%x (ok=%v)", addr, ok)
	}

	loc, ok := LookupSourceByAddress(dbg, 0x401000)
	if !ok {
		t.Fatal("LookupSourceByAddress failed")
	}
	if loc.File != "main.kark" || loc.Line != 10 || loc.Function != "main" {
		t.Errorf("unexpected source location: %+v", loc)
	}
}

func TestGenerateDebugMap(t *testing.T) {
	dbg := &DebugInfo{
		SourceFiles: []*SourceFile{
			{Path: "a.kark"}, {Path: "b.kark"},
		},
		LineInfo: []*LineInfo{
			{Address: 0x1000, FileIndex: 0, Line: 1, Column: 1, Length: 1, Function: "fn1"},
			{Address: 0x2000, FileIndex: 1, Line: 5, Column: 1, Length: 1, Function: "fn2"},
		},
	}

	m := GenerateDebugMap(dbg)
	if m["a.kark"][1] != 0x1000 {
		t.Errorf("expected a.kark:1 -> 0x1000, got 0x%x", m["a.kark"][1])
	}
	if m["b.kark"][5] != 0x2000 {
		t.Errorf("expected b.kark:5 -> 0x2000, got 0x%x", m["b.kark"][5])
	}
}

func TestFormatHelpers(t *testing.T) {
	if got := FormatAddress(0x401020); got != "0x401020" {
		t.Errorf("FormatAddress: expected '0x401020', got '%s'", got)
	}
	loc := &SourceLocation{File: "main.kark", Line: 10, Column: 5, Function: "main"}
	if got := FormatSourceLocation(loc); got != "main.kark:10" {
		t.Errorf("FormatSourceLocation: expected 'main.kark:10', got '%s'", got)
	}
	if got := FormatSourceLocation(nil); got != "<unknown>" {
		t.Errorf("FormatSourceLocation(nil): expected '<unknown>', got '%s'", got)
	}
}
