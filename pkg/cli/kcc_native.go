package cli

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"karkain/pkg/codegen"
	"karkain/pkg/native"
)

// Phase 151 — the kcc native seam.
//
// Increment 150 gave the C-free native targets (native-x86_64-{linux,windows,
// macos}) a complete Go backend, and made kcc the DEFAULT engine for everything
// else. That combination produced a real dishonesty, measured in
// docs/audit/PHASE-151-BASELINE.md §1.1:
//
//	KARKAIN_ENGINE=kcc karkain build hello.kark --target native-x86_64-linux
//	=== [Verbose] Lexer Token Stream ===          <- GO lexer, not kcc
//	=== [native-x86_64-linux] BUILD ... ===      <- Go dispatch banner
//	native image: 351 bytes
//
// The kcc run and the go run produce the SAME SHA-256 (237f5f1a...4b13). The
// user asked for the self-hosted engine and silently got the Go one, with no
// error, no warning, and an image that looks like proof of parity it cannot
// provide. `cmd/karkain/main.go` implemented that hand-off with a plain
// `else { result = cli.BuildCommand(...) }`.
//
// This file is the place that hand-off is replaced. The contract from the
// 151 baseline is explicit:
//
//	"No silent Go fallback. If kcc cannot lower a construct it must be a loud
//	 refusal naming the construct, never a quiet hand-off to the Go engine.
//	 An invisible fallback is worse than an error."
//
// So the seam below is a REFUSAL today and becomes a DISPATCH in 151A. The
// switch is one boolean so the transition is a single reviewed edit rather than
// a hunt through the dispatch tree, and so `kccProducedImage` in
// phase151_parity_test.go has exactly one place to learn about.

// Phase 151A-1 changed WHERE the refusal comes from, which is the whole point
// of this increment. 151D-first replaced the hand-off with a refusal SYNTHESISED
// HERE, in Go, while kcc was still never consulted: that removed the
// image-without-provenance dishonesty, but `kccProducedImage` could not become
// true, because the marker was a string Go chose to print. Now the request is
// staged and run THROUGH kcc, and kcc itself parses, type-checks and answers it
// (see `buildNativeFile` in src/compiler/main.kark). The refusal and the
// provenance line both come from the self-hosted engine, which is what turns
// provenance from decoration into evidence.
//
// HONEST BOUNDARY. kcc still has no machine-code codegen — there is no
// elf/pe/macho/x86 token in src/compiler/*.kark — so it cannot produce an image
// and still refuses. This file deliberately does NOT reach into pkg/native to
// take Go bytes and present them as kcc's; that would be the original fallacy
// with an extra process hop. Real emission is 151A/B/C, scoped in
// docs/audit/PHASE-151A-BASELINE.md.

// kccOwnsNativeTargets reports whether the self-hosted engine emits C-free
// native images.
//
// Phase 151D: TRUE, but for ONE target. kcc emits a real user program only as
// native-x86_64-windows, through natNativeProgram -- the same driver
// `karkain native-ast` drives and the 9c/9e/9f/9g/9h gates prove. kcc has no
// ELF or Mach-O emitter for a user program, so those two targets are refused by
// kcc with K116 and remain Go-backend-only. The name says "targets" plural
// because the seam is target-scoped, but the evidence is single-target: do not
// read this constant as a claim about all three.
const kccOwnsNativeTargets = true

// kccNativeOwnedTargets lists the C-free targets kcc can emit for a real user
// program. Phase 151D: Windows ONLY, through natNativeProgram — the driver
// `karkain native-ast` drives and the 9c/9e/9f/9g/9h gates prove.
//
// kcc has no ELF or Mach-O emitter for a user program: native_elf.kark and
// native_macho.kark lower the FIXED reference corpora that 151C/151C2 compare
// against the Go oracle, and neither takes a program. Those two targets
// therefore keep refusing with K116 and remain Go-backend-only.
//
// This is a named list rather than an inline target comparison so the ownership
// claim is data the gate can assert on directly, instead of being re-derived
// from each dispatch site where the two copies could drift apart.
var kccNativeOwnedTargets = []string{NativeWindowsTarget}

