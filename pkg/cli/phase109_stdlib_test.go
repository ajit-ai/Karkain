package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"karkain/pkg/codegen"
)

// Phase 109 gate: Standard Library v2.
//
// The importable std.* modules (collections, string, io, encoding, crypto)
// must compile and run byte-identically on BOTH engines (Go front end and the
// self-hosted kcc engine) through the normal module-aware toolchain. Gates:
//
//   - every single-module example in examples/stdlib/** runs to a pinned
//     golden output on both engines with byte-identical stdout;
//   - the multi-module E2E (examples/stdlib_v2) builds deterministically and
//     reproduces byte-identical output across both engines;
//   - malformed hex/base64 input raises the same source-located runtime error
//     on both engines (same file and line);
//   - importing a nonexistent stdlib module fails on both engines;
//   - the module-aware assembler actually pulls the stdlib sources into the
//     compiled unit (assembly-level guard).

// phase109StdlibExamples pins the exact stdout each example must produce
// (relative path under examples/ -> golden output).
var phase109StdlibExamples = map[string]string{
	"stdlib/strings/main.kark":    "5\n1\nfoobar\nababab\ntrue\ntrue\n1\n6\n6\nhello\nworld\npadded\npad\npad\nHELLO WORLD\nhello world\na+b+c\ncba\ne\n43\n7\n3\n0\n3\na\nb\nc\nx-y-z\n",
	"stdlib/strings/utf8.kark":    "13\n1\n68c3a96c6c6f2077c3b6726c64\n1\naMOpbGxv\n1\n1\nKARKAIN\nHéLLO WöRLD\n",
	"stdlib/collections/main.kark": "true\n4\n27\n1\n9\n5\n2\n5\n2\n2\n8\n4\n7\n2\ntrue\n2\nthree\n1\n3\n3\n60\n",
	"stdlib/encoding_crypto/main.kark": "68656c6c6f\nhello\naGVsbG8=\nhello\n1\nhi\nba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\nddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f\n",
	"stdlib/io/main.kark": "1\ntrue\nalpha\nbeta\n\n1\n3\nalpha\nbeta\ngamma\ntrue\n0\ntrue\nfalse\n",
}

// phase109MultiModuleGolden is the pinned stdout of examples/stdlib_v2.
const phase109MultiModuleGolden = "hello KARKAIN\n2\nalpha\nbeta\n9\n26\ntrue\nroundtrip\n00e0cba20c10cac449eb885a9926a4b646f0ac163ed7fbc5704d9d8a057ef44d\n0\n1\n2\n3\n4\n5\n6\n7\n8\n9\n"

// phase109RunExample builds the given example through the named engine and
// runs the resulting executable from a clean temp working directory so any
// example-created files never touch the repo. Returns normalized stdout.
// rel is the example's path under examples/, e.g. filepath.Join("stdlib","strings") or
// filepath.Join("stdlib","strings","utf8.kark").
func phase109RunExample(t *testing.T, engine, rel string) string {
	t.Helper()
	root := repoRoot(t)
	src := filepath.Join(root, "examples", rel)
	if !strings.HasSuffix(src, ".kark") {
		src = filepath.Join(src, "main.kark")
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("example missing: %s (%v)", src, err)
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "main.exe")

	var buildOut bytes.Buffer
	switch engine {
	case "go":
		res := BuildCommandIncremental(src, exe, codegen.Config{}, false, filepath.Join(dir, ".cache"))
		if res.ExitCode != ExitSuccess {
			t.Fatalf("Go build %s: %q (exit %d)", rel, res.Message, res.ExitCode)
		}
	case "kcc":
		res := KCCBuildCommand(&buildOut, src, exe, codegen.Config{}, false)
		if res.ExitCode != ExitSuccess {
			t.Fatalf("kcc build %s: %q (exit %d)", rel, res.Message, res.ExitCode)
		}
	default:
		t.Fatalf("unknown engine %q", engine)
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
		t.Fatalf("run %s (%s engine) failed: %v (%s)", rel, engine, err, stderr.String())
	}
	return strings.ReplaceAll(stdout.String(), "\r\n", "\n")
}

