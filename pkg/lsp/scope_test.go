package lsp

// Phase 136, Slice B gate — scope model, typed hover, scoped definition.
//
// Positions are asserted exactly (0-based lines, UTF-16 columns), not as
// substrings: the line-0 placeholder ranges of the old implementation fail
// these tests by construction.

import (
	"encoding/json"
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

func parseScopeFixture(t *testing.T, text string) (*parser.Program, []string) {
	t.Helper()
	prog := parser.New(lexer.New(text)).ParseProgram()
	return prog, strings.Split(text, "\n")
}

func TestScopeModel_Positions(t *testing.T) {
	text := "func twice(n int) {\n    let doubled = n * 2\n    return doubled\n}\nfunc main() {\n    print(twice(21))\n}\n"
	prog, _ := parseScopeFixture(t, text)
	m := BuildScopeModel(prog, text)

	at := func(line, char int) *Binding { return ResolveAt(m, text, "file:///t.kark", line, char) }

	// Call-site reference resolves to the declaration name span.
	if b := at(5, 12); b == nil || b.Kind != BindFunc || b.Line != 0 || b.Col != 5 || b.EndCol != 10 {
		t.Fatalf("twice() resolves to %+v, want func decl at 0:5-10", b)
	}
	// Local use resolves to the let binding with its inferred type.
	if b := at(2, 14); b == nil || b.Kind != BindVar || b.Line != 1 || b.Col != 8 || b.EndCol != 15 {
		t.Fatalf("doubled resolves to %+v, want var decl at 1:8-15", b)
	} else if !strings.Contains(b.Detail, "doubled") || !strings.Contains(b.Detail, "int") {
		t.Fatalf("var detail should name and type the binding, got %q", b.Detail)
	}
	// Parameter use resolves to the parameter with its annotation.
	if b := at(1, 18); b == nil || b.Kind != BindParam || !strings.Contains(b.Detail, "n: int") {
		t.Fatalf("param n resolves to %+v, want detail 'n: int'", b)
	}
}

func TestScopeModel_Shadowing(t *testing.T) {
	// NOTE: bare top-level `let` produces no AST statement (the parser only
	// keeps declarations there), so shadowing is exercised with a nested
	// block — which is also what pins brace-matched scope ends.
	text := "func main() {\n    let x = 1\n    if true {\n        let x = 2\n        print(x)\n    }\n    print(x)\n}\n"
	prog, _ := parseScopeFixture(t, text)
	m := BuildScopeModel(prog, text)
	at := func(line, char int) *Binding { return ResolveAt(m, text, "file:///t.kark", line, char) }

	// Inside the if-block the shadowing let wins (decl line 3).
	inner := at(4, 15)
	if inner == nil || inner.Line != 3 {
		t.Fatalf("inner x should resolve to line 3, got %+v", inner)
	}
	if inner.ScopeEnd != 5 {
		t.Fatalf("inner scope should end at the block close (line 5), got %d", inner.ScopeEnd)
	}
	// After the block the outer binding wins again (decl line 1).
	if b := at(6, 11); b == nil || b.Line != 1 {
		t.Fatalf("outer x should resolve to line 1, got %+v", b)
	}
	// Use before the only declaration is invisible.
	text2 := "func main() {\n    print(y)\n    let y = 3\n}\n"
	prog2, _ := parseScopeFixture(t, text2)
	m2 := BuildScopeModel(prog2, text2)
	if b := ResolveAt(m2, text2, "file:///t.kark", 1, 11); b != nil {
		t.Fatalf("use-before-decl should resolve to nothing, got %+v", b)
	}
}

func TestScopeModel_Members(t *testing.T) {
	text := "type Point struct {\n    x f64\n    y f64\n}\nenum Color {\n    Red,\n    Green\n}\n"
	prog, _ := parseScopeFixture(t, text)
	m := BuildScopeModel(prog, text)
	at := func(line, char int) *Binding { return ResolveAt(m, text, "file:///t.kark", line, char) }

	if b := at(0, 7); b == nil || b.Kind != BindStruct || b.Line != 0 {
		t.Fatalf("Point resolves to %+v, want struct decl", b)
	}
	// Types are usable as qualifiers from any line (member path skips scope).
	if b := ResolveAt(m, "xxx\nPoint.x", "file:///t.kark", 1, 2); b == nil || b.Kind != BindField || !strings.Contains(b.Detail, "x: f64") {
		t.Fatalf("Point.x resolves to %+v, want field detail 'x: f64'", b)
	}
	if b := ResolveAt(m, "xxx\nColor.Red", "file:///t.kark", 1, 2); b == nil || b.Kind != BindVariant || !strings.Contains(b.Detail, "Color.Red") {
		t.Fatalf("Color.Red resolves to %+v, want variant detail", b)
	}
	if b := ResolveAt(m, "xxx\nPoint.nope", "file:///t.kark", 1, 2); b != nil {
		t.Fatalf("unknown member should resolve to nothing, got %+v", b)
	}
	// Bare field/variant names do not leak into lexical scope.
	if b := at(1, 5); b != nil {
		t.Fatalf("bare field name should not resolve lexically, got %+v", b)
	}
}

func openDoc(t *testing.T, tc *testClient, uri, text string) {
	t.Helper()
	tc.sendNotification(MethodTextDocumentDidOpen, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "karkain", Version: 1, Text: text},
	})
}