// kccNativeOwns reports whether kcc emits targetName. False means kcc must
// refuse it — never fall back to the Go engine for it.
func kccNativeOwns(targetName string) bool {
	for _, t := range kccNativeOwnedTargets {
		if t == targetName {
			return true
		}
	}
	return false
}

// kccNativeProvenance is the marker `buildNativeFile` prints from inside
// src/compiler/main.kark. The Go fallback cannot produce it, so its presence
// proves the self-hosted engine handled the request. Declared on both sides so
// the emitter and the gate are pinned to one spelling.
const kccNativeProvenance = "[kcc] native request"

// kccNativeImageMarker is the second line kcc prints when it actually has bytes
// to hand over, followed by the image hex. Provenance alone proves kcc was
// consulted; this marker is what proves kcc EMITTED, so the two facts stay
// separable and neither can stand in for the other.
const kccNativeImageMarker = "[kcc] native image "

// KCCNativeBuildCommand is the kcc half of `build` for a C-free native target.
//
// While kccOwnsNativeTargets is false the outcome is still a refusal (ExitEnv):
// the caller asked for a backend that does not exist yet, and inventing an image
// would be the exact failure this seam exists to remove. What changed in 151A-1
// is that the refusal is kcc's, not ours.
func KCCNativeBuildCommand(targetFile, outputPath string, cfg codegen.Config, verbose bool) CommandResult {
	if kccOwnsNativeTargets {
		return kccNativeBuild(targetFile, outputPath, cfg.Target, verbose)
	}
	return kccNativeConsult(targetFile, cfg.Target, false)
}

// KCCNativeRunCommand is the kcc half of `run` for a C-free native target.
//
// Phase 151D: for the target kcc owns, the image is emitted by kcc and then
// executed here -- running a binary is transport, not compilation, so this is
// not a fallback. For the targets kcc does not own it consults kcc and carries
// back its K116 refusal, exactly as before.
func KCCNativeRunCommand(targetFile string, cfg codegen.Config, verbose bool) CommandResult {
	if !kccOwnsNativeTargets {
		return kccNativeConsult(targetFile, cfg.Target, true)
	}

	target := cfg.Target
	if target == "" {
		target = NativeLinuxTarget
	}
	// A cross-run needs an emulator. The refusal names the fix, matching the
	// Go backend's Phase-111 contract rather than inventing a second one.
	if !nativeHostCanRun(native.OSWindows) {
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf(
			"cannot run a %s binary on %s/%s: cross-run requires an emulator or a remote target; use `karkain build --target %s -o <path>` to build only",
			target, runtime.GOOS, runtime.GOARCH, target)}
	}

	tmp, err := os.CreateTemp("", "karkain-kcc-native-*"+nativeImageExt(native.OSWindows))
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("temp file: %v", err)}
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)

	// kcc emits, this function writes and runs. Any refusal from kcc arrives
	// here as a non-success result and is propagated untouched.
	res := kccNativeBuild(targetFile, tmpName, target, verbose)
	if res.ExitCode != ExitSuccess {
		return res
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("chmod temp: %v", err)}
	}

	cmd := exec.Command(tmpName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		// Phase-100 contract (mirrors the Go backend): a program that fails at
		// runtime is a program failure, not a compilation failure.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Execution Error: %v", err)}
		}
		return CommandResult{ExitCode: classifyCompileError(err), Message: fmt.Sprintf("Execution Error: %v", err)}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: ""}
}

