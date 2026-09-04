package pm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitRefKind classifies the requested git reference for a dependency so the
// resolver can resolve it deterministically to an immutable commit SHA.
type GitRefKind string

const (
	// GitRefDefault resolves to the remote's default branch HEAD.
	GitRefDefault GitRefKind = "default"
	// GitRefBranch resolves to the HEAD of a named branch.
	GitRefBranch GitRefKind = "branch"
	// GitRefTag resolves to a named tag.
	GitRefTag GitRefKind = "tag"
	// GitRefCommit treats the version as an exact commit SHA.
	GitRefCommit GitRefKind = "commit"
)

// GitResolution is the outcome of resolving a git reference to an immutable
// commit. The SHA is the final, reproducible identity of a git dependency
// (per the P2 master prompt: never trust only `clone --depth 1`).
type GitResolution struct {
	// URL is the fetched repository URL.
	URL string
	// Kind is how the request was interpreted.
	Kind GitRefKind
	// RequestedRef is the original requested reference ("" for default).
	RequestedRef string
	// SHA is the resolved immutable commit SHA (full 40-hex).
	SHA string
	// Subdir is an optional subdirectory inside the repo that contains the
	// package ("" means repo root).
	Subdir string
}

// ErrGit is a concrete package error for git operations carrying E-PKG-GIT-* codes.
type ErrGit struct {
	Code    ErrCode
	URL     string
	Ref     string
	Message string
	Cause   error
}

func (e *ErrGit) Error() string {
	ref := e.Ref
	if ref == "" {
		ref = "(default)"
	}
	if e.Cause != nil {
		return fmt.Sprintf("error[%s]: %s (url=%s ref=%s): %v", e.Code, e.Message, e.URL, ref, e.Cause)
	}
	return fmt.Sprintf("error[%s]: %s (url=%s ref=%s)", e.Code, e.Message, e.URL, ref)
}

func (e *ErrGit) Unwrap() error { return e.Cause }

// runGit executes git with the given args in dir, returning combined output.
// Arguments are passed as a slice (no shell interpolation) for security.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// gitAvailable reports whether the `git` binary can be located.
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// classifyGitRef determines how to interpret the requested version/reference.
// An empty version or "HEAD" means the default branch; a 40-hex string is an
// exact commit; otherwise it is treated as a branch-or-tag ref.
func classifyGitRef(version string) (GitRefKind, string) {
	if version == "" || version == "HEAD" {
		return GitRefDefault, ""
	}
	if len(version) == 40 && isHex(version) {
		return GitRefCommit, version
	}
	// A branch name and tag share the same ref namespace in git; resolve via
	// rev-parse which handles both. We report it as branch for display but the
	// resolution is ref-agnostic and pinned to the resulting commit.
	return GitRefBranch, version
}

