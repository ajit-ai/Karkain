package pm

import (
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
	Name        string            `json:"name"`
	Latest      string            `json:"latest"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	License     string            `json:"license"`
	Repository  string            `json:"repository"`
	Keywords    []string          `json:"keywords"`
	Downloads   int               `json:"downloads"`
	Versions    []string          `json:"versions"`
	Dependencies map[string]string `json:"dependencies"`
}

// SearchResult is a single search hit.
type SearchResult struct {
	Name        string
	Version     string
	Description string
	Author      string
}

// NewRegistryClient creates a client pointing at the default registry.
func NewRegistryClient() *RegistryClient {
	regURL := os.Getenv("KARKAIN_REGISTRY")
	if regURL == "" {
		regURL = DefaultRegistryURL
	}
	return &RegistryClient{
		BaseURL:    regURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SearchRegistry searches the registry for packages matching the query.
func SearchRegistry(query string) ([]SearchResult, error) {
	c := NewRegistryClient()
	resp, err := c.get("/packages?q=" + url.QueryEscape(query))
	if err != nil {
		return nil, fmt.Errorf("registry search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, nil
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	_ = body
	// TODO: parse JSON response when registry is implemented
	return nil, fmt.Errorf("registry not yet available at %s", c.BaseURL)
}

// PackageInfo fetches metadata for a specific package.
func PackageInfo(name string) (*PackageMetadata, error) {
	c := NewRegistryClient()
	resp, err := c.get("/packages/" + url.PathEscape(name))
	if err != nil {
		return nil, fmt.Errorf("registry lookup failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("package %q not found on registry", name)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	_ = body
	// TODO: parse JSON response when registry is implemented
	return nil, fmt.Errorf("registry not yet available at %s", c.BaseURL)
}

// LatestVersion fetches the latest version of a package from the registry.
func LatestVersion(name string) (string, error) {
	info, err := PackageInfo(name)
	if err != nil {
		return "", err
	}
	return info.Latest, nil
}

// PublishPackage publishes a package to the registry.
func PublishPackage(projectDir string, manifest *Manifest, token string) error {
	c := NewRegistryClient()
	c.HTTPClient = &http.Client{Timeout: 120 * time.Second}

	_ = projectDir
	_ = manifest

	// TODO: create tarball, compute checksum, upload
	req, err := http.NewRequest("POST", c.BaseURL+"/packages/"+url.PathEscape(manifest.Name)+"/publish", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("authentication failed — run 'karkain pkg login' again")
	}
	if resp.StatusCode == 403 {
		return fmt.Errorf("permission denied for package %q", manifest.Name)
	}
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		return nil
	}
	return fmt.Errorf("registry returned status %d", resp.StatusCode)
}

// FetchFromRegistry downloads a specific package version from the registry.
func FetchFromRegistry(name, version, destDir string) error {
	c := NewRegistryClient()
	dlURL := fmt.Sprintf("%s/packages/%s/%s/download", c.BaseURL, url.PathEscape(name), url.PathEscape(version))

	resp, err := c.get(strings.TrimPrefix(dlURL, c.BaseURL))
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("package %s@%s not found on registry", name, version)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	// Write response body to destDir as extracted package
	outPath := filepath.Join(destDir, name+"@"+version+".tmp")
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return err
	}

	// TODO: extract tarball into destDir when registry is real
	os.Remove(outPath)
	return nil
}

func (c *RegistryClient) get(path string) (*http.Response, error) {
	return c.HTTPClient.Get(c.BaseURL + path)
}
