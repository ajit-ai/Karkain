package cli

import "strings"

// Exit codes for the Karkain toolchain (CLI completion §23). One exit code per
// failure class lets scripts and CI branch on the failure kind instead of a
// generic non-zero status.
const (
	ExitSuccess = 0 // command completed successfully
	ExitFailure = 1 // general/program failure (execution, file/system IO)
	ExitUsage   = 2 // CLI usage error (unknown flag/command, missing argument, invalid target/extension)
	ExitCompile = 3 // lexer/parser/sema/borrow/type/codegen failure
	ExitTest    = 4 // test failure
	ExitPackage = 5 // package/dependency failure
	ExitEnv     = 6 // infrastructure/toolchain failure (e.g. no usable C compiler)
)

// validTargets is the closed set of targets the Go toolchain can actually
// produce output for today. `native` and `c23` share the native write-`.c`
// path; `wasm32-wasi` selects the clang WASI cross-compiler. Anything else is
// rejected rather than silently falling back to native.
var validTargets = map[string]bool{
	"native":      true,
	"c23":         true,
	"wasm32-wasi": true,
}

// ValidateTarget checks a `--target` value against the supported set and
// returns a usage-style error for unsupported targets.
func ValidateTarget(target string) error {
	if target == "" {
		return nil
	}
	if !validTargets[target] {
		return &TargetError{Target: target}
	}
	return nil
}

// TargetError describes an unsupported --target value.
type TargetError struct {
	Target string
}

func (e *TargetError) Error() string {
	return "unsupported target '" + e.Target + "' (supported targets: native, c23, wasm32-wasi)"
}

// classifyCompileError maps a codegen error to an exit code: infrastructure
// failures (missing C compiler, broken toolchain) are ExitEnv; everything else
// is a compile failure (ExitCompile).
func classifyCompileError(err error) int {
	if err != nil && strings.Contains(err.Error(), "no supported C compiler") {
		return ExitEnv
	}
	return ExitCompile
}
