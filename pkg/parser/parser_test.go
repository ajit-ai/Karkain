package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// parseSource is defined in quantum_test.go (shared package helper).

// parseBounded runs ParseProgram on a goroutine and fails the test if it does not
// return within the timeout. This guards against unbounded-loop (OOM) regressions.
func parseBounded(t *testing.T, src string) *Program {
	t.Helper()
	done := make(chan *Program, 1)
	go func() {
		done <- parseSource(t, src)
	}()
	select {
	case prog := <-done:
		return prog
	case <-time.After(10 * time.Second):
		t.Fatal("parser did not terminate (stall/OOM regression)")
		return nil
	}
}

// TestCallArgStallDoesNotOOM guards the Phase 56-C regression: an unexpected (illegal)
// token inside a call argument list used to make parsePrimaryExpr return nil without
// advancing, so the argument loop appended forever and exhausted memory. It must now
// terminate quickly and report a parse error.
func TestCallArgStallDoesNotOOM(t *testing.T) {
	var src strings.Builder
	src.WriteString("func main() {\n")
	for i := 0; i < 200; i++ {
		src.WriteString("    f(")
		src.WriteString(strings.Repeat("1,", 50))
		src.WriteString("\x01) // \x01 is an illegal token\n")
	}
	src.WriteString("}\n")

	prog := parseBounded(t, src.String())
	if prog == nil {
		t.Fatal("ParseProgram returned nil")
	}
}

// TestCompilerSourcesParseWithoutOOM is a second guard on the original OOM: the
// concatenated self-hosted compiler sources must parse without unbounded memory use.
func TestCompilerSourcesParseWithoutOOM(t *testing.T) {
	root := filepath.Join("..", "..")
	srcDir := filepath.Join(root, "src", "compiler")
	files := []string{"ast.kark", "lexer.kark", "parser.kark", "sema.kark", "codegen.kark", "main.kark"}

	var full strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(srcDir, f))
		if err != nil {
			t.Skipf("compiler source %s not found (skipping): %v", f, err)
		}
		full.Write(data)
		full.WriteString("\n\n")
	}

	prog := parseBounded(t, full.String())
	if prog == nil {
		t.Fatal("ParseProgram returned nil")
	}
}

func TestParseFunctionCall(t *testing.T) {
	prog := parseSource(t, "func main() { println(1, 2, 3) }")
	body := findFuncBody(prog)
	if body == nil {
		t.Fatal("main function not found")
	}
}

func TestParseDotCall(t *testing.T) {
	prog := parseSource(t, "func main() { a := C.sqrt(4.0) }")
	body := findFuncBody(prog)
	if body == nil {
		t.Fatal("main function not found")
	}
}

func TestParseArrayLiteral(t *testing.T) {
	prog := parseSource(t, "func main() { a := [1, 2, 3] }")
	body := findFuncBody(prog)
	if body == nil {
		t.Fatal("main function not found")
	}
}

func TestParseMapLiteral(t *testing.T) {
	prog := parseSource(t, "func main() { m := {1: 2, 3: 4} }")
	body := findFuncBody(prog)
	if body == nil {
		t.Fatal("main function not found")
	}
}

func TestParseMatch(t *testing.T) {
	prog := parseSource(t, "func main() { let x = 5; match x { 1 => a(), _ => b() } }")
	body := findFuncBody(prog)
	if len(body) != 2 {
		t.Fatalf("expected 2 body statements, got %d", len(body))
	}
}

// exprStmtCall unwraps the ExprStmt at index i and returns the inner CallExpr
// (used by the chained-index / postfix-operator regression tests).
func exprStmtCall(body []Node, i int) *CallExpr {
	if i >= len(body) {
		return nil
	}
	es, ok := body[i].(*ExprStmt)
	if !ok {
		return nil
	}
	call, ok := es.Expression.(*CallExpr)
	if !ok {
		return nil
	}
	return call
}

// TestChainedIndexBindsToSameLeft guards the Phase 56-C fix: `params[i][0]` used to
// leak the trailing [0] out as a separate expression and get mis-parsed.
func TestChainedIndexBindsToSameLeft(t *testing.T) {
	prog := parseSource(t, `func main() { emitC11Type(fields[i][0]) }`)
	call := exprStmtCall(findFuncBody(prog), 0)
	if call == nil {
		t.Fatal("expected a CallExpr statement")
	}
	if len(call.Args) != 1 {
		t.Fatalf("expected 1 call arg, got %d", len(call.Args))
	}
	outer, ok := call.Args[0].(*IndexExpr)
	if !ok {
		t.Fatalf("expected outer IndexExpr, got %T", call.Args[0])
	}
	inner, ok := outer.Left.(*IndexExpr)
	if !ok {
		t.Fatalf("expected inner IndexExpr under outer.Left, got %T", outer.Left)
	}
	ident, ok := inner.Left.(*Identifier)
	if !ok || ident.Name != "fields" {
		t.Fatalf("innermost left should be identifier 'fields', got %T", inner.Left)
	}
}

