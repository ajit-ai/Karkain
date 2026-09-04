package lexer

import (
	"testing"
)

func TestTokenOffsets(t *testing.T) {
	input := "func main() { let x int = 42 }"
	l := New(input)
	tokens := []Token{}
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF {
			break
		}
	}

	expected := []struct {
		typ     TokenType
		literal string
	}{
		{TokenFunc, "func"},
		{TokenIdent, "main"},
		{TokenLParen, "("},
		{TokenRParen, ")"},
		{TokenLBrace, "{"},
		{TokenLet, "let"},
		{TokenIdent, "x"},
		{TokenIdent, "int"},
		{TokenAssign, "="},
		{TokenInt, "42"},
		{TokenRBrace, "}"},
		{TokenEOF, ""},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}

	for i, exp := range expected {
		tok := tokens[i]
		if tok.Type != exp.typ {
			t.Errorf("token %d: expected type %s, got %s", i, exp.typ, tok.Type)
		}
		got := tok.Literal(input)
		if got != exp.literal {
			t.Errorf("token %d: expected literal %q, got %q (Start=%d Len=%d)", i, exp.literal, got, tok.Start, tok.Len)
		}
	}
}

func TestTokenZeroCopy(t *testing.T) {
	input := "func compute(x int, y int) int { return x + y }"
	l := New(input)

	for {
		tok := l.NextToken()
		if tok.Type == TokenEOF {
			break
		}
		lit := tok.Literal(input)
		if len(lit) > 0 {
			if input[tok.Start:tok.Start+uint32(tok.Len)] != lit {
				t.Errorf("token literal mismatch: offset-based=%q, method=%q",
					input[tok.Start:tok.Start+uint32(tok.Len)], lit)
			}
		}
	}
}

func TestTokenLineCol(t *testing.T) {
	input := "let a\nlet b"
	l := New(input)

	tok := l.NextToken()
	if tok.Line != 1 || tok.Col != 1 {
		t.Errorf("expected line 1 col 1 for 'let', got line %d col %d", tok.Line, tok.Col)
	}
	tok = l.NextToken()
	if tok.Line != 1 || tok.Col != 5 {
		t.Errorf("expected line 1 col 5 for 'a', got line %d col %d", tok.Line, tok.Col)
	}
	tok = l.NextToken()
	if tok.Line != 2 || tok.Col != 1 {
		t.Errorf("expected line 2 col 1 for second 'let', got line %d col %d", tok.Line, tok.Col)
	}
	tok = l.NextToken()
	if tok.Line != 2 || tok.Col != 5 {
		t.Errorf("expected line 2 col 5 for 'b', got line %d col %d", tok.Line, tok.Col)
	}
}

func TestMultiCharOperators(t *testing.T) {
	input := "== != <= >= && ||"
	l := New(input)
	expected := []struct {
		typ     TokenType
		literal string
	}{
		{TokenEqual, "=="},
		{TokenNotEqual, "!="},
		{TokenLessEqual, "<="},
		{TokenGreaterEqual, ">="},
		{TokenAnd, "&&"},
		{TokenOr, "||"},
	}

	for i, exp := range expected {
		tok := l.NextToken()
		if tok.Type != exp.typ {
			t.Errorf("token %d: expected type %s, got %s", i, exp.typ, tok.Type)
		}
		got := tok.Literal(input)
		if got != exp.literal {
			t.Errorf("token %d: expected %q, got %q", i, exp.literal, got)
		}
	}
}

func TestStringTokenOffset(t *testing.T) {
	input := "\"hello world\""
	l := New(input)
	tok := l.NextToken()

	if tok.Type != TokenString {
		t.Fatalf("expected TokenString, got %s", tok.Type)
	}
	got := tok.Literal(input)
	if got != "hello world" {
		t.Errorf("expected 'hello world', got %q", got)
	}
}

func TestStringTokenWithEscapes(t *testing.T) {
	input := "\"line1\\nline2\""
	l := New(input)
	tok := l.NextToken()

	if tok.Type != TokenString {
		t.Fatalf("expected TokenString, got %s", tok.Type)
	}
	got := tok.Literal(input)
	if got != `line1\nline2` {
		t.Errorf("expected raw 'line1\\nline2', got %q", got)
	}
}

// TestStringTokenEscapedQuote guards the Phase 56-C fix: the lexer used to drop an
// escaped character from the token length, so a string like `"\""` tokenized only
// `\` instead of the full `\"` content. This mis-tokenization corrupted the
// self-hosted compiler's emitted C (e.g. a lone `\"` made a whole call argument
// vanish), which caused the Stage2 gcc failures.
func TestStringTokenEscapedQuote(t *testing.T) {
	input := `"\""`
	l := New(input)
	tok := l.NextToken()

	if tok.Type != TokenString {
		t.Fatalf("expected TokenString, got %s", tok.Type)
	}
	got := tok.Literal(input)
	if got != `\"` {
		t.Errorf("expected raw '\\\"' content, got %q", got)
	}
}

