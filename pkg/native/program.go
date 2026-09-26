package native

import (
	"fmt"
	"math"
	"strconv"

	"karkain/pkg/parser"
)

// Straight-line program lowering, Phase 145.
//
// Supported surface (gate-pinned, everything else is a deterministic
// K145 error): top-level `func` declarations (≤6 params; int params and
// int returns only), `let` bindings of ints and strings, int/string
// literals, `+ - *` on ints, unary minus, calls, `print(expr)` of an int
// or string, and `return` (main's value becomes the process exit code;
// a missing trailing return yields 0). Slot kinds are tracked so a
// string never flows into int code: that is a K145 error, never a
// miscompile. Types are otherwise trusted from the source — callers
// validate first; the gate fixtures are fixed programs.
//
// Values live in stack slots ([rsp+off], 8 bytes each): params, then one
// slot per let (two for strings: ptr+len), then the binary-operand scratch
// stack, then the caller-frame extras area, then a fixed 96-byte argument
// spill area (6 args x 16 bytes). Eval scratch is rax/rcx/rdx (+ the
// stack) ONLY, so argument spill slots and not-yet-loaded argument
// registers are never disturbed by nested evaluation. Helpers take fixed
// registers and may clobber anything.
//
// Native calling convention (Phase 148): argument 8-byte units pack in
// order — ints take one unit, strings two (ptr+len). Units 0-5 travel in
// RDI,RSI,RDX,RCX,R8,R9; further units travel in the caller-frame extras
// array whose address reaches the callee in R10 (set with lea just before
// the call; the callee homes stack units at entry, before R10 can die).
// String returns leave (RAX=ptr, RDX=len). rsp never moves during argument
// evaluation: register units stage through the per-arg spill, extras
// through the extras area, and only then do registers reload.

// OS names accepted by CompileProgramForOS (Phase 149). Emission is
// pure Go on every host; only the container, the syscall numbers (or
// kernel32 boundary on Windows) and the _start tail vary per OS.
const (
	OSLinux   = "linux"
	OSWindows = "windows"
	OSMacOS   = "macos"
)

// sysWrite returns the write syscall number for the target OS
// (Linux/macOS raw-syscall path; Windows uses kernel32 instead).
func (b *Builder) sysWrite() uint32 {
	if b.goos == OSMacOS {
		return 0x2000004
	}
	return 1
}

// sysExit returns the exit syscall number for the target OS.
func (b *Builder) sysExit() uint32 {
	if b.goos == OSMacOS {
		return 0x2000001
	}
	return 60
}

// argRegs is the user-function calling convention (System V order).
var argRegs = []Reg{RDI, RSI, RDX, RCX, R8, R9}

// argSpillBytes reserves caller-side spill space for 6 args x (ptr+len).
const argSpillBytes = 96

// maxBinDepth bounds nested binary-expression evaluation. Binary operands
// stage through depth-indexed scratch slots (never push: pushing moves rsp
// and silently shifts every frame-relative slot address — the Phase-147
// `add(20,22) = 40` defect, where the right operand re-read slot a).
// Nesting deeper than this is a loud K145 error, never a silent miscompile.
const maxBinDepth = 64

// addrPatch records an imm64 placeholder resolving to a .rodata address.
type addrPatch struct {
	pos   int
	roOff uint64
}

// iatPatch records a movabs placeholder resolving to a PE IAT slot
// address (Phase 149): kernel32 calls go through the import table.
type iatPatch struct {
	pos   int
	index int
}

// absPatch records a mov-[imm64] placeholder resolving to a PE IAT slot
// address (Phase 149): the loader-independent bootstrap publishes the
// PEB-resolved kernel32 addresses straight into the slots.
type absPatch struct {
	pos   int
	index int
}

// Value kinds (Phase 150A): the native Value model. KindInt is zero so
// every pre-150 default (untyped parameters, missing returns, unknown
// slots) keeps meaning int without touching its logic. Strings and arrays
// occupy two frame slots (ptr+len); ints and floats occupy one slot
// (floats carry f64 bits, converted to XMM only inside arithmetic).
const (
	KindInt = iota
	KindString
	KindFloat
	KindArray
)

// kindUnits returns the 8-byte frame/call slots a value kind occupies.
func kindUnits(k int) int {
	if k == KindString || k == KindArray {
		return 2
	}
	return 1
}

// kindName renders a kind for K145 diagnostics.
func kindName(k int) string {
	switch k {
	case KindString:
		return "string"
	case KindFloat:
		return "float"
	case KindArray:
		return "array"
	default:
		return "int"
	}
}

// Builder lowers one program to .text+.rodata.
type Builder struct {
	e       *Emitter
	goos    string // Phase 149: OSLinux, OSWindows or OSMacOS
	rodata  []byte
	strs    map[string]uint64
	patches []addrPatch
	ipatches []iatPatch // Phase 149: PE import-slot placeholders
	apatches []absPatch // Phase 149: bootstrap IAT publishes
	funcs   map[string]bool
	ftab    map[string]*parser.FuncDecl // Phase 148: name -> declaration (arity, param kinds)
	retKind map[string]int              // Phase 150A: name -> value kind (was bool string-ness)
	slots   map[string]int
	kinds   map[string]int // Phase 150A: name -> value kind (was bool string-ness)
	frame   int
	binTemp int // Phase 147: base offset of the binary-operand scratch stack
	// Phase 150B: base offset of the string-concat staging area. Separate
	// from binTemp because a concat needs five units per nesting level
	// (left ptr/len, right ptr/len, block) where an int binary needs one.
	strTemp   int
	extrasBase int // Phase 148: base offset of the caller-frame extras area
	scanErr error
	uid     int // Phase 148: monotonically increasing label discriminator
	loops   []loopTgt
	// Phase 150A: per-statement hidden for-in index slots, keyed by the
	// ForInStmt node itself. A single shared "for$idx" slot would make a
	// nested for-in resume its parent with the inner loop's counter, so
	// each loop owns its own 8 bytes. Keying by node keeps the layout
	// pass (scanLets) and the emission pass (emitForIn) in agreement
	// without depending on traversal order or source line numbers.
	forIdx map[*parser.ForInStmt]int
	// Phase 150A: whether the program can reach a float value at all.
	// Decided by a whole-unit pre-pass (scanFloatUsage) because
	// emitHelpers runs before any function body is emitted, so a flag
	// set during emission would be too late. Gating print_float on it
	// keeps every int/string-only image byte-identical to increment 149
	// instead of carrying ~700 bytes of unused formatter.
	usesFloat bool
	// Phase 150B: whether the program concatenates strings anywhere, and the
	// arena size that provably suffices. Both come from one pre-pass
	// (scanValueUsage) so the allocator, the frame layout and the emission
	// all agree without re-deriving anything.
	usesConcat bool
	// usesStrEq: the program compares two strings for equality, which also
	// needs the string scratch area. Kept apart from usesConcat because only
	// concatenation needs the heap — comparison reads bytes in place.
	usesStrEq bool
	// usesStrSlice: the program slices a string, which also needs the string
	// staging area (and, like comparison, no heap).
	usesStrSlice bool
	// usesPush: the program calls push(), which needs the heap arena (and,
	// unlike the string operations, the string staging area for its copy).
	usesPush bool
	// pushInLoop: a push() reached from inside a loop body. Its runtime count
	// is not bounded at compile time, so the arena bound cannot cover it and
	// it is refused with a loud K145 instead.
	pushInLoop bool
	// Phase 150B: the bump arena. Layout is a 16-byte header followed by
	// heapSize usable bytes: [cursor][limit][data...]. cursor and limit
	// are ABSOLUTE addresses held in the arena itself, so the allocator
	// needs no globals and no writable image text; they are initialised
	// lazily on the first alloc (a zero cursor means "not yet started").
	// `heap` is nil for any program that never allocates, which is what
	// keeps every pre-150B image byte-identical.
	heap     []byte
	heapSize int
	// hpatches resolve to absolute addresses inside the arena, exactly as
	// patches do for .rodata. PE needs them in the DIR64 fixup list (the
	// host loader rebases), so buildReloc joins this list.
	hpatches []addrPatch
}

// Arena header layout (Phase 150B). Offsets are into the arena image.
const (
	heapCursorOff = 0 // absolute bump cursor
	heapLimitOff  = 8 // absolute end of the arena
	heapHeaderLen = 16
	// heapMinSize floors the computed arena bound so trivial programs get a
	// usable arena without the bound arithmetic having to be exact.
	heapMinSize = 1024
)

// scanValueUsage is the Phase-150A/150B whole-unit pre-pass. It answers the
// two questions that must be settled before any code is emitted:
//
//   - may a float value exist? (gates print_float)
//   - does the program concatenate strings, and how large must the heap
//     arena be so no allocation can ever exhaust it? (gates alloc, the
//     string-concat frame area, and the writable data segment)
//
// Both have to be pre-passes because emitHelpers runs before any body, and
// because the frame layout needs the answer before it can assign offsets.
//
// The float criterion is "a float literal or a float annotation appears
// anywhere", a superset of "some print can actually print a float": a float
// value can only originate at one of those two roots, because exprKind
// derives float from a literal, from a float-typed variable/parameter, or
// from a call whose inferred return kind is float — and that inference
// bottoms out in the same two roots.
//
// The arena bound is a true upper bound on every runtime allocation, which is
// what makes the Int3-on-exhaustion path unreachable for any program that
// compiles. A site whose operands are both string literals needs exactly
// len(l)+len(r) bytes. Any other site is bounded by the program's total
// literal byte count: every string value in such a program is either a
// literal or a concatenation of literals, so no value can be longer than
// all the literal bytes put together.
func scanValueUsage(prog *parser.Program) (usesFloat, usesConcat, usesStrEq, usesStrSlice, usesPush, pushInLoop bool, heapSize int) {
	var concatSites, concatHeap int
	concatSites, concatHeap = scanConcatSites(prog)
	usesConcat = concatSites > 0
	heapSize = concatHeap
	usesStrEq = false
	strNames := collectStringNames(prog)
	usesStrEq, usesStrSlice = scanStringViewOps(prog, strNames)
	pushSites, maxArrLit, pushInLoop := scanPushSites(prog)
	usesPush = pushSites > 0
	if usesPush {
		// Arena bound for pushes: site i of a chain sees at most
		// (largest literal length + i) elements, so the sum over all sites is
		// bounded by pushSites * (maxArrLit + pushSites) 8-byte elements. A
		// push inside a loop is refused outright, so no site can execute more
		// than once and the bound holds.
		heapSize = pushSites * (maxArrLit + pushSites) * 8
		if heapSize < heapMinSize {
			heapSize = heapMinSize
		}
	}

	// A separate, float-only walk: unlike the concat search it has no
	// type inference to do, and keeping the two concerns apart means a
	// bug in string classification can never suppress float detection.
	var walkExpr func(n parser.Node)
	walkExpr = func(n parser.Node) {
		if n == nil {
			return
		}
		switch x := n.(type) {
		case *parser.Float64Literal:
			usesFloat = true
			return
		case *parser.VarDeclStmt:
			if x.Type == "float" {
				usesFloat = true
				return
			}
			walkExpr(x.Value)
		case *parser.CallExpr:
			for _, a := range x.Args {
				walkExpr(a)
			}
		case *parser.BinaryExpr:
			walkExpr(x.Left)
			walkExpr(x.Right)
		case *parser.UnaryExpr:
			walkExpr(x.Operand)
		case *parser.IndexExpr:
			walkExpr(x.Left)
			walkExpr(x.Index)
		case *parser.ArrayLiteral:
			for _, e := range x.Elements {
				walkExpr(e)
			}
		}
	}
	var walkStmts func(stmts []parser.Node)
	walkStmts = func(stmts []parser.Node) {
		for _, s := range stmts {
			switch x := s.(type) {
			case *parser.FuncDecl:
				for _, pt := range x.ParamTypes {
					if pt == "float" {
						usesFloat = true
					}
				}
				walkStmts(x.Body)
			case *parser.VarDeclStmt:
				walkExpr(x)
			case *parser.PrintStmt:
				walkExpr(x.Value)
			case *parser.ExprStmt:
				walkExpr(x.Expression)
			case *parser.ReturnStmt:
				walkExpr(x.Value)
			case *parser.IfStmt:
				walkExpr(x.Condition)
				walkStmts(x.Consequence)
				walkStmts(x.Alternative)
			case *parser.WhileStmt:
				walkExpr(x.Condition)
				walkStmts(x.Body)
			case *parser.ForStmt:
				walkExpr(x.Condition)
				walkExpr(x.Init)
				walkExpr(x.Post)
				walkStmts(x.Body)
			case *parser.ForInStmt:
				walkExpr(x.Iter)
				walkStmts(x.Body)
			case *parser.BlockStmt:
				walkStmts(x.Statements)
			}
		}
	}
	walkStmts(prog.Statements)
	return usesFloat, usesConcat, usesStrEq, usesStrSlice, usesPush, pushInLoop, heapSize
}

// scanPushSites counts push() call sites, records the largest array-literal
// length in the program, and flags any push reached from inside a loop.
//
// The loop flag is the honest bound: a push inside a `while`/`for`/`for-in`
// body can execute an unbounded number of times, so no compile-time arena
// size can cover it. Rather than let a program run out of arena and trap, the
// caller turns the flag into a precise K145 (checkPushLoop).
func scanPushSites(prog *parser.Program) (sites, maxLit int, inLoop bool) {
	isPush := func(n parser.Node) bool {
		c, ok := n.(*parser.CallExpr)
		return ok && c.Function == "push"
	}
	var walkExpr func(n parser.Node, depth int)
	walkExpr = func(n parser.Node, depth int) {
		if n == nil {
			return
		}
		if isPush(n) {
			sites++
			if depth > 0 {
				inLoop = true
			}
		}
		if lit, ok := n.(*parser.ArrayLiteral); ok {
			if len(lit.Elements) > maxLit {
				maxLit = len(lit.Elements)
			}
			for _, e := range lit.Elements {
				walkExpr(e, depth)
			}
			return
		}
		switch x := n.(type) {
		case *parser.BinaryExpr:
			walkExpr(x.Left, depth)
			walkExpr(x.Right, depth)
		case *parser.UnaryExpr:
			walkExpr(x.Operand, depth)
		case *parser.CallExpr:
			for _, a := range x.Args {
				walkExpr(a, depth)
			}
		case *parser.IndexExpr:
			walkExpr(x.Left, depth)
			walkExpr(x.Index, depth)
		case *parser.SliceExpr:
			walkExpr(x.Target, depth)
			walkExpr(x.Start, depth)
			walkExpr(x.End, depth)
		}
	}
	var walkStmts func(stmts []parser.Node, depth int)
	walkStmts = func(stmts []parser.Node, depth int) {
		for _, s := range stmts {
			switch x := s.(type) {
			case *parser.FuncDecl:
				walkStmts(x.Body, depth)
			case *parser.VarDeclStmt:
				walkExpr(x.Value, depth)
			case *parser.PrintStmt:
				walkExpr(x.Value, depth)
			case *parser.ExprStmt:
				walkExpr(x.Expression, depth)
			case *parser.ReturnStmt:
				walkExpr(x.Value, depth)
			case *parser.IfStmt:
				walkExpr(x.Condition, depth)
				// A conditional body is not a loop: a push there runs at most
				// once per entry, which the site count already covers.
				walkStmts(x.Consequence, depth)
				walkStmts(x.Alternative, depth)
			case *parser.WhileStmt:
				walkExpr(x.Condition, depth)
				walkStmts(x.Body, depth+1)
			case *parser.ForStmt:
				walkExpr(x.Condition, depth)
				walkExpr(x.Init, depth)
				walkExpr(x.Post, depth)
				walkStmts(x.Body, depth+1)
			case *parser.ForInStmt:
				walkExpr(x.Iter, depth)
				walkStmts(x.Body, depth+1)
			case *parser.BlockStmt:
				walkStmts(x.Statements, depth)
			}
		}
	}
	walkStmts(prog.Statements, 0)
	return sites, maxLit, inLoop
}

