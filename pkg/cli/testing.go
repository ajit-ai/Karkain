package cli

import (
	"bytes"
	"fmt"
	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"karkain/pkg/testing"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultTestTimeout = 30 * time.Second

// discoverTestCases extracts Karkain-native test declarations from a parsed
// program and returns them in deterministic (name-sorted) order. KTF-001
// recognizes the `test_`-prefixed function convention as the native test
// declaration form (reconciled with the existing runner, not a new grammar).
func discoverTestCases(prog *parser.Program, file string) []testing.TestCase {
	var cases []testing.TestCase
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*parser.FuncDecl); ok {
			if strings.HasPrefix(fn.Name, "test_") {
				cases = append(cases, testing.TestCase{
					ID:   testIdentity(file, fn.Name),
					Name: fn.Name,
					File: file,
					Line: fn.Line,
					Kind: testing.KindUnit,
				})
			}
		}
	}
	return testing.SortByID(cases)
}

// testIdentity builds the stable, deterministic identifier for a test as
// "<module>:<test-name>".
func testIdentity(file, name string) string {
	base := filepath.Base(file)
	if base == "" {
		return name
	}
	return base + ":" + name
}

// lookupTestFunc finds the declaration of a discovered test within the test
// program.
func lookupTestFunc(prog *parser.Program, name string) *parser.FuncDecl {
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*parser.FuncDecl); ok && fn.Name == name {
			return fn
		}
	}
	return nil
}

// testFileOwnScope collects the test file's own non-test, non-main top-level
// declarations so that self-contained *_test.kark files can call helper
// functions defined in the same file. This is the conformance-corpus scope
// model: a test file owns the code it tests.
func testFileOwnScope(prog *parser.Program) []parser.Node {
	var scope []parser.Node
	for _, stmt := range prog.Statements {
		if fn, ok := stmt.(*parser.FuncDecl); ok {
			if strings.HasPrefix(fn.Name, "test_") || fn.Name == "main" {
				continue
			}
		}
		scope = append(scope, stmt)
	}
	return scope
}

// runSingleTest compiles and executes a single test function (with the shared
// module scope), capturing its output, duration and any assertion failure.
// A test that exceeds the timeout is terminated with a timeout failure.
func runSingleTest(fn *parser.FuncDecl, ownStmts, moduleStmts []parser.Node, tc testing.TestCase, cfg codegen.Config) testing.TestResult {
	res := testing.TestResult{ID: tc.ID, Name: tc.Name, Status: testing.StatusFail}
	if fn == nil {
		res.Failure = &testing.Failure{Message: "test function not found"}
		return res
	}

	stmts := make([]parser.Node, 0, len(moduleStmts)+len(ownStmts)+2)
	stmts = append(stmts, moduleStmts...)
	stmts = append(stmts, ownStmts...)
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
	var stdout, stderr bytes.Buffer
	testCfg.Stdout = &stdout
	testCfg.Stderr = &stderr

	tmpCFile := tc.File + ".test.c"
	defer os.Remove(tmpCFile)

	type testExecResult struct {
		err error
	}
	ch := make(chan testExecResult, 1)

	start := time.Now()
	go func() {
		cg := codegen.New(testCfg)
		err := cg.GenerateAndCompile(miniProg, tmpCFile)
		ch <- testExecResult{err: err}
	}()

	select {
	case result := <-ch:
		res.Duration = time.Since(start)
		res.Stdout = stdout.String()
		res.Stderr = stderr.String()

		if result.err != nil {
			if f := testing.ParseAssertionFailure(res.Stderr); f != nil {
				res.Failure = f
			} else {
				res.Failure = &testing.Failure{Message: result.err.Error()}
			}
			return res
		}
		res.Status = testing.StatusPass
		return res

	case <-time.After(defaultTestTimeout):
		res.Duration = defaultTestTimeout
		res.Failure = &testing.Failure{Message: fmt.Sprintf("test timed out after %s", defaultTestTimeout)}
		return res
	}
}

// runTestsInFile discovers, filters and runs the tests declared in a single
// file, returning the structured results in deterministic order. A filter of ""
// selects all tests.
func runTestsInFile(testFile string, cfg codegen.Config, verbose bool, filter string) []testing.TestResult {
	var results []testing.TestResult

	content, err := os.ReadFile(testFile)
	if err != nil {
		if verbose {
			fmt.Printf("  error reading %s: %v\n", testFile, err)
		}
		return results
	}
	l := lexer.New(string(content))
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		if verbose {
			fmt.Printf("  parse errors in %s: %s\n", testFile, strings.Join(p.Errors, "; "))
		}
		return results
	}
	prog = parser.ApplyMacroExpansion(prog)

	cases := testing.Filter(discoverTestCases(prog, testFile), filter)
	if len(cases) == 0 {
		return results
	}

	moduleStmts := testFileModuleScope(testFile)
	ownStmts := testFileOwnScope(prog)

	for _, tc := range cases {
		fn := lookupTestFunc(prog, tc.Name)
		r := runSingleTest(fn, ownStmts, moduleStmts, tc, cfg)
		results = append(results, r)
		if verbose {
			printSingleResult(r)
		}
	}
	return results
}

// testFileModuleScope assembles the shared Karkain module scope for a test
// file (project local deps + sibling modules), returning the parsed statements
// or nil when there is no project scope.
func testFileModuleScope(testFile string) []parser.Node {
	if modSrc, err := projectModuleSources(testFile); err == nil && modSrc != "" {
		ml := lexer.New(modSrc)
		mp := parser.New(ml)
		mprog := mp.ParseProgram()
		if len(mp.Errors) == 0 {
			return parser.ApplyMacroExpansion(mprog).Statements
		}
	}
	return nil
}

// printSingleResult renders one test's outcome to stdout.
func printSingleResult(r testing.TestResult) {
	line := fmt.Sprintf("  %-4s %-32s %s", string(r.Status), r.Name, r.Duration.Round(time.Millisecond))
	if r.Failed() && r.Failure != nil {
		line += ": " + r.Failure.Message
	}
	fmt.Println(line)
}

// printTestSummary renders the aggregate summary line.
func printTestSummary(s testing.Summary) {
	plural := "tests"
	if s.Total == 1 {
		plural = "test"
	}
	fmt.Printf("\n=== Test Summary: %d %s, %d passed, %d failed, %d skipped ===\n",
		s.Total, plural, s.Passed, s.Failed, s.Skipped)
}

// appliedFilterLabel returns a human-readable note when a filter was applied,
// or "" when all tests were selected.
func appliedFilterLabel(filter string) string {
	if filter == "" {
		return ""
	}
	return fmt.Sprintf("filter '%s'", filter)
}

// summaryLine renders the compact result line used as the command message.
func summaryLine(s testing.Summary) string {
	return fmt.Sprintf("%d passed; %d failed; %d skipped; %d total",
		s.Passed, s.Failed, s.Skipped, s.Total)
}

// printResults renders all results (used by the non-verbose default path).
func printResults(label string, results []testing.TestResult) {
	if label != "" {
		fmt.Printf("Running %d tests (%s)\n\n", len(results), label)
	}
	for _, r := range results {
		printSingleResult(r)
	}
}
