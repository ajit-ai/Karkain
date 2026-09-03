package pm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeManifest writes a karkain.toml with the given name/version/deps.
// dep values are {name: localRelativePath} and stored as local dependencies
// with URL pointing at "deps/<name>".
func writeManifest(t *testing.T, dir, name, version string, localDeps map[string]string) {
	t.Helper()
	m := &Manifest{Name: name, Version: version, Dependencies: make(map[string]Dependency)}
	for depName, _ := range localDeps {
		m.Dependencies[depName] = Dependency{Name: depName, Version: "1.0.0",
			Source: "local", URL: filepath.Join("deps", depName)}
	}
	if err := WriteManifest(filepath.Join(dir, ManifestFile), m); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// mkLocalDep creates a local dependency package under <proj>/deps/<name>
// with its own manifest and an optional set of transitive local deps.
func mkLocalDep(t *testing.T, proj, name string, transitive map[string]string) {
	t.Helper()
	dir := filepath.Join(proj, "deps", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir dep: %v", err)
	}
	writeManifest(t, dir, name, "1.0.0", transitive)
}

func TestResolver_SingleDirect(t *testing.T) {
	proj := t.TempDir()
	mkLocalDep(t, proj, "liba", nil)
	writeManifest(t, proj, "app", "1.0.0", map[string]string{"liba": ""})

	r, err := NewResolver(proj)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	g, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(g.Nodes))
	}
	node := g.Nodes["liba"]
	if node == nil {
		t.Fatal("expected liba node")
	}
	if !node.Direct {
		t.Error("liba should be a direct dependency")
	}
}

func TestResolver_TransitiveLocal(t *testing.T) {
	proj := t.TempDir()
	mkLocalDep(t, proj, "liba", map[string]string{"libb": ""})
	mkLocalDep(t, proj, "libb", nil)
	writeManifest(t, proj, "app", "1.0.0", map[string]string{"liba": ""})

	r, err := NewResolver(proj)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	g, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes (liba, libb), got %d: %+v", len(g.Nodes), g.Nodes)
	}
	if g.Nodes["libb"] == nil {
		t.Error("expected transitive libb node")
	}
}

func TestResolver_CycleTerminates(t *testing.T) {
	proj := t.TempDir()
	mkLocalDep(t, proj, "liba", map[string]string{"libb": ""})
	mkLocalDep(t, proj, "libb", map[string]string{"liba": ""})
	writeManifest(t, proj, "app", "1.0.0", map[string]string{"liba": ""})

	r, _ := NewResolver(proj)
	g, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve should not error on cycle, got %v", err)
	}
	if g.Nodes["liba"] == nil || g.Nodes["libb"] == nil {
		t.Error("cycle should still produce both nodes")
	}
}

func TestResolver_DeterministicOrder(t *testing.T) {
	proj := t.TempDir()
	mkLocalDep(t, proj, "zlib", nil)
	mkLocalDep(t, proj, "alib", nil)
	writeManifest(t, proj, "app", "1.0.0", map[string]string{"zlib": "", "alib": ""})

	r, _ := NewResolver(proj)
	g1, _ := r.Resolve()
	keys1 := g1.SortKeys()

	r2, _ := NewResolver(proj)
	g2, _ := r2.Resolve()
	keys2 := g2.SortKeys()

	if len(keys1) != 2 || keys1[0] != "alib" || keys1[1] != "zlib" {
		t.Fatalf("expected sorted keys [alib zlib], got %v", keys1)
	}
	if len(keys2) != 2 || keys2[0] != keys1[0] {
		t.Fatalf("non-deterministic order: %v vs %v", keys1, keys2)
	}
}

func TestTreeLines_Formatting(t *testing.T) {
	proj := t.TempDir()
	mkLocalDep(t, proj, "liba", map[string]string{"libb": ""})
	mkLocalDep(t, proj, "libb", nil)
	writeManifest(t, proj, "app", "1.0.0", map[string]string{"liba": ""})

	lines, err := TreeLines(proj)
	if err != nil {
		t.Fatalf("TreeLines: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("expected tree lines")
	}
	if lines[0] != "app" {
		t.Errorf("expected root line 'app', got %q", lines[0])
	}
	joined := strings.Join(lines, "\n")
	if !contains(joined, "liba") {
		t.Errorf("expected 'liba' in tree, got lines: %v", lines)
	}
	// libb must appear exactly once (nested under liba), not as a top-level root.
	if countSub(joined, "libb") != 1 {
		t.Errorf("expected libb once (nested), got lines: %v", lines)
	}
	// libb must appear AFTER liba (nested).
	if idxA, idxB := strings.Index(joined, "liba"), strings.Index(joined, "libb"); idxB < idxA {
		t.Errorf("libb should be nested after liba: %v", lines)
	}
}

func TestLockFromGraph_RoundTrip(t *testing.T) {
	proj := t.TempDir()
	mkLocalDep(t, proj, "liba", nil)
	writeManifest(t, proj, "app", "1.0.0", map[string]string{"liba": ""})

	graph, err := ResolveAndLock(proj)
	if err != nil {
		t.Fatalf("ResolveAndLock: %v", err)
	}
	path := LockPath(proj)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lockfile should exist: %v", err)
	}

	lf, err := ReadLockFile(path)
	if err != nil {
		t.Fatalf("ReadLockFile: %v", err)
	}
	if len(lf.Packages) != len(graph.Nodes) {
		t.Errorf("lock packages (%d) != graph nodes (%d)", len(lf.Packages), len(graph.Nodes))
	}
}

func TestNewProject_Distinct(t *testing.T) {
	parent := t.TempDir()
	res, err := NewProject(parent, "p1")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	if _, err := os.Stat(filepath.Join(res.ProjectDir, ManifestFile)); err != nil {
		t.Fatalf("new project should create manifest: %v", err)
	}

	// Second create should fail (directory exists).
	if _, err := NewProject(parent, "p1"); err == nil {
		t.Error("expected error creating existing project")
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

func countSub(s, sub string) int {
	return strings.Count(s, sub)
}
