package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 144 — Toolchain Sovereignty (PM resolution): the self-hosted engine
// assembles manifest/workspace/registry dependencies itself
// (projectDepSources in src/compiler/main.kark); the CLI mirrors project
// trees for kcc-native assembly (kccMirrorProject) and injects NOTHING.
//
// The strongest proof runs the kcc BINARY directly (phase95KCC): no Go
// bridge code executes at all — assembly success means Karkain resolved
// the dependency graph alone.

// phase144StageWorkspace copies examples/workspace to an isolated temp dir
// (never mutate the repo fixture) and returns the app entry + lib dir.
func phase144StageWorkspace(t *testing.T) (appMain, libDir, root string) {
	t.Helper()
	src := filepath.Join(repoRoot(t), "examples", "workspace")
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatalf("stage workspace: %v", err)
	}
	return filepath.Join(dst, "app", "src", "main.kark"),
		filepath.Join(dst, "library"),
		dst
}

func TestPhase144_DirectKccWorkspace(t *testing.T) {
	appMain, _, _ := phase144StageWorkspace(t)
	kcc := phase95KCC(t)

	out, err := exec.Command(kcc, "check", appMain).CombinedOutput()
	if err != nil {
		t.Fatalf("direct kcc check of workspace app failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "[ok]") {
		t.Fatalf("direct kcc check lacks [ok]:\n%s", out)
	}
}

func TestPhase144_CliBothEngineParity(t *testing.T) {
	hasGCC(t)
	selectKCCEngine(t, phase95KCC(t))
	appMain, _, _ := phase144StageWorkspace(t)
	bin := buildKarkain(t)

	goOut, err := runBin(t, bin, t.TempDir(), "run", appMain, "--engine", "go")
	if err != nil {
		t.Fatalf("go run of workspace app failed: %v\n%s", err, goOut)
	}
	kccOut, err := runBin(t, bin, t.TempDir(), "run", appMain, "--engine", "kcc")
	if err != nil {
		t.Fatalf("kcc run of workspace app failed: %v\n%s", err, kccOut)
	}
	// KCCRunCommand prefixes the sandbox build line; the program output is
	// everything after it.
	kccProg := kccOut
	if idx := strings.Index(kccProg, "\n"); idx >= 0 && strings.HasPrefix(kccProg, "[ok]") {
		kccProg = kccProg[idx+1:]
	}
	for _, want := range []string{"hi from library api", "hi from app"} {
		if !strings.Contains(goOut, want) {
			t.Errorf("go output lacks %q:\n%s", want, goOut)
		}
		if !strings.Contains(kccOut, want) {
			t.Errorf("kcc output lacks %q:\n%s", want, kccOut)
		}
	}
	if norm(goOut) != norm(kccProg) {
		t.Errorf("engine parity mismatch:\n-- go --\n%s\n-- kcc --\n%s", goOut, kccProg)
	}
}

func TestPhase144_MissingDepRejected(t *testing.T) {
	selectKCCEngine(t, phase95KCC(t))
	appMain, libDir, _ := phase144StageWorkspace(t)
	if err := os.RemoveAll(libDir); err != nil {
		t.Fatal(err)
	}
	bin := buildKarkain(t)

	// CLI (kcc engine): the staged check must fail without the dep.
	kccOut, err := runBin(t, bin, t.TempDir(), "run", appMain, "--engine", "kcc")
	if err == nil {
		t.Fatalf("kcc run with missing dep unexpectedly succeeded:\n%s", kccOut)
	}
	if !strings.Contains(kccOut, "greet") {
		t.Errorf("kcc missing-dep diagnostic should name 'greet':\n%s", kccOut)
	}

	// Direct kcc binary: same rejection, zero Go involvement.
	kcc := phase95KCC(t)
	out, err := exec.Command(kcc, "check", appMain).CombinedOutput()
	if err == nil && strings.Contains(string(out), "[ok]") {
		t.Fatalf("direct kcc check with missing dep unexpectedly [ok]:\n%s", out)
	}
	if !strings.Contains(string(out), "greet") {
		t.Errorf("direct kcc missing-dep diagnostic should name 'greet':\n%s", out)
	}
}

func TestPhase144_RegistryCacheShape(t *testing.T) {
	hasGCC(t)
	selectKCCEngine(t, phase95KCC(t))
	dir := t.TempDir()
	manifest := "name = \"app\"\nversion = \"1.0.0\"\n\n[dependencies]\ngreetlib = { version = \"1.0.0\", source = \"registry\" }\n"
	if err := os.WriteFile(filepath.Join(dir, "karkain.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dir, ".karkain", "cache", "greetlib@1.0.0")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "lib.kark"), []byte("func greet2() {\n    print(\"hi from registry\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(main, []byte("func main() {\n    greet2()\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Direct kcc: proves the cache-name computation lives in Karkain.
	kcc := phase95KCC(t)
	if out, err := exec.Command(kcc, "check", main).CombinedOutput(); err != nil || !strings.Contains(string(out), "[ok]") {
		t.Fatalf("direct kcc check of registry-cache project failed: %v\n%s", err, out)
	}

	// CLI parity through the mirror chain.
	bin := buildKarkain(t)
	goOut, err := runBin(t, bin, t.TempDir(), "run", main, "--engine", "go")
	if err != nil {
		t.Fatalf("go run of registry-cache project failed: %v\n%s", err, goOut)
	}
	kccOut, err := runBin(t, bin, t.TempDir(), "run", main, "--engine", "kcc")
	if err != nil {
		t.Fatalf("kcc run of registry-cache project failed: %v\n%s", err, kccOut)
	}
	kccProg := kccOut
	if idx := strings.Index(kccProg, "\n"); idx >= 0 && strings.HasPrefix(kccProg, "[ok]") {
		kccProg = kccProg[idx+1:]
	}
	if norm(goOut) != norm(kccProg) || !strings.Contains(kccProg, "hi from registry") {
		t.Errorf("registry parity mismatch:\n-- go --\n%s\n-- kcc --\n%s", goOut, kccProg)
	}
}

func TestPhase144_WiringPresence(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "src", "compiler", "main.kark"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	for _, fn := range []string{
		"func findProjectRoot(", "func manifestDepRecords(", "func lockLookup(",
		"func sanitizeCacheComponent(", "func depCacheName(", "func depSourceDir(",
		"func depDirPaths(", "func projectDepSources(", "func pmSplitFirst(",
		"func pmStripQuotes(", "func pmSplitTopLevel(", "func pmParseDepValue(",
		"func pmIsAbs(", "func normalizeSourceURL(", "func findWorkspaceRoot(",
		"func pmPathSeen(",
	} {
		if !strings.Contains(src, fn) {
			t.Errorf("compiler lacks %s", fn)
		}
	}
	if !strings.Contains(src, "projectDepSources(path) + content") &&
		!strings.Contains(src, "content = projectDepSources(path)") {
		t.Error("assembleProject does not prepend dependency sources")
	}
	eng, err := os.ReadFile(filepath.Join(root, "pkg", "cli", "kcc_engine.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range []string{"func kccMirrorProject(", "func pmWorkspaceRoot("} {
		if !strings.Contains(string(eng), fn) {
			t.Errorf("kcc_engine.go lacks %s", fn)
		}
	}
}

func TestPhase144_CliBuildKir(t *testing.T) {
	hasGCC(t)
	selectKCCEngine(t, phase95KCC(t))
	appMain, _, _ := phase144StageWorkspace(t)
	bin := buildKarkain(t)
	dir := t.TempDir()

	// kcc build links a real executable through the mirror chain.
	exe := filepath.Join(dir, "app")
	out, err := runBin(t, bin, dir, "build", appMain, "--engine", "kcc", "-o", exe)
	if err != nil {
		t.Fatalf("kcc build of workspace app failed: %v\n%s", err, out)
	}
	runOut, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatalf("built artifact failed: %v\n%s", err, runOut)
	}
	if !strings.Contains(string(runOut), "hi from library api") ||
		!strings.Contains(string(runOut), "hi from app") {
		t.Errorf("built artifact output wrong:\n%s", runOut)
	}

	// kcc kir verifies the assembled project (deps included).
	kirOut, err := runBin(t, bin, dir, "kir", "--verify", appMain, "--engine", "kcc")
	if err != nil {
		t.Fatalf("kcc kir --verify of workspace app failed: %v\n%s", err, kirOut)
	}
	if !strings.Contains(kirOut, "kir verify:") {
		t.Errorf("kir --verify lacks verification line:\n%s", kirOut)
	}
}
