package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	ktext "karkain/pkg/testing"
)

func hasGCC(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
}

// gccTempDir is a t.TempDir replacement for tests that hand files to the C
// toolchain. On Windows the OS/AV-indexer may briefly hold a handle on a
// freshly compiled artifact; t.TempDir's clean up then fails the test with a
// bogus "used by another process" error. Retry the removal best-effort instead.
func gccTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "karkain-gcc-test")
	if err != nil {
		t.Fatalf("mkTempDir: %v", err)
	}
	t.Cleanup(func() {
		for i := 0; i < 25; i++ {
			if err := os.RemoveAll(dir); err == nil {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		_ = os.RemoveAll(dir)
	})
	return dir
}

func writeTestFile(t *testing.T, dir, name, src string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(src), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func parseTestProgram(t *testing.T, src string) *parser.Program {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse errors: %s", strings.Join(p.Errors, "; "))
	}
	return parser.ApplyMacroExpansion(prog)
}

// --- Discovery ---

func TestKTFDiscovery_FindsTestsAndIsDeterministic(t *testing.T) {
	prog := parseTestProgram(t, `
func helper() { return 1 }
func test_zebra() { assert(1 == 1) }
func test_alpha() { assert(1 == 1) }
func ordinary() { return 2 }
func test_middle() { assert(1 == 1) }
`)
	cases := discoverTestCases(prog, "dummy_test.kark")
	// Only test_-prefixed functions are discovered.
	if len(cases) != 3 {
		t.Fatalf("expected 3 test cases, got %d: %+v", len(cases), cases)
	}
	// Deterministic (name-sorted) ordering.
	want := []string{"test_alpha", "test_middle", "test_zebra"}
	for i, tc := range cases {
		if tc.Name != want[i] {
			t.Errorf("order[%d] = %s, want %s", i, tc.Name, want[i])
		}
	}
	// Stable identity.
	if cases[0].ID != "dummy_test.kark:test_alpha" {
		t.Errorf("unexpected identity %q", cases[0].ID)
	}
}

func TestKTFDiscovery_IgnoresNonTestDeclarations(t *testing.T) {
	prog := parseTestProgram(t, `
func main() { print(1) }
func helper() { return 1 }
type alias = int
struct Point { x int, y int }
`)
	cases := discoverTestCases(prog, "empty_test.kark")
	if len(cases) != 0 {
		t.Fatalf("expected 0 cases, got %d", len(cases))
	}
}

func TestKTFDiscovery_OrderingStableAcrossInput(t *testing.T) {
	src := `
func test_c() { assert(1 == 1) }
func test_a() { assert(1 == 1) }
func test_b() { assert(1 == 1) }
`
	// Reversing the source order of statements must not change discovery order.
	reversed := `
func test_b() { assert(1 == 1) }
func test_a() { assert(1 == 1) }
func test_c() { assert(1 == 1) }
`
	c1 := discoverTestCases(parseTestProgram(t, src), "f.kark")
	c2 := discoverTestCases(parseTestProgram(t, reversed), "f.kark")
	if len(c1) != 3 || len(c2) != 3 {
		t.Fatalf("unexpected counts: %d %d", len(c1), len(c2))
	}
	for i := range c1 {
		if c1[i].Name != c2[i].Name || c1[i].ID != c2[i].ID {
			t.Errorf("mismatch at %d: %s vs %s", i, c1[i].Name, c2[i].Name)
		}
	}
}

// --- Filtering (pure model) ---

func TestKTFFilter_SelectsMatching(t *testing.T) {
	base := []ktext.TestCase{
		{ID: "a.kark:test_foo", Name: "test_foo"},
		{ID: "a.kark:test_bar", Name: "test_bar"},
		{ID: "b.kark:test_baz", Name: "test_baz"},
	}
	got := ktext.Filter(base, "test_ba")
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].Name != "test_bar" || got[1].Name != "test_baz" {
		t.Errorf("unexpected filtered set: %+v", got)
	}
}

