package pm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeProject writes a minimal karkain.toml for a project at dir.
func makeProject(t *testing.T, dir string, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	m := &Manifest{Name: name, Version: "1.0.0", Dependencies: map[string]Dependency{}}
	if err := WriteManifest(filepath.Join(dir, ManifestFile), m); err != nil {
		t.Fatal(err)
	}
}

// addDep adds a dependency of the given source/kind/url to dir's manifest.
func addDep(t *testing.T, dir, name string, dep Dependency) {
	t.Helper()
	p := filepath.Join(dir, ManifestFile)
	m, err := ParseManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]Dependency{}
	}
	m.Dependencies[name] = dep
	if err := WriteManifest(p, m); err != nil {
		t.Fatal(err)
	}
}

// addDevDep adds a dev-dependency to dir's manifest.
func addDevDep(t *testing.T, dir, name string, dep Dependency) {
	t.Helper()
	p := filepath.Join(dir, ManifestFile)
	m, err := ParseManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.DevDependencies == nil {
		m.DevDependencies = map[string]Dependency{}
	}
	m.DevDependencies[name] = dep
	if err := WriteManifest(p, m); err != nil {
		t.Fatal(err)
	}
}

// seedCacheDir creates a fake "fetched" package directory in the project cache
// at the source-aware identity.
func seedCacheDir(t *testing.T, projectDir, dirName string) string {
	t.Helper()
	dir := filepath.Join(projectDir, CacheModules, dirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.kark"), []byte("func depfn() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestDependencySources_LocalResolvesToCanonicalPath verifies a local dep maps
// to its on-disk canonical source path (not the cache).
func TestDependencySources_LocalResolvesToCanonicalPath(t *testing.T) {
	root := t.TempDir()
	makeProject(t, root, "app")
	depDir := filepath.Join(root, "mylib")
	if err := os.MkdirAll(depDir, 0755); err != nil {
		t.Fatal(err)
	}
	WriteManifest(filepath.Join(depDir, ManifestFile), &Manifest{Name: "mylib", Version: "0.1.0"})
	addDep(t, root, "mylib", Dependency{Name: "mylib", Version: "0.1.0", Source: "local", URL: "mylib"})

	srcs := DependencySources(root)
	if len(srcs) != 1 {
		t.Fatalf("expected 1 dep, got %d: %+v", len(srcs), srcs)
	}
	s := srcs[0]
	if s.Name != "mylib" {
		t.Errorf("name = %s", s.Name)
	}
	if !s.Resolved() {
		t.Errorf("local dep should resolve, got Err: %v", s.Err)
	}
	if s.Cached {
		t.Errorf("local dep should not be marked cached")
	}
	want, _ := filepath.Abs(depDir)
	if s.Dir != want {
		t.Errorf("Dir = %q, want %q", s.Dir, want)
	}
}

// TestDependencySources_LocalMissing verifies a missing local dep reports an error.
func TestDependencySources_LocalMissing(t *testing.T) {
	root := t.TempDir()
	makeProject(t, root, "app")
	addDep(t, root, "ghost", Dependency{Name: "ghost", Version: "0.1.0", Source: "local", URL: "deps/ghost"})

	srcs := DependencySources(root)
	if len(srcs) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(srcs))
	}
	if srcs[0].Resolved() {
		t.Errorf("missing local dep should not resolve")
	}
	if srcs[0].Err == nil || !strings.Contains(srcs[0].Err.Error(), string(ErrLocal)) {
		t.Errorf("expected local-not-found code, got: %v", srcs[0].Err)
	}
}

// TestDependencySources_RegistryFetched vs NotFetched verifies registry deps
// resolve to the cached dir only after being fetched, and error otherwise.
func TestDependencySources_RegistryFetched(t *testing.T) {
	root := t.TempDir()
	makeProject(t, root, "app")
	addDep(t, root, "rlib", Dependency{Name: "rlib", Version: "3.0.0", Source: "registry"})

	// Not yet fetched -> not resolved.
	srcs := DependencySources(root)
	if len(srcs) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(srcs))
	}
	if srcs[0].Resolved() {
		t.Errorf("unfetched registry dep should not resolve")
	}
	if srcs[0].Err == nil || !strings.Contains(srcs[0].Err.Error(), string(ErrRegistry)) {
		t.Errorf("expected registry-not-fetched code, got: %v", srcs[0].Err)
	}

	// Now seed the cache at the source-aware identity name@version and re-check.
	seedCacheDir(t, root, "rlib@3.0.0")
	srcs = DependencySources(root)
	if len(srcs) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(srcs))
	}
	if !srcs[0].Resolved() {
		t.Fatalf("fetched registry dep should resolve, got Err: %v", srcs[0].Err)
	}
	if !srcs[0].Cached {
		t.Errorf("registry dep should be marked cached")
	}
	want := filepath.Join(root, CacheModules, "rlib@3.0.0")
	if srcs[0].Dir != want {
		t.Errorf("Dir = %q, want %q", srcs[0].Dir, want)
	}
}

