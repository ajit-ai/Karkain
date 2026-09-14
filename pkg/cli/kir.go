package cli

import (
	"io"
	"os"
	"strings"
)

// KCCKirCommand routes `karkain kir` through the self-hosted engine. The KIR
// text emitter lives in src/compiler/kir.kark, so this command only ever runs
// on the kcc engine — it is an honest gateway to a compiler-owned component.
// Assembly mirrors KCCCheckCommand: the Go-side project preflight and target
// preflight run first, then kcc sees a single assembled file in a temp
// sandbox. Success is a zero exit plus the KIR text ending in a path-less
// `[ok] kir text: N lines` confirmation, so repeated runs are byte-identical.
func KCCKirCommand(w io.Writer, file string, verbose bool) CommandResult {
	// Phase 105: same Go-side multi-file syntax preflight as check, so every
	// recoverable parse error surfaces with precise per-file spans before any
	// engine runs.
	if errDiags := projectSyntaxDiagnostics(file); len(errDiags) > 0 {
		renderDiagnostics("", errDiags)
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(checkStageSyntax, len(errDiags))}
	}
	bin, err := kccBinaryPath(w)
	if err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: err.Error()}
	}
	prog, err := kccAssembleSource(file)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	if bad, n := kccTargetPreflight(file, prog); bad {
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(checkStageSema, n)}
	}

	sandbox, err := os.MkdirTemp("", "karkain-kcc-kir")
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	defer func() {
		for i := 0; i < 25; i++ {
			if err := os.RemoveAll(sandbox); err == nil {
				return
			}
			os.RemoveAll(sandbox)
		}
	}()
	// Phase 122: flat projects are staged so kcc's own assembler composes the
	// input (identical to the check path); legacy assembly stays a single file.
	kirFile, err := kccStageInput(file, sandbox)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	out, code := runKCC(bin, "kir", kirFile)
	if code == 0 && strings.Contains(out, "[ok] kir text:") {
		// The path-less marker proves byte-determinism: kcc's own output never
		// leaks the temp sandbox location.
		return CommandResult{ExitCode: ExitSuccess, Message: out}
	}
	return CommandResult{ExitCode: ExitCompile, Message: out}
}

// KCCKirVerifyCommand routes `karkain kir --verify <file>` through the
// self-hosted engine (Phase 121). Unlike KCCKirCommand it does not dump the
// KIR text: it runs the compiler-owned structural verifier (kirVerify in
// src/compiler/kir.kark) so the compiler checks its own KIR emission against
// the v1 structural contract. Success is the deterministic count markers; a
// failure prints error[K121] problem lines and no [ok], mapping to ExitCompile.
func KCCKirVerifyCommand(w io.Writer, file string, verbose bool) CommandResult {
	if errDiags := projectSyntaxDiagnostics(file); len(errDiags) > 0 {
		renderDiagnostics("", errDiags)
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(checkStageSyntax, len(errDiags))}
	}
	bin, err := kccBinaryPath(w)
	if err != nil {
		return CommandResult{ExitCode: ExitEnv, Message: err.Error()}
	}
	prog, err := kccAssembleSource(file)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	if bad, n := kccTargetPreflight(file, prog); bad {
		return CommandResult{ExitCode: ExitCompile, Message: checkStageMessage(checkStageSema, n)}
	}

	sandbox, err := os.MkdirTemp("", "karkain-kcc-verifykir")
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	defer func() {
		for i := 0; i < 25; i++ {
			if err := os.RemoveAll(sandbox); err == nil {
				return
			}
			os.RemoveAll(sandbox)
		}
	}()
	// Phase 122: flat projects are staged so kcc's own assembler composes the
	// input (identical to the check path); legacy assembly stays a single file.
	kirFile, err := kccStageInput(file, sandbox)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: err.Error()}
	}
	out, code := runKCC(bin, "verifykir", kirFile)
	if code == 0 && strings.Contains(out, "[ok] kir verify:") {
		return CommandResult{ExitCode: ExitSuccess, Message: out}
	}
	return CommandResult{ExitCode: ExitCompile, Message: out}
}