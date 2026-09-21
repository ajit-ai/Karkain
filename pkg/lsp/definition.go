package lsp

// Phase 136, Slice C — scoped go-to-definition.
//
// Same-document references resolve through the scope model (narrowest
// scope, declaration order). Cross-module references (`mod.name` with
// `import mod`) resolve to the defining file, searched deterministically:
// open documents first (sorted URI), then on-disk siblings of the current
// file, then declared project dependencies. The first file whose model
// defines the member wins; anything unresolvable yields nil (never an
// error, never a guess). The old implementation matched names across all
// open documents in map order (nondeterministic on collisions); that path
// is gone.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"karkain/pkg/pm"
)

// OpenDoc is one open document usable for cross-module resolution.
type OpenDoc struct {
	URI   string
	Text  string
	Model *ScopeModel
}

// DocContext carries everything ResolveDefinition needs without touching
// server state (handlers build it; tests build it by hand).
type DocContext struct {
	URI      string
	Text     string
	Model    *ScopeModel
	Imports  []string
	OpenDocs []OpenDoc // sorted by URI
}

// ModuleCandidate is one file that may define a module's members.
type ModuleCandidate struct {
	URI    string
	Text   string
	Model  *ScopeModel // nil until parsed
	parsed bool
}

// modelFor parses the candidate text on first use.
func (c *ModuleCandidate) modelFor() *ScopeModel {
	if c.Model != nil {
		return c.Model
	}
	if c.parsed {
		return nil
	}
	c.parsed = true
	prog := parser.New(lexer.New(c.Text)).ParseProgram()
	c.Model = BuildScopeModel(prog, c.Text)
	return c.Model
}

// uriToPath maps a file:// URI to a filesystem path ("", false when the URI
// is not a file URI).
func uriToPath(uri string) (string, bool) {
	if !strings.HasPrefix(uri, "file://") {
		return "", false
	}
	p := strings.TrimPrefix(uri, "file://")
	if len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:] // file:///C:/x -> C:/x
	}
	return filepath.FromSlash(p), true
}

// pathToURI maps an absolute path to a file:// URI.
func pathToURI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file://" + p
}

// dirKarkFiles lists the .kark files directly inside dir, sorted.
func dirKarkFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".kark" {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}

// moduleCandidates returns the files that may define module qual, in
// deterministic priority order: open documents, on-disk siblings,
// declared project dependencies.
func moduleCandidates(qual, curURI string, open []OpenDoc) []ModuleCandidate {
	var out []ModuleCandidate
	seen := map[string]bool{}
	add := func(uri, text string, model *ScopeModel) {
		if seen[uri] {
			return
		}
		seen[uri] = true
		out = append(out, ModuleCandidate{URI: uri, Text: text, Model: model})
	}

	// 1. Open documents whose file name matches <qual>.kark.
	for _, d := range open {
		if p, ok := uriToPath(d.URI); ok && filepath.Base(p) == qual+".kark" {
			add(d.URI, d.Text, d.Model)
		}
	}

	// 2. On-disk siblings of the current file.
	if curPath, ok := uriToPath(curURI); ok {
		dir := filepath.Dir(curPath)
		if fi, err := os.Stat(filepath.Join(dir, qual+".kark")); err == nil && !fi.IsDir() {
			p := filepath.Join(dir, qual+".kark")
			if data, err := os.ReadFile(p); err == nil {
				add(pathToURI(p), string(data), nil)
			}
		}
		if fi, err := os.Stat(filepath.Join(dir, qual)); err == nil && fi.IsDir() {
			for _, f := range dirKarkFiles(filepath.Join(dir, qual)) {
				if data, err := os.ReadFile(f); err == nil {
					add(pathToURI(f), string(data), nil)
				}
			}
		}
		// 3. Declared project dependencies with this exact name.
		if ws, werr := pm.FindProjectRoot(dir); werr == nil {
			for _, ds := range pm.DependencySources(ws) {
				if ds.Name != qual || !ds.Resolved() {
					continue
				}
				for _, f := range dirKarkFiles(ds.Dir) {
					if data, err := os.ReadFile(f); err == nil {
						add(pathToURI(f), string(data), nil)
					}
				}
				for _, f := range dirKarkFiles(filepath.Join(ds.Dir, "src")) {
					if data, err := os.ReadFile(f); err == nil {
						add(pathToURI(f), string(data), nil)
					}
				}
			}
		}
	}
	return out
}