func isHex(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// ResolveGitRevision clones a repository (full, not depth-1) into a temp dir and
// resolves the requested reference to an immutable commit SHA. The temp clone is
// returned with the repo checked out at the resolved SHA. Callers must RemoveAll
// the returned cloneDir when done.
//
// Reference interpretation (per P2 master):
//   - "" or "HEAD"          -> default branch
//   - 40-hex                -> exact commit
//   - anything else         -> branch or tag ref (resolved via rev-parse)
//
// The returned SHA is always the full 40-hex commit produced by `git rev-parse HEAD`
// after checking out the requested ref, so it is reproducible and immutable.
func ResolveGitRevision(url, version, subdir string) (*GitResolution, string, error) {
	if !gitAvailable() {
		return nil, "", &ErrGit{Code: ErrGitNotFound, URL: url, Ref: version,
			Message: "git executable not found on PATH; cannot fetch git dependency"}
	}

	kind, requested := classifyGitRef(version)

	tmpDir, err := os.MkdirTemp("", "karkain-git-*")
	if err != nil {
		return nil, "", &ErrGit{Code: ErrGitClone, URL: url, Ref: version,
			Message: "cannot create temp dir for clone", Cause: err}
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	// Full clone (not --depth 1) so we can resolve arbitrary refs deterministically.
	if out, err := runGit("", "clone", "--quiet", url, tmpDir); err != nil {
		cleanup()
		return nil, "", &ErrGit{Code: ErrGitClone, URL: url, Ref: version,
			Message: "failed to clone repository", Cause: fmt.Errorf("%s", strings.TrimSpace(out))}
	}

	// Fetch requested refs to make sure branches/tags that are not in the default
	// clone are present. Reject shallow/filtered caches: ensure we can rev-parse.
	if requested != "" {
		if out, err := runGit(tmpDir, "fetch", "--quiet", "--tags", "origin"); err != nil {
			// A tag pointing at the same SHA as a local ref can be fine; only fail
			// if the subsequent rev-parse cannot find the ref at all.
			_ = out
		}
	}

	// Determine the branch to check out.
	checkoutRef := requested
	if kind == GitRefDefault {
		// Determine the default branch.
		sym, err := runGit(tmpDir, "symbolic-ref", "refs/remotes/origin/HEAD")
		if err != nil {
			cleanup()
			return nil, "", &ErrGit{Code: ErrGitRevision, URL: url, Ref: version,
				Message: "cannot determine default branch", Cause: fmt.Errorf("%s", strings.TrimSpace(sym))}
		}
		// sym looks like refs/remotes/origin/<branch>
		checkoutRef = strings.TrimPrefix(strings.TrimSpace(sym), "refs/remotes/origin/")
	}

	// Resolve the chosen ref to an immutable commit SHA.
	revOut, err := runGit(tmpDir, "rev-parse", checkoutRef+"^{commit}")
	if err != nil {
		cleanup()
		return nil, "", &ErrGit{Code: ErrGitRevision, URL: url, Ref: version,
			Message: fmt.Sprintf("cannot resolve reference %q to a commit", checkoutRef),
			Cause:   fmt.Errorf("%s", strings.TrimSpace(revOut))}
	}
	sha := strings.TrimSpace(revOut)
	if len(sha) != 40 || !isHex(sha) {
		cleanup()
		return nil, "", &ErrGit{Code: ErrGitRevision, URL: url, Ref: version,
			Message: fmt.Sprintf("rev-parse did not yield a 40-hex commit for %q (got %q)", checkoutRef, sha)}
	}

	// Check out the resolved SHA so the working tree matches the pinned commit.
	if out, err := runGit(tmpDir, "checkout", "--quiet", sha); err != nil {
		cleanup()
		return nil, "", &ErrGit{Code: ErrGitRevision, URL: url, Ref: version,
			Message: "failed to check out resolved commit", Cause: fmt.Errorf("%s", strings.TrimSpace(out))}
	}

	res := &GitResolution{
		URL:          url,
		Kind:         kind,
		RequestedRef: requested,
		SHA:          sha,
		Subdir:       subdir,
	}

	// If a subdir is requested and non-empty, descend into it. The clone dir then
	// points at the package root within the resolved commit.
	if subdir != "" {
		pkgRoot := filepath.Join(tmpDir, filepath.FromSlash(subdir))
		clean := filepath.Clean(pkgRoot)
		repoRoot := filepath.Clean(tmpDir)
		if !withinDir(clean, repoRoot) {
			cleanup()
			return nil, "", &ErrGit{Code: ErrGitSubdir, URL: url, Ref: version,
				Message: fmt.Sprintf("subdir %q escapes repository root (path traversal)", subdir)}
		}
		if st, err := os.Stat(clean); err != nil || !st.IsDir() {
			cleanup()
			return nil, "", &ErrGit{Code: ErrGitSubdir, URL: url, Ref: version,
				Message: fmt.Sprintf("package subdir %q does not exist in repository", subdir)}
		}
	}

	return res, tmpDir, nil
}

// withinDir reports whether path is equal to or inside base (both cleaned).
func withinDir(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

// fetchGit clones a git dependency into destDir at the resolved committed SHA.
// On macOS the version string is interpreted as described in classifyGitRef. If
// version is empty, the default branch HEAD is resolved.
func fetchGit(url, version, destDir string) error {
	return fetchGitAt(url, version, "", destDir)
}

// fetchGitAt performs the full reproducible fetch and copies the package files
// (at the resolved commit, optionally from a subdir) into destDir.
func fetchGitAt(url, version, subdir, destDir string) error {
	res, cloneDir, err := ResolveGitRevision(url, version, subdir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(cloneDir)

	srcDir := cloneDir
	if res.Subdir != "" {
		srcDir = filepath.Join(cloneDir, filepath.FromSlash(res.Subdir))
	}

	if err := copyDir(srcDir, destDir); err != nil {
		return &ErrGit{Code: ErrGitClone, URL: url, Ref: res.SHA,
			Message: "failed to copy resolved package into cache", Cause: err}
	}
	return nil
}

// cloneAtRev clones a repository and checks out the given already-resolved
// immutable commit, then copies the package files into destDir. Unlike
// fetchGitAt it does NOT re-resolve the reference (assumes rev is a full 40-hex
// SHA). This is the cache-first / lock-pinned path (P2.9): no ref resolution,
// just materialize the pinned commit.
func cloneAtRev(url, rev, subdir, destDir string) error {
	if !gitAvailable() {
		return &ErrGit{Code: ErrGitNotFound, URL: url, Ref: rev,
			Message: "git executable not found on PATH; cannot fetch git dependency"}
	}
	tmpDir, err := os.MkdirTemp("", "karkain-git-*")
	if err != nil {
		return &ErrGit{Code: ErrGitClone, URL: url, Ref: rev,
			Message: "cannot create temp dir for clone", Cause: err}
	}
	defer os.RemoveAll(tmpDir)

	if out, err := runGit("", "clone", "--quiet", url, tmpDir); err != nil {
		return &ErrGit{Code: ErrGitClone, URL: url, Ref: rev,
			Message: "failed to clone repository", Cause: fmt.Errorf("%s", strings.TrimSpace(out))}
	}
	if out, err := runGit(tmpDir, "checkout", "--quiet", rev); err != nil {
		// The pinned commit may not be reachable from the default clone head;
		// fetch it explicitly before retrying.
		runGit(tmpDir, "fetch", "--quiet", "origin", rev)
		if out2, err2 := runGit(tmpDir, "checkout", "--quiet", rev); err2 != nil {
			return &ErrGit{Code: ErrGitRevision, URL: url, Ref: rev,
				Message: "cannot check out pinned commit", Cause: fmt.Errorf("%s", strings.TrimSpace(out+out2))}
		}
		_ = out
	}

	srcDir := tmpDir
	if subdir != "" {
		srcDir = filepath.Join(tmpDir, filepath.FromSlash(subdir))
		clean := filepath.Clean(srcDir)
		if !withinDir(clean, filepath.Clean(tmpDir)) {
			return &ErrGit{Code: ErrGitSubdir, URL: url, Ref: rev,
				Message: fmt.Sprintf("subdir %q escapes repository root (path traversal)", subdir)}
		}
		if st, e := os.Stat(clean); e != nil || !st.IsDir() {
			return &ErrGit{Code: ErrGitSubdir, URL: url, Ref: rev,
				Message: fmt.Sprintf("package subdir %q does not exist in repository", subdir)}
		}
	}

	if err := copyDir(srcDir, destDir); err != nil {
		return &ErrGit{Code: ErrGitClone, URL: url, Ref: rev,
			Message: "failed to copy pinned package into cache", Cause: err}
	}
	return nil
}

// revFile is the name of the file recording a resolved immutable git commit for
// a cached package.
const revFile = ".rev"

// WriteRevFile writes the resolved immutable commit SHA into a cached package
// directory so the exact pinned revision is available without re-resolving.
func WriteRevFile(dir, sha string) error {
	return os.WriteFile(filepath.Join(dir, revFile), []byte(sha+"\n"), 0644)
}

// ReadRevFile reads a resolved immutable commit SHA previously written to a
// cached package directory, or returns "" if absent.
func ReadRevFile(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, revFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
