package cli

import (
	"fmt"

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

// kccOwnsNativeTargets reports whether the self-hosted engine emits C-free
// native images. It is false today: kcc has no machine-code codegen at all
// (verified in the 151 baseline — no elf/pe/macho/x86 tokens in
// src/compiler/*.kark), it emits C23 only.
//
// It is a named constant rather than an inline condition so that "kcc owns
// native" is a single fact in the codebase instead of a rule re-derived at
// each dispatch site, where the two of them could drift apart.
const kccOwnsNativeTargets = false

// KCCNativeBuildCommand is the kcc half of `build` for a C-free native target.
//
// While kccOwnsNativeTargets is false this is a loud refusal (ExitEnv) rather
// than a fallback: the caller asked for a backend that does not exist, and
// inventing a result would be the exact failure this seam exists to remove.
func KCCNativeBuildCommand(file, outputPath string, cfg codegen.Config, verbose bool) CommandResult {
	if kccOwnsNativeTargets {
		// 151A lands HERE: assemble with kcc, then lower through kcc's own
		// encoder and container writers.
		//
		// This arm is deliberately NOT a call into cfreeBuildForOS. Flipping
		// the constant above without implementing this arm must fail loudly,
		// because the one implementation that would "work" here is the exact
		// silent Go fallback this file exists to delete.
		return CommandResult{ExitCode: ExitEnv, Message: fmt.Sprintf(
			"error[K116]: internal inconsistency -- kccOwnsNativeTargets is true but increment 151A " +
				"(kcc machine-code codegen) has not landed, so there is nothing to dispatch to for --target %s.\n" +
				"  This is a build-time guard, not a user error: kccOwnsNativeTargets must stay false until " +
				"this arm emits through kcc.", cfg.Target)}
	}
	return CommandResult{ExitCode: ExitEnv, Message: kccNativeUnavailableMessage(cfg.Target)}
}

// KCCNativeRunCommand is the kcc half of `run` for a C-free native target. Same
// contract as KCCNativeBuildCommand: loud now, kcc-owned after 151A.
func KCCNativeRunCommand(file string, cfg codegen.Config, verbose bool) CommandResult {
	return CommandResult{ExitCode: ExitEnv, Message: kccNativeUnavailableMessage(cfg.Target)}
}

// kccNativeUnavailableMessage names the code, the engine, and the target, so the
// diagnostic is actionable without reading the source. It deliberately does NOT
// suggest `--engine go` as a "fix" without saying why: routing to the Go engine
// is a different engine, and the user is entitled to know they are leaving kcc.
func kccNativeUnavailableMessage(targetName string) string {
	target := targetName
	if target == "" {
		target = NativeLinuxTarget
	}
	return fmt.Sprintf(
		"error[K116]: the self-hosted engine (kcc) has no machine-code backend, so it cannot emit --target %s.\n"+
			"  kcc currently emits C23 only; the %s C-free targets are implemented by the Go backend alone.\n"+
			"  This is refused rather than silently handed to the Go engine, because a quiet hand-off\n"+
			"  produces an image that looks like kcc output and is not (see docs/audit/PHASE-151-BASELINE.md 1.1).\n"+
			"  To build with the Go engine, ask for it explicitly: --engine go (or KARKAIN_ENGINE=go).\n"+
			"  kcc native codegen is increment 151A.",
		target, target)
}
