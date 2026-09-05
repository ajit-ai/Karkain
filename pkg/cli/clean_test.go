package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return err == nil
}

func TestCleanCommand_RemovesOnlySourcedArtifacts(t *testing.T) {
	root := t.TempDir()

	// Sources.
	writeFile(t, filepath.Join(root, "app.kark"), "func main() { print(1) }\n")
	writeFile(t, filepath.Join(root, "helper.kark"), "func helper() { return 1 }\n")
	writeFile(t, filepath.Join(root, "kernels.kark"), "func main() {}\n")
	writeFile(t, filepath.Join(root, "foo_test.kark"), "func test_x() { assert(1 == 1) }\n")

	// Artifacts that must be removed (source-anchored).
	writeFile(t, filepath.Join(root, "app.c"), "int main(void){}\n")
	writeFile(t, filepath.Join(root, "app.exe"), "MZ")
	writeFile(t, filepath.Join(root, "helper.c"), "int helper(void){}\n")
	writeFile(t, filepath.Join(root, "kernels_do_thing.wgsl"), "shader")
	writeFile(t, filepath.Join(root, "kernels_vec.cl"), "kernel")
	writeFile(t, filepath.Join(root, "foo_test.test.c"), "int main(void){}\n")
	writeFile(t, filepath.Join(root, "foo_test.test.exe"), "MZ")

	// Hand-written C without a sibling .kark must never be touched.
	writeFile(t, filepath.Join(root, "handwritten.c"), "int handwritten(void){}\n")
	// Manifest/lock must survive.
	writeFile(t, filepath.Join(root, "karkain.toml"), "name = \"app\"\n")
	// A stray .exe with no sibling source is preserved.
	writeFile(t, filepath.Join(root, "stray.exe"), "MZ")

	res := CleanCommand(root, false)
	if res.ExitCode != ExitSuccess {
		t.Fatalf("CleanCommand = (%d, %q), want success", res.ExitCode, res.Message)
	}

	for _, gone := range []string{
		"app.c", "app.exe", "helper.c", "kernels_do_thing.wgsl", "kernels_vec.cl",
		"foo_test.test.c", "foo_test.test.exe",
	} {
		if exists(t, filepath.Join(root, gone)) {
			t.Errorf("expected %s to be removed", gone)
		}
	}
	for _, kept := range []string{
		"app.kark", "helper.kark", "kernels.kark", "foo_test.kark",
		"handwritten.c", "karkain.toml", "stray.exe",
	} {
		if !exists(t, filepath.Join(root, kept)) {
			t.Errorf("expected %s to be preserved", kept)
		}
	}
}

func TestCleanCommand_AllEmptiesBin(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "app.kark"), "func main() { print(1) }\n")
	writeFile(t, filepath.Join(root, "karkain.toml"), "name = \"app\"\n")
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(binDir, "karkain_test.exe"), "MZ")

	res := CleanCommand(root, true)
	if res.ExitCode != ExitSuccess {
		t.Fatalf("CleanCommand --all = (%d, %q), want success", res.ExitCode, res.Message)
	}
	if entries, err := os.ReadDir(binDir); err != nil || len(entries) != 0 {
		t.Errorf("bin/ should be empty after --all, got %d entries (err=%v)", len(entries), err)
	}
	if !exists(t, filepath.Join(root, "karkain.toml")) {
		t.Error("karkain.toml must never be removed")
	}
}

func TestCleanCommand_PathErrors(t *testing.T) {
	if res := CleanCommand(filepath.Join(t.TempDir(), "missing"), false); res.ExitCode != ExitFailure {
		t.Errorf("missing dir: got %d, want Failure", res.ExitCode)
	}
	file := filepath.Join(t.TempDir(), "note.txt")
	writeFile(t, file, "x")
	if res := CleanCommand(file, false); res.ExitCode != ExitUsage {
		t.Errorf("non-directory: got %d, want Usage", res.ExitCode)
	}
}
