package lexer

import "testing"

// v1.2.27 fuzzing foundation (REL-003): the lexer must never panic or spin
// forever on arbitrary bytes. A hard token bound makes a non-terminating
// token stream fail deterministically instead of hanging the fuzz worker.
func FuzzLexerNeverPanics(f *testing.F) {
	f.Add("func main() { print(42) }")
	f.Add("let x = \"unterminated")
	f.Add("/* unterminated comment")
	f.Add("trait Numeric { fn add(x int) int; }")
	f.Add("\x00\xff\x01")
	f.Fuzz(func(t *testing.T, src string) {
		l := New(src)
		for i := 0; i < 10000; i++ {
			if l.NextToken().Type == TokenEOF {
				return
			}
		}
		t.Fatal("lexer did not reach EOF within 10000 tokens")
	})
}