// kccNativeBuild is the Phase 151D true arm: kcc emits the image, and this
// layer does the transport -- staging, validating provenance, decoding the hex
// kcc printed, and writing it where the user asked. It does NOT compile
// anything itself. There is no call into cfreeBuildForOS and no path by which
// a Go-emitted image could reach the filesystem labelled as a kcc build.
//
// The outcome space is deliberately fine-grained, because "the build failed"
// is not actionable:
//
//	(1) kcc refused (unsupported target, or a program it cannot lower)
//	    -> ExitEnv / ExitCompile carrying kcc's own diagnostic
//	(2) kcc emitted bytes with provenance  -> write them, ExitSuccess
//	(3) kcc emitted bytes WITHOUT provenance -> refuse, write NOTHING
//	(4) kcc emitted provenance but no bytes -> refuse, write NOTHING
//
// (3) is the load-bearing case. An image with no provenance cannot be
// distinguished from Go output, so accepting it would recreate exactly the
// dishonesty this file exists to delete -- and it would be the *quiet* version,
// because the bytes would look right. TestPhase151D_* mutation-tests it.
func kccNativeBuild(targetFile, outputPath, targetName string, verbose bool) CommandResult {
	target := targetName
	if target == "" {
		target = NativeLinuxTarget
	}

	// A target kcc does not own is answered by kcc itself, so the refusal and
	// its provenance come from the engine that actually declined. This is what
	// keeps an unsupported target from ever reaching the emission code below.
	if !kccNativeOwns(target) {
		return kccNativeConsult(targetFile, target, false)
	}

	bin, err := kccBinaryPath(nil)
	if err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf(
			"cannot build --target %s with the self-hosted engine: %v\n"+
				"  kcc must be reachable to emit a native image. This is deliberately NOT a K116\n"+
				"  refusal: kcc was never asked, so this is a toolchain problem, not a missing\n"+
				"  backend. Check KARKAIN_KCC, or build the self-hosted engine.", target, err)}
	}

	sandbox, err := os.MkdirTemp("", "karkain-kcc-native-")
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("kcc native sandbox: %v", err)}
	}
	defer os.RemoveAll(sandbox)

	staged, err := kccStageInput(targetFile, sandbox)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("kcc native staging: %v", err)}
	}

	out, runErr := kccNativeInvoke(bin, staged, target, false)
	if out == "" {
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf(
			"the self-hosted engine produced no answer for --target %s (kcc exited: %v).\n"+
				"  Expected it to state its native provenance and then either emit or refuse.", target, runErr)}
	}

	// (1a) kcc refused the target outright. Its words, its code; this layer
	// only picks the CLI exit status.
	if strings.Contains(out, "error[K116]") {
		return CommandResult{ExitCode: ExitEnv, Message: out}
	}
	// (1b) the program was rejected on its own terms, before any backend
	// question arises. A type error must not be reported as a missing backend.
	if strings.Contains(out, "error[K1") {
		return CommandResult{ExitCode: ExitCompile, Message: out}
	}
	if strings.Contains(out, "error[K145]") {
		return CommandResult{ExitCode: ExitCompile, Message: out}
	}

	// (4) provenance is present but there are no bytes.
	img, refusal := kccNativeAcceptOutput(out, target)
	if refusal != nil {
		return *refusal
	}

	outPath, err := kccNativeOutputPath(targetFile, outputPath, target)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	if err := os.WriteFile(outPath, img, 0o755); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("write image: %v", err)}
	}
	// os.WriteFile does not chmod an existing file, and WriteFile preserves the
	// mode of a file it overwrites: the explicit Chmod is what keeps a rebuilt
	// image runnable on POSIX hosts.
	if err := os.Chmod(outPath, 0o755); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("chmod image: %v", err)}
	}

	if verbose {
		fmt.Printf("=== [%s] BUILD (kcc, no C compiler) ===\n", target)
		fmt.Printf("kcc provenance: %s\n", kccNativeProvenance)
		fmt.Printf("kcc image: %d bytes -> %s\n", len(img), outPath)
	}
	// Both attribution lines travel WITH the answer, not only through
	// --verbose. They are the evidence that these bytes are kcc's -- the first
	// that kcc handled the request, the second that kcc EMITTED -- so hiding
	// them behind a flag would make the ownership claim uncheckable by anyone
	// reading the output.
	return CommandResult{ExitCode: ExitSuccess, Message: kccNativeAttribution(out, target) + "\n" + outPath}
}

