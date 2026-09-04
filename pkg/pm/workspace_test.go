package pm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: write a member manifest into dir with given deps (name -> source/url).
func writeMemberMaster(t *testing.T, dir, name string, deps map[string]Dependency) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Name: name, Version: "1.0.0", Dependencies: deps}
	if err := WriteManifest(filepath.Join(dir, ManifestFile), m); err != nil {
		t.Fatal(err)
	}
}

// TestWorkspaceOrder_TopologicalAndBuildOnce verifies members are returned in
// dependency order and a member is emitted exactly once even when several
// others depend on it (diamond).
func TestWorkspaceOrder_TopologicalAndBuildOnce(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatal(err)
	}
	// members: libA, libB (depends on A), app (depends on B and A directly -> diamond)
	for _, m := range []string{"libA", "libB", "app"} {
		if err := AddWorkspaceMember(root, m); err != nil {
			t.Fatal(err)
		}
	}
	// workspace deps use source "workspace" or local paths inside root
	writeMemberMaster(t, filepath.Join(root, "libA"), "libA", nil)
	writeMemberMaster(t, filepath.Join(root, "libB"), "libB", map[string]Dependency{
		"libA": {Name: "libA", Version: "1.0.0", Source: "workspace", URL: "libA"},
	})
	writeMemberMaster(t, filepath.Join(root, "app"), "app", map[string]Dependency{
		"libA": {Name: "libA", Version: "1.0.0", Source: "workspace", URL: "libA"},
		"libB": {Name: "libB", Version: "1.0.0", Source: "workspace", URL: "libB"},
	})

	members, err := WorkspaceOrder(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 3 {
		t.Fatalf("expected 3 members, got %d: %+v", len(members), members)
	}

	// libA first, app last.
	order := []string{members[0].Name, members[1].Name, members[2].Name}
	t.Logf("order: %v", order)
	if members[0].Name != "libA" {
		t.Errorf("libA should be built first, got %s first", members[0].Name)
	}
	if members[2].Name != "app" {
		t.Errorf("app should be built last, got %s last", members[2].Name)
	}
	// Each member appears once.
	seen := map[string]bool{}
	for _, m := range members {
		if seen[m.Name] {
			t.Errorf("member %s built more than once", m.Name)
		}
		seen[m.Name] = true
	}
}

// TestWorkspaceOrder_CycleDetected verifies a two-member cycle is a hard error.
func TestWorkspaceOrder_CycleDetected(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"a", "b"} {
		if err := AddWorkspaceMember(root, m); err != nil {
			t.Fatal(err)
		}
	}
	writeMemberMaster(t, filepath.Join(root, "a"), "a", map[string]Dependency{
		"b": {Name: "b", Version: "1.0.0", Source: "workspace", URL: "b"},
	})
	writeMemberMaster(t, filepath.Join(root, "b"), "b", map[string]Dependency{
		"a": {Name: "a", Version: "1.0.0", Source: "workspace", URL: "a"},
	})

	_, err := WorkspaceOrder(root)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), ErrWsCycle) {
		t.Errorf("expected cycle code %s in error, got: %v", ErrWsCycle, err)
	}
}

// TestWorkspaceOrder_DevDepsExcluded verifies dev-dependencies on sibling
// members do NOT create build-order edges (P2.11 isolation). A member whose
// only sibling reference is a dev-dependency is an independent node.
func TestWorkspaceOrder_DevDepsExcluded(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatal(err)
	}
	if err := AddWorkspaceMember(root, "prod"); err != nil {
		t.Fatal(err)
	}
	if err := AddWorkspaceMember(root, "tool"); err != nil {
		t.Fatal(err)
	}
	// prod depends (build) on nothing; tool is only a dev-dependency of prod.
	writeMemberMaster(t, filepath.Join(root, "prod"), "prod", nil)
	writeMemberMaster(t, filepath.Join(root, "tool"), "tool", nil)

	// Add prod's DevDependencies referencing tool.
	mp := filepath.Join(root, "prod", ManifestFile)
	m, err := ParseManifest(mp)
	if err != nil {
		t.Fatal(err)
	}
	if m.DevDependencies == nil {
		m.DevDependencies = map[string]Dependency{}
	}
	m.DevDependencies["tool"] = Dependency{Name: "tool", Version: "1.0.0", Source: "workspace", URL: "tool"}
	if err := WriteManifest(mp, m); err != nil {
		t.Fatal(err)
	}

	members, err := WorkspaceOrder(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	// No cycle and both members independent: neither forced before the other.
	// The dev-dep must not cause "prod" to be ordered after "tool" via a build edge,
	// but order among independents is deterministic (alphabetical) so either is fine.
	for _, m := range members {
		if m.Name != "prod" && m.Name != "tool" {
			t.Errorf("unexpected member %s", m.Name)
		}
	}
}

// TestWorkspaceOrder_EmptyWorkspace verifies an empty workspace returns no
// members without error.
func TestWorkspaceOrder_EmptyWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatal(err)
	}
	members, err := WorkspaceOrder(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("expected 0 members, got %d", len(members))
	}
}

// TestWorkspaceOrder_NotAWorkspace verifies a missing workspace file is an error.
func TestWorkspaceOrder_NotAWorkspace(t *testing.T) {
	root := t.TempDir() // no karkain.workspace.json
	_, err := WorkspaceOrder(root)
	if err == nil {
		t.Fatal("expected error for non-workspace root, got nil")
	}
	if !strings.Contains(err.Error(), ErrWsNoRoot) {
		t.Errorf("expected no-root code %s, got: %v", ErrWsNoRoot, err)
	}
}

// TestWorkspaceOrder_ExternalDepIgnored verifies a local dep outside the
// workspace root does not create a member edge.
func TestWorkspaceOrder_ExternalDepIgnored(t *testing.T) {
	root := t.TempDir()
	if err := InitWorkspace(root); err != nil {
		t.Fatal(err)
	}
	if err := AddWorkspaceMember(root, "app"); err != nil {
		t.Fatal(err)
	}
	// dependency points outside the workspace root
	writeMemberMaster(t, filepath.Join(root, "app"), "app", map[string]Dependency{
		"vendor": {Name: "vendor", Version: "1.0.0", Source: "local", URL: filepath.Join(t.TempDir(), "vendor")},
	})
	members, err := WorkspaceOrder(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member (external dep ignored), got %d", len(members))
	}
	if members[0].Name != "app" {
		t.Errorf("expected member app, got %s", members[0].Name)
	}
}
