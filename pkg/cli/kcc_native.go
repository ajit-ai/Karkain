package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"karkain/pkg/codegen"
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
// native images. It is false today: 151A-1 gave kcc the request and the
// refusal, but kcc has no machine-code codegen, so it cannot produce an image.
//
// It is a named constant rather than an inline condition so that "kcc owns
// native" is a single fact in the codebase instead of a rule re-derived at each
// dispatch site, where the two of them could drift apart.
const kccOwnsNativeTargets = false

// kccNativeProvenance is the marker `buildNativeFile` prints from inside
// src/compiler/main.kark. The Go fallback cannot produce it, so its presence
// proves the self-hosted engine handled the request. Declared on both sides so
// the emitter and the gate are pinned to one spelling.
const kccNativeProvenance = "[kcc] native request"

// KCCNativeBuildCommand is the kcc half of `build` for a C-free native target.
//
// While kccOwnsNativeTargets is false the outcome is still a refusal (ExitEnv):
// the caller asked for a backend that does not exist yet, and inventing an image
// would be the exact failure this seam exists to remove. What changed in 151A-1
// is that the refusal is kcc's, not ours.
func KCCNativeBuildCommand(targetFile, outputPath string, cfg codegen.Config, verbose bool) CommandResult {
	if kccOwnsNativeTargets {
		// Real emission lands HERE (151A/B/C): kcc lowers the program through
		// its own encoder and container writers and this arm returns the path
		// of the image kcc wrote.
		//
		// This arm is deliberately NOT a call into cfreeBuildForOS. That call
		// would compile and would pass every parity test (the bytes would
		// match!), which is the precise dishonesty this file exists to delete.
		// Flipping the constant without implementing this arm must fail loudly.
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf(
			"error[K116]: internal inconsistency -- kccOwnsNativeTargets is true but kcc machine-code "+
				"emission has not landed, so there is nothing to return for --target %s.\n"+
				"  This is a build-time guard, not a user error: kccOwnsNativeTargets must stay false "+
				"until this arm returns an image produced by kcc.", cfg.Target)}
	}
	return kccNativeConsult(targetFile, cfg.Target, false)
}

// KCCNativeRunCommand is the kcc half of `run` for a C-free native target. Same
// contract as KCCNativeBuildCommand.
func KCCNativeRunCommand(targetFile string, cfg codegen.Config, verbose bool) CommandResult {
	return kccNativeConsult(targetFile, cfg.Target, true)
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