// kccNativeAttribution lifts kcc's own provenance and image lines out of its
// answer so the CLI can echo them. They are lifted rather than reconstructed,
// because a reconstructed string would satisfy a grep for the marker without
// kcc having printed anything -- the same self-defeating assertion this file
// documents for the diagnostics above.
func kccNativeAttribution(out, target string) string {
	var keep []string
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, kccNativeProvenance+" "+target) ||
			strings.Contains(ln, kccNativeImageMarker) {
			keep = append(keep, strings.TrimSpace(ln))
		}
	}
	if len(keep) == 0 {
		return kccNativeProvenance + " " + target
	}
	return strings.Join(keep, "\n")
}

// kccNativeAcceptOutput turns kcc's answer into image bytes, or refuses.
//
// It is a PURE function of kcc's output -- no process, no filesystem -- so the
// provenance gate can be tested directly rather than only through an end-to-end
// run that needs a real kcc binary. Every rejection path here must leave the
// caller with nothing to write.
//
// The order is deliberate: provenance is checked BEFORE any hex is decoded, so
// bytes that cannot be attributed to kcc are never even parsed, let alone
// accepted. The reverse order would make the gate decorative.
func kccNativeAcceptOutput(out, target string) ([]byte, *CommandResult) {
	// PROVENANCE GATE. Without it the bytes cannot be accepted as kcc's.
	//
	// This message must NOT contain the provenance marker in any form,
	// including a quoted copy: the gate proves provenance by searching the
	// output, so a diagnostic quoting it would satisfy its own check.
	if !strings.Contains(out, kccNativeProvenance) {
		return nil, &CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf(
			"the self-hosted engine emitted bytes for --target %s WITHOUT its provenance line:\n%s\n"+
				"  Bytes that cannot be attributed to kcc cannot be told apart from Go-backend\n"+
				"  output, so they are refused rather than written. No image was produced.\n"+
				"  If kcc really emitted this, the self-hosted driver lost its marker; if it did\n"+
				"  not, something upstream fell back silently and this is the seam catching it.",
			target, strings.TrimRight(out, "\n"))}
	}

	// provenance is present but there are no bytes.
	hexImg, ok := kccNativeHexFromOutput(out)
	if !ok {
		return nil, &CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf(
			"the self-hosted engine claimed the %s request but emitted no image bytes for it.\n"+
				"  Its output was:\n%s\n  No image was produced.", target, strings.TrimRight(out, "\n"))}
	}

	img, err := hex.DecodeString(hexImg)
	if err != nil {
		return nil, &CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf(
			"the self-hosted engine emitted a malformed image for --target %s: %v", target, err)}
	}
	// Structural validation with the ORACLE's own parser, so a kcc that emits
	// well-formed hex but a broken container is caught here rather than by the
	// user's loader.
	if _, _, err := native.ParsePE(img); err != nil {
		return nil, &CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf(
			"the self-hosted engine emitted a structurally invalid image for --target %s: %v", target, err)}
	}
	return img, nil
}

// kccNativeHexFromOutput pulls the image hex out of kcc's answer.
//
// The contract is positional, not a substring search for "something that looks
// like hex": the image is the first non-empty line AFTER the image marker. A
// grep for any long hex run would happily match a hex string inside a
// diagnostic, which is how a refusal could be read as an image.
func kccNativeHexFromOutput(out string) (string, bool) {
	lines := strings.Split(out, "\n")
	seen := false
	for _, ln := range lines {
		ln = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(ln, "\r"), "\n"))
		if !seen {
			if strings.Contains(ln, kccNativeImageMarker) {
				seen = true
			}
			continue
		}
		if ln == "" {
			continue
		}
		return ln, true
	}
	return "", false
}

// kccNativeOutputPath resolves where the kcc image is written. With no -o it
// follows the same <source dir>/build/<name><ext> convention as the Go backend
// so the two engines place output identically and `--target` is the only
// visible difference.
func kccNativeOutputPath(targetFile, outputPath, target string) (string, error) {
	if outputPath != "" {
		return outputPath, nil
	}
	base := strings.TrimSuffix(targetFile, filepath.Ext(targetFile))
	buildDir := filepath.Join(filepath.Dir(targetFile), "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return "", fmt.Errorf("create build dir: %v", err)
	}
	return filepath.Join(buildDir, filepath.Base(base)+nativeImageExt(native.OSWindows)), nil
}

