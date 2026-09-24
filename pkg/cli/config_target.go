package cli

import (
	"fmt"
	"sort"
	"strings"

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

// TargetCommand implements `karkain target [compute-target]`: it lists the
// host platform, the closed set of targets the toolchain can genuinely
// produce output for today, and the default used when --target is omitted.
// With an optional compute-target argument it prints the Phase-124 capability
// view of that target instead; unknown compute-target names are a usage error.
func TargetCommand(rest ...string) CommandResult {
	if len(rest) > 0 {
		return computeTargetDetail(rest[0])
	}
	fmt.Println("Host: " + target.Host().String())
	fmt.Println("Supported targets:")
	for _, t := range sortedTargets() {
		fmt.Printf("  %-14s %s\n", t, aliasTargetNote(t))
	}
	for _, t := range target.SupportedTargets() {
		fmt.Printf("  %-14s %s\n", t.String(), target.Describe(t))
	}
	fmt.Println("Default: native")
	fmt.Println("Compute targets (Phase 124 experimental):")
	for _, ct := range target.ComputeTargets() {
		fmt.Printf("  %-20s %-13s %s\n", ct.Name, ct.Maturity.String(), ct.Description)
	}
	return CommandResult{ExitCode: ExitSuccess, Message: ""}
}

// computeTargetDetail renders the capability view of one registered compute
// target: identity, maturity, memory/execution model, the capability set, the
// accepted KIR v1 classes and the native tensor ops.
func computeTargetDetail(name string) CommandResult {
	ct := target.LookupComputeTarget(name)
	if ct == nil {
		fmt.Printf("Error: unknown compute target '%s'\n", name)
		fmt.Println("Known compute targets: " + strings.Join(computeTargetNames(), ", "))
		return CommandResult{ExitCode: ExitUsage, Message: ""}
	}
	fmt.Printf("Compute target: %s\n", ct.Name)
	fmt.Printf("Family:         %s\n", ct.Family)
	fmt.Printf("Maturity:       %s\n", ct.Maturity.String())
	if ct.Triple != nil {
		fmt.Printf("Host triple:    %s\n", ct.Triple.String())
	}
	fmt.Printf("Memory model:   %s\n", ct.MemoryModel)
	fmt.Printf("Execution model:  %s\n", ct.ExecutionModel)
	fmt.Printf("Description:    %s\n", ct.Description)
	fmt.Printf("Capabilities:   %s\n", strings.Join(ct.Capabilities.List(), ", "))
	fmt.Printf("KIR classes:    %s\n", strings.Join(ct.KIRClasses.List(), ", "))
	if ops := ct.TensorOpNames(); len(ops) > 0 {
		fmt.Printf("Tensor ops:     %s\n", strings.Join(ops, ", "))
	}
	return CommandResult{ExitCode: ExitSuccess, Message: ""}
}

func computeTargetNames() []string {
	cts := target.ComputeTargets()
	out := make([]string, 0, len(cts))
	for _, ct := range cts {
		out = append(out, ct.Name)
	}
	return out
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
	case NativeLinuxTarget:
		return "(C-free x86-64 Linux executable; run needs linux/amd64)"
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
