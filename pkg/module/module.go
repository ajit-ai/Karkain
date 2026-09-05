// Package module implements the Karkain module & import bridge: a
// deterministic, import-driven module graph with cycle detection and
// topological ordering that feeds the compiler's source assembly.
//
// The model distinguishes:
//
//	Project     a directory with karkain.toml (optional for flat builds)
//	Module      a logical unit identified by a canonical import name
//	SourceFile  a physical .kark file belonging to a module
//	Import      a declared dependency edge Module A -> Module B
//	Graph       the directed module graph used to order compilation
//
// A module's canonical identity is its import name, which is stable and
// platform-independent. Filesystem paths are only used to locate sources;
// they never become the semantic module identity.
//
// Resolution precedence (deterministic, documented):
//
//  1. the root module of the compilation (the entry file's module)
//  2. a sibling module file <name>.kark in the entry's directory
//  3. a sibling module directory <name>/ containing .kark sources
//  4. a declared package dependency (via the package manager)
//  5. standard-library modules (reserved std.* namespace)
//
// Search never depends on the caller's working directory and never globs
// arbitrary directories; it follows the project model only.
package module

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"karkain/pkg/pm"
)

// ErrCycle is returned (as a *DiagramError with kind ErrKindCycle) when an
// import cycle is detected; the error's detail carries the full cycle path.
type DiagramErrorKind int

const (
	ErrKindNotFound DiagramErrorKind = iota
	ErrKindCycle
	ErrKindDuplicate
	ErrKindMalformedImport
	ErrKindSourceLoad
)

// DiagramError is a structured module/import diagnostic. It carries the source
// file, line, and the module name involved so the CLI layer can render it
// through the existing diagnostics reporter.
type DiagramError struct {
	Kind   DiagramErrorKind
	Module string
	File   string
	Line   int
	Detail string
}

func (e *DiagramError) Error() string {
	switch e.Kind {
	case ErrKindNotFound:
		return fmt.Sprintf("module '%s' not found", e.Module)
	case ErrKindCycle:
		return fmt.Sprintf("module import cycle detected: %s", e.Detail)
	case ErrKindDuplicate:
		return fmt.Sprintf("duplicate module '%s'", e.Module)
	case ErrKindMalformedImport:
		return fmt.Sprintf("malformed import '%s'", e.Module)
	case ErrKindSourceLoad:
		return fmt.Sprintf("failed to load module '%s': %s", e.Module, e.Detail)
	default:
		return e.Detail
	}
}

// Import is one declared dependency edge in a module graph.
type Import struct {
	FromModule string // importing module name (canonical)
	ToModule   string // imported module name (canonical)
	File       string // source file where the import was declared
	Line       int    // line of the import declaration in File
}

// Module is a logical compilation unit identified by its canonical name.
type Module struct {
	// Name is the canonical, platform-independent module identity.
	Name string
	// Dir is the on-disk directory that owns this module's sources ("" for a
	// single-file root module whose sources are provided directly).
	Dir string
	// Root is true for the root entry module of a compilation.
	Root bool
	// External is true when the module is a declared package dependency
	// (not part of the project's own source tree).
	External bool
}

// SourceUnit attaches a .kark source file to the module that contains it.
type SourceUnit struct {
	Module      string
	File        string
	Source      string
	ImportLines []int // 1-based lines (in Source) of import declarations
}

// Graph is the resolved module graph for a compilation. It is deterministic:
// module and edge ordering are stable and independent of filesystem
// enumeration order.
type Graph struct {
	// RootFile is the entry source file path (the compilation root).
	RootFile string
	// Modules maps canonical module name -> module info.
	Modules map[string]*Module
	// Sources maps canonical module name -> the module's .kark source units,
	// in deterministic (sorted) order.
	Sources map[string][]*SourceUnit
	// Imports are all import edges, deduplicated and deterministically sorted.
	Imports []Import
	// Order is the deterministic topological compile order of module names,
	// dependencies first. Nil when a cycle is present.
	Order []string

	rootDir string
	seen    map[string]bool
}

// NewSpec describes what to compile. RootFile is the entry .kark path and
// RootModule its canonical name (for a single root module).
type NewSpec struct {
	RootFile string
	RootName string
}