// topBindings returns every module-level binding of a model, in source
// order. Locals of other functions never match: only globals are
// addressable across modules.
func topBindings(m *ScopeModel) []Binding {
	if m == nil {
		return nil
	}
	var out []Binding
	for _, b := range m.Bindings {
		if b.Global {
			out = append(out, b)
		}
	}
	return out
}

// ModuleMembers lists the exported names of module qual across every
// candidate file (deduplicated by name, first file wins). It powers member
// completion (`mod.`); definition jumps use topBinding for single hits.
func ModuleMembers(doc *DocContext, qual string) []Binding {
	var out []Binding
	seen := map[string]bool{}
	// Struct/enum members of the current document come first.
	if doc.Model != nil {
		if fields, ok := doc.Model.Structs[qual]; ok {
			for _, b := range fields {
				if !seen[b.Name] {
					seen[b.Name] = true
					out = append(out, b)
				}
			}
		}
		if variants, ok := doc.Model.Enums[qual]; ok {
			for _, b := range variants {
				if !seen[b.Name] {
					seen[b.Name] = true
					out = append(out, b)
				}
			}
		}
	}
	cands := moduleCandidates(qual, doc.URI, doc.OpenDocs)
	for i := range cands {
		for _, b := range topBindings(cands[i].modelFor()) {
			if !seen[b.Name] {
				seen[b.Name] = true
				out = append(out, b)
			}
		}
	}
	return out
}

// topBinding returns the module-level binding for name, preferring
// declarations (globals) in source order. Locals of other functions never
// match: only global bindings are addressable across modules.
func topBinding(m *ScopeModel, name string) *Binding {
	if m == nil {
		return nil
	}
	for i := range m.Bindings {
		if b := &m.Bindings[i]; b.Name == name && b.Global {
			return b
		}
	}
	return nil
}

// ResolveDefinition finds the jump target of the identifier at (line, char).
// Same-document lexical bindings win; dotted qualifiers fall back to member
// (struct/enum) and then module resolution; a bare imported name jumps to
// its module file. Nil means "no definition" (keywords, unknowns).
func ResolveDefinition(doc *DocContext, line, char int) *Location {
	lines := strings.Split(doc.Text, "\n")
	word := wordAtPos(lines, line, char)
	if word == "" {
		return nil
	}

	locOf := func(uri string, b *Binding) *Location {
		return &Location{
			URI: uri,
			Range: Range{
				Start: Position{Line: b.Line, Character: b.Col},
				End:   Position{Line: b.Line, Character: b.EndCol},
			},
		}
	}

	if dot := strings.LastIndex(word, "."); dot >= 0 {
		qual, member := word[:dot], word[dot+1:]
		// Struct/enum members in the current document.
		if b := ResolveAt(doc.Model, doc.Text, doc.URI, line, char); b != nil &&
			(b.Kind == BindField || b.Kind == BindVariant) {
			return locOf(doc.URI, b)
		}
		// Module member in another file.
		cands := moduleCandidates(qual, doc.URI, doc.OpenDocs)
		for i := range cands {
			if b := topBinding(cands[i].modelFor(), member); b != nil {
				return locOf(cands[i].URI, b)
			}
		}
		return nil
	}

	// Same-document lexical binding (narrowest scope, decl order).
	if b := ResolveAt(doc.Model, doc.Text, doc.URI, line, char); b != nil {
		return locOf(doc.URI, b)
	}

	// Bare imported module name jumps to its file.
	for _, imp := range doc.Imports {
		if imp != word {
			continue
		}
		if cands := moduleCandidates(word, doc.URI, doc.OpenDocs); len(cands) > 0 {
			return &Location{URI: cands[0].URI, Range: Range{}}
		}
	}
	return nil
}
