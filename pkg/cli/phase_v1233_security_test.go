// v1.2.33: SEC-001's two deliverable halves — a deterministic SBOM and a
// real CVE response SLA. Artifact signing (cosign, increment 165) is
// explicitly NOT claimed; the gate asserts the policy says so.
package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var v1233Once sync.Once
var v1233CLI string
var v1233Err error

func v1233Prepare(t *testing.T) string {
	t.Helper()
	v1233Once.Do(func() { v1233Err = buildKarkainErr(t, &v1233CLI) })
	if v1233Err != nil {
		t.Fatalf("building karkain: %v", v1233Err)
	}
	return v1233CLI
}

func TestPhaseV1233_SBOMIsDeterministicAndHashExact(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"karkain-v1.2.0-windows-amd64.zip":  "windows archive",
		"karkain-v1.2.0-linux-amd64.tar.gz": "linux archive",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a, err := GenerateSBOM(dir, "v1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateSBOM(dir, "v1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("SBOM is not deterministic for an unchanged release directory")
	}
	var doc struct {
		Schema  string `json:"schema"`
		Product struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"product"`
		External  []string `json:"external_go_modules"`
		Artifacts []struct {
			Name   string `json:"name"`
			SHA256 string `json:"sha256"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(a, &doc); err != nil {
		t.Fatalf("SBOM is not valid JSON: %v", err)
	}
	if doc.Schema != "karkain-sbom-v1" || doc.Product.Name != "Karkain" || doc.Product.Version != "v1.2.0" {
		t.Errorf("wrong SBOM identity: %+v", doc)
	}
	if len(doc.External) != 0 {
		t.Errorf("go.mod declares no external modules, got %v", doc.External)
	}
	if len(doc.Artifacts) != len(files) {
		t.Fatalf("artifact count = %d, want %d", len(doc.Artifacts), len(files))
	}
	for _, art := range doc.Artifacts {
		body, ok := files[art.Name]
		if !ok {
			t.Errorf("unexpected artifact %q", art.Name)
			continue
		}
		sum := sha256.Sum256([]byte(body))
		if art.SHA256 != hex.EncodeToString(sum[:]) {
			t.Errorf("sha256 for %s = %s, want %s", art.Name, art.SHA256, hex.EncodeToString(sum[:]))
		}
	}
}

func TestPhaseV1233_SBOMCommandMatchesPackage(t *testing.T) {
	cliBin := v1233Prepare(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.bin"), []byte("artifact"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "sbom.json")
	cmd := exec.Command(cliBin, "sbom", "--release-dir", dir, "--version", "v1.2.0", "--output", out)
	if err := cmd.Run(); err != nil {
		t.Fatalf("karkain sbom failed: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want, err := GenerateSBOM(dir, "v1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("CLI SBOM differs from package output:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestPhaseV1233_SecurityPolicyEvidence(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "SECURITY.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"1.2.0 (Stable)",
		"Response SLA",
		"7 calendar days",
		"30 calendar days",
		"Software bill of materials",
		"karkain-sbom-v1",
		"Artifact signing",
		"Not implemented",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("SECURITY.md is missing %q", want)
		}
	}
}
