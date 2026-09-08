package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 99: self-hosted parser + type checker gate.
//
// The self-hosted compiler (kcc, bootstrapped from src/compiler) must now
// understand the current language surface on its own: it parses every program
// in the pinned corpora, its two-pass type checker (types.kark + checker.kark)
// accepts those programs, rejects the type-error fixtures, and — the Phase 99
// self-hosted source gate — the compiler's own assembled source tree type-checks
// clean through the self-hosted pipeline without any Go front-end help.
//
// Each subtest uses the isolated kcc built by phase95KCC and drives it through
// the real KCCCheckCommand engine path (resolveSources assembly → kcc check).
// The fixture collection lives in examples/type_errors/ (one main.kark per
// directory, mirroring the algorithm/probe corpus layout).

// phase99TypeErrorFixtures lists every type-error fixture directory and the
// error[K1XX] code it must produce. The fixture set is the Phase 99
// "type checker catches hand-crafted error cases" gate.
var phase99TypeErrorFixtures = []struct {
	name string
	code string
}{
	{"err01_undefined_call", "K101"},
	{"err02_undefined_call_lambda", "K101"},
	{"err03_undefined_identifier", "K102"},
	{"err04_wrong_arg_count_few", "K103"},
	{"err05_wrong_arg_count_many", "K103"},
	{"err06_builtin_arity_few", "K104"},
	{"err07_builtin_arity_many", "K104"},
	{"err08_struct_literal_unknown_type", "K106"},
	{"err09_duplicate_function", "K107"},
	{"err10_break_outside_loop", "K108"},
	{"err11_continue_outside_loop", "K108"},
	{"err12_call_type_as_function", "K109"},
	{"err13_type_mismatch_annotation", "K112"},
	{"err14_undefined_call_in_loop", "K101"},
}

// phase99Corpus returns the pinned "all example programs" corpus: the
// conformance files plus every examples/algorithms and examples/probes main.kark
// (excluding the phase81 diagnostic probes, which are not standalone programs).
func phase99Corpus(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	var files []string
	confs, err := filepath.Glob(filepath.Join(root, "conformance", "*.kark"))
	if err != nil {
		t.Fatalf("glob conformance: %v", err)
	}
	files = append(files, confs...)
	for _, dir := range []string{
		filepath.Join(root, "examples", "algorithms"),
		filepath.Join(root, "examples", "probes"),
	} {
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && d.Name() == "phase81" {
				return filepath.SkipDir
			}
			if !d.IsDir() && d.Name() == "main.kark" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	return files
}

// isolateKarkFile copies a .kark file into its own fresh temporary directory
// (as main.kark). kcc check runs on the assembled sibling scope, so an isolated
// copy gives the checker exactly one program to see — mirroring the conformance
// harness's self-contained file scope rather than the directory-wide project
// scope used for real projects.
func isolateKarkFile(t *testing.T, path string) string {
	t.Helper()
	dir := t.TempDir()
	dst := filepath.Join(dir, "main.kark")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read corpus file %s: %v", path, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write isolated file: %v", err)
	}
	return dst
}

// TestPhase99_SelfHostedParserAndTypeChecker is the Phase 99 gate. It builds one
// isolated kcc, then runs all sub-gates through KCCCheckCommand:
//
//  1. Parser + type checker accept the full example corpus
//  2. Type checker rejects every hand-crafted error fixture with its code
//  3. Cross-file duplicate definitions are still detected at assembly scope
//  4. The compiler's own assembled source tree parses + type-checks clean
func TestPhase99_SelfHostedParserAndTypeChecker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping self-hosted Phase 99 gate in short mode")
	}
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	hasGCC(t)

	t.Run("CorpusAccept", func(t *testing.T) {
		files := phase99Corpus(t)
		if len(files) < 25 {
			t.Fatalf("corpus too small: %d files, want >= 25 example programs", len(files))
		}
		for _, file := range files {
			file := file
			isolated := isolateKarkFile(t, file)
			var buf bytes.Buffer
			res := KCCCheckCommand(&buf, isolated, false)
			if res.ExitCode != ExitSuccess {
				t.Errorf("kcc check rejected valid program %s (exit %d):\n%s", file, res.ExitCode, strings.TrimSpace(res.Message))
			} else if !strings.Contains(res.Message, "[ok]") {
				t.Errorf("kcc check of %s succeeded without the [ok] marker", file)
			}
		}
	})

	// The self-contained file scope above mirrors the conformance harness
	// (each conformance file is its own unit). The checker must ALSO flag
	// duplicate definitions when files are concatenated at assembly scope,
	// exactly like the Go resolver (resolve.go duplicate-across-files): the
	// conformance directory shares factorial/fib across the whole set, so an
	// assembled check of the directory must fail with K107.
	t.Run("CrossFileDuplicateDetected", func(t *testing.T) {
		root := repoRoot(t)
		first, err := filepath.Glob(filepath.Join(root, "conformance", "*.kark"))
		if err != nil || len(first) == 0 {
			t.Skipf("conformance corpus not found: %v", err)
		}
		var buf bytes.Buffer
		res := KCCCheckCommand(&buf, first[0], false)
		if res.ExitCode != ExitCompile {
			t.Fatalf("assembled conformance directory: want ExitCompile, got %d:\n%s", res.ExitCode, strings.TrimSpace(res.Message))
		}
		if !strings.Contains(res.Message, "K107") {
			t.Fatalf("assembled conformance directory should report K107 duplicate functions:\n%s", strings.TrimSpace(res.Message))
		}
	})

	t.Run("ErrorFixturesRejected", func(t *testing.T) {
		root := repoRoot(t)
		if len(phase99TypeErrorFixtures) < 10 {
			t.Fatalf("too few type-error fixtures: %d, want >= 10", len(phase99TypeErrorFixtures))
		}
		for _, fx := range phase99TypeErrorFixtures {
			fx := fx
			file := filepath.Join(root, "examples", "type_errors", fx.name, "main.kark")
			var buf bytes.Buffer
			res := KCCCheckCommand(&buf, file, false)
			out := res.Message
			if res.ExitCode != ExitCompile {
				t.Errorf("fixture %s: want ExitCompile, got %d:\n%s", fx.name, res.ExitCode, strings.TrimSpace(out))
			}
			if strings.Contains(out, "[ok]") {
				t.Errorf("fixture %s: rejected program still reported as [ok]", fx.name)
			}
			if !strings.Contains(out, fx.code) {
				t.Errorf("fixture %s: output lacks expected error code %s:\n%s", fx.name, fx.code, strings.TrimSpace(out))
			}
		}
	})

	t.Run("CompilerSourcesTypeCheck", func(t *testing.T) {
		root := repoRoot(t)
		compilerMain := filepath.Join(root, "src", "compiler", "main.kark")
		var buf bytes.Buffer
		res := KCCCheckCommand(&buf, compilerMain, false)
		out := res.Message
		if res.ExitCode != ExitSuccess {
			t.Fatalf("self-hosted compiler sources did not parse/type-check through kcc (exit %d):\n%s", res.ExitCode, strings.TrimSpace(out))
		}
		if !strings.Contains(out, "[ok]") {
			t.Fatalf("compiler sources checked without the [ok] marker:\n%s", strings.TrimSpace(out))
		}
	})
}
