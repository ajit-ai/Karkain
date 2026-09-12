package pm

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DepSource describes the on-disk source location of one resolved dependency
// (registry/git/local/workspace). It is the bridge between the package
// ecosystem and build/run/check: the compiler ecosystem assembles .kark
// sources from each dependency's canonical directory.
//
// Local/workspace dependencies resolve to their canonical source path (the
// directory containing karkain.toml), which is never cached by policy. Registry
// and git dependencies resolve to their fetched cache directory (after an
// explicit `fetch`/`update`), which is source-aware (name@version / name@rev).
type DepSource struct {
	Name    string
	Source  string // "registry", "git", "local", "workspace"
	Version string
	Rev     string // git rev from the lock file (git deps only), else ""
	Dir     string // canonical directory holding the dependency's .kark sources
	Cached  bool   // true when Dir is the fetched cache (registry/git)
	Err     error  // non-nil when the dependency cannot be located on disk
}

// Resolved returns true when the dependency has a usable source directory.
func (d DepSource) Resolved() bool { return d.Dir != "" && d.Err == nil }

// DependencySources returns deterministic source info for every non-dev
// dependency of the project at projectDir, in sorted-by-name order. It does
// not touch the network and never auto-fetches: registry/git dependencies only
// produce a directory when their package is already present in the local cache
// (via `fetch`/`update`). DevDependencies are excluded so they never leak into
// build/run/check. Local/workspace deps are always resolved (canonical path).
func DependencySources(projectDir string) []DepSource {
	manifestPath := filepath.Join(projectDir, ManifestFile)
	m, err := ParseManifest(manifestPath)
	if err != nil {
		return nil
	}

	lf, _ := ReadLocked(projectDir)
	lockRev := map[string]string{}
	lockVer := map[string]string{}
	if lf != nil {
		for _, p := range lf.Packages {
			lockRev[p.Name] = p.Rev
			lockVer[p.Name] = p.Version
		}
	}

	names := sortedDeps(m.Dependencies)
	out := make([]DepSource, 0, len(names))
	for _, name := range names {
		dep := m.Dependencies[name]
		src := DepSource{Name: name, Source: dep.Source, Version: dep.Version}
		switch dep.Source {
		case string(SourceWorkspace):
			// Workspace deps resolve against the parity workspace ROOT (the
			// nearest ancestor carrying karkain.workspace.json), matching
			// WorkspaceOrder/workspaceDepDirs. Resolving against the importing
			// member's own dir silently skipped sibling members, so workspace
			// builds referencing sibling functions produced undefined symbols.
			dir := dep.URL
			if dir == "" {
				dir = filepath.Join(projectDir, "deps", name)
			} else if !filepath.IsAbs(dir) {
				if wsRoot := findWorkspaceRoot(projectDir); wsRoot != "" {
					dir = filepath.Join(wsRoot, dir)
				} else {
					dir = filepath.Join(projectDir, dir)
				}
			}
			if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
				src.Err = &PkgError{Code: ErrLocal, Package: name,
					Message: "workspace dependency not found (missing directory)"}
			} else {
				src.Dir = filepath.Clean(dir)
			}
		case string(SourceLocal), "":
			// Canonical source path. Workspace/local deps may be declared
			// without an explicit source string; treat path deps as local.
			// The dependency directory itself is the source tree; a manifest is
			// not required for source assembly (leaf dirs of .kark files are
			// valid), so we require the directory to exist, not a karkain.toml.
			dir := dep.URL
			if dir == "" {
				dir = filepath.Join(projectDir, "deps", name)
			} else if !filepath.IsAbs(dir) {
				dir = filepath.Join(projectDir, dir)
			}
			if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
				src.Err = &PkgError{Code: ErrLocal, Package: name,
					Message: "local dependency not found (missing directory)"}
			} else {
				src.Dir = filepath.Clean(dir)
			}
		case string(SourceRegistry):
			ver := dep.Version
			if ver == "" {
				ver = lockVer[name]
			}
			src.Version = ver
			rev := ""
			cacheDir := filepath.Join(projectDir, CacheModules, cacheDirName(dep, rev))
			src.Dir = cacheDir
			if fi, err := os.Stat(cacheDir); err == nil && fi.IsDir() {
				src.Cached = true
			} else {
				src.Err = &PkgError{Code: ErrRegistry, Package: name,
					Message: "registry dependency not fetched (run 'karkain pkg fetch' or 'update')"}
			}
		case string(SourceGit):
			rev := lockRev[name]
			src.Rev = rev
			cacheDir := filepath.Join(projectDir, CacheModules, cacheDirName(dep, rev))
			src.Dir = cacheDir
			if fi, err := os.Stat(cacheDir); err == nil && fi.IsDir() {
				src.Cached = true
			} else {
				src.Err = &PkgError{Code: ErrGitNotFound, Package: name,
					Message: "git dependency not fetched (run 'karkain pkg fetch' or 'update')"}
			}
		default:
			src.Err = &PkgError{Code: ErrSource, Package: name,
				Message: "unknown dependency source: " + dep.Source}
		}
		out = append(out, src)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// findWorkspaceRoot returns the nearest ancestor of dir (inclusive) that holds
// a karkain.workspace.json, or "" when dir is not inside a workspace.
func findWorkspaceRoot(dir string) string {
	cur, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for {
		if fi, serr := os.Stat(filepath.Join(cur, WorkspaceFile)); serr == nil && !fi.IsDir() {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}

// dependencySourceDirs returns just the resolved directories, upstream-first
// (sorted by name for determinism), omitting unresolved entries silently. It is
// a convenience wrapper used by the CLI source assembler.
func dependencySourceDirs(projectDir string) []string {
	var dirs []string
	for _, s := range DependencySources(projectDir) {
		dir := strings.TrimSpace(s.Dir)
		if dir == "" {
			continue
		}
		dirs = append(dirs, s.Dir)
	}
	sort.Strings(dirs)
	return dirs
}