func TestFloatToken(t *testing.T) {
	input := "3.14159"
	l := New(input)
	tok := l.NextToken()

	if tok.Type != TokenFloat64 {
		t.Fatalf("expected TokenFloat64, got %s", tok.Type)
	}
	got := tok.Literal(input)
	if got != "3.14159" {
		t.Errorf("expected '3.14159', got %q", got)
	}
}

func TestGetInputBytes(t *testing.T) {
	input := "let x = 42"
	l := New(input)

	if string(l.GetInputBytes()) != l.GetInput() {
		t.Error("GetInputBytes and GetInput return different data")
	}
}

func TestArrowToken(t *testing.T) {
	input := "->"
	l := New(input)
	tok := l.NextToken()
	got := tok.Literal(input)
	if got != "-" {
		t.Errorf("expected '-' (Go lexer returns single char), got %q", got)
	}
}

func TestBangEqualsToken(t *testing.T) {
	input := "!="
	l := New(input)
	tok := l.NextToken()

	if tok.Type != TokenNotEqual {
		t.Fatalf("expected TokenNotEqual, got %s", tok.Type)
	}
	got := tok.Literal(input)
	if got != "!=" {
		t.Errorf("expected '!=', got %q", got)
	}
}

func TestBigIntToken(t *testing.T) {
	input := "42n"
	l := New(input)
	tok := l.NextToken()

	if tok.Type != TokenBigInt {
		t.Fatalf("expected TokenBigInt, got %s", tok.Type)
	}
	got := tok.Literal(input)
	if got != "42n" {
		t.Errorf("expected '42n', got %q", got)
	}
}

func TestBigFloatToken(t *testing.T) {
	input := "3.14b"
	l := New(input)
	tok := l.NextToken()

	if tok.Type != TokenBigFloat {
		t.Fatalf("expected TokenBigFloat, got %s", tok.Type)
	}
	got := tok.Literal(input)
	if got != "3.14b" {
		t.Errorf("expected '3.14b', got %q", got)
	}
}

func TestBigIntKeyword(t *testing.T) {
	input := "bigint"
	l := New(input)
	tok := l.NextToken()

	if tok.Type != TokenBigInt {
		t.Fatalf("expected TokenBigInt for keyword 'bigint', got %s", tok.Type)
	}
}

func TestBigFloatKeyword(t *testing.T) {
	input := "bigfloat"
	l := New(input)
	tok := l.NextToken()

	if tok.Type != TokenBigFloat {
		t.Fatalf("expected TokenBigFloat for keyword 'bigfloat', got %s", tok.Type)
	}
}

func TestBigIntLargeNumber(t *testing.T) {
	input := "9999999999999999999999999999999999999n"
	l := New(input)
	tok := l.NextToken()

	if tok.Type != TokenBigInt {
		t.Fatalf("expected TokenBigInt, got %s", tok.Type)
	}
	got := tok.Literal(input)
	if got != "9999999999999999999999999999999999999n" {
		t.Errorf("expected large bigint literal, got %q", got)
	}
}

// A leading UTF-8 byte-order mark must be skipped and must not corrupt the
// first token. Files saved with a BOM (common on Windows editors) concatenated
// with siblings used to silently drop their first declaration.
func TestLeadingBOMSkipped(t *testing.T) {
	input := "\xEF\xBB\xBFabc123"
	l := New(input)
	tok := l.NextToken()
	if tok.Type == TokenIllegal {
		t.Fatalf("expected a valid first token after BOM, got %s", tok.Type)
	}
	// Offsets are relative to the BOM-stripped buffer; the first identifier
	// token must begin at the very start of the stripped source.
	if tok.Start != 0 {
		t.Errorf("expected first token offset 0, got %d", tok.Start)
	}
	got := string(tok.LiteralBytes(l.GetInputBytes()))
	if got != "abc123" {
		t.Errorf("expected literal %q sans BOM, got %q", "abc123", got)
	}
}

// The BOM fix must keep the leading `func` of a concatenated first file intact
// so a sibling function is not silently dropped before `main`.
func TestBOMDoesNotDropLeadingFunc(t *testing.T) {
	input := "\xEF\xBB\xBFfunc lib_add(a) { return a }\nfunc main() {}\n"
	l := New(input)
	types := []TokenType{}
	for {
		tok := l.NextToken()
		if tok.Type == TokenEOF {
			break
		}
		types = append(types, tok.Type)
	}
	if len(types) == 0 {
		t.Fatal("expected tokens")
	}
	if types[0] != TokenFunc {
		t.Fatalf("expected first token TokenFunc, got %s", types[0])
	}
}
