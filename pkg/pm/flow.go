package pm

import (
	"fmt"
	"os"
	"path/filepath"
)

// LockFromGraph produces a LockFile from a resolved dependency graph.
// Nodes are emitted in sorted (deterministic) order.
func LockFromGraph(graph *DepGraph) *LockFile {
	lf := &LockFile{}
	keys := graph.SortKeys()
	for _, name := range keys {
		node := graph.Nodes[name]
		lf.Packages = append(lf.Packages, LockEntry{
			Name:      node.Name,
			Version:   node.Version,
			Source:    node.Source,
			URL:       node.URL,
			Integrity: false,
		})
	}
	return lf
}

// LockPath returns the path to karkain.lock inside a project.
func LockPath(projectDir string) string {
	return filepath.Join(projectDir, LockFileName)
}

// ResolveAndLock re-resolves the dependency graph and writes karkain.lock.
// It is the deterministic entry point for `update`.
func ResolveAndLock(projectDir string) (*DepGraph, error) {
	resolver, err := NewResolver(projectDir)
	if err != nil {
		return nil, err
	}
	graph, err := resolver.Resolve()
	if err != nil {
		return nil, err
	}
	lf := LockFromGraph(graph)
	SetResolvedNow(lf)
	if err := WriteLockFile(LockPath(projectDir), lf); err != nil {
		return nil, fmt.Errorf("cannot write lockfile: %w", err)
	}
	return graph, nil
}

// ReadLocked reads the existing karkain.lock. Missing lockfile is not an
// error; it returns (nil, nil).
func ReadLocked(projectDir string) (*LockFile, error) {
	path := LockPath(projectDir)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	return ReadLockFile(path)
}

// registeredFetch fetches a single locked dependency into the cache,
// verifying integrity when a checksum is available. It is safe against
// partial writes (writes to a temp dir then atomically renames).
func registeredFetch(projectDir string, entry LockEntry) error {
	dep := Dependency{Name: entry.Name, Version: entry.Version, Source: entry.Source, URL: entry.URL}
	if err := FetchModule(projectDir, dep); err != nil {
		return fmt.Errorf("failed to fetch %s@%s: %w", entry.Name, entry.Version, err)
	}
	return nil
}

// FetchLocked fetches exactly the packages recorded in karkain.lock.
// It implements the `fetch` (not `update`) semantics: obtain the already
// resolved versions without re-resolving.
func FetchLocked(projectDir string) (*LockFile, error) {
	lf, err := ReadLocked(projectDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read lockfile: %w", err)
	}
	if lf == nil {
		return nil, fmt.Errorf("no %s found; run 'karkain update' first to resolve dependencies", LockFileName)
	}
	for _, entry := range lf.Packages {
		if err := registeredFetch(projectDir, entry); err != nil {
			return lf, err
		}
	}
	return lf, nil
}

// ListDepsDetails returns the resolved dependency details for display.
// It resolves the graph (without network) and reports direct + transitive.
type DepDetail struct {
	Name     string
	Version  string
	Source   string
	Direct   bool
	Resolved string // resolved version (from lock) when available
}

// ResolvedDetails builds a flat, sorted list of dependencies for `list`.
func ResolvedDetails(projectDir string) ([]DepDetail, error) {
	resolver, err := NewResolver(projectDir)
	if err != nil {
		return nil, err
	}
	graph := resolver.ResolveDirect()

	lf, _ := ReadLocked(projectDir)
	lockVers := make(map[string]string)
	if lf != nil {
		for _, p := range lf.Packages {
			lockVers[p.Name] = p.Version
		}
	}

	details := make([]DepDetail, 0, len(graph.Nodes))
	for _, name := range graph.SortKeys() {
		node := graph.Nodes[name]
		details = append(details, DepDetail{
			Name:     node.Name,
			Version:  node.Version,
			Source:   node.Source,
			Direct:   node.Direct,
			Resolved: lockVers[name],
		})
	}
	return details, nil
}

// TreeLines renders the dependency graph as deterministic text lines.
func TreeLines(projectDir string) ([]string, error) {
	resolver, err := NewResolver(projectDir)
	if err != nil {
		return nil, err
	}
	graph, err := resolver.Resolve()
	if err != nil {
		return nil, err
	}

	rootName := resolver.Root().Name
	if rootName == "" {
		rootName = filepath.Base(projectDir)
	}
	lines := []string{rootName}

	var render func(name string, prefix string)
	render = func(name string, prefix string) {
		children := graph.Children(name)
		for i, child := range children {
			connector := "├── "
			if i == len(children)-1 {
				connector = "└── "
			}
			lines = append(lines, fmt.Sprintf("%s%s%s %s", prefix, connector, child, versionOf(graph, child)))
			if len(graph.Children(child)) > 0 {
				childPrefix := prefix + "│   "
				if connector == "└── " {
					childPrefix = prefix + "    "
				}
				render(child, childPrefix)
			}
		}
	}

	roots := make([]string, 0)
	for _, name := range graph.SortKeys() {
		if graph.Nodes[name].Direct {
			roots = append(roots, name)
		}
	}
	for i, name := range roots {
		connector := "├── "
		if i == len(roots)-1 {
			connector = "└── "
		}
		lines = append(lines, fmt.Sprintf("%s%s %s", connector, name, versionOf(graph, name)))
		if len(graph.Children(name)) > 0 {
			childPrefix := "│   "
			if connector == "└── " {
				childPrefix = "    "
			}
			render(name, childPrefix)
		}
	}
	return lines, nil
}

func versionOf(graph *DepGraph, name string) string {
	node := graph.Nodes[name]
	if node == nil || node.Version == "" {
		return ""
	}
	return node.Version
}

// NewProject creates a new project directory distinct from in-place init.
// new: `karkain new <name>` creates <name>/ as a fresh project.
func NewProject(parentDir, name string) (*InitResult, error) {
	projectDir := filepath.Join(parentDir, name)
	if _, err := os.Stat(projectDir); err == nil {
		return nil, fmt.Errorf("directory %q already exists", projectDir)
	}
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create project directory: %w", err)
	}
	return InitProject(projectDir, name)
}
