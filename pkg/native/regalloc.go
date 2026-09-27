package native

import "karkain/pkg/parser"

// Phase 150C — register allocation.
//
// Locals normally live in frame slots: every read is a load and every
// write is a store through a constant rsp-relative displacement. That is
// simple and correct, and it is what every pre-150C image does. This pass
// promotes int locals into callee-saved registers so the common case (a
// counter or accumulator touched many times inside a loop) stops paying
// memory traffic per access.
//
// # The allocatable set
//
// allocRegs is R12-R15 and nothing else, for two independent reasons:
//
//   - They are callee-saved under BOTH ABIs in play (System V and Win64),
//     so a value parked there survives a call to a user function, a
//     helper, or a kernel32 export without a spill.
//   - They are untouched by the rest of the backend. The expression
//     evaluator works in RAX/RCX/RDX/RSI/RDI/R8-R11 and the emitted
//     helpers (print_int, print_float, alloc, the map/record/array
//     lowerings, the PEB bootstrap) clobber RBX. R12-R15 appear nowhere
//     in emit.go or program.go outside their own constant declarations,
//     so nothing can silently steal an allocated value.
//
// RBX and RBP are deliberately NOT allocatable: both are live scratch
// inside emitted helpers, so handing one to a local would be a
// cross-helper clobber.

// allocRegs is the allocatable set, in preference order.
var allocRegs = []Reg{R12, R13, R14, R15}

// allocRegCount is len(allocRegs). It is a named constant because a slice
// length is not a constant expression, and the engagement gate below is
// written in terms of it.
const allocRegCount = 4

// raMinLive is the number of simultaneously-live int locals at which
// register allocation earns its keep. At or below allocRegCount every
// live value fits in a register anyway, so there is no pressure to
// relieve and the existing slot path is already spill-free — the only
// thing a push/pop pair would buy is a marginally different encoding.
const raMinLive = allocRegCount + 1

// regRange is one candidate's linearised live range, in "points" of the
// emission-order walk. Ranges are never split: a local either owns its
// register for the whole range or keeps its frame slot. Splitting (and
// the reload points it needs) is future work, and a non-splitting
// allocator is exactly as correct for the programs in the v1 surface.
type regRange struct {
	name       string
	start, end int
}

// hasControlFlow reports whether the function contains a loop or a
// branch. It is one half of the engagement gate (see planRegs).
func (b *Builder) hasControlFlow(stmts []parser.Node) bool {
	found := false
	var walk func([]parser.Node)
	walk = func(list []parser.Node) {
		for _, s := range list {
			if found {
				return
			}
			switch n := s.(type) {
			case *parser.IfStmt:
				found = true
			case *parser.WhileStmt:
				found = true
			case *parser.ForStmt:
				found = true
			case *parser.ForInStmt:
				walk(n.Body)
			case *parser.BlockStmt:
				walk(n.Statements)
			}
		}
	}
	walk(stmts)
	return found
}

// planRegs decides which int locals get a register.
//
// The gate is deliberate and is what keeps every pre-150C image
// byte-identical:
//
//   - Register allocation engages only under real pressure, i.e. more
//     simultaneously-live int locals than there are allocatable
//     registers. Below that there is nothing to relieve.
//   - AND only when the function contains control flow. In a
//     straight-line body every value is typically read once, so a
//     push/pop pair around the function would cost more than the single
//     load it removes. The Phase-148 seven-parameter adder is the
//     motivating example: seven live values, but one expression.
//
// The result is a name -> register map plus the register list in the
// exact order the prologue must push and the epilogue must pop.
func (b *Builder) planRegs(fd *parser.FuncDecl) {
	b.regOf = map[string]Reg{}
	b.regsUsed = nil
	if !b.hasControlFlow(fd.Body) {
		return
	}
	ranges, peak := b.liveRanges(fd.Body)
	if peak < raMinLive {
		return
	}
	// Linear scan in order of first definition. A register is available
	// when the local currently holding it is dead before this range
	// starts; otherwise the candidate spills to its frame slot, which
	// is the pre-150C behaviour and needs no new code path.
	heldBy := map[Reg]*regRange{}
	assigned := map[string]Reg{}
	for i := range ranges {
		r := ranges[i]
		for reg, other := range heldBy {
			if other.end < r.start {
				delete(heldBy, reg)
			}
		}
		for _, reg := range allocRegs {
			if _, busy := heldBy[reg]; busy {
				continue
			}
			heldBy[reg] = r
			assigned[r.name] = reg
			break
		}
	}
	// Publish regsUsed in allocRegs order so the push/pop sequence is a
	// pure function of the allocation, never of map iteration order.
	b.regOf = assigned
	used := make([]Reg, 0, len(allocRegs))
	for _, reg := range allocRegs {
		for _, got := range assigned {
			if got == reg {
				used = append(used, reg)
				break
			}
		}
	}
	b.regsUsed = used
}

