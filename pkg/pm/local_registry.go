package pm

// Phase 135 — Local registry MVP (local filesystem only).
//
// Phase 135 is local-only. No network registry exists yet.
//
// Layout (deterministic, human-inspectable, no database, no daemon):
//
//   <root>/registry.json                          marker (schema + kind)
//   <root>/packages/<name>/<version>/manifest    copy of karkain.toml at publish
//   <root>/packages/<name>/<version>/source/     published source tree
//   <root>/packages/<name>/<version>/sha256      tree digest (tamper evidence)
//   <root>/index/<name>                          JSON {name, versions[], latest}
//
// `registry init` creates the layout, `publish` appends immutable versions,
// resolution (`add`/`fetch`/`update`) verifies the digest and extracts through
// the normal manifest/lockfile/cache flow with the existing checksum record.
// Published versions are immutable: republishing name@version is refused.
// The registry root is selected by `--registry <dir>` (flag wins) or the
// KARKAIN_REGISTRY environment variable; both must name a local directory.
// There is no network lookup and no silent remote fallback.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LocalRegistryMarker identifies a directory-origin registry root.
const LocalRegistryMarker = "registry.json"

// RegistryKind identifies how a registry reference resolves.
type RegistryKind string

const (
	// RegistryLocal is a directory-origin registry (local path on disk).
	RegistryLocal RegistryKind = "local"
	// RegistryRemote is an http(s) registry speaking the v1 protocol.
	RegistryRemote RegistryKind = "remote"
)

// LocalRegistrySchema is the on-disk layout version.
const LocalRegistrySchema = 1

// LocalIndex is the persisted per-package lookup entry (<root>/index/<name>).
type LocalIndex struct {
	Name     string   `json:"name"`
	Versions []string `json:"versions"` // sorted ascending (semver-aware)
	Latest   string   `json:"latest"`
}

// ValidatePackageName rejects identities that are unsafe as registry paths:
// empty names, absolute paths, separators, traversal, dot-names, and names
// outside [A-Za-z0-9._-]. A package must never escape the registry root
// through a crafted name.
func ValidatePackageName(name string) error {
	if name == "" {
		return &PkgError{Code: ErrRegistry, Message: "invalid package name (empty)"}
	}
	if name == "." || name == ".." {
		return &PkgError{Code: ErrRegistry, Package: name, Message: "invalid package name (reserved)"}
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return &PkgError{Code: ErrRegistry, Package: name,
			Message: fmt.Sprintf("invalid package name %q (path separators and traversal are rejected)", name)}
	}
	if filepath.IsAbs(name) {
		return &PkgError{Code: ErrRegistry, Package: name, Message: fmt.Sprintf("invalid package name %q (absolute paths are rejected)", name)}
	}
	hasAlnum := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9':
			hasAlnum = true
		case r == '-' || r == '_' || r == '.':
			// allowed punctuation
		default:
			return &PkgError{Code: ErrRegistry, Package: name,
				Message: fmt.Sprintf("invalid package name %q (only letters, digits, '-', '_' and '.' are allowed)", name)}
		}
	}
	if !hasAlnum {
		return &PkgError{Code: ErrRegistry, Package: name, Message: fmt.Sprintf("invalid package name %q (must contain a letter or digit)", name)}
	}
	return nil
}

// ValidateVersion rejects empty or non-semver versions using the existing
// version representation (no second version type).
func ValidateVersion(version string) error {
	if strings.TrimSpace(version) == "" {
		return &PkgError{Code: ErrManifest, Message: "invalid package version (empty)"}
	}
	if _, err := ParseVersion(version); err != nil {
		return &PkgError{Code: ErrManifest, Message: fmt.Sprintf("invalid package version %q (must be semver): %v", version, err)}
	}
	return nil
}

// joinUnder joins elems onto root and refuses any result escaping root
// (defense in depth behind ValidatePackageName).
func joinUnder(root string, elems ...string) (string, error) {
	p := filepath.Join(append([]string{root}, elems...)...)
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return "", &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("invalid registry path: %v", err)}
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", &PkgError{Code: ErrRegistry, Message: "registry path escapes the registry root (rejected)"}
	}
	return p, nil
}

// IsLocalRegistryDir reports whether dir carries the registry marker.
func IsLocalRegistryDir(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, LocalRegistryMarker))
	return err == nil && fi.Mode().IsRegular()
}

