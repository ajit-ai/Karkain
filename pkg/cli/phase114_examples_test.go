package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karkain/pkg/codegen"
)

// Phase 114 gate: the official example corpus.
//
// Every Runnable example in the 15-category corpus (examples/01-fundamentals â€¦
// examples/14-security + developer tools) must compile and run to a pinned
// golden output on BOTH engines (Go front end and the self-hosted kcc engine)
// with byte-identical stdout. Experimental examples (08-concurrency) are only
// Go-engine so far (kcc parity is a documented post-107 boundary) and are
// validated on the Go leg alone. Test-mode files (*_test.kark) are validated
// through the real karkain test runner instead of a plain run.

// phase114Examples pins goldens: rel path under examples/ -> expected stdout.
// status == "go" means the example is Go-engine only (experimental).
var phase114Examples = map[string]struct {
	golden string
	engine string // "" = both engines, "go" = Go engine only
}{
	"01-fundamentals/01_hello_world.kark":    {"Hello, Karkain!\n", ""},
	"01-fundamentals/02_variables.kark":      {"42\nkarkain\n123\n", ""},
	"01-fundamentals/03_constants.kark":      {"540\n8\n", ""},
	"01-fundamentals/04_functions.kark":      {"49\n7\nnegative\nnon-negative\n120\n1\n", ""},
	"01-fundamentals/05_conditionals.kark":   {"A\nB\nC\nD\nF\neven\n", ""},
	"01-fundamentals/06_loops.kark":          {"0\n1\n2\n3\n4\n55\n[3, 2, 1]\n", ""},
	"01-fundamentals/07_strings.kark":        {"karkain\n7\nk\ni\nkark\ncomputed: 42\n", ""},
	"01-fundamentals/08_arrays.kark":         {"5\n2\n11\n[3, 5, 7]\n0\n[0, 1, 4, 9, 16]\n5\n3\n", ""},
	"01-fundamentals/09_maps.kark":           {"92\n84\n97\n2\n1\n0\n3\nana\nbob\ncam\n", ""},
	"01-fundamentals/10_structs.kark":        {"ana\n100\n150\n150\n130\n", ""},
	"01-fundamentals/11_match.kark":          {"300\n7\n1000000\nthree\n", ""},
	"01-fundamentals/13_enums.kark":          {"1\n1\n100\n200\n300\n0\n", ""},
	"01-fundamentals/14_adt_match.kark":      {"10\n20\n30\n1\n", ""},
	"01-fundamentals/15_closures.kark":      {"42\n42\n42\n42\n16\n1\n", ""},
	"01-fundamentals/12_casts.kark":          {"3\n3.4\n4.5\n9\n7\n256\n3\n3.5\n640\n0\n", ""},
	"02-algorithms/01_linear_search.kark":    {"2\n3\n-1\n", ""},
	"02-algorithms/02_binary_search.kark":    {"3\n0\n7\n-1\n-1\n-1\n", ""},
	"02-algorithms/03_min_max.kark":          {"1\n12\n-9\n-1\n", ""},
	"02-algorithms/04_bubble_sort.kark":      {"[1, 2, 3, 5, 7, 8, 9]\n[1, 2, 3, 4]\n[1, 2, 3, 4]\n", ""},
	"02-algorithms/05_frequency_count.kark":  {"3\n2\n1\n4\n", ""},
	"02-algorithms/06_fibonacci.kark":        {"0\n1\n5\n55\n610\n6765\n", ""},
	"02-algorithms/07_factorial.kark":        {"1\n1\n120\n40320\n479001600\n", ""},
	"02-algorithms/08_gcd.kark":              {"6\n1\n20\n12\n42\n", ""},
	"02-algorithms/09_prime_sieve.kark":      {"[2, 3, 5, 7]\n10\n[2, 3, 5, 7, 11, 13, 17, 19, 23, 29]\n", ""},
	"02-algorithms/10_palindrome.kark":       {"1\n0\n1\n1\nolleh\n", ""},
	"02-algorithms/11_stack.kark":            {"3\n30\n20\n10\n0\n", ""},
	"02-algorithms/12_queue.kark":            {"10\n20\n50\n20\n30\n40\n50\n", ""},
	"02-algorithms/13_knapsack.kark":         {"7\n9\n", ""},
	"03-systems/01_file_io.kark":             {"true\n3\nalpha\nbeta\ngamma\nfalse\n", ""},
	"04-networking/01_tcp_echo.kark":        {"4\nping\n4\npong\n3\n", ""},
	"04-networking/02_tcp_roundtrip.kark":   {"3\none\n3\ntwo\n3\n", ""},
	"05-data/01_word_frequency.kark":         {"3\n2\n2\n1\n8\n", ""},
	"05-data/02_csv_aggregate.kark":          {"15\n20\n15\n2\n50\n20\n", ""},
	"05-data/03_payload_roundtrip.kark":      {"4b61726b61696e2064617461\nKarkain data\nS2Fya2FpbiBkYXRh\nKarkain data\n1\n1\n", ""},
	"05-data/04_token_stats.kark":            {"9\n3\n35\n", ""},
	"06-database/01_db_crud.kark":            {"3\n1\nalice\n90\n2\nbob\n80\n3\ncarol\n95\n100\nalice\ncarol\ndb2\nembedded\n", ""},
	"06-database/02_db_persist.kark":         {"true\n2\n1\nalpha\n2\nbeta\n", ""},
	"07-web/01_http_loopback.kark":           {"90\nGET\n/hello\nok\n0\n70\n200\nOK\nhello /hello\n", ""},
	"07-web/02_http_codec.kark":              {"GET\n/items\ntext/plain\n3\n201\nCreated\nok\n", ""},
	"08-concurrency/01_parallel_sum.kark":    {"285\n", "go"},
	"08-concurrency/02_channel_ping.kark":    {"5\n0\n8\n", "go"},
	"09-ai/01_nearest_neighbor.kark":         {"10\n60\n100\n60\n", ""},
	"09-ai/02_linear_classifier.kark":        {"5\n1\n-1\n2\n-3\n", ""},
	"09-ai/03_numerics_forward.kark":         {"0.3\n0.23\n2.71828\n0.5\n0.761594\n0\n3\n1.41421\n10\n5\n25\n1\n0.0466667\n0.2\n0.23\n", ""},
	"10-machine-learning/01_linear_regression.kark": {"0.9\n1.3\n6.7\n", ""},
	"10-machine-learning/02_gradient_descent.kark":  {"3\n7\n", ""},
	"10-machine-learning/03_mlp_forward.kark":      {"0\n0\n0\n0\n1\n3\n", ""},
	"12-scientific-computing/01_sqrt_newton.kark":   {"1.41421\n3\n2\n0.5\n", ""},
	"12-scientific-computing/02_numerical_integration.kark": {"0.34375\n0.333374\n0.333333\n", ""},
	"12-scientific-computing/03_statistics.kark":            {"5\n4\n2\n", ""},
	"12-scientific-computing/04_matrix_multiply.kark":       {"[4, 1, 4]\n[1, 2, 4]\n[6, 4, 1]\n4\n1\n", ""},
	"13-finance/01_compound_interest.kark":                  {"115762\n231854\n20000\n10000\n", ""},
	"13-finance/02_loan_amortization.kark":                  {"12950.5\n11231.4\n12754.8\n", ""},
	"13-finance/03_npv.kark":                                {"-454.545\n3992.71\n", ""},
	"13-finance/04_portfolio.kark":                          {"5\n3\n7\n", ""},
	"14-security/01_digests.kark":                           {"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\nddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f\n", ""},
	"14-security/02_encoding_roundtrip.kark":                {"766572696679206d65\nverify me\ndmVyaWZ5IG1l\nverify me\n", ""},
	"14-security/03_password_hash.kark":                     {"f03ca5a084a27b196a052c25363583cd51f5827b46c0830dff54643606c80c66\n1\n0\n64\n", ""},
	"14-security/04_utf8_text.kark":                         {"1\n636166c3a9\n1\nCAF\xc3\xa9\n", ""},
	"15-developer-tools/01_hello_toolchain.kark":            {"write -> build -> run\n", ""},
}

