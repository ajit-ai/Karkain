package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"karkain/pkg/diagnostics"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"karkain/pkg/source"
)

// funcMainPattern matches a top-level `func main(` so the preflight excludes
// non-root files that would be dropped from the assembled unit (a project may
// host several candidate main files; only the root one is the entry point).
var funcMainPattern = regexp.MustCompile(`(?m)^\s*func\s+main\s*\(`)

// projectSyntaxFiles returns the deterministic, ordered list of .kark files
// that make up the compilation unit for a CLI target: the project source files
// (project builds resolved via karkain.toml, including local dependencies)
// when available, otherwise the classic sibling join (same-directory .kark
// files minus the root, sorted, root appended last). Files that would be
// excluded from the assembled unit (non-root files hosting `func main`) are
// omitted so the preflight never reports errors for code the compiler drops.
func projectSyntaxFiles(targetFile string) []string {
	cleanPath := effectiveRootFile(targetFile)
	rootBase := filepath.Base(cleanPath)

	files, err := projectSourceFiles(cleanPath)
	if err != nil {
		files = siblingKarkFiles(cleanPath)
	}

	var out []string
	seen := map[string]bool{}
	for _, f := range files {
		if seen[f] {
			continue
		}
		seen[f] = true
		if f != cleanPath && filepath.Base(f) != rootBase {
			if data, rerr := os.ReadFile(f); rerr == nil && funcMainPattern.Match(data) {
				continue
			}
		}
		out = append(out, f)
	}
	return out
}

// siblingKarkFiles lists same-directory .kark files with the root file last,
// mirroring the legacy deterministic sibling-join order.
func siblingKarkFiles(rootFile string) []string {
	dir := filepath.Dir(rootFile)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var siblings []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".kark" {
			continue
		}
		if filepath.Clean(filepath.Join(dir, e.Name())) == filepath.Clean(rootFile) {
			continue
		}
		siblings = append(siblings, filepath.Join(dir, e.Name()))
	}
	sort.Strings(siblings)
	return append(siblings, rootFile)
}

// parseProjectSyntax parses every file of the compilation unit independently
// and collects all recoverable syntax diagnostics. Files are parsed one at a
// time so the parser's error recovery is scoped to a single file and the
// resulting spans are precise per file. Diagnostics carry their own excerpt so
// the renderer never needs a concatenated unit to draw the source line.
func parseProjectSyntax(files []string) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		src := string(data)
		l := lexer.New(src)
		p := parser.New(l)
		p.ParseProgram()
		if len(p.Errors) == 0 {
			continue
		}
		for i, parseErr := range p.Errors {
			line, _ := extractLineCol(parseErr)
			col := 1
			if i < len(p.ErrorCols) && p.ErrorCols[i] > 0 {
				col = p.ErrorCols[i]
			}
			d := diagnostics.ErrorDiagnostic(f, line, col, diagnostics.CodeSyntax, parseErr)
			d.Excerpt = source.Excerpt(src, line, 80)
			diags = append(diags, d)
		}
	}
	return diags
}

// projectSyntaxDiagnostics runs the multi-file syntax preflight for a CLI
// target and returns the combined diagnostic set. It returns an empty slice
// when every file of the unit parses cleanly, in which case normal
// assembly/analysis proceeds unchanged.
func projectSyntaxDiagnostics(targetFile string) []diagnostics.Diagnostic {
	files := projectSyntaxFiles(targetFile)
	return parseProjectSyntax(files)
}