// RequireLocalRegistry validates a registry path: it must exist, be a
// directory, and carry the marker (created by `registry init`).
func RequireLocalRegistry(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", &PkgError{Code: ErrRegistry,
			Message: "no registry configured (pass --registry <dir> or set KARKAIN_REGISTRY to a registry directory)"}
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("invalid registry path %q: %v", dir, err)}
	}
	fi, err := os.Stat(abs)
	if err != nil || !fi.IsDir() {
		return "", &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("registry unavailable (not a directory): %s", abs)}
	}
	if !IsLocalRegistryDir(abs) {
		return "", &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("not a local registry (no %s): %s (run 'karkain pkg registry init <dir>' first)", LocalRegistryMarker, abs)}
	}
	return abs, nil
}

// RegistryRef resolves the effective registry reference: an explicit flag
// value wins, else the KARKAIN_REGISTRY environment. The result must name a
// local registry directory — http(s) references are rejected with an honest
// local-only diagnostic (Phase 135 performs no network lookup).
func RegistryRef(flagVal string) (string, error) {
	ref := strings.TrimSpace(flagVal)
	if ref == "" {
		ref = strings.TrimSpace(os.Getenv("KARKAIN_REGISTRY"))
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return "", &PkgError{Code: ErrRegistry,
			Message: "network registries are not supported (Phase 135 is local-only; use a registry directory)"}
	}
	return RequireLocalRegistry(ref)
}

// RegistryRefForDep returns the effective registry for a dependency: its
// explicit URL when set, else the flag/env reference. An http(s) URL keeps
// the pre-135 remote-client behavior (explicit user choice, existing tests);
// otherwise the reference must be a local registry directory.
func RegistryRefForDep(flagVal string, dep Dependency) (RegistryKind, string, error) {
	if strings.TrimSpace(dep.URL) != "" {
		if u := strings.TrimSpace(dep.URL); strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			return RegistryRemote, strings.TrimSuffix(u, "/"), nil
		}
		target, err := RequireLocalRegistry(dep.URL)
		return RegistryLocal, target, err
	}
	ref := strings.TrimSpace(flagVal)
	if ref == "" {
		ref = strings.TrimSpace(os.Getenv("KARKAIN_REGISTRY"))
	}
	if ref == "" {
		return "", "", &PkgError{Code: ErrRegistry,
			Message: "no registry configured for registry dependency (pass --registry <dir>, set the dependency url, or set KARKAIN_REGISTRY to a registry directory)"}
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return RegistryRemote, strings.TrimSuffix(ref, "/"), nil
	}
	target, err := RequireLocalRegistry(ref)
	return RegistryLocal, target, err
}

// InitLocalRegistry creates the registry layout. Re-init over an existing
// registry is refused, never merged.
func InitLocalRegistry(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("invalid registry path %q: %v", dir, err)}
	}
	if IsLocalRegistryDir(abs) {
		return &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("registry already initialized at %s", abs)}
	}
	for _, sub := range []string{"packages", "index"} {
		if err := os.MkdirAll(filepath.Join(abs, sub), 0755); err != nil {
			return &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("cannot create registry directory: %v", err)}
		}
	}
	marker := map[string]any{"schema": LocalRegistrySchema, "kind": "karkain-local-registry"}
	raw, _ := json.MarshalIndent(marker, "", "  ")
	if err := os.WriteFile(filepath.Join(abs, LocalRegistryMarker), append(raw, '\n'), 0644); err != nil {
		return &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("cannot write registry marker: %v", err)}
	}
	return nil
}

// versionDir returns the storage directory for name@version (validated).
func versionDir(regDir, name, version string) (string, error) {
	if err := ValidatePackageName(name); err != nil {
		return "", err
	}
	return joinUnder(regDir, "packages", name, version)
}

// indexPath returns the lookup file for a package (validated).
func indexPath(regDir, name string) (string, error) {
	if err := ValidatePackageName(name); err != nil {
		return "", err
	}
	return joinUnder(regDir, "index", name)
}

// ReadLocalIndex loads the lookup entry for a package (nil when unpublished).
// Corrupt JSON is a hard error, never an empty guess.
func ReadLocalIndex(regDir, name string) (*LocalIndex, error) {
	p, err := indexPath(regDir, name)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, &PkgError{Code: ErrRegistry, Package: name, Message: fmt.Sprintf("cannot read registry index: %v", err)}
	}
	var idx LocalIndex
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, &PkgError{Code: ErrRegistry, Package: name, Message: fmt.Sprintf("corrupt registry metadata (index/%s): %v", name, err)}
	}
	return &idx, nil
}

// writeLocalIndex persists the lookup entry atomically (temp + rename).
func writeLocalIndex(regDir string, idx *LocalIndex) error {
	p, err := indexPath(regDir, idx.Name)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0644); err != nil {
		return &PkgError{Code: ErrRegistry, Package: idx.Name, Message: fmt.Sprintf("cannot write registry index: %v", err)}
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return &PkgError{Code: ErrRegistry, Package: idx.Name, Message: fmt.Sprintf("cannot publish registry index: %v", err)}
	}
	return nil
}

