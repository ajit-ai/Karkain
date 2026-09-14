package cli

// Phase 120 — GA Envelope gate.
//
// This suite guards the Phase 120 GA Envelope invariants: a release pipeline
// that can be triggered manually and that produces the certified 13-archive
// set, a deterministic unit-test job (packages run sequentially, mitigating
// the Phase 107 concurrency flake), honest install documentation matching the
// certified archives, the developer-preview status pointer, the owner
// attribution files, and the unchanged v1.0.0 identity.

import (
	"path/filepath"
	"strings"
	"testing"
)

// certifiedArchives are the exact 13 v1.0.0 artifact names validated against
// the release checksums (docs/audit KARKAIN-1.0.0 certification §16 and the
// releases/ directory).
func certifiedArchives() []string {
	return []string{
		"karkain-v1.0.0-darwin-amd64.tar.gz",
		"karkain-v1.0.0-darwin-arm64.tar.gz",
		"karkain-v1.0.0-freebsd-amd64.tar.gz",
		"karkain-v1.0.0-linux-386.tar.gz",
		"karkain-v1.0.0-linux-amd64.tar.gz",
		"karkain-v1.0.0-linux-arm.tar.gz",
		"karkain-v1.0.0-linux-arm64.tar.gz",
		"karkain-v1.0.0-linux-ppc64le.tar.gz",
		"karkain-v1.0.0-linux-s390x.tar.gz",
		"karkain-v1.0.0-netbsd-amd64.tar.gz",
		"karkain-v1.0.0-openbsd-amd64.tar.gz",
		"karkain-v1.0.0-windows-amd64.zip",
		"karkain-v1.0.0-windows-arm64.zip",
	}
}

// TestPhase120_GaEnvelope assembles the whole Phase 120 gate.
func TestPhase120_GaEnvelope(t *testing.T) {
	bin := buildPreviewBinary(t)
	root := repoRoot(t)

	t.Run("VersionIdentity", func(t *testing.T) {
		out, err := runBin(t, bin, t.TempDir(), "--version")
		if err != nil {
			t.Fatalf("karkain --version failed: %v\n%s", err, out)
		}
		for _, want := range []string{"Karkain Compiler v1.0.0", "Stable Build"} {
			if !strings.Contains(out, want) {
				t.Errorf("--version does not contain %q: %s", want, out)
			}
		}
		if ver := strings.TrimSpace(mustRead(t, filepath.Join(root, "VERSION"))); ver != "1.0.0" {
			t.Errorf("VERSION = %q, want %q", ver, "1.0.0")
		}
	})

	t.Run("CIWorkflow", func(t *testing.T) {
		ci := mustRead(t, filepath.Join(root, ".github", "workflows", "ci.yml"))
		// The release pipeline must be manually triggerable.
		if !strings.Contains(ci, "workflow_dispatch:") {
			t.Error("ci.yml is missing workflow_dispatch (manual release trigger)")
		}
		// Unit tests must run package-sequentially (Phase 107 flake mitigation).
		if !strings.Contains(ci, "go test -p 1 ./pkg/lexer/...") {
			t.Error("ci.yml unit-test step is not package-sequential (-p 1)")
		}
		// Build matrix must cover the certified 13-archive set.
		for _, want := range []string{"goarch: arm", `goarch: "386"`, "- goos: netbsd", "- goos: openbsd"} {
			if !strings.Contains(ci, want) {
				t.Errorf("ci.yml build matrix missing %q", want)
			}
		}
		for _, bad := range []string{"armv7", "goarch: i386"} {
			if strings.Contains(ci, bad) {
				t.Errorf("ci.yml still references legacy goarch %q", bad)
			}
		}
		if !strings.Contains(ci, "- NetBSD (amd64)") || !strings.Contains(ci, "- OpenBSD (amd64)") {
			t.Error("ci.yml release body does not list NetBSD/OpenBSD")
		}
	})

	t.Run("InstallDocsMatchCertifiedArchives", func(t *testing.T) {
		doc := mustRead(t, filepath.Join(root, "docs", "source", "getting-started", "installation.rst"))
		for _, arch := range certifiedArchives() {
			if !strings.Contains(doc, arch) {
				t.Errorf("installation.rst missing certified archive %q", arch)
			}
		}
		for _, bad := range []string{"linux-armv7", "linux-i386"} {
			if strings.Contains(doc, bad) {
				t.Errorf("installation.rst still references legacy archive %q", bad)
			}
		}
		if !strings.Contains(doc, "13-archive set") {
			t.Error("installation.rst does not state the 13-archive set")
		}
	})

	t.Run("DeveloperPreviewPointer", func(t *testing.T) {
		doc := mustRead(t, filepath.Join(root, "docs", "source", "development", "developer-preview.rst"))
		if !strings.Contains(doc, "superseded by **Karkain 1.0.0 (Stable)**") {
			t.Error("developer-preview.rst does not point at the 1.0.0 stable status")
		}
		if !strings.Contains(doc, ":doc:`/status/index`") {
			t.Error("developer-preview.rst does not reference the current status page /status/index")
		}
	})

	t.Run("OwnerAttribution", func(t *testing.T) {
		authors := mustRead(t, filepath.Join(root, "AUTHORS"))
		if !strings.Contains(authors, "Ajit Kumar") {
			t.Error("AUTHORS does not name the maintainer")
		}
		readme := mustRead(t, filepath.Join(root, "README.md"))
		if !strings.Contains(readme, "Ajit Kumar") {
			t.Error("README.md does not credit the maintainer")
		}
		conf := mustRead(t, filepath.Join(root, "docs", "source", "conf.py"))
		if !strings.Contains(conf, `author = "Ajit Kumar"`) {
			t.Error("docs/source/conf.py does not set the author to the maintainer")
		}
	})
}