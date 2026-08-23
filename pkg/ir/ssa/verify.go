package ssa

import "fmt"

// Verify — Phase 53: validate module well-formedness.
// Checks:
//  1. every function has an entry block that is first in Blocks
//  2. every block ends in exactly one terminator (br/jmp/ret)
//  3. no instructions after the terminator
//  4. branch targets exist and receive the right number of block args
//  5. register single-assignment (SSA): each Dest written once per function
//  6. binop operators are known; rawc must have payload
func (m *Module) Verify() error {
	for _, fn := range m.Functions {
		if err := verifyFunction(fn); err != nil {
			return fmt.Errorf("function @%s: %w", fn.Name, err)
		}
	}
	return nil
}

func verifyFunction(fn *Function) error {
	if len(fn.Blocks) == 0 || fn.Blocks[0] != fn.Entry {
		return fmt.Errorf("entry block must be first")
	}

	blocks := make(map[string]*Block, len(fn.Blocks))
	for _, b := range fn.Blocks {
		if _, dup := blocks[b.Name]; dup {
			return fmt.Errorf("duplicate block name %q", b.Name)
		}
		blocks[b.Name] = b
	}

	written := make(map[string]bool)
	paramCount := func(b *Block) int { return len(b.Params) }

	for _, b := range fn.Blocks {
		for i, in := range b.Instrs {
			isLast := i == len(b.Instrs)-1
			switch in.Op {
			case OpBr, OpJmp, OpRet:
				if !isLast {
					return fmt.Errorf("block %s: terminator not last", b.Name)
				}
			default:
				if isLast {
					return fmt.Errorf("block %s: missing terminator", b.Name)
				}
			}

			if in.Dest != "" {
				if written[in.Dest] {
					return fmt.Errorf("block %s: register %%%s assigned twice (SSA violation)", b.Name, in.Dest)
				}
				written[in.Dest] = true
			}

			switch in.Op {
			case OpBinOp:
				if !validBinOps[in.OpStr] {
					return fmt.Errorf("block %s: unknown binop %q", b.Name, in.OpStr)
				}
				if len(in.Args) != 2 {
					return fmt.Errorf("block %s: binop needs 2 args", b.Name)
				}
			case OpBr:
				if len(in.Args) != 1 {
					return fmt.Errorf("block %s: br needs condition", b.Name)
				}
				tb, ok := blocks[in.ThenTarget]
				if !ok {
					return fmt.Errorf("block %s: unknown target ^%s", b.Name, in.ThenTarget)
				}
				eb, ok := blocks[in.ElseTarget]
				if !ok {
					return fmt.Errorf("block %s: unknown target ^%s", b.Name, in.ElseTarget)
				}
				if len(in.BranchArgs) == 0 {
					in.BranchArgs = [][]Operand{{}, {}}
				}
				if len(in.BranchArgs[0]) != paramCount(tb) || len(in.BranchArgs[1]) != paramCount(eb) {
					return fmt.Errorf("block %s: br arg count mismatch", b.Name)
				}
			case OpJmp:
				tb, ok := blocks[in.JmpTarget]
				if !ok {
					return fmt.Errorf("block %s: unknown target ^%s", b.Name, in.JmpTarget)
				}
				if len(in.BranchArgs) > 0 && len(in.BranchArgs[0]) != paramCount(tb) {
					return fmt.Errorf("block %s: jmp arg count mismatch", b.Name)
				}
			case OpRawC:
				if in.RawC == "" {
					return fmt.Errorf("block %s: empty rawc payload", b.Name)
				}
			case OpCall, OpCallVoid:
				if in.OpStr == "" {
					return fmt.Errorf("block %s: call without target", b.Name)
				}
			}
		}
	}
	return nil
}