// sortLocalVersions orders versions ascending: semver-aware when every entry
// parses, plain string order otherwise (deterministic either way).
func sortLocalVersions(versions []string) {
	if parsed, err := ParseVersions(versions); err == nil {
		sort.Slice(parsed, func(i, j int) bool { return Compare(parsed[i], parsed[j]) < 0 })
		for i, v := range parsed {
			versions[i] = v.String()
		}
		return
	}
	sort.Strings(versions)
}

// publishSkipNames are files/dirs excluded from a published source tree
// (same convention as the package tarball builder).
var publishSkipNames = map[string]bool{
	CacheDir:    true, // .karkain
	".git":      true,
	".DS_Store": true,
}

// copySourceTree copies src into dst (created), sorted for determinism,
// skipping cache/VCS metadata. dst must not exist.
func copySourceTree(src, dst string) error {
	var files []string
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		base := filepath.Base(rel)
		if publishSkipNames[rel] || publishSkipNames[base] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	for _, rel := range files {
		s, serr := os.Stat(filepath.Join(src, rel))
		if serr != nil {
			return serr
		}
		d := filepath.Join(dst, rel)
		if s.IsDir() {
			if err := os.MkdirAll(d, 0755); err != nil {
				return err
			}
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(src, rel))
		if rerr != nil {
			return rerr
		}
		if err := os.MkdirAll(filepath.Dir(d), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(d, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// digestSourceTree hashes the stored tree deterministically:
// sorted forward-slash relative paths + sizes + bytes.
func digestSourceTree(dir string) (string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	h := sha256.New()
	for _, rel := range files {
		data, rerr := os.ReadFile(filepath.Join(dir, rel))
		if rerr != nil {
			return "", rerr
		}
		fmt.Fprintf(h, "%s\x00%d\x00", filepath.ToSlash(rel), len(data))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// PublishLocal publishes the project at projectDir into the local registry.
// It validates identity, version and source boundaries, stores the manifest
// copy + source tree + digest, and records the version in the index.
// Republishing an existing name@version is refused (immutable).
func PublishLocal(regDir, projectDir string, manifest *Manifest) error {
	reg, err := RequireLocalRegistry(regDir)
	if err != nil {
		return err
	}
	if err := ValidatePackageName(manifest.Name); err != nil {
		return err
	}
	if err := ValidateVersion(manifest.Version); err != nil {
		return err
	}
	vdir, err := versionDir(reg, manifest.Name, manifest.Version)
	if err != nil {
		return err
	}
	if _, err := os.Stat(vdir); err == nil {
		return &PkgError{Code: ErrRegistry, Package: manifest.Name,
			Message: fmt.Sprintf("version %q of package %q is already published (published versions are immutable; bump the version to publish again)", manifest.Version, manifest.Name)}
	}
	// Stage the source tree in a temp dir first so a failed publish never
	// leaves a half-written version directory behind.
	stage, err := os.MkdirTemp(filepath.Join(reg, "packages"), ".stage-*")
	if err != nil {
		return &PkgError{Code: ErrRegistry, Package: manifest.Name, Message: fmt.Sprintf("cannot stage publish: %v", err)}
	}
	defer os.RemoveAll(stage)
	stagedSrc := filepath.Join(stage, "source")
	if err := copySourceTree(projectDir, stagedSrc); err != nil {
		return &PkgError{Code: ErrRegistry, Package: manifest.Name, Message: fmt.Sprintf("cannot stage package sources: %v", err)}
	}
	manifestBytes, err := os.ReadFile(filepath.Join(projectDir, ManifestFile))
	if err != nil {
		return &PkgError{Code: ErrManifest, Package: manifest.Name, Message: fmt.Sprintf("cannot read package manifest: %v", err)}
	}
	if err := os.WriteFile(filepath.Join(stage, "manifest"), manifestBytes, 0644); err != nil {
		return &PkgError{Code: ErrRegistry, Package: manifest.Name, Message: fmt.Sprintf("cannot stage package manifest: %v", err)}
	}
	digest, err := digestSourceTree(stagedSrc)
	if err != nil {
		return &PkgError{Code: ErrRegistry, Package: manifest.Name, Message: fmt.Sprintf("cannot digest package sources: %v", err)}
	}
	if err := os.WriteFile(filepath.Join(stage, "sha256"), []byte(digest+"\n"), 0644); err != nil {
		return &PkgError{Code: ErrRegistry, Package: manifest.Name, Message: fmt.Sprintf("cannot stage package digest: %v", err)}
	}
	if err := os.MkdirAll(filepath.Dir(vdir), 0755); err != nil {
		return &PkgError{Code: ErrRegistry, Package: manifest.Name, Message: fmt.Sprintf("cannot create package directory: %v", err)}
	}
	if err := os.Rename(stage, vdir); err != nil {
		return &PkgError{Code: ErrRegistry, Package: manifest.Name, Message: fmt.Sprintf("cannot store published package: %v", err)}
	}
	idx, err := ReadLocalIndex(reg, manifest.Name)
	if err != nil {
		return err
	}
	if idx == nil {
		idx = &LocalIndex{Name: manifest.Name}
	}
	idx.Versions = append(idx.Versions, manifest.Version)
	sortLocalVersions(idx.Versions)
	idx.Latest = idx.Versions[len(idx.Versions)-1]
	return writeLocalIndex(reg, idx)
}

// ResolveLocalVersion matches a constraint ("" for latest) against the
// published versions in the index, returning the exact version. Exact
// versions match directly; otherwise the existing semver constraint
// machinery decides (no new solver).
func ResolveLocalVersion(regDir, name, constraint string) (string, error) {
	reg, err := RequireLocalRegistry(regDir)
	if err != nil {
		return "", err
	}
	if err := ValidatePackageName(name); err != nil {
		return "", err
	}
	idx, err := ReadLocalIndex(reg, name)
	if err != nil {
		return "", err
	}
	if idx == nil || len(idx.Versions) == 0 {
		return "", &PkgError{Code: ErrPackageMissing, Package: name, Message: fmt.Sprintf("package %q not found in local registry %s", name, reg)}
	}
	constraint = strings.TrimSpace(constraint)
	if constraint == "" || constraint == "*" {
		return idx.Latest, nil
	}
	for _, v := range idx.Versions {
		if v == constraint {
			return v, nil
		}
	}
	candidates, perr := ParseVersions(idx.Versions)
	if perr != nil {
		return "", &PkgError{Code: ErrVersionMissing, Package: name,
			Message: fmt.Sprintf("version %q of package %q not found (available: %s)", constraint, name, strings.Join(idx.Versions, ", "))}
	}
	best, berr := LatestSatisfying(candidates, constraint)
	if berr != nil {
		return "", &PkgError{Code: ErrVersionMissing, Package: name,
			Message: fmt.Sprintf("no published version of %q satisfies %q (available: %s)", name, constraint, strings.Join(idx.Versions, ", "))}
	}
	return best.String(), nil
}

// FetchLocal verifies the stored digest and copies the published source tree
// into destDir (created). A digest mismatch or corrupt metadata is a hard
// error — never a silent install.
func FetchLocal(regDir, name, version, destDir string) error {
	reg, err := RequireLocalRegistry(regDir)
	if err != nil {
		return err
	}
	vdir, err := versionDir(reg, name, version)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(vdir); err != nil || !fi.IsDir() {
		idx, _ := ReadLocalIndex(reg, name)
		avail := ""
		if idx != nil && len(idx.Versions) > 0 {
			avail = fmt.Sprintf(" (available: %s)", strings.Join(idx.Versions, ", "))
		}
		return &PkgError{Code: ErrVersionMissing, Package: name,
			Message: fmt.Sprintf("version %q of package %q not found in local registry%s", version, name, avail)}
	}
	want, err := os.ReadFile(filepath.Join(vdir, "sha256"))
	if err != nil {
		return &PkgError{Code: ErrRegistry, Package: name,
			Message: fmt.Sprintf("corrupt registry metadata (missing digest for %s@%s)", name, version)}
	}
	actual, err := digestSourceTree(filepath.Join(vdir, "source"))
	if err != nil {
		return &PkgError{Code: ErrRegistry, Package: name,
			Message: fmt.Sprintf("corrupt registry metadata (cannot read stored sources for %s@%s)", name, version)}
	}
	if strings.TrimSpace(string(want)) != actual {
		return &PkgError{Code: ErrRegistry, Package: name,
			Message: fmt.Sprintf("integrity check failed for %s@%s (stored sources changed after publish)", name, version)}
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &PkgError{Code: ErrCache, Message: fmt.Sprintf("cannot create destination: %v", err)}
	}
	if err := copySourceTree(filepath.Join(vdir, "source"), destDir); err != nil {
		return &PkgError{Code: ErrCache, Message: fmt.Sprintf("cannot extract package %s@%s: %v", name, version, err)}
	}
	return nil
}
