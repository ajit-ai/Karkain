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
	{string(diagnostics.CodeK001), "Syntax error from the lexer or parser (numeric form of E-K-SYN): the source does not conform to the Karkain grammar.", "Inspect the reported line/column and the caret. Balance braces and parentheses, quote strings, and only use tokens the grammar recognizes."},
	{string(diagnostics.CodeK002), "Name-resolution error (numeric form of E-K-RES): an undefined function reference, a duplicate top-level definition, or access to a private function from another file.", "Define the referenced name exactly once, or remove the duplicate definition. Mark names public when they must be callable across files."},
	{string(diagnostics.CodeK003), "Borrow-checking violation (numeric form of E-K-BRW): a value is used after its owning scope ends, or reborrows would alias live references.", "Restrict the value's use to its declaring scope, or copy the data you need. The borrow checker reports the offending references and their scopes."},
	{string(diagnostics.CodeK004), "Semantic-analysis error (numeric form of E-K-SEM): invalid kernel/actor/coroutine/quantum declarations or an unknown @target attribute.", "Follow the declaration rules the message cites: unique actor names and handlers, valid coroutine and quantum operations, and @target values in {cpu, npu, gpu}."},
	{string(diagnostics.CodeK005), "Type-checking error (numeric form of E-K-TYP): a value or operation does not match its expected type.", "Match operand types to the operation (numbers vs strings vs arrays), and align every branch of a conditional."},
	{string(diagnostics.CodeK006), "Code-generation/backend error (numeric form of E-K-CG): the front end accepted the program but a backend could not emit or compile it.", "Check the target (--target) and target-specific restrictions. Codegen is Karkain-owned; report backend errors with the failing construct."},
	{string(diagnostics.CodeK007), "Package/dependency integration error during build (numeric form of E-K-PKG): a manifest dependency could not be resolved into sources.", "Run karkain update to re-resolve the lockfile, verify the dependency source is reachable, and confirm version constraints."},
	{string(diagnostics.CodeK008), "Infrastructure error (numeric form of E-K-ENV): a required external tool (C compiler, linker, filesystem path) is missing or unusable.", "Install a supported C compiler (GCC, Clang, or MSVC) and make it reachable from PATH, then retry."},
	{string(diagnostics.CodeK100), "Warning (numeric form of W-K-UNUSED): a `let`/`var` name is declared in a function but never read. Warnings do not stop compilation.", "Drop the declaration, or read/inspect the variable somewhere before the function ends."},
	{"K101", "Self-hosted (kcc) checker: call to an undefined function — no function of that name is declared in the program.", "Define the function, or fix the spelling so the call matches an existing function or builtin."},
	{"K102", "Self-hosted (kcc) checker: read of an undefined identifier — no variable/parameter of that name is in scope.", "Declare the name before using it, or fix the spelling to match a name that exists in the current scope."},
	{"K103", "Self-hosted (kcc) checker: function/kernel arity mismatch — a call passes a different number of arguments than the definition declares.", "Pass exactly the declared number of arguments, or change the definition to match the call site."},
	{"K104", "Self-hosted (kcc) checker: builtin arity mismatch — a language/standard builtin received the wrong number of arguments.", "Pass the builtin's documented argument count."},
	{"K106", "Self-hosted (kcc) checker: struct literal uses an undefined type name.", "Define the struct, or fix the type name used in the literal."},
	{"K107", "Self-hosted (kcc) checker: duplicate top-level definition — a function or type is declared more than once.", "Keep exactly one declaration of the name at top level."},
	{"K108", "Self-hosted (kcc) checker: `break` or `continue` appears outside of any loop.", "Move the `break`/`continue` inside a loop body, or remove it."},
	{"K109", "Self-hosted (kcc) checker: a type name is being called as a function.", "Do not call type names directly; construct values through the supported literal/constructor forms."},
	{"K112", "Self-hosted (kcc) checker: a variable's declared primitive annotation does not match the type of its initializer.", "Make the annotation and initializer agree (int/float/bool/string vs the inferred value type)."},
	{"K113", "Self-hosted (kcc) checker: reassignment of a constant (`const`) name.", "Declare the name with `let`/`var` if it must be reassigned, or keep it constant."},
	{"K114", "Closure escape rejection (both engines; Go reports it as error[K002]): `return` of a closure that captures function-local state. Captures are by-ref into the dying frame, so the value would dangle.", "Only return non-capturing closures (they carry a null env and stay sound), or restructure so the closure never outlives the frame it captures from."},
	{string(diagnostics.CodeWarnUnused), "Warning: a `let`/`var` name is declared in a function but never read. Warnings do not stop compilation; they flag dead data so the declaration can be removed.", "Drop the declaration, or read/inspect the variable somewhere before the function ends. Write-only variables (assigned but never read) are reported for the same reason."},
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
	// Phase 117: accept every registered code shape — the dashed forms
	// (E-K-SYN, E-PKG-LOCK) and the numeric forms (K001, K101, K113) are all
	// legitimate; only an entirely unrecognized token is rejected below.
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
