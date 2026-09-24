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
// object/linker pipeline; `native-x86_64-linux` selects the Karkain-owned
// C-free machine-code backend (Phase 145/147, CLI in Phase 148).
// Phase 111 adds conventional target triples
// (x86_64-windows, x86_64-linux, aarch64-linux, ...) which are validated
// through the target model in pkg/target rather than this map. Anything else
// is rejected rather than silently falling back to native.
var validTargets = map[string]bool{
	"native":              true,
	"c23":                 true,
	"wasm32-wasi":         true,
	"native-link":         true,
	"native-x86_64-linux": true,
}

// NativeLinuxTarget is the Phase-148 C-free machine-code target: static
// x86-64 Linux executables with no C compiler anywhere on the path.
// Emission is pure Go (buildable from any host); execution needs
// linux/amd64 (run elsewhere is refused with a build-only hint).
const NativeLinuxTarget = "native-x86_64-linux"

// IsNativeTarget reports whether a --target value selects the C-free
// machine-code backend (as opposed to the C23/gcc pipeline, the Phase-84
// object pipeline, WASM, or a conventional cross triple).
func IsNativeTarget(value string) bool {
	return value == NativeLinuxTarget
}

// supportedTargetsLine is the deterministic human list appended to target
// diagnostics and shown by `karkain target`.
func supportedTargetsLine() string {
	names := []string{"native, c23, native-link, wasm32-wasi, native-x86_64-linux"}
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
// aliases `native`, `c23`, `native-link` and `native-x86_64-linux` mean
// "compile for the host" (the native machine-code target additionally
// enforces linux/amd64 at run time); a recognized triple means that target.
// The second return value is false when the value is not a concrete triple
// (wasm32-wasi is a triple target handled by the dedicated WASM backend).
func SelectedTarget(value string) (target.Target, bool) {
	if value == "" || value == "native" || value == "c23" || value == "native-link" || value == NativeLinuxTarget {
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
