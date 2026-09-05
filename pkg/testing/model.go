// Package testing defines the Karkain-native test model: structured test
// cases, results, statuses and summaries shared by the toolchain (KTF-001).
//
// The language owns test semantics and assertions; this package owns the
// toolchain-side representation so future KTF phases (compile-pass, compile-
// fail, diagnostics, conformance, runtime, ...) can add new test kinds without
// rewriting the model.
package testing

import (
	"sort"
	"strings"
	"time"
)

// Status is the outcome of a single test execution.
type Status string

const (
	// StatusPass indicates the test compiled and ran without failing.
	StatusPass Status = "PASS"
	// StatusFail indicates the test failed (an assertion aborted it, or it did
	// not compile/run).
	StatusFail Status = "FAIL"
	// StatusSkip indicates the test was intentionally not run (e.g. excluded
	// by a filter). Reserved in KTF-001; retained for future kinds.
	StatusSkip Status = "SKIP"
)

// Kind classifies the style of a test. KTF-001 implements only UNIT; the kind
// is an extension seam for later phases.
type Kind string

const (
	// KindUnit is a Karkain-level unit test (default for KTF-001).
	KindUnit Kind = "unit"
)

// TestCase is a single, discretely addressable test discovered from Karkain
// source.
type TestCase struct {
	// ID is the stable, deterministic identity of the test (module-qualified).
	ID string
	// Name is the human-readable test name (function name, minus any prefix).
	Name string
	// File is the source file the test was declared in.
	File string
	// Line is the declaration line in File (0 when unavailable).
	Line int
	// Kind classifies the test (always KindUnit in KTF-001).
	Kind Kind
}

// Failure carries structured information about a failing test.
type Failure struct {
	// Message is the diagnostic headline (e.g. "assertion failed: assert_eq").
	Message string
	// Expected is the expected value as text, when available.
	Expected string
	// Actual is the observed value as text, when available.
	Actual string
	// Location is a source reference, when available.
	Location string
}

// TestResult is the outcome of executing a TestCase.
type TestResult struct {
	ID       string
	Name     string
	Status   Status
	Duration time.Duration
	Failure  *Failure
	Stdout   string
	Stderr   string
}

// Passed reports whether the result is a pass.
func (r TestResult) Passed() bool { return r.Status == StatusPass }

// Failed reports whether the result is a failure.
func (r TestResult) Failed() bool { return r.Status == StatusFail }

// Summary aggregates test results deterministically.
type Summary struct {
	Total   int
	Passed  int
	Failed  int
	Skipped int
}

// Summarize aggregates the given results.
func Summarize(results []TestResult) Summary {
	s := Summary{Total: len(results)}
	for _, r := range results {
		switch r.Status {
		case StatusPass:
			s.Passed++
		case StatusFail:
			s.Failed++
		case StatusSkip:
			s.Skipped++
		}
	}
	return s
}

// Filter returns the tests whose ID or Name contains the (case-sensitive)
// substring pattern. An empty pattern matches everything. The result preserves
// the input ordering (deterministic).
func Filter(tests []TestCase, pattern string) []TestCase {
	if pattern == "" {
		out := make([]TestCase, len(tests))
		copy(out, tests)
		return out
	}
	out := make([]TestCase, 0, len(tests))
	for _, t := range tests {
		if strings.Contains(t.ID, pattern) || strings.Contains(t.Name, pattern) {
			out = append(out, t)
		}
	}
	return out
}

// SortByID returns a stable, deterministic ordering of tests by ID.
func SortByID(tests []TestCase) []TestCase {
	out := make([]TestCase, len(tests))
	copy(out, tests)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
