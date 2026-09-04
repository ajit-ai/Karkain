package pm

import (
	"fmt"
	"os"
	"path/filepath"
)

// LockFromGraph produces a LockFile from a resolved dependency graph.
// Nodes are emitted in sorted (deterministic) order. Git nodes carry their
// resolved immutable Rev; cached-package checksums are recorded when present.
func LockFromGraph(graph *DepGraph) *LockFile {
	lf := &LockFile{}
	keys := graph.SortKeys()
	for _, name := range keys {
		node := graph.Nodes[name]
		entry := LockEntry{
			Name:      node.Name,
			Version:   node.Version,
			Source:    node.Source,
			URL:       node.URL,
			Rev:       node.Rev,
			Integrity: false,
		}
		lf.Packages = append(lf.Packages, entry)
	}
	return lf
}

// LockPath returns the path to karkain.lock inside a project.
func LockPath(projectDir string) string {
	return filepath.Join(projectDir, LockFileName)
}

// ResolveAndLock re-resolves the dependency graph and writes karkain.lock.
// It is the deterministic entry point for `update`. Git dependencies are
// resolved to immutable commits and pinned in the lock; cached packages have
// integrity checksums recorded.
func ResolveAndLock(projectDir string) (*DepGraph, error) {
	resolver, err := NewResolver(projectDir)
	if err != nil {
		return nil, err
	}
	graph, err := resolver.Resolve()
	if err != nil {
		return nil, err
	}

	// Resolve git deps to immutable commits and pin them (P2.9).
	if err := attachGitRevs(graph); err != nil {
		return nil, fmt.Errorf("cannot resolve git revisions for lockfile: %w", err)
	}

	lf := LockFromGraph(graph)
	SetResolvedNow(lf)
	if err := WriteLockFile(LockPath(projectDir), lf); err != nil {
		return nil, fmt.Errorf("cannot write lockfile: %w", err)
	}
	return graph, nil
}

// attachGitRevs resolves each git-source node to its immutable commit and sets
// node.Rev so LockFromGraph records the pinned revision. Network resolution only
// happens for git deps, and only when not already cached with a matching rev.
func attachGitRevs(graph *DepGraph) error {
	keys := graph.SortKeys()
	for _, name := range keys {
		node := graph.Nodes[name]
		if node.Source != string(SourceGit) {
			continue
		}
		if node.Rev != "" {
			continue
		}
		res, cloneDir, err := ResolveGitRevision(node.URL, node.Version, "")
		if err != nil {
			return err
		}
		os.RemoveAll(cloneDir)
		node.Rev = res.SHA
	}
	return nil
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
	// Cache-first: if the package is already present and valid at the locked
	// identity, skip the network entirely (P2.8).
	if hit, _ := CachedValid(projectDir, dep, entry.Rev); hit {
		return nil
	}

	// For git deps pinned in the lock, fetch the exact pinned commit without
	// re-resolving (cache-first + reproducible).
	if entry.Source == string(SourceGit) {
		if entry.Rev == "" {
			// No pinned rev recorded; resolve normally.
			if err := FetchModule(projectDir, dep); err != nil {
				return fmt.Errorf("failed to fetch %s@%s: %w", entry.Name, entry.Version, err)
			}
			return nil
		}
		cacheFileName := cacheDirName(dep, entry.Rev)
		if err := fetchGitPinned(projectDir, entry, cacheFileName); err != nil {
			return fmt.Errorf("failed to fetch %s@%s: %w", entry.Name, entry.Version, err)
		}
		return nil
	}

	if err := FetchModule(projectDir, dep); err != nil {
		return fmt.Errorf("failed to fetch %s@%s: %w", entry.Name, entry.Version, err)
	}
	return nil
}

// fetchGitPinned checks out a locked git commit into the source-aware cache dir
// (atomic: temp dir then rename), recording a checksum and the .rev file.
func fetchGitPinned(projectDir string, entry LockEntry, cacheFileName string) error {
	cacheDir := filepath.Join(projectDir, CacheModules, cacheFileName)
	if err := os.MkdirAll(filepath.Join(projectDir, CacheModules), 0755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(filepath.Dir(cacheDir), ".tmp-"+entry.Name+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := cloneAtRev(entry.URL, entry.Rev, "", tmpDir); err != nil {
		return err
	}
	if err := WriteRevFile(tmpDir, entry.Rev); err != nil {
		return err
	}
	if err := writeChecksumAndPromote(projectDir, tmpDir, cacheDir); err != nil {
		return err
	}
	return nil
}

// CachedValid reports whether a dependency of the given source identity is
// already present and valid in the cache. For git deps the identity is the
// resolved rev; for registry deps it is the resolved version. It returns (ok,
// checksum-verified). This is the offline/cache-first gate.
func CachedValid(projectDir string, dep Dependency, rev string) (bool, error) {
	if dep.Source == "local" || dep.Source == "workspace" {
		// Local/workspace deps are not cached (canonical source path policy).
		return false, nil
	}
	dir := filepath.Join(projectDir, CacheModules, cacheDirName(dep, rev))
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return false, nil
	}
	ok, err := VerifyChecksumFile(dir)
	if err != nil {
		// No checksum file recorded -> treat as valid (present) but unverified.
		return true, nil
	}
	return ok, nil
}

// writeChecksumAndPromote validates a freshly fetched temp dir is non-empty,
// replaces any stale cache dir, atomically renames into place, and records a
// checksum file. Shared by FetchModule and the cache-first git path.
func writeChecksumAndPromote(projectDir, tmpDir, cacheDir string) error {
	if err := validateFetched(tmpDir); err != nil {
		return err
	}
	_ = os.RemoveAll(cacheDir)
	if err := os.Rename(tmpDir, cacheDir); err != nil {
		return fmt.Errorf("cannot move fetched package into cache: %w", err)
	}
	if err := WriteChecksumFile(cacheDir); err != nil {
		return fmt.Errorf("cannot write checksum for cached package: %w", err)
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
