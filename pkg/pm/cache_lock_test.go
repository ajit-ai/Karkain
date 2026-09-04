package pm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/pm/regserver"
)

// ============================================================
// P2.7 - Source-aware cache identity
// ============================================================

func TestCacheDirName_SourceAware(t *testing.T) {
	// Registry (no url) -> legacy name@version (VerifyIntegrity compatible).
	reg := Dependency{Name: "math", Version: "1.2.3", Source: "registry"}
	if got := cacheDirName(reg, ""); got != "math@1.2.3" {
		t.Errorf("registry cache key = %q, want math@1.2.3", got)
	}

	// Git -> name@rev (source-aware, immutable identity).
	git := Dependency{Name: "vec", Version: "main", Source: "git", URL: "https://example.com/v.git"}
	const rev = "0123456789abcdef0123456789abcdef01234567"
	if got := cacheDirName(git, rev); got != "vec@"+rev {
		t.Errorf("git cache key = %q, want vec@<rev>", got)
	}

	// Git without resolved rev falls back to the requested ref.
	if got := cacheDirName(git, ""); got != "vec@main" {
		t.Errorf("git no-rev cache key = %q, want vec@main", got)
	}

	// Local preserves legacy `name`.
	loc := Dependency{Name: "locallib", Version: "0.1.0", Source: "local", URL: "../locallib"}
	if got := cacheDirName(loc, ""); got != "locallib" {
		t.Errorf("local cache key = %q, want locallib", got)
	}
}

func TestCacheDirName_GitRevDiffersByIdentity(t *testing.T) {
	git := Dependency{Name: "vec", Version: "main", Source: "git", URL: "https://example.com/v.git"}
	k1 := cacheDirName(git, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	k2 := cacheDirName(git, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if k1 == k2 {
		t.Error("two git revisions of the same package must not share a cache dir")
	}
}

// ============================================================
// P2.8 - Cache-first / offline resolution
// ============================================================

func TestCachedValid_FalseWhenAbsent(t *testing.T) {
	proj := t.TempDir()
	dep := Dependency{Name: "none", Version: "1.0.0", Source: "registry"}
	ok, err := CachedValid(proj, dep, "")
	if err != nil {
		t.Fatalf("CachedValid: %v", err)
	}
	if ok {
		t.Error("CachedValid should be false for a missing package")
	}
}

func TestCachedValid_LocalNeverCachedByChecksum(t *testing.T) {
	proj := t.TempDir()
	dep := Dependency{Name: "localx", Version: "1.0.0", Source: "local", URL: "../x"}
	ok, _ := CachedValid(proj, dep, "")
	if ok {
		t.Error("local deps should not be reported cache-valid (canonical source policy)")
	}
}

func TestFetchLocked_CacheFirstSkipsNetwork(t *testing.T) {
	// A registry package fully present + verified in cache should be satisfied
	// without any registry by registeredFetch (offline path).
	srv := regserver.New()
	defer srv.Close()
	withRegistry(t, "http://127.0.0.1:1") // unreachable; cache must suffice

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "offline.kark"), []byte("fn offline() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	cacheDir := filepath.Join(dir, CacheModules, "offlib@1.0.0")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "offline.kark"), []byte("fn offline() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteChecksumFile(cacheDir); err != nil {
		t.Fatal(err)
	}

	entry := LockEntry{Name: "offlib", Version: "1.0.0", Source: "registry"}
	dep := Dependency{Name: "offlib", Version: "1.0.0", Source: "registry"}
	if ok, _ := CachedValid(dir, dep, ""); !ok {
		t.Fatal("expected cache-valid true for the pre-populated package")
	}
	if err := registeredFetch(dir, entry); err != nil {
		t.Fatalf("registeredFetch should use cache offline, got: %v", err)
	}
}

// ============================================================
// P2.9 - Lock-file pinning (git rev)
// ============================================================

func TestLockFromGraph_RecordsGitRev(t *testing.T) {
	graph := &DepGraph{
		Root: "app",
		Nodes: map[string]*GraphNode{
			"gitdep": {Name: "gitdep", Version: "main", Source: "git", URL: "https://example.com/r.git", Rev: "c0ffee0000000000000000000000000000000000", Direct: true},
			"regdep": {Name: "regdep", Version: "2.0.0", Source: "registry", Direct: false},
		},
	}
	lf := LockFromGraph(graph)
	byName := map[string]LockEntry{}
	for _, e := range lf.Packages {
		byName[e.Name] = e
	}
	g, ok := byName["gitdep"]
	if !ok {
		t.Fatal("gitdep missing from lock")
	}
	if g.Rev != "c0ffee0000000000000000000000000000000000" {
		t.Errorf("git dep rev = %q, want pinned commit", g.Rev)
	}
}

func TestLock_GitRevRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LockFileName)
	lf := &LockFile{Packages: []LockEntry{
		{Name: "gitdep", Version: "main", Source: "git", URL: "https://example.com/r.git", Rev: "0123456789abcdef0123456789abcdef01234567", Integrity: true},
	}}
	if err := WriteLockFile(path, lf); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadLockFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Packages[0].Rev != "0123456789abcdef0123456789abcdef01234567" {
		t.Errorf("rev round-trip failed: %q", got.Packages[0].Rev)
	}
	if got.Packages[0].URL != "https://example.com/r.git" {
		t.Errorf("url round-trip failed: %q", got.Packages[0].URL)
	}
}

