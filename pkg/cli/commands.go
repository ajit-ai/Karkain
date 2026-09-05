package cli

import (
	"encoding/binary"
	"fmt"
	"karkain/pkg/codegen"
	"karkain/pkg/diagnostics"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"karkain/pkg/pm"
	"karkain/pkg/sema"
	"karkain/pkg/testing"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const versionString = "Karkain Compiler v1.0.0 (%s/%s, LSP Engine & IDE Tooling)"

// CommandResult holds the outcome of a CLI command
type CommandResult struct {
	ExitCode int
	Message  string
}

// RunCommand parses, type-checks, transpiles, compiles and executes a .kark file
func RunCommand(targetFile string, cfg codegen.Config, verbose bool) CommandResult {
	if err := ValidateKarFile(targetFile); err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: err.Error()}
	}

	sourceText, err := resolveSourcesRun(targetFile)
	if err != nil {
		return sourceLoadResult(err)
	}

	if verbose {
		printTokenStream(sourceText)
	}

	prog := parseSource(sourceText, verbose)
	if prog == nil {
		return CommandResult{ExitCode: ExitCompile, Message: "Parse failed"}
	}

	// Phase 43: Run borrow checker
	if errs := runBorrowCheck(prog); len(errs) > 0 {
		msg := "Borrow check failed:\n"
		for _, e := range errs {
			msg += "  " + e.Message + "\n"
		}
		return CommandResult{ExitCode: ExitCompile, Message: msg}
	}

	cfg.RunAfter = true
	cfg.Verbose = verbose
	cg := codegen.New(cfg)

	if err := cg.GenerateAndCompile(prog, targetFile); err != nil {
		return CommandResult{ExitCode: classifyCompileError(err), Message: fmt.Sprintf("Execution Error: %v", err)}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: ""}
}

// BuildCommand compiles a .kark file into a native executable
func BuildCommand(targetFile string, outputPath string, cfg codegen.Config, verbose bool) CommandResult {
	if err := ValidateKarFile(targetFile); err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: err.Error()}
	}

	sourceText, err := resolveSourcesRun(targetFile)
	if err != nil {
		return sourceLoadResult(err)
	}

	if verbose {
		printTokenStream(sourceText)
	}

	prog := parseSource(sourceText, verbose)
	if prog == nil {
		return CommandResult{ExitCode: ExitCompile, Message: "Parse failed"}
	}

	// Phase 43: Run borrow checker
	if errs := runBorrowCheck(prog); len(errs) > 0 {
		msg := "Borrow check failed:\n"
		for _, e := range errs {
			msg += "  " + e.Message + "\n"
		}
		return CommandResult{ExitCode: ExitCompile, Message: msg}
	}

	cfg.RunAfter = false
	cfg.CompileOnly = true
	cfg.Verbose = verbose
	if outputPath != "" {
		cfg.OutputPath = outputPath
	}

	// Emit GPU shaders if kernels are present
	emitGPUShaders(prog, targetFile, verbose)

	cg := codegen.New(cfg)
	if err := cg.GenerateAndCompile(prog, targetFile); err != nil {
		return CommandResult{ExitCode: classifyCompileError(err), Message: fmt.Sprintf("Build Error: %v", err)}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: "Build successful."}
}

