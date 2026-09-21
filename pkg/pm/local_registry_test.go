package pm

// Phase 135 unit tests: local directory registry (init / publish / index /
// resolve / fetch). All state lives in temp dirs; no network, no global globs.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkLocalProject creates a minimal publishable project with one source file.
func mkLocalProject(t *testing.T, name, version, srcName, src string) string {
	t.Helper()
	dir := t.TempDir()
	manifest := "name = \"" + name + "\"\nversion = \"" + version + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", srcName), []byte(src), 0644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return dir
}

func mustParseManifest(t *testing.T, projectDir string) *Manifest {
	t.Helper()
	m, err := ParseManifest(filepath.Join(projectDir, ManifestFile))
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

func TestLocalRegistryInit(t *testing.T) {
	reg := filepath.Join(t.TempDir(), "reg")
	if err := InitLocalRegistry(reg); err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, want := range []string{LocalRegistryMarker, "packages", "index"} {
		if _, err := os.Stat(filepath.Join(reg, want)); err != nil {
			t.Fatalf("missing %s after init: %v", want, err)
		}
	}
	if err := InitLocalRegistry(reg); err == nil {
		t.Fatalf("re-init should be refused")
	} else if !strings.Contains(err.Error(), "already initialized") {
		t.Fatalf("re-init error should say already initialized, got: %v", err)
	}
}

func TestLocalRegistryNameValidation(t *testing.T) {
	for _, bad := range []string{"", ".", "..", "../evil", "a/b", `a\b`, "/abs", "C:\\win", "a..b/c", "!!!", "-", "...", "has space"} {
		if err := ValidatePackageName(bad); err == nil {
			t.Errorf("ValidatePackageName(%q) should fail", bad)
		}
	}
	for _, good := range []string{"hello", "my-lib_2.0", "a", "x.y-z_9"} {
		if err := ValidatePackageName(good); err != nil {
			t.Errorf("ValidatePackageName(%q) should pass: %v", good, err)
		}
	}
}

func TestLocalRegistryVersionValidation(t *testing.T) {
	for _, bad := range []string{"", "abc", "1.0", "v1", "1.0.0.0.0"} {
		if err := ValidateVersion(bad); err == nil {
			t.Errorf("ValidateVersion(%q) should fail", bad)
		}
	}
	for _, good := range []string{"1.0.0", "0.0.1", "2.3.4-alpha", "10.20.30"} {
		if err := ValidateVersion(good); err != nil {
			t.Errorf("ValidateVersion(%q) should pass: %v", good, err)
		}
	}
}

func TestLocalRegistryPublishResolveFetch(t *testing.T) {
	reg := filepath.Join(t.TempDir(), "reg")
	if err := InitLocalRegistry(reg); err != nil {
		t.Fatalf("init: %v", err)
	}
	proj := mkLocalProject(t, "hello", "1.0.0", "hello.kark",
		"public func greeting(name) {\n    return \"Hello \" + name\n}\n")
	if err := PublishLocal(reg, proj, mustParseManifest(t, proj)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Storage layout: manifest + source + digest.
	vdir := filepath.Join(reg, "packages", "hello", "1.0.0")
	for _, want := range []string{"manifest", "sha256", filepath.Join("source", "src", "hello.kark")} {
		if _, err := os.Stat(filepath.Join(vdir, want)); err != nil {
			t.Fatalf("missing stored %s: %v", want, err)
		}
	}
	// No cache debris may leak into the published tree.
	if _, err := os.Stat(filepath.Join(vdir, "source", CacheDir)); !os.IsNotExist(err) {
		t.Fatalf("published tree must exclude %s", CacheDir)
	}

	// Index lookup without scanning sources.
	idx, err := ReadLocalIndex(reg, "hello")
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if idx == nil || idx.Latest != "1.0.0" || len(idx.Versions) != 1 || idx.Versions[0] != "1.0.0" {
		t.Fatalf("unexpected index: %+v", idx)
	}

	// Resolution: exact, empty, wildcard, semver range.
	for _, tc := range []struct{ constraint, want string }{
		{"1.0.0", "1.0.0"},
		{"", "1.0.0"},
		{"*", "1.0.0"},
		{"^1.0.0", "1.0.0"},
	} {
		got, err := ResolveLocalVersion(reg, "hello", tc.constraint)
		if err != nil {
			t.Fatalf("resolve %q: %v", tc.constraint, err)
		}
		if got != tc.want {
			t.Fatalf("resolve %q = %q, want %q", tc.constraint, got, tc.want)
		}
	}

	// Fetch round-trip preserves bytes.
	dest := filepath.Join(t.TempDir(), "out")
	if err := FetchLocal(reg, "hello", "1.0.0", dest); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "src", "hello.kark"))
	if err != nil {
		t.Fatalf("read fetched: %v", err)
	}
	if !strings.Contains(string(got), "Hello") {
		t.Fatalf("fetched content mismatch: %q", got)
	}
}

