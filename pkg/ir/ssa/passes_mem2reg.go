package ssa

// Mem2RegPass — Phase 94: memory to register promotion pass.
//
// Promotes OpLoad/OpStore pairs for simple variables to SSA registers.
// A variable is promotable if it is stored exactly once (in any block)
// and that store dominates all loads of the same cell.

type Mem2RegPass struct{}

func (Mem2RegPass) Name() string { return "mem2reg" }

func (Mem2RegPass) Run(fn *Function) bool {
	dt := BuildDominatorTree(fn)
	cells := collectCellOps(fn)
	changed := false

	for _, info := range cells {
		if len(info.stores) != 1 || len(info.loads) == 0 {
			continue
		}
		storeBlockName := info.stores[0].block.Name
		storeVal := info.stores[0].val

		// Check that the store block dominates all load blocks.
		allDominated := true
		for _, ld := range info.loads {
			if !dt.Dominates(storeBlockName, ld.block.Name) {
				allDominated = false
				break
			}
		}
		if !allDominated {
			continue
		}

		// Replace all loads with the stored value (reverse order to keep indices).
		for i := len(info.loads) - 1; i >= 0; i-- {
			ld := info.loads[i]
			replaceLoadAt(ld.block, ld.instrIdx, ld.destReg, storeVal)
			changed = true
		}

		// Remove the store.
		removeInstr(info.stores[0].block, info.stores[0].instrIdx)
		changed = true
	}
	return changed
}

type cellStore struct {
	block    *Block
	instrIdx int
	val      Operand
}

type cellLoad struct {
	block    *Block
	instrIdx int
	destReg  string
}

type cellOps struct {
	stores []cellStore
	loads  []cellLoad
}

func collectCellOps(fn *Function) map[string]*cellOps {
	result := make(map[string]*cellOps)
	for _, b := range fn.Blocks {
		for i, in := range b.Instrs {
			if in.Op == OpStore {
				ci := result[in.Cell]
				if ci == nil {
					ci = &cellOps{}
					result[in.Cell] = ci
				}
				val := Operand{}
				if len(in.Args) > 0 {
					val = in.Args[0]
				}
				ci.stores = append(ci.stores, cellStore{block: b, instrIdx: i, val: val})
			}
			if in.Op == OpLoad {
				ci := result[in.Cell]
				if ci == nil {
					ci = &cellOps{}
					result[in.Cell] = ci
				}
				ci.loads = append(ci.loads, cellLoad{block: b, instrIdx: i, destReg: in.Dest})
			}
		}
	}
	return result
}

func replaceLoadAt(b *Block, idx int, destReg string, val Operand) {
	// Rewrite all uses of destReg in subsequent instructions to use val.
	for i := idx + 1; i < len(b.Instrs); i++ {
		in := &b.Instrs[i]
		for j := range in.Args {
			if !in.Args[j].IsCst && in.Args[j].Reg == destReg {
				in.Args[j] = val
			}
		}
		for g := range in.BranchArgs {
			for j := range in.BranchArgs[g] {
				if !in.BranchArgs[g][j].IsCst && in.BranchArgs[g][j].Reg == destReg {
					in.BranchArgs[g][j] = val
				}
			}
		}
	}
	// Remove the load instruction.
	b.Instrs = append(b.Instrs[:idx], b.Instrs[idx+1:]...)
}

func removeInstr(b *Block, idx int) {
	b.Instrs = append(b.Instrs[:idx], b.Instrs[idx+1:]...)
}
