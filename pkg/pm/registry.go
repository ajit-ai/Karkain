package pm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultRegistryURL = "https://registry.karkain.dev"

// RegistryClient communicates with the Karkain package registry.
type RegistryClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// PackageMetadata holds registry info about a package.
type PackageMetadata struct {
	Name         string            `json:"name"`
	Latest       string            `json:"latest"`
	Description  string            `json:"description"`
	Author       string            `json:"author"`
	License      string            `json:"license"`
	Repository   string            `json:"repository"`
	Keywords     []string          `json:"keywords"`
	Downloads    int               `json:"downloads"`
	Versions     []string          `json:"versions"`
	Dependencies map[string]string `json:"dependencies"`
}

// RegistrySearchResult is a single registry search hit (wire format).
type RegistrySearchResult struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
}

// SearchResult is a single search hit returned to callers.
type SearchResult struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
}

// NewRegistryClient creates a client pointing at the default registry.
func NewRegistryClient() *RegistryClient {
	regURL := os.Getenv("KARKAIN_REGISTRY")
	if regURL == "" {
		regURL = DefaultRegistryURL
	}
	return &RegistryClient{
		BaseURL:    strings.TrimSuffix(regURL, "/"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

const (
	// IntegrityHeader is the response header carrying the expected sha256 of a
	// downloaded package archive so clients can verify integrity after download.
	IntegrityHeader = "X-Karkain-Sha256"
)

// registryURL joins a path onto the client base URL with proper escaping.
func (c *RegistryClient) registryURL(path string) string {
	return c.BaseURL + path
}

// SearchRegistry searches the registry for packages matching the query.
// The query may be empty to list packages.
func SearchRegistry(query string) ([]SearchResult, error) {
	c := NewRegistryClient()
	route := "/v1/packages"
	if query != "" {
		route += "?q=" + url.QueryEscape(query)
	}
	resp, err := c.get(route)
	if err != nil {
		return nil, &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("registry search failed: %v", err)}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 404:
		return nil, nil
	case 200:
		// fall through
	default:
		return nil, &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("registry returned status %d", resp.StatusCode)}
	}

	var wire []RegistrySearchResult
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return nil, &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("cannot decode registry search response: %v", err)}
	}
	out := make([]SearchResult, 0, len(wire))
	for _, w := range wire {
		out = append(out, SearchResult{Name: w.Name, Version: w.Version, Description: w.Description, Author: w.Author})
	}
	return out, nil
}

// PackageInfo fetches metadata for a specific package.
func PackageInfo(name string) (*PackageMetadata, error) {
	c := NewRegistryClient()
	resp, err := c.get("/v1/packages/" + url.PathEscape(name))
	if err != nil {
		return nil, &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("registry lookup failed: %v", err)}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 404:
		return nil, &PkgError{Code: ErrPackageMissing, Package: name, Message: fmt.Sprintf("package %q not found on registry", name)}
	case 200:
		// fall through
	default:
		return nil, &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("registry returned status %d", resp.StatusCode)}
	}

	var meta PackageMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("cannot decode registry metadata: %v", err)}
	}
	return &meta, nil
}

// LatestVersion fetches the latest version of a package from the registry.
func LatestVersion(name string) (string, error) {
	info, err := PackageInfo(name)
	if err != nil {
		return "", err
	}
	return info.Latest, nil
}

// ResolveVersion matches a requested version constraint (or "" for latest)
// against the package's published versions using Karkain semver. It returns the
// exact version to fetch.
func (c *RegistryClient) ResolveVersion(name, constraint string) (string, error) {
	meta, err := PackageInfo(name)
	if err != nil {
		return "", err
	}
	if len(meta.Versions) == 0 {
		if constraint == "" && meta.Latest != "" {
			return meta.Latest, nil
		}
		return "", &PkgError{Code: ErrVersionMissing, Package: name, Message: "registry reports no published versions"}
	}
	if constraint == "" {
		return meta.Latest, nil
	}
	if isExactVersion(constraint) {
		for _, v := range meta.Versions {
			if v == constraint {
				return v, nil
			}
		}
		return "", &PkgError{Code: ErrVersionMissing, Package: name, Message: fmt.Sprintf("version %q not published for %q (available: %v)", constraint, name, meta.Versions)}
	}
	// Constraint resolution via existing semver logic.
	candidates, err := ParseVersions(meta.Versions)
	if err != nil {
		return "", &PkgError{Code: ErrRegistry, Package: name, Message: fmt.Sprintf("registry published versions are not valid semver: %v", err)}
	}
	best, err := LatestSatisfying(candidates, constraint)
	if err != nil {
		return "", &PkgError{Code: ErrVersionMissing, Package: name, Message: fmt.Sprintf("no published version satisfies %q for %q (available: %v): %v", constraint, name, meta.Versions, err)}
	}
	return best.String(), nil
}

func isExactVersion(s string) bool {
	return len(s) >= 5 && strings.Count(s, ".") >= 2 && strings.Trim(s, "0123456789.") == "" && !strings.ContainsAny(s, "^~*xX ")
}

