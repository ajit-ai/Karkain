package cli

// Phase 135 — Local registry MVP gate (local filesystem only; no network).
//
// Proves the genuine data flow with the real binary and the real pipeline:
// publish -> filesystem registry -> index -> resolve -> compiler ->
// executable -> runtime output. Every registry lives in a temp dir and is
// removed with it; the child binary receives KARKAIN_REGISTRY only through
// its own environment (never process-global).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runBinEnv is shared with phase118_rc_test.go (same package): it runs the
// binary with KARKAIN_ENGINE=go plus extra environment entries.

func withLocalRegistry(reg string) []string {
	return []string{"KARKAIN_REGISTRY=" + reg}
}

func writeLibSource(t *testing.T, libDir string) {
	t.Helper()
	src := "public func greeting(name) {\n    return \"Hello \" + name\n}\n"
	if err := os.WriteFile(filepath.Join(libDir, "src", "superhello.kark"), []byte(src), 0644); err != nil {
		t.Fatalf("write lib source: %v", err)
	}
}

func writeAppMain(t *testing.T, appDir, dep string) {
	t.Helper()
	main := "import " + dep + "\n\nfunc main() {\n    print(" + dep + ".greeting(\"karkain\"))\n}\n"
	if err := os.WriteFile(filepath.Join(appDir, "src", "main.kark"), []byte(main), 0644); err != nil {
		t.Fatalf("write app main: %v", err)
	}
}

