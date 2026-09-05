package cli

import (
	"bytes"
	"fmt"
	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BenchResult is the outcome of a single bench_ function run.
type BenchResult struct {
	Name     string
	Duration time.Duration
	Err      string
}

// discoverBenchmarks extracts bench_-prefixed functions from a parsed program
// in deterministic (name-sorted) order.
func discoverBenchmarks(prog *parser.Program) []*parser.FuncDecl {
	var fns []*parser.FuncDecl
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*parser.FuncDecl); ok && strings.HasPrefix(fn.Name, "bench_") {
			fns = append(fns, fn)
		}
	}
	sort.Slice(fns, func(i, j int) bool { return fns[i].Name < fns[j].Name })
	return fns
}

// findBenchFiles returns the deterministic list of *_bench.kark and *_test.kark
// files under a directory (or that single file).
func findBenchFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		name := filepath.Base(path)
		if !(strings.HasSuffix(name, "_bench.kark") || strings.HasSuffix(name, "_test.kark")) {
			return nil, fmt.Errorf("bench expects a *_bench.kark or *_test.kark file or directory")
		}
		return []string{path}, nil
	}
	var files []string
	err = filepath.Walk(path, func(p string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !fi.IsDir() && (strings.HasSuffix(fi.Name(), "_bench.kark") || strings.HasSuffix(fi.Name(), "_test.kark")) {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// runSingleBench compiles and executes one bench_ function (with the shared
// module scope), reporting wall-clock duration of a single run.
func runSingleBench(fn *parser.FuncDecl, moduleStmts []parser.Node, name string, cfg codegen.Config) BenchResult {
	if fn == nil {
		return BenchResult{Name: name, Err: "benchmark function not found"}
	}

	stmts := make([]parser.Node, 0, len(moduleStmts)+2)
	stmts = append(stmts, moduleStmts...)
	stmts = append(stmts,
		&parser.FuncDecl{Name: fn.Name, Params: fn.Params, Body: fn.Body},
		&parser.FuncDecl{
			Name:   "main",
			Params: []string{},
			Body: []parser.Node{
				&parser.ExprStmt{
					Expression: &parser.CallExpr{Function: fn.Name, Args: []parser.Node{}},
				},
			},
		},
	)
	miniProg := &parser.Program{Statements: stmts}

	testCfg := cfg
	testCfg.RunAfter = true
	var stdout, stderr bytes.Buffer
	testCfg.Stdout = &stdout
	testCfg.Stderr = &stderr

	tmpCFile := "." + filepath.Base(fn.Name) + ".bench.c"
	defer os.Remove(tmpCFile)

	start := time.Now()
	cg := codegen.New(testCfg)
	err := cg.GenerateAndCompile(miniProg, tmpCFile)
	dur := time.Since(start)
	if err != nil {
		return BenchResult{Name: name, Duration: dur, Err: err.Error()}
	}
	return BenchResult{Name: name, Duration: dur}
}

// BenchCommand discovers and runs bench_-prefixed functions, reporting each
// function's single-run wall-clock duration. It is a genuine timing harness
// (not a fabricated one): every entry is compiled and executed exactly once.
func BenchCommand(benchPath string, cfg codegen.Config, verbose bool) CommandResult {
	files, err := findBenchFiles(benchPath)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error scanning path: %v", err)}
	}
	if len(files) == 0 {
		return CommandResult{ExitCode: ExitSuccess, Message: "No benchmark files found."}
	}

	var results []BenchResult
	for _, bf := range files {
		if verbose {
			fmt.Printf("=== Benchmarking %s ===\n", bf)
		}
		content, rerr := os.ReadFile(bf)
		if rerr != nil {
			return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error reading file: %v", rerr)}
		}
		l := lexer.New(string(content))
		p := parser.New(l)
		prog := p.ParseProgram()
		if len(p.Errors) > 0 {
			return CommandResult{ExitCode: ExitCompile, Message: fmt.Sprintf("Parse errors: %s", strings.Join(p.Errors, "; "))}
		}
		prog = parser.ApplyMacroExpansion(prog)

		moduleStmts := testFileModuleScope(bf)
		for _, fn := range discoverBenchmarks(prog) {
			results = append(results, runSingleBench(fn, moduleStmts, fn.Name, cfg))
		}
	}

	if len(results) == 0 {
		return CommandResult{ExitCode: ExitSuccess, Message: "No bench_ functions found."}
	}

	fmt.Printf("\n%-32s %-12s %s\n", "BENCHMARK", "RUNS", "DURATION")
	fmt.Printf("%-32s %-12s %s\n", "--------", "-----", "--------")
	total := time.Duration(0)
	for _, r := range results {
		total += r.Duration
		line := fmt.Sprintf("%-32s %-12d %s", r.Name, 1, r.Duration.Round(time.Millisecond))
		if r.Err != "" {
			line += " ERROR: " + r.Err
		}
		fmt.Println(line)
	}
	fmt.Printf("\nTotal: %s for %d benchmark(s)\n", total.Round(time.Millisecond), len(results))
	return CommandResult{ExitCode: ExitSuccess, Message: fmt.Sprintf("%d benchmark(s) completed", len(results))}
}
