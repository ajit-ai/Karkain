package pm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================
// P2.1 — Manifest + Package Identity Hardening
// ============================================================

func TestManifest_RepositoryField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestFile)
	m := &Manifest{
		Name:       "myapp",
		Version:    "1.0.0",
		Repository: "https://github.com/ajit-ai/myapp",
	}
	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Repository != "https://github.com/ajit-ai/myapp" {
		t.Errorf("repository round-trip failed: got %q", got.Repository)
	}
}

func TestManifest_DevDependencies_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestFile)
	m := &Manifest{
		Name:    "myapp",
		Version: "1.0.0",
		Dependencies: map[string]Dependency{
			"prod_lib": {Name: "prod_lib", Version: "1.0.0", Source: "registry"},
		},
		DevDependencies: map[string]Dependency{
			"test_util": {Name: "test_util", Version: "0.2.0", Source: "registry"},
		},
	}
	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := got.Dependencies["prod_lib"]; !ok {
		t.Error("prod dep missing after round trip")
	}
	if _, ok := got.DevDependencies["test_util"]; !ok {
		t.Error("dev dep missing after round trip")
	}
	if _, ok := got.Dependencies["test_util"]; ok {
		t.Error("dev dep leaked into production dependencies")
	}
}

func TestManifest_DevDepsNotInProdResolution(t *testing.T) {
	// Rule 8: dev-dependencies must not resolve as production deps.
	dir := t.TempDir()
	writeManifest(t, dir, "root_pkg", "1.0.0", nil)
	path := filepath.Join(dir, ManifestFile)
	m, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m.DevDependencies = map[string]Dependency{
		"test_only": {Name: "test_only", Version: "9.9.9", Source: "local", URL: filepath.Join(dir, "nonexistent_dep")},
	}
	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := NewResolver(dir)
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	g, err := res.Resolve()
	if err != nil {
		t.Fatalf("resolve should not fail on unresolvable dev dep: %v", err)
	}
	if _, ok := g.Nodes["test_only"]; ok {
		t.Error("dev dependency leaked into the production dependency graph")
	}
}

func TestManifest_Validate_InvalidName(t *testing.T) {
	cases := []string{"", "has space", "UPPER", "starts-with-dash-", "-leaddash", "trail-", "bad!name"}
	for _, name := range cases {
		m := &Manifest{Name: name, Version: "1.0.0"}
		issues := ValidateManifest(m)
		if !containsAny(issues, "not a valid package name") && !containsAny(issues, "name is required") {
			t.Errorf("name %q should produce a validation issue, got %v", name, issues)
		}
	}
}

func TestManifest_Validate_ValidName(t *testing.T) {
	cases := []string{"myapp", "my-app", "my_app", "a1", "x-y_z"}
	for _, name := range cases {
		m := &Manifest{Name: name, Version: "1.0.0"}
		if issues := ValidateManifest(m); containsAny(issues, "not a valid package name") {
			t.Errorf("valid name %q wrongly flagged: %v", name, issues)
		}
	}
}

func TestManifest_Validate_NewRules(t *testing.T) {
	// git dep missing url
	m1 := &Manifest{Name: "app", Version: "1.0.0",
		Dependencies: map[string]Dependency{"g": {Name: "g", Version: "1.0.0", Source: "git"}}}
	if !containsAny(ValidateManifest(m1), "requires a url") {
		t.Errorf("git dep without url should be flagged")
	}

	// local dep missing url
	m2 := &Manifest{Name: "app", Version: "1.0.0",
		Dependencies: map[string]Dependency{"l": {Name: "l", Version: "1.0.0", Source: "local"}}}
	if !containsAny(ValidateManifest(m2), "requires a url") {
		t.Errorf("local dep without url should be flagged")
	}

	// unknown source
	m3 := &Manifest{Name: "app", Version: "1.0.0",
		Dependencies: map[string]Dependency{"x": {Name: "x", Version: "1.0.0", Source: "bogus"}}}
	if !containsAny(ValidateManifest(m3), "unknown source") {
		t.Errorf("unknown source should be flagged")
	}

	// missing version
	m4 := &Manifest{Name: "app", Version: "1.0.0",
		Dependencies: map[string]Dependency{"x": {Name: "x", Source: "registry"}}}
	if !containsAny(ValidateManifest(m4), "missing a version") {
		t.Errorf("missing dep version should be flagged")
	}
}

