package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/pm"
)

// Phase 97: the self-hosted compiler (kcc) is the DEFAULT engine, and manifest
// dependency resolution (pm.DependencySources) feeds the source-assembly
// pipeline so kcc check/build/run/test compile the full project. These focused
// tests prove (1) the default engine flip, (2) the explicit Go fallback, and
// (3) local/workspace dependency sources flowing into the pipeline.

// makeProject writes a minimal karkain.toml for a project at dir.
func makePhase97Project(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := &pm.Manifest{Name: name, Version: "1.0.0", Dependencies: map[string]pm.Dependency{}}
	if err := pm.WriteManifest(filepath.Join(dir, pm.ManifestFile), m); err != nil {
		t.Fatal(err)
	}
}

// addLocalDep declares a local-path dependency in proj's manifest.
func addPhase97LocalDep(t *testing.T, proj, name, url string) {
	t.Helper()
	p := filepath.Join(proj, pm.ManifestFile)
	m, err := pm.ParseManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if m.Dependencies == nil {
		m.Dependencies = map[string]pm.Dependency{}
	}
	m.Dependencies[name] = pm.Dependency{Name: name, Version: "0.1.0", Source: string(pm.SourceLocal), URL: url}
	if err := pm.WriteManifest(p, m); err != nil {
		t.Fatal(err)
	}
}

// TestPhase97_DefaultEngineIsKCC verifies the default-engine flip: no env and
// unrelated env values select kcc; only an explicit "go" selects the Go engine.
func TestPhase97_DefaultEngineIsKCC(t *testing.T) {
	t.Setenv("KARKAIN_ENGINE", "")
	if EngineFromEnv() != EngineKCC {
		t.Errorf("default (empty env) = %v, want EngineKCC", EngineFromEnv())
	}
	t.Setenv("KARKAIN_ENGINE", "anything-else")
	if EngineFromEnv() != EngineKCC {
		t.Errorf("unrelated env = %v, want EngineKCC", EngineFromEnv())
	}
	t.Setenv("KARKAIN_ENGINE", "kcc")
	if EngineFromEnv() != EngineKCC {
		t.Errorf("explicit kcc = %v, want EngineKCC", EngineFromEnv())
	}
}

// TestPhase97_GoEngineExplicitFallback verifies the Go engine stays reachable
// via explicit selection.
func TestPhase97_GoEngineExplicitFallback(t *testing.T) {
	t.Setenv("KARKAIN_ENGINE", "go")
	if EngineFromEnv() != EngineGo {
		t.Errorf("KARKAIN_ENGINE=go = %v, want EngineGo", EngineFromEnv())
	}
	t.Setenv("KARKAIN_ENGINE", "")
	if k, err := EngineFlag("go"); err != nil || k != EngineGo {
		t.Errorf("EngineFlag(go) = %v, %v", k, err)
	}
	if k, err := EngineFlag("kcc"); err != nil || k != EngineKCC {
		t.Errorf("EngineFlag(kcc) = %v, %v", k, err)
	}
}

