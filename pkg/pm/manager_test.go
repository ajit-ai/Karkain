package pm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageManager_Init(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "my_project")

	result, err := InitProject(projectDir, "test_project")
	if err != nil {
		t.Fatalf("InitProject failed: %v", err)
	}

	if result.ProjectDir != projectDir {
		t.Errorf("expected ProjectDir %q, got %q", projectDir, result.ProjectDir)
	}

	if result.Manifest == "" {
		t.Fatal("expected Manifest path to be set")
	}
	if _, err := os.Stat(result.Manifest); os.IsNotExist(err) {
		t.Fatalf("manifest file does not exist: %s", result.Manifest)
	}

	manifest, err := ParseManifest(result.Manifest)
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}
	if manifest.Name != "test_project" {
		t.Errorf("expected name %q, got %q", "test_project", manifest.Name)
	}
	if manifest.Version != "0.1.0" {
		t.Errorf("expected version %q, got %q", "0.1.0", manifest.Version)
	}

	expectedDirs := []string{"src", "tests", ".karkain", ".karkain/cache"}
	for _, d := range expectedDirs {
		fullPath := filepath.Join(projectDir, d)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", d)
		}
	}

	expectedFiles := []string{"src/main.kark", "tests/main_test.kark", ".gitignore", "karkain.toml"}
	for _, f := range expectedFiles {
		fullPath := filepath.Join(projectDir, f)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	mainKar := filepath.Join(projectDir, "src", "main.kark")
	data, err := os.ReadFile(mainKar)
	if err != nil {
		t.Fatalf("cannot read main.kark: %v", err)
	}
	if !strings.Contains(string(data), "func main()") {
		t.Error("main.kark should contain func main()")
	}
	if !strings.Contains(string(data), "test_project") {
		t.Error("main.kark should reference the project name")
	}
}

func TestPackageManager_InitExistingDir(t *testing.T) {
	projectDir := t.TempDir()

	result, err := InitProject(projectDir, "existing_project")
	if err != nil {
		t.Fatalf("InitProject on existing dir failed: %v", err)
	}

	if _, err := os.Stat(result.Manifest); os.IsNotExist(err) {
		t.Fatalf("manifest file was not created")
	}
}

func TestPackageManager_ManifestParsing(t *testing.T) {
	manifestContent := `name = "my_app"
version = "1.2.3"
author = "Test Author"
description = "A test application"
license = "MIT"
targets = ["native", "wasm32-wasi"]

[dependencies]
stdlib = { version = "0.14.0", source = "registry" }
math_utils = { version = "2.0.0", source = "git", url = "https://github.com/example/math_utils" }
local_lib = { version = "0.1.0", source = "local", url = "../local_lib" }
`
	manifestPath := filepath.Join(t.TempDir(), "karkain.toml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("cannot write test manifest: %v", err)
	}

	m, err := ParseManifest(manifestPath)
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}

	if m.Name != "my_app" {
		t.Errorf("expected name %q, got %q", "my_app", m.Name)
	}
	if m.Version != "1.2.3" {
		t.Errorf("expected version %q, got %q", "1.2.3", m.Version)
	}
	if m.Author != "Test Author" {
		t.Errorf("expected author %q, got %q", "Test Author", m.Author)
	}
	if m.Description != "A test application" {
		t.Errorf("expected description %q, got %q", "A test application", m.Description)
	}
	if m.License != "MIT" {
		t.Errorf("expected license %q, got %q", "MIT", m.License)
	}
	if len(m.Targets) != 2 || m.Targets[0] != "native" || m.Targets[1] != "wasm32-wasi" {
		t.Errorf("unexpected targets: %v", m.Targets)
	}

	if len(m.Dependencies) != 3 {
		t.Fatalf("expected 3 dependencies, got %d", len(m.Dependencies))
	}

	stdlibDep, ok := m.Dependencies["stdlib"]
	if !ok {
		t.Fatal("stdlib dependency not found")
	}
	if stdlibDep.Version != "0.14.0" {
		t.Errorf("stdlib version: expected %q, got %q", "0.14.0", stdlibDep.Version)
	}
	if stdlibDep.Source != "registry" {
		t.Errorf("stdlib source: expected %q, got %q", "registry", stdlibDep.Source)
	}

	gitDep := m.Dependencies["math_utils"]
	if gitDep.Source != "git" {
		t.Errorf("math_utils source: expected %q, got %q", "git", gitDep.Source)
	}
	if gitDep.URL != "https://github.com/example/math_utils" {
		t.Errorf("math_utils URL: expected %q, got %q", "https://github.com/example/math_utils", gitDep.URL)
	}

	localDep := m.Dependencies["local_lib"]
	if localDep.Source != "local" {
		t.Errorf("local_lib source: expected %q, got %q", "local", localDep.Source)
	}
	if localDep.URL != "../local_lib" {
		t.Errorf("local_lib URL: expected %q, got %q", "../local_lib", localDep.URL)
	}
}

