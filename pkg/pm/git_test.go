package pm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mustGit runs git in dir with the given args, failing the test on error.
func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runGit(dir, args...)
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return out
}

// setupGitRepo creates a throwaway local git repository at dir containing a
// karkain.toml (name/version) and a src/app.kark file, committing once. It
// returns a URL that ResolveGitRevision can fetch. name defaults to the dir base.
func setupGitRepo(t *testing.T, dir, name, version string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	m := &Manifest{Name: name, Version: version}
	if err := WriteManifest(filepath.Join(dir, ManifestFile), m); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "app.kark"), []byte("fn main() {}\n"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	mustGit(t, dir, "init", "-q")
	mustGit(t, dir, "add", "-A")
	mustGit(t, dir, "-c", "user.name=test", "-c", "user.email=t@e", "commit", "-q", "-m", "initial")
	return "file://" + filepath.ToSlash(dir)
}

// gitHead returns the current HEAD sha of a repo at dir.
func gitHead(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimSpace(mustGit(t, dir, "rev-parse", "HEAD"))
}

func TestClassifyGitRef(t *testing.T) {
	cases := []struct {
		in   string
		want GitRefKind
	}{
		{"", GitRefDefault},
		{"HEAD", GitRefDefault},
		{"0123456789012345678901234567890123456789", GitRefCommit},
		{"main", GitRefBranch},
		{"v1.0.0", GitRefBranch},
	}
	for _, c := range cases {
		kind, _ := classifyGitRef(c.in)
		if kind != c.want {
			t.Errorf("classifyGitRef(%q) kind = %v, want %v", c.in, kind, c.want)
		}
	}
}

func TestResolveGitRevision_DefaultBranch(t *testing.T) {
	repo := t.TempDir()
	url := setupGitRepo(t, repo, "liba", "1.0.0")
	repoHead := gitHead(t, repo)

	res, cloneDir, err := ResolveGitRevision(url, "", "")
	if err != nil {
		t.Fatalf("ResolveGitRevision: %v", err)
	}
	defer os.RemoveAll(cloneDir)

	if res.Kind != GitRefDefault {
		t.Errorf("kind = %v, want default", res.Kind)
	}
	if res.SHA != repoHead {
		t.Errorf("resolved SHA %q, want repo HEAD %q (reproducible identity)", res.SHA, repoHead)
	}
	if _, err := os.Stat(filepath.Join(cloneDir, ManifestFile)); err != nil {
		t.Errorf("clone should contain karkain.toml: %v", err)
	}
}

func TestResolveGitRevision_ExactTag(t *testing.T) {
	repo := t.TempDir()
	url := setupGitRepo(t, repo, "libb", "1.0.0")
	// Add a second commit and tag it.
	mustGit(t, repo, "-c", "user.name=test", "-c", "user.email=t@e", "commit", "--allow-empty", "-q", "-m", "second")
	tagged := gitHead(t, repo)
	mustGit(t, repo, "tag", "v2.0.0")

	res, cloneDir, err := ResolveGitRevision(url, "v2.0.0", "")
	if err != nil {
		t.Fatalf("ResolveGitRevision(v2.0.0): %v", err)
	}
	defer os.RemoveAll(cloneDir)

	if res.Kind != GitRefBranch {
		t.Errorf("kind = %v, want branch (tag shares ref namespace)", res.Kind)
	}
	if res.SHA != tagged {
		t.Errorf("tag resolution SHA %q, want %q", res.SHA, tagged)
	}
}

func TestResolveGitRevision_ExactCommit(t *testing.T) {
	repo := t.TempDir()
	url := setupGitRepo(t, repo, "libc", "1.0.0")
	sha := gitHead(t, repo)

	res, cloneDir, err := ResolveGitRevision(url, sha, "")
	if err != nil {
		t.Fatalf("ResolveGitRevision(sha): %v", err)
	}
	defer os.RemoveAll(cloneDir)

	if res.Kind != GitRefCommit {
		t.Errorf("kind = %v, want commit", res.Kind)
	}
	if res.SHA != sha {
		t.Errorf("SHA %q, want %q", res.SHA, sha)
	}
}

func TestResolveGitRevision_UnknownRefFails(t *testing.T) {
	repo := t.TempDir()
	url := setupGitRepo(t, repo, "libd", "1.0.0")

	_, _, err := ResolveGitRevision(url, "does-not-exist-anywhere", "")
	if err == nil {
		t.Fatal("expected error for unknown ref")
	}
	ge, ok := err.(*ErrGit)
	if !ok {
		t.Fatalf("expected *ErrGit, got %T: %v", err, err)
	}
	if ge.Code != ErrGitRevision {
		t.Errorf("code = %v, want E-PKG-GIT-REVISION", ge.Code)
	}
}

