package cli

import (
	"fmt"
	"sort"

	"karkain/pkg/codegen"
)

// sortedTargets returns the supported --target values in deterministic order.
func sortedTargets() []string {
	out := make([]string, 0, len(validTargets))
	for t := range validTargets {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// TargetCommand implements `karkain target`: it lists the closed set of
// targets the toolchain can genuinely produce output for today, and the
// default used when --target is omitted.
func TargetCommand() CommandResult {
	fmt.Println("Supported targets:")
	for _, t := range sortedTargets() {
		fmt.Printf("  %s\n", t)
	}
	fmt.Println("Default: native")
	return CommandResult{ExitCode: ExitSuccess, Message: ""}
}

// ConfigCommand implements `karkain config`: it prints the effective codegen
// configuration a build would use with the current defaults and flags (the
// same Config produced by codegen.NewConfig).
func ConfigCommand() CommandResult {
	cfg := codegen.NewConfig()
	fmt.Println("Karkain effective configuration:")
	fmt.Printf("  target:        %s\n", cfg.Target)
	fmt.Printf("  compile-only:  %t\n", cfg.CompileOnly)
	fmt.Printf("  run-after:     %t\n", cfg.RunAfter)
	fmt.Printf("  debug:         %t\n", cfg.Debug)
	fmt.Printf("  verbose:       %t\n", cfg.Verbose)
	fmt.Printf("  ssa-pipeline:  %t\n", !cfg.DisableSSA)
	fmt.Printf("  output-path:   %s\n", orDefault(cfg.OutputPath, "(compiler-chosen)"))
	return CommandResult{ExitCode: ExitSuccess, Message: ""}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// DefaultTarget is the target used when --target is not given.
const DefaultTarget = "native"
