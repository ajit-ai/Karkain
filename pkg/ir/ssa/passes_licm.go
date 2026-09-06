package ssa

// LICMPass — Phase 94: loop-invariant code motion pass.
//
// Hoists loop-invariant computations out of natural loops. A computation is
// loop-invariant if all its operands are either constants or defined outside
// the loop. Natural loops are identified by back-edges (branch target dominates
// the branch source).

type LICMPass struct{}

func (LICMPass) Name() string { return "licm" }

func (LICMPass) Run(fn *Function) bool {
	dt := BuildDominatorTree(fn)
	loops := findBackEdges(fn, dt)
	changed := false

	for _, loop := range loops {
		if hoistLoopInvariant(fn, dt, loop) {
			changed = true
		}
	}
	return changed
}

type naturalLoop struct {
	header *Block
	body   map[string]bool
}

func findBackEdges(fn *Function, dt *DomTree) []naturalLoop {
	var loops []naturalLoop
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			var targets []string
			if in.Op == OpBr {
				targets = []string{in.ThenTarget, in.ElseTarget}
			} else if in.Op == OpJmp {
				targets = []string{in.JmpTarget}
			}
			for _, target := range targets {
				if dt.Dominates(target, b.Name) {
					loop := buildLoop(fn, target, dt)
					loops = append(loops, loop)
				}
			}
		}
	}
	return loops
}

func buildLoop(fn *Function, headerName string, dt *DomTree) naturalLoop {
	body := make(map[string]bool)
	body[headerName] = true

	// Include all blocks dominated by header that are reachable from
	// any predecessor of header within the dominator tree.
	for _, b := range fn.Blocks {
		if b.Name == headerName {
			continue
		}
		if dt.Dominates(headerName, b.Name) {
			body[b.Name] = true
		}
	}

	var header *Block
	for _, b := range fn.Blocks {
		if b.Name == headerName {
			header = b
			break
		}
	}

	return naturalLoop{header: header, body: body}
}

func hoistLoopInvariant(fn *Function, dt *DomTree, loop naturalLoop) bool {
	if loop.header == nil {
		return false
	}

	// Collect all registers defined inside the loop.
	loopDefs := make(map[string]bool)
	for _, b := range fn.Blocks {
		if !loop.body[b.Name] {
			continue
		}
		for _, in := range b.Instrs {
			if in.Dest != "" {
				loopDefs[in.Dest] = true
			}
		}
	}

	changed := false
	for _, b := range fn.Blocks {
		if !loop.body[b.Name] || b.Name == loop.header.Name {
			continue
		}

		// Find hoistable instructions (pure, loop-invariant operands).
		var hoistIdxs []int
		for i, in := range b.Instrs {
			if isTerminator(in) {
				continue
			}
			if in.Dest != "" && loopDefs[in.Dest] && isLoopInvariantOp(in, loopDefs) {
				hoistIdxs = append(hoistIdxs, i)
			}
		}

		// Hoist in reverse order.
		for i := len(hoistIdxs) - 1; i >= 0; i-- {
			idx := hoistIdxs[i]
			instr := b.Instrs[idx]
			// Insert before the first terminator in the header.
			pos := len(loop.header.Instrs)
			if pos > 0 && isTerminator(loop.header.Instrs[pos-1]) {
				pos--
			}
			// Slice insertion.
			loop.header.Instrs = append(loop.header.Instrs, Instr{}) // grow
			copy(loop.header.Instrs[pos+1:], loop.header.Instrs[pos:])
			loop.header.Instrs[pos] = instr
			b.Instrs = append(b.Instrs[:idx], b.Instrs[idx+1:]...)
			changed = true
		}
	}
	return changed
}

func isLoopInvariantOp(in Instr, loopDefs map[string]bool) bool {
	for _, a := range in.Args {
		if a.IsCst {
			continue
		}
		if a.Reg != "" && loopDefs[a.Reg] {
			return false
		}
	}
	return true
}
