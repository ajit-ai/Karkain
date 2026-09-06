package ssa

// DCESimplePass — Phase 94: dead code elimination pass.
//
// Removes instructions whose results are never used.
// Side-effecting ops (call, print, index_set, rawc, terminators) are kept.
// Iterates to fixpoint to handle dead-use chains.

type DCESimplePass struct{}

func (DCESimplePass) Name() string { return "dce" }

func (DCESimplePass) Run(fn *Function) bool {
	changed := false
	for {
		used := collectAllUses(fn)
		iterChanged := false
		for _, b := range fn.Blocks {
			keep := b.Instrs[:0]
			for _, in := range b.Instrs {
				if isAlive(in, used) {
					keep = append(keep, in)
				} else {
					iterChanged = true
				}
			}
			if len(keep) < len(b.Instrs) {
				b.Instrs = keep
			}
		}
		if !iterChanged {
			break
		}
		changed = true
	}
	return changed
}

func collectAllUses(fn *Function) map[string]bool {
	used := make(map[string]bool)
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			for _, a := range in.Args {
				if !a.IsCst && a.Reg != "" {
					used[a.Reg] = true
				}
			}
			for _, g := range in.BranchArgs {
				for _, a := range g {
					if !a.IsCst && a.Reg != "" {
						used[a.Reg] = true
					}
				}
			}
		}
	}
	return used
}

func isAlive(in Instr, used map[string]bool) bool {
	if isTerminator(in) {
		return true
	}
	if in.Dest == "" {
		return true
	}
	if used[in.Dest] {
		return true
	}
	switch in.Op {
	case OpCall, OpCallVoid, OpIndexSet, OpPrint, OpRawC:
		return true
	}
	return false
}