// CheckCommand validates a .kark file without producing output binaries
func CheckCommand(targetFile string, verbose bool) CommandResult {
	if err := ValidateKarFile(targetFile); err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: err.Error()}
	}

	sourceText, srcMap, err := resolveSourcesCheck(targetFile)
	if err != nil {
		return sourceLoadResult(err)
	}

	src := sourceText
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()

	// Check for parse errors
	if len(p.Errors) > 0 {
		reporter := diagnostics.NewReporter(src, targetFile)
		for _, parseErr := range p.Errors {
			line, col := extractLineCol(parseErr)
			fmt.Fprint(os.Stderr, reporter.Report(diagnostics.SeverityError, line, col, parseErr))
		}
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("%d parse error(s) found", len(p.Errors))}
	}

	// Apply macro expansion
	prog = parser.ApplyMacroExpansion(prog)

	// Whole-program name-resolution diagnostics (Option B): report duplicate
	// top-level definitions and undefined bare function references at the
	// Karkain level. This pass is purely additive (it never changes emitted
	// output), so build/run are unaffected.
	resolver := sema.NewResolver(prog, srcMap)
	if resolveErrs := resolver.Resolve(); len(resolveErrs) > 0 {
		reporter := diagnostics.NewReporter(src, targetFile)
		for _, re := range resolveErrs {
			fmt.Fprint(os.Stderr, reporter.Report(diagnostics.SeverityError, re.Line, 1, re.Msg))
		}
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("%d name-resolution error(s) found", len(resolveErrs))}
	}

	// Run kernel analyzer for semantic checks
	analyzer := sema.NewKernelAnalyzer()
	errorCount := 0
	for _, stmt := range prog.Statements {
		if kernel, ok := stmt.(*parser.KernelDeclStmt); ok {
			kernelErrors := analyzer.AnalyzeKernel(kernel)
			if len(kernelErrors) > 0 {
				reporter := diagnostics.NewReporter(src, targetFile)
				for _, ke := range kernelErrors {
					// Use line 1 as fallback since kernel errors don't carry line info
					fmt.Fprint(os.Stderr, reporter.Report(diagnostics.SeverityError, 1, 1, ke.Error()))
				}
				errorCount += len(kernelErrors)
			}
		}
	}

	if errorCount > 0 {
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("%d semantic error(s) found", errorCount)}
	}

	if verbose {
		fmt.Printf("Check passed: %s (%d statements)\n", targetFile, len(prog.Statements))
	}
	return CommandResult{ExitCode: ExitSuccess, Message: "Check passed."}
}

// TestCommand discovers and runs *_test.kark files and functions prefixed with
// test_ or @test. It is the KTF-001 native test entry point.
func TestCommand(testPath string, cfg codegen.Config, verbose bool) CommandResult {
	return TestCommandFiltered(testPath, cfg, verbose, "")
}

// TestCommandFiltered is TestCommand with a deterministic substring filter over
// the stable test identity/name. An empty filter selects all tests.
func TestCommandFiltered(testPath string, cfg codegen.Config, verbose bool, filter string) CommandResult {
	info, err := os.Stat(testPath)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error accessing path: %v", err)}
	}

	var testFiles []string
	if !info.IsDir() {
		testFiles = []string{testPath}
	} else {
		testFiles, err = findTestFiles(testPath)
		if err != nil {
			return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error scanning directory: %v", err)}
		}
	}

	// Deterministic ordering: sort the discovered files.
	sort.Strings(testFiles)
	if len(testFiles) == 0 {
		return CommandResult{ExitCode: ExitSuccess, Message: "No test files found."}
	}

	var allResults []testing.TestResult
	for _, tf := range testFiles {
		if verbose {
			fmt.Printf("=== Running tests in %s ===\n", tf)
		}
		res := runTestsInFile(tf, cfg, verbose, filter)
		allResults = append(allResults, res...)
	}

	if len(allResults) == 0 {
		if filter != "" {
			return CommandResult{ExitCode: ExitSuccess, Message: fmt.Sprintf("No tests matched filter '%s'.", filter)}
		}
		return CommandResult{ExitCode: ExitSuccess, Message: "No tests found."}
	}

	// Deterministic output ordering: group consecutive emissions but keep the
	// stable per-test ordering (already sorted within files, files sorted).
	if !verbose {
		printResults(appliedFilterLabel(filter), allResults)
	}

	summary := testing.Summarize(allResults)
	printTestSummary(summary)

	if summary.Failed > 0 {
		return CommandResult{ExitCode: ExitTest, Message: summaryLine(summary)}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: summaryLine(summary)}
}

// --- internal helpers ---

// ValidateKarFile checks that the given path is a non-empty .kark file path
func ValidateKarFile(path string) error {
	if path == "" {
		return fmt.Errorf("No input .kark file specified")
	}

	// Check if the path is a known subcommand (e.g., "transpile", "build", "run")
	subcommands := []string{"transpile", "build", "run", "check", "test", "lsp", "init", "add", "fetch"}
	for _, cmd := range subcommands {
		if path == cmd {
			return fmt.Errorf("Input file must be a .kark file: %s", path)
		}
	}

	// Clean the path to handle Windows backslashes and relative paths
	cleanPath := filepath.Clean(path)

	// Check if the path is a directory
	info, err := os.Stat(cleanPath)
	if err == nil && info.IsDir() {
		// If it's a directory, assume main.kark inside it
		cleanPath = filepath.Join(cleanPath, "main.kark")
	}

	if strings.ToLower(filepath.Ext(cleanPath)) != ".kark" {
		return fmt.Errorf("Input file must be a .kark file: %s", path)
	}
	return nil
}

