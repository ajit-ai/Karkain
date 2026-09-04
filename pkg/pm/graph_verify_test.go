package pm

import (
	"path/filepath"
	"testing"
)

// ============================================================
// P2.2 — Dependency Graph Verification
// ============================================================

func TestResolver_MultipleDirect(t *testing.T) {
	proj := t.TempDir()
	mkLocalDep(t, proj, "liba", nil)
	mkLocalDep(t, proj, "libb", nil)
	mkLocalDep(t, proj, "libc", nil)
	writeManifest(t, proj, "app", "1.0.0", map[string]string{"liba": "", "libb": "", "libc": ""})

	r, err := NewResolver(proj)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	g, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(g.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(g.Nodes))
	}
	for _, n := range []string{"liba", "libb", "libc"} {
		if g.Nodes[n] == nil {
			t.Errorf("missing node %s", n)
			continue
		}
		if !g.Nodes[n].Direct {
			t.Errorf("%s should be direct", n)
		}
	}
}

func TestResolver_DeepTransitiveChain(t *testing.T) {
	proj := t.TempDir()
	mkLocalDep(t, proj, "liba", map[string]string{"libb": ""})
	mkLocalDep(t, proj, "libb", map[string]string{"libc": ""})
	mkLocalDep(t, proj, "libc", map[string]string{"libd": ""})
	mkLocalDep(t, proj, "libd", nil)
	writeManifest(t, proj, "app", "1.0.0", map[string]string{"liba": ""})

	r, _ := NewResolver(proj)
	g, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(g.Nodes) != 4 {
		t.Fatalf("expected 4 nodes (a,b,c,d), got %d", len(g.Nodes))
	}
	for _, n := range []string{"liba", "libb", "libc", "libd"} {
		if g.Nodes[n] == nil {
			t.Errorf("missing transitive node %s", n)
		}
	}
	// Only liba should be direct; libb..libd are transitive.
	if g.Nodes["liba"].Direct != true {
		t.Error("liba should be direct")
	}
	for _, n := range []string{"libb", "libc", "libd"} {
		if g.Nodes[n].Direct {
			t.Errorf("%s should NOT be direct", n)
		}
	}
}

func TestResolver_ChildrenRelationships(t *testing.T) {
	proj := t.TempDir()
	mkLocalDep(t, proj, "liba", map[string]string{"libb": "", "libc": ""})
	mkLocalDep(t, proj, "libb", nil)
	mkLocalDep(t, proj, "libc", nil)
	writeManifest(t, proj, "app", "1.0.0", map[string]string{"liba": ""})

	r, _ := NewResolver(proj)
	g, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	children := g.Children("liba")
	if len(children) != 2 {
		t.Errorf("liba should have 2 children (libb, libc), got %v", children)
	}
	if children[0] != "libb" || children[1] != "libc" {
		t.Errorf("children not sorted: %v", children)
	}
}

func TestResolver_MissingLocalDep(t *testing.T) {
	proj := t.TempDir()
	// liba declared but its deps/liba directory/manifest does not exist.
	writeManifest(t, proj, "app", "1.0.0", map[string]string{"liba": ""})

	r, _ := NewResolver(proj)
	g, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve should not hard-fail on missing local dep manifest: %v", err)
	}
	// expandLocal returns early for missing manifest, leaving only the decl node.
	if g.Nodes["liba"] == nil {
		t.Error("missing local dep should still appear as declared node")
	}
	if len(g.Nodes) != 1 {
		t.Errorf("expected only declared node, got %d: %+v", len(g.Nodes), g.Nodes)
	}
}

func TestResolver_SourceKindOnNodes(t *testing.T) {
	proj := t.TempDir()
	mkLocalDep(t, proj, "liba", nil)
	writeManifest(t, proj, "app", "1.0.0", map[string]string{"liba": ""})

	// Add a non-local dependency by editing the manifest directly.
	path := filepath.Join(proj, ManifestFile)
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m.Dependencies["remotegit"] = Dependency{Name: "remotegit", Version: "2.0.0", Source: "git", URL: "https://example.com/r.git"}
	m.Dependencies["remotereg"] = Dependency{Name: "remotereg", Version: "1.5.0", Source: "registry"}
	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("write: %v", err)
	}

	r, _ := NewResolver(proj)
	g, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if g.Nodes["liba"].Source != "local" {
		t.Errorf("liba source = %q, want local", g.Nodes["liba"].Source)
	}
	if g.Nodes["remotegit"].Source != "git" {
		t.Errorf("remotegit source = %q, want git", g.Nodes["remotegit"].Source)
	}
	if g.Nodes["remotereg"].Source != "registry" {
		t.Errorf("remotereg source = %q, want registry", g.Nodes["remotereg"].Source)
	}
}

func TestResolver_HardVersionConflict(t *testing.T) {
	proj := t.TempDir()
	// Two direct deps with conflicting declared versions.
	path := filepath.Join(proj, ManifestFile)
	m := &Manifest{Name: "app", Version: "1.0.0", Dependencies: map[string]Dependency{
		"liba": {Name: "liba", Version: "1.0.0", Source: "local", URL: filepath.Join("deps", "liba")},
	}}
	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Create a resolver, then inject a conflicting dep call directly.
	r, err := NewResolver(proj)
	if err != nil {
		t.Fatalf("NewResolver: %v", err)
	}
	r.add("liba", Dependency{Name: "liba", Version: "1.0.0", Source: "local", URL: "deps/liba"}, "app", true)
	r.add("liba", Dependency{Name: "liba", Version: "2.0.0", Source: "local", URL: "deps/liba"}, "other", true)
	if len(r.Errors()) == 0 {
		t.Fatal("expected a conflict error when adding two versions of same package")
	}
}

func TestResolver_ResolveDirect_ExcludesDev(t *testing.T) {
	proj := t.TempDir()
	writeManifest(t, proj, "app", "1.0.0", map[string]string{"liba": ""})
	path := filepath.Join(proj, ManifestFile)
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m.DevDependencies = map[string]Dependency{
		"testdep": {Name: "testdep", Version: "1.0.0", Source: "local", URL: filepath.Join("deps", "testdep")},
	}
	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("write: %v", err)
	}

	r, _ := NewResolver(proj)
	g := r.ResolveDirect()
	if _, ok := g.Nodes["testdep"]; ok {
		t.Error("ResolveDirect must not pull dev-dependencies")
	}
}
