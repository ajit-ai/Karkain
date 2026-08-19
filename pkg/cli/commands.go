package cli

import (
	"encoding/binary"
	"fmt"
	"karkain/pkg/codegen"
	"karkain/pkg/diagnostics"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"karkain/pkg/sema"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const versionString = "Karkain Compiler v1.0.0 (%s/%s, LSP Engine & IDE Tooling)"

// CommandResult holds the outcome of a CLI command
type CommandResult struct {
	ExitCode int
	Message  string
}

// RunCommand parses, type-checks, transpiles, compiles and executes a .kar file
func RunCommand(targetFile string, cfg codegen.Config, verbose bool) CommandResult {
	if err := ValidateKarFile(targetFile); err != nil {
		return CommandResult{ExitCode: 1, Message: err.Error()}
	}

	sourceText, err := loadSourceWithSiblings(targetFile)
	if err != nil {
		return CommandResult{ExitCode: 1, Message: fmt.Sprintf("Error reading file: %v", err)}
	}

	if verbose {
		printTokenStream(sourceText)
	}

	prog := parseSource(sourceText, verbose)
	if prog == nil {
		return CommandResult{ExitCode: 1, Message: "Parse failed"}
	}

	cfg.RunAfter = true
	cg := codegen.New(cfg)

	if err := cg.GenerateAndCompile(prog, targetFile); err != nil {
		return CommandResult{ExitCode: 1, Message: fmt.Sprintf("Execution Error: %v", err)}
	}
	return CommandResult{ExitCode: 0, Message: ""}
}

// BuildCommand compiles a .kar file into a native executable
func BuildCommand(targetFile string, outputPath string, cfg codegen.Config, verbose bool) CommandResult {
	if err := ValidateKarFile(targetFile); err != nil {
		return CommandResult{ExitCode: 1, Message: err.Error()}
	}

	sourceText, err := loadSourceWithSiblings(targetFile)
	if err != nil {
		return CommandResult{ExitCode: 1, Message: fmt.Sprintf("Error reading file: %v", err)}
	}

	if verbose {
		printTokenStream(sourceText)
	}

	prog := parseSource(sourceText, verbose)
	if prog == nil {
		return CommandResult{ExitCode: 1, Message: "Parse failed"}
	}

	cfg.RunAfter = false
	cfg.CompileOnly = true
	if outputPath != "" {
		cfg.OutputPath = outputPath
	}

	// Emit GPU shaders if kernels are present
	emitGPUShaders(prog, targetFile, verbose)

	cg := codegen.New(cfg)
	if err := cg.GenerateAndCompile(prog, targetFile); err != nil {
		return CommandResult{ExitCode: 1, Message: fmt.Sprintf("Build Error: %v", err)}
	}
	return CommandResult{ExitCode: 0, Message: "Build successful."}
}

// CheckCommand validates a .kar file without producing output binaries
func CheckCommand(targetFile string, verbose bool) CommandResult {
	if err := ValidateKarFile(targetFile); err != nil {
		return CommandResult{ExitCode: 1, Message: err.Error()}
	}

	sourceText, err := os.ReadFile(targetFile)
	if err != nil {
		return CommandResult{ExitCode: 1, Message: fmt.Sprintf("Error reading file: %v", err)}
	}

	src := string(sourceText)
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
		return CommandResult{ExitCode: 1, Message: fmt.Sprintf("%d parse error(s) found", len(p.Errors))}
	}

	// Apply macro expansion
	prog = parser.ApplyMacroExpansion(prog)

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
		return CommandResult{ExitCode: 1, Message: fmt.Sprintf("%d semantic error(s) found", errorCount)}
	}

	if verbose {
		fmt.Printf("Check passed: %s (%d statements)\n", targetFile, len(prog.Statements))
	}
	return CommandResult{ExitCode: 0, Message: "Check passed."}
}