func loadSourceWithSiblings(targetFile string) (string, error) {
	// Handle directories by appending main.kark
	cleanPath := filepath.Clean(targetFile)
	info, err := os.Stat(cleanPath)
	if err == nil && info.IsDir() {
		cleanPath = filepath.Join(cleanPath, "main.kark")
	}

	dir := filepath.Dir(cleanPath)
	entries, err := os.ReadDir(dir)
	var fullContent strings.Builder

	funcMainRegex := regexp.MustCompile(`(?m)^\s*func\s+main\s*\(`)

	if err == nil && len(entries) > 1 {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".kark" && entry.Name() != filepath.Base(cleanPath) {
				data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
				if readErr == nil {
					fileStr := string(data)
					if !funcMainRegex.MatchString(fileStr) {
						fullContent.WriteString(fileStr)
						fullContent.WriteString("\n\n")
					}
				}
			}
		}
	}

	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return "", err
	}
	fullContent.WriteString(string(content))
	return fullContent.String(), nil
}

// resolveSources builds the concatenated Karkain source for a root file.
//
// Non-project builds behave exactly like loadSourceWithSiblings: the root file
// plus its same-directory sibling modules (without `func main`) joined in
// sorted, deterministic order.
//
// Project builds (a karkain.toml is found at or above the root file) also pull
// in the sources of resolved *local* dependencies, mirroring the endorsed
// project layout. Local dependency sources are emitted BEFORE the project's own
// modules and the root file, so dependencies are defined upstream of what
// consumes them. Each source file is included at most once. Registry and git
// dependencies keep their existing "not yet available" semantics and do not
// contribute sources (their module layout is not yet formalized).
func resolveSources(targetFile string) (string, error) {
	cleanPath := filepath.Clean(targetFile)
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		cleanPath = filepath.Join(cleanPath, "main.kark")
	}

	files, err := projectSourceFiles(cleanPath)
	if err != nil {
		// Not (or not fully) inside a resolvable Karkain project: fall back to
		// the classic sibling-join, which is also the deterministic order for
		// the non-project case.
		return loadSourceWithSiblings(cleanPath)
	}

	funcMainRegex := regexp.MustCompile(`(?m)^\s*func\s+main\s*\(`)
	var sb strings.Builder
	seen := map[string]bool{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if funcMainRegex.Match(data) {
			continue // keep only the root file as the entry point
		}
		absp, _ := filepath.Abs(f)
		if seen[absp] {
			continue
		}
		seen[absp] = true
		sb.Write(data)
		sb.WriteString("\n\n")
	}

	// The root file is the entry point and is appended last, always.
	rp, _ := filepath.Abs(cleanPath)
	if !seen[rp] {
		if data, err := os.ReadFile(cleanPath); err == nil {
			sb.Write(data)
		}
	}
	return sb.String(), nil
}

// resolveSourcesWithMap returns both the concatenated source text and a
// SourceMap (line number → source file path) for visibility enforcement.
func resolveSourcesWithMap(targetFile string) (string, sema.SourceMap, error) {
	cleanPath := filepath.Clean(targetFile)
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		cleanPath = filepath.Join(cleanPath, "main.kark")
	}

	files, err := projectSourceFiles(cleanPath)
	if err != nil {
		return siblingJoinWithMap(cleanPath)
	}

	funcMainRegex := regexp.MustCompile(`(?m)^\s*func\s+main\s*\(`)
	var sb strings.Builder
	sm := sema.SourceMap{}
	curLine := 1
	seen := map[string]bool{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if funcMainRegex.Match(data) {
			continue
		}
		absp, _ := filepath.Abs(f)
		if seen[absp] {
			continue
		}
		seen[absp] = true
		src := string(data)
		lines := strings.Count(src, "\n") + 1
		for i := 1; i <= lines; i++ {
			sm[curLine] = f
			curLine++
		}
		sb.WriteString(src)
		sb.WriteString("\n\n")
		curLine++ // the \n
		curLine++ // the extra \n
	}

	rp, _ := filepath.Abs(cleanPath)
	if !seen[rp] {
		if data, err := os.ReadFile(cleanPath); err == nil {
			src := string(data)
			lines := strings.Count(src, "\n") + 1
			for i := 1; i <= lines; i++ {
				sm[curLine] = cleanPath
				curLine++
			}
			sb.WriteString(src)
		}
	}
	return sb.String(), sm, nil
}