// TestPhase97_ManifestDepsInCompile verifies kccAssembleSource (the kcc
// check/build/run loader) pulls a local dependency's source upstream of the
// root file.
func TestPhase97_ManifestDepsInCompile(t *testing.T) {
	proj := t.TempDir()
	makePhase97Project(t, proj, "app")

	// A local dependency whose only module defines depfunc().
	depDir := filepath.Join(proj, "mylib")
	if err := os.MkdirAll(depDir, 0o755); err != nil {
		t.Fatal(err)
	}
	depMod := filepath.Join(depDir, "lib.kark")
	if err := os.WriteFile(depMod, []byte("func depfunc() int { return 42 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	addPhase97LocalDep(t, proj, "mylib", "mylib")

	root := filepath.Join(proj, "main.kark")
	if err := os.WriteFile(root, []byte("func main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := kccAssembleSource(root)
	if err != nil {
		t.Fatalf("kccAssembleSource: %v", err)
	}
	if !strings.Contains(got, "func depfunc()") {
		t.Errorf("assembled source does not include dependency module\n%s", got)
	}
	// The root file is the entry point and comes last.
	if strings.Index(got, "depfunc") > strings.Index(got, "func main()") {
		t.Errorf("dependency source must precede the root/entry file")
	}
}

// TestPhase97_WorkspaceDepInCompile mirrors local for workspace-source deps.
func TestPhase97_WorkspaceDepInCompile(t *testing.T) {
	proj := t.TempDir()
	makePhase97Project(t, proj, "app")

	wsDir := filepath.Join(proj, "wslib")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "ws.kark"), []byte("func wsfn() int { return 7 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(proj, pm.ManifestFile)
	m, err := pm.ParseManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	m.Dependencies["wslib"] = pm.Dependency{Name: "wslib", Version: "0.1.0", Source: string(pm.SourceWorkspace), URL: "wslib"}
	if err := pm.WriteManifest(p, m); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(proj, "main.kark")
	if err := os.WriteFile(root, []byte("func main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := kccAssembleSource(root)
	if err != nil {
		t.Fatalf("kccAssembleSource: %v", err)
	}
	if !strings.Contains(got, "func wsfn()") {
		t.Errorf("workspace dependency source missing from assembly\n%s", got)
	}
}

// TestPhase97_LocalDepRunsThroughKCC builds a project whose main calls a
// function from a local dependency and runs it end-to-end through the kcc
// engine, asserting the dependency's code actually executes.
func TestPhase97_LocalDepRunsThroughKCC(t *testing.T) {
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	hasGCC(t)

	proj := t.TempDir()
	makePhase97Project(t, proj, "app")
	depDir := filepath.Join(proj, "mylib")
	if err := os.MkdirAll(depDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(depDir, "lib.kark"),
		[]byte("func answer() int { return 42 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	addPhase97LocalDep(t, proj, "mylib", "mylib")

	mainFile := filepath.Join(proj, "main.kark")
	mainSrc := "func main() {\n" +
		"    let v = answer()\n" +
		"    print(\"answer=\" + str(v))\n" +
		"}\n"
	if err := os.WriteFile(mainFile, []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	res := KCCRunCommand(nil, mainFile, mustConfig(t), false)
	if res.ExitCode != ExitSuccess {
		t.Fatalf("kcc run with local dep: exit=%d\n%s", res.ExitCode, res.Message)
	}
	if !strings.Contains(res.Message, "answer=42") {
		t.Errorf("kcc run output missing dependency result, got:\n%s", res.Message)
	}
}

// TestPhase97_CheckBuildRunTestShareDependencyAwarePipeline verifies the source
// assembly pipeline (resolveSources for check/build/run, projectModuleSources
// for test) resolves manifest dependencies across all four commands.
func TestPhase97_CheckBuildRunTestShareDependencyAwarePipeline(t *testing.T) {
	proj := t.TempDir()
	makePhase97Project(t, proj, "app")
	depDir := filepath.Join(proj, "mylib")
	if err := os.MkdirAll(depDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(depDir, "lib.kark"),
		[]byte("func answer() int { return 42 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	addPhase97LocalDep(t, proj, "mylib", "mylib")

	root := filepath.Join(proj, "main.kark")
	if err := os.WriteFile(root, []byte("func main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// check/build/run share resolveSources.
	rs, err := resolveSources(root)
	if err != nil {
		t.Fatalf("resolveSources: %v", err)
	}
	if !strings.Contains(rs, "func answer()") {
		t.Errorf("resolveSources (check/build/run) missing dependency\n%s", rs)
	}

	// test uses projectModuleSources.
	testFile := filepath.Join(proj, "app_test.kark")
	if err := os.WriteFile(testFile, []byte("func test_uses_dep() { assert_eq(answer(), 42) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mod, err := projectModuleSources(testFile)
	if err != nil {
		t.Fatalf("projectModuleSources: %v", err)
	}
	if !strings.Contains(mod, "func answer()") {
		t.Errorf("projectModuleSources (test) missing dependency\n%s", mod)
	}
	if strings.Contains(mod, "func main()") {
		t.Errorf("projectModuleSources must exclude the entry point")
	}
}
