package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Phase 96: the self-hosted engine (kcc) owns the native test runner. The
// acceptance contract is that `kcc test` reproduces the Go runner's behavior
// on the same corpora:
//
//   - conformance/*_test.kark: every test_* function passes (exit 0, summary
//     shows zero failures)
//   - a failing *_test.kark: exit code maps to ExitTest(4) via the Go wrapper
//   - discovery order, per-test counting and the summary format are stable
//
// The kcc process itself always exits 0 (generated main returns 0), so
// KCCTestCommand parses the `N passed; M failed; S skipped; T total` summary
// line to derive the CLI exit code, mirroring TestCommandFiltered.

// suiteSummaryRe extracts passed/failed counts from a kcc test summary line.
var suiteSummaryRe = regexp.MustCompile(`(\d+) passed;\s*(\d+) failed;\s*(\d+) skipped;\s*(\d+) total`)

func parseKCCSummary(t *testing.T, out string) (passed, failed, skipped, total int) {
	t.Helper()
	m := suiteSummaryRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no summary line in kcc test output:\n%s", out)
	}
	return atoiS(t, m[1]), atoiS(t, m[2]), atoiS(t, m[3]), atoiS(t, m[4])
}

func atoiS(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Fatalf("non-numeric segment %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// TestPhase96_KCCConformanceParity runs the full conformance corpus through the
// self-hosted test runner and asserts the same zero-failure outcome (and total
// test count) as the Go path.
func TestPhase96_KCCConformanceParity(t *testing.T) {
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	hasGCC(t)

	dir := conformanceDir(t)

	res := KCCTestCommand(nil, dir, "")
	if res.ExitCode != ExitSuccess {
		t.Fatalf("kcc conformance: exit=%d msg=\n%s", res.ExitCode, res.Message)
	}
	passed, failed, _, total := parseKCCSummary(t, res.Message)
	if failed != 0 {
		t.Errorf("kcc conformance: %d tests failed\n%s", failed, res.Message)
	}

	files, goTests := conformanceCounts(t, dir)
	if total != goTests {
		t.Errorf("kcc reported %d tests, Go discovery found %d across %d files", total, goTests, files)
	}
	if passed != total {
		t.Errorf("kcc reported %d passed but %d total", passed, total)
	}
	if !strings.Contains(res.Message, "=== Test Summary:") {
		t.Errorf("kcc output missing summary header:\n%s", res.Message)
	}
}

// TestPhase96_KCCFailingTest maps the exit-4 contract: a passing test file
// exits success, a file with a failing assertion exits ExitTest(4). Both the
// Go runner and the kcc wrapper must agree on the process exit code.
func TestPhase96_KCCFailingTest(t *testing.T) {
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	hasGCC(t)

	dir := t.TempDir()
	pass := filepath.Join(dir, "ok_test.kark")
	content := "func test_addition() {\n" +
		"    let a = 2 + 3\n" +
		"    assert_eq(a, 5)\n" +
		"}\n"
	if err := os.WriteFile(pass, []byte(content), 0o644); err != nil {
		t.Fatalf("write ok_test.kark: %v", err)
	}
	fail := filepath.Join(dir, "bad_test.kark")
	bad := "func test_wrong() {\n" +
		"    let a = 2 + 3\n" +
		"    assert_eq(a, 99)\n" +
		"}\n"
	if err := os.WriteFile(fail, []byte(bad), 0o644); err != nil {
		t.Fatalf("write bad_test.kark: %v", err)
	}

	// kcc side.
	kccRes := KCCTestCommand(nil, dir, "")
	if kccRes.ExitCode != ExitTest {
		t.Errorf("kcc failing suite: exit=%d want %d\n%s", kccRes.ExitCode, ExitTest, kccRes.Message)
	}
	p, f, _, tot := parseKCCSummary(t, kccRes.Message)
	if tot != 2 || p != 1 || f != 1 {
		t.Errorf("kcc failing suite: passed=%d failed=%d total=%d want 1/1/2\n%s", p, f, tot, kccRes.Message)
	}

	// Go side must agree.
	goRes := TestCommandFiltered(dir, mustConfig(t), false, "")
	if goRes.ExitCode != ExitTest {
		t.Errorf("go failing suite: exit=%d want %d", goRes.ExitCode, ExitTest)
	}
}

// TestPhase96_KCCSingleFileAndFilter exercises the single-file path and the
// substring filter, both of which must mirror the Go runner.
func TestPhase96_KCCSingleFileAndFilter(t *testing.T) {
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	hasGCC(t)

	dir := t.TempDir()
	file := filepath.Join(dir, "multi_test.kark")
	content := "func test_alpha() { assert_eq(1, 1) }\n" +
		"func test_beta()  { assert_eq(2, 2) }\n"
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("write multi_test.kark: %v", err)
	}

	// Single file (no discovery) must run both tests.
	single := KCCTestCommand(nil, file, "")
	if single.ExitCode != ExitSuccess {
		t.Fatalf("single file: exit=%d\n%s", single.ExitCode, single.Message)
	}
	_, _, _, tot := parseKCCSummary(t, single.Message)
	if tot != 2 {
		t.Errorf("single file: total=%d want 2\n%s", tot, single.Message)
	}

	// Filter selects a subset (Go: substring match on Name).
	filtered := KCCTestCommand(nil, file, "beta")
	if filtered.ExitCode != ExitSuccess {
		t.Fatalf("filtered: exit=%d\n%s", filtered.ExitCode, filtered.Message)
	}
	p, _, _, t2 := parseKCCSummary(t, filtered.Message)
	if t2 != 1 || p != 1 {
		t.Errorf("filtered: passed=%d total=%d want 1/1", p, t2)
	}
}

// TestPhase96_KCCEmptyDirectory mirrors the Go runner's "No test files found."
// success response for a directory without *_test.kark files.
func TestPhase96_KCCEmptyDirectory(t *testing.T) {
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)

	dir := t.TempDir() // empty
	res := KCCTestCommand(nil, dir, "")
	if res.ExitCode != ExitSuccess {
		t.Fatalf("empty dir: exit=%d want ExitSuccess\n%s", res.ExitCode, res.Message)
	}
	if !strings.Contains(res.Message, "No test files found.") {
		t.Errorf("empty dir: missing 'No test files found.' in %q", res.Message)
	}
}