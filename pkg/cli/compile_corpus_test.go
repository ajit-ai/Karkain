package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"karkain/pkg/codegen"
	ktesting "karkain/pkg/testing"
)

// corpusDirForTest resolves the bundled corpus from the pkg/cli working dir.
func corpusDirForTest() string { return DefaultCompileCorpus }

func TestLoadCompileManifest_BundledCorpus(t *testing.T) {
	cases, err := loadCompileManifest(corpusDirForTest())
	if err != nil {
		t.Fatalf("bundled corpus should load: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("bundled corpus must not be empty")
	}
	sawPass, sawFail := false, false
	for _, c := range cases {
		if c.Expect == ktesting.ExpectPass {
			sawPass = true
		}
		if c.Expect == ktesting.ExpectFail {
			sawFail = true
		}
		if !fileExists(filepath.Join(corpusDirForTest(), c.File)) {
			t.Errorf("case %s references a missing file", c.File)
		}
	}
	if !sawPass || !sawFail {
		t.Errorf("corpus must contain both pass and fail cases (pass=%v fail=%v)", sawPass, sawFail)
	}
}

func TestLoadCompileManifest_DeterministicOrder(t *testing.T) {
	a, err := loadCompileManifest(corpusDirForTest())
	if err != nil {
		t.Fatal(err)
	}
	b, err := loadCompileManifest(corpusDirForTest())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Error("manifest load must be deterministic")
	}
}

func TestLoadCompileManifest_RejectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	src := `{"cases": [{"file": "missing.kark", "expect": "fail", "code": "E-K-SYN"}]}`
	if err := os.WriteFile(filepath.Join(dir, CompileManifestFile), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCompileManifest(dir); err == nil {
		t.Error("manifest referencing a missing file must error")
	}
}

func TestLoadCompileManifest_RejectsBadExpect(t *testing.T) {
	dir := t.TempDir()
	src := `{"cases": [{"file": "x.kark", "expect": "maybe"}]}`
	if err := os.WriteFile(filepath.Join(dir, CompileManifestFile), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.kark"), []byte("func main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCompileManifest(dir); err == nil {
		t.Error("manifest with unknown expect must error")
	}
}

func TestRunFailCases_AllClassify(t *testing.T) {
	cases, err := loadCompileManifest(corpusDirForTest())
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, c := range cases {
		if c.Expect != ktesting.ExpectFail {
			continue
		}
		checked++
		r := runFailCase(c, corpusDirForTest(), false)
		if !r.Passed() {
			t.Errorf("fail case %s did not classify: %s", c.File, r.Error)
		}
	}
	if checked == 0 {
		t.Fatal("no fail cases found to classify")
	}
}

func TestRunFailCase_UnexpectedCleanFileFails(t *testing.T) {
	cases, err := loadCompileManifest(corpusDirForTest())
	if err != nil {
		t.Fatal(err)
	}
	var clean ktesting.CompileCase
	for _, c := range cases {
		if c.Expect == ktesting.ExpectPass {
			clean = c
			break
		}
	}
	if clean.File == "" {
		t.Fatal("corpus has no pass case to misuse")
	}
	bad := ktesting.CompileCase{File: clean.File, Expect: ktesting.ExpectFail, Code: "E-K-SYN"}
	r := runFailCase(bad, corpusDirForTest(), false)
	if r.Passed() {
		t.Errorf("a clean file expected to fail must not pass: %s", r.Error)
	}
}

func TestRunPassCases_CompileOrSkip(t *testing.T) {
	canCompile := haveCCompiler()
	cases, err := loadCompileManifest(corpusDirForTest())
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, c := range cases {
		if c.Expect != ktesting.ExpectPass {
			continue
		}
		checked++
		r := runPassCase(c, corpusDirForTest(), canCompile, codegen.NewConfig())
		if r.Status == ktesting.StatusSkip {
			continue // no C toolchain on this machine: skip is the honest result
		}
		if !r.Passed() {
			t.Errorf("pass case %s did not compile: %s", c.File, r.Error)
		}
	}
	if checked == 0 {
		t.Fatal("no pass cases found")
	}
}

func TestRunCompileCorpus_NoFailures(t *testing.T) {
	s := RunCompileCorpus(corpusDirForTest(), false, codegen.NewConfig())
	if s.Failed != 0 {
		t.Errorf("bundled corpus must have no failures: %+v", s)
	}
	if s.Total == 0 {
		t.Error("corpus ran no cases")
	}
}

func TestCLI_TestCompile_E2E(t *testing.T) {
	bin := buildKarkain(t)
	root := t.TempDir()
	corpus := filepath.Join(repoRoot(t), "pkg", "cli", "testdata", "compile")
	out, err := runBin(t, bin, root, "test", "--compile", corpus)
	if err != nil {
		t.Fatalf("karkain test --compile should exit 0: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Compile Corpus Summary") {
		t.Errorf("expected summary in output:\n%s", out)
	}
	if !strings.Contains(out, "0 failed") {
		t.Errorf("expected zero failures:\n%s", out)
	}
}

func TestCLI_TestCompile_FailingCorpusExitsTestStatus(t *testing.T) {
	bin := buildKarkain(t)
	root := t.TempDir()
	corpus := filepath.Join(root, "corpus")
	if err := os.MkdirAll(corpus, 0755); err != nil {
		t.Fatal(err)
	}
	// A fail case declared with the wrong error code can never match.
	manifest := `{"cases": [{"file": "bad.kark", "expect": "fail", "code": "E-K-SYN"}]}`
	if err := os.WriteFile(filepath.Join(corpus, CompileManifestFile), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	// help_me is an E-K-RES finding, not E-K-SYN: the case must fail.
	if err := os.WriteFile(filepath.Join(corpus, "bad.kark"), []byte("func main() {\n    help_me()\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := runBin(t, bin, root, "test", "--compile", corpus)
	if err == nil {
		t.Fatalf("mismatched fail case should exit non-zero:\n%s", out)
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 4 {
		t.Errorf("expected exit code 4 (test failure), got %v:\n%s", ee.ExitCode(), out)
	}
	if !strings.Contains(out, "no diagnostic matched") {
		t.Errorf("expected mismatch diagnostic in output:\n%s", out)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
