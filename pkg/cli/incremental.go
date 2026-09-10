package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"karkain/pkg/codegen"
	"karkain/pkg/compiler"
	"karkain/pkg/module"
)

// unitFiles returns the deterministic, ordered list of source files that make
// up the compilation unit for an incremental build, mirroring the assembler
// exactly: import-driven module ordering when the root uses imports, otherwise
// project/dependency sources (falling back to the sibling join), with
// main-bearing non-root files dropped and the root file appended last.
func unitFiles(targetFile string) ([]string, error) {
	clean := effectiveRootFile(targetFile)

	if raw, err := os.ReadFile(clean); err == nil && detectImports(string(raw)) {
		g, gerr := module.New(module.NewSpec{RootFile: clean, RootName: ""})
		if gerr != nil {
			return nil, gerr
		}
		return filterMainExceptRoot(g.SortedFiles(), clean), nil
	}

	files, err := projectSourceFiles(clean)
	if err != nil {
		files = siblingKarkFiles(clean)
	}
	return filterMainExceptRoot(files, clean), nil
}

// filterMainExceptRoot keeps only files that the assembler includes: dependency
// and sibling modules (dropping non-root files that host `func main`) followed
// by the root file, deduplicated by absolute path.
func filterMainExceptRoot(files []string, root string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(f string) {
		absp, aerr := filepath.Abs(f)
		if aerr != nil {
			absp = f
		}
		if seen[absp] {
			return
		}
		seen[absp] = true
		out = append(out, f)
	}
	for _, f := range files {
		if f == root || filepath.Clean(f) == filepath.Clean(root) {
			continue
		}
		if data, rerr := os.ReadFile(f); rerr == nil && funcMainPattern.Match(data) {
			continue
		}
		add(f)
	}
	add(root)
	return out
}

// incrementalExePath computes the executable output path using the same
// default the code generator applies when -o is absent.
func incrementalExePath(targetFile, outputPath string) string {
	if outputPath != "" {
		return outputPath
	}
	base := strings.TrimSuffix(effectiveRootFile(targetFile), filepath.Ext(effectiveRootFile(targetFile)))
	if runtime.GOOS == "windows" {
		base += ".exe"
	}
	return base
}

// incrementalCName derives the generated C artifact name (matching codegen's
// tmpCFile naming: <root base>.c).
func incrementalCName(targetFile string) string {
	root := effectiveRootFile(targetFile)
	return strings.TrimSuffix(root, filepath.Ext(root)) + ".c"
}

// CacheDirFor returns the default incremental cache directory: `.karkain-cache`
// next to the project root file, so `karkain clean` (which walks the project
// tree) removes it alongside source-anchored artifacts.
func CacheDirFor(targetFile string) string {
	return filepath.Join(filepath.Dir(effectiveRootFile(targetFile)), compiler.CacheDirName)
}

// statusSummary renders the per-module incremental result as a compact line.
func statusSummary(plan *compiler.Plan) string {
	var compiled, reused, invalidated int
	for _, m := range plan.Modules {
		switch m.Status {
		case compiler.StatusCompiled:
			compiled++
		case compiler.StatusReused:
			reused++
		case compiler.StatusInvalidated:
			invalidated++
		}
	}
	return fmt.Sprintf("incremental: %d module(s), %d compiled, %d reused, %d invalidated",
		len(plan.Modules), compiled, reused, invalidated)
}

