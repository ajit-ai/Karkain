package cli

import (
	"fmt"
	"sort"

	"karkain/pkg/codegen"
	"karkain/pkg/target"
)

// sortedTargets returns the supported --target values in deterministic order:
// the legacy aliases first, then the conventional triples.
func sortedTargets() []string {
	out := make([]string, 0, len(validTargets))
	for t := range validTargets {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// TargetCommand implements `karkain target`: it lists the host platform, the
// closed set of targets the toolchain can genuinely produce output for today,
// and the default used when --target is omitted.
func TargetCommand() CommandResult {
	fmt.Println("Host: " + target.Host().String())
	fmt.Println("Supported targets:")
	for _, t := range sortedTargets() {
		fmt.Printf("  %-14s %s\n", t, aliasTargetNote(t))
	}
	for _, t := range target.SupportedTargets() {
		fmt.Printf("  %-14s %s\n", t.String(), target.Describe(t))
	}
	fmt.Println("Default: native")
	return CommandResult{ExitCode: ExitSuccess, Message: ""}
}

func aliasTargetNote(name string) string {
	switch name {
	case "native":
		return "(host default; " + target.Host().String() + ")"
	case "c23":
		return "(C23 source output; " + target.Host().String() + ")"
	case "native-link":
		return "(Phase 84 object/linker pipeline)"
	case "wasm32-wasi":
		return "(WebAssembly WASI module)"
	}
	return ""
}

// ConfigCommand implements `karkain config`: it prints the effective codegen
// configuration a build would use with the current defaults and flags (the
// same Config produced by codegen.NewConfig), plus the active engine.
func ConfigCommand() CommandResult {
	cfg := codegen.NewConfig()
	fmt.Println("Karkain effective configuration:")
	fmt.Printf("  target:        %s\n", cfg.Target)
	fmt.Printf("  engine:        %s\n", engineName(EngineFromEnv()))
	fmt.Printf("  compile-only:  %t\n", cfg.CompileOnly)
	fmt.Printf("  run-after:     %t\n", cfg.RunAfter)
	fmt.Printf("  debug:         %t\n", cfg.Debug)
	fmt.Printf("  verbose:       %t\n", cfg.Verbose)
	fmt.Printf("  ssa-pipeline:  %t\n", !cfg.DisableSSA)
	fmt.Printf("  output-path:   %s\n", orDefault(cfg.OutputPath, "(compiler-chosen)"))
	return CommandResult{ExitCode: ExitSuccess, Message: ""}
}

// engineName renders an EngineKind for `karkain config`.
func engineName(k EngineKind) string {
	if k == EngineGo {
		return "go"
	}
	return "kcc"
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// DefaultTarget is the target used when --target is not given.
const DefaultTarget = "native"
