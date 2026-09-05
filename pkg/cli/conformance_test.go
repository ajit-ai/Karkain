package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// conformanceDir locates the top-level Karkain conformance corpus relative to
// the package working directory (go test runs with cwd = pkg/cli).
func conformanceDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := filepath.Join(wd, "..", "..", "conformance")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("conformance corpus not found at %s (placeholder .kark files must be real)", dir)
	}
	return dir
}

// conformanceCounts parses every *_test.kark in the corpus and returns the
// number of files and the number of test_ declarations they contain.
func conformanceCounts(t *testing.T, dir string) (files, tests int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read conformance dir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, "_test.kark") {
			continue
		}
		files++
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		p := parser.New(lexer.New(string(src)))
		prog := p.ParseProgram()
		if len(p.Errors) > 0 {
			t.Fatalf("conformance file %s does not parse: %s", name, strings.Join(p.Errors, "; "))
		}
		for _, stmt := range prog.Statements {
			if fn, ok := stmt.(*parser.FuncDecl); ok && strings.HasPrefix(fn.Name, "test_") {
				tests++
			}
		}
	}
	return files, tests
}

// TestConformanceCorpus_RunsClean verifies the Karkain-native conformance
// corpus passes end to end: every *_test.kark runs green through the real
// front end + SSA IR + C runtime with zero failures.
func TestConformanceCorpus_RunsClean(t *testing.T) {
	hasGCC(t)
	dir := conformanceDir(t)

	res := TestCommand(dir, codegen.NewConfig(), false)

	if res.ExitCode != ExitSuccess {
		t.Fatalf("conformance corpus failed: exit=%d msg=%q", res.ExitCode, res.Message)
	}
	if !strings.Contains(res.Message, "0 failed") {
		t.Fatalf("unexpected summary %q", res.Message)
	}
}

// TestConformanceCorpus_DeclarationsFound guards against silent corpus erosion:
// a parse error in a corpus file makes the runner skip it (non-verbose), so a
// broken edit would otherwise go unnoticed. Every file must parse and declare
// at least one test, and the total must not drop below the corpus floor.
func TestConformanceCorpus_DeclarationsFound(t *testing.T) {
	dir := conformanceDir(t)
	files, tests := conformanceCounts(t, dir)

	if files < 5 {
		t.Fatalf("conformance corpus eroded: only %d *_test.kark files", files)
	}
	if tests < 20 {
		t.Fatalf("conformance corpus eroded: only %d test_ declarations", tests)
	}
}