package hir

import (
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

func parseSource(t *testing.T, src string) *parser.Program {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse errors: %v", p.Errors)
	}
	return prog
}

func TestBuildHIR_HelloWorld(t *testing.T) {
	src := `func main() {
    print("Hello, World!")
}`
	prog := parseSource(t, src)
	m := BuildHIR(prog)

	if len(m.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(m.Functions))
	}
	if m.Functions[0].Name != "main" {
		t.Errorf("expected function name 'main', got %q", m.Functions[0].Name)
	}
	if len(m.Functions[0].Body) != 1 {
		t.Fatalf("expected 1 statement in body, got %d", len(m.Functions[0].Body))
	}
	if m.Functions[0].Body[0].Kind != StmtPrint {
		t.Errorf("expected print statement, got %d", m.Functions[0].Body[0].Kind)
	}
}

func TestBuildHIR_FunctionWithParams(t *testing.T) {
	src := `func add(a, b) {
    return a + b
}`
	prog := parseSource(t, src)
	m := BuildHIR(prog)

	if len(m.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(m.Functions))
	}
	fn := m.Functions[0]
	if fn.Name != "add" {
		t.Errorf("expected function name 'add', got %q", fn.Name)
	}
	if len(fn.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(fn.Params))
	}
	if fn.Params[0].Name != "a" {
		t.Errorf("expected param name 'a', got %q", fn.Params[0].Name)
	}
	if fn.Params[1].Name != "b" {
		t.Errorf("expected param name 'b', got %q", fn.Params[1].Name)
	}
}

func TestBuildHIR_IfElse(t *testing.T) {
	src := `func check(x) {
    if x > 0 {
        print("positive")
    } else {
        print("non-positive")
    }
}`
	prog := parseSource(t, src)
	m := BuildHIR(prog)

	if len(m.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(m.Functions))
	}
	if len(m.Functions[0].Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(m.Functions[0].Body))
	}
	ifStmt := m.Functions[0].Body[0]
	if ifStmt.Kind != StmtIf {
		t.Fatalf("expected if statement, got %d", ifStmt.Kind)
	}
	if len(ifStmt.IfElse) == 0 {
		t.Error("expected else branch")
	}
}

func TestBuildHIR_Variables(t *testing.T) {
	src := `func main() {
    let x = 42
    let y = "hello"
    let z = true
}`
	prog := parseSource(t, src)
	m := BuildHIR(prog)

	if len(m.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(m.Functions))
	}
	if len(m.Functions[0].Body) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(m.Functions[0].Body))
	}
	for i, name := range []string{"x", "y", "z"} {
		if m.Functions[0].Body[i].VarName != name {
			t.Errorf("expected var name %q, got %q", name, m.Functions[0].Body[i].VarName)
		}
	}
}

func TestBuildHIR_BinaryExpr(t *testing.T) {
	src := `func calc() {
    let result = 1 + 2 * 3
}`
	prog := parseSource(t, src)
	m := BuildHIR(prog)

	if len(m.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(m.Functions))
	}
	if len(m.Functions[0].Body) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(m.Functions[0].Body))
	}
	stmt := m.Functions[0].Body[0]
	if stmt.Kind != StmtVarDecl {
		t.Fatalf("expected var decl, got %d", stmt.Kind)
	}
	if stmt.VarValue.Kind != ExprBinary {
		t.Fatalf("expected binary expr, got %d", stmt.VarValue.Kind)
	}
}

func TestFormat_Module(t *testing.T) {
	src := `func main() {
    let x = 42
    print(x)
}`
	prog := parseSource(t, src)
	m := BuildHIR(prog)

	output := m.Format()
	if !strings.Contains(output, "fn main()") {
		t.Errorf("formatted output should contain 'fn main()', got:\n%s", output)
	}
	if !strings.Contains(output, "let x = 42") {
		t.Errorf("formatted output should contain 'let x = 42', got:\n%s", output)
	}
	if !strings.Contains(output, "print x") {
		t.Errorf("formatted output should contain 'print x', got:\n%s", output)
	}
}

func TestRoundTrip_5Programs(t *testing.T) {
	programs := []struct {
		name string
		src  string
	}{
		{
			name: "hello",
			src: `func main() {
    print("Hello, World!")
}`,
		},
		{
			name: "arithmetic",
			src: `func calc(a, b) {
    return a + b * 2
}`,
		},
		{
			name: "control_flow",
			src: `func check(x) {
    if x > 0 {
        print("positive")
    } else {
        print("negative")
    }
}`,
		},
		{
			name: "variables",
			src: `func main() {
    let x = 10
    let y = 20
    let sum = x + y
    print(sum)
}`,
		},
		{
			name: "while_loop",
			src: `func count(n) {
    let i = 0
    while i < n {
        print(i)
        i = i + 1
    }
}`,
		},
	}

	for _, prog := range programs {
		t.Run(prog.name, func(t *testing.T) {
			ast := parseSource(t, prog.src)
			hir := BuildHIR(ast)

			if hir == nil {
				t.Fatal("BuildHIR returned nil")
			}

			output := hir.Format()
			if output == "" {
				t.Fatal("Format returned empty string")
			}

			// Verify the output is valid (non-empty, contains the function name)
			if !strings.Contains(output, "fn ") {
				t.Errorf("output should contain 'fn ', got:\n%s", output)
			}
		})
	}
}
