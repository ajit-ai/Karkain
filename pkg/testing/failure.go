package testing

import (
	"strings"
)

// parseAssertionFailure extracts structured failure information from a test's
// captured stderr. It understands the diagnostic format emitted by the KTF-001
// assertion runtime:
//
//	assertion failed: assert_eq: assert_eq(actual, expected)  [loc]
//	  expected: 5
//	  actual:   4
//
// When stderr does not match the well-known prefix an empty Failure is
// returned (the caller may fall back to the raw message via the ExitError).
func ParseAssertionFailure(stderr string) *Failure {
	if stderr == "" {
		return nil
	}
	lines := strings.Split(strings.TrimRight(stderr, "\r\n"), "\n")
	// Trim a single trailing CR on each line (Windows).
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}

	head := ""
	var expected, actual string
	haveExpected := false
	haveActual := false
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if i == 0 {
			// head like "assertion failed: assert_eq: expr  [loc]"
			head = t
			// extract optional [loc]
			if idx := strings.Index(t, "  ["); idx >= 0 {
				_ = idx
			}
			continue
		}
		if strings.HasPrefix(t, "expected:") {
			expected = strings.TrimSpace(strings.TrimPrefix(t, "expected:"))
			haveExpected = true
			continue
		}
		if strings.HasPrefix(t, "actual:") {
			actual = strings.TrimSpace(strings.TrimPrefix(t, "actual:"))
			haveActual = true
			continue
		}
		if strings.HasPrefix(t, "unexpectedly equal") {
			actual = strings.TrimSpace(strings.TrimPrefix(t, "unexpectedly equal (actual):"))
			haveActual = true
			continue
		}
	}

	if !strings.Contains(head, "assert") {
		return nil
	}

	f := &Failure{Message: head}
	if haveExpected {
		f.Expected = expected
	}
	if haveActual {
		f.Actual = actual
	}
	f.Location = extractLocation(head)
	return f
}

// extractLocation pulls the bracketed source reference out of a diagnostic
// head line, e.g. "assertion failed: ... [file.kark:42]".
func extractLocation(head string) string {
	start := strings.Index(head, "[")
	if start < 0 {
		return ""
	}
	end := strings.Index(head[start:], "]")
	if end < 0 {
		return ""
	}
	return head[start : start+end+1]
}