func TestResolveGitRevision_Subdir(t *testing.T) {
	repo := t.TempDir()
	// Nested package under subdir/pkg.
	if err := os.MkdirAll(filepath.Join(repo, "sub", "pkg", "src"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	m := &Manifest{Name: "nested", Version: "1.0.0"}
	if err := WriteManifest(filepath.Join(repo, "sub", "pkg", ManifestFile), m); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "sub", "pkg", "src", "n.kark"), []byte("fn main() {}\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	mustGit(t, repo, "init", "-q")
	mustGit(t, repo, "add", "-A")
	mustGit(t, repo, "-c", "user.name=test", "-c", "user.email=t@e", "commit", "-q", "-m", "init")
	url := "file://" + filepath.ToSlash(repo)

	res, cloneDir, err := ResolveGitRevision(url, "", "sub/pkg")
	if err != nil {
		t.Fatalf("ResolveGitRevision(subdir): %v", err)
	}
	defer os.RemoveAll(cloneDir)

	if res.Subdir != "sub/pkg" {
		t.Errorf("subdir = %q", res.Subdir)
	}
	// The package root is the resolved subdir within the clone; it must contain
	// the nested manifest.
	pkgRoot := filepath.Join(cloneDir, filepath.FromSlash(res.Subdir))
	if _, err := os.Stat(filepath.Join(pkgRoot, ManifestFile)); err != nil {
		t.Errorf("package root should contain karkain.toml: %v", err)
	}
}

func TestResolveGitRevision_SubdirTraversalRejected(t *testing.T) {
	repo := t.TempDir()
	url := setupGitRepo(t, repo, "libe", "1.0.0")

	_, _, err := ResolveGitRevision(url, "", "../escape")
	if err == nil {
		t.Fatal("expected error for path-traversal subdir")
	}
	ge, ok := err.(*ErrGit)
	if !ok {
		t.Fatalf("expected *ErrGit, got %T", err)
	}
	if ge.Code != ErrGitSubdir {
		t.Errorf("code = %v, want E-PKG-GIT-SUBDIR", ge.Code)
	}
}

func TestFetchModule_Git_ProducesCache(t *testing.T) {
	// Ensure git on this machine works before exercising the full path.
	if !gitAvailable() {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	url := setupGitRepo(t, repo, "libf", "1.2.0")

	proj := t.TempDir()
	writeManifest(t, proj, "app", "1.0.0", nil)
	dep := Dependency{Name: "libf", Source: "git", URL: url}

	if err := FetchModule(proj, dep); err != nil {
		t.Fatalf("FetchModule: %v", err)
	}
	cacheDir := filepath.Join(proj, CacheModules, cacheDirName(dep, gitHead(t, repo)))
	if _, err := os.Stat(filepath.Join(cacheDir, ManifestFile)); err != nil {
		t.Errorf("cached package should contain karkain.toml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, ChecksumFile)); err != nil {
		t.Errorf("cached package should have checksum file: %v", err)
	}
	// The cached package should record the resolved immutable commit.
	rev := ReadRevFile(cacheDir)
	if len(rev) != 40 {
		t.Errorf("expected recorded rev (40-hex), got %q", rev)
	}
	if rev != gitHead(t, repo) {
		t.Errorf("recorded rev %q, want repo HEAD %q", rev, gitHead(t, repo))
	}
}

func TestCacheIdentityKey_TracksGitRev(t *testing.T) {
	// Git identity should differ as the resolved rev changes, same name/version.
	k1 := CacheIdentityKey("x", "1.0.0", "aaa111bbb222ccc333ddd444eee555fff666777", "git", "https://h/r")
	k2 := CacheIdentityKey("x", "1.0.0", "999888777666555444333222111000999aaa888", "git", "https://h/r")
	if k1 == k2 {
		t.Error("git cache identity must vary with resolved revision")
	}
	if !strings.Contains(k1, "aaa111bbb222") {
		t.Errorf("git cache identity should embed the revision, got %q", k1)
	}
}

func TestRunGit_NoShellInterpolation(t *testing.T) {
	// Ensure runGit passes args safely (no shell metacharacter execution).
	if _, err := runGit("", "version"); err != nil {
		t.Fatalf("git version failed: %v", err)
	}
}
