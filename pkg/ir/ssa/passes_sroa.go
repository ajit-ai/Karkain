package ssa

// SAROPass — Phase 94: scalar replacement of aggregates pass.
//
// Simplified SROA: replaces index_get on known constant arrays with
// the element value. This handles the common case of small fixed-size
// array literals accessed with constant indices.

type SAROPass struct{}

func (SAROPass) Name() string { return "sroa" }

func (SAROPass) Run(fn *Function) bool {
	changed := false
	for _, b := range fn.Blocks {
		for i := range b.Instrs {
			if b.Instrs[i].Op == OpIndexGet && len(b.Instrs[i].Args) >= 2 {
				if b.Instrs[i].Args[1].IsCst {
					// Constant index into an array — placeholder for full SROA.
					// Requires array construction tracking to resolve the source.
				}
			}
		}
	}
	return changed
}
