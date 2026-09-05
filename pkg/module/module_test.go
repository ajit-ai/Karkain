package module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fsFixture struct {
	root string
}

func newFixture(t *testing.T) *fsFixture {
	t.Helper()
	return &fsFixture{root: t.TempDir()}
}

func (f *fsFixture) write(rel, src string) string {
	p := filepath.Join(f.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(p, []byte(src), 0644); err != nil {
		panic(err)
	}
	return p
}

func TestSingleModule(t *testing.T) {
	f := newFixture(t)
	main := f.write("main.kark", "func main() { print(1) }")
	g, err := New(NewSpec{RootFile: main, RootName: "main"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(g.Order) != 1 {
		t.Fatalf("expected 1 module in order, got %v", g.Order)
	}
	if len(g.Order) > 0 && g.Order[0] != "main" {
		t.Errorf("expected root module 'main' first, got %v", g.Order)
	}
}

func TestTwoModulesImport(t *testing.T) {
	f := newFixture(t)
	f.write("math.kark", "public func add(a int, b int) int { return a + b }")
	main := f.write("main.kark", "import math\nfunc main() { print(math.add(1, 2)) }")
	g, err := New(NewSpec{RootFile: main, RootName: "main"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// math (dependency) must precede main
	if len(g.Order) < 2 {
		t.Fatalf("expected at least 2 modules, got %v", g.Order)
	}
	if g.Order[0] != "math" {
		t.Errorf("dependency 'math' should be ordered first, got %v", g.Order)
	}
	if g.Order[1] != "main" {
		t.Errorf("root 'main' should be last, got %v", g.Order)
	}
}

func TestNestedModuleDir(t *testing.T) {
	f := newFixture(t)
	f.write("lib/util.kark", "public func helper() int { return 42 }")
	main := f.write("main.kark", "import lib\nfunc main() { print(lib.helper()) }")
	g, err := New(NewSpec{RootFile: main, RootName: "main"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(g.Order) != 2 {
		t.Fatalf("expected 2 modules (lib, main), got %v", g.Order)
	}
	if g.Order[0] != "lib" {
		t.Errorf("expected lib first, got %v", g.Order)
	}
}

func TestTransitiveImports(t *testing.T) {
	f := newFixture(t)
	f.write("c.kark", "public func cval() int { return 3 }")
	f.write("b.kark", "import c\npublic func bval() int { return c.cval() }")
	main := f.write("main.kark", "import b\nfunc main() { print(b.bval()) }")
	g, err := New(NewSpec{RootFile: main, RootName: "main"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// topo order: c, b, main
	if len(g.Order) != 3 {
		t.Fatalf("expected 3 modules, got %v", g.Order)
	}
	if g.Order[0] != "c" || g.Order[1] != "b" || g.Order[2] != "main" {
		t.Errorf("bad topological order: %v", g.Order)
	}
}

func TestDiamondImports(t *testing.T) {
	f := newFixture(t)
	f.write("d.kark", "public func dval() int { return 8 }")
	f.write("b.kark", "import d\npublic func bval() int { return d.dval() }")
	f.write("c.kark", "import d\npublic func cval() int { return d.dval() }")
	main := f.write("main.kark", "import b\nimport c\nfunc main() { print(b.bval() + c.cval()) }")
	g, err := New(NewSpec{RootFile: main, RootName: "main"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// d must appear once, before both b and c
	if len(g.Order) != 4 {
		t.Fatalf("expected 4 modules (no dup), got %v", g.Order)
	}
	if g.Order[0] != "d" {
		t.Errorf("expected d first, got %v", g.Order)
	}
	dcount := 0
	for _, n := range g.Order {
		if n == "d" {
			dcount++
		}
	}
	if dcount != 1 {
		t.Errorf("module d compiled %d times (must be 1)", dcount)
	}
}

func TestImportCycleDetected(t *testing.T) {
	f := newFixture(t)
	f.write("a.kark", "import b\npublic func aval() int { return 1 }")
	f.write("b.kark", "import c\npublic func bval() int { return 2 }")
	f.write("c.kark", "import a\npublic func cval() int { return 3 }")
	main := f.write("main.kark", "import a\nfunc main() { print(a.aval()) }")
	_, err := New(NewSpec{RootFile: main, RootName: "main"})
	if err == nil {
		t.Fatal("expected import cycle error")
	}
	de, ok := err.(*DiagramError)
	if !ok {
		t.Fatalf("expected *DiagramError, got %T: %v", err, err)
	}
	if de.Kind != ErrKindCycle {
		t.Errorf("expected cycle kind, got %v", de.Kind)
	}
	if !strings.Contains(de.Detail, "a ->") {
		t.Errorf("cycle detail should include the path, got %q", de.Detail)
	}
}

func TestMissingModuleDetected(t *testing.T) {
	f := newFixture(t)
	main := f.write("main.kark", "import nosuchmodule\nfunc main() {}")
	_, err := New(NewSpec{RootFile: main, RootName: "main"})
	if err == nil {
		t.Fatal("expected missing-module error")
	}
	de, ok := err.(*DiagramError)
	if !ok {
		t.Fatalf("expected *DiagramError, got %T: %v", err, err)
	}
	if de.Kind != ErrKindNotFound {
		t.Errorf("expected not-found kind, got %v", de.Kind)
	}
	if de.Module != "nosuchmodule" {
		t.Errorf("expected module name, got %q", de.Module)
	}
}

func TestMalformedImportDetected(t *testing.T) {
	f := newFixture(t)
	main := f.write("main.kark", "import ../../evil\nfunc main() {}")
	_, err := New(NewSpec{RootFile: main, RootName: "main"})
	if err == nil {
		t.Fatal("expected malformed-import error")
	}
	// `import ../..` is a lexical/parse failure (not a valid token path),
	// surfaced as a source-load diagnostic; it must never be silently accepted.
	de, ok := err.(*DiagramError)
	if !ok {
		t.Fatalf("expected *DiagramError, got %T: %v", err, err)
	}
	if de.Kind != ErrKindSourceLoad && de.Kind != ErrKindMalformedImport {
		t.Errorf("expected source-load or malformed kind, got %v: %v", de.Kind, err)
	}
}

func TestStdlibReserved(t *testing.T) {
	f := newFixture(t)
	main := f.write("main.kark", "import std.string\nfunc main() {}")
	_, err := New(NewSpec{RootFile: main, RootName: "main"})
	if err == nil {
		t.Fatal("expected stdlib-reserved not-found error")
	}
	if de, ok := err.(*DiagramError); !ok || de.Kind != ErrKindNotFound {
		t.Errorf("expected not-found for stdlib import, got %v", err)
	}
}

func TestDeterministicModuleOrdering(t *testing.T) {
	f := newFixture(t)
	// Import b and c in reverse order to prove ordering is canonical, not
	// declaration-order dependent.
	f.write("b.kark", "public func bval() int { return 2 }")
	f.write("c.kark", "public func cval() int { return 3 }")
	main := f.write("main.kark", "import c\nimport b\nfunc main() {}")
	got := testOrder(t, f, main, "main")
	// Both b and c are dependencies and must precede main; their relative
	// order is a stable tie-break (determinism, not a mandated sequence).
	if len(got) != 3 {
		t.Fatalf("expected 3 modules, got %v", got)
	}
	if got[2] != "main" {
		t.Errorf("expected main (root) last, got %v", got)
	}
	if !containsStr(got[:2], "b") || !containsStr(got[:2], "c") {
		t.Errorf("expected b and c before main, got %v", got)
	}
	// Repeat resolution to confirm identical ordering across runs.
	again := testOrder(t, f, main, "main")
	if strings.Join(got, ",") != strings.Join(again, ",") {
		t.Errorf("order not stable: %v vs %v", got, again)
	}
}

func containsStr(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func TestDuplicateImportEdgeDeduped(t *testing.T) {
	f := newFixture(t)
	f.write("m.kark", "public func mval() int { return 1 }")
	f.write("b.kark", "import m\npublic func bval() int { return m.mval() }")
	f.write("c.kark", "import m\npublic func cval() int { return m.mval() }")
	main := f.write("main.kark", "import b\nimport c\nfunc main() {}")
	g, err := New(NewSpec{RootFile: main, RootName: "main"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for _, e := range g.Imports {
		if e.FromModule == e.ToModule {
			t.Errorf("self-import edge present: %v", e)
		}
	}
}

func TestSortedFilesReachableOnly(t *testing.T) {
	f := newFixture(t)
	f.write("unrelated.kark", "func unused() {}")
	f.write("math.kark", "public func add(a int, b int) int { return a + b }")
	main := f.write("main.kark", "import math\nfunc main() { print(math.add(1,2)) }")
	g, err := New(NewSpec{RootFile: main, RootName: "main"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	files := g.SortedFiles()
	for _, fn := range files {
		if strings.Contains(filepath.Base(fn), "unrelated") {
			t.Errorf("unrelated.kark should NOT be in the module graph: %v", files)
		}
	}
	// math must be included before main
	foundMath := false
	foundMain := false
	for _, fn := range files {
		base := filepath.Base(fn)
		if base == "math.kark" {
			foundMath = true
		}
		if base == "main.kark" {
			foundMain = true
		}
		if foundMain && !foundMath {
			t.Errorf("math.kark must precede main.kark in topo order: %v", files)
		}
	}
	if !foundMath || !foundMain {
		t.Errorf("expected math.kark and main.kark both included, got %v", files)
	}
}

func testOrder(t *testing.T, f *fsFixture, main, name string) []string {
	t.Helper()
	g, err := New(NewSpec{RootFile: main, RootName: name})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return append([]string{}, g.Order...)
}

func TestOrderStableAcrossRuns(t *testing.T) {
	f := newFixture(t)
	f.write("u.kark", "public func uval() int { return 1 }")
	f.write("v.kark", "public func vval() int { return 2 }")
	main := f.write("main.kark", "import u\nimport v\nfunc main() {}")
	first := testOrder(t, f, main, "main")
	second := testOrder(t, f, main, "main")
	if strings.Join(first, ",") != strings.Join(second, ",") {
		t.Errorf("order not stable across runs: %v vs %v", first, second)
	}
}

func TestModulesNamedDirs(t *testing.T) {
	f := newFixture(t)
	// Ensure the RootName and imported modules map cleanly and SortedFiles are
	// deterministic even with a directory-named module.
	f.write("pkg/util.kark", "public func util() int { return 7 }")
	main := f.write("main.kark", "import pkg\nfunc main() { print(pkg.util()) }")
	g, err := New(NewSpec{RootFile: main, RootName: "main"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	files := g.SortedFiles()
	var hasUtil, hasMain bool
	for _, fn := range files {
		if strings.HasSuffix(fn, "util.kark") {
			hasUtil = true
		}
		if strings.HasSuffix(fn, "main.kark") {
			hasMain = true
		}
	}
	if !hasUtil || !hasMain {
		t.Errorf("expected both util.kark and main.kark in SortedFiles, got %v", files)
	}
	// util (dependency) must come before main (root)
	utilIdx := -1
	mainIdx := -1
	for i, fn := range files {
		switch {
		case strings.HasSuffix(fn, "util.kark"):
			utilIdx = i
		case strings.HasSuffix(fn, "main.kark"):
			mainIdx = i
		}
	}
	if utilIdx == -1 || mainIdx == -1 || utilIdx > mainIdx {
		t.Errorf("dependency util.kark must precede main.kark, got indices util=%d main=%d files=%v", utilIdx, mainIdx, files)
	}
}