// TestIndexThenOperator guards the Phase 56-C fix: `a[i] + b` used to leave the `+`
// unconsumed so it leaked out as a spurious extra call argument (a[i], then a nil
// binary-op arg). It must now parse as a single binary argument of the call.
func TestIndexThenOperator(t *testing.T) {
	prog := parseSource(t, `func main() { emitLine(state, params[i][1] + ": " + name) }`)
	call := exprStmtCall(findFuncBody(prog), 0)
	if call == nil {
		t.Fatal("expected a CallExpr statement")
	}
	if len(call.Args) != 2 {
		t.Fatalf("expected 2 call args (state + expr), got %d", len(call.Args))
	}
	bin, ok := call.Args[1].(*BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr as 2nd arg, got %T", call.Args[1])
	}
	if bin.Operator != "+" {
		t.Fatalf("expected outer '+' operator, got %q", bin.Operator)
	}
	leftBin, ok := bin.Left.(*BinaryExpr)
	if !ok || leftBin.Operator != "+" {
		t.Fatalf("expected inner BinaryExpr '+' as left, got %T", bin.Left)
	}
	if _, ok := leftBin.Left.(*IndexExpr); !ok {
		t.Fatalf("expected IndexExpr at left of inner binary, got %T", bin.Left)
	}
}

func TestParser_PublicModifier(t *testing.T) {
	prog := parseSource(t, "public func foo() {\n  print(1)\n}\nfunc bar() {\n  print(2)\n}\n")
	if len(prog.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Statements))
	}
	fn0, ok := prog.Statements[0].(*FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl, got %T", prog.Statements[0])
	}
	if !fn0.Public {
		t.Error("expected foo to be Public")
	}
	fn1, ok := prog.Statements[1].(*FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl, got %T", prog.Statements[1])
	}
	if fn1.Public {
		t.Error("expected bar to NOT be Public")
	}
}

func TestParser_PublicStruct(t *testing.T) {
	prog := parseSource(t, "public type Foo {\n  x: int\n}\ntype Bar {\n  y: int\n}\n")
	if len(prog.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Statements))
	}
	s0, ok := prog.Statements[0].(*StructDeclStmt)
	if !ok {
		t.Fatalf("expected StructDeclStmt, got %T", prog.Statements[0])
	}
	if !s0.Public {
		t.Error("expected Foo to be Public")
	}
	s1, ok := prog.Statements[1].(*StructDeclStmt)
	if !ok {
		t.Fatalf("expected StructDeclStmt, got %T", prog.Statements[1])
	}
	if s1.Public {
		t.Error("expected Bar to NOT be Public")
	}
}

func TestParser_PublicEnum(t *testing.T) {
	prog := parseSource(t, "public enum Color {\n  Red\n  Blue\n}\nenum Shape {\n  Circle\n}\n")
	if len(prog.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Statements))
	}
	e0, ok := prog.Statements[0].(*EnumDecl)
	if !ok {
		t.Fatalf("expected EnumDecl, got %T", prog.Statements[0])
	}
	if !e0.Public {
		t.Error("expected Color to be Public")
	}
	e1, ok := prog.Statements[1].(*EnumDecl)
	if !ok {
		t.Fatalf("expected EnumDecl, got %T", prog.Statements[1])
	}
	if e1.Public {
		t.Error("expected Shape to NOT be Public")
	}
}

func TestParser_PublicErrorOnInvalidTarget(t *testing.T) {
	prog := parseSource(t, "public let x = 1\n")
	if len(prog.Statements) != 0 {
		t.Errorf("expected 0 valid statements after public error, got %d", len(prog.Statements))
	}
}

func TestParser_ModuleImport(t *testing.T) {
	prog := parseSource(t, "import math\nimport utils\nfunc main() { print(1) }\n")
	if len(prog.Imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(prog.Imports))
	}
	if prog.Imports[0].Name != "math" {
		t.Errorf("expected import name 'math', got '%s'", prog.Imports[0].Name)
	}
	if prog.Imports[1].Name != "utils" {
		t.Errorf("expected import name 'utils', got '%s'", prog.Imports[1].Name)
	}
}

func TestParser_CImportSkippedGracefully(t *testing.T) {
	prog := parseSource(t, "import \"C\" {\n  int strlen(const char *s);\n}\nfunc main() { print(1) }\n")
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement (func main), got %d", len(prog.Statements))
	}
}
