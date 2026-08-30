package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

func loadCompilerSource(t *testing.T) string {
	t.Helper()
	root := filepath.Join("..", "..")
	srcDir := filepath.Join(root, "src", "compiler")

	files := []string{"ast.kark", "lexer.kark", "parser.kark", "sema.kark", "codegen.kark", "main.kark"}
	var full strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(srcDir, f))
		if err != nil {
			t.Fatalf("Error reading %s: %v", f, err)
		}
		full.Write(data)
		full.WriteString("\n\n")
	}
	return full.String()
}

func TestCodeGenProfile(t *testing.T) {
	source := loadCompilerSource(t)
	t.Logf("Total source: %d bytes", len(source))

	t0 := time.Now()
	l := lexer.New(source)
	p := parser.New(l)
	prog := p.ParseProgram()
	t.Logf("Parse: %v (%d statements)", time.Since(t0), len(prog.Statements))

	_ = parser.ApplyMacroExpansion(prog)

	t1 := time.Now()
	cfg := codegen.NewConfig()
	cfg.CompileOnly = true
	g := codegen.New(cfg)
	err := g.GenerateAndCompile(prog, filepath.Join("..", "..", "src", "compiler", "main.kark"))
	t.Logf("Codegen (CompileOnly=true): %v (err=%v)", time.Since(t1), err)
}