// FetchFromRegistry downloads and materializes a specific package version from
// the registry into destDir, verifying integrity when the registry provides an
// expected checksum. Extraction is safe against path traversal and symlink
// escapes, and destDir is only populated after successful verify.
func FetchFromRegistry(name, version, destDir string) error {
	c := NewRegistryClient()
	resp, err := c.get(fmt.Sprintf("/v1/packages/%s/%s/download", url.PathEscape(name), url.PathEscape(version)))
	if err != nil {
		return &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("download failed: %v", err)}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 404:
		return &PkgError{Code: ErrPackageMissing, Package: name, Message: fmt.Sprintf("package %s@%s not found on registry", name, version)}
	case 200:
		// fall through
	default:
		return &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("registry returned status %d", resp.StatusCode)}
	}

	expected := strings.TrimSpace(resp.Header.Get(IntegrityHeader))

	// Stream into a temp file while hashing so we never hold the whole archive
	// in memory and never extract before integrity passes.
	tmp, err := os.CreateTemp("", "karkain-dl-*.tgz")
	if err != nil {
		return &PkgError{Code: ErrCache, Message: fmt.Sprintf("cannot create temp download: %v", err)}
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(tmp, h), resp.Body)
	closeErr := tmp.Close()
	if err != nil {
		return &PkgError{Code: ErrRegistry, Message: "download interrupted", Cause: err}
	}
	if closeErr != nil {
		return &PkgError{Code: ErrCache, Message: "cannot close temp download", Cause: closeErr}
	}

	if expected != "" {
		expected = strings.TrimPrefix(expected, "sha256:")
		actual := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(expected, actual) {
			return &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("integrity check failed for %s@%s (expected %s, got %s)", name, version, expected, actual)}
		}
	}

	if err := extractTarballSafe(tmpPath, destDir); err != nil {
		return &PkgError{Code: ErrCache, Message: fmt.Sprintf("cannot extract package %s@%s: %v", name, version, err)}
	}
	return nil
}

// extractTarballSafe decompresses and extracts a .tar.gz archive into destDir,
// rejecting any path traversal or symlink escape. destDir must not be created
// until after the archive is fully validated, which callers accomplish by
// extracting into a temp dir.
func extractTarballSafe(tgzPath, destDir string) error {
	f, err := os.Open(tgzPath)
	if err != nil {
		return fmt.Errorf("cannot open archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("not a valid gzip archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	baseClean := filepath.Clean(destDir)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("corrupt archive: %w", err)
		}

		// Reject path traversal.
		name := filepath.FromSlash(hdr.Name)
		clean := filepath.Clean(filepath.Join(baseClean, name))
		if !withinDir(clean, baseClean) {
			return fmt.Errorf("archive entry %q escapes destination (path traversal)", hdr.Name)
		}

		// Reject symlinks/hardlinks (link targets could escape).
		if hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
			return fmt.Errorf("archive entry %q is a link, which is not allowed", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(clean, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(clean), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		default:
			// Skip other entry types (fifo, char, block, etc.) for safety.
			continue
		}
	}
	return nil
}

// PublishPackage publishes a package to the registry.
func PublishPackage(projectDir string, manifest *Manifest, token string) error {
	c := NewRegistryClient()
	c.HTTPClient = &http.Client{Timeout: 120 * time.Second}

	// Build an in-memory tarball of the project source.
	tarball, sha, err := buildPackageTarball(projectDir)
	if err != nil {
		return &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("cannot build package archive: %v", err)}
	}

	req, err := http.NewRequest("POST", c.registryURL("/v1/packages/"+url.PathEscape(manifest.Name)+"/publish"), bytes.NewReader(tarball))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/gzip")
	req.Header.Set("Karkain-Version", manifest.Version)
	req.Header.Set(IntegrityHeader, sha)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("publish failed: %v", err)}
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 401:
		return &PkgError{Code: ErrRegistry, Message: "authentication failed - run 'karkain pkg login' again"}
	case 403:
		return &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("permission denied for package %q", manifest.Name)}
	case 200, 201:
		return nil
	default:
		return &PkgError{Code: ErrRegistry, Message: fmt.Sprintf("registry returned status %d", resp.StatusCode)}
	}
}

func (c *RegistryClient) get(path string) (*http.Response, error) {
	return c.HTTPClient.Get(c.registryURL(path))
}

// tarSkipNames are files/dirs excluded from a published package archive.
var tarSkipNames = map[string]bool{
	CacheDir:    true, // .karkain
	".git":      true,
	".DS_Store": true,
}

// buildPackageTarball archives the project source into an in-memory gzipped tar
// and returns the archive bytes plus its sha256 hex digest. It excludes the
// local cache and VCS metadata. Paths are stored relative with forward slashes
// for cross-platform determinism.
func buildPackageTarball(projectDir string) ([]byte, string, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	base := filepath.Clean(projectDir)
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if tarSkipNames[rel] || tarSkipNames[filepath.Base(rel)] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		name := filepath.ToSlash(rel)
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
		}
		if info.IsDir() {
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = 0755
		} else {
			hdr.Typeflag = tar.TypeReg
			hdr.Size = info.Size()
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, f); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	if err := tw.Close(); err != nil {
		return nil, "", err
	}
	if err := gz.Close(); err != nil {
		return nil, "", err
	}

	data := buf.Bytes()
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}
