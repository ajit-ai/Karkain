package cli

import (
	"karkain/pkg/codegen"
	"os"
	"strings"
)

// DebugCommand compiles and runs targetFile with debug tracing enabled
// (Phase 112). Every function enter/leave records a deterministic
// karkain:<file>:enter/leave <func> trace line on stderr while the program's
// normal stdout passes through exactly as `karkain run` would. The trace is
// opt-in — `karkain run`/`karkain build` never instrument the program. The
// self-hosted kcc engine is not yet trace-aware (deferred, explicit boundary
// with no silent fallback); only the Go engine supports tracing.
func DebugCommand(targetFile, engine string, verbose bool) CommandResult {
	if engine != "" && !strings.EqualFold(engine, "go") {
		return CommandResult{ExitCode: ExitFailure,
			Message: "Debug Error: karkain debug supports the Go engine only; the self-hosted kcc engine is not yet trace-aware (deferred, Phase 112 boundary)."}
	}

	if err := ValidateKarFile(targetFile); err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: err.Error()}
	}

	cfg := codegen.Config{Trace: true, Stdout: os.Stdout, Stderr: os.Stderr}
	return RunCommand(targetFile, cfg, verbose)
}