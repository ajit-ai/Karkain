// Package parity implements the cross-backend differential comparison harness
// established by Phase 80 (SIMD / Vector Execution Architecture). The CPU
// backend is the reference oracle — the only backend that produces real numeric
// values — and every other backend result is compared against it.
//
// Two comparison modes:
//
//  1. Numeric parity: both sides produced Values → per-element MaxAbsDiff with
//     tolerance; this is how CPU-SIMD is validated against the CPU-scalar
//     oracle.
//  2. Structural parity: the other backend produced no numeric values (e.g. the
//     GPU/NPU execute stubs) → recorded as NoNumericOutput so the gap is
//     explicit and never silently treated as equality.
package parity

import (
	"fmt"
	"math"
	"sort"

	"karkain/pkg/backend"
	"karkain/pkg/tensor"
)

// Candidate couples a backend result with its stable name for reporting.
type Candidate struct {
	Name   string
	Result *backend.Result
}

// Diff is one element-level difference between an oracle and a candidate for
// one output tensor.
type Diff struct {
	OutputID string
	Index    int
	Expected float64
	Actual   float64
	AbsDiff  float64
	Pass     bool
}

// Comparison summarizes the parity verdict for one candidate result.
type Comparison struct {
	NumericParity   bool // true when the candidate produced numeric Values
	NoNumericOutput bool // true when candidate had no Values for the outputs
	MaxAbsDiff      float64
	Worst           *Diff
	DiffCount       int
	ElementCount    int
	Tolerance       float64
	Pass            bool
}

// Report aggregates per-candidate comparisons for one execution of one graph.
type Report struct {
	CaseName    string
	Tolerance   float64
	Candidates  []*Candidate
	ByCandidate map[string]*Comparison
	NoNumeric   []string // candidate names that produced no numeric output at all
	Pass        bool
}

// Compare ranks the numeric results of every candidate against the oracle
// result (the CPU-scalar reference) for the graph's declared outputs.
func Compare(caseName string, graph *tensor.TensorGraph, oracle *backend.Result, candidates []*Candidate, tolerance float64) *Report {
	rep := &Report{
		CaseName:    caseName,
		Tolerance:   tolerance,
		Candidates:  candidates,
		ByCandidate: map[string]*Comparison{},
		Pass:        true,
	}

	outputs := graph.Outputs
	for _, cand := range candidates {
		c := &Comparison{NumericParity: true, Tolerance: tolerance, Pass: true}

		if !hasNumericOutput(cand.Result, outputs) {
			c.NumericParity = false
			c.NoNumericOutput = true
			c.Pass = false
			rep.NoNumeric = append(rep.NoNumeric, cand.Name)
			rep.ByCandidate[cand.Name] = c
			continue
		}

		var worst float64
		diffs := 0
		elems := 0
		for _, n := range outputs {
			ref := oracle.Values[n.ID]
			act := cand.Result.Values[n.ID]
			elems += len(ref)
			if len(ref) != len(act) {
				c.Pass = false
			}
			m := len(ref)
			if len(act) < m {
				m = len(act)
			}
			for i := 0; i < m; i++ {
				d := math.Abs(ref[i] - act[i])
				if math.IsNaN(d) {
					c.Pass = false
					continue
				}
				if d > worst {
					worst = d
					c.Worst = &Diff{
						OutputID: n.ID, Index: i,
						Expected: ref[i], Actual: act[i], AbsDiff: d,
						Pass: d <= tolerance,
					}
				}
				if d > tolerance {
					diffs++
					c.Pass = false
				}
			}
		}
		c.ElementCount = elems
		c.DiffCount = diffs
		c.MaxAbsDiff = worst
		if !c.Pass {
			rep.Pass = false
		}
		rep.ByCandidate[cand.Name] = c
	}
	sort.Strings(rep.NoNumeric)
	return rep
}

// Reflects the numeric-result contract: a backend result is numeric only if it
// actually produced Values for at least one declared output tensor.
func hasNumericOutput(r *backend.Result, outputs []*tensor.TensorNode) bool {
	if r == nil {
		return false
	}
	for _, n := range outputs {
		if vals, ok := r.Values[n.ID]; ok && len(vals) > 0 {
			return true
		}
	}
	return false
}

// ReportString renders a Report as a deterministic human-readable block.
func ReportString(rep *Report) string {
	s := fmt.Sprintf("=== Parity %s (tolerance %g) ===\n", rep.CaseName, rep.Tolerance)
	for _, cand := range rep.Candidates {
		c := rep.ByCandidate[cand.Name]
		state := "PASS"
		if c.NoNumericOutput {
			state = "NO-NUMERIC-OUTPUT"
		} else if !c.Pass {
			state = "MISMATCH"
		}
		extra := ""
		if c.Worst != nil {
			extra = fmt.Sprintf("  worst[%s:%d] exp=%g act=%g diff=%g",
				c.Worst.OutputID, c.Worst.Index, c.Worst.Expected, c.Worst.Actual, c.Worst.AbsDiff)
		}
		s += fmt.Sprintf("  [%s] %-12s maxdiff=%.3e elems=%d%s\n",
			state, cand.Name, c.MaxAbsDiff, c.ElementCount, extra)
	}
	if len(rep.NoNumeric) > 0 {
		s += fmt.Sprintf("  no-numeric-output backend(s): %v (numeric parity requires hardware execution)\n", rep.NoNumeric)
	}
	s += fmt.Sprintf("  verdict: %v\n", rep.Pass)
	return s
}
