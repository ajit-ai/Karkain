package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Karkain's SBOM is deliberately small and deterministic: the release
// artifacts and their SHA-256 digests, plus the product identity. go.mod has
// no external dependencies, so there is no dependency list to drift. No
// timestamp is recorded because release verification is by digest and a
// timestamp would make an otherwise identical SBOM differ between runs.
type sbomDocument struct {
	Schema          string         `json:"schema"`
	Product         sbomProduct    `json:"product"`
	Generator       string         `json:"generator"`
	Note            string         `json:"note"`
	ExternalModules []string       `json:"external_go_modules"`
	Artifacts       []sbomArtifact `json:"artifacts"`
}

type sbomProduct struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type sbomArtifact struct {
	Name   string `json:"name"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// GenerateSBOM returns a deterministic JSON SBOM for every file in releaseDir.
// The SBOM file itself is excluded when regenerating in place.
func GenerateSBOM(releaseDir, version string) ([]byte, error) {
	entries, err := os.ReadDir(releaseDir)
	if err != nil {
		return nil, err
	}
	doc := sbomDocument{
		Schema:          "karkain-sbom-v1",
		Product:         sbomProduct{Name: "Karkain", Version: version},
		Generator:       "karkain sbom",
		Note:            "Deterministic digest manifest; no timestamp. go.mod declares no external Go modules.",
		ExternalModules: []string{},
		Artifacts:       []sbomArtifact{},
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "sbom.json" {
			continue
		}
		path := filepath.Join(releaseDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		doc.Artifacts = append(doc.Artifacts, sbomArtifact{
			Name:   e.Name(),
			Bytes:  int64(len(data)),
			SHA256: hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(doc.Artifacts, func(i, j int) bool { return doc.Artifacts[i].Name < doc.Artifacts[j].Name })
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// WriteSBOM writes the generated document to path (or stdout when path is "").
func WriteSBOM(releaseDir, version, path string) error {
	data, err := GenerateSBOM(releaseDir, version)
	if err != nil {
		return fmt.Errorf("generate SBOM: %w", err)
	}
	if path == "" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