func TestPackageManager_ManifestRoundTrip(t *testing.T) {
	original := &Manifest{
		Name:         "roundtrip_test",
		Version:      "1.0.0",
		Author:       "Test",
		Description:  "Roundtrip test",
		License:      "MIT",
		Targets:      []string{"native"},
		Dependencies: map[string]Dependency{
			"dep_a": {Name: "dep_a", Version: "2.0.0", Source: "registry"},
			"dep_b": {Name: "dep_b", Version: "3.0.0", Source: "git", URL: "https://example.com/repo"},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "karkain.toml")

	if err := WriteManifest(path, original); err != nil {
		t.Fatalf("WriteManifest failed: %v", err)
	}

	parsed, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}

	if parsed.Name != original.Name {
		t.Errorf("name mismatch: %q vs %q", parsed.Name, original.Name)
	}
	if parsed.Version != original.Version {
		t.Errorf("version mismatch: %q vs %q", parsed.Version, original.Version)
	}
	if len(parsed.Dependencies) != len(original.Dependencies) {
		t.Errorf("dependency count mismatch: %d vs %d", len(parsed.Dependencies), len(original.Dependencies))
	}

	for name, origDep := range original.Dependencies {
		parsedDep, ok := parsed.Dependencies[name]
		if !ok {
			t.Errorf("dependency %q missing after round-trip", name)
			continue
		}
		if parsedDep.Version != origDep.Version {
			t.Errorf("dep %q version: %q vs %q", name, parsedDep.Version, origDep.Version)
		}
		if parsedDep.Source != origDep.Source {
			t.Errorf("dep %q source: %q vs %q", name, parsedDep.Source, origDep.Source)
		}
		if parsedDep.URL != origDep.URL {
			t.Errorf("dep %q URL: %q vs %q", name, parsedDep.URL, origDep.URL)
		}
	}
}

func TestPackageManager_AddDependency(t *testing.T) {
	projectDir := t.TempDir()
	_, err := InitProject(projectDir, "dep_test")
	if err != nil {
		t.Fatalf("InitProject failed: %v", err)
	}

	err = AddDependency(projectDir, "mylib", "1.0.0", "registry", "")
	if err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	m, err := ParseManifest(filepath.Join(projectDir, "karkain.toml"))
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}

	dep, ok := m.Dependencies["mylib"]
	if !ok {
		t.Fatal("dependency 'mylib' not found after adding")
	}
	if dep.Version != "1.0.0" {
		t.Errorf("expected version %q, got %q", "1.0.0", dep.Version)
	}
	if dep.Source != "registry" {
		t.Errorf("expected source %q, got %q", "registry", dep.Source)
	}

	err = AddDependency(projectDir, "gitlib", "2.0.0", "git", "https://github.com/test/lib")
	if err != nil {
		t.Fatalf("AddDependency (git) failed: %v", err)
	}

	m, err = ParseManifest(filepath.Join(projectDir, "karkain.toml"))
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}

	if len(m.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(m.Dependencies))
	}

	gitDep := m.Dependencies["gitlib"]
	if gitDep.URL != "https://github.com/test/lib" {
		t.Errorf("git dep URL: expected %q, got %q", "https://github.com/test/lib", gitDep.URL)
	}
}

func TestPackageManager_RemoveDependency(t *testing.T) {
	projectDir := t.TempDir()
	_, err := InitProject(projectDir, "rm_test")
	if err != nil {
		t.Fatalf("InitProject failed: %v", err)
	}

	AddDependency(projectDir, "to_remove", "1.0.0", "registry", "")
	AddDependency(projectDir, "to_keep", "2.0.0", "registry", "")

	err = RemoveDependency(projectDir, "to_remove")
	if err != nil {
		t.Fatalf("RemoveDependency failed: %v", err)
	}

	m, err := ParseManifest(filepath.Join(projectDir, "karkain.toml"))
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}

	if _, ok := m.Dependencies["to_remove"]; ok {
		t.Error("dependency 'to_remove' should have been removed")
	}
	if _, ok := m.Dependencies["to_keep"]; !ok {
		t.Error("dependency 'to_keep' should still exist")
	}

	err = RemoveDependency(projectDir, "nonexistent")
	if err == nil {
		t.Error("removing nonexistent dependency should fail")
	}
}

func TestPackageManager_ListDependencies(t *testing.T) {
	projectDir := t.TempDir()
	_, err := InitProject(projectDir, "list_test")
	if err != nil {
		t.Fatalf("InitProject failed: %v", err)
	}

	AddDependency(projectDir, "zebra", "1.0.0", "registry", "")
	AddDependency(projectDir, "alpha", "2.0.0", "registry", "")
	AddDependency(projectDir, "beta", "3.0.0", "registry", "")

	names, err := ListDependencies(projectDir)
	if err != nil {
		t.Fatalf("ListDependencies failed: %v", err)
	}

	expected := []string{"alpha", "beta", "zebra"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d deps, got %d", len(expected), len(names))
	}
	for i, n := range expected {
		if names[i] != n {
			t.Errorf("names[%d] = %q, expected %q", i, names[i], n)
		}
	}
}

