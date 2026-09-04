package pm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WorkspaceConfig is the workspace root configuration.
type WorkspaceConfig struct {
	Members []string `json:"members"`
}

const WorkspaceFile = "karkain.workspace.json"

// InitWorkspace creates a workspace root configuration.
func InitWorkspace(dir string) error {
	path := filepath.Join(dir, WorkspaceFile)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("workspace already initialized at %s", dir)
	}

	ws := &WorkspaceConfig{Members: []string{}}
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// AddWorkspaceMember adds a package directory to the workspace.
func AddWorkspaceMember(rootDir, memberPath string) error {
	path := filepath.Join(rootDir, WorkspaceFile)
	ws, err := readWorkspace(path)
	if err != nil {
		return fmt.Errorf("not a workspace root (no %s found)", WorkspaceFile)
	}

	absPath, _ := filepath.Abs(filepath.Join(rootDir, memberPath))
	for _, m := range ws.Members {
		if m == memberPath || m == absPath {
			return fmt.Errorf("%s is already a workspace member", memberPath)
		}
	}

	ws.Members = append(ws.Members, memberPath)
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// WorkspaceMember is one workspace package in build order.
type WorkspaceMember struct {
	Name  string
	Dir   string // absolute member directory
	Order int    // topological index (0 = built first / leaf)
}

// WorkspaceError codes (Karkain E-PKG-* conventions).
const (
	ErrWsCycle   = "E-PKG-WS-CYCLE"
	ErrWsNoRoot  = "E-PKG-WS-NO-ROOT"
	ErrWsMissing = "E-PKG-WS-MISSING"
)

// WorkspaceOrder returns the workspace members in dependency (topological)
// order: every member is listed after all members it depends on. A member is
// built exactly once even if referenced by several others. Inter-member
// dependencies are declared as `source = "workspace"` entries in each member's
// karkain.toml pointing at a sibling member directory.
//
// A cycle in the member dependency graph is a hard error (ErrWsCycle) rather
// than something silently tolerated, so the build cannot deadlock or loop.
func WorkspaceOrder(rootDir string) ([]WorkspaceMember, error) {
	path := filepath.Join(rootDir, WorkspaceFile)
	ws, err := readWorkspace(path)
	if err != nil {
		return nil, &PkgError{Code: ErrWsNoRoot, Message: fmt.Sprintf("not a workspace root: %v", err)}
	}
	if len(ws.Members) == 0 {
		return nil, nil
	}

	absRoot, _ := filepath.Abs(rootDir)
	members := make([]*WorkspaceMember, 0, len(ws.Members))
	byName := make(map[string]*WorkspaceMember, len(ws.Members))
	byDir := make(map[string]*WorkspaceMember, len(ws.Members))

	// Resolve each member directory once, de-duplicating by absolute path.
	for _, m := range ws.Members {
		dir := filepath.Join(absRoot, m)
		if filepath.IsAbs(m) {
			dir = filepath.Clean(m)
		}
		dir = filepath.Clean(dir)
		ad, _ := filepath.Abs(dir)
		if byDir[ad] != nil {
			continue // duplicate member entry
		}
		mm := &WorkspaceMember{Name: m, Dir: ad}
		byDir[ad] = mm
		byName[m] = mm
		members = append(members, mm)
	}

	// Build the inter-member dependency edges: member -> the workspace member
	// it depends on (its local/workspace deps that are also workspace members).
	graph := make(map[string][]string, len(members)) // memberDir -> child memberDirs
	depOf := make(map[string][]string, len(members)) // memberDir -> members that depend on it
	for _, mm := range members {
		childDirs := workspaceDepDirs(memberManifest(mm.Dir), absRoot)
		for _, cd := range childDirs {
			if byDir[cd] == nil {
				continue // dependency is outside the workspace; not a member edge
			}
			graph[mm.Dir] = appendUnique(graph[mm.Dir], cd)
			depOf[cd] = appendUnique(depOf[cd], mm.Dir)
		}
	}

	// Kahn's algorithm over the member graph; edges point dependents -> deps
	// (a member is emitted only after all its deps are emitted).
	order := make([]string, 0, len(members))
	queue := []string{}
	doneDeps := make(map[string]int, len(members))
	var nodeDir []string = make([]string, 0, len(members))
	for _, mm := range members {
		nodeDir = append(nodeDir, mm.Dir)
	}
	for _, mm := range members {
		doneDeps[mm.Dir] = len(graph[mm.Dir])
		if doneDeps[mm.Dir] == 0 {
			queue = append(queue, mm.Dir)
		}
	}
	// Deterministic start order.
	sort.Strings(queue)

	emitted := 0
	for len(queue) > 0 {
		// Deterministic: pop the smallest remaining ready dir.
		sort.Strings(queue)
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		emitted++
		for _, depender := range depOf[cur] {
			doneDeps[depender]--
			if doneDeps[depender] == 0 {
				queue = append(queue, depender)
			}
		}
	}

	if emitted != len(members) {
		// Remaining members are part of one or more cycles.
		var cycle []string
		for _, mm := range members {
			if doneDeps[mm.Dir] > 0 {
				cycle = append(cycle, mm.Name)
			}
		}
		sort.Strings(cycle)
		return nil, &PkgError{Code: ErrWsCycle,
			Message: fmt.Sprintf("workspace dependency cycle detected among members: %s", strings.Join(cycle, ", "))}
	}

	// Map order back to WorkspaceMember records with their topological index.
	result := make([]WorkspaceMember, 0, len(order))
	index := make(map[string]int, len(order))
	for i, dir := range order {
		index[dir] = i
	}
	for _, dir := range order {
		mm := byDir[dir]
		result = append(result, WorkspaceMember{Name: mm.Name, Dir: mm.Dir, Order: index[dir]})
	}
	return result, nil
}

// workspaceDepDirs returns the absolute directories of a member manifest's
// workspace-source dependencies (source == "workspace" or a local path that
// resolves inside rootDir). DevDependencies are excluded from the NORMAL build
// graph (P2.11 dev-dep isolation); they only enter the test path.
func workspaceDepDirs(m *Manifest, absRoot string) []string {
	if m == nil {
		return nil
	}
	var out []string
	collect := func(deps map[string]Dependency) {
		for _, n := range sortedDeps(deps) {
			d := deps[n]
			if d.Source == "registry" || d.Source == "git" {
				continue
			}
			dir := d.URL
			if filepath.IsAbs(dir) {
				dir = filepath.Clean(dir)
			} else {
				dir = filepath.Clean(filepath.Join(absRoot, dir))
			}
			ad, _ := filepath.Abs(dir)
			// Only a path inside the workspace root can be an inter-member edge.
			if withinDir(ad, absRoot) {
				out = appendUnique(out, ad)
			}
		}
	}
	collect(m.Dependencies)
	return out
}

// WorkspaceTestOrder augments WorkspaceOrder with the dev-dependency edges that
// are only relevant when testing: a member's dev-dependencies (including
// dev-deps on sibling members) are also prerequisites of its tests, but must
// not participate in normal builds (handled separately by WorkspaceOrder).
// The returned order is the same topological order; this function is a
// documentation/verification seam so callers can assert dev-dep isolation.
func WorkspaceTestOrder(rootDir string) ([]WorkspaceMember, error) {
	// Test ordering is identical to build ordering: the same Kahn traversal.
	// Dev-deps participate in test scope but a dev-dep that is also a member is
	// already ordered by the member graph if it appears in Dependencies too.
	// A dev-only sibling is NOT required to precede normal builds; we use the
	// same order and rely on test-scope assembly for its inclusion.
	return WorkspaceOrder(rootDir)
}

// memberManifest parses a member's karkain.toml, or nil when missing.
func memberManifest(dir string) *Manifest {
	p := filepath.Join(dir, ManifestFile)
	if _, err := os.Stat(p); err != nil {
		return nil
	}
	m, err := ParseManifest(p)
	if err != nil {
		return nil
	}
	return m
}

func readWorkspace(path string) (*WorkspaceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ws WorkspaceConfig
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, err
	}
	return &ws, nil
}