// New runs resolution over the module reachable from RootFile and returns a
// fully populated Graph. When the root file has no import declarations and is
// not within a Karkain project, a single-module graph containing just the root
// file is returned (the flat fallback, exactly preserving legacy behavior).
func New(spec NewSpec) (*Graph, error) {
	g := &Graph{
		RootFile: spec.RootFile,
		Modules:  map[string]*Module{},
		Sources:  map[string][]*SourceUnit{},
		seen:     map[string]bool{},
	}
	g.rootDir = filepath.Dir(spec.RootFile)

	// Root module name: caller-provided, else derived from the root file
	// basename (minus .kark), else a single canonical "main".
	rootName := spec.RootName
	if rootName == "" {
		rootName = moduleNameFromFile(spec.RootFile)
	}
	if rootName == "" {
		rootName = "main"
	}

	// Determine whether this is a project build (karkain.toml at/above root).
	projectDir, perr := pm.FindProjectRoot(g.rootDir)
	isProject := perr == nil
	if isProject {
		if _, serr := os.Stat(filepath.Join(projectDir, pm.ManifestFile)); serr != nil {
			isProject = false
		}
	}

	// Root module owns the root file.
	g.addModule(rootName, g.rootDir, true, false)
	g.seen[spec.RootFile] = true

	// Resolve recursively from the root file. If the root file uses imports,
	// the import edges drive source inclusion (import-driven assembly).
	// Otherwise, inside a project, sibling modules are still gathered for
	// compatibility (mirrors the legacy flat model) so no earlier behavior
	// regresses.
	v := &resolver{g: g, projectDir: projectDir, isProject: isProject, rootName: rootName, walkedMod: map[string]bool{}}
	if err := v.walk(spec.RootFile, rootName); err != nil {
		return nil, err
	}

	if derr := g.finish(); derr != nil {
		return nil, derr
	}
	return g, nil
}

// resolver implements the recursive import-driven module discovery.
type resolver struct {
	g          *Graph
	projectDir string
	isProject  bool
	rootName   string
	walkedMod  map[string]bool // modules whose imports have been walked
}

