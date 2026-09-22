package cli

import (
	"fmt"
	"os"
	"strings"

	"karkain/pkg/wit"
)

// WitCommand implements `karkain wit [--stubs] <file.wit>`: it parses a WIT
// (WebAssembly Interface Types) file from Phase 139's subset and prints the
// canonical interface summary, or with --stubs the compilable Karkain type
// declarations. Missing files are usage errors; malformed WIT fails the
// command like a compile error (deterministic diagnostics, no partial
// output). Function call lowering is future work — see pkg/wit.
func WitCommand(args ...string) CommandResult {
	stubs := false
	file := ""
	for _, a := range args {
		if a == "--stubs" {
			stubs = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			return CommandResult{ExitCode: ExitUsage, Message: fmt.Sprintf("unknown flag '%s' (usage: karkain wit [--stubs] <file.wit>)", a)}
		}
		if file == "" {
			file = a
		}
	}
	if file == "" {
		return CommandResult{ExitCode: ExitUsage, Message: "usage: karkain wit [--stubs] <file.wit>"}
	}
	if !strings.HasSuffix(file, ".wit") {
		return CommandResult{ExitCode: ExitUsage, Message: fmt.Sprintf("not a WIT file: '%s' (want a .wit path)", file)}
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: fmt.Sprintf("cannot read '%s': %v", file, err)}
	}
	doc, err := wit.Parse(string(raw))
	if err != nil {
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("%s: malformed WIT: %v", file, err)}
	}
	if stubs {
		out, err := doc.Stubs()
		if err != nil {
			return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("%s: %v", file, err)}
		}
		return CommandResult{ExitCode: ExitSuccess, Message: out}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: doc.Summary()}
}
