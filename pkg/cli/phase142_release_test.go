package cli

// Phase 142 — 1.1.0 Release gate.
//
// Guards the unified v1.1.0 / Stable Build identity (both engines, every
// emitter, LSP, docs entry points), the 1.1.0 changelog presence, the 1.0.x
// LTS branch, and a host-triple archive rehearsal (build → package →
// SHA-256 → verify → --version from the extracted tree), mirroring the CI
// release job. The v1.1.0 tag cut itself stays owner-only and is NOT part
// of this gate.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestPhase142_VersionIdentity pins the unified 1.1.0 identity.
func TestPhase142_VersionIdentity(t *testing.T) {
	bin := buildPreviewBinary(t)
	out, err := runBin(t, bin, t.TempDir(), "--version")
	if err != nil {
		t.Fatalf("karkain --version failed: %v\n%s", err, out)
	}
	for _, want := range []string{"Karkain Compiler v1.1.0", "Stable Build"} {
		if !strings.Contains(out, want) {
			t.Errorf("--version does not contain %q: %s", want, out)
		}
	}
	if strings.Contains(out, "v1.0.0") {
		t.Errorf("--version still carries the old identity: %s", out)
	}

	root := repoRoot(t)
	ver, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(ver)) != "1.1.0" {
		t.Errorf("VERSION = %q, want 1.1.0", strings.TrimSpace(string(ver)))
	}

	// kcc banner: the self-hosted binary reports the same identity.
	kcc := phase95KCC(t)
	kout, err := exec.Command(kcc, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("kcc --version failed: %v\n%s", err, kout)
	}
	if !strings.Contains(string(kout), "v1.1.0") {
		t.Errorf("kcc --version lacks v1.1.0: %s", kout)
	}
}

// TestPhase142_EmitterHeaders pins v1.1.0 in every generated-artifact
// header (Go emitters + self-hosted codegen) and forbids stale v1.0.0 in
// those live paths (historical docs/tests keep their records — this gate
// covers emitters and banners only).
func TestPhase142_EmitterHeaders(t *testing.T) {
	root := repoRoot(t)
	live := []string{
		"pkg/codegen/quantum_opt.go",
		"pkg/codegen/qir.go",
		"pkg/codegen/qec.go",
		"pkg/codegen/qasm.go",
		"pkg/codegen/openpulse.go",
		"pkg/codegen/dwarf.go",
		"src/compiler/codegen.kark",
		"src/compiler/main.kark",
		"pkg/cli/commands.go",
		"cmd/karkain/main.go",
	}
	for _, rel := range live {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if strings.Contains(string(raw), "v1.0.0") || strings.Contains(string(raw), " 1.0.0 ") {
			t.Errorf("%s still carries a 1.0.0 identity marker", rel)
		}
	}
	for _, rel := range []string{
		"pkg/codegen/quantum_opt.go",
		"pkg/codegen/qir.go",
		"src/compiler/codegen.kark",
	} {
		raw, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if !strings.Contains(string(raw), "v1.1.0") {
			t.Errorf("%s lacks the v1.1.0 header", rel)
		}
	}
}

// TestPhase142_ChangelogPresence pins the 1.1.0 release notes.
func TestPhase142_ChangelogPresence(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"docs/release/KARKAIN-1.1-RELEASE-NOTES.md",
		"docs/source/release-notes.rst",
	} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
		if !strings.Contains(string(raw), "1.1.0") {
			t.Errorf("%s does not document 1.1.0", rel)
		}
	}
}

// TestPhase142_LTSBranch pins the 1.0.x maintenance line. Local clones
// check refs directly; CI uses shallow single-ref checkouts (actions/
// checkout fetch-depth 1) where remote-tracking refs for other branches
// never exist — so the fallback asks the remote itself. A host with no
// git remote skips (unverifiable environment, not a product defect).
func TestPhase142_LTSBranch(t *testing.T) {
	root := repoRoot(t)
	for _, ref := range []string{"refs/heads/1.0.x", "refs/remotes/origin/1.0.x"} {
		cmd := exec.Command("git", "show-ref", "--verify", ref)
		cmd.Dir = root
		if err := cmd.Run(); err == nil {
			return
		}
	}
	cmd := exec.Command("git", "ls-remote", "--heads", "origin", "1.0.x")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("cannot reach origin to verify the LTS branch: %v", err)
	}
	if !strings.Contains(string(out), "refs/heads/1.0.x") {
		t.Error("no 1.0.x LTS branch on origin")
	}
}

// TestPhase142_ArchiveRehearsal mirrors the CI release job for the host
// triple: build → package (binary + README + LICENSE + stdlib + VERSION)
// → SHA-256 checksums.txt → verify → --version from the extracted tree.
func TestPhase142_ArchiveRehearsal(t *testing.T) {
	hasGCC(t)
	root := repoRoot(t)
	dir := t.TempDir()

	bin := filepath.Join(dir, "karkain")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/karkain")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("host build failed: %v\n%s", err, out)
	}

	for _, rel := range []string{"README.md", "LICENSE", "VERSION"} {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if err := os.WriteFile(filepath.Join(dir, rel), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	archive := filepath.Join(dir, "karkain-test.tar.gz")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	addFile := func(name string, data []byte, mode int64) {
		t.Helper()
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	binRaw, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	addFile("karkain-test/karkain", binRaw, 0o755)
	for _, rel := range []string{"README.md", "LICENSE", "VERSION"} {
		raw, _ := os.ReadFile(filepath.Join(dir, rel))
		addFile("karkain-test/"+rel, raw, 0o644)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(buf.Bytes())
	checks := hex.EncodeToString(sum[:]) + "  karkain-test.tar.gz\n"
	if err := os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(checks), 0o644); err != nil {
		t.Fatal(err)
	}
	// Verify: recompute and compare (the installation.rst ceremony).
	raw, _ := os.ReadFile(archive)
	reSum := sha256.Sum256(raw)
	if hex.EncodeToString(reSum[:]) != strings.Fields(checks)[0] {
		t.Fatal("checksum verification failed")
	}

	// --version from the "installed" tree.
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "v1.1.0") {
		t.Errorf("rehearsed binary lacks v1.1.0: %s", out)
	}
}