// kccNativeConsult stages the real project for the self-hosted engine, runs kcc
// on it with the native target, and returns kcc's OWN answer.
//
// It uses the user's program, not a stand-in: a refusal about a program kcc
// never read would be a claim kcc cannot support. The project is staged into a
// sandbox by the existing kccStageInput so no artifact lands beside the source.
//
// Three outcomes are kept DISTINCT, because they are three different problems
// with three different fixes:
//
//	(1) the program does not type-check -> ExitCompile, kcc's own diagnostics
//	(2) kcc ran and refused            -> ExitEnv, kcc's provenance + K116
//	(3) kcc never answered             -> ExitEnv, reported as a tool failure
//
// (3) is deliberately NOT a K116: kcc did not refuse, it was never reached, so
// reporting a backend-capability code for it would be a false explanation.
func kccNativeConsult(targetFile, targetName string, isRun bool) CommandResult {
	target := targetName
	if target == "" {
		target = NativeLinuxTarget
	}

	bin, err := kccBinaryPath(nil)
	if err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf(
			"cannot consult the self-hosted engine for --target %s: %v\n"+
				"  kcc must be reachable to answer a native request. This is deliberately NOT a K116\n"+
				"  refusal: kcc was never asked, so this is a toolchain problem, not a missing\n"+
				"  backend. Check KARKAIN_KCC, or build the self-hosted engine.", target, err)}
	}

	sandbox, err := os.MkdirTemp("", "karkain-kcc-native-")
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("kcc native sandbox: %v", err)}
	}
	defer os.RemoveAll(sandbox)

	staged, err := kccStageInput(targetFile, sandbox)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("kcc native staging: %v", err)}
	}

	out, runErr := kccNativeInvoke(bin, staged, target, isRun)
	if out == "" {
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf(
			"the self-hosted engine produced no answer for --target %s (kcc exited: %v).\n"+
				"  Expected it to state its native provenance and then refuse with K116.", target, runErr)}
	}

	// (1) The program is rejected on its own terms, before any backend
	// question arises. A type error must not be reported as a missing backend.
	if strings.Contains(out, "error[K1") && !strings.Contains(out, "K116") {
		return CommandResult{ExitCode: ExitCompile, Message: out}
	}

	// (3) kcc produced output but never claimed the request, so its own
	// provenance line is missing. Report that rather than assuming a refusal.
	//
	// IMPORTANT: this message must NOT contain the provenance marker text, in
	// any form including a quoted copy. The gate proves provenance by searching
	// the output for that marker, so a diagnostic quoting it would satisfy its
	// own check — a self-defeating assertion that let a mutation which removed
	// kcc's provenance line pass this gate. It is described in words only.
	if !strings.Contains(out, kccNativeProvenance) {
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf(
			"the self-hosted engine did not answer the native request for --target %s:\n%s\n"+
				"  kcc is expected to print a provenance line identifying the self-hosted engine,\n"+
				"  followed by a K116 refusal. Neither was present, so no capability claim is\n"+
				"  being made on its behalf.",
			target, strings.TrimRight(out, "\n"))}
	}

	// (2) kcc's own answer. Its words, its exit semantics; this layer only
	// carries them and picks the CLI exit code.
	return CommandResult{ExitCode: ExitEnv, Message: out}
}

// kccNativeInvoke runs `kcc build|run <staged> --target <target>` and returns the
// combined output. The kcc engine has no exit primitive of its own, so the exit
// status is deliberately NOT used to classify the outcome — only the output is.
func kccNativeInvoke(bin, staged, target string, isRun bool) (string, error) {
	verb := "build"
	if isRun {
		verb = "run"
	}
	cmd := exec.Command(bin, verb, staged, "--target", target)
	// Force the staged sandbox as the working directory so a kcc that writes a
	// file cannot touch the user's tree.
	if dir := filepath.Dir(staged); dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