// TestPhase109_StdlibModules_RunBothEngines verifies every single-module
// example produces byte-identical stdout on both engines AND matches the
// pinned golden output (NIST vectors for crypto, RFC 4648 for base64/hex).
func TestPhase109_StdlibModules_RunBothEngines(t *testing.T) {
	skipIfNoCompiler(t)

	for name, want := range phase109StdlibExamples {
		t.Run(name, func(t *testing.T) {
			goOut := phase109RunExample(t, "go", name)
			kccOut := phase109RunExample(t, "kcc", name)
			if goOut != kccOut {
				t.Fatalf("engine parity mismatch:\n-- go --\n%q\n-- kcc --\n%q", goOut, kccOut)
			}
			if goOut != want {
				t.Fatalf("stdout = %q, want pinned golden %q", goOut, want)
			}
		})
	}
}

// TestPhase109_StdlibMultiModuleE2E builds examples/stdlib_v2 (which imports
// strings + collections + encoding + crypto at once), asserts byte-identical
// stdout on both engines, and proves the build is deterministic by re-running
// the same executable and by rebuilding it fresh.
func TestPhase109_StdlibMultiModuleE2E(t *testing.T) {
	skipIfNoCompiler(t)

	goOut := phase109RunExample(t, "go", "stdlib_v2")
	kccOut := phase109RunExample(t, "kcc", "stdlib_v2")
	if goOut != kccOut {
		t.Fatalf("multi-module engine parity mismatch:\n-- go --\n%q\n-- kcc --\n%q", goOut, kccOut)
	}
	if goOut != phase109MultiModuleGolden {
		t.Fatalf("multi-module stdout = %q, want pinned golden %q", goOut, phase109MultiModuleGolden)
	}

	// Determinism: re-run the kcc-built executable and re-build it in a fresh
	// sandbox; both must reproduce the same bytes.
	root := repoRoot(t)
	dir := t.TempDir()
	exe := filepath.Join(dir, "main.exe")
	var bout bytes.Buffer
	res := KCCBuildCommand(&bout, filepath.Join(root, "examples", "stdlib_v2", "main.kark"), exe, codegen.Config{}, false)
	if res.ExitCode != ExitSuccess {
		t.Fatalf("kcc rebuild: %q (exit %d)", res.Message, res.ExitCode)
	}
	run := exec.Command(exe)
	run.Dir = dir
	out2, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("rebuild run failed: %v", err)
	}
	if strings.ReplaceAll(string(out2), "\r\n", "\n") != kccOut {
		t.Fatalf("determinism violated: rebuilt run %q != first %q", out2, kccOut)
	}
}

// TestPhase109_StdlibAssemblyPullsModules guards the module-aware assembler:
// the compiled unit for the multi-module example must contain the stdlib
// module sources (not just the entry file), otherwise imports would be hollow.
func TestPhase109_StdlibAssemblyPullsModules(t *testing.T) {
	root := repoRoot(t)
	main := filepath.Join(root, "examples", "stdlib_v2", "main.kark")
	text, err := resolveSourcesRun(main)
	if err != nil {
		t.Fatalf("resolveSourcesRun(stdlib_v2): %v", err)
	}
	for _, marker := range []string{
		"func str_to_upper", // stdlib/string/string.kark
		"func array_sum",    // stdlib/collections/collections.kark
		"func hex_encode",   // stdlib/encoding/encoding.kark
		"func sha256",       // stdlib/crypto/crypto.kark
	} {
		if !strings.Contains(text, marker) {
			t.Errorf("assembled unit missing stdlib marker %q", marker)
		}
	}
}