// publishLib creates, sources and publishes a library project, returning its dir.
func publishLib(t *testing.T, bin, root, reg, libName string) string {
	t.Helper()
	if out, err := runBin(t, bin, root, "new", libName); err != nil {
		t.Fatalf("karkain new %s failed: %v\n%s", libName, err, out)
	}
	libDir := filepath.Join(root, libName)
	writeLibSource(t, libDir)
	env := withLocalRegistry(reg)
	out, err := runBinEnv(t, bin, libDir, env, "pkg", "publish", "--registry", reg)
	if err != nil {
		t.Fatalf("publish failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Published "+libName+"@0.1.0") {
		t.Fatalf("publish should confirm %s@0.1.0, got:\n%s", libName, out)
	}
	return libDir
}

func TestPhase135_LocalRegistryE2E(t *testing.T) {
	skipIfNoCompiler(t)
	bin := buildKarkain(t)
	root := t.TempDir()
	reg := filepath.Join(root, "reg")

	// 1. Registry init creates the mission layout.
	out, err := runBin(t, bin, root, "pkg", "registry", "init", reg)
	if err != nil {
		t.Fatalf("registry init failed: %v\n%s", err, out)
	}
	for _, want := range []string{"registry.json", "packages", "index"} {
		if _, serr := os.Stat(filepath.Join(reg, want)); serr != nil {
			t.Fatalf("registry init should create %s: %v", want, serr)
		}
	}

	// 2. Create + publish the library.
	publishLib(t, bin, root, reg, "superhello")

	// Index discovers hello -> versions without scanning sources.
	idxRaw, err := os.ReadFile(filepath.Join(reg, "index", "superhello"))
	if err != nil {
		t.Fatalf("index entry missing after publish: %v", err)
	}
	if !strings.Contains(string(idxRaw), "0.1.0") {
		t.Fatalf("index should list 0.1.0, got:\n%s", idxRaw)
	}

	// 3. Consumer project declares the dependency through the real manifest.
	if out, err := runBin(t, bin, root, "new", "app"); err != nil {
		t.Fatalf("karkain new app failed: %v\n%s", err, out)
	}
	appDir := filepath.Join(root, "app")
	writeAppMain(t, appDir, "superhello")
	env := withLocalRegistry(reg)

	out, err = runBinEnv(t, bin, appDir, env, "add", "superhello", "0.1.0")
	if err != nil {
		t.Fatalf("karkain add failed: %v\n%s", err, out)
	}
	manifestRaw, _ := os.ReadFile(filepath.Join(appDir, "karkain.toml"))
	if !strings.Contains(string(manifestRaw), "superhello") || !strings.Contains(string(manifestRaw), "registry") {
		t.Fatalf("manifest should declare the registry dependency, got:\n%s", manifestRaw)
	}

	// 4. Fetch resolves + verifies + caches; lockfile pins the version.
	out, err = runBinEnv(t, bin, appDir, env, "fetch")
	if err != nil {
		t.Fatalf("karkain fetch failed: %v\n%s", err, out)
	}
	lockRaw, lerr := os.ReadFile(filepath.Join(appDir, "karkain.lock"))
	if lerr != nil {
		t.Fatalf("fetch should write karkain.lock: %v", lerr)
	}
	if !strings.Contains(string(lockRaw), "superhello") {
		t.Fatalf("lockfile should pin superhello, got:\n%s", lockRaw)
	}

	// 5. The consumer genuinely compiles against the published package and runs.
	out, err = runBinEnv(t, bin, appDir, env, "run", filepath.Join("src", "main.kark"))
	if err != nil {
		t.Fatalf("karkain run failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Hello karkain") {
		t.Fatalf("run should print the library result, got:\n%s", out)
	}
}

func TestPhase135_LocalRegistryNegatives(t *testing.T) {
	skipIfNoCompiler(t)
	bin := buildKarkain(t)
	root := t.TempDir()
	reg := filepath.Join(root, "reg")
	env := withLocalRegistry(reg)

	if out, err := runBin(t, bin, root, "pkg", "registry", "init", reg); err != nil {
		t.Fatalf("registry init failed: %v\n%s", err, out)
	}
	publishLib(t, bin, root, reg, "superhello")

	// A. Missing package: fetch fails with a meaningful diagnostic.
	if out, err := runBin(t, bin, root, "new", "appmissing"); err != nil {
		t.Fatalf("new appmissing failed: %v\n%s", err, out)
	}
	missDir := filepath.Join(root, "appmissing")
	writeAppMain(t, missDir, "missing")
	if out, err := runBinEnv(t, bin, missDir, env, "add", "missing", "1.0.0"); err != nil {
		t.Fatalf("add missing should record the dep (fetch is what fails): %v\n%s", err, out)
	}
	out, err := runBinEnv(t, bin, missDir, env, "fetch")
	if err == nil {
		t.Fatalf("fetch of missing@1.0.0 should fail")
	}
	if !strings.Contains(out, "missing") {
		t.Fatalf("missing-package diagnostic should name the package, got:\n%s", out)
	}
	// And the build itself fails instead of compiling without the dep.
	writeAppMain(t, missDir, "missing")
	out, err = runBinEnv(t, bin, missDir, env, "check", filepath.Join("src", "main.kark"))
	if err == nil {
		t.Fatalf("check with unresolved import should fail")
	}
	if out == "" {
		t.Fatalf("check should render a diagnostic for the missing import")
	}

	// B. Duplicate publication: refused, contents intact.
	idxBefore, _ := os.ReadFile(filepath.Join(reg, "index", "superhello"))
	srcBefore, _ := os.ReadFile(filepath.Join(reg, "packages", "superhello", "0.1.0", "source", "src", "superhello.kark"))
	out, err = runBinEnv(t, bin, filepath.Join(root, "superhello"), env, "pkg", "publish", "--registry", reg)
	if err == nil {
		t.Fatalf("second publish of superhello@0.1.0 should fail")
	}
	if !strings.Contains(out, "already published") {
		t.Fatalf("duplicate diagnostic should say already published, got:\n%s", out)
	}
	idxAfter, _ := os.ReadFile(filepath.Join(reg, "index", "superhello"))
	if string(idxAfter) != string(idxBefore) {
		t.Fatalf("index changed after refused duplicate publish")
	}
	srcAfter, _ := os.ReadFile(filepath.Join(reg, "packages", "superhello", "0.1.0", "source", "src", "superhello.kark"))
	if string(srcAfter) != string(srcBefore) {
		t.Fatalf("stored source changed after refused duplicate publish")
	}

	// C. Unsafe package identity: rejected, nothing escapes the registry.
	if out, err := runBin(t, bin, root, "new", "evilpkg"); err != nil {
		t.Fatalf("new evilpkg failed: %v\n%s", err, out)
	}
	evilDir := filepath.Join(root, "evilpkg")
	evilManifest := "name = \"../evil\"\nversion = \"1.0.0\"\n"
	if err := os.WriteFile(filepath.Join(evilDir, "karkain.toml"), []byte(evilManifest), 0644); err != nil {
		t.Fatalf("write evil manifest: %v", err)
	}
	rootBefore, _ := os.ReadDir(root)
	out, err = runBinEnv(t, bin, evilDir, env, "pkg", "publish", "--registry", reg)
	if err == nil {
		t.Fatalf("publish with traversal name should fail")
	}
	if !strings.Contains(out, "invalid package name") {
		t.Fatalf("unsafe-name diagnostic should say invalid package name, got:\n%s", out)
	}
	rootAfter, _ := os.ReadDir(root)
	if len(rootAfter) != len(rootBefore) {
		t.Fatalf("outside-registry files created by traversal attempt")
	}
	pkgEntries, _ := os.ReadDir(filepath.Join(reg, "packages"))
	if len(pkgEntries) != 1 || pkgEntries[0].Name() != "superhello" {
		t.Fatalf("packages dir polluted by traversal attempt")
	}

	// D. Version mismatch: unavailable version fails with a diagnostic.
	if out, err := runBin(t, bin, root, "new", "appmismatch"); err != nil {
		t.Fatalf("new appmismatch failed: %v\n%s", err, out)
	}
	mmDir := filepath.Join(root, "appmismatch")
	if out, err := runBinEnv(t, bin, mmDir, env, "add", "superhello", "9.9.9"); err != nil {
		t.Fatalf("add superhello 9.9.9 should record the dep: %v\n%s", err, out)
	}
	out, err = runBinEnv(t, bin, mmDir, env, "fetch")
	if err == nil {
		t.Fatalf("fetch of superhello@9.9.9 should fail")
	}
	if !strings.Contains(out, "9.9.9") {
		t.Fatalf("mismatch diagnostic should name the version, got:\n%s", out)
	}
}