func TestKTFFilter_UnmatchedReturnsEmpty(t *testing.T) {
	base := []ktext.TestCase{{ID: "a.kark:test_foo", Name: "test_foo"}}
	got := ktext.Filter(base, "nomatch")
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

func TestKTFFilter_EmptyMatchesAll(t *testing.T) {
	base := []ktext.TestCase{{ID: "a.kark:test_foo", Name: "test_foo"}}
	got := ktext.Filter(base, "")
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
}

// --- Aggregation (pure model) ---

func TestKTFSummary_Aggregates(t *testing.T) {
	results := []ktext.TestResult{
		{Status: ktext.StatusPass},
		{Status: ktext.StatusPass},
		{Status: ktext.StatusFail},
		{Status: ktext.StatusSkip},
	}
	s := ktext.Summarize(results)
	if s.Total != 4 || s.Passed != 2 || s.Failed != 1 || s.Skipped != 1 {
		t.Fatalf("unexpected summary: %+v", s)
	}
}

// --- Assertion runtime (E2E, structured) ---

func TestKTFAssertions_PassAndFailStructured(t *testing.T) {
	hasGCC(t)
	dir := t.TempDir()
	file := writeTestFile(t, dir, "assert_test.kark", `
func test_all_pass() {
    assert(1 == 1)
    assert_eq(2 + 3, 5)
    assert_ne(2 + 2, 5)
}
func test_false_condition() {
    assert(2 == 3)
}
`)
	cfg := codegen.NewConfig()
	results := runTestsInFile(file, cfg, false, "")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	s := ktext.Summarize(results)
	if s.Passed != 1 || s.Failed != 1 {
		t.Fatalf("expected 1 pass 1 fail, got %+v", s)
	}
	// Verify the failure carries structured expected/actual from assert_eq.
	failResult := results[1]
	if failResult.Status != ktext.StatusFail {
		t.Fatalf("expected fail, got %s", failResult.Status)
	}
	if failResult.Failure == nil {
		t.Fatal("expected failure detail")
	}
	if !strings.Contains(failResult.Failure.Message, "assert") {
		t.Errorf("unexpected failure message: %q", failResult.Failure.Message)
	}
}

func TestKTFAssert_eq_FailureExtractsExpectedActual(t *testing.T) {
	hasGCC(t)
	dir := t.TempDir()
	file := writeTestFile(t, dir, "eq_test.kark", `
func test_eq_fail() {
    assert_eq(2 + 2, 5)
}
`)
	cfg := codegen.NewConfig()
	results := runTestsInFile(file, cfg, false, "")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != ktext.StatusFail {
		t.Fatalf("expected fail, got %s", r.Status)
	}
	if r.Failure == nil {
		t.Fatal("expected failure")
	}
	if r.Failure.Expected != "5" {
		t.Errorf("expected= %q, want 5", r.Failure.Expected)
	}
	if r.Failure.Actual != "4" {
		t.Errorf("actual= %q, want 4", r.Failure.Actual)
	}
}

func TestKTFAssert_ne_Failure(t *testing.T) {
	hasGCC(t)
	dir := t.TempDir()
	file := writeTestFile(t, dir, "ne_test.kark", `
func test_ne_fail() {
    assert_ne(2 + 2, 4)
}
`)
	cfg := codegen.NewConfig()
	results := runTestsInFile(file, cfg, false, "")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != ktext.StatusFail {
		t.Fatalf("assert_ne with equal values should fail, got %s", results[0].Status)
	}
}

func TestKTFAssert_CapturedStdout(t *testing.T) {
	hasGCC(t)
	dir := t.TempDir()
	file := writeTestFile(t, dir, "out_test.kark", `
func test_prints() {
    print("hello-test")
    assert(1 == 1)
}
`)
	cfg := codegen.NewConfig()
	results := runTestsInFile(file, cfg, false, "")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != ktext.StatusPass {
		t.Fatalf("expected pass, got %s", results[0].Status)
	}
	if !strings.Contains(results[0].Stdout, "hello-test") {
		t.Errorf("expected program stdout captured, got %q", results[0].Stdout)
	}
}

// --- Command-level exit status + filter (E2E) ---

func TestKTFTestCommand_PassExitZero_FailExitNonZero(t *testing.T) {
	hasGCC(t)
	dir := gccTempDir(t)
	passFile := writeTestFile(t, dir, "pass_test.kark", `
func test_ok() { assert(1 == 1) }
`)
	failFile := writeTestFile(t, dir, "fail_test.kark", `
func test_bad() { assert(1 == 2) }
`)
	cfg := codegen.NewConfig()

	passRes := TestCommandFiltered(passFile, cfg, false, "")
	if passRes.ExitCode != 0 {
		t.Errorf("passing test should exit 0, got %d: %s", passRes.ExitCode, passRes.Message)
	}

	failRes := TestCommandFiltered(failFile, cfg, false, "")
	if failRes.ExitCode == 0 {
		t.Error("failing test should exit non-zero")
	}
}

func TestKTFTestCommand_FilterSelectsTests(t *testing.T) {
	hasGCC(t)
	dir := gccTempDir(t)
	file := writeTestFile(t, dir, "multi_test.kark", `
func test_wanted() { assert(1 == 1) }
func test_other()  { assert(1 == 1) }
`)
	cfg := codegen.NewConfig()

	out := captureStdout(t, func() {
		res := TestCommandFiltered(file, cfg, false, "wanted")
		if res.ExitCode != 0 {
			t.Errorf("filtered run should exit 0, got %d: %s", res.ExitCode, res.Message)
		}
	})
	if strings.Contains(out, "test_other") {
		t.Errorf("filter should exclude test_other, got output:\n%s", out)
	}
	if !strings.Contains(out, "test_wanted") {
		t.Errorf("filter should include test_wanted, got output:\n%s", out)
	}
}

func TestKTFTestCommand_UnmatchedFilterExitsZero(t *testing.T) {
	hasGCC(t)
	dir := gccTempDir(t)
	file := writeTestFile(t, dir, "m_test.kark", `
func test_ok() { assert(1 == 1) }
`)
	cfg := codegen.NewConfig()
	res := TestCommandFiltered(file, cfg, false, "does-not-exist")
	if res.ExitCode != 0 {
		t.Errorf("unmatched filter should not be a failure, got exit %d: %s", res.ExitCode, res.Message)
	}
	if !strings.Contains(res.Message, "No tests matched") {
		t.Errorf("unexpected message: %q", res.Message)
	}
}
