package pm

// Phase 135 remediation regression tests (items 1 and 4): project-relative
// registry anchoring and internal-registry exclusion on publish. Temp dirs
// only; no network. CLI-level coverage (subdir invocation, flag errors,
// conflicts) lives in pkg/cli/phase135_registry_fixes_test.go.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalRegistryAnchorRef(t *testing.T) {
	// Absolute paths stay absolute (cleaned, never joined).
	absBase := t.TempDir()
	absReg := filepath.Join(absBase, "reg")
	if got := AnchorRegistryRef(filepath.Join(absBase, "proj"), absReg); got != absReg {
		t.Fatalf("absolute ref changed: %q", got)
	}
	// Relative paths join the project directory.
	got := AnchorRegistryRef(filepath.Join("a", "proj"), filepath.Join(".", "reg"))
	if got != filepath.Join("a", "proj", "reg") {
		t.Fatalf("relative ref not anchored: %q", got)
	}
	// URLs and empty references pass through untouched.
	for _, ref := range []string{"", "https://example.com/reg", "http://h/reg/"} {
		if AnchorRegistryRef("proj", ref) != strings.TrimSpace(ref) {
			t.Fatalf("ref %q should pass through", ref)
		}
	}
}

func TestLocalRegistryRefForDepProjectRelative(t *testing.T) {
	// Registry + project in temp dirs; the process CWD (pkg/pm during tests)
	// is unrelated to both, so success proves projectDir anchoring rather
	// than CWD resolution.
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	if err := os.MkdirAll(filepath.Join(proj, "src"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	reg := filepath.Join(proj, "reg")
	if err := InitLocalRegistry(reg); err != nil {
		t.Fatalf("init: %v", err)
	}

	dep := Dependency{Name: "hello", Version: "1.0.0", Source: "registry", URL: "reg"}
	kind, target, err := RegistryRefForDep(proj, "", dep)
	if err != nil {
		t.Fatalf("relative dep URL should anchor to projectDir: %v", err)
	}
	if kind != RegistryLocal {
		t.Fatalf("kind = %q, want local", kind)
	}
	if target != reg {
		t.Fatalf("target = %q, want %q", target, reg)
	}

	// Relative flag value anchors the same way.
	kind, target, err = RegistryRefForDep(proj, "./reg", Dependency{Name: "h", Version: "*", Source: "registry"})
	if err != nil {
		t.Fatalf("relative flag should anchor to projectDir: %v", err)
	}
	if kind != RegistryLocal || target != reg {
		t.Fatalf("flag anchor = (%q, %q), want (local, %q)", kind, target, reg)
	}

	// Absolute references are unaffected.
	kind, target, err = RegistryRefForDep(filepath.Join("elsewhere"), reg, Dependency{Name: "h"})
	if err != nil || kind != RegistryLocal || target != reg {
		t.Fatalf("absolute ref changed: (%q, %q, %v)", kind, target, err)
	}
}

func TestLocalRegistryPublishSkipsInternalRegistry(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	if err := os.MkdirAll(filepath.Join(proj, "src"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, ManifestFile), []byte("name = \"hello\"\nversion = \"1.0.0\"\n"), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, "src", "hello.kark"), []byte("public func f() { return 1 }\n"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	// A sibling directory that merely shares a name prefix must be kept.
	if err := os.MkdirAll(filepath.Join(proj, "registry-other"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, "registry-other", "keep.txt"), []byte("keep\n"), 0644); err != nil {
		t.Fatalf("write keep: %v", err)
	}
	// The real registry lives nested inside the project.
	reg := filepath.Join(proj, "a", "my-reg")
	if err := InitLocalRegistry(reg); err != nil {
		t.Fatalf("init: %v", err)
	}
	m, err := ParseManifest(filepath.Join(proj, ManifestFile))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := PublishLocal(reg, proj, m); err != nil {
		t.Fatalf("publish: %v", err)
	}

	stored := filepath.Join(reg, "packages", "hello", "1.0.0", "source")
	for _, leaked := range []string{"my-reg", filepath.Join("a", "my-reg")} {
		if _, err := os.Stat(filepath.Join(stored, leaked)); !os.IsNotExist(err) {
			t.Fatalf("published source embeds the registry tree at %s", leaked)
		}
	}
	if _, err := os.Stat(filepath.Join(stored, "registry-other", "keep.txt")); err != nil {
		t.Fatalf("prefix-similar sibling must be kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stored, "src", "hello.kark")); err != nil {
		t.Fatalf("real sources must be kept: %v", err)
	}
	// The exclusion is covered by the digest: fetch verifies and extracts.
	dest := filepath.Join(root, "out")
	if err := FetchLocal(reg, "hello", "1.0.0", dest); err != nil {
		t.Fatalf("fetch after exclusion: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "src", "hello.kark")); err != nil {
		t.Fatalf("fetched tree incomplete: %v", err)
	}
}

func TestLocalRegistryPublishAtProjectRoot(t *testing.T) {
	// Degenerate layout: the registry root IS the project directory. Only
	// the registry-owned top-level entries are skipped; project sources stay.
	proj := t.TempDir()
	if err := os.MkdirAll(filepath.Join(proj, "src"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, ManifestFile), []byte("name = \"hello\"\nversion = \"1.0.0\"\n"), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, "src", "hello.kark"), []byte("public func f() { return 1 }\n"), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := InitLocalRegistry(proj); err != nil {
		t.Fatalf("init: %v", err)
	}
	m, err := ParseManifest(filepath.Join(proj, ManifestFile))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := PublishLocal(proj, proj, m); err != nil {
		t.Fatalf("publish: %v", err)
	}
	stored := filepath.Join(proj, "packages", "hello", "1.0.0", "source")
	for _, skipped := range []string{"registry.json", "packages", "index"} {
		if _, err := os.Stat(filepath.Join(stored, skipped)); !os.IsNotExist(err) {
			t.Fatalf("published source embeds registry entry %s", skipped)
		}
	}
	if _, err := os.Stat(filepath.Join(stored, "src", "hello.kark")); err != nil {
		t.Fatalf("real sources must be kept: %v", err)
	}
}
