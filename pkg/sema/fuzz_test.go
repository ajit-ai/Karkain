package sema

import (
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// v1.2.27 fuzzing foundation (REL-003): the resolver and monomorphization
// pass must never panic on an arbitrary parsed program. Diagnostics are
// expected; only a panic or a hang is a finding.
func FuzzResolverNeverPanics(f *testing.F) {
	f.Add("func main() { let x = 1; print(x) }")
	f.Add("func id[T](x T) { return x } func main() { print(id(1)) }")
	f.Add("trait T { fn f(x int) int; } impl T for int { } func main() { print(1) }")
	f.Add("func main() { undefined(1) }")
	f.Add("\x00\xff")
	f.Fuzz(func(t *testing.T, src string) {
		p := parser.New(lexer.New(src))
		prog := p.ParseProgram()
		if prog == nil {
			return
		}
		prog = parser.ApplyMacroExpansion(prog)
		if prog == nil {
			return
		}
		r := NewResolver(prog, nil)
		_ = r.Resolve()
		_ = MonomorphizeProgram(prog)
	})
}