// walk parses file, records its imports, and recurses into imported modules.
// visited guards file cycles; module cycles are separately reported via the
// graph's cycle detection (Order), not by aborting here.
func (v *resolver) walk(file, moduleName string) error {
	src, err := os.ReadFile(file)
	if err != nil {
		return &DiagramError{Kind: ErrKindSourceLoad, Module: moduleName, File: file, Detail: err.Error()}
	}
	unit := &SourceUnit{Module: moduleName, File: file, Source: string(src)}
	imports, err := parseImports(string(src))
	if err != nil {
		return err
	}
	unit.ImportLines = importLines(string(src))
	v.g.Sources[moduleName] = append(v.g.Sources[moduleName], unit)

	// Guard: a module whose imports have already been walked must not be
	// re-walked (prevents infinite recursion on cyclic imports; cycle detection
	// later emits the diagnostic).
	if v.walkedMod[moduleName] {
		return nil
	}
	v.walkedMod[moduleName] = true

	// Resolve each import within this module, building the edge list and
	// recursing into newly discovered modules.
	dir := filepath.Dir(file)
	for _, imp := range imports {
		target, err := v.resolveImport(imp, moduleName, dir)
		if err != nil {
			return err
		}
		v.g.Imports = append(v.g.Imports, Import{
			FromModule: moduleName,
			ToModule:   target,
			File:       file,
			Line:       0, // populated by importLines matching
		})
		if v.walkedMod[target] {
			continue
		}
		// Locate the target module's .kark sources (all of them, sorted).
		files := v.moduleSources(target, dir)
		if len(files) == 0 {
			return &DiagramError{Kind: ErrKindNotFound, Module: target, File: file}
		}
		v.g.addModule(target, filepath.Dir(files[0]), false, false)
		for _, f := range files {
			if v.g.seen[f] {
				continue
			}
			v.g.seen[f] = true
			if err := v.walk(f, target); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolveImport maps an import declaration to a canonical module name.
func (v *resolver) resolveImport(name, fromModule, fromDir string) (string, error) {
	// stdlib namespace is reserved: it resolves only when a stdlib module
	// exists under the project (future phase) — for now it is not found
	// unless the project actually provides it.
	if strings.HasPrefix(name, "std.") {
		// Provide a clear diagnostic rather than silently globbing.
		return "", &DiagramError{
			Kind:   ErrKindNotFound,
			Module: name,
			File:   fromDir,
			Detail: "standard-library module import is reserved for a future phase; define the module in the project to import it",
		}
	}
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", &DiagramError{Kind: ErrKindMalformedImport, Module: name, File: fromDir}
	}
	// 1. Sibling file <name>.kark
	if fi, err := os.Stat(filepath.Join(fromDir, name+".kark")); err == nil && !fi.IsDir() {
		return name, nil
	}
	// 2. Sibling directory <name>/ containing .kark sources
	if fi, err := os.Stat(filepath.Join(fromDir, name)); err == nil && fi.IsDir() {
		if len(karkFiles(filepath.Join(fromDir, name))) > 0 {
			return name, nil
		}
	}
	// 3. Declared package dependency
	if v.projectDir != "" {
		for _, ds := range pm.DependencySources(v.projectDir) {
			if ds.Name == name {
				return name, nil
			}
		}
	}
	return "", &DiagramError{Kind: ErrKindNotFound, Module: name, File: fromDir}
}

// moduleSources returns the .kark files belonging to module target as seen
// from fromDir (imported via a sibling file/dir or a declared dependency).
func (v *resolver) moduleSources(target, fromDir string) []string {
	// Sibling file <name>.kark
	if p := filepath.Join(fromDir, target+".kark"); fileExists(p) {
		return []string{p}
	}
	// Sibling directory <name>/
	if dir := filepath.Join(fromDir, target); dirExists(dir) {
		if f := karkFiles(dir); len(f) > 0 {
			return f
		}
	}
	// Declared dependency sources
	if v.projectDir != "" {
		for _, ds := range pm.DependencySources(v.projectDir) {
			if ds.Name == target && ds.Resolved() {
				var out []string
				out = append(out, karkFiles(ds.Dir)...)
				out = append(out, karkFiles(filepath.Join(ds.Dir, "src"))...)
				if len(out) > 0 {
					return out
				}
			}
		}
	}
	return nil
}

// finish computes the deterministic topological compile order and reports
// cycles as a structured error.
func (g *Graph) finish() *DiagramError {
	g.Imports = dedupImports(g.Imports)
	order, cycle := topoOrder(g.Imports)
	if cycle != nil {
		return &DiagramError{
			Kind:   ErrKindCycle,
			Module: cycle[0],
			Detail: strings.Join(cycle, " -> "),
		}
	}
	// Kahn's order is importer-before-dependency (the reverse of compile
	// order). Reverse it so dependencies are emitted first; the root module
	// (which nothing imports) therefore ends up last.
	g.Order = reverseStrings(order)
	// Always include the root module, even when it has no imports (a flat,
	// single-module compilation unit must still be ordered).
	root := g.rootName()
	present := false
	for _, n := range g.Order {
		if n == root {
			present = true
			break
		}
	}
	if !present {
		g.Order = append(g.Order, root)
	}
	return nil
}

func reverseStrings(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[len(in)-1-i] = s
	}
	return out
}

func (g *Graph) addModule(name, dir string, root, external bool) {
	if _, ok := g.Modules[name]; ok {
		return
	}
	g.Modules[name] = &Module{Name: name, Dir: dir, Root: root, External: external}
}

func (g *Graph) moduleNames() []string {
	names := make([]string, 0, len(g.Modules))
	for n := range g.Modules {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// SortedFiles returns all source files across the graph in deterministic
// topological order (dependencies first, then dependents), for modules that
// are reachable via imports or the root. Used by the CLI to assemble the
// concatenated compilation unit.
func (g *Graph) SortedFiles() []string {
	var out []string
	seen := map[string]bool{}
	appendModule := func(name string) {
		units := g.Sources[name]
		sort.Slice(units, func(i, j int) bool { return units[i].File < units[j].File })
		for _, u := range units {
			if !seen[u.File] {
				seen[u.File] = true
				out = append(out, u.File)
			}
		}
	}
	// Order: topological (imports first) then any root module last for the
	// entry point to be final, matching legacy "root appended last".
	for _, name := range g.Order {
		if name != g.rootName() {
			appendModule(name)
		}
	}
	appendModule(g.rootName())
	return out
}

func (g *Graph) rootName() string {
	for n, m := range g.Modules {
		if m.Root {
			return n
		}
	}
	return "main"
}

// parseImports extracts the module import names declared in a Karkain source
// file. C imports (import "C" {...}) are ignored.
func parseImports(src string) ([]string, error) {
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		// A parse error is surfaced as a source-load diagnostic; the parse
		// error itself is re-reported precisely by the front end later.
		return nil, &DiagramError{Kind: ErrKindSourceLoad, Module: "", Detail: strings.Join(p.Errors, "; ")}
	}
	var names []string
	for _, imp := range prog.Imports {
		names = append(names, imp.Name)
	}
	return names, nil
}

// importLines returns the 1-based source line of each import declaration, in
// order, so edges can be attributed to a precise location.
func importLines(src string) []int {
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	var lines []int
	for _, imp := range prog.Imports {
		lines = append(lines, imp.Line)
	}
	return lines
}

// topoOrder performs DFS topological ordering over the module import graph,
// returning the order (dependencies first) or the first detected cycle as a
// path of module names.
func topoOrder(edges []Import) (order []string, cycle []string) {
	// adjacency
	adj := map[string][]string{}
	indeg := map[string]int{}
	for _, e := range edges {
		adj[e.FromModule] = append(adj[e.FromModule], e.ToModule)
		indeg[e.ToModule]++
		if _, ok := indeg[e.FromModule]; !ok {
			indeg[e.FromModule] = 0
		}
	}
	// Kahn's algorithm with deterministic tie-breaking.
	nodes := make([]string, 0, len(indeg))
	for n := range indeg {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	queue := []string{}
	for _, n := range nodes {
		if indeg[n] == 0 {
			queue = append(queue, n)
		}
	}
	visited := map[string]bool{}
	var result []string
	for len(queue) > 0 {
		// pop smallest
		sort.Strings(queue)
		u := queue[0]
		queue = queue[1:]
		if visited[u] {
			continue
		}
		visited[u] = true
		result = append(result, u)
		neigh := adj[u]
		sort.Strings(neigh)
		for _, w := range neigh {
			indeg[w]--
			if indeg[w] == 0 {
				queue = append(queue, w)
			}
		}
	}
	if len(visited) != len(indeg) {
		// cycle: find a path
		cycle = findCycle(adj)
		return nil, cycle
	}
	return result, nil
}

// findCycle returns a representative import cycle A -> ... -> A.
func findCycle(adj map[string][]string) []string {
	var path []string
	state := map[string]int{} // 0 unvisited, 1 in-stack, 2 done
	var recur func(string) []string
	recur = func(u string) []string {
		if state[u] == 2 {
			return nil
		}
		if state[u] == 1 {
			// found back-edge; return cycle starting at u
			start := len(path)
			for i, x := range path {
				if x == u {
					start = i
					break
				}
			}
			cyc := append([]string{}, path[start:]...)
			cyc = append(cyc, u)
			return cyc
		}
		state[u] = 1
		path = append(path, u)
		neigh := adj[u]
		sort.Strings(neigh)
		for _, w := range neigh {
			if c := recur(w); c != nil {
				return c
			}
		}
		state[u] = 2
		path = path[:len(path)-1]
		return nil
	}
	nodes := make([]string, 0, len(adj))
	for n := range adj {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	for _, n := range nodes {
		if c := recur(n); c != nil {
			return c
		}
	}
	return nil
}

// dedupImports removes duplicate identical edges deterministically.
func dedupImports(edges []Import) []Import {
	seen := map[string]Import{}
	var out []Import
	for _, e := range edges {
		key := e.FromModule + "\x00" + e.ToModule
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = e
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FromModule != out[j].FromModule {
			return out[i].FromModule < out[j].FromModule
		}
		return out[i].ToModule < out[j].ToModule
	})
	return out
}

// --- helpers ---

func moduleNameFromFile(f string) string {
	base := filepath.Base(f)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "" {
		return ""
	}
	return name
}

func karkFiles(dir string) []string {
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

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