// phase114TestFiles are *_test.kark corpus files that must be validated through
// the real test runner (they use language-level assert builtins), not a plain
// run.
var phase114TestFiles = []string{
	"15-developer-tools/02_assertions_test.kark",
}

// phase114RunExample builds the example through the given engine and runs the
// resulting executable from a clean temp working directory (IO examples create
// and delete their demo file there). Returns normalized stdout. Mirrors
// phase109RunExample with retry for transient Windows gcc failures.
func phase114RunExample(t *testing.T, engine, rel string) string {
	t.Helper()
	src := filepath.Join(repoRoot(t), "examples", rel)
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("example missing: %s (%v)", src, err)
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "main.exe")

	const attempts = 4
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		var buildOut bytes.Buffer
		res := phase114BuildExample(engine, &buildOut, src, exe)
		if res.ExitCode != ExitSuccess {
			lastErr = &phase114BuildError{engine: engine, rel: rel, msg: res.Message}
			if attempt < attempts && strings.Contains(res.Message, "C compilation failed") {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			t.Fatalf("%s build %s: %q (exit %d)", engine, rel, res.Message, res.ExitCode)
		}
		if _, err := os.Stat(exe); err != nil {
			t.Fatalf("expected executable after %s build: %v", engine, err)
		}
		run := exec.Command(exe)
		run.Dir = dir // clean temp cwd: IO examples create/delete their demo file here
		var stdout, stderr bytes.Buffer
		run.Stdout = &stdout
		run.Stderr = &stderr
		if err := run.Run(); err != nil {
			lastErr = err
			if attempt < attempts {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			t.Fatalf("run %s (%s engine) failed: %v (%s)", rel, engine, err, stderr.String())
		}
		return strings.ReplaceAll(stdout.String(), "\r\n", "\n")
	}
	t.Fatalf("transient failures running %s: %v", rel, lastErr)
	return ""
}

