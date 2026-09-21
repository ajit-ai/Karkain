package lsp

// Phase 136, Slice A gate — semantic tokens.
//
// The encoder is lexer-driven and delta-encoded per the protocol. These
// tests pin the exact decoded stream (not substrings): token classes,
// positions (0-based lines, UTF-16 units), gap-recovered comments, and the
// full JSON-RPC request path with capability advertisement.

import (
	"encoding/json"
	"testing"
)

// decodedTok is one delta-decoded token for assertion.
type decodedTok struct {
	line   int
	start  int
	length int
	typ    uint32
}

func decodeTokens(t *testing.T, data []uint32) []decodedTok {
	t.Helper()
	if len(data)%5 != 0 {
		t.Fatalf("token data length %d is not a multiple of 5", len(data))
	}
	var out []decodedTok
	line, start := 0, 0
	for i := 0; i+4 < len(data); i += 5 {
		dl, ds := int(data[i]), int(data[i+1])
		line += dl
		if dl == 0 && i > 0 {
			start += ds
		} else {
			start = ds
		}
		out = append(out, decodedTok{line: line, start: start, length: int(data[i+2]), typ: data[i+3]})
		if data[i+4] != 0 {
			t.Fatalf("unexpected token modifiers %d (none defined)", data[i+4])
		}
	}
	return out
}

// assertSortedBounds enforces the protocol contract: sorted by (line,
// start) with every span inside its line.
func assertSortedBounds(t *testing.T, text string, toks []decodedTok) {
	t.Helper()
	lines := splitLines(text)
	for i, e := range toks {
		if i > 0 {
			prev := toks[i-1]
			if e.line < prev.line || (e.line == prev.line && e.start < prev.start) {
				t.Fatalf("tokens not sorted at index %d: %+v after %+v", i, e, prev)
			}
		}
		if e.line < 0 || e.line >= len(lines) {
			t.Fatalf("token %+v outside %d lines", e, len(lines))
		}
		if e.start < 0 || e.start+e.length > utf16Units(lines[e.line]) {
			t.Fatalf("token %+v exceeds line %d (%q)", e, e.line, lines[e.line])
		}
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func TestSemanticTokens_Golden(t *testing.T) {
	text := "func main() {\n    let x = 42\n    print(\"hi\") // greet\n}"
	got := decodeTokens(t, EncodeSemanticTokens(text))
	assertSortedBounds(t, text, got)

	kw, vr, nm, st, op, cm := SemTokKeyword, SemTokVariable, SemTokNumber, SemTokString, SemTokOperator, SemTokComment
	want := []decodedTok{
		{0, 0, 4, kw},  // func
		{0, 5, 4, vr},  // main
		{0, 9, 1, op},  // (
		{0, 10, 1, op}, // )
		{0, 12, 1, op}, // {
		{1, 4, 3, kw},  // let
		{1, 8, 1, vr},  // x
		{1, 10, 1, op}, // =
		{1, 12, 2, nm}, // 42
		{2, 4, 5, kw},  // print (PRINT keyword token)
		{2, 9, 1, op},  // (
		{2, 10, 4, st}, // "hi" (quotes included)
		{2, 14, 1, op}, // )
		{2, 16, 8, cm}, // // greet (gap-recovered)
		{3, 0, 1, op},  // }
	}
	if len(got) != len(want) {
		t.Fatalf("token count = %d, want %d:\n%+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSemanticTokens_UTF16(t *testing.T) {
	// Multibyte content in positions the lexer treats opaquely (string
	// literal, line comment): byte columns and UTF-16 units differ, and the
	// encoder must emit units.
	text := "let x = 1 // h\xc3\xa9llo\nprint(\"\xc3\xa9\")"
	got := decodeTokens(t, EncodeSemanticTokens(text))
	assertSortedBounds(t, text, got)

	byPos := map[[2]int]decodedTok{}
	for _, e := range got {
		byPos[[2]int{e.line, e.start}] = e
	}
	// "// héllo" starts after "let x = 1 " (10 ASCII units) with 8 units.
	if e, ok := byPos[[2]int{0, 10}]; !ok || e.typ != SemTokComment || e.length != 8 {
		t.Fatalf("comment token = %+v, want {0 10 8 comment}", e)
	}
	// "\"é\"" starts at byte col 6 ("print("), i.e. 6 units, length 3 units.
	if e, ok := byPos[[2]int{1, 6}]; !ok || e.typ != SemTokString || e.length != 3 {
		t.Fatalf("string token = %+v, want {1 6 3 string}", e)
	}
}

func TestSemanticTokens_EmptyAndBroken(t *testing.T) {
	if data := EncodeSemanticTokens(""); data == nil || len(data) != 0 {
		t.Fatalf("empty document should yield empty (non-nil) data, got %v", data)
	}

	// Unterminated string: total function, sorted, in-bounds, string present.
	broken := "func broken() {\n    let s = \"oops\n}"
	got := decodeTokens(t, EncodeSemanticTokens(broken))
	assertSortedBounds(t, broken, got)
	found := false
	for _, e := range got {
		if e.typ == SemTokString {
			found = true
		}
	}
	if !found {
		t.Fatalf("unterminated string should still classify, got %+v", got)
	}

	// Punctuation-only input classifies every char as operator, nothing lost.
	ops := decodeTokens(t, EncodeSemanticTokens("(){};,"))
	if len(ops) != 6 {
		t.Fatalf("operator run should yield 6 tokens, got %+v", ops)
	}
	for _, e := range ops {
		if e.typ != SemTokOperator || e.length != 1 {
			t.Fatalf("operator token malformed: %+v", e)
		}
	}
}

func TestSemanticTokens_FullRequest(t *testing.T) {
	tc := newTestClient()
	resp := tc.sendRequest(1, MethodInitialize, InitializeParams{
		RootURI:      "file:///workspace",
		Capabilities: ClientCapabilities{TextDocument: &TextDocumentClientCapabilities{}},
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("initialize failed: %+v", resp)
	}
	var initResult InitializeResult
	raw, _ := json.Marshal(resp.Result)
	json.Unmarshal(raw, &initResult)
	if initResult.Capabilities.SemanticTokensProvider == nil {
		t.Fatal("expected semanticTokensProvider capability")
	}
	legend := initResult.Capabilities.SemanticTokensProvider.Legend.TokenTypes
	if len(legend) == 0 || legend[SemTokKeyword] != "keyword" {
		t.Fatalf("legend inconsistent with encoder contract: %v", legend)
	}
	tc.sendNotification(MethodInitialized, map[string]interface{}{})

	text := "func main() {\n    print(1)\n}"
	uri := "file:///workspace/sem.kark"
	tc.sendNotification(MethodTextDocumentDidOpen, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "karkain", Version: 1, Text: text},
	})

	resp = tc.sendRequest(2, MethodTextDocumentSemanticFull, SemanticTokensParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("semanticTokens/full failed: %+v", resp)
	}
	var result SemanticTokensResult
	raw, _ = json.Marshal(resp.Result)
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("cannot decode semantic result: %v", err)
	}
	got := decodeTokens(t, result.Data)
	want := decodeTokens(t, EncodeSemanticTokens(text))
	if len(got) != len(want) {
		t.Fatalf("request/data mismatch: %d vs %d tokens", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d = %+v, want %+v", i, got[i], want[i])
		}
	}

	// Unknown document: well-formed empty stream, never an error.
	resp = tc.sendRequest(3, MethodTextDocumentSemanticFull, SemanticTokensParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/nope.kark"},
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("unknown document should yield empty stream, not error: %+v", resp)
	}
}