// regOfFor reports the register holding name, if it was allocated.
func (b *Builder) regOfFor(name string) (Reg, bool) {
	r, ok := b.regOf[name]
	return r, ok
}

// liveRanges walks the body in emission order and returns one range per
// int local plus the peak number of simultaneously-live locals.
//
// Loops need explicit handling: a value used inside a loop body is live
// for the WHOLE loop, because the body may execute zero times before a
// read and many times after a write. Ranges are therefore widened to
// span the loop that uses them, which is conservative in the safe
// direction — it can only cause an extra spill, never a wrong answer.
func (b *Builder) liveRanges(stmts []parser.Node) ([]*regRange, int) {
	// NOTE: ranges holds POINTERS. An earlier version appended values,
	// which silently snapshotted every range at creation time and left
	// the liveness numbers at zero -- so the allocator never engaged.
	// The gate (TestPhase150C_AllocationEngages) is what caught it.
	var ranges []*regRange
	idx := map[string]*regRange{}
	pos := 0

	// touch records a use (or a def) of name at the current point.
	touch := func(name string, def bool) {
		if b.kinds[name] != KindInt {
			return
		}
		r, ok := idx[name]
		if !ok {
			r = &regRange{name: name, start: pos, end: pos}
			idx[name] = r
			ranges = append(ranges, r)
		}
		if def && pos < r.start {
			r.start = pos
		}
		if pos > r.end {
			r.end = pos
		}
	}
	// hold forces every named int local to stay live across [from, to].
	hold := func(names map[string]bool, from, to int) {
		for name := range names {
			r, ok := idx[name]
			if !ok {
				continue
			}
			if from < r.start {
				r.start = from
			}
			if to > r.end {
				r.end = to
			}
		}
	}

	var walkExpr func(parser.Node)
	var walkStmts func([]parser.Node)
	walkExpr = func(n parser.Node) {
		pos++
		switch x := n.(type) {
		case *parser.Identifier:
			touch(x.Name, false)
		case *parser.BinaryExpr:
			// `x = v` defines x rather than using it.
			if x.Operator == "=" {
				if id, ok := x.Left.(*parser.Identifier); ok {
					touch(id.Name, true)
					walkExpr(x.Right)
					return
				}
			}
			walkExpr(x.Left)
			walkExpr(x.Right)
		case *parser.UnaryExpr:
			walkExpr(x.Operand)
		case *parser.IndexExpr:
			walkExpr(x.Left)
			walkExpr(x.Index)
		case *parser.SliceExpr:
			walkExpr(x.Target)
			walkExpr(x.Start)
			walkExpr(x.End)
		case *parser.DotExpr:
			walkExpr(x.Left)
		case *parser.CallExpr:
			for _, a := range x.Args {
				walkExpr(a)
			}
		case *parser.ArrayLiteral:
			for _, e := range x.Elements {
				walkExpr(e)
			}
		case *parser.MapLiteral:
			for _, k := range x.Keys {
				walkExpr(k)
			}
			for _, v := range x.Values {
				walkExpr(v)
			}
		case *parser.StructLiteral:
			for _, f := range x.Fields {
				walkExpr(f)
			}
		}
	}
	walkStmts = func(list []parser.Node) {
		for _, s := range list {
			switch n := s.(type) {
			case *parser.VarDeclStmt:
				pos++
				touch(n.Name, true)
				walkExpr(n.Value)
			case *parser.ExprStmt:
				walkExpr(n.Expression)
			case *parser.PrintStmt:
				walkExpr(n.Value)
			case *parser.ReturnStmt:
				walkExpr(n.Value)
			case *parser.IfStmt:
				walkExpr(n.Condition)
				walkStmts(n.Consequence)
				walkStmts(n.Alternative)
			case *parser.WhileStmt:
				from := pos
				names := b.usedNames([]parser.Node{n})
				walkExpr(n.Condition)
				walkStmts(n.Body)
				hold(names, from, pos)
			case *parser.ForStmt:
				if n.Init != nil {
					walkStmts([]parser.Node{n.Init})
				}
				from := pos
				names := b.usedNames([]parser.Node{n})
				if n.Condition != nil {
					walkExpr(n.Condition)
				}
				walkStmts(n.Body)
				if n.Post != nil {
					walkStmts([]parser.Node{n.Post})
				}
				hold(names, from, pos)
			case *parser.ForInStmt:
				// The iterator variable is rewritten every iteration and
				// its hidden index slot is keyed by the statement node, so
				// it stays in the frame.
				walkExpr(n.Iter)
				walkStmts(n.Body)
			case *parser.BlockStmt:
				walkStmts(n.Statements)
			}
		}
	}
	walkStmts(stmts)

	// Peak simultaneous liveness over the point-ordered ranges.
	type ev struct {
		at   int
		step int
	}
	events := make([]ev, 0, len(ranges)*2)
	for _, r := range ranges {
		events = append(events, ev{r.start, 1}, ev{r.end + 1, -1})
	}
	for i := 1; i < len(events); i++ {
		for j := i; j > 0 && (events[j].at < events[j-1].at ||
			(events[j].at == events[j-1].at && events[j].step < events[j-1].step)); j-- {
			events[j], events[j-1] = events[j-1], events[j]
		}
	}
	peak, live := 0, 0
	for _, e := range events {
		live += e.step
		if live > peak {
			peak = live
		}
	}

	// Keep only well-formed ranges and order them by start (then name, so
	// the order is total and the allocation stays deterministic).
	out := make([]*regRange, 0, len(ranges))
	for _, r := range ranges {
		if r.end < r.start {
			continue
		}
		out = append(out, r)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && (out[j].start < out[j-1].start ||
			(out[j].start == out[j-1].start && out[j].name < out[j-1].name)); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, peak
}

// usedNames collects every local name a statement list mentions. A loop
// uses it to widen the live ranges of the values it touches, so the
// widening is driven by the same traversal the emitter performs.
func (b *Builder) usedNames(list []parser.Node) map[string]bool {
	out := map[string]bool{}
	var ex func(parser.Node)
	ex = func(n parser.Node) {
		switch x := n.(type) {
		case *parser.Identifier:
			out[x.Name] = true
		case *parser.BinaryExpr:
			ex(x.Left)
			ex(x.Right)
		case *parser.UnaryExpr:
			ex(x.Operand)
		case *parser.IndexExpr:
			ex(x.Left)
			ex(x.Index)
		case *parser.SliceExpr:
			ex(x.Target)
			ex(x.Start)
			ex(x.End)
		case *parser.DotExpr:
			ex(x.Left)
		case *parser.CallExpr:
			for _, a := range x.Args {
				ex(a)
			}
		case *parser.ArrayLiteral:
			for _, e := range x.Elements {
				ex(e)
			}
		case *parser.MapLiteral:
			for _, k := range x.Keys {
				ex(k)
			}
			for _, v := range x.Values {
				ex(v)
			}
		case *parser.StructLiteral:
			for _, f := range x.Fields {
				ex(f)
			}
		}
	}
	var st func([]parser.Node)
	st = func(list []parser.Node) {
		for _, s := range list {
			switch n := s.(type) {
			case *parser.VarDeclStmt:
				out[n.Name] = true
				ex(n.Value)
			case *parser.ExprStmt:
				ex(n.Expression)
			case *parser.PrintStmt:
				ex(n.Value)
			case *parser.ReturnStmt:
				ex(n.Value)
			case *parser.IfStmt:
				ex(n.Condition)
				st(n.Consequence)
				st(n.Alternative)
			case *parser.WhileStmt:
				ex(n.Condition)
				st(n.Body)
			case *parser.ForStmt:
				if n.Init != nil {
					st([]parser.Node{n.Init})
				}
				ex(n.Condition)
				st(n.Body)
				if n.Post != nil {
					st([]parser.Node{n.Post})
				}
			case *parser.ForInStmt:
				ex(n.Iter)
				st(n.Body)
			case *parser.BlockStmt:
				st(n.Statements)
			}
		}
	}
	st(list)
	return out
}