func TestManifest_Validate_ValidFull(t *testing.T) {
	m := &Manifest{
		Name:    "app",
		Version: "1.0.0",
		Dependencies: map[string]Dependency{
			"reg":  {Name: "reg", Version: "1.0.0", Source: "registry"},
			"gitx": {Name: "gitx", Version: "1.0.0", Source: "git", URL: "https://example.com/repo.git"},
			"loc":  {Name: "loc", Version: "1.0.0", Source: "local", URL: "../loc"},
		},
	}
	if issues := ValidateManifest(m); len(issues) != 0 {
		t.Errorf("valid manifest flagged: %v", issues)
	}
}

func containsAny(issues []string, sub string) bool {
	for _, i := range issues {
		if strings.Contains(i, sub) {
			return true
		}
	}
	return false
}

// ============================================================
// P2.3 — Source Abstraction
// ============================================================

func TestSourceKind_Parse(t *testing.T) {
	cases := map[string]struct {
		kind SourceKind
		ok   bool
	}{
		"local":     {SourceLocal, true},
		"workspace": {SourceWorkspace, true},
		"git":       {SourceGit, true},
		"registry":  {SourceRegistry, true},
		"bogus":     {"", false},
		"":          {"", false},
	}
	for s, want := range cases {
		kind, ok := ParseSourceKind(s)
		if kind != want.kind || ok != want.ok {
			t.Errorf("ParseSourceKind(%q) = (%q,%v), want (%q,%v)", s, kind, ok, want.kind, want.ok)
		}
	}
}

func TestCacheIdentityKey_SourceAware(t *testing.T) {
	gitKey := CacheIdentityKey("foo", "1.0.0", "abc123", "git", "https://github.com/a/b.git")
	regKey := CacheIdentityKey("foo", "1.0.0", "", "registry", "")
	if gitKey == regKey {
		t.Error("git and registry identities for same name@version must differ")
	}
	if !strings.Contains(gitKey, "abc123") {
		t.Errorf("git identity should embed resolved revision, got %q", gitKey)
	}
}

func TestCacheIdentityKey_SameSourceURLAliases(t *testing.T) {
	k1 := CacheIdentityKey("foo", "1.0.0", "", "git", "https://github.com/a/b.git")
	k2 := CacheIdentityKey("foo", "1.0.0", "", "git", "https://github.com/a/b")
	if k1 != k2 {
		t.Errorf("trailing .git should normalize to same identity: %q vs %q", k1, k2)
	}
}

func TestSanitizeCacheComponent_NoTraversal(t *testing.T) {
	if got := SanitizeCacheComponent("../evil"); strings.Contains(got, "/") || strings.Contains(got, "\\") {
		t.Errorf("sanitize must remove path separators, got %q", got)
	}
	if got := SanitizeCacheComponent("a b/c"); strings.Contains(got, " ") {
		t.Errorf("sanitize must remove spaces, got %q", got)
	}
}

func TestWorkspaceSourceKind_ValidInManifest(t *testing.T) {
	m := &Manifest{Name: "app", Version: "1.0.0",
		Dependencies: map[string]Dependency{"w": {Name: "w", Version: "1.0.0", Source: "workspace"}}}
	if issues := ValidateManifest(m); len(issues) != 0 {
		t.Errorf("workspace source should validate cleanly: %v", issues)
	}
}

func TestWriteManifest_FileExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ManifestFile)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("expect manifest absent before write")
	}
	m := &Manifest{Name: "app", Version: "1.0.0"}
	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("manifest should exist after write: %v", err)
	}
}
