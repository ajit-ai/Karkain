package parser

import (
	"testing"

	"karkain/pkg/lexer"
)

// v1.2.31: async/await are sugar over the existing spawn/join runtime.
// Lowering happens in the parser so no new AST node has to be understood by
// sema, HIR, IR, codegen or the native backend.
func TestPhaseV1231_AsyncAwaitLowerToSpawnJoin(t *testing.T) {
	src := "func double(x) { return x * 2 }\nfunc main() { let t = async double(21); print(await t) }\n"
	p := New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse errors: %v", p.Errors)
	}
	mainFn, ok := prog.Statements[1].(*FuncDecl)
	if !ok || mainFn.Name != "main" {
		t.Fatalf("second statement is not main: %T", prog.Statements[1])
	}
	decl, ok := mainFn.Body[0].(*VarDeclStmt)
	if !ok {
		t.Fatalf("first statement is not a declaration: %T", mainFn.Body[0])
	}
	spawn, ok := decl.Value.(*SpawnExpr)
	if !ok {
		t.Fatalf("async did not lower to SpawnExpr: %T", decl.Value)
	}
	if spawn.ActorName != "double" || len(spawn.Args) != 1 {
		t.Errorf("bad spawn lowering: %+v", spawn)
	}
	printStmt, ok := mainFn.Body[1].(*PrintStmt)
	if !ok {
		t.Fatalf("second statement is not a print: %T", mainFn.Body[1])
	}
	call, ok := printStmt.Value.(*CallExpr)
	if !ok {
		t.Fatalf("print argument is not a call: %T", printStmt.Value)
	}
	if call.Function != "join" || len(call.Args) != 1 {
		t.Fatalf("await did not lower to join(t): %+v", call)
	}
}