func requestHover(t *testing.T, tc *testClient, id int, uri string, line, char int) HoverResult {
	t.Helper()
	resp := tc.sendRequest(id, MethodTextDocumentHover, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: char},
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("hover failed: %+v", resp)
	}
	var out HoverResult
	raw, _ := json.Marshal(resp.Result)
	json.Unmarshal(raw, &out)
	return out
}

func requestDefinition(t *testing.T, tc *testClient, id int, uri string, line, char int) (Location, bool) {
	t.Helper()
	resp := tc.sendRequest(id, MethodTextDocumentDefinition, DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: char},
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("definition failed: %+v", resp)
	}
	if resp.Result == nil {
		return Location{}, false
	}
	var out Location
	raw, _ := json.Marshal(resp.Result)
	json.Unmarshal(raw, &out)
	if out.URI == "" {
		return Location{}, false
	}
	return out, true
}

func initClient(t *testing.T) *testClient {
	t.Helper()
	tc := newTestClient()
	tc.sendRequest(1, MethodInitialize, InitializeParams{
		RootURI:      "file:///workspace",
		Capabilities: ClientCapabilities{TextDocument: &TextDocumentClientCapabilities{}},
	})
	tc.sendNotification(MethodInitialized, map[string]interface{}{})
	return tc
}

func TestHover_TypedBindings(t *testing.T) {
	tc := initClient(t)
	uri := "file:///workspace/hover.kark"
	text := "func twice(n int) {\n    let doubled = n * 2\n    return doubled\n}\n"
	openDoc(t, tc, uri, text)

	// Local with inferred type.
	h := requestHover(t, tc, 2, uri, 2, 14)
	if !strings.Contains(h.Contents.Value, "doubled") || !strings.Contains(h.Contents.Value, "int") {
		t.Fatalf("var hover should show name and inferred type, got %q", h.Contents.Value)
	}
	if h.Range.Start.Line != 1 || h.Range.Start.Character != 8 {
		t.Fatalf("var hover range should be the decl span 1:8, got %+v", h.Range)
	}

	// Parameter with annotation.
	h = requestHover(t, tc, 3, uri, 1, 18)
	if !strings.Contains(h.Contents.Value, "n: int") {
		t.Fatalf("param hover should show 'n: int', got %q", h.Contents.Value)
	}

	// Keyword docs still work.
	h = requestHover(t, tc, 4, uri, 0, 1)
	if !strings.Contains(h.Contents.Value, "func") {
		t.Fatalf("keyword hover should mention func, got %q", h.Contents.Value)
	}

	// Unknown identifier yields null (JSON null, not an error).
	uri2 := "file:///workspace/unknown.kark"
	openDoc(t, tc, uri2, "func main() {\n    print(nosuchthing)\n}\n")
	resp := tc.sendRequest(5, MethodTextDocumentHover, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri2},
		Position:     Position{Line: 1, Character: 12},
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("hover failed: %+v", resp)
	}
	if resp.Result != nil {
		t.Fatalf("unknown identifier should hover null, got %+v", resp.Result)
	}
}

func TestDefinition_ScopedRanges(t *testing.T) {
	tc := initClient(t)
	uri := "file:///workspace/def.kark"
	text := "func helper() {\n    println(\"helping\")\n}\n\nfunc main() {\n    let helper = 1\n    helper()\n}\n"
	openDoc(t, tc, uri, text)

	// The call resolves to the shadowing local (decl line 5), not the
	// top-level function of the same name.
	loc, ok := requestDefinition(t, tc, 2, uri, 6, 5)
	if !ok {
		t.Fatal("expected a definition for the shadowed call")
	}
	if loc.URI != uri || loc.Range.Start.Line != 5 || loc.Range.Start.Character != 8 {
		t.Fatalf("shadowed call should jump to 5:8, got %+v", loc)
	}

	// A use with no shadowing in scope jumps to the function name span.
	loc, ok = requestDefinition(t, tc, 3, uri, 0, 7)
	if !ok {
		t.Fatal("expected a definition at the declaration")
	}
	if loc.Range.Start.Line != 0 || loc.Range.Start.Character != 5 || loc.Range.End.Character != 11 {
		t.Fatalf("declaration jump should span 0:5-11, got %+v", loc.Range)
	}

	// Keywords have no definition.
	if _, ok := requestDefinition(t, tc, 4, uri, 0, 1); ok {
		t.Fatal("keyword should have no definition")
	}
}
