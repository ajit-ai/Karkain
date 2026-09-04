package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveSources_NonProject preserves classic behavior for files outside a
// Karkain project: the result must match loadSourceWithSiblings exactly.
func TestResolveSources_NonProject(t *testing.T) {
	dir := t.TempDir()
	mainFile := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(mainFile, []byte("func main() {\n  print(\"hi\")\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	helperFile := filepath.Join(dir, "helper.kark")
	if err := os.WriteFile(helperFile, []byte("func util() { return 1 }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveSources(mainFile)
	if err != nil {
		t.Fatalf("resolveSources: %v", err)
	}
	want, err := loadSourceWithSiblings(mainFile)
	if err != nil {
		t.Fatalf("loadSourceWithSiblings: %v", err)
	}
	if got != want {
		t.Errorf("non-project resolveSources diverged from classic:\n got=%q\nwant=%q", got, want)
	}
}

// TestResolveSources_ProjectLocalDepAssemblesUpstream verifies that in a real
// project, local dependency sources are emitted before the project's own
// modules and before the root file, and that all sources are concatenated once.
func TestResolveSources_ProjectLocalDepAssemblesUpstream(t *testing.T) {
	root := t.TempDir()

	// A local dependency directory following the endorsed layout (src/).
	depRoot := filepath.Join(root, "local_lib")
	if err := os.MkdirAll(depRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(depRoot, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	depSrc := filepath.Join(depRoot, "src", "math.kark")
	if err := os.WriteFile(depSrc, []byte("func lib_add(a, b) { return a + b }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Project manifest with a local dependency.
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := "karkain.toml"
	manifestContent := "[dependencies]\nlocal_lib = { version = \"0.1.0\", source = \"local\", url = \"local_lib\" }\n"
	if err := os.WriteFile(filepath.Join(root, manifest), []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Project sources: a sibling module and the root file.
	helperFile := filepath.Join(root, "src", "helper.kark")
	if err := os.WriteFile(helperFile, []byte("func project_helper() { return 99 }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mainFile := filepath.Join(root, "src", "main.kark")
	if err := os.WriteFile(mainFile, []byte("func main() {\n  print(lib_add(1, 2))\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveSources(mainFile)
	if err != nil {
		t.Fatalf("resolveSources: %v", err)
	}

	// Dependency source must appear before the project sibling, and both before main.
	depIdx := strings.Index(got, "lib_add")
	helperIdx := strings.Index(got, "project_helper")
	mainIdx := strings.Index(got, "func main")
	if depIdx == -1 || helperIdx == -1 || mainIdx == -1 {
		t.Fatalf("expected dep, helper and main markers in output:\n%q", got)
	}
	if !(depIdx < helperIdx && helperIdx < mainIdx) {
		t.Errorf("expected dependency < helper < main order, got depIdx=%d helperIdx=%d mainIdx=%d", depIdx, helperIdx, mainIdx)
	}

	// The root file must appear exactly once.
	if strings.Count(got, "func main") != 1 {
		t.Errorf("expected root file to appear once, got %d", strings.Count(got, "func main"))
	}
}

// TestResolveSources_ProjectSkipsRootSibling verifies the root file is not also
// pulled in as a sibling module (it is appended as the entry point).
func TestResolveSources_ProjectSkipsRootSibling(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "karkain.toml"), []byte("[dependencies]\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mainFile := filepath.Join(root, "src", "main.kark")
	if err := os.WriteFile(mainFile, []byte("func main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveSources(mainFile)
	if err != nil {
		t.Fatalf("resolveSources: %v", err)
	}
	if strings.Count(got, "func main") != 1 {
		t.Errorf("expected single main, got %q", got)
	}
}