// siblingJoinWithMap is the map-producing equivalent of loadSourceWithSiblings:
// it joins the root entry file with its same-directory sibling .kark modules
// (excluding files that declare `func main`) in sorted, deterministic order,
// and records each line's owning file. Keeping run/build and check on the same
// assembly prevents name-resolution mismatches between the pipelines.
func siblingJoinWithMap(rootFile string) (string, sema.SourceMap, error) {
	dir := filepath.Dir(rootFile)
	entries, err := os.ReadDir(dir)
	var sb strings.Builder
	sm := sema.SourceMap{}
	curLine := 1
	funcMainRegex := regexp.MustCompile(`(?m)^\s*func\s+main\s*\(`)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".kark" || entry.Name() == filepath.Base(rootFile) {
				continue
			}
			p := filepath.Join(dir, entry.Name())
			data, rerr := os.ReadFile(p)
			if rerr != nil {
				continue
			}
			if funcMainRegex.MatchString(string(data)) {
				continue
			}
			src := string(data)
			lines := strings.Count(src, "\n") + 1
			for i := 1; i <= lines; i++ {
				sm[curLine] = p
				curLine++
			}
			sb.WriteString(src)
			sb.WriteString("\n\n")
			curLine += 2
		}
	}
	data, rerr := os.ReadFile(rootFile)
	if rerr != nil {
		return "", nil, rerr
	}
	src := string(data)
	lines := strings.Count(src, "\n") + 1
	for i := 1; i <= lines; i++ {
		sm[curLine] = rootFile
		curLine++
	}
	sb.WriteString(src)
	return sb.String(), sm, nil
}

// projectSourceFiles returns the deterministic, deduplicated list of module
// source files to compile for the given root file inside a project: local
// dependency sources (upstream) then the project's sibling modules (root file
// excluded; added separately). It returns an error when the file is not inside
// a resolvable Karkain project.
func projectSourceFiles(rootFile string) ([]string, error) {
	projectDir, err := pm.FindProjectRoot(filepath.Dir(rootFile))
	if err != nil {
		return nil, err
	}
	// Only a Karkain project (karkain.toml) drives project-aware assembly. A
	// go.mod-rooted directory (FindProjectRoot also matches go.mod) must not be
	// treated as a Karkain project.
	manifestPath := filepath.Join(projectDir, pm.ManifestFile)
	if _, serr := os.Stat(manifestPath); serr != nil {
		return nil, fmt.Errorf("not a Karkain project: %w", serr)
	}

	var order []string
	added := map[string]bool{}

	addSortedKark := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		names := []string{}
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".kark" {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			p := filepath.Join(dir, n)
			ap, _ := filepath.Abs(p)
			if added[ap] {
				continue
			}
			added[ap] = true
			order = append(order, p)
		}
	}

	// 1. Dependency sources, upstream (deterministic, sorted-by-name).
	//    Local/workspace deps resolve to their canonical source path; registry
	//    and git deps to their fetched cache dir. Dev-dependencies are
	//    excluded. Sources are only included when the dependency is present on
	//    disk (a registry/git dep must first be `fetch`ed / `update`n).
	for _, ds := range pm.DependencySources(projectDir) {
		if !ds.Cached && ds.Source != string(pm.SourceLocal) && ds.Source != string(pm.SourceWorkspace) {
			continue // registry/git dep not fetched; nothing to assemble
		}
		depRoot := ds.Dir
		if depRoot == "" {
			continue
		}
		if info, ierr := os.Stat(depRoot); ierr == nil && info.IsDir() {
			// Mirror the endorsed project layout: sources live at the
			// dependency root and/or its src/ subdirectory.
			addSortedKark(depRoot)
			addSortedKark(filepath.Join(depRoot, "src"))
		}
	}

	// 2. Project sibling modules (target file's directory), root excluded.
	addSortedKark(filepath.Dir(rootFile))

	return order, nil
}

