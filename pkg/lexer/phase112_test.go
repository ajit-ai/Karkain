package lexer

import (
	"testing"
)

// Phase 112 gate: the `const` keyword lexes as its own token type and the
// lexer accepts nested block comments /* ... */ with correct line/column
// tracking, matching the self-hosted kcc engine's comment rules.

func TestPhase112_ConstToken(t *testing.T) {
	l := New("let a; var b; const MAX = 10")
	var got []TokenType
	for {
		tok := l.NextToken()
		if tok.Type == TokenEOF {
			break
		}
		if tok.Type != TokenIdent && tok.Type != TokenAssign &&
			tok.Type != TokenInt && tok.Type != TokenSemicolon {
			got = append(got, tok.Type)
		}
	}
	for _, tt := range []TokenType{TokenLet, TokenVar, TokenConst} {
		for _, g := range got {
			if g == tt {
				goto found
			}
		}
		t.Errorf("expected token %q in stream, got %v", tt, got)
	found:
	}
}

func TestPhase112_BlockCommentSingleLine(t *testing.T) {
	l := New("let a /* ignore this */ = 1")
	tok := l.NextToken()
	if tok.Type != TokenLet {
		t.Fatalf("expected LET, got %v", tok)
	}
	tok = l.NextToken()
	if tok.Type != TokenIdent || string(tok.LiteralBytes(l.Input)) != "a" {
		t.Fatalf("expected ident 'a', got %v %q", tok.Type, string(tok.LiteralBytes(l.Input)))
	}
	tok = l.NextToken()
	if tok.Type != TokenAssign {
		t.Fatalf("expected ASSIGN after block comment, got %v", tok)
	}
if tok.Col != 24 {
		t.Errorf("expected assign col 24 after comment, got %d", tok.Col)
	}
	tok = l.NextToken()
	if tok.Type != TokenInt || string(tok.LiteralBytes(l.Input)) != "1" {
		t.Fatalf("expected INT 1, got %v %q", tok.Type, string(tok.LiteralBytes(l.Input)))
	}
}

func TestPhase112_BlockCommentMultiLine(t *testing.T) {
	l := New("let a\n/* line one\nline two */\nlet b")
	tok := l.NextToken()
	if tok.Type != TokenLet || tok.Line != 1 {
		t.Fatalf("expected LET line1, got %v line %d", tok.Type, tok.Line)
	}
	tok = l.NextToken()
	if tok.Type != TokenIdent || tok.Line != 1 {
		t.Fatalf("expected ident line1, got %v line %d", tok.Type, tok.Line)
	}
	tok = l.NextToken()
	if tok.Type != TokenLet || tok.Line != 4 {
		t.Fatalf("expected LET line4 (after 2-line comment), got %v line %d", tok.Type, tok.Line)
	}
}

func TestPhase112_BlockCommentNested(t *testing.T) {
	l := New("let a /* outer /* inner */ still outer */ let b")
	tok := l.NextToken()
	if tok.Type != TokenLet {
		t.Fatalf("expected LET, got %v", tok)
	}
	tok = l.NextToken()
	if tok.Type != TokenIdent || string(tok.LiteralBytes(l.Input)) != "a" {
		t.Fatalf("expected ident 'a', got %v %q", tok.Type, string(tok.LiteralBytes(l.Input)))
	}
	// The nested comment consumes everything, so the next token is the second let.
	tok = l.NextToken()
	if tok.Type != TokenLet {
		t.Fatalf("expected nested comment to close at inner terminator, got %v %q", tok.Type, string(tok.LiteralBytes(l.Input)))
	}
	tok = l.NextToken()
	if tok.Type != TokenIdent || string(tok.LiteralBytes(l.Input)) != "b" {
		t.Fatalf("expected ident 'b', got %v %q", tok.Type, string(tok.LiteralBytes(l.Input)))
	}
}

func TestPhase112_LineCommentStillWorks(t *testing.T) {
	l := New("let a // comment\nlet b")
	tok := l.NextToken()
	if tok.Type != TokenLet || tok.Line != 1 {
		t.Fatalf("expected LET line1, got %v line %d", tok.Type, tok.Line)
	}
	tok = l.NextToken()
	if tok.Type != TokenIdent || tok.Line != 1 {
		t.Fatalf("expected ident line1, got %v line %d", tok.Type, tok.Line)
	}
	tok = l.NextToken()
	if tok.Type != TokenLet || tok.Line != 2 {
		t.Fatalf("expected LET line2 after line comment, got %v line %d", tok.Type, tok.Line)
	}
}