type phase114BuildError struct {
	engine, rel, msg string
}

func (e *phase114BuildError) Error() string {
	return e.engine + " build " + e.rel + ": " + e.msg
}

func phase114BuildExample(engine string, w *bytes.Buffer, src, exe string) CommandResult {
	if engine == "go" {
		return BuildCommandIncremental(src, exe, codegen.Config{}, false, filepath.Join(filepath.Dir(exe), ".cache"))
	}
	return KCCBuildCommand(w, src, exe, codegen.Config{}, false)
}

// TestPhase114_CorpusExamples_GoEngine builds and runs every corpus example
// through the Go front end and asserts the pinned golden stdout.
func TestPhase114_CorpusExamples_GoEngine(t *testing.T) {
	skipIfNoCompiler(t)
	for rel, spec := range phase114Examples {
		t.Run(rel, func(t *testing.T) {
			got := phase114RunExample(t, "go", rel)
			if got != spec.golden {
				t.Fatalf("Go engine stdout = %q, want pinned golden %q", got, spec.golden)
			}
		})
	}
}

// TestPhase114_CorpusExamples_KCCParity runs every both-engine example through
// the self-hosted kcc engine, asserts byte-identical stdout with the Go leg,
// and pins the shared golden. Experimental (Go-only) examples are excluded.
func TestPhase114_CorpusExamples_KCCParity(t *testing.T) {
	skipIfNoCompiler(t)
	for rel, spec := range phase114Examples {
		if spec.engine == "go" {
			continue
		}
		t.Run(rel, func(t *testing.T) {
			// Two independent clips: the callee retries transient gcc failures,
			// so a parity mismatch here means a real engine divergence.
			goOut := phase114RunExample(t, "go", rel)
			kccOut := phase114RunExample(t, "kcc", rel)
			if kccOut != spec.golden {
				t.Fatalf("kcc stdout = %q, want pinned golden %q", kccOut, spec.golden)
			}
			if kccOut != goOut {
				t.Fatalf("engine parity mismatch:\n-- go --\n%q\n-- kcc --\n%q", goOut, kccOut)
			}
		})
	}
}

// TestPhase114_CorpusTestFiles runs the *_test.kark corpus files through the
// real karkain test runner (threading the same phase96/109 machinery) and
// asserts all tests pass.
func TestPhase114_CorpusTestFiles(t *testing.T) {
	skipIfNoCompiler(t)
	for _, rel := range phase114TestFiles {
		t.Run(rel, func(t *testing.T) {
			path := filepath.Join(repoRoot(t), "examples", rel)
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("test file missing: %s (%v)", path, err)
			}
			res := KCCTestCommand(nil, path, "")
			if res.ExitCode != ExitSuccess {
				t.Fatalf("karkain test %s: %q (exit %d)", rel, res.Message, res.ExitCode)
			}
		})
	}
}

// TestPhase114_CorpusCoverage guards the inventory: every pinned example file
// must exist on disk, and the coverage must be complete (no silent drop).
func TestPhase114_CorpusCoverage(t *testing.T) {
	root := filepath.Join(repoRoot(t), "examples")
	for rel := range phase114Examples {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("pinned example missing on disk: %s (%v)", rel, err)
		}
	}
	for _, rel := range phase114TestFiles {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("pinned test file missing on disk: %s (%v)", rel, err)
		}
	}
	if len(phase114Examples) != 59 {
		t.Errorf("expected 59 pinned examples, got %d", len(phase114Examples))
	}
}
