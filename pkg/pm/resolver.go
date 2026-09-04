package pm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GraphNode represents one resolved package in the dependency graph.
type GraphNode struct {
	Name     string
	Version  string
	Source   string // "registry", "git", "local", "workspace"
	URL      string
	Rev      string // resolved immutable git commit (Source == "git" only)
	Direct   bool   // true if a direct dependency of the root project
	Children []string
}

// DepGraph is a deterministic resolved dependency graph rooted at a project.
type DepGraph struct {
	Root  string
	Nodes map[string]*GraphNode
}

// SortKeys returns the node names in sorted order for deterministic output.
func (g *DepGraph) SortKeys() []string {
	names := make([]string, 0, len(g.Nodes))
	for n := range g.Nodes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Children returns the sorted child names of a node.
func (g *DepGraph) Children(name string) []string {
	node := g.Nodes[name]
	if node == nil {
		return nil
	}
	out := append([]string(nil), node.Children...)
	sort.Strings(out)
	return out
}

// ResolutionError describes why a dependency could not be resolved.
type ResolutionError struct {
	Pkg     string
	Dep     string
	Message string
}

func (e *ResolutionError) Error() string {
	return fmt.Sprintf("dependency %q required by %q: %s", e.Dep, e.Pkg, e.Message)
}

// Resolver builds a deterministic dependency graph from a project manifest.
//
// Only local/path dependencies are recursed into offline (their transitive
// dependencies are read from each package's karkain.toml). Registry and git
// dependencies are recorded at their declared version but not expanded, since
// that requires network access that this phase does not fake.
type Resolver struct {
	rootDir string
	root    *Manifest
	graph   *DepGraph
	errors  []error
}

// NewResolver creates a resolver rooted at a project directory.
func NewResolver(projectDir string) (*Resolver, error) {
	m, err := ParseManifest(filepath.Join(projectDir, ManifestFile))
	if err != nil {
		return nil, fmt.Errorf("cannot read manifest: %w", err)
	}
	return &Resolver{
		rootDir: projectDir,
		root:    m,
		graph:   &DepGraph{Root: projectDir, Nodes: make(map[string]*GraphNode)},
	}, nil
}

// Root returns the root manifest.
func (r *Resolver) Root() *Manifest { return r.root }

// Graph returns the current (possibly partial) graph.
func (r *Resolver) Graph() *DepGraph { return r.graph }

// Errors returns any collected resolution errors.
func (r *Resolver) Errors() []error { return r.errors }

// Resolve expands all direct dependencies into a graph. Direct-only local
// expansion is deterministic. If any hard conflict is found, it is returned
// as an error.
func (r *Resolver) Resolve() (*DepGraph, error) {
	for _, name := range sortedDeps(r.root.Dependencies) {
		dep := r.root.Dependencies[name]
		r.add(name, dep, r.root.Name, true)
	}
	if len(r.errors) > 0 {
		return r.graph, fmt.Errorf("dependency resolution failed:\n  %s", joinErrors(r.errors))
	}
	return r.graph, nil
}

// ResolveDirect is a fast path used by commands that only need the direct
// dependency set and do not want network-requiring expansion.
func (r *Resolver) ResolveDirect() *DepGraph {
	for _, name := range sortedDeps(r.root.Dependencies) {
		dep := r.root.Dependencies[name]
		node := &GraphNode{Name: name, Version: dep.Version, Source: dep.Source, URL: dep.URL, Direct: true}
		if prev, ok := r.graph.Nodes[name]; ok {
			if prev.Version != dep.Version {
				r.errors = append(r.errors, &ResolutionError{Pkg: r.root.Name, Dep: name,
					Message: fmt.Sprintf("conflicting versions %q vs %q", prev.Version, dep.Version)})
			}
			prev.Direct = true
			continue
		}
		r.graph.Nodes[name] = node
	}
	return r.graph
}

// add inserts a node (direct or transitive) and, for local/path deps, expands
// its transitive children.
func (r *Resolver) add(name string, dep Dependency, requiredBy string, direct bool) {
	if existing, ok := r.graph.Nodes[name]; ok {
		if existing.Version != dep.Version {
			// Not necessarily a hard error for non-local deps (multiple
			// registries could resolve independently), but a conflict for
			// distinct declared versions. Surface as an error for now.
			r.errors = append(r.errors, &ResolutionError{Pkg: requiredBy, Dep: name,
				Message: fmt.Sprintf("conflicting versions %q vs %q", existing.Version, dep.Version)})
		}
		if direct {
			existing.Direct = true
		}
		return
	}

	node := &GraphNode{
		Name:    name,
		Version: dep.Version,
		Source:  dep.Source,
		URL:     dep.URL,
		Direct:  direct,
	}
	r.graph.Nodes[name] = node

	if dep.Source == "local" && dep.URL != "" {
		r.expandLocal(name, dep, requiredBy)
	}
}

// expandLocal reads a local dependency's karkain.toml and adds its children.
// Recursion terminates because add() returns early for already-present nodes,
// which also guards against local dependency cycles (a -> b -> a).
func (r *Resolver) expandLocal(name string, dep Dependency, requiredBy string) {
	depDir := dep.URL
	if !filepath.IsAbs(depDir) {
		depDir = filepath.Join(r.rootDir, depDir)
	}
	depDir = filepath.Clean(depDir)

	subManifest := filepath.Join(depDir, ManifestFile)
	if _, err := os.Stat(subManifest); err != nil {
		return // leaf local dep with no manifest of its own
	}

	sub, err := ParseManifest(subManifest)
	if err != nil {
		r.errors = append(r.errors, &ResolutionError{Pkg: name, Dep: name,
			Message: fmt.Sprintf("cannot read dependency manifest: %v", err)})
		return
	}

	node := r.graph.Nodes[name]
	for _, childName := range sortedDeps(sub.Dependencies) {
		child := sub.Dependencies[childName]
		if node != nil {
			node.Children = appendUnique(node.Children, childName)
		}
		r.add(childName, child, name, false)
	}
}

func sortedDeps(deps map[string]Dependency) []string {
	names := make([]string, 0, len(deps))
	for n := range deps {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func joinErrors(errs []error) string {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, e.Error())
	}
	return strings.Join(parts, "\n  ")
}
