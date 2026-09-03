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
