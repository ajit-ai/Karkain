package testing

import (
	"testing"
	"time"
)

func TestSummarizeCountsEachStatus(t *testing.T) {
	results := []TestResult{
		{Status: StatusPass},
		{Status: StatusFail},
		{Status: StatusSkip},
		{Status: StatusPass},
	}
	s := Summarize(results)
	if s.Total != 4 || s.Passed != 2 || s.Failed != 1 || s.Skipped != 1 {
		t.Fatalf("unexpected summary: %+v", s)
	}
}

func TestResultStatusHelpers(t *testing.T) {
	if r := (TestResult{Status: StatusPass}); !r.Passed() || r.Failed() {
		t.Errorf("pass status helpers wrong")
	}
	if r := (TestResult{Status: StatusFail}); r.Passed() || !r.Failed() {
		t.Errorf("fail status helpers wrong")
	}
}

func TestFilterPreservesOrderAndIsStable(t *testing.T) {
	base := []TestCase{
		{ID: "a:k:z", Name: "z"},
		{ID: "a:k:a", Name: "a"},
		{ID: "a:k:m", Name: "m"},
	}
	got := Filter(base, "a")
	if len(got) != 3 {
		t.Fatalf("all match 'a', got %d", len(got))
	}
	if got[0].Name != "z" || got[2].Name != "m" {
		t.Errorf("order not preserved: %+v", got)
	}
}

func TestSortByIDDeterministic(t *testing.T) {
	base := []TestCase{{ID: "b"}, {ID: "a"}, {ID: "c"}}
	sorted := SortByID(base)
	for i, want := range []string{"a", "b", "c"} {
		if sorted[i].ID != want {
			t.Errorf("sorted[%d] = %s, want %s", i, sorted[i].ID, want)
		}
	}
	// Original unchanged.
	if base[0].ID != "b" {
		t.Errorf("SortByID mutated input")
	}
}

func TestParseAssertionFailure(t *testing.T) {
	stderr := "assertion failed: assert_eq: assert_eq(actual, expected)\n" +
		"  expected: 5\n" +
		"  actual:   4\n"
	f := ParseAssertionFailure(stderr)
	if f == nil {
		t.Fatal("expected failure parsed")
	}
	if f.Expected != "5" || f.Actual != "4" {
		t.Errorf("expected= %q actual= %q", f.Expected, f.Actual)
	}
	if !contains(f.Message, "assert_eq") {
		t.Errorf("message missing assertion kind: %q", f.Message)
	}
}

func TestParseAssertionFailure_NoMatch(t *testing.T) {
	if f := ParseAssertionFailure(""); f != nil {
		t.Errorf("empty stderr should be nil, got %+v", f)
	}
	if f := ParseAssertionFailure("gcc: error: nothing to do"); f != nil {
		t.Errorf("non-assertion stderr should be nil, got %+v", f)
	}
}

func TestParseAssertionFailure_Location(t *testing.T) {
	stderr := "assertion failed: assert(condition)  [prog.kark:4]\n"
	f := ParseAssertionFailure(stderr)
	if f == nil {
		t.Fatal("expected failure parsed")
	}
	if f.Location != "[prog.kark:4]" {
		t.Errorf("location = %q", f.Location)
	}
}

func TestDurationRecording(t *testing.T) {
	r := TestResult{Status: StatusPass, Duration: 1500 * time.Millisecond}
	if r.Duration != 1500*time.Millisecond {
		t.Errorf("duration not preserved")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
