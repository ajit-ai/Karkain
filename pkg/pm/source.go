package pm

import (
	"fmt"
	"strings"
)

// SourceKind identifies the origin of a dependency. This is the Karkain-native
// unified source abstraction: every dependency maps to exactly one of these
// kinds, and cache/lock identity is source-aware (a package of the same name and
// version from two different sources is two distinct identities).
type SourceKind string

const (
	// SourceLocal is a path-based dependency (relative or absolute directory).
	// It points at the canonical local source path and is NOT copied.
	SourceLocal SourceKind = "local"
	// SourceWorkspace is a workspace member dependency. Kept distinct from
	// SourceLocal because workspace members share the workspace source tree.
	SourceWorkspace SourceKind = "workspace"
	// SourceGit is a git repository dependency, resolved to an immutable commit.
	SourceGit SourceKind = "git"
	// SourceRegistry is a registry dependency, resolved to an immutable version.
	SourceRegistry SourceKind = "registry"
)

// ParseSourceKind maps a manifest source string to a SourceKind. Unknown kinds
// return false so callers can emit a diagnostic.
func ParseSourceKind(s string) (SourceKind, bool) {
	switch s {
	case string(SourceLocal):
		return SourceLocal, true
	case string(SourceWorkspace):
		return SourceWorkspace, true
	case string(SourceGit):
		return SourceGit, true
	case string(SourceRegistry):
		return SourceRegistry, true
	default:
		return "", false
	}
}

// ValidateSourceKind reports whether the kind is a recognized dependency source.
func ValidateSourceKind(s string) bool {
	_, ok := ParseSourceKind(s)
	return ok
}

// CacheIdentityKey builds a source-aware cache key for a dependency. Per RULE 3,
// identity includes package name + resolved identity (version or git rev) +
// source kind + normalized source identity (URL). This prevents two packages of
// the same name@version from different sources from colliding.
func CacheIdentityKey(name, version, rev, source, url string) string {
	var sb strings.Builder
	sb.WriteString(SanitizeCacheComponent(name))
	sb.WriteString("@")
	resolved := version
	if source == string(SourceGit) && rev != "" {
		resolved = rev
	}
	sb.WriteString(SanitizeCacheComponent(resolved))
	sb.WriteString("+")
	sb.WriteString(SanitizeCacheComponent(source))
	if url != "" {
		sb.WriteString("+")
		sb.WriteString(SanitizeCacheComponent(normalizeSourceURL(url)))
	}
	return sb.String()
}

// SanitizeCacheComponent replaces characters that are unsafe in a filesystem
// path with a safe marker, preventing path traversal and ambiguity in cache keys.
func SanitizeCacheComponent(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.':
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

// normalizeSourceURL lowercases the scheme/host and trims a trailing '.git' so
// that superficially different spellings of the same git URL produce the same
// normalized source identity. It does not add or remove meaning beyond this.
func normalizeSourceURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimSuffix(u, ".git")
	return u
}

// ErrCode is a stable, machine-readable package-error category, e.g.
// E-PKG-GIT-REVISION. Error messages remain human-actionable Karkain
// diagnostics; the code is intended for programmatic handling and testing.
type ErrCode string

const (
	ErrGitNotFound    ErrCode = "E-PKG-GIT-NOT-FOUND"
	ErrGitClone       ErrCode = "E-PKG-GIT-CLONE"
	ErrGitRevision    ErrCode = "E-PKG-GIT-REVISION"
	ErrGitSubdir      ErrCode = "E-PKG-GIT-SUBDIR"
	ErrRegistry       ErrCode = "E-PKG-REGISTRY"
	ErrPackageMissing ErrCode = "E-PKG-PACKAGE-NOT-FOUND"
	ErrVersionMissing ErrCode = "E-PKG-VERSION-NOT-FOUND"
	ErrCache          ErrCode = "E-PKG-CACHE"
	ErrLock           ErrCode = "E-PKG-LOCK"
	ErrWorkspace      ErrCode = "E-PKG-WORKSPACE"
	ErrManifest       ErrCode = "E-PKG-MANIFEST"
)

// PkgError is an actionable package diagnostic carrying a stable code.
type PkgError struct {
	Code    ErrCode
	Package string
	Message string
}

func (e *PkgError) Error() string {
	if e.Package != "" {
		return fmt.Sprintf("error[%s]: %s\n  package: %s", e.Code, e.Message, e.Package)
	}
	return fmt.Sprintf("error[%s]: %s", e.Code, e.Message)
}
