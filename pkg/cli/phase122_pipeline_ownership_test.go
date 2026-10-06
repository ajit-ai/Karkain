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
		// verifies exactly 9574 KIR lines. NOTE: the count grew from the
		// Phase 122 baseline 6399 to 6645 (Phase 123's enum-ADT commit
		// extended the compiler sources), then to 6910 (Phase 125A's
		// socket runtime + net builtins extended codegen/sema/checker),
		// then to 7416 (Phase 133's first-class fn surface extended
		// ast/parser/checker/sema/codegen/kir, including the escape
		// checker and wrapper prototypes), and now to 8986 (Phase 137's
		// concurrency parity added +1706 source lines: parser spawn/receive
		// forms, Spawn/Receive AST nodes, checker/sema builtin tables, KIR
		// rendering, the codegen prescan + wrapper/glue emission, and the
		// generated conc_runtime.kark embed), and now to 9574 (Phase 146's
		// generics v1: 146A Go-parser TypeArgs + 146C kcc mono pass,
		// keyword type-arg gate, index-then-brace fix, template skips;
		// verified deterministic 9574/9574 text/verify on both runs),
		// and now to 9642 (increment 151P0's bitwise operators: five new
		// lexer token kinds and their scan arms, the kcc kPrec table, the
		// infix operator mapping, and the binary_op C helper block in both
		// pkg/codegen and the self-hosted codegen emitter; kir verify
		// passes on the new count), and now to 10530 (increment 151C3's PE32+ container writer: the new src/compiler/native_pe.kark module plus its driver arm in main.kark), which itself followed 10264 (increment 151C2's Mach-O PIE container writer: native_macho.kark), which itself followed 10031 (increment 151C's ELF64 container writer: native_elf.kark), which itself followed 9889 (increment 151B's native
		// machine-code encoder: the new src/compiler/native_emit.kark module
		// plus its driver arm in main.kark; kir verify passes on the new
		// count);
		// and now to 10611 (LH-1, kcc `?` propagation parity: the
		// UnaryExpr question-mark lowering in codegen.kark, the expression-level
		// Ok/Err/Some/None cases in parsePrimary, and the Result/Option
		// constructors and accessors in ast.kark; kir verify passes on the new
		// count);
		// and now to 10626 (LH-2, kcc `match` arm binding parity: the
		// arm-local binding scope helper ckCheckArmBody in checker.kark, used
		// by both Match branches; kir verify passes on the new count);
		// and now to 10838 (151A Step 1, the native value/frame foundation:
		// the new src/compiler/native_value.kark module, the frame and
		// slot-access primitives plus selector-table entries added to
		// native_emit.kark, and the three subcommand arms in main.kark;
		// kir verify passes on the new count);
		// and now to 11040 (151A Step 2, integer statement/expression
		// lowering in kcc: the int lowering and comparison corpora plus
		// natIntIf and the selector-table entries in native_emit.kark, the
		// natIntBody/natIntCond helpers in native_value.kark, and the
		// native-value-int / native-value-cmp / native-value-refuse arms
		// in main.kark; kir verify passes on the new count);
		// and now to 11069 (151A Step 2b, int reassignment in kcc: the two
		// new natIntBody shapes for a reassigned local read bare and read as
		// a binary operand, the two additional ?-propagation refusals, and
		// the corpus/refusal-count arms in main.kark; kir verify passes on
		// the new count);
		// and now to 11373 (151A Step 3, control-flow lowering in kcc: the
		// ten loop reference programs, the natLoopCondLocal/natLoopFinishCmp/
		// natLoopAssignAddLit helpers, natLoopLabels (reported as its own
		// surface because a rel32 carries only a displacement), and the
		// native-value-loop / native-value-loop-labels arms in main.kark;
		// kir verify passes on the new count);
		// and now to 11377 (LH-2 bare match-arm body checking: ckCheckArmBody
		// now dispatches on the arm-body node shape rather than accepting only
		// Block, so the ExprStmt and Print bodies codegen already supports are
		// semantically checked too. +4, not the +2 code lines the diff shows:
		// KIR renders `else if` as a nested If inside the else field, which
		// costs more than one line. Measured, not predicted. kir verify passes
		// on the new count);
		// and now to 11412 (the `_start` entry stub and the Linux exit tail
		// in kcc: natSyscall plus its two selector-table entries in
		// native_emit.kark, natRegRDI/natStartStub/natStartCorpus in
		// native_value.kark, and the native-value-start arm in main.kark.
		// Measured with `karkain kir --verify src/compiler/kir.kark`
		// (11412 text / 11412 verify), not predicted; kir verify passes on
		// the new count).
		// the pin is refreshed to the validated current value.
		// and now to 11756 (increment 151A Step 5/6: the Windows entry
		// (`and rsp, -16`), the loader-independent PEB bootstrap and its
		// three export resolves, the Win64 exit tail and the native-value-win
		// dispatch, plus the fourteen emitter primitives and the Step-6 PE
		// composition helpers). Measured with `karkain kir --verify
		// src/compiler/kir.kark` (11756 text / 11756 verify), NOT predicted --
		// `kir verify` passes on the new count, so the growth is the new code
		// rendering, not a structural break. The +344 delta is consistent with
		// the functions and statements added; comments do not emit KIR.
		// and now to 11859 (increment 151A Step 7: int `print` in kcc --
		// natCqo/natDivReg/natStoreMem8/natPushReg/natPopReg/natJns plus their
		// six natMask selector entries in native_emit.kark, and natSysWrite/
		// natRodataRef/natWriteStdout/natPrintNewline/natPrintIntHelper/
		// natPrintIntCorpus plus the native-value-print dispatch in main.kark).
		// Measured with `karkain kir --verify src/compiler/kir.kark`
		// (11859 text / 11859 verify), NOT predicted.
		//
		// The +103 is Step 7's delta ALONE, and it was measured rather than
		// inferred: with the three Step-7 files reverted to their HEAD content
		// in a scratch copy of the tree, the same command reports 11412 --
		// which is exactly HEAD's own pin, so the baseline is real rather than
		// assumed -- and 11859 - 11412 = 447, of which 11756 - 11412 = 344 is
		// Steps 5/6, leaving 103 for Step 7. `kir verify` passes on the new
		// count, so the growth is the new code rendering, not a break.
		//
		// Recorded because this was nearly misread as a regression: an earlier
		// run of the Step 5 gate failed here with `karkain kir --verify`
		// exiting 3 and empty output, and direct `kcc verifykir` exited
		// 0xC0000005 on check/kir/verifykir alike. That was the documented
		// ~4 GB host low-RAM class, NOT the new code: with free RAM above
		// ~1.7 GB the identical command passes at 11859, and the all-HEAD
		// scratch tree passed at 473 MB free. The crash did not reproduce on
		// the working tree once memory was available, and no code was changed
		// in response to it.
		// and now to 11950 (increment 151A Step 8a: the SSE2 scalar-double
		// primitives in kcc -- natAddsdXmmXmm, natSubsdXmmXmm, natMulsdXmmXmm,
		// natDivsdXmmXmm, natUcomisdXmmXmm, natCvtsi2sdXmmGp,
		// natCvttsd2siGpXmm, natXorpdXmmXmm and natFloatCorpus in
		// native_emit.kark, plus ten natMask selectors 53-62 and the
		// native-value-float arm in main.kark). Measured with `karkain kir
		// --verify src/compiler/kir.kark` (11950 text / 11950 verify), NOT
		// predicted: 11950 - 11859 = 91, and `kir verify` passes on the new
		// count, so the growth is the new code rendering and not a break.
		// and now to 12163 (increment 151A Step 8b: print_float in kcc --
		// natMovRegImm64, natAndRegReg, natShrRegImm, natLeaRegStack,
		// natMovzxRegMem8, natStoreMem8Off, natMovXmmRegGp and natMovGpRegXmm
		// plus six natMask selectors 63-68 in native_emit.kark, and the
		// natPrintFloatHelper itself in native_value.kark). Measured with
		// `karkain kir --verify src/compiler/kir.kark` (12163 text / 12163
		// verify), NOT predicted: 12163 - 11950 = 213, and `kir verify` passes
		// on the new count.
		// and now to 12302 (increment 151A Step 8c: float statement lowering in
		// kcc -- natJp, natJbe and natJb plus six natMask selectors 69-71 in
		// native_emit.kark, and natFloatLit/natFloatLet/natFloatLocal/
		// natFloatBinary/natFloatDiv/natFloatNeg/natFloatCond/natFloatStmtCorpus
		// in native_value.kark). Measured with `karkain kir --verify
		// src/compiler/kir.kark` (12302 text / 12302 verify), NOT predicted:
		// 12302 - 12163 = 139.
		// and now to 12419 (increment 151A Steps 8d and 8e: the call ABI and
		// floats through it -- natArgTemp, natCallArgsN, natCallArgsExtras,
		// natPrologueArgsN, natPrologueExtras, natArgReg and the seven-arm
		// natCallCorpus in 8d (+87), then natFloatArgCall, natFloatParamHome,
		// natFloatReturn and the four-arm natFloatCallCorpus in 8e (+30)).
		// Measured with `karkain kir --verify src/compiler/kir.kark` (12419 text
		// / 12419 verify), NOT predicted: 12389 + 30 = 12419.
		//
		// Neither step moved a frame displacement. Step 8d was expected to --
		// an earlier reading claimed the per-arg spill was a missing frame
		// region requiring a re-pin of every frame number, which was WRONG: the
		// oracle computes argTemp(i) = frame - argSpillBytes + i*16, addressing
		// downward from the frame size, and Step 1 had already reserved it.
		kirSrc := filepath.Join(root, "src", "compiler", "kir.kark")
		out, err := runBin(t, bin, root, "kir", "--verify", kirSrc)
		if err != nil {
			t.Fatalf("kir --verify on compiler source failed: %v\n%s", err, out)
		}
		if got := phase121Count(t, out, "[ok] kir text: "); got != 12419 {
			t.Errorf("whole-tree kir text = %d lines, want 12419:\n%s", got, out)
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
