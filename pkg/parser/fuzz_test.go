package parser

import (
	"testing"

	"karkain/pkg/lexer"
)

// v1.2.27 fuzzing foundation (REL-003): arbitrary input must not panic the
// parser. Parse errors are expected and are deliberately not assertions —
// only a panic, a hang, or an out-of-range AST write is a finding here.
func FuzzParserNeverPanics(f *testing.F) {
	f.Add("func main() { print(1 + 2) }")
	f.Add("let = 42")
	f.Add("func main() { if (true) { print(1) } }")
	f.Add("trait T { fn f(x int) int; } impl T for int { func f(x int) int { return x } }")
	f.Add("\x00\xff")
	f.Fuzz(func(t *testing.T, src string) {
		p := New(lexer.New(src))
		_ = p.ParseProgram()
	})
}
