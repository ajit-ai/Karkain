package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"karkain/pkg/codegen"
	"karkain/pkg/diagnostics"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"karkain/pkg/sema"
	"karkain/pkg/testing"
)

// CompileManifest is the on-disk description of the compile-pass/compile-fail
// corpus. It is the single source of truth for what the corpus asserts.
type CompileManifest struct {
	Cases []manifestCompileCase `json:"cases"`
}

// manifestCompileCase is the JSON form of a CompileCase.
type manifestCompileCase struct {
	File    string `json:"file"`
	Expect  string `json:"expect"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// CompileManifestFile is the manifest filename inside a corpus directory.
const CompileManifestFile = "manifest.json"

// DefaultCompileCorpus is the repository-bundled corpus directory.
const DefaultCompileCorpus = "testdata/compile"

// loadCompileManifest reads and validates a corpus manifest, resolving every
// referenced file against the corpus root. Missing files are hard errors so a
// stale manifest cannot silently reduce coverage.
func loadCompileManifest(corpusDir string) ([]testing.CompileCase, error) {
	raw, err := os.ReadFile(filepath.Join(corpusDir, CompileManifestFile))
	if err != nil {
		return nil, fmt.Errorf("cannot read corpus manifest: %v", err)
	}
	var m CompileManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("corpus manifest is not valid JSON: %v", err)
	}
	var cases []testing.CompileCase
	for _, c := range m.Cases {
		if c.File == "" {
			return nil, fmt.Errorf("corpus case with empty file")
		}
		switch c.Expect {
		case "pass", "fail":
		default:
			return nil, fmt.Errorf("corpus case %s: unknown expect %q", c.File, c.Expect)
		}
		full := filepath.Join(corpusDir, c.File)
		if _, serr := os.Stat(full); serr != nil {
			return nil, fmt.Errorf("corpus case %s: file missing: %v", c.File, serr)
		}
		cases = append(cases, testing.CompileCase{
			File:    c.File,
			Expect:  testing.CompileExpect(c.Expect),
			Code:    diagnostics.Code(c.Code),
			Message: c.Message,
		})
	}
	return testing.SortCompileByID(cases), nil
}

// captureFrontendDiagnostics runs the complete deterministic front end —
// lexer/parser, macro expansion, name resolution, kernel sema, borrow checker —
// and returns every finding as a structured diagnostic. It mirrors the lint
// pipeline so the corpus asserts exactly what a user would see. Corpus files
// compile standalone (no project scope), so the resolver receives no cross-file
// source map.
func captureFrontendDiagnostics(targetFile, source string) []testing.Diagnostic {
	l := lexer.New(source)
	p := parser.New(l)
	prog := p.ParseProgram()

	var diags []testing.Diagnostic

	if len(p.Errors) > 0 {
		for _, pe := range p.Errors {
			line, col := extractLineCol(pe)
			diags = append(diags, testing.Diagnostic{
				File: targetFile, Line: line, Col: col,
				Code: string(diagnostics.CodeSyntax), Message: pe,
			})
		}
		return diags
	}

	prog = parser.ApplyMacroExpansion(prog)

	resolver := sema.NewResolver(prog, nil)
	if resolveErrs := resolver.Resolve(); len(resolveErrs) > 0 {
		for _, re := range resolveErrs {
			diags = append(diags, testing.Diagnostic{
				File: targetFile, Line: re.Line, Col: 1,
				Code: string(diagnostics.CodeResolve), Message: re.Msg,
			})
		}
		return diags
	}

	analyzer := sema.NewKernelAnalyzer()
	for _, stmt := range prog.Statements {
		if kernel, ok := stmt.(*parser.KernelDeclStmt); ok {
			for _, ke := range analyzer.AnalyzeKernel(kernel) {
				diags = append(diags, testing.Diagnostic{
					File: targetFile, Line: 1, Col: 1,
					Code: string(diagnostics.CodeSema), Message: ke.Error(),
				})
			}
		}
	}
	if len(diags) > 0 {
		return diags
	}

	for _, be := range runBorrowCheck(prog) {
		diags = append(diags, testing.Diagnostic{
			File: targetFile, Line: int(be.Line), Col: 1,
			Code: string(diagnostics.CodeBorrow), Message: be.Message,
		})
	}
	return diags
}

// runPassCase compiles a corpus file through the native backend (front end is a
// cheap pre-check). The case passes only when the front end is clean AND codegen
// produces output. When no C compiler is installed the requirement cannot be
// met deterministically, so the case is skipped rather than failed.
func runPassCase(c testing.CompileCase, corpusDir string, canCompile bool, cfg codegen.Config) testing.CompileResult {
	res := testing.CompileResult{ID: string(c.Expect) + ":" + c.File, File: c.File, Expect: c.Expect, Status: testing.StatusFail}
	if !canCompile {
		res.Status = testing.StatusSkip
		res.Error = "skipped: no C compiler found (GCC, Clang, or MSVC cl.exe)"
		return res
	}
	content := readCorpusFile(corpusDir, c.File)

	start := time.Now()
	defer func() { res.Duration = time.Since(start) }()

	if front := captureFrontendDiagnostics(c.File, content); len(front) > 0 {
		res.Diagnostics = front
		res.Error = fmt.Sprintf("expected compile-pass but front end reported %d diagnostic(s)", len(front))
		return res
	}

	l := lexer.New(content)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		res.Error = "front end produced parse errors after clean capture"
		return res
	}
	prog = parser.ApplyMacroExpansion(prog)

	compileCfg := cfg
	compileCfg.CompileOnly = true
	var stdout, stderr bytes.Buffer
	compileCfg.Stdout = &stdout
	compileCfg.Stderr = &stderr

	tmpCFile := filepath.Join(os.TempDir(), "karkain-corpus-"+strings.ReplaceAll(c.File, "/", "_")+".pass.c")
	defer os.Remove(tmpCFile)

	cg := codegen.New(compileCfg)
	if err := cg.GenerateAndCompile(prog, tmpCFile); err != nil {
		res.Error = fmt.Sprintf("compile-pass failed at codegen: %v", err)
		if v := stderr.String(); v != "" {
			res.Error += "\n" + v
		}
		return res
	}
	res.Status = testing.StatusPass
	return res
}

// runFailCase exercises the front end and passes only when a diagnostic of the
// declared class (and message substring, when given) is produced.
func runFailCase(c testing.CompileCase, corpusDir string, verbose bool) testing.CompileResult {
	res := testing.CompileResult{ID: string(c.Expect) + ":" + c.File, File: c.File, Expect: c.Expect, Status: testing.StatusFail}
	content := readCorpusFile(corpusDir, c.File)

	start := time.Now()
	defer func() { res.Duration = time.Since(start) }()

	diags := captureFrontendDiagnostics(c.File, content)
	res.Diagnostics = diags
	if len(diags) == 0 {
		res.Error = "expected compile-fail but no diagnostic was produced"
		return res
	}

	for _, d := range diags {
		if c.Code != "" && d.Code != string(c.Code) {
			continue
		}
		if c.Message != "" && !strings.Contains(d.Message, c.Message) {
			continue
		}
		res.Status = testing.StatusPass
		return res
	}
	res.Error = fmt.Sprintf("no diagnostic matched code %q message %q (got %d diagnostic(s))",
		c.Code, c.Message, len(diags))
	return res
}

// readCorpusFile reads one corpus source file.
func readCorpusFile(corpusDir, file string) string {
	raw, err := os.ReadFile(filepath.Join(corpusDir, file))
	if err != nil {
		return ""
	}
	return string(raw)
}

// RunCompileCorpus exercises every case in the corpus, printing one line per
// case and returning the aggregate summary. Deterministic order and output.
func RunCompileCorpus(corpusDir string, verbose bool, cfg codegen.Config) testing.CompileSummary {
	cases, err := loadCompileManifest(corpusDir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return testing.CompileSummary{Total: 1, Failed: 1} // Error state is a failure
	}

	if len(cases) == 0 {
		fmt.Println("Compile corpus is empty")
		return testing.CompileSummary{}
	}

	fmt.Printf("Running %d compile corpus case(s)\n", len(cases))
	canCompile := haveCCompiler()
	var results []testing.CompileResult
	for _, c := range cases {
		var r testing.CompileResult
		if c.Expect == testing.ExpectPass {
			r = runPassCase(c, corpusDir, canCompile, cfg)
		} else {
			r = runFailCase(c, corpusDir, verbose)
		}
		results = append(results, r)

		state := "PASS"
		switch {
		case r.Passed():
			state = "PASS"
		case r.Status == testing.StatusSkip:
			state = "SKIP"
		default:
			state = "FAIL"
		}
		line := fmt.Sprintf("  %-4s %-24s %s", state, c.Expect, c.File)
		if !r.Passed() {
			line += "  " + r.Error
		}
		fmt.Println(line)
		if verbose && r.Passed() && len(r.Diagnostics) > 0 {
			for _, d := range r.Diagnostics {
				fmt.Printf("        [%s] line %d: %s\n", d.Code, d.Line, d.Message)
			}
		}
	}

	s := testing.SummarizeCompile(results)
	summary := fmt.Sprintf("\n=== Compile Corpus Summary: %d cases, %d passed, %d failed, %d skipped ===\n",
		s.Total, s.Passed, s.Failed, s.Skipped)
	if s.Failed > 0 {
		summary = fmt.Sprintf("\n=== Compile Corpus Summary: %d cases, %d passed, %d FAILED, %d skipped ===\n",
			s.Total, s.Passed, s.Failed, s.Skipped)
	}
	fmt.Print(summary)
	return s
}

// haveCCompiler reports whether a C toolchain that codegen accepts is on PATH.
// Mirrors the discovery codegen uses so the corpus never reports a spurious
// failure for the pass cases.
func haveCCompiler() bool {
	for _, name := range []string{"gcc", "clang", "cc"} {
		if _, err := exec.LookPath(name); err == nil {
			return true
		}
	}
	if _, err := exec.LookPath("cl.exe"); err == nil {
		return true
	}
	return false
}