// projectModuleSources returns only the *non-main* module sources for the
// project containing nodeFile: local dependency sources then sibling modules
// (files that define `func main` excluded). This is the shared application
// scope a test file can exercise. It returns ("", nil) when nodeFile is not
// inside a Karkain project, so flat/test-only builds are unaffected.
func projectModuleSources(nodeFile string) (string, error) {
	clean := filepath.Clean(nodeFile)
	if info, err := os.Stat(clean); err == nil && info.IsDir() {
		clean = filepath.Join(clean, "main.kark")
	}
	projectDir, err := pm.FindProjectRoot(filepath.Dir(clean))
	if err != nil {
		return "", nil
	}
	if _, serr := os.Stat(filepath.Join(projectDir, pm.ManifestFile)); serr != nil {
		return "", nil
	}
	root := filepath.Join(projectDir, "src", "main.kark")
	if _, serr := os.Stat(root); serr != nil {
		root = clean
	}
	files, err := projectSourceFiles(root)
	if err != nil {
		return "", err
	}
	funcMainRegex := regexp.MustCompile(`(?m)^\s*func\s+main\s*\(`)
	var sb strings.Builder
	seen := map[string]bool{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if funcMainRegex.Match(data) {
			continue
		}
		absp, _ := filepath.Abs(f)
		if seen[absp] {
			continue
		}
		seen[absp] = true
		sb.Write(data)
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
}

func parseSource(sourceText string, verbose bool) *parser.Program {
	l := lexer.New(sourceText)
	p := parser.New(l)
	prog := p.ParseProgram()

	if verbose {
		fmt.Printf("=== [Verbose] Parsed AST Statements count: %d ===\n", len(prog.Statements))
	}

	prog = parser.ApplyMacroExpansion(prog)

	if verbose {
		fmt.Printf("=== [Verbose] After Macro Expansion Statements count: %d ===\n", len(prog.Statements))
	}

	return prog
}

// Phase 43: Run borrow checker on a parsed program
func runBorrowCheck(prog *parser.Program) []sema.BorrowError {
	checker := sema.NewBorrowChecker()
	return checker.Check(prog)
}

func printTokenStream(sourceText string) {
	fmt.Println("=== [Verbose] Lexer Token Stream ===")
	l := lexer.New(sourceText)
	for {
		tok := l.NextToken()
		fmt.Printf("Line %d | Type: %-10s | Literal: %q\n", tok.Line, tok.Type, tok.Literal(sourceText))
		if tok.Type == "EOF" {
			break
		}
	}
	fmt.Println("====================================")
}

func extractLineCol(errMsg string) (int, int) {
	re := regexp.MustCompile(`line (\d+)`)
	matches := re.FindStringSubmatch(errMsg)
	line := 1
	if len(matches) > 1 {
		fmt.Sscanf(matches[1], "%d", &line)
	}
	return line, 1
}

func emitGPUShaders(prog *parser.Program, targetFile string, verbose bool) {
	baseName := strings.TrimSuffix(targetFile, filepath.Ext(targetFile))
	for _, stmt := range prog.Statements {
		if kernel, ok := stmt.(*parser.KernelDeclStmt); ok {
			// WGSL
			wgslGen := codegen.NewWGSLGenerator()
			wgsl, err := wgslGen.GenerateWGSL(kernel)
			if err == nil && wgsl != "" {
				wgslFile := baseName + "_" + kernel.Name + ".wgsl"
				os.WriteFile(wgslFile, []byte(wgsl), 0644)
				if verbose {
					fmt.Printf("Emitted WGSL shader: %s\n", wgslFile)
				}
			}

			// OpenCL
			clGen := codegen.NewGPUGenerator()
			openclSrc, err := clGen.GenerateOpenCL(kernel)
			if err == nil && openclSrc != "" {
				clFile := baseName + "_" + kernel.Name + ".cl"
				os.WriteFile(clFile, []byte(openclSrc), 0644)
				if verbose {
					fmt.Printf("Emitted OpenCL shader: %s\n", clFile)
				}
			}

			// SPIR-V
			spirvGen := codegen.NewSPIRVGenerator()
			spirvData, err := spirvGen.GenerateSPIRV(kernel)
			if err == nil && spirvData != nil {
				spvFile := baseName + "_" + kernel.Name + ".spv"
				spvBytes := make([]byte, len(spirvData)*4)
				for i, word := range spirvData {
					binary.LittleEndian.PutUint32(spvBytes[i*4:], word)
				}
				os.WriteFile(spvFile, spvBytes, 0644)
				if verbose {
					fmt.Printf("Emitted SPIR-V binary: %s\n", spvFile)
				}
			}
		}
	}
}

func findTestFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, "_test.kark") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func runSingleTestFile(testFile string, cfg codegen.Config, verbose bool) CommandResult {
	// Validate the test file path
	if err := ValidateKarFile(testFile); err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: err.Error()}
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error reading file: %v", err)}
	}

	src := string(content)
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors) > 0 {
		return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("Parse errors: %s", strings.Join(p.Errors, "; "))}
	}

	prog = parser.ApplyMacroExpansion(prog)

	// Project-aware test scope: gather the shared module sources (local deps +
	// sibling modules, no `func main`) so test functions can call the code they
	// are testing without a language-level import. Flat builds (no project)
	// get an empty scope and behave exactly as before.
	var moduleStmts []parser.Node
	if modSrc, err := projectModuleSources(testFile); err == nil && modSrc != "" {
		ml := lexer.New(modSrc)
		mp := parser.New(ml)
		mprog := mp.ParseProgram()
		if len(mp.Errors) == 0 {
			mprog = parser.ApplyMacroExpansion(mprog)
			moduleStmts = mprog.Statements
		} else {
			return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("Module parse errors: %s", strings.Join(mp.Errors, "; "))}
		}
	}

	// Discover test functions
	testFuncs := discoverTestFunctions(prog)
	if len(testFuncs) == 0 {
		if verbose {
			fmt.Printf("  No test functions found in %s\n", filepath.Base(testFile))
		}
		return CommandResult{ExitCode: 0, Message: "No test functions found"}
	}

	// Run each test function
	failed := 0
	for _, fn := range testFuncs {
		if verbose {
			fmt.Printf("  Running test: %s... ", fn.Name)
		}

		// Create a mini-program with the project module scope plus this function
		// and a main that calls it.
		stmts := make([]parser.Node, 0, len(moduleStmts)+2)
		stmts = append(stmts, moduleStmts...)
		stmts = append(stmts,
			&parser.FuncDecl{
				Name:   fn.Name,
				Params: fn.Params,
				Body:   fn.Body,
			},
			&parser.FuncDecl{
				Name:   "main",
				Params: []string{},
				Body: []parser.Node{
					&parser.ExprStmt{
						Expression: &parser.CallExpr{
							Function: fn.Name,
							Args:     []parser.Node{},
						},
					},
				},
			},
		)
		miniProg := &parser.Program{Statements: stmts}

		testCfg := cfg
		testCfg.RunAfter = true
		cg := codegen.New(testCfg)

		tmpCFile := testFile + ".test.c"
		defer os.Remove(tmpCFile)

		if err := cg.GenerateAndCompile(miniProg, tmpCFile); err != nil {
			failed++
			if verbose {
				fmt.Printf("FAIL (%v)\n", err)
			}
		} else {
			if verbose {
				fmt.Println("PASS")
			}
		}
	}

	if failed > 0 {
		return CommandResult{ExitCode: ExitTest, Message: fmt.Sprintf("%d test(s) failed", failed)}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: "All tests passed"}
}

func discoverTestFunctions(prog *parser.Program) []*parser.FuncDecl {
	var testFuncs []*parser.FuncDecl
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*parser.FuncDecl); ok {
			if strings.HasPrefix(fn.Name, "test_") {
				testFuncs = append(testFuncs, fn)
			}
		}
	}
	return testFuncs
}

// HasKernelDecl checks if a program contains kernel declarations
func HasKernelDecl(prog *parser.Program) bool {
	for _, stmt := range prog.Statements {
		if _, ok := stmt.(*parser.KernelDeclStmt); ok {
			return true
		}
	}
	return false
}

// RunProcess executes a command and streams its output
func RunProcess(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
