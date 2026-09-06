package ssa

// Liveness analysis for SSA IR.
// Computes which registers are live at each program point.
// Used for register allocation and dead code elimination.

// LiveRange — the range of instructions where a register is live.
type LiveRange struct {
	Reg     string
	DefSite InstrRef // where the register is defined
	UseSite InstrRef // where the register is used (last use)
	Start   int      // instruction index where liveness starts
	End     int      // instruction index where liveness ends
	Block   string   // block where the range is defined
}

// InstrRef — reference to a specific instruction in the SSA.
type InstrRef struct {
	Block string // block name
	Index int    // instruction index within the block
}

// LivenessInfo — liveness information for an entire function.
type LivenessInfo struct {
	// LiveIn maps each block name to the set of registers live at the entry.
	LiveIn map[string]map[string]bool

	// LiveOut maps each block name to the set of registers live at the exit.
	LiveOut map[string]map[string]bool

	// LiveGen maps each block name to the set of registers defined (generated) in the block.
	LiveGen map[string]map[string]bool

	// LiveKill maps each block name to the set of registers used (killed) in the block.
	LiveKill map[string]map[string]bool

	// Ranges maps each register name to its live range.
	Ranges map[string]*LiveRange

	// TotalRegs is the total number of distinct registers in the function.
	TotalRegs int
}

// ComputeLiveness performs liveness analysis on a function.
// Returns liveness information for all blocks.
func ComputeLiveness(fn *Function) *LivenessInfo {
	li := &LivenessInfo{
		LiveIn:   make(map[string]map[string]bool),
		LiveOut:  make(map[string]map[string]bool),
		LiveGen:  make(map[string]map[string]bool),
		LiveKill: make(map[string]map[string]bool),
		Ranges:   make(map[string]*LiveRange),
	}

	// Initialize all block sets
	for _, b := range fn.Blocks {
		li.LiveIn[b.Name] = make(map[string]bool)
		li.LiveOut[b.Name] = make(map[string]bool)
		li.LiveGen[b.Name] = make(map[string]bool)
		li.LiveKill[b.Name] = make(map[string]bool)
	}

	// Compute gen and kill sets for each block
	for _, b := range fn.Blocks {
		gen := make(map[string]bool)
		kill := make(map[string]bool)

		for _, in := range b.Instrs {
			// Uses (read before written)
			for _, arg := range in.Args {
				if !arg.IsCst && arg.Reg != "" {
					gen[arg.Reg] = true
				}
			}

			// Branch args (uses in branch targets)
			for _, branchArg := range in.BranchArgs {
				for _, arg := range branchArg {
					if !arg.IsCst && arg.Reg != "" {
						gen[arg.Reg] = true
					}
				}
			}

			// Definition (written) — kills the register
			if in.Dest != "" {
				kill[in.Dest] = true
				delete(gen, in.Dest) // kill overrides gen
			}
		}

		li.LiveGen[b.Name] = gen
		li.LiveKill[b.Name] = kill
	}

	// Iterative dataflow analysis (backward)
	changed := true
	for changed {
		changed = false
		for _, b := range fn.Blocks {
			// LiveOut(B) = union of LiveIn(S) for all successors S of B
			liveOut := make(map[string]bool)
			for _, in := range b.Instrs {
				if in.Op == OpBr {
					for reg := range li.LiveIn[in.ThenTarget] {
						liveOut[reg] = true
					}
					for reg := range li.LiveIn[in.ElseTarget] {
						liveOut[reg] = true
					}
				} else if in.Op == OpJmp {
					for reg := range li.LiveIn[in.JmpTarget] {
						liveOut[reg] = true
					}
				}
			}

			// LiveIn(B) = Gen(B) union (LiveOut(B) - Kill(B))
			liveIn := make(map[string]bool)
			for reg := range li.LiveGen[b.Name] {
				liveIn[reg] = true
			}
			for reg := range liveOut {
				if !li.LiveKill[b.Name][reg] {
					liveIn[reg] = true
				}
			}

			// Check if changed
			if len(liveIn) != len(li.LiveIn[b.Name]) || len(liveOut) != len(li.LiveOut[b.Name]) {
				changed = true
			} else {
				for reg := range liveIn {
					if !li.LiveIn[b.Name][reg] {
						changed = true
						break
					}
				}
			}
			for reg := range liveOut {
				if !li.LiveOut[b.Name][reg] {
					changed = true
					break
				}
			}

			li.LiveIn[b.Name] = liveIn
			li.LiveOut[b.Name] = liveOut
		}
	}

	// Compute live ranges and total register count
	allRegs := make(map[string]bool)
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			if in.Dest != "" {
				allRegs[in.Dest] = true
			}
			for _, arg := range in.Args {
				if !arg.IsCst && arg.Reg != "" {
					allRegs[arg.Reg] = true
				}
			}
		}
	}

	li.TotalRegs = len(allRegs)

	// Build live ranges
	for reg := range allRegs {
		defBlock := ""
		defIdx := 0
		useBlock := ""
		useIdx := 0

		for _, b := range fn.Blocks {
			for i, in := range b.Instrs {
				if in.Dest == reg {
					defBlock = b.Name
					defIdx = i
				}
				for _, arg := range in.Args {
					if !arg.IsCst && arg.Reg == reg {
						useBlock = b.Name
						useIdx = i
					}
				}
			}
		}

		if defBlock != "" {
			li.Ranges[reg] = &LiveRange{
				Reg:     reg,
				DefSite: InstrRef{Block: defBlock, Index: defIdx},
				UseSite: InstrRef{Block: useBlock, Index: useIdx},
				Block:   defBlock,
			}
		}
	}

	return li
}

// NumLiveAt returns the number of registers live at a specific instruction.
func (li *LivenessInfo) NumLiveAt(blockName string, instrIndex int) int {
	count := 0
	for range li.LiveIn[blockName] {
		count++
	}
	// Simplified: just count LiveIn registers
	return count
}

// Interferences returns the set of registers that interfere with the given register.
// Two registers interfere if their live ranges overlap.
func (li *LivenessInfo) Interferences(reg string) map[string]bool {
	intf := make(map[string]bool)

	myRange, ok := li.Ranges[reg]
	if !ok {
		return intf
	}

	for otherReg, otherRange := range li.Ranges {
		if otherReg == reg {
			continue
		}
		// Simple overlap check: if ranges are in the same block and overlap
		if myRange.Block == otherRange.Block {
			if myRange.Start <= otherRange.End && otherRange.Start <= myRange.End {
				intf[otherReg] = true
			}
		}
		// Cross-block interference: if one is live at the entry of the other's block
		if li.LiveIn[myRange.Block][otherReg] {
			intf[otherReg] = true
		}
	}

	return intf
}

// IsDead returns true if a register is defined but never used.
func (li *LivenessInfo) IsDead(reg string) bool {
	// Check if the register appears in any use position
	for _, b := range li.LiveGen {
		if b[reg] {
			return false
		}
	}
	// If it's only in LiveKill but never in LiveGen of successors, it's dead
	return true
}