// scanConcatSites finds every string-concatenation site and returns how many
// there are plus a provably sufficient arena size.
//
// A site is a `+` whose two operands are both strings, so the pre-pass needs
// a (syntactic) string classifier. It is deliberately conservative in the
// *safe* direction: anything it cannot prove is a string is treated as not a
// string, so it never invents a concat site that emission would not make —
// which would cost a needless writable segment. A missed site is the
// dangerous direction, so emitStrConcat also refuses loudly if it is ever
// reached with no arena (see its heapSize guard).
func scanConcatSites(prog *parser.Program) (sites, heapSize int) {
	strNames := map[string]bool{}
	// strKind reports whether n is a string-typed expression, using the
	// names gathered so far.
	var strKind func(n parser.Node) bool
	strKind = func(n parser.Node) bool {
		switch x := n.(type) {
		case *parser.StringLiteral:
			return true
		case *parser.Identifier:
			return strNames[x.Name]
		case *parser.BinaryExpr:
			return x.Operator == "+" && strKind(x.Left) && strKind(x.Right)
		case *parser.CallExpr:
			// A call is treated as a string when any string is passed to
			// it: in v1 the only such shape is a string-returning helper,
			// and misclassifying the other way round would be the loud,
			// detected case (emitStrConcat's own kind check).
			for _, a := range x.Args {
				if strKind(a) {
					return true
				}
			}
			return false
		}
		return false
	}
	// Seed from `x string` parameters, then learn bindings, repeating until
	// stable so a binding used by a later binding resolves regardless of
	// ordering.
	seeded := false
	totalLit := 0
	var countExpr func(n parser.Node)
	countExpr = func(n parser.Node) {
		switch x := n.(type) {
		case *parser.StringLiteral:
			totalLit += len(x.Value)
		case *parser.BinaryExpr:
			if x.Operator == "+" && strKind(x.Left) && strKind(x.Right) {
				sites++
			}
			countExpr(x.Left)
			countExpr(x.Right)
		case *parser.UnaryExpr:
			countExpr(x.Operand)
		case *parser.CallExpr:
			for _, a := range x.Args {
				countExpr(a)
			}
		case *parser.IndexExpr:
			countExpr(x.Left)
			countExpr(x.Index)
		case *parser.ArrayLiteral:
			for _, e := range x.Elements {
				countExpr(e)
			}
		case *parser.VarDeclStmt:
			if strKind(x.Value) {
				strNames[x.Name] = true
			}
			countExpr(x.Value)
		}
	}
	var walkStmts func(stmts []parser.Node)
	walkStmts = func(stmts []parser.Node) {
		for _, s := range stmts {
			switch x := s.(type) {
			case *parser.FuncDecl:
				for i, pt := range x.ParamTypes {
					if pt == "string" && i < len(x.Params) {
						strNames[x.Params[i]] = true
					}
				}
				walkStmts(x.Body)
			case *parser.VarDeclStmt:
				countExpr(x)
			case *parser.PrintStmt:
				countExpr(x.Value)
			case *parser.ExprStmt:
				countExpr(x.Expression)
			case *parser.ReturnStmt:
				countExpr(x.Value)
			case *parser.IfStmt:
				countExpr(x.Condition)
				walkStmts(x.Consequence)
				walkStmts(x.Alternative)
			case *parser.WhileStmt:
				countExpr(x.Condition)
				walkStmts(x.Body)
			case *parser.ForStmt:
				countExpr(x.Condition)
				countExpr(x.Init)
				countExpr(x.Post)
				walkStmts(x.Body)
			case *parser.ForInStmt:
				countExpr(x.Iter)
				walkStmts(x.Body)
			case *parser.BlockStmt:
				walkStmts(x.Statements)
			}
		}
	}
	for pass := 0; pass < 8; pass++ {
		before := len(strNames)
		walkStmts(prog.Statements)
		if seeded && len(strNames) == before {
			break
		}
		seeded = true
	}
	if sites > 0 {
		// Every string value in a concatenating program is a literal or a
		// concatenation of literals, so no value can be longer than the
		// program's total literal bytes. sites * totalLit therefore bounds
		// the sum of all runtime allocations, which is what makes the
		// allocator's exhaustion trap unreachable for a program that
		// compiles.
		heapSize = sites * totalLit
		if heapSize < heapMinSize {
			heapSize = heapMinSize
		}
	}
	return sites, heapSize
}
// collectStringNames returns every name bound to a string value, so the
// pre-passes can classify a `+` or a `==` by operand type. It seeds from
// `x string` parameters, then learns bindings, repeating until the set stops
// growing so ordering between bindings never affects the answer.
func collectStringNames(prog *parser.Program) map[string]bool {
	strNames := map[string]bool{}
	var strKind func(n parser.Node) bool
	strKind = func(n parser.Node) bool {
		switch x := n.(type) {
		case *parser.StringLiteral:
			return true
		case *parser.Identifier:
			return strNames[x.Name]
		case *parser.BinaryExpr:
			return x.Operator == "+" && strKind(x.Left) && strKind(x.Right)
		case *parser.CallExpr:
			// A call counts as a string when any string is passed to it: in
			// v1 the only such shape is a string-returning helper. The
			// dangerous direction (missing a site) is covered by the loud
			// guards in emitStrConcat and emitStrEqCond.
			for _, a := range x.Args {
				if strKind(a) {
					return true
				}
			}
			return false
		}
		return false
	}
	// Only *statements* matter here: a `let` binding is a statement, so no
	// expression position can introduce a new string name and the walk does
	// not need to descend into expressions at all.
	var walkStmts func(stmts []parser.Node)
	walkStmts = func(stmts []parser.Node) {
		for _, s := range stmts {
			switch x := s.(type) {
			case *parser.FuncDecl:
				for i, pt := range x.ParamTypes {
					if pt == "string" && i < len(x.Params) {
						strNames[x.Params[i]] = true
					}
				}
				walkStmts(x.Body)
			case *parser.VarDeclStmt:
				if strKind(x.Value) {
					strNames[x.Name] = true
				}
			case *parser.IfStmt:
				walkStmts(x.Consequence)
				walkStmts(x.Alternative)
			case *parser.WhileStmt:
				walkStmts(x.Body)
			case *parser.ForStmt:
				walkStmts(x.Body)
			case *parser.ForInStmt:
				walkStmts(x.Body)
			case *parser.BlockStmt:
				walkStmts(x.Statements)
			}
		}
	}
	for pass := 0; pass < 8; pass++ {
		before := len(strNames)
		walkStmts(prog.Statements)
		if len(strNames) == before {
			break
		}
	}
	return strNames
}

// scanStringViewOps reports which string operations that only *read* bytes
// (and so need the string staging area but no heap) the program uses: string
// equality/inequality, and string slicing.
func scanStringViewOps(prog *parser.Program, strNames map[string]bool) (eq, slice bool) {
	var strKind func(n parser.Node) bool
	strKind = func(n parser.Node) bool {
		switch x := n.(type) {
		case *parser.StringLiteral:
			return true
		case *parser.Identifier:
			return strNames[x.Name]
		case *parser.BinaryExpr:
			return x.Operator == "+" && strKind(x.Left) && strKind(x.Right)
		case *parser.CallExpr:
			for _, a := range x.Args {
				if strKind(a) {
					return true
				}
			}
			return false
		}
		return false
	}
	var walkExpr func(n parser.Node)
	walkExpr = func(n parser.Node) {
		if n == nil || (eq && slice) {
			return
		}
		switch x := n.(type) {
		case *parser.SliceExpr:
			if strKind(x.Target) {
				slice = true
			}
		case *parser.BinaryExpr:
			if (x.Operator == "==" || x.Operator == "!=") && strKind(x.Left) && strKind(x.Right) {
				eq = true
			}
		}
	}
	var walkStmts func(stmts []parser.Node)
	walkStmts = func(stmts []parser.Node) {
		for _, s := range stmts {
			switch x := s.(type) {
			case *parser.VarDeclStmt:
				walkExpr(x.Value)
			case *parser.PrintStmt:
				walkExpr(x.Value)
			case *parser.ExprStmt:
				walkExpr(x.Expression)
			case *parser.ReturnStmt:
				walkExpr(x.Value)
			case *parser.IfStmt:
				walkExpr(x.Condition)
				walkStmts(x.Consequence)
				walkStmts(x.Alternative)
			case *parser.WhileStmt:
				walkExpr(x.Condition)
				walkStmts(x.Body)
			case *parser.ForStmt:
				walkExpr(x.Condition)
				walkExpr(x.Init)
				walkExpr(x.Post)
				walkStmts(x.Body)
			case *parser.ForInStmt:
				walkExpr(x.Iter)
				walkStmts(x.Body)
			case *parser.BlockStmt:
				walkStmts(x.Statements)
			}
		}
	}
	walkStmts(prog.Statements)
	return eq, slice
}

// loopTgt records a loop's jump targets for break/continue (Phase 148).
type loopTgt struct {
	brk  string // break jumps here (past the loop)
	cont string // continue jumps here (condition check / post statement)
}

// fresh returns a program-unique label (Emitter.Mark panics on duplicates,
// so control-flow labels can never be a bare fixed string).
func (b *Builder) fresh(prefix string) string {
	b.uid++
	return fmt.Sprintf("%s$%d", prefix, b.uid)
}

// CompileProgram lowers prog to a linked static Linux executable image
// (the Phase-145 default; CompileProgramForOS selects the OS).
func CompileProgram(prog *parser.Program) ([]byte, error) {
	return CompileProgramForOS(prog, OSLinux)
}

// CompileProgramForOS lowers prog to a linked static executable image
// for the named OS ("linux", "windows", "macos"): one encoder and one
// lowering feed per-OS containers, syscall numbers (or the kernel32
// boundary on Windows) and entry tails. Anything else is a K145 error.
func CompileProgramForOS(prog *parser.Program, osName string) ([]byte, error) {
	b := newBuilder()
	b.goos = osName
	switch osName {
	case OSLinux, OSWindows, OSMacOS:
	default:
		return nil, fmt.Errorf("error[K145]: unsupported native OS '%s' (want linux, windows or macos)", osName)
	}
	for _, stmt := range prog.Statements {
		if fd, ok := stmt.(*parser.FuncDecl); ok {
			if b.funcs[fd.Name] {
				return nil, fmt.Errorf("error[K145]: duplicate function '%s'", fd.Name)
			}
			b.funcs[fd.Name] = true
			b.ftab[fd.Name] = fd
		}
	}
	if !b.funcs["main"] {
		return nil, fmt.Errorf("error[K145]: no 'main' function for native target")
	}
	// Phase 148: string-return inference for the whole unit before any
	// emission (callers need callee kinds; see retKindOf).
	for name := range b.ftab {
		if _, err := b.retKindOf(name); err != nil {
			return nil, err
		}
	}
	b.usesFloat, b.usesConcat, b.usesStrEq, b.usesStrSlice, b.usesPush, b.pushInLoop, b.heapSize = scanValueUsage(prog)
	b.emitHelpers()
	for _, stmt := range prog.Statements {
		if fd, ok := stmt.(*parser.FuncDecl); ok {
			if err := b.emitFunc(fd); err != nil {
				return nil, err
			}
		}
	}
	b.e.Mark("_start")
	if b.goos == OSWindows {
		// Alignment first: everything downstream (main's frame math,
		// every kernel32 call) assumes rsp%16==0 here. Then the
		// loader-independent bootstrap fills the IAT before main runs.
		b.e.AndRspNeg16()
		b.emitWinBootstrap()
	}
	b.e.Call("karkain_main")
	if b.goos == OSWindows {
		b.emitWindowsExit()
	} else {
		b.e.MovRegReg(RDI, RAX)
		b.e.MovRegImm32(RAX, b.sysExit())
		b.e.Syscall()
	}
	text := b.e.Bytes()
	textOff := elfHeaderSize + progHeaderSize
	base := uint64(BaseAddr)
	switch b.goos {
	case OSMacOS:
		textOff = machoTextOff
		base = MachoBase
	case OSWindows:
		textOff = peTextOff
		base = PEBaseAddr
	}
	// Phase 150B: the arena. ELF and PE carry it in a writable segment
	// (a second R+W PT_LOAD; and the already-writable .idata on PE).
	// Mach-O has a single R+X __TEXT and no writable segment, and
	// increment 150D owns that container — so a program that allocates
	// gets a loud, specific refusal here instead of an image whose first
	// bump-cursor store would fault.
	arenaLen := heapHeaderLen + b.heapSize
	var data []byte
	if b.heapSize > 0 {
		if b.goos == OSMacOS {
			return nil, fmt.Errorf("error[K145]: heap allocation is not supported on the macos native container yet (a writable __DATA segment lands with 150D)")
		}
		// The arena image: a zeroed header (the cursor self-initialises on
		// first alloc) followed by heapSize usable bytes. PE's linker reads
		// it straight off the Builder; ELF gets it as the data segment.
		b.heap = make([]byte, arenaLen)
		data = b.heap
	}
	// ELF's header count depends on whether a data segment follows, so the
	// .text offset is only known now.
	if b.goos == OSLinux && len(data) > 0 {
		textOff = elfHeaderSize + progHeaderSize*2
		base = uint64(BaseAddr)
	}
	entry := textOff + b.e.labels["_start"]
	// The arena's absolute address, needed to resolve hpatches. For ELF it
	// is the page-aligned file offset the linker will place it at; for PE it
	// sits immediately after the import structures inside .idata.
	heapBase := uint64(0)
	switch b.goos {
	case OSWindows:
		heapBase = uint64(PEBaseAddr+peIdataRVA) + uint64(len(buildIdata()))
	default:
		heapBase = uint64(BaseAddr) + uint64(peAlignUp(textOff+len(text)+len(b.rodata), 0x1000))
	}
	// Windows resolves all three patch kinds inside LinkPE (rodata against
	// the .text base, IAT slots and arena addresses against .idata);
	// ELF resolves rodata and arena here.
	if b.goos == OSWindows {
		return LinkPE(b, text, b.rodata, textOff, entry)
	}
	roBase := base + uint64(textOff) + uint64(len(text))
	out := append([]byte{}, text...)
	resolve := func(p addrPatch, bse uint64) {
		v := bse + p.roOff
		out[p.pos] = byte(v)
		out[p.pos+1] = byte(v >> 8)
		out[p.pos+2] = byte(v >> 16)
		out[p.pos+3] = byte(v >> 24)
		out[p.pos+4] = byte(v >> 32)
		out[p.pos+5] = byte(v >> 40)
		out[p.pos+6] = byte(v >> 48)
		out[p.pos+7] = byte(v >> 56)
	}
	for _, p := range b.patches {
		resolve(p, roBase)
	}
	for _, p := range b.hpatches {
		resolve(p, heapBase)
	}
	switch b.goos {
	case OSMacOS:
		return LinkMachO(out, b.rodata, textOff, entry)
	default:
		return Link(out, b.rodata, data, textOff, entry)
	}
}
func newBuilder() *Builder {
	return &Builder{e: NewEmitter(), strs: map[string]uint64{}, funcs: map[string]bool{}, ftab: map[string]*parser.FuncDecl{}, retKind: map[string]int{}, forIdx: map[*parser.ForInStmt]int{}}
}

// paramKind reports the value kind of parameter i of fd (Phase-46
// annotations `name string` / `name float`; untyped parameters are ints.
// Array parameters have no annotation syntax in 150A: a function whose
// body indexes a parameter is rejected loudly at layout (150B work).
func paramKind(fd *parser.FuncDecl, i int) int {
	if i < len(fd.ParamTypes) {
		switch fd.ParamTypes[i] {
		case "string":
			return KindString
		case "float":
			return KindFloat
		}
	}
	return KindInt
}

// retKindOf infers the value kind a function returns: every `return` in
// its body (descending into control flow) must agree; no returns means
// int (the missing-return-yields-0 rule). Cyclic call graphs that never
// ground out are a loud K145.
func (b *Builder) retKindOf(name string) (int, error) {
	if k, done := b.retKind[name]; done {
		return k, nil
	}
	return b.retKindVisit(name, map[string]bool{})
}

