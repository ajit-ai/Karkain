package cli

import (
	"errors"
	"strings"

	"karkain/pkg/target"
)

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
	ExitLint    = 7 // lint-specific failure (warnings treated as errors)
)

// validTargets is the closed set of legacy target aliases the toolchain
// accepts. `native` and `c23` share the native write-`.c` path; `wasm32-wasi`
// selects the Karkain-owned WASM backend; `native-link` uses the Phase-84
// object/linker pipeline. Phase 111 adds conventional target triples
// (x86_64-windows, x86_64-linux, aarch64-linux, ...) which are validated
// through the target model in pkg/target rather than this map. Anything else
// is rejected rather than silently falling back to native.
var validTargets = map[string]bool{
	"native":      true,
	"c23":         true,
	"wasm32-wasi": true,
	"native-link": true,
}

// supportedTargetsLine is the deterministic human list appended to target
// diagnostics and shown by `karkain target`.
func supportedTargetsLine() string {
	names := []string{"native, c23, native-link, wasm32-wasi"}
	for _, t := range target.SupportedTargets() {
		names = append(names, t.String())
	}
	return strings.Join(names, ", ")
}

// ValidateTarget checks a `--target` value against the supported set and
// returns a usage-style error for unsupported targets.
func ValidateTarget(value string) error {
	if value == "" {
		return nil
	}
	if validTargets[value] {
		return nil
	}
	if _, err := target.Parse(value); err != nil {
		var pe *target.ParseError
		if errors.As(err, &pe) {
			return &TargetError{Target: value, Reason: pe.Error()}
		}
		return &TargetError{Target: value, Reason: err.Error()}
	}
	return nil
}

// NormalizeTarget canonicalizes a validated --target value: legacy aliases are
// returned unchanged; conventional triples are reduced to the canonical short
// form (x86_64-pc-windows-msvc → x86_64-windows). Errors are usage-style.
func NormalizeTarget(value string) (string, error) {
	if value == "" || validTargets[value] {
		return value, nil
	}
	t, err := target.Parse(value)
	if err != nil {
		var pe *target.ParseError
		if errors.As(err, &pe) {
			return value, &TargetError{Target: value, Reason: pe.Error()}
		}
		return value, &TargetError{Target: value, Reason: err.Error()}
	}
	return t.String(), nil
}

// SelectedTarget resolves a --target value into a target model. The legacy
// aliases `native`, `c23` and `native-link` mean "compile for the host"; a
// recognized triple means that target. The second return value is false when
// the value is not a concrete triple (wasm32-wasi is a triple target handled
// by the dedicated WASM backend).
func SelectedTarget(value string) (target.Target, bool) {
	if value == "" || value == "native" || value == "c23" || value == "native-link" {
		return target.Host(), false
	}
	t, err := target.Parse(value)
	if err != nil {
		return target.Target{}, false
	}
	return t, true
}

// TargetError describes an unsupported --target value.
type TargetError struct {
	Target string
	Reason string
}

func (e *TargetError) Error() string {
	msg := "unsupported target '" + e.Target + "'"
	if e.Reason != "" {
		msg += ": " + e.Reason
	} else {
		msg += " (supported targets: " + supportedTargetsLine() + ")"
	}
	return msg
}

// classifyCompileError maps a codegen error to an exit code: infrastructure
// failures (missing C compiler, broken toolchain, missing cross-linker) are
// ExitEnv; everything else is a compile failure (ExitCompile).
func classifyCompileError(err error) int {
	if err == nil {
		return ExitSuccess
	}
	if strings.Contains(err.Error(), "no supported C compiler") {
		return ExitEnv
	}
	var xe *target.ToolchainError
	if errors.As(err, &xe) {
		return ExitEnv
	}
	return ExitCompile
}