func TestLocalRegistryDuplicatePublish(t *testing.T) {
	reg := filepath.Join(t.TempDir(), "reg")
	if err := InitLocalRegistry(reg); err != nil {
		t.Fatalf("init: %v", err)
	}
	proj := mkLocalProject(t, "hello", "1.0.0", "hello.kark", "public func f() { return 1 }\n")
	if err := PublishLocal(reg, proj, mustParseManifest(t, proj)); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(reg, "index", "hello"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	srcBefore, err := os.ReadFile(filepath.Join(reg, "packages", "hello", "1.0.0", "source", "src", "hello.kark"))
	if err != nil {
		t.Fatalf("read stored source: %v", err)
	}

	if err := PublishLocal(reg, proj, mustParseManifest(t, proj)); err == nil {
		t.Fatalf("duplicate publish should fail")
	} else if !strings.Contains(err.Error(), "already published") {
		t.Fatalf("duplicate error should say already published, got: %v", err)
	}

	after, _ := os.ReadFile(filepath.Join(reg, "index", "hello"))
	if string(after) != string(before) {
		t.Fatalf("index changed after refused publish")
	}
	srcAfter, _ := os.ReadFile(filepath.Join(reg, "packages", "hello", "1.0.0", "source", "src", "hello.kark"))
	if string(srcAfter) != string(srcBefore) {
		t.Fatalf("stored source changed after refused publish")
	}
}

func TestLocalRegistryUnsafeNamePublish(t *testing.T) {
	reg := filepath.Join(t.TempDir(), "reg")
	if err := InitLocalRegistry(reg); err != nil {
		t.Fatalf("init: %v", err)
	}
	proj := mkLocalProject(t, "hello", "1.0.0", "hello.kark", "public func f() { return 1 }\n")
	m := mustParseManifest(t, proj)

	parentBefore := dirNames(t, filepath.Dir(reg))
	for _, evil := range []string{"../evil", "..", "/abs", "a/b", ""} {
		m.Name = evil
		if err := PublishLocal(reg, proj, m); err == nil {
			t.Fatalf("publish with name %q should fail", evil)
		}
	}
	if got := dirNames(t, filepath.Dir(reg)); !equalStrings(parentBefore, got) {
		t.Fatalf("outside-registry files created: before %v after %v", parentBefore, got)
	}
	entries, _ := os.ReadDir(filepath.Join(reg, "packages"))
	if len(entries) != 0 {
		t.Fatalf("packages dir should stay empty, has %d entries", len(entries))
	}
}

func TestLocalRegistryMissingAndMismatch(t *testing.T) {
	reg := filepath.Join(t.TempDir(), "reg")
	if err := InitLocalRegistry(reg); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := ResolveLocalVersion(reg, "missing", "1.0.0"); err == nil {
		t.Fatalf("resolve missing package should fail")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing-package error should say not found, got: %v", err)
	}
	proj := mkLocalProject(t, "hello", "1.0.0", "hello.kark", "public func f() { return 1 }\n")
	if err := PublishLocal(reg, proj, mustParseManifest(t, proj)); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := ResolveLocalVersion(reg, "hello", "9.9.9"); err == nil {
		t.Fatalf("resolve unavailable version should fail")
	} else if !strings.Contains(err.Error(), "9.9.9") {
		t.Fatalf("mismatch error should name the version, got: %v", err)
	}
	if err := FetchLocal(reg, "hello", "9.9.9", filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatalf("fetch unavailable version should fail")
	}
}

func TestLocalRegistryTamperAndCorrupt(t *testing.T) {
	reg := filepath.Join(t.TempDir(), "reg")
	if err := InitLocalRegistry(reg); err != nil {
		t.Fatalf("init: %v", err)
	}
	proj := mkLocalProject(t, "hello", "1.0.0", "hello.kark", "public func f() { return 1 }\n")
	if err := PublishLocal(reg, proj, mustParseManifest(t, proj)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Tamper with the stored source: fetch must refuse integrity failure.
	stored := filepath.Join(reg, "packages", "hello", "1.0.0", "source", "src", "hello.kark")
	if err := os.WriteFile(stored, []byte("public func f() { return 999 }\n"), 0644); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	if err := FetchLocal(reg, "hello", "1.0.0", filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatalf("fetch of tampered package should fail")
	} else if !strings.Contains(err.Error(), "integrity check failed") {
		t.Fatalf("tamper error should say integrity check failed, got: %v", err)
	}

	// Corrupt the index: resolution must report corrupt metadata, not guess.
	if err := os.WriteFile(filepath.Join(reg, "index", "hello"), []byte("{nope"), 0644); err != nil {
		t.Fatalf("corrupt index: %v", err)
	}
	if _, err := ResolveLocalVersion(reg, "hello", "1.0.0"); err == nil {
		t.Fatalf("resolve with corrupt index should fail")
	} else if !strings.Contains(err.Error(), "corrupt registry metadata") {
		t.Fatalf("corrupt error should say corrupt registry metadata, got: %v", err)
	}
}

func TestLocalRegistryRefSelection(t *testing.T) {
	reg := filepath.Join(t.TempDir(), "reg")
	if err := InitLocalRegistry(reg); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Explicit flag wins over the environment.
	t.Setenv("KARKAIN_REGISTRY", filepath.Join(t.TempDir(), "other"))
	got, err := RegistryRef(reg)
	if err != nil {
		t.Fatalf("flag ref: %v", err)
	}
	if abs, _ := filepath.Abs(reg); got != abs {
		t.Fatalf("flag should win over env: got %s want %s", got, abs)
	}

	// Env fallback when no flag is given.
	t.Setenv("KARKAIN_REGISTRY", reg)
	got, err = RegistryRef("")
	if err != nil {
		t.Fatalf("env ref: %v", err)
	}
	if abs, _ := filepath.Abs(reg); got != abs {
		t.Fatalf("env fallback failed: got %s want %s", got, abs)
	}

	// Nothing configured: actionable error, no silent fallback.
	t.Setenv("KARKAIN_REGISTRY", "")
	if _, err := RegistryRef(""); err == nil {
		t.Fatalf("empty ref should fail")
	} else if !strings.Contains(err.Error(), "--registry") {
		t.Fatalf("empty-ref error should mention --registry, got: %v", err)
	}

	// Network references are honestly refused (local-only phase).
	if _, err := RegistryRef("https://example.com/reg"); err == nil {
		t.Fatalf("http ref should fail in the local-only phase")
	} else if !strings.Contains(err.Error(), "local-only") {
		t.Fatalf("http-ref error should say local-only, got: %v", err)
	}

	// Non-registry directory is unavailable, not silently accepted.
	if _, err := RegistryRef(t.TempDir()); err == nil {
		t.Fatalf("plain directory should fail without a marker")
	} else if !strings.Contains(err.Error(), "registry init") {
		t.Fatalf("plain-dir error should mention registry init, got: %v", err)
	}
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
