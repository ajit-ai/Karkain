package cli

import (
	"fmt"
	"sort"
	"strings"

	"karkain/pkg/diagnostics"
	kpkg "karkain/pkg/pm"
)

// explanation pairs a stable code's meaning with actionable remediation.
type explanation struct {
	Code        string
	Meaning     string
	Remediation string
}

// errorRegistry is the curated, deterministic set of error codes the toolchain
// can produce (compiler classes from pkg/diagnostics plus the package-manager
// codes owned by pkg/pm). It backs `karkain explain <code>`.
var errorRegistry = []explanation{
	{string(diagnostics.CodeSyntax), "Syntax error from the lexer or parser: the source does not conform to the Karkain grammar.", "Inspect the reported line/column and the caret. Balance braces and parentheses, quote strings, and only use tokens the grammar recognizes."},
	{string(diagnostics.CodeResolve), "Name-resolution error: an undefined function reference, a duplicate top-level definition, or access to a private function from another file.", "Define the referenced name exactly once, or remove the duplicate definition. Mark names public when they must be callable across files."},
	{string(diagnostics.CodeBorrow), "Borrow-checking violation: a value is used after its owning scope ends, or reborrows would alias live references.", "Restrict the value's use to its declaring scope, or copy the data you need. The borrow checker reports the offending references and their scopes."},
	{string(diagnostics.CodeSema), "Semantic-analysis error: invalid kernel/actor/coroutine/quantum declarations (e.g. duplicate handlers, bad qubit ops).", "Follow the declaration rules the message cites: unique actor names and handlers, valid coroutine and quantum operations."},
	{string(diagnostics.CodeType), "Type-checking error: a value or operation does not match its expected type.", "Match operand types to the operation (numbers vs strings vs arrays), and align every branch of a conditional."},
	{string(diagnostics.CodeCodegen), "Code-generation/backend error: the front end accepted the program but a backend could not emit or compile it.", "Check the target (--target) and target-specific restrictions. Codegen is Karkain-owned; report backend errors with the failing construct."},
	{string(diagnostics.CodePackage), "Package/dependency integration error during build: a manifest dependency could not be resolved into sources.", "Run karkain update to re-resolve the lockfile, verify the dependency source is reachable, and confirm version constraints."},
	{string(diagnostics.CodeEnv), "Infrastructure error: a required external tool (C compiler, linker, filesystem path) is missing or unusable.", "Install a supported C compiler (GCC, Clang, or MSVC) and make it reachable from PATH, then retry."},
	{string(kpkg.ErrGitNotFound), "A git dependency did not resolve to a repository source.", "Check the dependency URL and network access, then re-run karkain update."},
	{string(kpkg.ErrGitClone), "Cloning a git dependency failed.", "Verify the repository exists, is reachable, and the working directory is writable."},
	{string(kpkg.ErrGitRevision), "The requested git revision (commit/tag/branch) was not found.", "Use a revision that exists in the repository; re-check tag spelling."},
	{string(kpkg.ErrGitSubdir), "The git dependency's subdirectory path was not found in the repository.", "Point the dependency at an existing subdirectory of the repository."},
	{string(kpkg.ErrRegistry), "A registry interaction failed (lookup, download, metadata).", "Confirm the registry is reachable and the package/version is registered."},
	{string(kpkg.ErrPackageMissing), "The requested package does not exist (locally or in the upstream source).", "Check the package name and source; local dependencies must point at a real directory with a manifest."},
	{string(kpkg.ErrVersionMissing), "The requested version was not found for the package.", "List available versions or relax the version constraint."},
	{string(kpkg.ErrCache), "The package cache could not be reached or was found corrupt.", "Inspect the cache (karkain pkg cache path); clean it and re-fetch if corrupted."},
	{string(kpkg.ErrLock), "The lockfile is missing, unreadable, or out of sync with the manifest.", "Run karkain update to write a consistent karkain.lock from the manifest."},
	{string(kpkg.ErrWorkspace), "A workspace operation failed (missing root config, member graph problem, or cycle).", "Verify karkain.workspace.json exists and members declare their workspace dependencies; resolve any cycle."},
	{string(kpkg.ErrManifest), "The project manifest (karkain.toml) is missing or invalid.", "Run karkain new/init to scaffold a valid manifest, then re-add dependencies."},
	{string(kpkg.ErrLocal), "A local dependency could not be resolved from its path.", "Point the dependency at an existing directory containing a karkain.toml manifest."},
	{string(kpkg.ErrSource), "An unknown dependency source type (expected registry, git, local, or workspace).", "Correct the dependency's source field to one of the supported kinds."},
}

// explainCode looks up a code and formats its documentation, returning
// (text, true) for known codes or ("", false) otherwise. The code argument is
// matched case-insensitively for convenience.
func explainCode(code string) (string, bool) {
	want := strings.ToUpper(strings.TrimSpace(code))
	for _, e := range errorRegistry {
		if e.Code == want {
			return fmt.Sprintf("%s\n  %s\n\nRemediation:\n  %s", e.Code, e.Meaning, e.Remediation), true
		}
	}
	return "", false
}

// ExplainCommand implements `karkain explain <code>`.
func ExplainCommand(code string) CommandResult {
	if code == "" {
		fmt.Println("Usage: karkain explain <error-code>")
		fmt.Println("Run 'karkain --help' for available commands")
		return CommandResult{ExitCode: ExitUsage, Message: ""}
	}
	if !strings.ContainsAny(code, "-") {
		fmt.Printf("Error: '%s' is not a valid error code (expected a code like E-K-SYN or E-PKG-LOCK)\n", code)
		return CommandResult{ExitCode: ExitUsage, Message: ""}
	}
	text, ok := explainCode(code)
	if !ok {
		fmt.Printf("Error: unknown error code '%s'\n", strings.ToUpper(code))
		fmt.Println("Run 'karkain explain --list' to see all registered codes.")
		return CommandResult{ExitCode: ExitUsage, Message: ""}
	}
	fmt.Println(text)
	return CommandResult{ExitCode: ExitSuccess, Message: ""}
}

// ExplainListCommand prints every registered code with its meaning.
func ExplainListCommand() CommandResult {
	sorted := make([]explanation, len(errorRegistry))
	copy(sorted, errorRegistry)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Code < sorted[j].Code })
	for _, e := range sorted {
		fmt.Printf("%-22s %s\n", e.Code, e.Meaning)
	}
	return CommandResult{ExitCode: ExitSuccess, Message: ""}
}