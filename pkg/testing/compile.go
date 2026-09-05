package testing

import (
	"sort"
	"time"

	"karkain/pkg/diagnostics"
)

// KindCompile classifies a compile-pass/compile-fail corpus case (KTF-002).
const KindCompile Kind = "compile"

// CompileExpect is the expected outcome of a compile case.
type CompileExpect string

const (
	// ExpectPass requires the corpus file to pass the full front end and
	// compile successfully through the native C backend.
	ExpectPass CompileExpect = "pass"
	// ExpectFail requires the corpus file to produce a diagnostic of the
	// declared error-code class (and, when set, whose message contains the
	// declared substring). No failure -> the case itself fails.
	ExpectFail CompileExpect = "fail"
)

// CompileCase is one entry of the compile-pass/compile-fail corpus. It is
// declared by a manifest and exercised by the corpus runner.
type CompileCase struct {
	// File is the corpus source path, relative to the corpus root.
	File string
	// Expect is the required outcome (pass or fail).
	Expect CompileExpect
	// Code is the expected diagnostic class for fail cases (e.g. E-K-SYN).
	// Ignored for pass cases.
	Code diagnostics.Code
	// Message is an optional substring the failure diagnostic must contain.
	// Ignored for pass cases and when empty.
	Message string
}

// Diagnostic is a structured compiler finding captured from the front end:
// location + stable error-code class + message.
type Diagnostic struct {
	File    string
	Line    int
	Col     int
	Code    string
	Message string
}

// CompileResult is the outcome of exercising one CompileCase.
type CompileResult struct {
	ID          string
	File        string
	Expect      CompileExpect
	Status      Status
	Duration    time.Duration
	Diagnostics []Diagnostic
	Error       string
}

// Passed reports whether the case met its expectation.
func (r CompileResult) Passed() bool { return r.Status == StatusPass }

// compileID is the stable identity "<expect>:<file>".
func compileID(c CompileCase) string {
	return string(c.Expect) + ":" + c.File
}

// CompileSummary aggregates corpus results deterministically.
type CompileSummary struct {
	Total   int
	Passed  int
	Failed  int
	Skipped int
}

// SummarizeCompile aggregates the given results.
func SummarizeCompile(results []CompileResult) CompileSummary {
	s := CompileSummary{Total: len(results)}
	for _, r := range results {
		switch r.Status {
		case StatusPass:
			s.Passed++
		case StatusSkip:
			s.Skipped++
		default:
			s.Failed++
		}
	}
	return s
}

// SortCompileByID returns a stable, deterministic ordering of cases by
// <expect>:<file> so runner output never depends on filesystem order or map
// iteration.
func SortCompileByID(cases []CompileCase) []CompileCase {
	out := make([]CompileCase, len(cases))
	copy(out, cases)
	sort.SliceStable(out, func(i, j int) bool { return compileID(out[i]) < compileID(out[j]) })
	return out
}