func TestPackageManager_ResolveModule(t *testing.T) {
	projectDir := t.TempDir()
	_, err := InitProject(projectDir, "resolve_test")
	if err != nil {
		t.Fatalf("InitProject failed: %v", err)
	}

	localMod := filepath.Join(projectDir, "src", "mymod.kark")
	if err := os.WriteFile(localMod, []byte("func hello() {}"), 0644); err != nil {
		t.Fatalf("cannot create module: %v", err)
	}

	stdlibDir := filepath.Join(projectDir, "stdlib", "io")
	if err := os.MkdirAll(stdlibDir, 0755); err != nil {
		t.Fatalf("cannot create stdlib dir: %v", err)
	}
	stdlibMod := filepath.Join(stdlibDir, "io.kark")
	if err := os.WriteFile(stdlibMod, []byte("// io module"), 0644); err != nil {
		t.Fatalf("cannot create stdlib module: %v", err)
	}

	resolved, err := ResolveModule(projectDir, "mymod")
	if err != nil {
		t.Fatalf("ResolveModule for local module failed: %v", err)
	}
	if filepath.Base(resolved) != "mymod.kark" {
		t.Errorf("expected mymod.kark, got %s", filepath.Base(resolved))
	}

	resolved, err = ResolveModule(projectDir, "io")
	if err != nil {
		t.Fatalf("ResolveModule for stdlib module failed: %v", err)
	}
	if !strings.Contains(resolved, "io") {
		t.Errorf("expected io module path, got %s", resolved)
	}

	_, err = ResolveModule(projectDir, "nonexistent")
	if err == nil {
		t.Error("resolving nonexistent module should fail")
	}
}

func TestPackageManager_ValidateManifest(t *testing.T) {
	tests := []struct {
		name       string
		manifest   *Manifest
		wantIssues int
	}{
		{
			name:       "valid manifest",
			manifest:   &Manifest{Name: "test", Version: "1.0.0"},
			wantIssues: 0,
		},
		{
			name:       "missing name",
			manifest:   &Manifest{Version: "1.0.0"},
			wantIssues: 1,
		},
		{
			name:       "missing version",
			manifest:   &Manifest{Name: "test"},
			wantIssues: 1,
		},
		{
			name:       "invalid semver",
			manifest:   &Manifest{Name: "test", Version: "not-a-version"},
			wantIssues: 1,
		},
		{
			name:       "both missing",
			manifest:   &Manifest{},
			wantIssues: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := ValidateManifest(tt.manifest)
			if len(issues) != tt.wantIssues {
				t.Errorf("expected %d issues, got %d: %v", tt.wantIssues, len(issues), issues)
			}
		})
	}
}

func TestStdlib_ModuleImport(t *testing.T) {
	stdlibModules := []string{
		"../../stdlib/io/io.kark",
		"../../stdlib/math/math.kark",
		"../../stdlib/gpu/gpu.kark",
		"../../stdlib/async/actor.kark",
	}

	for _, mod := range stdlibModules {
		t.Run(mod, func(t *testing.T) {
			absPath, err := filepath.Abs(mod)
			if err != nil {
				t.Fatalf("cannot resolve path: %v", err)
			}

			data, err := os.ReadFile(absPath)
			if err != nil {
				t.Fatalf("cannot read module %s: %v", mod, err)
			}

			content := string(data)
			if len(content) == 0 {
				t.Errorf("module %s is empty", mod)
			}

			if !strings.Contains(content, "func ") {
				t.Errorf("module %s has no function declarations", mod)
			}
		})
	}
}

func TestPackageManager_FetchLocal(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "lib.kark")
	if err := os.WriteFile(srcFile, []byte("func lib_func() {}"), 0644); err != nil {
		t.Fatalf("cannot create source file: %v", err)
	}

	projectDir := t.TempDir()
	InitProject(projectDir, "fetch_test")

	dep := Dependency{
		Name:    "local_lib",
		Version: "0.1.0",
		Source:  "local",
		URL:     srcFile,
	}

	err := FetchModule(projectDir, dep)
	if err != nil {
		t.Fatalf("FetchModule failed: %v", err)
	}

	cachedPath := filepath.Join(projectDir, CacheModules, "local_lib", "lib.kark")
	data, err := os.ReadFile(cachedPath)
	if err != nil {
		t.Fatalf("cached file not found: %v", err)
	}
	if string(data) != "func lib_func() {}" {
		t.Errorf("cached content mismatch: %q", string(data))
	}
}

func TestPackageManager_ParseSimpleVersions(t *testing.T) {
	content := `name = "simple"
version = "0.1.0"

[dependencies]
foo = "1.0.0"
bar = "2.3.4"
`
	path := filepath.Join(t.TempDir(), "karkain.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := ParseManifest(path)
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}

	if m.Name != "simple" {
		t.Errorf("expected name %q, got %q", "simple", m.Name)
	}

	if len(m.Dependencies) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(m.Dependencies))
	}

	if m.Dependencies["foo"].Version != "1.0.0" {
		t.Errorf("foo version: %q", m.Dependencies["foo"].Version)
	}
	if m.Dependencies["foo"].Source != "registry" {
		t.Errorf("foo source: %q", m.Dependencies["foo"].Source)
	}
}
