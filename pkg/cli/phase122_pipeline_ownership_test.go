package cli

// Phase 122 — Compiler Pipeline Ownership gate.
//
// Phase 122 moves project source assembly into the self-hosted compiler:
// src/compiler/main.kark gains assembleProject plus its helpers (marker-gated
// standard-library discovery, sibling join, module-import stripping) and every
// driver entry (check/build/run/kir/verifykir) compiles through it, so kcc
// composes its OWN input on the real CLI path for flat projects (no manifest,
// imports are std.* / sibling modules). The Go CLI stages flat projects into a
// sandbox (root + siblings + the stdlib tree at lib/) and keeps the legacy
// module-aware assembly for manifests/workspaces.
//
// Success criterion (baseline report): the default kcc path assembles
// std.*-importing projects (stdlib_v2) and sibling-importing projects
// (module_system) without any Go-side source concatenation, with byte-identical
// output, while the whole-tree KIR invariant (6645 lines) and the Phase 121
// self-verification hook survive unchanged.
//
// Regression covered: standard-library discovery is MARKER-GATED (a candidate
// stdlib/ root must carry the canonical std.string module). Without the gate,
// the walk-up from examples/stdlib_v2 picks the demo tree examples/stdlib
// (collections demo with its own func main) instead of the real repository
// stdlib. The StdlibShadowMarker subtest proves a shadowed fake stdlib is
// never adopted.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// normNL normalizes Windows \r\n line endings to \n so assertions work on both
// hosts regardless of which C runtime produced the output.
func normNL(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// stdlibV2Golden is the exact stdout of examples/stdlib_v2/main.kark on both
// engines (verified byte-identical Go and kcc).
const stdlibV2Golden = "hello KARKAIN\n2\nalpha\nbeta\n9\n26\ntrue\nroundtrip\n" +
	"00e0cba20c10cac449eb885a9926a4b646f0ac163ed7fbc5704d9d8a057ef44d\n" +
	"0\n1\n2\n3\n4\n5\n6\n7\n8\n9\n"

func TestPhase122_PipelineOwnership(t *testing.T) {
	bin := buildPreviewBinary(t)
	root := repoRoot(t)

	t.Run("CompilerSelfCheck", func(t *testing.T) {
		// The strongest assembly proof: the CLI stages src/compiler as a flat
		// project (10 .kark siblings) and the DEFAULT kcc engine assembles and
		// checks itself end-to-end through the checkFile KIR hook.
		out, err := runBin(t, bin, root, "check", filepath.Join(root, "src", "compiler", "main.kark"))
		if err != nil {
			t.Fatalf("default kcc check of the compiler assembly failed: %v\n%s", err, out)
		}
		if !strings.Contains(out, "[ok]") {
			t.Errorf("compiler self-check must [ok]:\n%s", out)
		}
	})

	t.Run("KIRContinuity", func(t *testing.T) {
		// Phase 121's whole-tree KIR invariant must survive the pipeline
		// change: the assembled compiler tree (sibling join) still emits and
		// verifies exactly 6910 KIR lines. NOTE: the count grew from the
		// Phase 122 baseline 6399 to 6645 (Phase 123's enum-ADT commit
		// extended the compiler sources) and then to 6910 (Phase 125A's
		// socket runtime + net builtins extended codegen/sema/checker); the
		// pin is refreshed to the validated current value.
		kirSrc := filepath.Join(root, "src", "compiler", "kir.kark")
		out, err := runBin(t, bin, root, "kir", "--verify", kirSrc)
		if err != nil {
			t.Fatalf("kir --verify on compiler source failed: %v\n%s", err, out)
		}
		if got := phase121Count(t, out, "[ok] kir text: "); got != 6910 {
			t.Errorf("whole-tree kir text = %d lines, want 6910:\n%s", got, out)
		}
		if tc, vc := phase121Count(t, out, "[ok] kir text: "), phase121Count(t, out, "[ok] kir verify: "); tc != vc {
			t.Errorf("kir.kark count mismatch: %d vs %d\n%s", tc, vc, out)
		}
	})

	t.Run("ModuleSystem", func(t *testing.T) {
		// Bare sibling import through the flat path: kcc's own assembler joins
		// math.kark before main.kark and the qualified calls resolve.
		mainFile := filepath.Join(root, "examples", "module_system", "main.kark")

		out, err := runBin(t, bin, root, "check", mainFile)
		if err != nil {
			t.Fatalf("check module_system failed: %v\n%s", err, out)
		}
		if !strings.Contains(out, "[ok]") {
			t.Errorf("module_system check must [ok]:\n%s", out)
		}

		out, err = runBin(t, bin, root, "run", mainFile)
		if err != nil {
			t.Fatalf("run module_system failed: %v\n%s", err, out)
		}
		if !strings.Contains(normNL(out), "42\n9") {
			t.Errorf("module_system run output wrong:\n%s", out)
		}

		// The KIR proves the sibling join: math.kark's functions assemble
		// BEFORE main.kark's main, exactly one main exists, and the output is
		// byte-deterministic.
		kir1, err := runBin(t, bin, root, "kir", mainFile)
		if err != nil {
			t.Fatalf("kir module_system failed: %v\n%s", err, kir1)
		}
		if !strings.Contains(kir1, "func twice (params: n)") || !strings.Contains(kir1, "func square (params: n)") {
			t.Errorf("kir module_system missing sibling functions:\n%s", kir1)
		}
		if strings.Count(kir1, "\nfunc main (params: )") != 1 {
			t.Errorf("kir module_system must contain exactly one main:\n%s", kir1)
		}
		if strings.Index(kir1, "func twice ") > strings.Index(kir1, "func main (params: )") {
			t.Errorf("sibling funcs must assemble before the root main:\n%s", kir1)
		}
		kir2, err := runBin(t, bin, root, "kir", mainFile)
		if err != nil {
			t.Fatalf("second kir module_system failed: %v\n%s", err, kir2)
		}
		if kir1 != kir2 {
			t.Errorf("module_system KIR is not byte-deterministic:\n--- 1 ---\n%s\n--- 2 ---\n%s", kir1, kir2)
		}
	})

	t.Run("StdlibV2", func(t *testing.T) {
		// Four std.* imports (string, collections, encoding, crypto) resolved
		// entirely by the self-hosted assembler on the default engine path,
		// byte-identical to the Phase 109 Go-engine golden.
		mainFile := filepath.Join(root, "examples", "stdlib_v2", "main.kark")

		out, err := runBin(t, bin, root, "check", mainFile)
		if err != nil {
			t.Fatalf("check stdlib_v2 failed: %v\n%s", err, out)
		}
		if !strings.Contains(out, "[ok]") {
			t.Errorf("stdlib_v2 check must [ok]:\n%s", out)
		}

		out, err = runBin(t, bin, root, "run", mainFile)
		if err != nil {
			t.Fatalf("run stdlib_v2 failed: %v\n%s", err, out)
		}
		if !strings.Contains(normNL(out), normNL(stdlibV2Golden)) {
			t.Errorf("stdlib_v2 run output diverged from golden:\n%s", out)
		}

		// The KIR must contain the real collections module (array_sum) and a
		// single main — not the shadowed demo tree (see StdlibShadowMarker).
		kir, err := runBin(t, bin, root, "kir", mainFile)
		if err != nil {
			t.Fatalf("kir stdlib_v2 failed: %v\n%s", err, kir)
		}
		if !strings.Contains(kir, "func array_sum (params: arr)") {
			t.Errorf("kir stdlib_v2 missing collections module:\n%s", kir)
		}
		if strings.Count(kir, "\nfunc main (params: )") != 1 {
			t.Errorf("kir stdlib_v2 must contain exactly one main:\n%s", kir)
		}
	})

	t.Run("StdlibShadowMarker", func(t *testing.T) {
		// Regression for the findStdlibRoot marker gate: a directory named
		// `stdlib` containing only a fake collections module (with its own func
		// main) must NOT be adopted as the standard library — even when it sits
		// between the project and the real repository stdlib during the walk-up.
		// The real stdlib carries the canonical std.string marker.
		base := filepath.Join(root, ".phase122-shadow")
		os.RemoveAll(base)
		defer os.RemoveAll(base)
		proj := filepath.Join(base, "proj")
		if err := os.MkdirAll(proj, 0o755); err != nil {
			t.Fatal(err)
		}
		mainFile := filepath.Join(proj, "main.kark")
		if err := os.WriteFile(mainFile, []byte("import std.collections\nfunc main() {\n    print(array_max([3, 7, 2]))\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// A fake module in the shadow dir: array_max would return 999 and a
		// duplicate func main appears if the shadow were adopted.
		fakeDir := filepath.Join(proj, "stdlib", "collections")
		if err := os.MkdirAll(fakeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fakeDir, "collections.kark"),
			[]byte("func array_max(arr) {\n    return 999\n}\nfunc main() {\n    print(\"SHADOW-BAD\")\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Direct kcc: the compiler's OWN walk-up must skip proj/stdlib (no
		// string module) and land on the repository stdlib. This is the exact
		// failure mode previously observed for examples/stdlib_v2.
		kcc, err := kccBinaryPath(io.Discard)
		if err != nil {
			t.Fatalf("kccBinaryPath: %v", err)
		}
		if out, code := runKCCDir(kcc, root, "check", mainFile); code != 0 || !strings.Contains(out, "[ok]") {
			t.Errorf("kcc-direct check with shadowed stdlib rejected real module: code=%d\n%s", code, out)
		}
		if out, code := runKCCDir(kcc, root, "run", mainFile); code != 0 || !strings.Contains(out, "7") || strings.Contains(out, "SHADOW-BAD") || strings.Contains(out, "999") {
			t.Errorf("kcc-direct run adopted the shadowed fake module:\n%s", out)
		}
		// CLI path (sandbox staging) agrees.
		if out, err := runBin(t, bin, root, "run", mainFile); err != nil || !strings.Contains(normNL(out), "7\n") {
			t.Errorf("CLI run with shadowed stdlib wrong: %v\n%s", err, out)
		}
	})

	t.Run("MissingModuleRejected", func(t *testing.T) {
		// A std.* import that resolves nowhere must be rejected on the default
		// engine path with a module-not-found message (no [ok] verdict).
		base := filepath.Join(root, ".phase122-neg")
		os.RemoveAll(base)
		defer os.RemoveAll(base)
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatal(err)
		}
		mainFile := filepath.Join(base, "main.kark")
		if err := os.WriteFile(mainFile, []byte("import std.nope\nfunc main() {\n    print(1)\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := runBin(t, bin, root, "check", mainFile)
		if err == nil {
			t.Fatalf("check with unresolvable import must fail, got success:\n%s", out)
		}
		if !strings.Contains(out, "nope") || !strings.Contains(out, "not found") {
			t.Errorf("missing-module rejection should name the module:\n%s", out)
		}
		if strings.Contains(out, "[ok]") {
			t.Errorf("[ok] must never appear for an unresolvable import:\n%s", out)
		}
	})

	t.Run("WiringPresence", func(t *testing.T) {
		// Guards the assembly wiring itself so a regression becomes a hard,
		// visible failure instead of silently returning to Go-side assembly.
		mainSrc := mustRead(t, filepath.Join(root, "src", "compiler", "main.kark"))
		for _, want := range []string{
			"func assembleProject(path)",
			"func findStdlibRoot(rootDir)",
			"func moduleImportNames(source)",
			"func siblingContent(dir, target)",
			"func stripModuleImports(text)",
			"error[K122]",
		} {
			if !strings.Contains(mainSrc, want) {
				t.Errorf("src/compiler/main.kark missing %q", want)
			}
		}
		engSrc := mustRead(t, filepath.Join(root, "pkg", "cli", "kcc_engine.go"))
		for _, want := range []string{
			"func kccStageInput(",
			"func flatAssemblyEligible(",
			"func findKarkainStdlib(",
			"func kccMirrorFlat(",
		} {
			if !strings.Contains(engSrc, want) {
				t.Errorf("pkg/cli/kcc_engine.go missing %q", want)
			}
		}
		kirSrc := mustRead(t, filepath.Join(root, "pkg", "cli", "kir.go"))
		if !strings.Contains(kirSrc, "kccStageInput(") {
			t.Error("pkg/cli/kir.go KIR commands must stage through kccStageInput")
		}
	})
}
