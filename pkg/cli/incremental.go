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
	// Phase 148: the incremental cache serves the C pipeline (monolith C
	// + objects); the C-free native backend has no C artifacts to cache,
	// so incremental+native is refused loudly (native-split caching is
	// post-148 work, never silent C fallback).
	if IsNativeTarget(cfg.Target) {
		return CommandResult{ExitCode: ExitUsage, Message: "karkain build --incremental does not support --target native-x86_64-linux yet (native-split caching is future work); build without --incremental"}
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

	// Phase 134: per-module translation-unit flow. Falls through to the
	// whole-assembly flow below for non-gcc toolchains, concurrency or
	// profiling programs, and native-link targets (all preserved exactly).
	if res, handled := buildSplitFlow(targetFile, cfg, verbose, cacheDir, files, plan, key, c, exePath); handled {
		return res
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

// sameAbsPath reports whether two paths name the same file.
func sameAbsPath(a, b string) bool {
	if a == b || filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && aa == bb
}

// printStatuses emits one line per module with its incremental status.
func printStatuses(plan *compiler.Plan) {
	for _, m := range plan.Modules {
		fmt.Printf("  karkain incremental: %-11s %s\n", m.Status, m.Path)
	}
}

// Phase 134: buildSplitFlow compiles through per-module translation units
// (shared runtime TU + one TU per source module, linked together) instead
// of one monolithic C unit. It returns handled=false (without side effects
// beyond parsing) when the program needs the whole-assembly flow: a
// non-gcc toolchain, concurrency or profiling surfaces, or a native-link
// target. All of those keep their exact v1 behavior.
func buildSplitFlow(targetFile string, cfg codegen.Config, verbose bool, cacheDir string, files []string, plan *compiler.Plan, key string, c *compiler.Cache, exePath string) (CommandResult, bool) {
	// All cache/artifact paths absolute: gcc children inherit the caller
	// working directory, so relative joins would resolve unpredictably.
	if abs, err := filepath.Abs(cacheDir); err == nil {
		cacheDir = abs
		c = compiler.NewCache(cacheDir)
	}
	if abs, err := filepath.Abs(exePath); err == nil {
		exePath = abs
	}
	if cfg.Target == "native-link" {
		return CommandResult{}, false
	}
	cc, compileFlags, linkLibs, consoleLink, gccStyle, err := codegen.SplitToolchain(cfg)
	if err != nil || !gccStyle {
		return CommandResult{}, false
	}

	sourceText, rerr := resolveSourcesRun(targetFile)
	if rerr != nil {
		return sourceLoadResult(rerr), true
	}
	if verbose {
		printTokenStream(sourceText)
	}
	prog := parseSource(sourceText, verbose)
	if prog == nil {
		return CommandResult{ExitCode: ExitCompile, Message: "Parse failed"}, true
	}
	if errs := runBorrowCheck(prog); len(errs) > 0 {
		msg := "Borrow check failed:\n"
		for _, e := range errs {
			msg += "  " + e.Message + "\n"
		}
		return CommandResult{ExitCode: ExitCompile, Message: msg}, true
	}

	rootFile := effectiveRootFile(targetFile)
	split, serr := codegen.GenerateSplit(cfg, prog, files, rootFile)
	if serr != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Build Error: %v", serr)}, true
	}
	if split.UsesConcurrency || split.Profiling {
		return CommandResult{}, false
	}

	// Runtime identity covers the compiler key, the flags that affect
	// runtime codegen, and the header text itself (self-invalidating).
	runtimeKey := compiler.RuntimeKey(key, split.Header, cfg.Profiling, split.UsesConcurrency, split.SimdNeedsAVX)
	prev := c.Load()

	// A runtime-key change rebuilds everything (same contract as the
	// compiler-key mismatch, which PlanBuild reports as invalidated).
	modules := plan.Modules
	if prev != nil && prev.RuntimeKey != "" && prev.RuntimeKey != runtimeKey {
		for _, m := range modules {
			if m.Status == compiler.StatusReused {
				m.Status = compiler.StatusInvalidated
			}
		}
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error creating cache: %v", err)}, true
	}
	writeCache := func(name, text string) error {
		return os.WriteFile(filepath.Join(cacheDir, name), []byte(text), 0o644)
	}

	// Runtime TU: header + sources, precompiled header (best-effort),
	// then the shared object. Rebuilt only on runtime-key change.
	runtimeObj := codegen.RuntimeObjectName
	needRuntime := prev == nil || prev.RuntimeKey != runtimeKey || !c.HasArtifact(runtimeObj)
	if needRuntime {
		if err := writeCache(codegen.RuntimeHeaderName, split.Header); err != nil {
			return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error writing runtime header: %v", err)}, true
		}
		if err := writeCache(codegen.RuntimeSourceName, split.RuntimeSrc); err != nil {
			return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error writing runtime source: %v", err)}, true
		}
		codegen.BuildPCH(cc, compileFlags, filepath.Join(cacheDir, codegen.RuntimeHeaderName))
		if err := codegen.CompileTU(cc, compileFlags, cacheDir, filepath.Join(cacheDir, codegen.RuntimeSourceName), filepath.Join(cacheDir, runtimeObj)); err != nil {
			return CommandResult{ExitCode: classifyCompileError(err), Message: fmt.Sprintf("Build Error: %v", err)}, true
		}
	}

	// Per-module TUs: compile what the plan (plus missing artifacts) demands.
	objects := []string{filepath.Join(cacheDir, runtimeObj)}
	objNames := map[string]string{}
	var recompiled []*compiler.ModuleRecord
	needLink := needRuntime
	for _, m := range modules {
		objName := compiler.ObjectNameFor(m.Path, m.ContentHash)
		objNames[m.Path] = objName
		objPath := filepath.Join(cacheDir, objName)
		cName := strings.TrimSuffix(objName, ".o") + ".c"
		unitText := split.ModuleTUs[m.Path]
		if sameAbsPath(m.Path, rootFile) {
			// The root TU is addressed by path, not by map lookup: the
			// assembly root is always last and may alias the map key form.
			unitText = split.RootTU
		}
		if err := writeCache(cName, unitText); err != nil {
			return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error writing TU: %v", err)}, true
		}
		if m.Status == compiler.StatusReused && c.HasArtifact(objName) {
			objects = append(objects, objPath)
			continue
		}
		if err := codegen.CompileTU(cc, compileFlags, cacheDir, filepath.Join(cacheDir, cName), objPath); err != nil {
			return CommandResult{ExitCode: classifyCompileError(err), Message: fmt.Sprintf("Build Error: %v", err)}, true
		}
		objects = append(objects, objPath)
		recompiled = append(recompiled, m)
		needLink = true
	}

	// Link when anything is fresh; otherwise the cached exe is still good
	// (the fast path above already handled the pure no-op).
	if needLink {
		if err := codegen.LinkObjects(cc, linkLibs, consoleLink, objects, exePath); err != nil {
			return CommandResult{ExitCode: classifyCompileError(err), Message: fmt.Sprintf("Build Error: %v", err)}, true
		}
	} else if _, err := os.Stat(exePath); err != nil {
		// Objects all reused but the output vanished (deleted): relink.
		if err := codegen.LinkObjects(cc, linkLibs, consoleLink, objects, exePath); err != nil {
			return CommandResult{ExitCode: classifyCompileError(err), Message: fmt.Sprintf("Build Error: %v", err)}, true
		}
	}

	// Beside-source concatenated view (deterministic full-program C, keeps
	// the Phase 105 main.c contract): header + runtime + root + modules.
	var view strings.Builder
	view.WriteString(split.Header)
	view.WriteString(split.RuntimeSrc)
	view.WriteString(split.RootTU)
	for _, f := range files {
		if t, ok := split.ModuleTUs[f]; ok {
			view.WriteString(t)
		}
	}
	if err := os.WriteFile(incrementalCName(targetFile), []byte(view.String()), 0o644); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error writing generated C: %v", err)}, true
	}
	manifest := compiler.ManifestFor(plan, plan.ProjectHash+".c", plan.ProjectHash+".exe")
	manifest.Version = compiler.ManifestVersion
	manifest.RuntimeKey = runtimeKey
	manifest.RuntimeObj = runtimeObj
	manifest.Objects = objNames
	// Phase 134: store only fresh artifacts. Reused objects are already
	// on disk with identical bytes (content-keyed names + key gating), so
	// rewriting them would only churn mtimes and mislead staleness reads.
	// The manifest itself is always rewritten (cheap, tiny).
	artifacts := map[string][]byte{}
	if !c.HasArtifact(plan.ProjectHash + ".c") {
		artifacts[plan.ProjectHash+".c"] = []byte(view.String())
	}
	linked := needLink
	if !linked {
		// No link ran: the on-disk exe is still good, but ensure the
		// cache holds it (it may predate the cache entry).
		if !c.HasArtifact(plan.ProjectHash + ".exe") {
			if eb, eerr := os.ReadFile(exePath); eerr != nil {
				return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error reading executable: %v", eerr)}, true
			} else {
				artifacts[plan.ProjectHash+".exe"] = eb
			}
		}
	} else {
		eb, eerr := os.ReadFile(exePath)
		if eerr != nil {
			return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error reading executable: %v", eerr)}, true
		}
		artifacts[plan.ProjectHash+".exe"] = eb
	}
	if needRuntime {
		if rob, roerr := os.ReadFile(filepath.Join(cacheDir, runtimeObj)); roerr != nil {
			return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error reading runtime object: %v", roerr)}, true
		} else {
			artifacts[runtimeObj] = rob
		}
	}
	for _, m := range recompiled {
		objPath := filepath.Join(cacheDir, objNames[m.Path])
		ob, oerr := os.ReadFile(objPath)
		if oerr != nil {
			return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error reading object: %v", oerr)}, true
		}
		artifacts[objNames[m.Path]] = ob
	}
	if serr := c.StoreArtifacts(manifest, artifacts); serr != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error writing incremental cache: %v", serr)}, true
	}

	if verbose {
		printStatuses(plan)
	}
	return CommandResult{ExitCode: ExitSuccess, Message: "Build successful (incremental). " + statusSummary(plan)}, true
}