// TestPhase109_StdlibNegativeParity verifies malformed hex/base64 inputs raise
// the same source-located runtime error on both engines (identical file and
// line), mirroring the Phase 100 parity contract.
func TestPhase109_StdlibNegativeParity(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "examples", "stdlib_errors")
	cases := []struct {
		sub  string
		kind string
	}{
		{"bad_hex", "invalid hex string"},
		{"bad_base64", "invalid base64 string"},
	}

	// Fixtures are single-file (no imports) so both engines assemble identical
	// text and must agree on the reported source location.
	for _, tc := range cases {
		t.Run(tc.sub, func(t *testing.T) {
			fixtureDir := t.TempDir()
			fixture := filepath.Join(fixtureDir, "main.kark")
			data, err := os.ReadFile(filepath.Join(dir, tc.sub, "main.kark"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fixture, data, 0o644); err != nil {
				t.Fatal(err)
			}

			var goOut, goErr bytes.Buffer
			goRes := RunCommand(fixture, codegen.Config{Stdout: &goOut, Stderr: &goErr}, false)
			if goRes.ExitCode != ExitFailure {
				t.Fatalf("Go engine: exit=%d want ExitFailure (out=%q err=%q)", goRes.ExitCode, goOut.String(), goErr.String())
			}
			goDiag := regexp.MustCompile(`runtime error: [^\n]* at ([^:]+):(\d+)`).FindStringSubmatch(goErr.String())
			if goDiag == nil {
				t.Fatalf("Go engine: no source-located diagnostic in %q", goErr.String())
			}
			if !strings.Contains(goErr.String(), tc.kind) {
				t.Errorf("Go engine: diagnostic kind mismatch in %q", goErr.String())
			}

			kccRes := KCCRunCommand(nil, fixture, codegen.Config{}, false)
			if kccRes.ExitCode != ExitFailure {
				t.Fatalf("kcc engine: exit=%d want ExitFailure (msg=%q)", kccRes.ExitCode, kccRes.Message)
			}
			kccDiag := regexp.MustCompile(`runtime error: [^\n]* at ([^:]+):(\d+)`).FindStringSubmatch(kccRes.Message)
			if kccDiag == nil {
				t.Fatalf("kcc engine: no source-located diagnostic in %q", kccRes.Message)
			}
			if !strings.Contains(kccRes.Message, tc.kind) {
				t.Errorf("kcc engine: diagnostic kind mismatch in %q", kccRes.Message)
			}

			if filepath.Base(goDiag[1]) != filepath.Base(kccDiag[1]) {
				t.Errorf("file parity: Go=%q kcc=%q", goDiag[1], kccDiag[1])
			}
			if goDiag[2] != kccDiag[2] {
				t.Errorf("line parity: Go=%s kcc=%s", goDiag[2], kccDiag[2])
			}
		})
	}
}

// TestPhase109_StdlibMissingModule verifies importing a nonexistent stdlib
// module is rejected on both engines (module-error diagnostic, non-success).
func TestPhase109_StdlibMissingModule(t *testing.T) {
	main := filepath.Join(repoRoot(t), "examples", "stdlib_errors", "missing_module", "main.kark")

	var goOut, goErr bytes.Buffer
	goRes := RunCommand(main, codegen.Config{Stdout: &goOut, Stderr: &goErr}, false)
	if goRes.ExitCode == ExitSuccess {
		t.Fatal("Go engine: missing stdlib import unexpectedly succeeded")
	}
	if !strings.Contains(goErr.String()+goRes.Message, "not found") {
		t.Errorf("Go engine: diagnostic does not mention the missing module: %q", goErr.String()+goRes.Message)
	}

	kccRes := KCCRunCommand(nil, main, codegen.Config{}, false)
	if kccRes.ExitCode == ExitSuccess {
		t.Fatal("kcc engine: missing stdlib import unexpectedly succeeded")
	}
	if !strings.Contains(kccRes.Message, "not found") {
		t.Errorf("kcc engine: diagnostic does not mention the missing module: %q", kccRes.Message)
	}
}