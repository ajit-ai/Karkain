package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/codegen"
)

// runModuleEntry assembles the import-driven unit for rootFile via the real
// CLI loader (resolveSourcesRun), then compiles+runs it with gcc.
func runModuleEntry(t *testing.T, rootFile string) string {
	t.Helper()
	hasGCC(t)
	text, err := resolveSourcesRun(rootFile)
	if err != nil {
		t.Fatalf("resolveSourcesRun(%s): %v", rootFile, err)
	}
	out := compileRunSource(t, text, rootFile, t.TempDir())
	return strings.ReplaceAll(out, "\r\n", "\n")
}

func writeKark(t *testing.T, dir, name, src string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// TestModuleE2E_SiblingFileImport: `import math` resolves to a sibling
// math.kark, whose functions are compiled upstream of the root entry.
func TestModuleE2E_SiblingFileImport(t *testing.T) {
	dir := t.TempDir()
	writeKark(t, dir, "math.kark", `func twice(x) {
    return x * 2
}
`)
	root := writeKark(t, dir, "main.kark", `import math
func main() {
    print(math.twice(21))
}
`)
	out := strings.TrimSpace(runModuleEntry(t, root))
	if out != "42" {
		t.Fatalf("sibling import: want 42, got %q", out)
	}
}

// TestModuleE2E_SubdirModuleImport: `import utils` resolves to a sibling
// directory utils/ containing .kark sources.
func TestModuleE2E_SubdirModuleImport(t *testing.T) {
	dir := t.TempDir()
	writeKark(t, dir, "utils/fib.kark", `func utils_fib(n) {
    if (n <= 1) {
        return n
    }
    return utils_fib(n - 1) + utils_fib(n - 2)
}
`)
	root := writeKark(t, dir, "main.kark", `import utils
func main() {
    print(utils.utils_fib(10))
}
`)
	out := strings.TrimSpace(runModuleEntry(t, root))
	if out != "55" {
		t.Fatalf("subdir import: want 55, got %q", out)
	}
}

// TestModuleE2E_DuplicateMainExcluded: a non-root module that also declares
// `func main` must be excluded from the assembled unit (only the root entry may
// host main), and the stray main must not execute.
func TestModuleE2E_DuplicateMainExcluded(t *testing.T) {
	dir := t.TempDir()
	writeKark(t, dir, "helper.kark", `func helper() {
    print(99)
}
func main() {
    print(999)
}
`)
	root := writeKark(t, dir, "main.kark", `import helper
func main() {
    print(1)
}
`)
	out := runModuleEntry(t, root)
	if strings.Contains(out, "999") || strings.Contains(out, "99") {
		t.Fatalf("stray main executed: %q", out)
	}
	if !strings.Contains(out, "1") {
		t.Fatalf("entry main missing: %q", out)
	}
}

// TestModuleE2E_CycleDiagnostic: an import cycle is reported as a compile-class
// module error through RunCommand's loader.
func TestModuleE2E_CycleDiagnostic(t *testing.T) {
	dir := t.TempDir()
	writeKark(t, dir, "a.kark", `import b
func from_a() {
    return 1
}
`)
	writeKark(t, dir, "b.kark", `import a
func from_b() {
    return 2
}
`)
	root := writeKark(t, dir, "main.kark", `import a
func main() {
    print(from_a())
}
`)
	res := RunCommand(root, codegen.NewConfig(), false)
	if res.ExitCode != ExitCompile {
		t.Fatalf("cycle: want ExitCode %d, got %d (%s)", ExitCompile, res.ExitCode, res.Message)
	}
	if !strings.Contains(res.Message, "module error") || !strings.Contains(res.Message, "cycle") {
		t.Fatalf("cycle: want module error mentioning cycle, got: %s", res.Message)
	}
}

// TestModuleE2E_NotFoundDiagnostic: importing a module that is neither a
// sibling file nor a sibling directory fails with a module-not-found
// diagnostic.
func TestModuleE2E_NotFoundDiagnostic(t *testing.T) {
	dir := t.TempDir()
	root := writeKark(t, dir, "main.kark", `import missingmod
func main() {
    print(1)
}
`)
	res := RunCommand(root, codegen.NewConfig(), false)
	if res.ExitCode != ExitCompile {
		t.Fatalf("not-found: want ExitCode %d, got %d (%s)", ExitCompile, res.ExitCode, res.Message)
	}
	if !strings.Contains(res.Message, "module 'missingmod' not found") {
		t.Fatalf("not-found: want diagnostic, got: %s", res.Message)
	}
}

// TestModuleE2E_CheckResolvesImports: CheckCommand feeds the module assembly
// (text + source map) into the name-resolution pass, so a valid cross-file
// call must pass cleanly.
func TestModuleE2E_CheckResolvesImports(t *testing.T) {
	dir := t.TempDir()
	writeKark(t, dir, "math.kark", `func twice(x) {
    return x * 2
}
`)
	root := writeKark(t, dir, "main.kark", `import math
func main() {
    print(math.twice(21))
}
`)
	res := CheckCommand(root, false)
	if res.ExitCode != ExitSuccess {
		t.Fatalf("check: want ExitCode %d, got %d (%s)", ExitSuccess, res.ExitCode, res.Message)
	}
}

// TestCheckCommand_SiblingJoinParity locks the check/run assembly agreement: a
// non-project directory whose root entry calls a sibling module must pass
// check exactly as it runs (both pipelines join siblings without `func main`).
func TestCheckCommand_SiblingJoinParity(t *testing.T) {
	dir := t.TempDir()
	writeKark(t, dir, "math.kark", `func twice(x) {
    return x * 2
}
`)
	root := writeKark(t, dir, "main.kark", `func main() {
    print(twice(21))
}
`)
	text, serr := resolveSources(root)
	if serr != nil {
		t.Fatalf("resolveSources(%s): %v", root, serr)
	}
	out := compileRunSource(t, text, root, t.TempDir())
	if got := strings.TrimSpace(strings.ReplaceAll(out, "\r\n", "\n")); got != "42" {
		t.Fatalf("run: want 42, got %q", got)
	}
	res := CheckCommand(root, false)
	if res.ExitCode != ExitSuccess {
		t.Fatalf("check must agree with run (sibling-join parity): got %d (%s)", res.ExitCode, res.Message)
	}
}

func mustSource(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}