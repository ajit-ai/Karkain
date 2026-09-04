package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// TestProjectTestRunner_SeesModulesAndDeps verifies that a test file under a
// Karkain project can call functions defined in a sibling module (src/helper.kark)
// and a local dependency (its src/math.kark), which the classic per-file runner
// could not see because it compiled only the test function in isolation.
func TestProjectTestRunner_SeesModulesAndDeps(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
	root := t.TempDir()

	// A local dependency following the endorsed layout (src/).
	depRoot := filepath.Join(root, "local_lib")
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
	if err := os.MkdirAll(filepath.Join(root, "tests"), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := "[dependencies]\nlocal_lib = { version = \"0.1.0\", source = \"local\", url = \"local_lib\" }\n"
	if err := os.WriteFile(filepath.Join(root, "karkain.toml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Sibling module defining a helper the test will call.
	helperFile := filepath.Join(root, "src", "helper.kark")
	if err := os.WriteFile(helperFile, []byte("func helper_double(x) { return x * 2 }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// The project entry point (not exercised by tests, but must exist for the
	// project root to be resolvable and not carry `func main` into test scope).
	mainFile := filepath.Join(root, "src", "main.kark")
	if err := os.WriteFile(mainFile, []byte("func main() { print(\"app\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Test file calls the sibling helper AND the local dependency function.
	testFile := filepath.Join(root, "tests", "main_test.kark")
	testSrc := "func test_module_scope() {\n" +
		"    print(helper_double(lib_add(10, 11)))\n" + // helper_double(21) = 42
		"}\n"
	if err := os.WriteFile(testFile, []byte(testSrc), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := codegen.NewConfig()
	// TestCommand needs an output path for the mini-programs.
	res := runSingleTestFile(testFile, cfg, false)
	if res.ExitCode != 0 {
		t.Fatalf("runSingleTestFile failed: %s", res.Message)
	}
}

// TestProjectTestRunner_FlatUnchanged verifies a non-project test file still
// runs with an empty module scope (unchanged behavior).
func TestProjectTestRunner_FlatUnchanged(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
	dir := t.TempDir()
	testFile := filepath.Join(dir, "math_test.kark")
	testSrc := "func test_flat() {\n    print(\"flat\")\n}\n"
	if err := os.WriteFile(testFile, []byte(testSrc), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := codegen.NewConfig()
	res := runSingleTestFile(testFile, cfg, false)
	if res.ExitCode != 0 {
		t.Fatalf("runSingleTestFile failed: %s", res.Message)
	}
}

// TestProjectTestRunner_ModuleNotDiscovered verifies discovery only picks up
// `test_`-prefixed functions, regardless of any test-named module code.
func TestProjectTestRunner_ModuleNotDiscovered(t *testing.T) {
	src := "func test_a() { print(1) }\nfunc test_b() { print(2) }\nfunc helper() { print(3) }\n"
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	fns := discoverTestFunctions(prog)
	if len(fns) != 2 {
		t.Errorf("expected 2 test functions, got %d", len(fns))
	}
}