// TestDependencySources_GitUsesLockRev verifies a git dep resolves to the
// source-aware cache dir keyed by its locked revision.
func TestDependencySources_GitUsesLockRev(t *testing.T) {
	root := t.TempDir()
	makeProject(t, root, "app")
	dep := Dependency{Name: "glib", Version: "v1.0.0", Source: "git", URL: "file:///repo/glib"}
	addDep(t, root, "glib", dep)

	// Write a lock with the pinned rev.
	rev := strings.Repeat("a", 40)
	WriteLockFile(filepath.Join(root, LockFileName), &LockFile{Packages: []LockEntry{
		{Name: "glib", Version: "v1.0.0", Source: "git", URL: "file:///repo/glib", Rev: rev},
	}})

	// Not fetched yet.
	srcs := DependencySources(root)
	if srcs[0].Resolved() {
		t.Errorf("unfetched git dep should not resolve")
	}

	// Seed cache at name@rev (lock) and re-check.
	seedCacheDir(t, root, "glib@"+rev)
	srcs = DependencySources(root)
	if len(srcs) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(srcs))
	}
	if !srcs[0].Resolved() {
		t.Fatalf("fetched git dep should resolve, got Err: %v", srcs[0].Err)
	}
	if srcs[0].Rev != rev {
		t.Errorf("Rev = %q, want %q", srcs[0].Rev, rev)
	}
	if !srcs[0].Cached {
		t.Errorf("git dep should be marked cached")
	}
	want := filepath.Join(root, CacheModules, "glib@"+rev)
	if srcs[0].Dir != want {
		t.Errorf("Dir = %q, want %q", srcs[0].Dir, want)
	}
}

// TestDependencySources_DevDepsExcluded verifies dev-dependencies never appear
// in the build dependency sources.
func TestDependencySources_DevDepsExcluded(t *testing.T) {
	root := t.TempDir()
	makeProject(t, root, "app")
	addDep(t, root, "prodlib", Dependency{Name: "prodlib", Version: "1.0.0", Source: "local", URL: "prodlib"})
	addDevDep(t, root, "devtool", Dependency{Name: "devtool", Version: "1.0.0", Source: "registry"})

	// seed a cache for devtool so it WOULD resolve if it leaked into build
	seedCacheDir(t, root, "devtool@1.0.0")

	srcs := DependencySources(root)
	for _, s := range srcs {
		if s.Name == "devtool" {
			t.Errorf("dev-dependency devtool leaked into build sources: %+v", s)
		}
	}
	if len(srcs) != 1 || srcs[0].Name != "prodlib" {
		t.Errorf("expected exactly prodlib, got: %+v", srcs)
	}
}

// TestDependencySources_SortedDeterministic verifies output is sorted by name.
func TestDependencySources_SortedDeterministic(t *testing.T) {
	root := t.TempDir()
	makeProject(t, root, "app")
	// add in reverse-alphabetical declaration order
	addDep(t, root, "zed", Dependency{Name: "zed", Version: "1.0.0", Source: "registry"})
	addDep(t, root, "alpha", Dependency{Name: "alpha", Version: "1.0.0", Source: "registry"})
	addDep(t, root, "mid", Dependency{Name: "mid", Version: "1.0.0", Source: "registry"})
	seedCacheDir(t, root, "zed@1.0.0")
	seedCacheDir(t, root, "alpha@1.0.0")
	seedCacheDir(t, root, "mid@1.0.0")

	srcs := DependencySources(root)
	if len(srcs) != 3 {
		t.Fatalf("expected 3 deps, got %d", len(srcs))
	}
	want := []string{"alpha", "mid", "zed"}
	for i, s := range srcs {
		if s.Name != want[i] {
			t.Errorf("order[%d] = %s, want %s", i, s.Name, want[i])
		}
	}
}

// TestDependencySources_Empty verifies a project with no deps yields none.
func TestDependencySources_Empty(t *testing.T) {
	root := t.TempDir()
	makeProject(t, root, "solo")
	srcs := DependencySources(root)
	if len(srcs) != 0 {
		t.Fatalf("expected 0 deps, got %d: %+v", len(srcs), srcs)
	}
}
