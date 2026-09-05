package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"karkain/pkg/lexer"
	"karkain/pkg/module"
	"karkain/pkg/parser"
	"karkain/pkg/sema"
)

// moduleErrorResult converts a module-diagram error into a CLI CommandResult
// rendered through the diagnostics convention. Module errors (cycles, missing
// module, malformed import) are compile-class failures.
func moduleErrorResult(de *module.DiagramError) CommandResult {
	return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("module error: %v", de.Error())}
}

// sourceLoadResult converts a run/build/check source-loading error into a
// CommandResult, surfacing module-graph failures as compile-class diagnostics
// and every other read failure as a plain file error.
func sourceLoadResult(err error) CommandResult {
	if de, ok := err.(*module.DiagramError); ok {
		return moduleErrorResult(de)
	}
	return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error reading file: %v", err)}
}

// detectImports reports whether the given source text contains any top-level
// module `import <name>` declarations (excluding C imports).
func detectImports(src string) bool {
	p := parseProgramStrict(src)
	return p != nil && len(p.Imports) > 0
}

func parseProgramStrict(src string) *parser.Program {
	pp := parser.New(lexer.New(src))
	prog := pp.ParseProgram()
	return parser.ApplyMacroExpansion(prog)
}

// effectiveRootFile maps a CLI target (possibly a directory) to the actual
// .kark entry file (directory -> main.kark).
func effectiveRootFile(target string) string {
	clean := filepath.Clean(target)
	if info, err := os.Stat(clean); err == nil && info.IsDir() {
		return filepath.Join(clean, "main.kark")
	}
	return clean
}

// resolveSourcesRun is the run/build pipeline's module-aware loader. When the
// entry file declares module imports it returns the import-driven concatenated
// unit (deps first, root last); otherwise it preserves the legacy
// sibling/project assembly exactly. A *module.DiagramError is a compile-class
// failure (cycle / not-found / malformed import).
func resolveSourcesRun(targetFile string) (string, error) {
	cleanPath := effectiveRootFile(targetFile)
	text, _, ok, merr := assembleModuleUnit(cleanPath)
	if ok {
		if merr != nil {
			return "", merr
		}
		return text, nil
	}
	return resolveSources(cleanPath)
}

// resolveSourcesCheck is the check pipeline's module-aware loader. It also
// returns the SourceMap for visibility enforcement (module assembly provides
// one; the legacy path keeps its existing behavior).
func resolveSourcesCheck(targetFile string) (string, sema.SourceMap, error) {
	cleanPath := effectiveRootFile(targetFile)
	text, sm, ok, merr := assembleModuleUnit(cleanPath)
	if ok {
		if merr != nil {
			return "", nil, merr
		}
		return text, sm, nil
	}
	return resolveSourcesWithMap(cleanPath)
}

// assembleModuleUnit resolves the module graph rooted at targetFile and returns
// the deterministic, import-driven concatenated source plus its source map for
// visibility enforcement. When the root file declares no module imports, it
// returns ok=false so the caller falls back to the classic sibling-join,
// preserving legacy behavior with zero regression.
//
// ok=true, err=nil  : import-driven unit assembled (text+map valid).
// ok=false          : root has no imports; caller should use legacy assembly.
// ok=true, err!=nil : module resolution failed (cycle/not-found/malformed);
//                     err is a *module.DiagramError to render as a diagnostic.
//
// text/map are valid iff ok && err==nil.
func assembleModuleUnit(targetFile string) (text string, sm sema.SourceMap, ok bool, err error) {
	root := effectiveRootFile(targetFile)

	// Fast path: if the root source declares no imports, leave assembly to the
	// legacy sibling-join (which also handles project dependency sources).
	raw, rerr := os.ReadFile(root)
	if rerr != nil {
		return "", nil, false, nil
	}
	if !detectImports(string(raw)) {
		return "", nil, false, nil
	}

	g, gerr := module.New(module.NewSpec{RootFile: root, RootName: ""})
	if gerr != nil {
		if de, ok := gerr.(*module.DiagramError); ok {
			return "", nil, true, de
		}
		return "", nil, true, gerr
	}

	files := g.SortedFiles()
	var sb strings.Builder
	sm = sema.SourceMap{}
	curLine := 1
	funcMainRegex := regexp.MustCompile(`(?m)^\s*func\s+main\s*\(`)
	for _, f := range files {
		data, ferr := os.ReadFile(f)
		if ferr != nil {
			continue
		}
		// Root entry file keeps `func main`; non-root modules must not host the
		// entry point (matches the legacy "root last" contract).
		if f != root && funcMainRegex.Match(data) {
			continue
		}
		src := string(data)
		lines := strings.Count(src, "\n") + 1
		for i := 1; i <= lines; i++ {
			sm[curLine] = f
			curLine++
		}
		sb.WriteString(src)
		sb.WriteString("\n\n")
		curLine++
		curLine++
	}
	return sb.String(), sm, true, nil
}
