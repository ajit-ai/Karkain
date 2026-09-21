package lsp

// Phase 136, Slice D gate — scoped completion.
//
// Pins the fixed behavior: prefix filtering applies to every source
// (the old code appended document symbols unfiltered), only in-scope
// names complete, member position completes members (never keywords),
// unknown qualifiers yield an empty list (never an error), and broken
// files still complete safely.

import (
	"encoding/json"
	"strings"
	"testing"
)

func requestCompletion(t *testing.T, tc *testClient, id int, uri string, line, char int) CompletionList {
	t.Helper()
	resp := tc.sendRequest(id, MethodTextDocumentCompletion, CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: char},
	})
	if resp == nil || resp.Error != nil {
		t.Fatalf("completion failed: %+v", resp)
	}
	var out CompletionList
	raw, _ := json.Marshal(resp.Result)
	json.Unmarshal(raw, &out)
	if out.Items == nil {
		t.Fatal("completion items should be a list, not null")
	}
	return out
}

func labelKinds(items []CompletionItem) map[string]int {
	m := map[string]int{}
	for _, it := range items {
		m[it.Label] = it.Kind
	}
	return m
}

const compURI = "file:///workspace/comp.kark"

const compText = `func helper() { }
type Point struct {
    x f64
    y f64
}
func main() {
    let local = 1
    hel
    loc
    Point.
}`

func TestCompletion_PrefixFiltersEverything(t *testing.T) {
	tc := initClient(t)
	openDoc(t, tc, compURI, compText)

	// "hel" must offer helper and nothing else (symbols were unfiltered).
	items := requestCompletion(t, tc, 2, compURI, 7, 7)
	for _, it := range items.Items {
		if !strings.HasPrefix(strings.ToLower(it.Label), "hel") {
			t.Fatalf("unfiltered completion %q for prefix 'hel'", it.Label)
		}
	}
	if _, ok := labelKinds(items.Items)["helper"]; !ok {
		t.Fatal("prefix 'hel' should offer helper")
	}
}

func TestCompletion_ScopedLocals(t *testing.T) {
	tc := initClient(t)
	openDoc(t, tc, compURI, compText)

	// "loc" offers the in-scope local with variable kind...
	items := requestCompletion(t, tc, 2, compURI, 8, 7)
	kinds := labelKinds(items.Items)
	if k, ok := kinds["local"]; !ok || k != CompletionKindVariable {
		t.Fatalf("'loc' should offer local as variable, got %+v", kinds)
	}
	// ...and nothing out of scope.
	for _, bad := range []string{"helper", "main", "Point", "x", "y"} {
		if _, ok := kinds[bad]; ok {
			t.Fatalf("out-of-prefix %q should not complete for 'loc'", bad)
		}
	}

	// A use-before-decl position sees no local: line 6 is the `let` line
	// itself, completing mid-name still offers the binding being typed only
	// when the cursor is past its declaration end... here cursor (6,10) is
	// inside `local` (cols 8-12): the name resolves to nothing yet the
	// prefix matches, so the binding must not leak from the future.
	items = requestCompletion(t, tc, 3, compURI, 6, 9)
	for _, it := range items.Items {
		if it.Label == "local" {
			t.Fatalf("declaration-in-progress should not complete its own future name")
		}
	}
}

func TestCompletion_Members(t *testing.T) {
	tc := initClient(t)
	openDoc(t, tc, compURI, compText)

	// After `Point.` only the struct fields complete — no keywords.
	items := requestCompletion(t, tc, 2, compURI, 9, 10)
	kinds := labelKinds(items.Items)
	if len(items.Items) != 2 {
		t.Fatalf("Point. should offer exactly x and y, got %+v", kinds)
	}
	if kinds["x"] != CompletionKindField || kinds["y"] != CompletionKindField {
		t.Fatalf("fields should have field kind, got %+v", kinds)
	}

	// Unknown qualifier: empty list, never an error.
	openDoc(t, tc, "file:///workspace/unk.kark", "func main() {\n    nosuch.\n}\n")
	items = requestCompletion(t, tc, 3, "file:///workspace/unk.kark", 1, 11)
	if len(items.Items) != 0 {
		t.Fatalf("unknown qualifier should complete nothing, got %+v", labelKinds(items.Items))
	}
}

func TestCompletion_ModuleMembers(t *testing.T) {
	tc := initClient(t)
	openDoc(t, tc, defMathURI, defMathText)
	openDoc(t, tc, defMainURI, defMainText)

	// After `math.` the module member completes (member of another file).
	items := requestCompletion(t, tc, 2, defMainURI, 3, 15)
	kinds := labelKinds(items.Items)
	if k, ok := kinds["twice"]; !ok || k != CompletionKindFunction {
		t.Fatalf("'math.' should offer twice as function, got %+v", kinds)
	}
	for _, it := range items.Items {
		if it.Label == "func" || it.Label == "let" {
			t.Fatalf("keyword %q should not complete in member position", it.Label)
		}
	}
}

func TestCompletion_BrokenFile(t *testing.T) {
	tc := initClient(t)
	uri := "file:///workspace/broken.kark"
	openDoc(t, tc, uri, "func broken( {\n    let x = \n")

	// Must not error; keywords still complete from the static set.
	items := requestCompletion(t, tc, 2, uri, 1, 11)
	if _, ok := labelKinds(items.Items)["func"]; !ok {
		t.Fatalf("broken file should still offer keywords, got %+v", labelKinds(items.Items))
	}
}
