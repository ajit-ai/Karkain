package codegen

import (
	"strings"
	"testing"

	"karkain/pkg/parser"
)

// Phase 101 gate: the runtime debugger foundation. Generated C must embed the
// frame-stack API, and every generated function must push a named frame at its
// prologue, stamp the current source line, and pop the frame when it leaves, so
// runtime errors can report the Karkain call chain with source file and line.

// TestPhase101_FrameStackInHeader verifies the runtime header carries the
// frame-stack helpers and the "  stack:" dump used to render call chains.
func TestPhase101_FrameStackInHeader(t *testing.T) {
	g := New(Config{})
	h := g.generateCHeader()
	for _, fn := range []string{
		"karkain_frame_enter",
		"karkain_frame_leave",
		"karkain_set_line",
		"KARKAIN_MAX_FRAMES",
		"karkain_frames",
		"  stack:",
	} {
		if !strings.Contains(h, fn) {
			t.Errorf("generated header missing %q", fn)
		}
	}
}

// TestPhase101_FuncDeclPrologueFrames verifies every generated function pushes a
// named frame and stamps its definition line right after the opening brace.
func TestPhase101_FuncDeclPrologueFrames(t *testing.T) {
	g := New(Config{})
	g.sourceFile = "probe.kark"

	helper := g.genFuncDecl(&parser.FuncDecl{Name: "helper", Line: 3})
	if !strings.Contains(helper, `karkain_frame_enter("helper", "probe.kark");`) {
		t.Errorf("helper prologue missing frame enter:\n%s", helper)
	}
	if !strings.Contains(helper, "karkain_set_line(3);") {
		t.Errorf("helper prologue missing set_line(3):\n%s", helper)
	}

	entry := g.genFuncDecl(&parser.FuncDecl{Name: "main", Line: 9})
	if !strings.Contains(entry, `karkain_frame_enter("main", "probe.kark");`) {
		t.Errorf("main prologue missing frame enter:\n%s", entry)
	}
	if !strings.Contains(entry, "karkain_set_line(9);") {
		t.Errorf("main prologue missing set_line(9):\n%s", entry)
	}
}

// TestPhase101_ReturnPopsFrameAfterValue verifies a return evaluates its value
// before popping the frame: if the expression itself raises (a checked call),
// the function's frame must still be on the stack.
func TestPhase101_ReturnPopsFrameAfterValue(t *testing.T) {
	g := New(Config{})
	ret := g.genStatement(&parser.ReturnStmt{
		Value: &parser.IntLiteral{Value: "1"},
		Line:  7,
	})
	if !strings.Contains(ret, "{ Value _karkain_fret = make_int(1); karkain_frame_leave(); return _karkain_fret; }") {
		t.Errorf("return emission must evaluate before popping the frame:\n%s", ret)
	}
}

// TestPhase101_GetArgsFramed verifies the canonical getArgs body participates in
// the frame stack too.
func TestPhase101_GetArgsFramed(t *testing.T) {
	g := New(Config{})
	g.sourceFile = "probe.kark"
	fn := g.genFuncDecl(&parser.FuncDecl{Name: "getArgs", Params: []string{}})
	if !strings.Contains(fn, `karkain_frame_enter("getArgs", "probe.kark");`) {
		t.Errorf("getArgs missing frame enter:\n%s", fn)
	}
	if !strings.Contains(fn, "karkain_frame_leave();") {
		t.Errorf("getArgs missing frame leave:\n%s", fn)
	}
}