// BuildCommandIncremental builds a native executable with a persistent,
// dependency-aware content-addressed cache. On a no-op (unchanged sources +
// compiler identity) it skips lexing/parsing, semantic analysis, C emission
// and gcc entirely and replays the cached executable. A failed build never
// poisons the cache: artifacts and manifest are written atomically only after
// a fully successful compile.
func BuildCommandIncremental(targetFile, outputPath string, cfg codegen.Config, verbose bool, cacheDir string) CommandResult {
	if err := ValidateKarFile(targetFile); err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: err.Error()}
	}
	if verbose {
		// resolve doubt about the target path resolution up front.
		fmt.Printf("incremental: target=%s cache=%s\n", effectiveRootFile(targetFile), cacheDir)
	}

	files, err := unitFiles(targetFile)
	if err != nil {
		return sourceLoadResult(err)
	}
	if len(files) == 0 {
		return CommandResult{ExitCode: ExitFailure, Message: "No source files resolved for incremental build."}
	}

	c := compiler.NewCache(cacheDir)
	prev := c.Load()
	key := compiler.CompilerKey(cfg)
	plan := compiler.PlanBuild(prev, files, key)
	cName := plan.ProjectHash + ".c"
	exeName := plan.ProjectHash + ".exe"
	exePath := incrementalExePath(targetFile, outputPath)

	// Fast path: nothing changed and both artifacts are cached.
	if !plan.Rebuild && c.HasArtifact(cName) && c.HasArtifact(exeName) {
		exeBytes, rerr := c.ReadArtifact(exeName)
		if rerr == nil {
			if werr := os.WriteFile(exePath, exeBytes, 0o755); werr != nil {
				return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error writing executable: %v", werr)}
			}
			if verbose {
				printStatuses(plan)
			}
			return CommandResult{ExitCode: ExitSuccess, Message: "Build successful (cached). " + statusSummary(plan)}
		}
	}

	// Full build path: assemble, parse, borrow-check, generate C, compile+link.
	cfg.RunAfter = false
	cfg.CompileOnly = false
	cfg.Verbose = verbose
	cfg.OutputPath = exePath

	sourceText, rerr := resolveSourcesRun(targetFile)
	if rerr != nil {
		return sourceLoadResult(rerr)
	}
	if verbose {
		printTokenStream(sourceText)
	}
	prog := parseSource(sourceText, verbose)
	if prog == nil {
		return CommandResult{ExitCode: ExitCompile, Message: "Parse failed"}
	}
	if errs := runBorrowCheck(prog); len(errs) > 0 {
		msg := "Borrow check failed:\n"
		for _, e := range errs {
			msg += "  " + e.Message + "\n"
		}
		return CommandResult{ExitCode: ExitCompile, Message: msg}
	}

	// Phase 85 native-link uses the object/linker pipeline, not the C
	// generate+compile path: it produces no monolith C artifact to cache, so
	// the incremental cache is skipped for that target (documented boundary).
	if cfg.Target == "native-link" {
		if native := nativeBuildCommand(prog, targetFile, exePath, verbose); native.ExitCode != ExitSuccess {
			return native
		}
		return CommandResult{ExitCode: ExitSuccess, Message: "Build successful (native-link)."}
	}

	emitGPUShaders(prog, targetFile, verbose)
	cg := codegen.New(cfg)
	if gerr := cg.GenerateAndCompile(prog, targetFile); gerr != nil {
		return CommandResult{ExitCode: classifyCompileError(gerr), Message: fmt.Sprintf("Build Error: %v", gerr)}
	}

	// Cache the fresh artifacts and manifest (atomic writes only).
	cBytes, cerr := os.ReadFile(incrementalCName(targetFile))
	if cerr != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error reading generated C: %v", cerr)}
	}
	exeBytes, eerr := os.ReadFile(exePath)
	if eerr != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error reading executable: %v", eerr)}
	}
	if serr := c.Store(compiler.ManifestFor(plan, cName, exeName), cBytes, exeBytes); serr != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error writing incremental cache: %v", serr)}
	}

	if verbose {
		printStatuses(plan)
	}
	return CommandResult{ExitCode: ExitSuccess, Message: "Build successful (incremental). " + statusSummary(plan)}
}

// printStatuses emits one line per module with its incremental status.
func printStatuses(plan *compiler.Plan) {
	for _, m := range plan.Modules {
		fmt.Printf("  karkain incremental: %-11s %s\n", m.Status, m.Path)
	}
}