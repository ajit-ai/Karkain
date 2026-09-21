package lsp

// Phase 136, Slice C gate — scoped go-to-definition.
//
// Same-document scoping is pinned in scope_test.go; these tests pin the
// cross-module path: qualified calls jump to the defining file's exact
// span, bare import names jump to the module file, unknown qualifiers yield
// null, and same-name collisions never leak across documents
// (deterministically — the old map-order search is gone).

import (
	"os"
	"path/filepath"
	"testing"
)

const defMathURI = "file:///workspace/math.kark"
const defMainURI = "file:///workspace/main.kark"

const defMathText = "public func twice(n int) {\n    return n * 2\n}\n"
const defMainText = "import math\n\nfunc main() {\n    print(math.twice(21))\n}\n"

func openDefDocs(t *testing.T, tc *testClient) {
	t.Helper()
	openDoc(t, tc, defMathURI, defMathText)
	openDoc(t, tc, defMainURI, defMainText)
}

func TestDefinition_CrossModule(t *testing.T) {
	tc := initClient(t)
	openDefDocs(t, tc)

	// `twice` inside `math.twice` (line 3, cols 15-19) jumps to math.kark.
	loc, ok := requestDefinition(t, tc, 2, defMainURI, 3, 17)
	if !ok {
		t.Fatal("expected a definition for math.twice")
	}
	if loc.URI != defMathURI {
		t.Fatalf("jump should land in math.kark, got %s", loc.URI)
	}
	if loc.Range.Start.Line != 0 || loc.Range.End.Character-loc.Range.Start.Character != 5 {
		t.Fatalf("jump should span the 5-char name on line 0, got %+v", loc.Range)
	}
	// The span must sit exactly on `twice` in `public func twice(n int) {`.
	line := "public func twice(n int) {"
	if loc.Range.Start.Character != 12 || loc.Range.End.Character != 17 {
		t.Fatalf("jump should span 0:12-17 in %q, got %+v", line, loc.Range)
	}
}

func TestDefinition_BareImport(t *testing.T) {
	tc := initClient(t)
	openDefDocs(t, tc)

	// The bare module name on the import line jumps to the module file.
	loc, ok := requestDefinition(t, tc, 2, defMainURI, 0, 8)
	if !ok {
		t.Fatal("expected a definition for the bare import name")
	}
	if loc.URI != defMathURI {
		t.Fatalf("bare import should jump to math.kark, got %s", loc.URI)
	}
}

func TestDefinition_UnknownQualifier(t *testing.T) {
	tc := initClient(t)
	openDefDocs(t, tc)
	uri := "file:///workspace/other.kark"
	openDoc(t, tc, uri, "func main() {\n    print(nosuchmod.foo())\n}\n")

	if _, ok := requestDefinition(t, tc, 2, uri, 1, 20); ok {
		t.Fatal("unknown qualifier should have no definition")
	}
	if _, ok := requestDefinition(t, tc, 2, uri, 1, 12); ok {
		t.Fatal("unknown qualified member should have no definition")
	}
}

func TestDefinition_NoCrossDocLeak(t *testing.T) {
	// Two open documents define the same name: each document's uses must
	// resolve locally, deterministically (25 iterations catch map-order
	// flakes by construction — there is no map iteration left).
	for i := 0; i < 25; i++ {
		tc := initClient(t)
		aURI := "file:///workspace/a.kark"
		bURI := "file:///workspace/b.kark"
		openDoc(t, tc, aURI, "func helper() {\n    return 1\n}\nfunc main() {\n    helper()\n}\n")
		openDoc(t, tc, bURI, "func helper() {\n    return 2\n}\nfunc main() {\n    helper()\n}\n")

		loc, ok := requestDefinition(t, tc, 2, aURI, 4, 5)
		if !ok || loc.URI != aURI || loc.Range.Start.Line != 0 {
			t.Fatalf("iter %d: a.kark use should stay in a.kark: %+v", i, loc)
		}
		loc, ok = requestDefinition(t, tc, 3, bURI, 4, 5)
		if !ok || loc.URI != bURI || loc.Range.Start.Line != 0 {
			t.Fatalf("iter %d: b.kark use should stay in b.kark: %+v", i, loc)
		}
	}
}

func TestDefinition_DiskSibling(t *testing.T) {
	// No math document open: the resolver falls back to the sibling file
	// on disk next to the requesting file.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "math.kark"), []byte(defMathText), 0644); err != nil {
		t.Fatalf("write math.kark: %v", err)
	}
	mainURI := pathToURI(filepath.Join(dir, "main.kark"))

	tc := initClient(t)
	openDoc(t, tc, mainURI, defMainText)

	loc, ok := requestDefinition(t, tc, 2, mainURI, 3, 17)
	if !ok {
		t.Fatal("expected a definition via the disk sibling")
	}
	wantURI := pathToURI(filepath.Join(dir, "math.kark"))
	if loc.URI != wantURI {
		t.Fatalf("jump should land in %s, got %s", wantURI, loc.URI)
	}
	if loc.Range.Start.Line != 0 {
		t.Fatalf("jump should land on line 0, got %+v", loc.Range)
	}
}