func TestResolveAndLock_PinsGitRev(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	url := setupGitRepo(t, repo, "libpin", "1.0.0")
	head := gitHead(t, repo)

	proj := t.TempDir()
	m := &Manifest{Name: "app", Version: "1.0.0", Dependencies: map[string]Dependency{
		"libpin": {Name: "libpin", Source: "git", URL: url},
	}}
	if err := WriteManifest(filepath.Join(proj, ManifestFile), m); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := ResolveAndLock(proj); err != nil {
		t.Fatalf("ResolveAndLock: %v", err)
	}
	lf, err := ReadLockFile(LockPath(proj))
	if err != nil {
		t.Fatalf("ReadLockFile: %v", err)
	}
	if len(lf.Packages) != 1 {
		t.Fatalf("expected 1 locked package, got %d", len(lf.Packages))
	}
	if lf.Packages[0].Rev != head {
		t.Errorf("locked rev = %q, want repo head %q (git dep must be pinned)", lf.Packages[0].Rev, head)
	}
	if lf.Packages[0].Source != "git" {
		t.Errorf("source = %q, want git", lf.Packages[0].Source)
	}
}

func TestCachedValid_GitRev(t *testing.T) {
	// A git package cached under name@rev must be reported valid for that rev.
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, CacheModules, "g@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "g.kark"), []byte("fn g() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteChecksumFile(cacheDir); err != nil {
		t.Fatal(err)
	}
	dep := Dependency{Name: "g", Source: "git", URL: "https://example.com/g.git"}
	if ok, _ := CachedValid(dir, dep, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); !ok {
		t.Error("git package at resolved rev should be cache-valid")
	}
	// A different rev must NOT be satisfied by the same cache entry.
	if ok2, _ := CachedValid(dir, dep, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); ok2 {
		t.Error("wrong rev must not be satisfied by cache")
	}
}

func TestVerifyIntegrity_SourceAwareCache(t *testing.T) {
	// VerifyIntegrity must find a git package cached under name@rev.
	dir := t.TempDir()
	const rev = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cacheDir := filepath.Join(dir, CacheModules, "g@"+rev)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "g.kark"), []byte("fn g() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	// Lock records the source-aware git identity.
	lf := &LockFile{Packages: []LockEntry{
		{Name: "g", Version: "main", Source: "git", URL: "https://example.com/g.git", Rev: rev},
	}}
	if err := WriteLockFile(LockPath(dir), lf); err != nil {
		t.Fatal(err)
	}
	res := VerifyIntegrity(dir)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if !res[0].Valid {
		t.Errorf("git package should verify as cached: %+v", res[0])
	}
	if strings.TrimSpace(res[0].Name) == "" {
		t.Error("unexpected empty name")
	}
}
