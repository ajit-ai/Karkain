package ssa

// SimplifyCFGPass — Phase 141: constant-branch folding plus unreachable-block
// deletion.
//
// A br over a constant boolean condition always takes one side: rewrite it
// to a jmp carrying the taken side's branch args, then delete every block
// unreachable from the entry block. This is what removes folded-out `if
// (false)` arms (constant conditions arise from FoldConstPass, so this pass
// runs directly after it). Both transforms are semantics-preserving by
// construction: the taken side is decided by a constant, and deleted blocks
// have no incoming edge left.
//
// The pass is loop-safe: back-edges keep their headers reachable (BFS from
// the entry follows every br/jmp target), so assisted loop bodies are never
// collected — only genuinely dead arms go.

type SimplifyCFGPass struct{}

func (SimplifyCFGPass) Name() string { return "simplify-cfg" }

func (SimplifyCFGPass) Run(fn *Function) bool {
	changed := false
	for _, b := range fn.Blocks {
		if len(b.Instrs) == 0 {
			continue
		}
		last := &b.Instrs[len(b.Instrs)-1]
		if last.Op != OpBr || len(last.Args) != 1 || !last.Args[0].IsCst {
			continue
		}
		taken := truthyConst(last.Args[0])
		// BranchArgs layout is [thenArgs, elseArgs]; default to the text
		// targets when args are absent (Verify normalizes the same way).
		target := last.ElseTarget
		var args []Operand
		if taken {
			target = last.ThenTarget
			if len(last.BranchArgs) > 0 {
				args = last.BranchArgs[0]
			}
		} else if len(last.BranchArgs) > 1 {
			args = last.BranchArgs[1]
		}
		*last = Instr{Op: OpJmp, JmpTarget: target, BranchArgs: [][]Operand{args}}
		changed = true
	}

	// Sweep unreachable blocks (entry always survives: BFS roots there).
	byName := make(map[string]*Block, len(fn.Blocks))
	for _, b := range fn.Blocks {
		byName[b.Name] = b
	}
	reachable := map[string]bool{}
	if len(fn.Blocks) > 0 {
		queue := []string{fn.Blocks[0].Name}
		reachable[fn.Blocks[0].Name] = true
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			b := byName[cur]
			if b == nil || len(b.Instrs) == 0 {
				continue
			}
			last := b.Instrs[len(b.Instrs)-1]
			for _, t := range brTargets(last) {
				if !reachable[t] {
					if _, ok := byName[t]; ok {
						reachable[t] = true
						queue = append(queue, t)
					}
				}
			}
		}
	}
	kept := fn.Blocks[:0]
	for _, b := range fn.Blocks {
		if reachable[b.Name] {
			kept = append(kept, b)
		} else {
			changed = true
		}
	}
	fn.Blocks = kept
	return changed
}

// brTargets returns the jump targets of a terminator (empty for ret).
func brTargets(in Instr) []string {
	switch in.Op {
	case OpBr:
		return []string{in.ThenTarget, in.ElseTarget}
	case OpJmp:
		return []string{in.JmpTarget}
	}
	return nil
}

// truthyConst evaluates a constant branch condition the way the backend's
// is_truthy does: zero numbers, empty strings and false are false.
func truthyConst(op Operand) bool {
	switch op.CstKind {
	case Bool:
		return op.BoolVal
	case I:
		return op.IntVal != 0
	case F:
		return op.FltVal != 0
	case Str:
		return op.StrVal != ""
	}
	return true
}