// TestCommand discovers and runs *_test.kar files and functions prefixed with test_ or @test
func TestCommand(testPath string, cfg codegen.Config, verbose bool) CommandResult {
	info, err := os.Stat(testPath)
	if err != nil {
		return CommandResult{ExitCode: 1, Message: fmt.Sprintf("Error accessing path: %v", err)}
	}

	var testFiles []string
	if !info.IsDir() {
		testFiles = []string{testPath}
	} else {
		testFiles, err = findTestFiles(testPath)
		if err != nil {
			return CommandResult{ExitCode: 1, Message: fmt.Sprintf("Error scanning directory: %v", err)}
		}
	}

	if len(testFiles) == 0 {
		return CommandResult{ExitCode: 0, Message: "No test files found."}
	}

	totalTests := 0
	passedTests := 0
	failedTests := 0

	for _, tf := range testFiles {
		totalTests++
		if verbose {
			fmt.Printf("=== Running tests in %s ===\n", tf)
		}

		result := runSingleTestFile(tf, cfg, verbose)
		if result.ExitCode == 0 {
			passedTests++
			if verbose {
				fmt.Printf("  PASS: %s\n", filepath.Base(tf))
			}
		} else {
			failedTests++
			fmt.Printf("  FAIL: %s - %s\n", filepath.Base(tf), result.Message)
		}
	}

	summary := fmt.Sprintf("\n=== Test Summary: %d total, %d passed, %d failed ===", totalTests, passedTests, failedTests)
	fmt.Println(summary)

	if failedTests > 0 {
		return CommandResult{ExitCode: 1, Message: summary}
	}
	return CommandResult{ExitCode: 0, Message: summary}
}

// --- internal helpers ---

// ValidateKarFile checks that the given path is a non-empty .kar file path
func ValidateKarFile(path string) error {
	if path == "" {
		return fmt.Errorf("No input .kar file specified")
	}

	// Check if the path is a known subcommand (e.g., "transpile", "build", "run")
	subcommands := []string{"transpile", "build", "run", "check", "test", "lsp", "init", "add", "fetch"}
	for _, cmd := range subcommands {
		if path == cmd {
			return fmt.Errorf("Input file must be a .kar file: %s", path)
		}
	}

	// Clean the path to handle Windows backslashes and relative paths
	cleanPath := filepath.Clean(path)

	// Check if the path is a directory
	info, err := os.Stat(cleanPath)
	if err == nil && info.IsDir() {
		// If it's a directory, assume main.kar inside it
		cleanPath = filepath.Join(cleanPath, "main.kar")
	}

	if strings.ToLower(filepath.Ext(cleanPath)) != ".kar" {
		return fmt.Errorf("Input file must be a .kar file: %s", path)
	}
	return nil
}

func loadSourceWithSiblings(targetFile string) (string, error) {
	// Handle directories by appending main.kar
	cleanPath := filepath.Clean(targetFile)
	info, err := os.Stat(cleanPath)
	if err == nil && info.IsDir() {
		cleanPath = filepath.Join(cleanPath, "main.kar")
	}

	dir := filepath.Dir(cleanPath)
	entries, err := os.ReadDir(dir)
	var fullContent strings.Builder

	funcMainRegex := regexp.MustCompile(`(?m)^\s*func\s+main\s*\(`)

	if err == nil && len(entries) > 1 {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".kar" && entry.Name() != filepath.Base(cleanPath) {
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

func printTokenStream(sourceText string) {
	fmt.Println("=== [Verbose] Lexer Token Stream ===")
	l := lexer.New(sourceText)
	for {
		tok := l.NextToken()
		fmt.Printf("Line %d | Type: %-10s | Literal: %q\n", tok.Line, tok.Type, tok.Literal)
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
		if !info.IsDir() && strings.HasSuffix(path, "_test.kar") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func runSingleTestFile(testFile string, cfg codegen.Config, verbose bool) CommandResult {
	// Validate the test file path
	if err := ValidateKarFile(testFile); err != nil {
		return CommandResult{ExitCode: 1, Message: err.Error()}
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		return CommandResult{ExitCode: 1, Message: fmt.Sprintf("Error reading file: %v", err)}
	}

	src := string(content)
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors) > 0 {
		return CommandResult{ExitCode: 1, Message: fmt.Sprintf("Parse errors: %s", strings.Join(p.Errors, "; "))}
	}

	prog = parser.ApplyMacroExpansion(prog)

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

		// Create a mini-program with only this function and a main that calls it
		miniProg := &parser.Program{
			Statements: []parser.Node{
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
			},
		}

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
		return CommandResult{ExitCode: 1, Message: fmt.Sprintf("%d test(s) failed", failed)}
	}
	return CommandResult{ExitCode: 0, Message: "All tests passed"}
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
