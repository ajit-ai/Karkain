package ssa

// CSEPass — Phase 94: common subexpression elimination pass.
//
// Within a single basic block, detects instructions that compute the same
// value from the same operands and replaces later uses with the earlier result.
// Only applies to pure operations (const, binop, unop).

type CSEPass struct{}

func (CSEPass) Name() string { return "cse" }

func (CSEPass) Run(fn *Function) bool {
	changed := false
	for _, b := range fn.Blocks {
		seen := make(map[string]string)
		var kept []Instr
		for _, in := range b.Instrs {
			if !isPureOp(in.Op) {
				kept = append(kept, in)
				continue
			}
			key := instrKey(in)
			if dest, ok := seen[key]; ok {
				// Replace this instruction: rewrite all subsequent uses of in.Dest → dest.
				// We already passed those instructions, so rewrite future ones in kept.
				rewriteRegInList(kept, in.Dest, dest)
				changed = true
				// Don't add this instruction — it's eliminated.
			} else {
				if in.Dest != "" {
					seen[key] = in.Dest
				}
				kept = append(kept, in)
			}
		}
		if len(kept) < len(b.Instrs) {
			b.Instrs = kept
		}
	}
	return changed
}

func isPureOp(op OpKind) bool {
	switch op {
	case OpConst, OpBinOp, OpUnOp:
		return true
	}
	return false
}

func instrKey(in Instr) string {
	parts := []string{string(rune(in.Op)), in.OpStr}
	for _, a := range in.Args {
		if a.IsCst {
			parts = append(parts, "c"+a.String())
		} else {
			parts = append(parts, "%"+a.Reg)
		}
	}
	total := 0
	for _, p := range parts {
		total += len(p) + 1
	}
	buf := make([]byte, 0, total)
	for i, p := range parts {
		if i > 0 {
			buf = append(buf, 0)
		}
		buf = append(buf, p...)
	}
	return string(buf)
}

// rewriteRegInList rewrites uses of old→new in a list of instructions.
func rewriteRegInList(instrs []Instr, old, new string) {
	for i := range instrs {
		in := &instrs[i]
		for j := range in.Args {
			if !in.Args[j].IsCst && in.Args[j].Reg == old {
				in.Args[j].Reg = new
			}
		}
		for g := range in.BranchArgs {
			for j := range in.BranchArgs[g] {
				if !in.BranchArgs[g][j].IsCst && in.BranchArgs[g][j].Reg == old {
					in.BranchArgs[g][j].Reg = new
				}
			}
		}
	}
}