func (b *Builder) retKindVisit(name string, visiting map[string]bool) (int, error) {
	if k, done := b.retKind[name]; done {
		return k, nil
	}
	if visiting[name] {
		return KindInt, fmt.Errorf("error[K145]: cannot determine return kind of recursive function '%s'", name)
	}
	fd, ok := b.ftab[name]
	if !ok {
		return KindInt, fmt.Errorf("error[K145]: undefined function '%s'", name)
	}
	visiting[name] = true
	// Local varDecl map for resolving returned identifiers, seeded with
	// the parameter kinds (untyped parameters are ints).
	vars := map[string]parser.Node{}
	pstr := map[string]int{}
	for i, p := range fd.Params {
		pstr[p] = paramKind(fd, i)
	}
	var collectVars func(stmts []parser.Node)
	collectVars = func(stmts []parser.Node) {
		for _, s := range stmts {
			switch n := s.(type) {
			case *parser.VarDeclStmt:
				vars[n.Name] = n.Value
			case *parser.IfStmt:
				collectVars(n.Consequence)
				collectVars(n.Alternative)
			case *parser.WhileStmt:
				collectVars(n.Body)
			case *parser.ForStmt:
				collectVars(n.Body)
			case *parser.ForInStmt:
				collectVars(n.Body)
			case *parser.BlockStmt:
				collectVars(n.Statements)
			}
		}
	}
	collectVars(fd.Body)
	seen := false
	kind := KindInt
	var walkReturns func(stmts []parser.Node) error
	walkReturns = func(stmts []parser.Node) error {
		for _, s := range stmts {
			switch n := s.(type) {
			case *parser.ReturnStmt:
				if n.Value == nil {
					continue
				}
				k, err := b.retKindOfExpr(n.Value, vars, pstr, visiting)
				if err != nil {
					return err
				}
				if !seen {
					seen, kind = true, k
				} else if kind != k {
					return fmt.Errorf("error[K145]: function '%s' mixes %s and %s returns", name, kindName(kind), kindName(k))
				}
			case *parser.IfStmt:
				if err := walkReturns(n.Consequence); err != nil {
					return err
				}
				if err := walkReturns(n.Alternative); err != nil {
					return err
				}
			case *parser.WhileStmt:
				if err := walkReturns(n.Body); err != nil {
					return err
				}
			case *parser.ForStmt:
				if err := walkReturns(n.Body); err != nil {
					return err
				}
			case *parser.ForInStmt:
				if err := walkReturns(n.Body); err != nil {
					return err
				}
			case *parser.BlockStmt:
				if err := walkReturns(n.Statements); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walkReturns(fd.Body); err != nil {
		return KindInt, err
	}
	delete(visiting, name)
	b.retKind[name] = kind
	return kind, nil
}

// retKindOfExpr classifies a returned value expression.
func (b *Builder) retKindOfExpr(n parser.Node, vars map[string]parser.Node, pstr map[string]int, visiting map[string]bool) (int, error) {
	switch x := n.(type) {
	case *parser.StringLiteral:
		return KindString, nil
	case *parser.IntLiteral:
		return KindInt, nil
	case *parser.Float64Literal:
		return KindFloat, nil
	case *parser.Identifier:
		if k, isParam := pstr[x.Name]; isParam {
			return k, nil
		}
		v, ok := vars[x.Name]
		if !ok {
			return KindInt, fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		if v == nil {
			return KindInt, nil
		}
		return b.retKindOfExpr(v, vars, pstr, visiting)
	case *parser.BinaryExpr:
		// Phase 150A: arithmetic inherits float when either operand is
		// float, exactly like exprKind. Returning KindInt here would let a
		// float-valued expression be inferred as an int return while the
		// body leaves f64 bits in RAX — a silent type confusion, so the
		// classification must agree with emission.
		switch x.Operator {
		case "+", "-", "*", "/", "%":
			lk, err := b.retKindOfExpr(x.Left, vars, pstr, visiting)
			if err != nil {
				return KindInt, err
			}
			rk, err := b.retKindOfExpr(x.Right, vars, pstr, visiting)
			if err != nil {
				return KindInt, err
			}
			if lk == KindFloat || rk == KindFloat {
				return KindFloat, nil
			}
			return KindInt, nil
		default:
			// A comparison yields an int on both engines.
			return KindInt, nil
		}
	case *parser.UnaryExpr:
		return b.retKindOfExpr(x.Operand, vars, pstr, visiting)
	case *parser.CallExpr:
		return b.retKindVisit(x.Function, visiting)
	default:
		return KindInt, fmt.Errorf("error[K145]: unsupported return expression %T", n)
	}
}

func (b *Builder) internRodata(s string) uint64 {
	if off, ok := b.strs[s]; ok {
		return off
	}
	off := uint64(len(b.rodata))
	b.rodata = append(b.rodata, s...)
	b.strs[s] = off
	return off
}

func (b *Builder) rodataRef(r Reg, s string) {
	b.patches = append(b.patches, addrPatch{pos: b.e.imm64Patch(r), roOff: b.internRodata(s)})
}

// emitWrite emits the payload write for (RDI=fd, RSI=ptr, RDX=len):
// a raw syscall on Linux/macOS (numbers via sysWrite), the kernel32
// WriteFile sequence on Windows (Phase 149).
func (b *Builder) emitWrite() {
	if b.goos == OSWindows {
		b.emitWinWrite()
		return
	}
	b.e.MovRegImm32(RAX, b.sysWrite())
	b.e.Syscall()
	// NOTE: keep this body as raw emitter calls — the 148C replaceAll
	// that introduced emitWrite once rewrote this branch into infinite
	// self-recursion. Do not route through emitWrite here.
}

// emitWinWrite emits write(1, RSI, RDX) through kernel32 WriteFile.
// Win64 passes the first four args in RCX,RDX,R8,R9 with a 32-byte
// caller shadow; kernel calls preserve RBX/RBP/RDI/RSI/R12-R15
// (callee-saved) but may clobber everything else — including RCX/RDX
// on return — so ptr/len ride the stack across the GetStdHandle call
// and registers reload after it. print_float issues several small
// writes back-to-back (sign, int part, ".", fraction, newline), so
// preserving RSI across this boundary is load-bearing: without it the
// second write reuses a clobbered pointer and prints garbage.
func (b *Builder) emitWinWrite() {
	b.e.PushReg(RSI)
	b.e.PushReg(RDX)
	b.e.PushReg(RBX)
	b.e.SubRsp(32)
	b.e.MovRegImm32(RCX, 0xFFFFFFF5) // STD_OUTPUT_HANDLE
	b.iatCall(IATGetStdHandle)
	b.e.AddRsp(32)
	b.e.PopReg(RDX)
	b.e.PopReg(RSI)
	b.e.MovRegReg(RCX, RAX)
	b.e.MovRegReg(R8, RDX)
	b.e.MovRegReg(RDX, RSI)
	// 48 = 32 shadow + 8 written-dword + 8 pad: Win64 requires
	// rsp%16==8 before the call (40 would flip it to 0 and fault).
	b.e.SubRsp(48)
	b.e.LeaRegStack(R9, 24)
	b.e.XorRegReg(RAX)
	b.e.StoreStack(RAX, 32)
	b.iatCall(IATWriteFile)
	b.e.AddRsp(48)
	// Win64 WriteFile may clobber RCX/RDX (and R8-R11) on return, so
	// restore the caller's RBX after the shadow is released. The
	// print_float integer loop keeps its end pointer in RBX across
	// writes; without this the digit length computes from garbage.
	b.e.PopReg(RBX)
}

// emitWindowsExit emits the _start tail on Windows: ExitProcess(main's
// value). Win64 requires caller shadow even for a single argument
// (entry alignment itself is fixed by AndRspNeg16 before the main call).
func (b *Builder) emitWindowsExit() {
	b.e.MovRegReg(RCX, RAX)
	b.e.SubRsp(32)
	b.iatCall(IATExitProcess)
}

// emitWinBootstrap emits the loader-independent kernel32 bootstrap
// (Phase 149): the host loader maps the image and runs entry but does
// not snap the IAT on minimal images (proven: slots keep file content
// at runtime), so _start resolves ExitProcess/GetStdHandle/WriteFile
// itself via the PEB and publishes the addresses into the IAT slots
// (writable .idata) before main runs. Standard shellcode technique,
// fully deterministic: PEB (gs:[0x60]) -> Ldr -> module walk matching
// "kernel32.dll" (never positional) -> export table walk per name.
// Clobbers RAX,RCX,RDX,RSI,RDI,RBP,R8-R11 (nothing is live pre-main);
// RSP is never moved. A missing module or export hits Int3: loud,
// never silent wrong-code.
func (b *Builder) emitWinBootstrap() {
	b.e.MovRegGsMem(RAX, 0x60)     // PEB
	b.e.LoadBaseOff(RAX, RAX, 0x18) // PEB->Ldr
	b.e.LoadBaseOff(RAX, RAX, 0x20) // InMemoryOrder head
	b.e.MovRegImm32(R11, 64)        // walk bound (every process has kernel32)
	walkLbl := b.fresh("k32walk")
	nextLbl := b.fresh("k32next")
	foundLbl := b.fresh("k32found")
	failLbl := b.fresh("k32fail")
	b.e.Mark(walkLbl)
	b.e.LoadBaseOff(RAX, RAX, 0) // Flink -> entry links
	b.e.MovRegReg(RBX, RAX)
	b.e.SubRegImm32(RBX, 0x10) // links field -> entry base
	// Phase-149 correction: 64-bit LDR_DATA_TABLE_ENTRY puts DllBase at
	// +0x30 and BaseDllName at +0x58 (Length +0, Buffer +8); +0x28/+0x50
	// is the 32-bit shape and skipped every module into the Int3.
	b.e.MovzxRegMem16(RCX, RBX, 0x58)
	b.e.CmpRegImm32(RCX, 24) // "kernel32.dll" is 24 bytes
	b.e.Jnz(nextLbl)
	b.e.LoadBaseOff(RDX, RBX, 0x60) // BaseDllName.Buffer
	// Phase-149 ground truth: BaseDllName arrives UPPERCASE
	// (KERNEL32.DLL — read live from the PEB), so fold each WCHAR
	// with 0x20 before comparing against lowercase (folding is a
	// no-op for the digits and dots in this alphabet).
	for k, ch := range "kernel32.dll" {
		b.e.MovzxRegMem16(R8, RDX, k*2)
		b.e.OrRegImm8(R8, 0x20)
		b.e.CmpRegImm32(R8, uint32(ch))
		b.e.Jnz(nextLbl)
	}
	b.e.LoadBaseOff(R10, RBX, 0x30) // DllBase -> kept for all resolves
	b.e.Jmp(foundLbl)
	b.e.Mark(nextLbl)
	b.e.DecReg(R11)
	b.e.Jnz(walkLbl)
	b.e.Mark(failLbl)
	b.e.Int3()
	b.e.Mark(foundLbl)
	b.emitWinResolve("ExitProcess", IATExitProcess)
	b.emitWinResolve("GetStdHandle", IATGetStdHandle)
	b.emitWinResolve("WriteFile", IATWriteFile)
}

// emitWinResolve emits one export-table walk resolving a kernel32 name
// (R10 holds the module base): names/ordinals/functions pointers,
// count-bounded candidate loop with unrolled byte compares, resolved
// address published to the IAT slot. Falls into Int3 when exhausted.
func (b *Builder) emitWinResolve(name string, slot int) {
	b.e.LoadBaseOff32(RCX, R10, 0x3C) // e_lfanew
	b.e.MovRegReg(RDX, R10)
	b.e.AddRegReg(RDX, RCX) // RDX = NT headers
	// Phase-149 ground truth: the export directory is at NT+136
	// (COFF 24 + Optional 112 — the +96 shape is the 32-bit header;
	// verified live against kernel32: dir[0] VA 0xA65F0). The old +120
	// read garbage and faulted the first namesPtr load.
	b.e.LoadBaseOff32(RCX, RDX, 136) // export directory RVA
	b.e.MovRegReg(RDX, R10)
	b.e.AddRegReg(RDX, RCX) // RDX = export directory
	b.e.LoadBaseOff32(R9, RDX, 24)  // NumberOfNames
	b.e.LoadBaseOff32(RCX, RDX, 32) // AddressOfNames
	b.e.MovRegReg(RSI, R10)
	b.e.AddRegReg(RSI, RCX) // RSI = namesPtr
	b.e.LoadBaseOff32(RCX, RDX, 36) // AddressOfNameOrdinals
	b.e.MovRegReg(RDI, R10)
	b.e.AddRegReg(RDI, RCX) // RDI = ordPtr
	b.e.LoadBaseOff32(RCX, RDX, 28) // AddressOfFunctions
	b.e.MovRegReg(RBP, R10)
	b.e.AddRegReg(RBP, RCX) // RBP = funcsPtr
	loopLbl := b.fresh("exprt")
	nextLbl := b.fresh("expnext")
	doneLbl := b.fresh("expdone")
	failLbl := b.fresh("expfail")
	b.e.Mark(loopLbl)
	b.e.TestRegReg(R9, R9)
	b.e.Jz(failLbl)
	b.e.LoadBaseOff32(RCX, RSI, 0) // nameRVA
	b.e.MovRegReg(RDX, R10)
	b.e.AddRegReg(RDX, RCX) // RDX = candidate name
	// Export names are ASCII bytes; compare them as aligned pairs
	// (odd tail pairs against the NUL terminator, always mapped).
	for k := 0; k < len(name); k += 2 {
		lo := uint32(name[k])
		var hi uint32
		if k+1 < len(name) {
			hi = uint32(name[k+1])
		}
		b.e.MovzxRegMem16(R8, RDX, k)
		b.e.CmpRegImm32(R8, lo|hi<<8)
		b.e.Jnz(nextLbl)
	}
	b.e.MovzxRegMem16(R8, RDI, 0) // ordinal
	b.e.LoadScaled32(RAX, RBP, R8, 4, 0)
	b.e.MovRegReg(RDX, R10)
	b.e.AddRegReg(RDX, RAX)
	// Phase-149 correction: 48 A3 stores RAX specifically (moffs form
	// is accumulator-only), so the resolved address moves RDX -> RAX
	// first — storing RDX's predecessor (the raw funcRVA) was silently
	// publishing garbage into the slot.
	b.e.MovRegReg(RAX, RDX)
	pos := b.e.StoreAbs64Placeholder()
	b.apatches = append(b.apatches, absPatch{pos: pos, index: slot})
	b.e.Jmp(doneLbl)
	b.e.Mark(nextLbl)
	b.e.AddRegImm32(RSI, 4)
	b.e.AddRegImm32(RDI, 2)
	b.e.DecReg(R9)
	b.e.Jmp(loopLbl)
	b.e.Mark(failLbl)
	b.e.Int3()
	b.e.Mark(doneLbl)
}

// iatCall emits movabs rax, <IAT slot index> + call rax for a kernel32
// import (Phase 149, PE target only). The slot address resolves at link
// time against the .idata base, reusing the imm64 placeholder machinery.
func (b *Builder) iatCall(index int) {
	pos := b.e.imm64Patch(RAX)
	b.ipatches = append(b.ipatches, iatPatch{pos: pos, index: index})
	b.e.CallReg(RAX)
}

func (b *Builder) emitHelpers() {
	b.e.Mark("print_int")
	b.e.SubRsp(64)
	b.e.MovRegReg(RAX, RDI)
	b.e.MovRegReg(RSI, RSP)
	b.e.AddRegImm32(RSI, 64)
	b.e.TestRegReg(RAX, RAX)
	b.e.Jns("print_int_pos")
	b.e.NegReg(RAX)
	b.e.PushReg(RAX)
	b.e.PushReg(RSI)
	b.e.MovRegImm32(RDX, 1)
	b.e.MovRegImm32(RAX, b.sysWrite())
	b.e.MovRegImm32(RDI, 1)
	b.rodataRef(RSI, "-")
	b.e.Syscall()
	b.e.PopReg(RSI)
	b.e.PopReg(RAX)
	b.e.Mark("print_int_pos")
	b.e.MovRegImm32(RCX, 10)
	b.e.Mark("print_int_loop")
	b.e.Cqo()
	b.e.DivReg(RCX)
	b.e.AddRegImm32(RDX, '0')
	b.e.DecReg(RSI)
	b.e.StoreMem8(RSI, RDX)
	b.e.TestRegReg(RAX, RAX)
	b.e.Jnz("print_int_loop")
	b.e.MovRegReg(RBX, RSP)
	b.e.AddRegImm32(RBX, 64)
	b.e.MovRegReg(RDX, RBX)
	b.e.SubRegReg(RDX, RSI)
	b.e.MovRegImm32(RDI, 1)
	b.emitWrite()
	b.e.AddRsp(64)
	// Phase 147: `print` terminates the line (matches the C-backend
	// contract the execution goldens pin: "42\n", not "42"). RDI is
	// still 1 from the payload write; syscalls clobber only RCX/R11.
	b.printNewline()
	b.e.Ret()

	b.e.Mark("print_str")
	b.e.MovRegReg(RDX, RSI)
	b.e.MovRegReg(RSI, RDI)
	b.e.MovRegImm32(RDI, 1)
	b.emitWrite()
	b.printNewline()
	b.e.Ret()

	// print_float: exact %g-compatible formatter for binary64.
	// Input: RDI = f64 bit pattern. Output: decimal text + '\n' to stdout.
	// Scratch: 20 × 8-byte limb array at [RSP+FRAME_LIMBS], digit buffer
	// at [RSP+FRAME_DIGITS], plus a few scalar slots.
	// Algorithm: exact binary64 decomposition → 20-limb big-int (limb[i]
	// is u64, base 2^64) → repeated multiply by 10 with full-limb
	// carry → extract 7th digit + sticky for round-half-even → format
	// per %g (fixed if -4 ≤ X < 6 else scientific) → trim trailing
	// zeros → write via emitWrite.
	//
	// Phase 150A: emitted only when the program can produce a float at all
	// (scanFloatUsage). print_float is the sole consumer of that state, and
	// gating it is what keeps int/string-only images byte-identical to
	// increment 149 (the ELF byte-identity differential pins that).
	if b.usesFloat {
		b.emitPrintFloatHelper()
	}

	// Phase 150B: the bump allocator, emitted only when the program
	// allocates (heapSize > 0). Same gating rationale as print_float.
	if b.heapSize > 0 {
		b.emitAllocHelper()
	}
}

// emitAllocHelper emits the Phase-150B bump allocator.
//
//	alloc(n)  ; RDI = n -> RAX = block of n bytes, or Int3 on exhaustion
//
// The arena is a 16-byte header ([cursor][limit]) followed by the data
// area, carried in a writable segment. Both addresses are absolute, and
// both are re-materialised here from link-time constants on every call, so
// nothing depends on caller-saved registers surviving the call. There is no
// free(): the arena is bump-only, which is sufficient for the compile-time
// bounded allocations 150B performs (string concat results, push results)
// and keeps the invariant "total live bytes <= arena size" checkable at
// compile time. Exhaustion is an Int3 — loud, never a silent overlap.
func (b *Builder) emitAllocHelper() {
	haveLbl := b.fresh("alloc$have")
	oomLbl := b.fresh("alloc$oom")
	doneLbl := b.fresh("alloc$done")
	b.e.Mark("alloc")
	// R10 = arena base, R11 = arena end: both from link-time imm64 patches
	// that the container resolves (and, on PE, that the DIR64 fixups cover).
	b.heapRef(R10, 0)
	b.heapRef(R11, heapHeaderLen+b.heapSize)
	b.e.LoadBaseOff(RAX, R10, heapCursorOff)
	b.e.TestRegReg(RAX, RAX)
	b.e.Jnz(haveLbl)
	// First call: the cursor is zero in the image, so start it after the
	// header. No separate init step, and no re-initialisation after that
	// because the stored cursor is non-zero from here on.
	b.e.MovRegReg(RAX, R10)
	b.e.AddRegImm32(RAX, heapHeaderLen)
	b.e.StoreBaseOff(RAX, R10, heapCursorOff)
	b.e.Mark(haveLbl)
	// new = cursor + n, rejected if it passes the limit. `ja` (unsigned
	// above) also catches the wraparound when n is absurd.
	b.e.MovRegReg(RCX, RDI)
	b.e.AddRegReg(RCX, RAX)
	b.e.CmpRegReg(RCX, R11)
	b.e.Ja(oomLbl)
	b.e.StoreBaseOff(RCX, R10, heapCursorOff)
	// RAX already holds the block start (the pre-bump cursor) on both paths
	// through here: the loaded cursor, or arena+16 on the first call. The
	// bumped cursor lives in RCX and goes to the arena, so the return value
	// is untouched.
	b.e.Jmp(doneLbl)
	b.e.Mark(oomLbl)
	b.e.Int3()
	b.e.Mark(doneLbl)
	b.e.Ret()
}

// heapRef records an imm64 site to be resolved to arenaBase+off at link time.
func (b *Builder) heapRef(r Reg, off int) {
	b.hpatches = append(b.hpatches, addrPatch{pos: b.e.imm64Patch(r), roOff: uint64(off)})
}

// emitPrintFloatHelper emits the print_float routine (Phase 150A).
//
// Input: RDI = f64 bit pattern. Output: decimal text + newline to stdout.
// Scratch: 64 bytes at [RSP+0..64); integer digits grow down from
// RSP+64, the 6 fraction bytes sit at [RSP+0..6).
//
// v1 semantics (documented, NOT full %g): finite magnitudes below 2^63
// print with up to 6 fractional digits and trailing fractional zeros
// trimmed ("1" not "1.000000"); NaN/Inf print "[-]nan"/"[-]inf" like the
// C backend. Magnitudes >= 2^63 saturate the truncating convert and are
// a known v1 boundary for 150B review. Shared by all three OS
// containers: output goes through emitWrite/printNewline only, so
// Linux/macOS syscalls and the Win64 WriteFile boundary work unchanged.
func (b *Builder) emitPrintFloatHelper() {
	b.e.Mark("print_float")
	b.e.SubRsp(64)
	b.e.MovRegReg(RAX, RDI)
	b.e.MovRegImm64(RCX, 1<<63)
	b.e.MovRegReg(R11, RAX)
	b.e.AndRegReg(R11, RCX)
	b.e.MovRegImm64(RCX, 0x7FFFFFFFFFFFFFFF)
	b.e.AndRegReg(RAX, RCX)
	b.e.MovRegReg(RCX, RAX)
	b.e.ShrRegImm(RCX, 52)
	b.e.MovRegImm32(R10, 0x7FF)
	b.e.AndRegReg(RCX, R10)
	b.e.CmpRegImm32(RCX, 0x7FF)
	specialLbl := b.fresh("fltsp")
	finiteLbl := b.fresh("fltfin")
	b.e.Jz(specialLbl)
	b.e.Jmp(finiteLbl)
	b.e.Mark(specialLbl)
	b.e.MovRegImm64(R10, 0x000FFFFFFFFFFFFF)
	b.e.AndRegReg(RAX, R10)
	b.e.TestRegReg(RAX, RAX)
	nanLbl := b.fresh("fltnan")
	b.e.Jnz(nanLbl)
	b.e.TestRegReg(R11, R11)
	noSignInf := b.fresh("fltnoinf")
	b.e.Jz(noSignInf)
	b.rodataRef(RSI, "-")
	b.e.MovRegImm32(RDX, 1)
	b.e.MovRegImm32(RDI, 1)
	b.emitWrite()
	b.e.Mark(noSignInf)
	b.rodataRef(RSI, "inf")
	b.e.MovRegImm32(RDX, 3)
	b.e.MovRegImm32(RDI, 1)
	b.emitWrite()
	b.printNewline()
	b.e.AddRsp(64)
	b.e.Ret()
	b.e.Mark(nanLbl)
	b.e.TestRegReg(R11, R11)
	noSignNan := b.fresh("fltnonan")
	b.e.Jz(noSignNan)
	b.rodataRef(RSI, "-")
	b.e.MovRegImm32(RDX, 1)
	b.e.MovRegImm32(RDI, 1)
	b.emitWrite()
	b.e.Mark(noSignNan)
	b.rodataRef(RSI, "nan")
	b.e.MovRegImm32(RDX, 3)
	b.e.MovRegImm32(RDI, 1)
	b.emitWrite()
	b.printNewline()
	b.e.AddRsp(64)
	b.e.Ret()
	b.e.Mark(finiteLbl)
	b.e.MovXmmRegGp(XMM0, RAX)
	b.e.Cvttsd2siGpXmm(RCX, XMM0)
	b.e.Cvtsi2sdXmmGp(XMM1, RCX)
	b.e.SubsdXmmXmm(XMM0, XMM1)
	b.e.TestRegReg(R11, R11)
	noSign := b.fresh("fltnosign")
	b.e.Jz(noSign)
	b.rodataRef(RSI, "-")
	b.e.MovRegImm32(RDX, 1)
	b.e.MovRegImm32(RDI, 1)
	b.emitWrite()
	b.e.Mark(noSign)
	b.e.MovRegReg(RAX, RCX)
	b.e.TestRegReg(RAX, RAX)
	intNz := b.fresh("fltintnz")
	intDone := b.fresh("fltintdone")
	b.e.Jnz(intNz)
	b.rodataRef(RSI, "0")
	b.e.MovRegImm32(RDX, 1)
	b.e.MovRegImm32(RDI, 1)
	b.emitWrite()
	b.e.Jmp(intDone)
	b.e.Mark(intNz)
	b.e.MovRegImm32(R10, 10)
	b.e.LeaRegStack(RSI, 64)
	intLoop := b.fresh("fltintloop")
	b.e.Mark(intLoop)
	b.e.Cqo()
	b.e.DivReg(R10)
	b.e.AddRegImm32(RDX, '0')
	b.e.DecReg(RSI)
	b.e.StoreMem8(RSI, RDX)
	b.e.TestRegReg(RAX, RAX)
	b.e.Jnz(intLoop)
	b.e.LeaRegStack(RBX, 64)
	b.e.MovRegReg(RDX, RBX)
	b.e.SubRegReg(RDX, RSI)
	b.e.MovRegImm32(RDI, 1)
	b.emitWrite()
	b.e.Mark(intDone)
	b.e.MovRegImm64(R10, 0x412E848000000000)
	b.e.MovXmmRegGp(XMM1, R10)
	b.e.MulsdXmmXmm(XMM0, XMM1)
	b.e.Cvttsd2siGpXmm(RCX, XMM0)
	b.e.MovRegImm32(R10, 100000)
	b.e.MovRegReg(RAX, RCX)
	b.e.Cqo()
	b.e.DivReg(R10)
	b.e.AddRegImm32(RAX, '0')
	b.e.StoreMem8Off(RAX, RSP, 0)
	b.e.MovRegReg(RCX, RDX)
	b.e.MovRegImm32(R10, 10000)
	b.e.MovRegReg(RAX, RCX)
	b.e.Cqo()
	b.e.DivReg(R10)
	b.e.AddRegImm32(RAX, '0')
	b.e.StoreMem8Off(RAX, RSP, 1)
	b.e.MovRegReg(RCX, RDX)
	b.e.MovRegImm32(R10, 1000)
	b.e.MovRegReg(RAX, RCX)
	b.e.Cqo()
	b.e.DivReg(R10)
	b.e.AddRegImm32(RAX, '0')
	b.e.StoreMem8Off(RAX, RSP, 2)
	b.e.MovRegReg(RCX, RDX)
	b.e.MovRegImm32(R10, 100)
	b.e.MovRegReg(RAX, RCX)
	b.e.Cqo()
	b.e.DivReg(R10)
	b.e.AddRegImm32(RAX, '0')
	b.e.StoreMem8Off(RAX, RSP, 3)
	b.e.MovRegReg(RCX, RDX)
	b.e.MovRegImm32(R10, 10)
	b.e.MovRegReg(RAX, RCX)
	b.e.Cqo()
	b.e.DivReg(R10)
	b.e.AddRegImm32(RAX, '0')
	b.e.StoreMem8Off(RAX, RSP, 4)
	b.e.AddRegImm32(RDX, '0')
	b.e.StoreMem8Off(RDX, RSP, 5)
	b.e.MovRegImm32(R11, 6)
	b.e.LeaRegStack(RAX, 0)
	trimLoop := b.fresh("flttrim")
	trimDone := b.fresh("flttrimdone")
	b.e.Mark(trimLoop)
	b.e.MovRegReg(RCX, R11)
	b.e.AddRegReg(RAX, RCX)
	b.e.MovzxRegMem8(RCX, RAX, -1)
	b.e.LeaRegStack(RAX, 0)
	b.e.CmpRegImm32(RCX, '0')
	b.e.Jnz(trimDone)
	b.e.DecReg(R11)
	b.e.TestRegReg(R11, R11)
	b.e.Jnz(trimLoop)
	b.e.Mark(trimDone)
	b.e.TestRegReg(R11, R11)
	noFrac := b.fresh("fltnofrac")
	b.e.Jz(noFrac)
	b.rodataRef(RSI, ".")
	b.e.MovRegImm32(RDX, 1)
	b.e.MovRegImm32(RDI, 1)
	b.emitWrite()
	b.e.LeaRegStack(RSI, 0)
	b.e.MovRegReg(RDX, R11)
	b.e.MovRegImm32(RDI, 1)
	b.emitWrite()
	b.e.Mark(noFrac)
	b.printNewline()
	b.e.AddRsp(64)
	b.e.Ret()
}


// printNewline emits write(1, "\n", 1) with RDI already holding 1.
func (b *Builder) printNewline() {
	b.rodataRef(RSI, "\n")
	b.e.MovRegImm32(RDX, 1)
	b.emitWrite()
}

func (b *Builder) layout(fd *parser.FuncDecl) error {
	b.slots = map[string]int{}
	b.kinds = map[string]int{}
	b.scanErr = nil
	b.forIdx = map[*parser.ForInStmt]int{}
	next := 0
	for i, p := range fd.Params {
		// Phase 148: string parameters occupy two slots (ptr+len).
		// Phase 150A: array parameters occupy two slots (ptr+len) as
		// well, but no annotation syntax names them yet — a body that
		// indexes a parameter is a loud K145 (150B work).
		b.slots[p] = next
		b.kinds[p] = paramKind(fd, i)
		next += 8 * kindUnits(b.kinds[p])
	}
	next = b.scanLets(fd.Body, next)
	if b.scanErr != nil {
		return b.scanErr
	}
	// Phase 147: binary-operand scratch stack (see maxBinDepth). rsp never
	// moves during expression evaluation, so slot addresses stay stable.
	b.binTemp = next
	next += 8 * maxBinDepth
	// Phase 150B: string staging (concat operands, string-comparison
	// operands), five 8-byte units per nesting level (see emitStrConcat).
	// Reserved only when the program has a string operation that needs it, so
	// a program that does not keeps its exact increment-149 frame shape (and
	// therefore its exact bytes). The decision is program-wide and taken
	// once in a pre-pass, so layout and emission can never disagree.
	if b.usesConcat || b.usesStrEq || b.usesStrSlice || b.usesPush {
		b.strTemp = next
		next += 8 * 5 * maxBinDepth
	}
	// Phase 148: caller-frame extras area for argument units past the six
	// register units (sized by the hungriest call site in this function).
	maxExtra := b.scanMaxExtras(fd.Body)
	b.extrasBase = next
	next += maxExtra * 8
	// Phase 149: Win64 calls fault on misaligned stacks, so the frame
	// is rounded to 16 on Windows (extras sizing can otherwise leave it
	// 8-mod-16; Linux never noticed because syscalls don't care, and its
	// images stay byte-frozen by gating this to Windows).
	if b.goos == OSWindows && next%16 != 0 {
		next += 8
	}
	b.frame = next + argSpillBytes
	return nil
}

// scanMaxExtras returns the largest extras-unit count of any call in a
// statement list: total caller-side 8-byte units (int 1, string 2) minus
// the six register units, floored at 0. It walks every expression
// position so nested calls size the area too; unknown callees are
// ignored here (emission rejects them loudly).
func (b *Builder) scanMaxExtras(stmts []parser.Node) int {
	max := 0
	var walkExpr func(n parser.Node)
	walkExpr = func(n parser.Node) {
		switch x := n.(type) {
		case *parser.CallExpr:
			units := 0
			for _, a := range x.Args {
				if isStr, err := b.isStringExpr(a); err == nil && isStr {
					units += 2
				} else {
					units++
				}
			}
			if e := units - len(argRegs); e > max {
				max = e
			}
			for _, a := range x.Args {
				walkExpr(a)
			}
		case *parser.IndirectCallExpr:
			for _, a := range x.Args {
				walkExpr(a)
			}
			walkExpr(x.Target)
		case *parser.BinaryExpr:
			walkExpr(x.Left)
			walkExpr(x.Right)
		case *parser.UnaryExpr:
			walkExpr(x.Operand)
		case *parser.IndexExpr:
			walkExpr(x.Left)
			walkExpr(x.Index)
		case *parser.ArrayLiteral:
			for _, e := range x.Elements {
				walkExpr(e)
			}
		case *parser.StructLiteral:
			for _, f := range x.Fields {
				walkExpr(f)
			}
		}
	}
	var walkStmts func(stmts []parser.Node)
	walkStmts = func(stmts []parser.Node) {
		for _, s := range stmts {
			switch n := s.(type) {
			case *parser.VarDeclStmt:
				if n.Value != nil {
					walkExpr(n.Value)
				}
			case *parser.ReturnStmt:
				if n.Value != nil {
					walkExpr(n.Value)
				}
			case *parser.ExprStmt:
				walkExpr(n.Expression)
			case *parser.PrintStmt:
				walkExpr(n.Value)
			case *parser.IfStmt:
				walkExpr(n.Condition)
				walkStmts(n.Consequence)
				walkStmts(n.Alternative)
			case *parser.WhileStmt:
				walkExpr(n.Condition)
				walkStmts(n.Body)
			case *parser.ForStmt:
				if n.Init != nil {
					walkStmts([]parser.Node{n.Init})
				}
				if n.Condition != nil {
					walkExpr(n.Condition)
				}
				if n.Post != nil {
					post := n.Post
					if be, ok := post.(*parser.BinaryExpr); ok && be.Operator == "=" {
						post = &parser.ExprStmt{Expression: be, Line: be.Line}
					}
					walkStmts([]parser.Node{post})
				}
				walkStmts(n.Body)
			case *parser.ForInStmt:
				walkExpr(n.Iter)
				walkStmts(n.Body)
			case *parser.BlockStmt:
				walkStmts(n.Statements)
			}
		}
	}
	walkStmts(stmts)
	return max
}
// into control-flow bodies and C-for initializers (Phase 148: loop bodies
// may declare variables; slots are function-wide, first declaration wins
// a slot and shadowing writes through — v1 semantics, documented). The
// first isStringExpr failure is recorded and returned by layout; emission
// re-validates every node anyway, so the diagnostic is identical.
func (b *Builder) scanLets(stmts []parser.Node, next int) int {
	for _, s := range stmts {
		switch n := s.(type) {
		case *parser.VarDeclStmt:
			if _, seen := b.slots[n.Name]; seen {
				continue
			}
			off := next
			next += 8
			k, err := b.exprKind(n.Value)
			if err != nil {
				if b.scanErr == nil {
					b.scanErr = err
				}
				return next
			}
			if kindUnits(k) == 2 {
				next += 8
			}
			if k == KindArray {
				// Phase 150A: array element storage lives in the frame
				// right after the (ptr,len) header, so every array has
				// a fixed compile-time footprint: header + N int slots.
				// Only int-element literals lower in 150A (see exprKind).
				if lit, ok := n.Value.(*parser.ArrayLiteral); ok {
					next += 8 * len(lit.Elements)
				}
			}
			b.slots[n.Name] = off
			b.kinds[n.Name] = k
		case *parser.IfStmt:
			next = b.scanLets(n.Consequence, next)
			next = b.scanLets(n.Alternative, next)
		case *parser.WhileStmt:
			next = b.scanLets(n.Body, next)
		case *parser.ForStmt:
			if n.Init != nil {
				next = b.scanLets([]parser.Node{n.Init}, next)
			}
			next = b.scanLets(n.Body, next)
		case *parser.ForInStmt:
			// Phase 150A: the loop variable owns a plain int slot and
			// the hidden index owns 8 bytes reserved after it, so every
			// loop — including a nested one — gets its own pair. Keying
			// the index by the statement node (not a shared name) is
			// what keeps nesting correct: one shared slot would let the
			// inner loop resume its parent with the inner counter.
			// Registration runs before the body scan so the body can
			// read the variable. Other slot names are function-wide with
			// first-declaration-wins, so shadowing writes through — the
			// documented v1 semantics.
			if n.KeyName == "" && n.VarName != "" {
				if _, seen := b.slots[n.VarName]; !seen {
					b.slots[n.VarName] = next
					b.kinds[n.VarName] = KindInt
					next += 8
				}
				b.forIdx[n] = next
				next += 8
			}
			next = b.scanLets(n.Body, next)
		case *parser.BlockStmt:
			next = b.scanLets(n.Statements, next)
		}
	}
	return next
}

func (b *Builder) emitFunc(fd *parser.FuncDecl) error {
	// Phase 148: main takes no arguments on the native target (there is
	// no argv protocol in v1; _start invokes it bare).
	if fd.Name == "main" && len(fd.Params) > 0 {
		return fmt.Errorf("error[K145]: 'main' takes no arguments on the native target")
	}
	name := "fn_" + fd.Name
	if fd.Name == "main" {
		name = "karkain_main"
	}
	b.e.Mark(name)
	if err := b.layout(fd); err != nil {
		return err
	}
	if b.frame > 0 {
		b.e.SubRsp(b.frame)
	}
	// Home the parameters under the native ABI (Phase 148): the first
	// six 8-byte units arrive in RDI,RSI,RDX,RCX,R8,R9 (a string takes
	// two units); further units arrive in the caller-frame extras array
	// addressed by R10. R10 dies at the first call, so stack homing runs
	// here at entry, before anything else.
	unit := 0
	for _, p := range fd.Params {
		units := kindUnits(b.kinds[p])
		for k := 0; k < units; k++ {
			if unit < len(argRegs) {
				b.e.StoreStack(argRegs[unit], b.slots[p]+k*8)
			} else {
				b.e.LoadBaseOff(RAX, R10, (unit-len(argRegs))*8)
				b.e.StoreStack(RAX, b.slots[p]+k*8)
			}
			unit++
		}
	}
	returned := false
	for _, s := range fd.Body {
		done, err := b.emitStmt(s, fd.Name)
		if err != nil {
			return err
		}
		if done {
			returned = true
		}
	}
	if !returned {
		b.e.XorRegReg(RAX)
	}
	b.e.Mark(name + "$ret")
	if b.frame > 0 {
		b.e.AddRsp(b.frame)
	}
	b.e.Ret()
	return nil
}

// emitStmt lowers one statement. It returns done=true for a mid-body
// `return` (the caller records it so a missing trailing return still
// yields 0). Phase 148: factored from emitFunc so loop and branch bodies
// reuse the exact straight-line paths (byte-identical for straight-line
// programs by construction).
func (b *Builder) emitStmt(s parser.Node, fname string) (bool, error) {
	switch n := s.(type) {
	case *parser.VarDeclStmt:
		if err := b.emitLet(n); err != nil {
			return false, err
		}
		return false, nil
	case *parser.PrintStmt:
		if err := b.emitPrint(n); err != nil {
			return false, err
		}
		return false, nil
	case *parser.ReturnStmt:
		if err := b.emitReturn(n, fname == "main"); err != nil {
			return false, err
		}
		name := "fn_" + fname
		if fname == "main" {
			name = "karkain_main"
		}
		b.e.Jmp(name + "$ret")
		return true, nil
	case *parser.ExprStmt:
		if err := b.emitExprStmt(n); err != nil {
			return false, err
		}
		return false, nil
	case *parser.IfStmt:
		return false, b.emitIf(n, fname)
	case *parser.WhileStmt:
		return false, b.emitWhile(n, fname)
	case *parser.ForStmt:
		return false, b.emitFor(n, fname)
	case *parser.BreakStmt:
		if len(b.loops) == 0 {
			return false, fmt.Errorf("error[K145]: break outside of a loop")
		}
		b.e.Jmp(b.loops[len(b.loops)-1].brk)
		return false, nil
	case *parser.ContinueStmt:
		if len(b.loops) == 0 {
			return false, fmt.Errorf("error[K145]: continue outside of a loop")
		}
		b.e.Jmp(b.loops[len(b.loops)-1].cont)
		return false, nil
	case *parser.ForInStmt:
		return false, b.emitForIn(n, fname)
	default:
		return false, fmt.Errorf("error[K145]: unsupported statement %T in '%s'", s, fname)
	}
}

// emitExprStmt lowers an expression statement: plain value expressions
// evaluate and discard, while `x = <int>` reassigns a slot variable
// (Phase 148: loop counters need reassignment; strings are immutable
// through this form — declare a fresh variable instead).
func (b *Builder) emitExprStmt(n *parser.ExprStmt) error {
	if be, ok := n.Expression.(*parser.BinaryExpr); ok && be.Operator == "=" {
		id, ok := be.Left.(*parser.Identifier)
		if !ok {
			return fmt.Errorf("error[K145]: assignment target must be a variable")
		}
		off, ok := b.slots[id.Name]
		if !ok {
			return fmt.Errorf("error[K145]: undefined identifier '%s'", id.Name)
		}
		if b.kinds[id.Name] == KindString || b.kinds[id.Name] == KindArray {
			return fmt.Errorf("error[K145]: cannot reassign %s '%s' (declare a fresh variable)", kindName(b.kinds[id.Name]), id.Name)
		}
		rk, err := b.exprKind(be.Right)
		if err != nil {
			return err
		}
		if rk != b.kinds[id.Name] {
			return fmt.Errorf("error[K145]: cannot assign a %s to %s variable '%s'", kindName(rk), kindName(b.kinds[id.Name]), id.Name)
		}
		if err := b.emitExpr(be.Right, 0); err != nil {
			return err
		}
		b.e.StoreStack(RAX, off)
		return nil
	}
	return b.emitExpr(n.Expression, 0)
}

// emitCond evaluates an int or float comparison and jumps to falseLabel when
// it does NOT hold. Only `== != < <= > >=` over int or float expressions
// lower in v1 (operands reuse the depth-indexed scratch evaluator, so rsp
// stays put); anything else is a loud K145, never a miscompiled truthiness
// test.
// emitStrEqCond lowers `a == b` / `a != b` over two strings by comparing
// bytes, and jumps to falseLabel when the comparison does not hold.
//
// Length first (a different length is decisively unequal, with no byte
// reads at all), then a byte loop. Both operands stage through the concat
// scratch area so the second evaluation cannot disturb the first, and rsp
// never moves. RDX and R8 die; no other scratch is touched.
func (b *Builder) emitStrEqCond(be *parser.BinaryExpr, falseLabel string) error {
	lk, err := b.exprKind(be.Left)
	if err != nil {
		return err
	}
	if lk != KindString {
		return fmt.Errorf("error[K145]: %s operand in string comparison (no implicit conversion)", kindName(lk))
	}
	rk, err := b.exprKind(be.Right)
	if err != nil {
		return err
	}
	if rk != KindString {
		return fmt.Errorf("error[K145]: %s operand in string comparison (no implicit conversion)", kindName(rk))
	}
	base := b.strTemp
	if err := b.emitStr(be.Left, 0); err != nil {
		return err
	}
	b.e.StoreStack(RDI, base)
	b.e.StoreStack(RSI, base+8)
	if err := b.emitStr(be.Right, 0); err != nil {
		return err
	}
	b.e.StoreStack(RDI, base+16)
	b.e.StoreStack(RSI, base+24)
	neLbl := b.fresh("streq$ne")
	eqLbl := b.fresh("streq$eq")
	// Unequal lengths decide it immediately.
	b.e.LoadStack(RAX, base+8)
	b.e.CmpRegReg(RAX, RSI) // RSI still holds the right length
	b.e.Jnz(neLbl)
	// Byte loop: compare one byte at a time through the scaled 8-bit loads.
	b.e.LoadStack(R8, base)    // left ptr
	b.e.LoadStack(R9, base+16) // right ptr
	b.e.MovRegReg(RCX, RAX)    // count
	b.e.XorRegReg(RDX)
	loopLbl := b.fresh("streq$loop")
	b.e.Mark(loopLbl)
	b.e.CmpRegReg(RDX, RCX)
	b.e.Jae(eqLbl)
	b.e.LoadScaled8(R10, R8, RDX, 1, 0)
	b.e.LoadScaled8(R11, R9, RDX, 1, 0)
	b.e.CmpRegReg(R10, R11)
	b.e.Jnz(neLbl)
	b.e.IncReg(RDX)
	b.e.Jmp(loopLbl)
	// Two outcomes, and the operator only decides which one continues into
	// the then-branch. Falling through is "condition holds"; the caller
	// emits the falseLabel immediately after this returns, so the false side
	// is always the explicit jump and the true side is the fall-through.
	b.e.Mark(neLbl)
	if be.Operator == "==" {
		b.e.Jmp(falseLabel)
	}
	b.e.Mark(eqLbl)
	if be.Operator == "!=" {
		b.e.Jmp(falseLabel)
	}
	return nil
}

func (b *Builder) emitCond(cond parser.Node, falseLabel string) error {
	be, ok := cond.(*parser.BinaryExpr)
	if !ok {
		return fmt.Errorf("error[K145]: condition must be a comparison (==, !=, <, <=, >, >=) over ints or floats")
	}
	switch be.Operator {
	case "==", "!=", "<", "<=", ">", ">=":
	default:
		return fmt.Errorf("error[K145]: condition must be a comparison (==, !=, <, <=, >, >=) over ints or floats, found '%s'", be.Operator)
	}
	if isStr, err := b.isStringExpr(be.Left); err != nil {
		return err
	} else if isStr {
		// Phase 150B: strings compare by content. Only equality and
		// inequality exist in v1 — ordering needs a lexicographic helper
		// that lands with the map work, and a loud refusal is better than
		// a silent pointer comparison.
		if be.Operator != "==" && be.Operator != "!=" {
			return fmt.Errorf("error[K145]: string ordering ('%s') is not supported; == and != compare content", be.Operator)
		}
		return b.emitStrEqCond(be, falseLabel)
	}
	if isStr, err := b.isStringExpr(be.Right); err != nil {
		return err
	} else if isStr {
		// Either side being a string makes this a string comparison; the
		// handler names whichever operand is not one.
		if be.Operator != "==" && be.Operator != "!=" {
			return fmt.Errorf("error[K145]: string ordering ('%s') is not supported; == and != compare content", be.Operator)
		}
		return b.emitStrEqCond(be, falseLabel)
	}
	lk, err := b.exprKind(be.Left)
	if err != nil {
		return err
	}
	rk, err := b.exprKind(be.Right)
	if err != nil {
		return err
	}
	// Phase 150A: a float operand must not reach CmpRegReg, which would
	// compare the IEEE-754 bit patterns as integers (every positive float
	// is "greater" than every other). Float relations use ucomisd instead.
	if lk == KindFloat || rk == KindFloat {
		if lk != rk {
			return fmt.Errorf("error[K145]: mixed %s and %s in comparison (no implicit numeric conversion)", kindName(lk), kindName(rk))
		}
		return b.emitFloatCond(be, falseLabel)
	}
	var jump func(string)
	switch be.Operator {
	case "==":
		jump = b.e.Jnz
	case "!=":
		jump = b.e.Jz
	case "<":
		jump = b.e.Jge
	case "<=":
		jump = b.e.Jg
	case ">":
		jump = b.e.Jle
	case ">=":
		jump = b.e.Jl
	}
	if err := b.emitExpr(be.Left, 1); err != nil {
		return err
	}
	b.e.StoreStack(RAX, b.binTemp)
	if err := b.emitExpr(be.Right, 1); err != nil {
		return err
	}
	b.e.MovRegReg(RCX, RAX)
	b.e.LoadStack(RAX, b.binTemp)
	b.e.CmpRegReg(RAX, RCX)
	jump(falseLabel)
	return nil
}

// emitFloatCond lowers a float64 relation, jumping to falseLabel when it does
// NOT hold. ucomisd is unordered-aware: it sets CF=ZF=PF=1 when either
// operand is NaN, so every ordered form below reports a NaN relation as
// false — which is what C's `<` does and what pkg/codegen's binary_op
// computes (`l < r` on NaN is false). The 150A surface cannot produce NaN
// (division by zero yields 0.0, not an infinity), so this is a defined
// answer rather than an accidental one.
//
// `<` and `<=` compare the operands swapped: `a < b` is `b > a`, and the
// CF=1-on-unordered property of ucomisd then makes the "jump if not above"
// forms fall out directly with no extra NaN test.
func (b *Builder) emitFloatCond(be *parser.BinaryExpr, falseLabel string) error {
	if err := b.emitFloat(be.Left, 1); err != nil {
		return err
	}
	b.e.StoreStack(RAX, b.binTemp)
	if err := b.emitFloat(be.Right, 1); err != nil {
		return err
	}
	b.e.MovXmmRegGp(XMM1, RAX) // right -> xmm1
	b.e.LoadStack(RAX, b.binTemp)
	b.e.MovXmmRegGp(XMM0, RAX) // left -> xmm0
	switch be.Operator {
	case "==":
		// holds iff ordered and bit-equal: JP catches unordered (PF=1),
		// JNZ catches not-equal (ZF=0). pkg/codegen compares floats with
		// `l == r`, so this is exact equality, not the 1e-9 epsilon that
		// values_equal uses for assert-style comparisons.
		b.e.UcomisdXmmXmm(XMM0, XMM1)
		b.e.Jp(falseLabel)
		b.e.Jnz(falseLabel)
	case "!=":
		// holds iff unordered or not-equal; both need an explicit branch
		// because neither JNZ nor JZ alone can express "or unordered".
		hold := b.fresh("fne")
		b.e.UcomisdXmmXmm(XMM0, XMM1)
		b.e.Jp(hold)
		b.e.Jnz(hold)
		b.e.Jmp(falseLabel)
		b.e.Mark(hold)
	case "<":
		b.e.UcomisdXmmXmm(XMM1, XMM0)
		b.e.Jbe(falseLabel)
	case "<=":
		b.e.UcomisdXmmXmm(XMM1, XMM0)
		b.e.Jb(falseLabel)
	case ">":
		b.e.UcomisdXmmXmm(XMM0, XMM1)
		b.e.Jbe(falseLabel)
	case ">=":
		b.e.UcomisdXmmXmm(XMM0, XMM1)
		b.e.Jb(falseLabel)
	}
	return nil
}

func (b *Builder) emitStmts(stmts []parser.Node, fname string) error {
	for _, s := range stmts {
		if _, err := b.emitStmt(s, fname); err != nil {
			return err
		}
	}
	return nil
}

// emitIf lowers `if cond { consequence } else { alternative }`.
func (b *Builder) emitIf(n *parser.IfStmt, fname string) error {
	elseLabel := b.fresh("ifelse")
	endLabel := b.fresh("ifend")
	if err := b.emitCond(n.Condition, elseLabel); err != nil {
		return err
	}
	if err := b.emitStmts(n.Consequence, fname); err != nil {
		return err
	}
	b.e.Jmp(endLabel)
	b.e.Mark(elseLabel)
	if err := b.emitStmts(n.Alternative, fname); err != nil {
		return err
	}
	b.e.Mark(endLabel)
	return nil
}

// emitWhile lowers `while cond { body }`.
func (b *Builder) emitWhile(n *parser.WhileStmt, fname string) error {
	loopLabel := b.fresh("while")
	endLabel := b.fresh("whileend")
	b.e.Mark(loopLabel)
	b.loops = append(b.loops, loopTgt{brk: endLabel, cont: loopLabel})
	if err := b.emitCond(n.Condition, endLabel); err != nil {
		b.loops = b.loops[:len(b.loops)-1]
		return err
	}
	if err := b.emitStmts(n.Body, fname); err != nil {
		b.loops = b.loops[:len(b.loops)-1]
		return err
	}
	b.loops = b.loops[:len(b.loops)-1]
	b.e.Jmp(loopLabel)
	b.e.Mark(endLabel)
	return nil
}

// emitFor lowers C-style `for (init; cond; post) { body }`. A nil
// condition means always-true; init/post are arbitrary statements
// (typically `let` and reassignment) emitted inline.
func (b *Builder) emitFor(n *parser.ForStmt, fname string) error {
	if n.Init != nil {
		if _, err := b.emitStmt(n.Init, fname); err != nil {
			return err
		}
	}
	loopLabel := b.fresh("for")
	postLabel := b.fresh("forpost")
	endLabel := b.fresh("forend")
	b.e.Mark(loopLabel)
	b.loops = append(b.loops, loopTgt{brk: endLabel, cont: postLabel})
	if n.Condition != nil {
		if err := b.emitCond(n.Condition, endLabel); err != nil {
			b.loops = b.loops[:len(b.loops)-1]
			return err
		}
	}
	if err := b.emitStmts(n.Body, fname); err != nil {
		b.loops = b.loops[:len(b.loops)-1]
		return err
	}
	b.e.Mark(postLabel)
	if n.Post != nil {
		// The parser reads the post clause as a bare expression, so a
		// `i = i + 1` post arrives as BinaryExpr("="), not an ExprStmt —
		// wrap it so the assignment path handles it identically.
		post := n.Post
		if be, ok := post.(*parser.BinaryExpr); ok && be.Operator == "=" {
			post = &parser.ExprStmt{Expression: be, Line: be.Line}
		}
		if _, err := b.emitStmt(post, fname); err != nil {
			b.loops = b.loops[:len(b.loops)-1]
			return err
		}
	}
	b.loops = b.loops[:len(b.loops)-1]
	b.e.Jmp(loopLabel)
	b.e.Mark(endLabel)
	return nil
}

func (b *Builder) argTemp(i int) int {
	return b.frame - argSpillBytes + i*16
}

func (b *Builder) emitLet(n *parser.VarDeclStmt) error {
	isStr, err := b.isStringExpr(n.Value)
	if err != nil {
		return err
	}
	off := b.slots[n.Name]
	if isStr {
		if err := b.emitStr(n.Value, 0); err != nil {
			return err
		}
		b.e.StoreStack(RDI, off)
		b.e.StoreStack(RSI, off+8)
		return nil
	}
	if b.kinds[n.Name] == KindArray {
		// Phase 150A/150B: an array binding is either a literal (elements
		// materialized in the frame, emitArrayValue) or a push (a fresh array
		// allocated in the arena, emitPush). Both write the same two-slot
		// (base, len) header, so indexing, len and for-in work unchanged
		// over a pushed array.
		switch av := n.Value.(type) {
		case *parser.ArrayLiteral:
			return b.emitArrayValue(n.Name, av)
		case *parser.CallExpr:
			return b.emitPush(n.Name, av)
		default:
			return fmt.Errorf("error[K145]: array value must be a literal or a push() on the native target (got %T)", n.Value)
		}
	}
	if err := b.emitExpr(n.Value, 0); err != nil {
		return err
	}
	b.e.StoreStack(RAX, off)
	return nil
}
// emitForIn lowers `for x in arr { body }` over int arrays (Phase 150A).
//
// Layout contract (see scanLets): the array header owns two frame slots
// (base at off, length at off+8); the N int elements sit immediately
// after at [off+16 + i*8]. The loop keeps a hidden 8-byte index slot
// reserved after the element area; bound checks read the header length
// in place (CmpMemReg) and elements load via the scaled form with the
// array base as SIB base — never rsp — so rsp never moves inside the
// loop and every frame slot address stays stable (Phase 147's rule).
// `break`/`continue` reuse the loop-label stack, so control flow nests
// with while/C-for bodies identically. Map iteration (KeyName != "")
// and non-array iterables stay loud K145 (150B work).
// checkPushArgs validates push(arr, v): an array receiver (a variable, since
// the header is a frame pair) and an int element. Phase 150B v1 keeps push
// to int arrays; a float or string element is a loud refusal rather than a
// silent truncation.
func (b *Builder) checkPushArgs(x *parser.CallExpr) error {
	if x.Module != "" || x.IsCFunc {
		return fmt.Errorf("error[K145]: module-qualified and C-interop calls are not supported (call to '%s')", x.Function)
	}
	if len(x.Args) != 2 {
		return fmt.Errorf("error[K145]: push() takes exactly 2 arguments (got %d)", len(x.Args))
	}
	if _, ok := x.Args[0].(*parser.Identifier); !ok {
		return fmt.Errorf("error[K145]: push() requires an array variable (got %T)", x.Args[0])
	}
	rk, err := b.exprKind(x.Args[0])
	if err != nil {
		return err
	}
	if rk != KindArray {
		return fmt.Errorf("error[K145]: push() requires an array receiver (got %s)", kindName(rk))
	}
	vk, err := b.exprKind(x.Args[1])
	if err != nil {
		return err
	}
	if vk != KindInt {
		return fmt.Errorf("error[K145]: push() element must be int (got %s)", kindName(vk))
	}
	return nil
}

// emitPush lowers `let name = push(arr, v)`.
//
// push is functional: it allocates a *new* (base, len+1) array in the arena,
// copies the old elements, and appends v. The old array is untouched (the
// arena is bump-only, so nothing is freed or moved) — which means indexing,
// len and for-in all keep working on the result through the unchanged
// two-slot header, and the pre-existing source array is still readable.
//
// Elements are 8-byte ints, so the copy uses the scaled 64-bit load/store
// forms; rsp never moves (the whole frame stays at fixed offsets).
func (b *Builder) emitPush(name string, x *parser.CallExpr) error {
	if err := b.checkPushArgs(x); err != nil {
		return err
	}
	if b.heapSize <= 0 {
		return fmt.Errorf("error[K145]: internal: push() reached emission with no heap arena (the push pre-pass missed this site)")
	}
	// A push inside a loop can run an unbounded number of times, which the
	// compile-time arena size cannot cover. Refuse it by name rather than let
	// the program exhaust the arena and trap at an arbitrary iteration.
	if b.pushInLoop {
		return fmt.Errorf("error[K145]: push() inside a loop is not supported on the native target (the heap arena is sized at compile time, so a repeated push cannot be bounded)")
	}
	src := x.Args[0].(*parser.Identifier)
	off := b.slots[src.Name]
	// Everything the post-allocation work needs is staged in the frame first,
	// so the alloc call cannot be relied on to preserve anything: it uses
	// RAX/RCX/R10/R11 and takes RDI, and relying on R8/R9 surviving would be
	// exactly the kind of implicit contract that broke in Phase 147.
	stage := b.strTemp
	// value
	if err := b.emitExpr(x.Args[1], 0); err != nil {
		return err
	}
	b.e.StoreStack(RAX, stage)
	// old base / old len
	b.e.LoadStack(RAX, off)
	b.e.StoreStack(RAX, stage+8)
	b.e.LoadStack(RAX, off+8)
	b.e.StoreStack(RAX, stage+16)
	// bytes = (old len + 1) * 8
	b.e.LoadStack(RDI, stage+16)
	b.e.AddRegImm32(RDI, 8)
	b.e.ShlRegImm(RDI, 3)
	b.e.Call("alloc") // RAX = new base
	b.e.StoreStack(RAX, stage+24)
	// Copy the old elements: [oldbase + i*8] -> [newbase + i*8], i < old len.
	b.e.LoadStack(R8, stage+8)  // old base
	b.e.LoadStack(R9, stage+16) // old len
	b.e.LoadStack(R10, stage+24) // new base
	loop := b.fresh("push$cp")
	done := b.fresh("push$done")
	b.e.XorRegReg(RDX)
	b.e.Mark(loop)
	b.e.CmpRegReg(RDX, R9)
	b.e.Jae(done)
	b.e.LoadScaled64(RAX, R8, RDX, 8, 0)
	b.e.StoreScaled64(RAX, R10, RDX, 8, 0)
	b.e.IncReg(RDX)
	b.e.Jmp(loop)
	b.e.Mark(done)
	// Append v at [newbase + oldlen*8] — a scaled store with the old length
	// as the index, so no separate address computation is needed.
	b.e.LoadStack(RAX, stage)
	b.e.StoreScaled64(RAX, R10, R9, 8, 0)
	// Header: base = newbase, len = old len + 1.
	dst := b.slots[name]
	b.e.LoadStack(RAX, stage+24)
	b.e.StoreStack(RAX, dst)
	b.e.LoadStack(RAX, stage+16)
	b.e.AddRegImm32(RAX, 8)
	b.e.StoreStack(RAX, dst+8)
	return nil
}

func (b *Builder) emitForIn(n *parser.ForInStmt, fname string) error {
	if n.KeyName != "" {
		return fmt.Errorf("error[K145]: for-in over maps is not supported on the native target yet (int arrays only)")
	}
	base, ok := n.Iter.(*parser.Identifier)
	if !ok {
		return fmt.Errorf("error[K145]: for-in iterates a variable on the native target (got %T)", n.Iter)
	}
	off, ok := b.slots[base.Name]
	if !ok {
		return fmt.Errorf("error[K145]: undefined identifier '%s'", base.Name)
	}
	if b.kinds[base.Name] != KindArray {
		return fmt.Errorf("error[K145]: for-in iterates an array on the native target (got %s '%s')", kindName(b.kinds[base.Name]), base.Name)
	}
	eoff, ok := b.forIdx[n]
	if !ok {
		return fmt.Errorf("error[K145]: for-in index slot missing (layout bug)")
	}
	voff, ok := b.slots[n.VarName]
	if !ok {
		return fmt.Errorf("error[K145]: for-in variable '%s' has no slot (layout bug)", n.VarName)
	}
	b.e.XorRegReg(RAX)
	b.e.StoreStack(RAX, eoff)
	loopLbl := b.fresh("forin")
	endLbl := b.fresh("forinend")
	contLbl := b.fresh("forincont")
	b.e.Mark(loopLbl)
	b.e.LoadStack(RCX, eoff)
	// CmpMemReg computes [len] - i, so the loop continues while that is
	// strictly positive: the exit branch is jle (i >= len), NOT jge.
	b.e.CmpMemReg(RSP, off+8, RCX)
	b.e.Jle(endLbl)
	b.loops = append(b.loops, loopTgt{brk: endLbl, cont: contLbl})
	b.e.LoadStack(RBX, off)
	b.e.LoadScaled64(RAX, RBX, RCX, 8, 0)
	b.e.StoreStack(RAX, voff)
	if err := b.emitStmts(n.Body, fname); err != nil {
		b.loops = b.loops[:len(b.loops)-1]
		return err
	}
	b.loops = b.loops[:len(b.loops)-1]
	b.e.Mark(contLbl)
	b.e.IncMem(RSP, eoff)
	b.e.Jmp(loopLbl)
	b.e.Mark(endLbl)
	return nil
}



func (b *Builder) emitPrint(n *parser.PrintStmt) error {
	isStr, err := b.isStringExpr(n.Value)
	if err != nil {
		return err
	}
	if isStr {
		if err := b.emitStr(n.Value, 0); err != nil {
			return err
		}
		b.e.Call("print_str")
		return nil
	}
	// Phase 150A: a float prints through print_float (a %g-compatible
	// formatter). print_int would render the IEEE-754 bit pattern as a
	// decimal integer, so the dispatch must be by kind, not by arity.
	if k, err := b.exprKind(n.Value); err != nil {
		return err
	} else if k == KindFloat {
		if err := b.emitFloat(n.Value, 0); err != nil {
			return err
		}
		b.e.MovRegReg(RDI, RAX)
		b.e.Call("print_float")
		return nil
	}
	if err := b.emitExpr(n.Value, 0); err != nil {
		return err
	}
	b.e.MovRegReg(RDI, RAX)
	b.e.Call("print_int")
	return nil
}

func (b *Builder) emitReturn(n *parser.ReturnStmt, isMain bool) error {
	if n.Value == nil {
		b.e.XorRegReg(RAX)
		return nil
	}
	if isStr, err := b.isStringExpr(n.Value); err != nil {
		return err
	} else if isStr {
		// Phase 148: string-return convention is (RAX=ptr, RDX=len).
		// main keeps the int-only rule (exit codes are ints).
		if isMain {
			return fmt.Errorf("error[K145]: string return values are not supported (function must return int)")
		}
		if err := b.emitStr(n.Value, 0); err != nil {
			return err
		}
		b.e.MovRegReg(RAX, RDI)
		b.e.MovRegReg(RDX, RSI)
		return nil
	}
	// Phase 150A Step 1: a non-main float return already leaves the bits in
	// RAX (emitExpr -> emitFloat), which is the one-unit float return
	// convention; main keeps the int-only rule because its value becomes the
	// process exit code.
	if isMain {
		if k, err := b.exprKind(n.Value); err != nil {
			return err
		} else if k == KindFloat {
			return fmt.Errorf("error[K145]: floating-point return values are not supported (function must return int)")
		}
	}
	return b.emitExpr(n.Value, 0)
}

func (b *Builder) isStringExpr(n parser.Node) (bool, error) {
	k, err := b.exprKind(n)
	if err != nil {
		return false, err
	}
	return k == KindString, nil
}

// exprKind classifies a value expression (Phase 150A). Ints and floats
// are distinguished so float arithmetic never silently truncates; arrays
// are fat (ptr+len) values. Binary/unary operators inherit int (float
// operators are checked at emission); calls follow the callee's inferred
// return kind. Anything else is a loud K145.
func (b *Builder) exprKind(n parser.Node) (int, error) {
	switch x := n.(type) {
	case *parser.StringLiteral:
		return KindString, nil
	case *parser.IntLiteral:
		return KindInt, nil
	case *parser.Float64Literal:
		return KindFloat, nil
	case *parser.Identifier:
		k, ok := b.kinds[x.Name]
		if !ok {
			return KindInt, fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		return k, nil
	case *parser.BinaryExpr:
		// Phase 150A: arithmetic inherits float when either operand is
		// float (mixed int+float is rejected loudly at emission);
		// comparisons always yield int.
		switch x.Operator {
		case "+", "-", "*", "/", "%":
			lk, err := b.exprKind(x.Left)
			if err != nil {
				return KindInt, err
			}
			rk, err := b.exprKind(x.Right)
			if err != nil {
				return KindInt, err
			}
			// Phase 150B: an operator over two strings is a string operation
			// whichever it is, so emitStr owns the decision: `+` concatenates
			// and every other operator gets the precise
			// "unsupported string operator" message. Classifying only `+` here
			// would route `s - "c"` into the int path, which reports the far
			// less helpful "string 's' in int position".
			if lk == KindString && rk == KindString {
				return KindString, nil
			}
			if lk == KindFloat || rk == KindFloat {
				return KindFloat, nil
			}
			return KindInt, nil
		default:
			return KindInt, nil
		}
	case *parser.UnaryExpr:
		return b.exprKind(x.Operand)
	case *parser.CallExpr:
		// len(arr) classifies int; push(arr, v) classifies array. Both
		// are validated here rather than accepted blindly, so an
		// unsupported receiver is rejected by the classifier that every
		// other path already consults.
		if x.Function == "len" {
			if err := b.checkLenArgs(x); err != nil {
				return KindInt, err
			}
			return KindInt, nil
		}
		if x.Function == "push" {
			// Phase 150B: push allocates a new (base, len+1) array in the
			// arena, so it is a real array constructor now. Validated here
			// rather than blindly, so the receiver/value kinds are checked by
			// the classifier every other path already consults.
			if err := b.checkPushArgs(x); err != nil {
				return KindInt, err
			}
			return KindArray, nil
		}
		// Phase 148: kind follows the callee's inferred return kind
		// (unknown callees already fail loudly at emission).
		if k, ok := b.retKind[x.Function]; ok {
			return k, nil
		}
		return KindInt, nil
	case *parser.SliceExpr:
		// Phase 150B: s[a:b] over a string yields a string (a view into the
		// same bytes — no copy, and no allocation). Slicing an array or an
		// int stays a loud refusal: the element-size and bounds rules differ
		// and deserve their own executed goldens rather than a shared path.
		tk, err := b.exprKind(x.Target)
		if err != nil {
			return KindInt, err
		}
		if tk != KindString {
			return KindInt, fmt.Errorf("error[K145]: slice target must be a string (got %s)", kindName(tk))
		}
		return KindString, nil
	case *parser.ArrayLiteral:
		// Phase 150A: int-element literals only (element kinds are
		// checked at emission, where the diagnostic names the index).
		return KindArray, nil
	case *parser.IndexExpr:
		// int arrays only in 150A: the length lives in the header.
		kIdx, errIdx := b.exprKind(x.Left)
		if errIdx != nil {
			return KindInt, errIdx
		}
		if kIdx == KindArray {
			return KindInt, nil
		}
		return KindInt, fmt.Errorf("error[K145]: index target must be an array (got %s)", kindName(kIdx))
	default:
		return KindInt, fmt.Errorf("error[K145]: unsupported expression %T", n)
	}
}

func (b *Builder) emitExpr(n parser.Node, depth int) error {
	switch x := n.(type) {
	case *parser.IntLiteral:
		b.e.MovRegImm64(RAX, uint64(parseIntLit(x.Value)))
		return nil
	case *parser.Identifier:
		// Phase 150A Step 1: a float identifier carries its IEEE-754 bits
		// in its single slot; a string identifier is two units.
		if b.kinds[x.Name] == KindFloat {
			return b.emitFloat(x, depth)
		}
		if b.kinds[x.Name] != KindInt {
			return fmt.Errorf("error[K145]: %s '%s' in int position", kindName(b.kinds[x.Name]), x.Name)
		}
		off, ok := b.slots[x.Name]
		if !ok {
			return fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		b.e.LoadStack(RAX, off)
		return nil
	case *parser.Float64Literal:
		return b.emitFloat(x, depth)
	case *parser.BinaryExpr:
		return b.emitBinary(x, depth)
	case *parser.UnaryExpr:
		if x.Operator != "-" {
			return fmt.Errorf("error[K145]: unsupported unary operator '%s'", x.Operator)
		}
		k, err := b.exprKind(x.Operand)
		if err != nil {
			return err
		}
		if k == KindFloat {
			// Phase 150A: negation flips the sign bit instead of computing
			// 0.0 - x (see emitFloatNeg).
			if err := b.emitFloat(x.Operand, depth); err != nil {
				return err
			}
			b.emitFloatNeg()
			return nil
		}
		if err := b.emitExpr(x.Operand, depth); err != nil {
			return err
		}
		b.e.NegReg(RAX)
		return nil
	case *parser.CallExpr:
		if x.Function == "len" && x.Module == "" && !x.IsCFunc {
			return b.emitLen(x)
		}
		return b.emitCallValue(x, depth)
	case *parser.ArrayLiteral:
		return b.emitArrayLit(x)
	case *parser.IndexExpr:
		return b.emitArrayIndex(x, depth)
	default:
		return fmt.Errorf("error[K145]: unsupported expression %T", n)
	}
}

// emitArrayLit lowers an int array literal (Phase 150A).
//
// The array value is (base, len): base points at the element area
// reserved in the frame right after the header (see scanLets), len is
// the element count. Elements evaluate through the int path one at a
// time (RAX) and store through the scaled form with the base as SIB
// base; rsp never moves. Non-int elements are a loud K145 naming the
// index — never a silent truncation. The literal pattern is the only
// array constructor in 150A; push() returns a new array (see emitPush)
// because 150A arrays are fixed-footprint frame values.
func (b *Builder) emitArrayLit(x *parser.ArrayLiteral) error {
	return fmt.Errorf("error[K145]: array literal outside let (150A lowers literals at binding)")
}

// checkLenArgs validates the single-array receiver of the len builtin.
// Phase 150A: len() reads an array header length and nothing else — a
// string receiver (whose length is the second unit) and a wrong arity are
// loud K145 rather than a silent read of the wrong slot.
func (b *Builder) checkLenArgs(x *parser.CallExpr) error {
	if x.Module != "" || x.IsCFunc {
		return fmt.Errorf("error[K145]: module-qualified and C-interop calls are not supported (call to '%s')", x.Function)
	}
	if len(x.Args) != 1 {
		return fmt.Errorf("error[K145]: len() takes exactly 1 argument (got %d)", len(x.Args))
	}
	k, err := b.exprKind(x.Args[0])
	if err != nil {
		return err
	}
	if k != KindArray {
		return fmt.Errorf("error[K145]: len() requires an array argument (got %s)", kindName(k))
	}
	return nil
}

// emitLen lowers len(arr) to the header length, one load with rsp fixed.
func (b *Builder) emitLen(x *parser.CallExpr) error {
	if err := b.checkLenArgs(x); err != nil {
		return err
	}
	// Reuse the index lowering's addressing: an identifier receiver is the
	// only 150A shape, and its length slot is base+8.
	base, ok := x.Args[0].(*parser.Identifier)
	if !ok {
		return fmt.Errorf("error[K145]: len() requires an array variable (got %T)", x.Args[0])
	}
	off, ok := b.slots[base.Name]
	if !ok {
		return fmt.Errorf("error[K145]: undefined identifier '%s'", base.Name)
	}
	b.e.LoadStack(RAX, off+8)
	return nil
}

// emitArrayValue materializes the literal bound to name: the (base, len)
// header plus N int elements in the frame area reserved by scanLets. The
// binding site is the only place that owns the element stores.
//
// Frame contract (see scanLets): [off] = element-area base, [off+8] = element
// count, elements at [off+16 + i*8]. The base is stored in the frame and
// reloaded per element, so nothing depends on RBX surviving a nested
// expression evaluation, and rsp never moves.
func (b *Builder) emitArrayValue(name string, x *parser.ArrayLiteral) error {
	off := b.slots[name]
	b.e.LeaRegStack(RBX, off+16)
	b.e.StoreStack(RBX, off)
	b.e.MovRegImm64(RAX, uint64(len(x.Elements)))
	b.e.StoreStack(RAX, off+8)
	for i, el := range x.Elements {
		// Element kinds are validated up front so the diagnostic names the
		// offending index instead of failing deep inside the int path.
		k, err := b.exprKind(el)
		if err != nil {
			return err
		}
		if k != KindInt {
			return fmt.Errorf("error[K145]: array element %d must be int (got %s)", i, kindName(k))
		}
		if err := b.emitExpr(el, 0); err != nil {
			return err
		}
		// RAX holds the value; RCX becomes the element index; the base is
		// reloaded from the frame so the scaled store needs no assumption
		// about which registers the expression evaluation above clobbered.
		b.e.LoadStack(RBX, off)
		b.e.MovRegImm64(RCX, uint64(i))
		b.e.StoreScaled64(RAX, RBX, RCX, 8, 0)
	}
	return nil
}

// emitArrayIndex lowers arr[i] over int arrays (Phase 150A): bounds
// are checked against the header length with a loud K145-style Int3
// trap on violation (negative or past-the-end), matching the C
// backend's checked-index contract (exit 1 there; Int3 here because
// the native target has no stderr runtime diagnostic yet — 150B
// wires the message). rsp never moves; only RAX/RCX/RBX die.
func (b *Builder) emitArrayIndex(x *parser.IndexExpr, depth int) error {
	base, ok := x.Left.(*parser.Identifier)
	if !ok {
		return fmt.Errorf("error[K145]: array index target must be a variable (got %T)", x.Left)
	}
	off, ok := b.slots[base.Name]
	if !ok {
		return fmt.Errorf("error[K145]: undefined identifier '%s'", base.Name)
	}
	if b.kinds[base.Name] != KindArray {
		return fmt.Errorf("error[K145]: index target must be an array (got %s '%s')", kindName(b.kinds[base.Name]), base.Name)
	}
	if err := b.emitExpr(x.Index, depth+1); err != nil {
		return err
	}
	b.e.MovRegReg(RCX, RAX)
	b.e.LoadStack(RBX, off)
	b.e.LoadStack(RAX, off+8)
	// Bounds: 0 <= i < len, else Int3 (loud, never wraparound).
	b.e.TestRegReg(RCX, RCX)
	badLbl := b.fresh("idxbad")
	b.e.Jns(badLbl + "$neg")
	b.e.Mark(badLbl)
	b.e.Int3()
	b.e.Mark(badLbl + "$neg")
	b.e.CmpRegReg(RCX, RAX)
	b.e.Jge(badLbl)
	b.e.LoadScaled64(RAX, RBX, RCX, 8, 0)
	return nil
}

// emitFloat lowers a float64 expression, leaving the IEEE-754 bit pattern in
// RAX — the approved Phase-150A representation (float64 -> bits -> RAX), one
// 8-byte unit exactly like an int.
//
// Phase 150A: literals, float variables, the four arithmetic operators,
// unary minus and float-returning calls. The recursion re-enters the same
// depth-indexed scratch discipline the int path uses, so a nested float
// expression (1.5 + 2.5 * 0.5, -(1.0 / 4.0), f(x)) lowers with rsp fixed.
// Anything that cannot produce a float64 bit pattern is a loud K145 rather
// than a silent reinterpretation of the pattern as an integer.
func (b *Builder) emitFloat(n parser.Node, depth int) error {
	switch x := n.(type) {
	case *parser.Float64Literal:
		v, err := strconv.ParseFloat(x.Value, 64)
		if err != nil {
			return fmt.Errorf("error[K145]: invalid float literal '%s'", x.Value)
		}
		// The parsed pattern is materialized verbatim — no rounding and no
		// sign-bit normalization — so the bits of 0.0 and -0.0 stay
		// distinct (the parser lexes a leading '-' as TokenMinus, so a
		// negative literal arrives here without its sign; see the
		// unary-minus guard in emitExpr).
		b.e.MovRegImm64(RAX, math.Float64bits(v))
		return nil
	case *parser.Identifier:
		if b.kinds[x.Name] != KindFloat {
			return fmt.Errorf("error[K145]: %s '%s' in float position", kindName(b.kinds[x.Name]), x.Name)
		}
		off, ok := b.slots[x.Name]
		if !ok {
			return fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		// One unit, like the int path: the stored 64-bit pattern is
		// reloaded unchanged and rsp never moves.
		b.e.LoadStack(RAX, off)
		return nil
	case *parser.BinaryExpr:
		// Arithmetic inherits float; a mixed int+float pair was already
		// rejected by the caller that classified the expression, and the
		// kind check repeats here so a direct emitFloat entry is safe too.
		lk, err := b.exprKind(x.Left)
		if err != nil {
			return err
		}
		rk, err := b.exprKind(x.Right)
		if err != nil {
			return err
		}
		if lk != KindFloat || rk != KindFloat {
			return fmt.Errorf("error[K145]: mixed %s and %s in arithmetic (no implicit numeric conversion)", kindName(lk), kindName(rk))
		}
		return b.emitFloatBinary(x, depth)
	case *parser.UnaryExpr:
		if x.Operator != "-" {
			return fmt.Errorf("error[K145]: unsupported unary operator '%s'", x.Operator)
		}
		if err := b.emitFloat(x.Operand, depth); err != nil {
			return err
		}
		b.emitFloatNeg()
		return nil
	case *parser.CallExpr:
		// A float-returning call already leaves its f64 bits in RAX (the
		// one-unit return convention). retKindOf pinned the callee's
		// return kind, so an int callee reaching here is a kind error.
		if k, ok := b.retKind[x.Function]; !ok || k != KindFloat {
			return fmt.Errorf("error[K145]: %s call in float position (call to '%s')", kindName(b.exprKindOrInt(x)), x.Function)
		}
		return b.emitCallValue(x, depth)
	default:
		return fmt.Errorf("error[K145]: unsupported float expression %T", n)
	}
}

// emitFloatNeg flips the sign bit of the f64 pattern in RAX. XOR with 1<<63
// is exact for every finite value — both zeros and NaN payloads keep their
// magnitude — so -0.0 stays -0.0 where a 0.0 - x or an integer NegReg would
// silently destroy it. Shared by emitExpr's int/float dispatch and emitFloat's
// unary case so the two paths cannot drift.
func (b *Builder) emitFloatNeg() {
	b.e.MovRegImm64(RCX, 1<<63)
	b.e.MovXmmRegGp(XMM1, RCX)
	b.e.MovXmmRegGp(XMM0, RAX)
	b.e.XorpdXmmXmm(XMM0, XMM1)
	b.e.MovGpRegXmm(RAX, XMM0)
}

// exprKindOrInt is a diagnostic-only kind lookup: it never reports an error,
// so a K145 message can still name the operand kind it refused.
func (b *Builder) exprKindOrInt(n parser.Node) int {
	k, err := b.exprKind(n)
	if err != nil {
		return KindInt
	}
	return k
}

func (b *Builder) emitStr(n parser.Node, depth int) error {
	switch x := n.(type) {
	case *parser.StringLiteral:
		off := b.internRodata(x.Value)
		b.patches = append(b.patches, addrPatch{pos: b.e.imm64Patch(RDI), roOff: off})
		b.e.MovRegImm32(RSI, uint32(len(x.Value)))
		return nil
	case *parser.Identifier:
		if b.kinds[x.Name] != KindString {
			return fmt.Errorf("error[K145]: %s '%s' in string position", kindName(b.kinds[x.Name]), x.Name)
		}
		off, ok := b.slots[x.Name]
		if !ok {
			return fmt.Errorf("error[K145]: undefined identifier '%s'", x.Name)
		}
		b.e.LoadStack(RDI, off)
		b.e.LoadStack(RSI, off+8)
		return nil
	case *parser.CallExpr:
		// Phase 148: a string-returning call leaves (RAX=ptr, RDX=len);
		// move the pair into the (RDI, RSI) string-value convention.
		if err := b.emitCallValue(x, depth); err != nil {
			return err
		}
		b.e.MovRegReg(RDI, RAX)
		b.e.MovRegReg(RSI, RDX)
		return nil
	case *parser.SliceExpr:
		return b.emitStrSlice(x)
	case *parser.BinaryExpr:
		// Phase 150B: `a + b` on two strings allocates a result in the heap
		// arena and concatenates. Any other operator is a loud K145 — there
		// is no implicit int conversion, same rule as the numeric paths.
		if x.Operator != "+" {
			return fmt.Errorf("error[K145]: unsupported string operator '%s' (only + concatenates)", x.Operator)
		}
		return b.emitStrConcat(x, depth)
	default:
		return fmt.Errorf("error[K145]: unsupported string expression %T (literals, variables, concatenation and string calls only)", n)
	}
}

// emitStrSlice lowers `s[a:b]` over a string into a *view*: the result is
// (s.ptr + a, b - a), so no copy and no allocation happen. A missing `b`
// (open-ended `s[a:]`) means "to the end of the string".
//
// Bounds are 0 <= a <= b <= len, checked with a loud Int3 on violation —
// the same contract as the array index, rather than a silent clamp.
func (b *Builder) emitStrSlice(x *parser.SliceExpr) error {
	if err := b.emitStr(x.Target, 0); err != nil {
		return err
	}
	// RDI = base, RSI = len. Stage them, then evaluate the bounds.
	base := b.strTemp
	b.e.StoreStack(RDI, base)
	b.e.StoreStack(RSI, base+8)
	// start
	if x.Start == nil {
		b.e.XorRegReg(RAX)
	} else {
		if err := b.emitExpr(x.Start, 0); err != nil {
			return err
		}
	}
	b.e.StoreStack(RAX, base+16)
	// end: absent means len
	if x.End == nil {
		b.e.LoadStack(RAX, base+8)
	} else {
		if err := b.emitExpr(x.End, 0); err != nil {
			return err
		}
	}
	b.e.StoreStack(RAX, base+24)
	badLbl := b.fresh("sl$bad")
	chkLbl := b.fresh("sl$chk")
	// start < 0 -> bad
	b.e.LoadStack(RCX, base+16)
	b.e.TestRegReg(RCX, RCX)
	b.e.Jns(chkLbl)
	b.e.Mark(badLbl)
	b.e.Int3()
	// end > len -> bad
	b.e.Mark(chkLbl)
	b.e.LoadStack(RAX, base+24)
	b.e.CmpRegReg(RAX, RSI) // RSI is still the string length
	b.e.Ja(badLbl)
	// start > end -> bad
	b.e.CmpRegReg(RAX, RCX) // RCX is still the start
	b.e.Jl(badLbl)
	// Result: (base + start, end - start).
	b.e.LoadStack(RDI, base)
	b.e.AddRegReg(RDI, RCX)
	b.e.SubRegReg(RSI, RCX)
	return nil
}

// emitStrConcat lowers `left + right` over two strings: allocate
// len(left)+len(right) bytes, copy the left bytes then the right bytes, and
// return the pair in the (RDI, RSI) string convention.
//
// Both operands are evaluated and staged into depth-indexed frame scratch
// slots before the allocation, for the same reason the int path stages its
// operands (Phase 147): the second operand's evaluation must not disturb the
// first, and rsp never moves. The copy is byte-wise because the lengths are
// runtime values — a constant-offset load cannot address them. The second
// copy's destination is reached by advancing the destination *pointer* by the
// left length rather than by an indexed displacement, since the SIB disp is a
// compile-time field in every x86-64 memory form.
func (b *Builder) emitStrConcat(x *parser.BinaryExpr, depth int) error {
	lk, err := b.exprKind(x.Left)
	if err != nil {
		return err
	}
	rk, err := b.exprKind(x.Right)
	if err != nil {
		return err
	}
	if lk != KindString || rk != KindString {
		return fmt.Errorf("error[K145]: + needs two strings (got %s and %s)", kindName(lk), kindName(rk))
	}
	if depth >= maxBinDepth {
		return fmt.Errorf("error[K145]: expression nesting exceeds %d binary levels", maxBinDepth)
	}
	// The arena, the alloc helper and this path are all gated on the same
	// pre-pass, so reaching emission without one is an internal
	// inconsistency (a concat site the pre-pass failed to classify). Fail
	// with a real diagnostic rather than emitting a call to an undefined
	// label, which would panic at link time.
	if b.heapSize <= 0 {
		return fmt.Errorf("error[K145]: internal: string concatenation reached emission with no heap arena (the concat pre-pass missed this site)")
	}
	// Five staging units per depth: left ptr/len, right ptr/len, and the
	// allocated block. This is a dedicated area (strTemp), not the int
	// path's 8-byte-per-depth bin scratch, which would be overrun.
	base := b.strTemp + depth*5*8
	if err := b.emitStr(x.Left, depth+1); err != nil {
		return err
	}
	b.e.StoreStack(RDI, base)
	b.e.StoreStack(RSI, base+8)
	if err := b.emitStr(x.Right, depth+1); err != nil {
		return err
	}
	b.e.StoreStack(RDI, base+16)
	b.e.StoreStack(RSI, base+24)
	// total = llen + rlen, then alloc(total).
	b.e.LoadStack(RDI, base+8)
	b.e.LoadStack(RSI, base+24)
	b.e.AddRegReg(RDI, RSI)
	b.e.Call("alloc")
	b.e.StoreStack(RAX, base+32)
	// Left half: dst = block.
	b.e.LoadStack(R9, base+32)
	b.e.LoadStack(R8, base)
	b.e.LoadStack(RCX, base+8)
	b.emitByteCopy(R9, R8, RCX)
	// Right half: dst = block + llen, which is the run-time offset the SIB
	// disp field cannot carry.
	b.e.LoadStack(R9, base+32)
	b.e.LoadStack(R8, base+8)
	b.e.AddRegReg(R9, R8)
	b.e.LoadStack(R8, base+16)
	b.e.LoadStack(RCX, base+24)
	b.emitByteCopy(R9, R8, RCX)
	// Result: (RDI = block, RSI = total).
	b.e.LoadStack(RDI, base+32)
	b.e.LoadStack(RSI, base+8)
	b.e.AddRegReg(RSI, RCX)
	return nil
}

// emitByteCopy copies n bytes from [src] to [dst]. RDX is the index and R8
// the byte shuttle, so callers must not hold live values in either.
func (b *Builder) emitByteCopy(dst, src, n Reg) {
	loop := b.fresh("cp")
	done := b.fresh("cp$done")
	b.e.XorRegReg(RDX)
	b.e.Mark(loop)
	b.e.CmpRegReg(RDX, n)
	b.e.Jae(done)
	b.e.LoadScaled8(R8, src, RDX, 1, 0)
	b.e.StoreScaled8(R8, dst, RDX, 1, 0)
	b.e.IncReg(RDX)
	b.e.Jmp(loop)
	b.e.Mark(done)
}

func (b *Builder) emitBinary(x *parser.BinaryExpr, depth int) error {
	lk, err := b.exprKind(x.Left)
	if err != nil {
		return err
	}
	rk, err := b.exprKind(x.Right)
	if err != nil {
		return err
	}
	// Phase 150A: float arithmetic is a separate SSE-only path. A mixed
	// int+float operand pair is a loud refusal, never an implicit
	// conversion: pkg/codegen's binary_op promotes int to double, but
	// silently doing that here would make `1 + 2.0` type-check on one
	// engine and not the other. No implicit numeric conversions.
	if lk == KindFloat || rk == KindFloat {
		if lk != rk {
			return fmt.Errorf("error[K145]: mixed %s and %s in arithmetic (no implicit numeric conversion)", kindName(lk), kindName(rk))
		}
		return b.emitFloatBinary(x, depth)
	}
	if x.Operator != "+" && x.Operator != "-" && x.Operator != "*" {
		return fmt.Errorf("error[K145]: unsupported operator '%s' (want +, - or *)", x.Operator)
	}
	if depth >= maxBinDepth {
		return fmt.Errorf("error[K145]: expression nesting exceeds %d binary levels", maxBinDepth)
	}
	// Phase 147: stage the left operand through the depth-indexed scratch
	// slot. The old code pushed rax, which moved rsp and shifted every
	// frame-relative slot address — the right operand then re-read the
	// left's slot (`add(20,22)` yielded 40). rsp never moves now.
	if err := b.emitExpr(x.Left, depth+1); err != nil {
		return err
	}
	b.e.StoreStack(RAX, b.binTemp+depth*8)
	if err := b.emitExpr(x.Right, depth+1); err != nil {
		return err
	}
	b.e.MovRegReg(RCX, RAX)
	b.e.LoadStack(RAX, b.binTemp+depth*8)
	switch x.Operator {
	case "+":
		b.e.AddRegReg(RAX, RCX)
	case "-":
		b.e.SubRegReg(RAX, RCX)
	case "*":
		b.e.MulRegReg(RAX, RCX)
	}
	return nil
}

// emitFloatBinary lowers float64 + - * / and leaves the f64 bit pattern in
// RAX, the same one-unit result convention literals and variables use.
// Operands stage through the int path's depth-indexed scratch slot: an XMM
// value is a plain 64-bit pattern, so the existing GP store/load spills it
// and rsp never moves (Phase 147's rule). No register allocator and no
// conversions — each step is a single SSE2 scalar instruction.
func (b *Builder) emitFloatBinary(x *parser.BinaryExpr, depth int) error {
	if x.Operator != "+" && x.Operator != "-" && x.Operator != "*" && x.Operator != "/" {
		return fmt.Errorf("error[K145]: unsupported float operator '%s' (want +, -, * or /)", x.Operator)
	}
	if depth >= maxBinDepth {
		return fmt.Errorf("error[K145]: expression nesting exceeds %d binary levels", maxBinDepth)
	}
	if err := b.emitFloat(x.Left, depth+1); err != nil {
		return err
	}
	b.e.StoreStack(RAX, b.binTemp+depth*8)
	if err := b.emitFloat(x.Right, depth+1); err != nil {
		return err
	}
	b.e.MovXmmRegGp(XMM1, RAX) // right -> xmm1
	b.e.LoadStack(RAX, b.binTemp+depth*8)
	b.e.MovXmmRegGp(XMM0, RAX) // left -> xmm0
	switch x.Operator {
	case "+":
		b.e.AddsdXmmXmm(XMM0, XMM1)
	case "-":
		b.e.SubsdXmmXmm(XMM0, XMM1)
	case "*":
		b.e.MulsdXmmXmm(XMM0, XMM1)
	case "/":
		b.emitFloatDiv(XMM0, XMM1)
	}
	b.e.MovGpRegXmm(RAX, XMM0)
	return nil
}

// emitFloatDiv emits num = num / den with Karkain's zero-divisor rule.
// pkg/codegen's binary_op returns 0.0 rather than trapping
// (`r != 0.0 ? l / r : 0.0`), so a zero divisor is short-circuited to a
// positive zero instead of producing an infinity that would poison every
// later step. ucomisd reports ZF=1 for 0.0, -0.0 and NaN — all three
// compare unequal to zero in C, so all three take the short-circuit, which
// is exactly the `r != 0.0` guard the C backend writes.
func (b *Builder) emitFloatDiv(num, den XmmReg) {
	dividend := b.fresh("fdiv")
	done := b.fresh("fdivend")
	b.e.XorpdXmmXmm(XMM2, XMM2) // xmm2 = +0.0
	b.e.UcomisdXmmXmm(den, XMM2)
	b.e.Jnz(dividend)
	b.e.XorpdXmmXmm(num, num) // zero divisor -> +0.0
	b.e.Jmp(done)
	b.e.Mark(dividend)
	b.e.DivsdXmmXmm(num, den)
	b.e.Mark(done)
}

func (b *Builder) emitCallValue(x *parser.CallExpr, depth int) error {
	if x.Module != "" || x.IsCFunc {
		return fmt.Errorf("error[K145]: module-qualified and C-interop calls are not supported (call to '%s')", x.Function)
	}
	fd, ok := b.ftab[x.Function]
	if !ok {
		return fmt.Errorf("error[K145]: undefined function '%s'", x.Function)
	}
	// Phase 148: exact arity (previously unchecked — a short call read
	// uninitialized slots, a long one silently dropped arguments).
	if len(x.Args) != len(fd.Params) {
		return fmt.Errorf("error[K145]: call to '%s' has %d args (want %d)", x.Function, len(x.Args), len(fd.Params))
	}
	// Kind check + unit plan: ints and floats take one unit, strings
	// take two; units 0-5 ride argRegs, further units ride the extras
	// area via R10. Arrays are not passable in 150A (no annotation
	// syntax names them; 150B work) — a loud K145, never a miscompile.
	argKind := make([]int, len(x.Args))
	for i, a := range x.Args {
		got, err := b.exprKind(a)
		if err != nil {
			return err
		}
		if got == KindArray {
			return fmt.Errorf("error[K145]: array argument for parameter '%s' is not supported (call to '%s')", fd.Params[i], x.Function)
		}
		want := paramKind(fd, i)
		if got != want {
			return fmt.Errorf("error[K145]: %s argument for %s parameter '%s' (call to '%s')", kindName(got), kindName(want), fd.Params[i], x.Function)
		}
		argKind[i] = got
	}
	// Evaluate + stage: register units to the per-arg spill, extras units
	// to the caller-frame extras area. rsp never moves (147's rule).
	// Floats ride RAX as f64 bits, exactly like ints.
	unit := 0
	for i, a := range x.Args {
		if argKind[i] == KindString {
			if err := b.emitStr(a, depth); err != nil {
				return err
			}
			b.stageUnit(i, unit, RDI)
			b.stageUnit(i, unit+1, RSI)
			unit += 2
			continue
		}
		if argKind[i] == KindFloat {
			if err := b.emitFloat(a, depth); err != nil {
				return err
			}
		} else if err := b.emitExpr(a, depth); err != nil {
			return err
		}
		b.stageUnit(i, unit, RAX)
		unit++
	}
	// Load the register units back (extras are already home).
	unit = 0
	for i := range x.Args {
		units := kindUnits(argKind[i])
		for k := 0; k < units; k++ {
			if unit < len(argRegs) {
				b.e.LoadStack(argRegs[unit], b.argTemp(i)+k*8)
			}
			unit++
		}
	}
	if unit > len(argRegs) {
		b.e.LeaRegStack(R10, b.extrasBase)
	}
	if x.Function == "main" {
		b.e.Call("karkain_main")
	} else {
		b.e.Call("fn_" + x.Function)
	}
	return nil
}

// stageUnit homes one evaluated unit: unit < 6 goes to the per-arg spill
// (argTemp(i) is 16 bytes: k selects the low/high half), further units go
// to the caller-frame extras area at the callee-visible extras index.
func (b *Builder) stageUnit(arg, unit int, r Reg) {
	if unit < len(argRegs) {
		off := b.argTemp(arg)
		if r == RSI {
			off += 8
		}
		b.e.StoreStack(r, off)
		return
	}
	b.e.StoreStack(r, b.extrasBase+(unit-len(argRegs))*8)
}

func parseIntLit(s string) int64 {
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return 0
	}
	return v
}
