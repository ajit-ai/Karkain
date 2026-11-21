package sema

import (
	"fmt"
	"strconv"
	"karkain/pkg/parser"
)

// checkBoundsSafety performs compile-time bounds checking where provable.
// It rejects index expressions with constant out-of-bounds indices on
// array literals, string literals, and tracked array variables.
// Runtime checks remain for all other cases.
func (r *Resolver) checkBoundsSafety(n *parser.IndexExpr, callerName string) {
	if n == nil || n.Index == nil || n.Left == nil {
		return
	}

	// Only check constant integer indices.
	intLit, ok := n.Index.(*parser.IntLiteral)
	if !ok {
		return
	}
	idx, err := strconv.Atoi(intLit.Value)
	if err != nil {
		return
	}
	if idx < 0 {
		r.errors = append(r.errors, ResolveError{
			Line: n.Line,
			Col:  0,
			Msg:  fmt.Sprintf("negative array index %d", idx),
		})
		return
	}

	// Check against array literals.
	if arrLit, ok := n.Left.(*parser.ArrayLiteral); ok {
		if idx >= len(arrLit.Elements) {
			r.errors = append(r.errors, ResolveError{
				Line: n.Line,
				Col:  0,
				Msg:  fmt.Sprintf("array index %d out of range (length %d)", idx, len(arrLit.Elements)),
			})
		}
		return
	}

	// Check against string literals.
	if strLit, ok := n.Left.(*parser.StringLiteral); ok {
		if idx >= len(strLit.Value) {
			r.errors = append(r.errors, ResolveError{
				Line: n.Line,
				Col:  0,
				Msg:  fmt.Sprintf("string index %d out of range (length %d)", idx, len(strLit.Value)),
			})
		}
		return
	}

	// Check against tracked array variables.
	//
	// The table is keyed by FUNCTION as well as by name. A bare name key is not
	// sound: `Value s = [[], []]` in one function would otherwise report
	// `s[14]` in a different function -- where `s` is a parameter -- as
	// "array index 14 out of range (length 2)". That is exactly how the
	// Phase 1.3 checker rejected the compiler's own sources, so the fix is
	// recorded in docs/audit/PHASE-1.3-BOUNDS-CORRECTIVE.md rather than
	// silently tightened.
	if ident, ok := n.Left.(*parser.Identifier); ok {
		fnSizes := r.arraySizes[callerName]
		if size, ok := fnSizes[ident.Name]; ok {
			if idx >= size {
				r.errors = append(r.errors, ResolveError{
					Line: n.Line,
					Col:  0,
					Msg:  fmt.Sprintf("array index %d out of range (length %d)", idx, size),
				})
			}
		}
	}
}

// trackArraySize records the size of an array variable declared from a literal.
//
// `fn` is the enclosing function ("" for module-level statements): a size is
// only ever consulted inside the function that declared it, because a name is
// not a binding and two functions may use the same name for different things.
func (r *Resolver) trackArraySize(fn, name string, size int) {
	if r.arraySizes == nil {
		r.arraySizes = make(map[string]map[string]int)
	}
	fnSizes := r.arraySizes[fn]
	if fnSizes == nil {
		fnSizes = make(map[string]int)
		r.arraySizes[fn] = fnSizes
	}
	fnSizes[name] = size
}

// forgetArraySize drops a tracked size when the same name is re-declared with
// a value whose length is not provable. Without it a later `var s = f()` would
// inherit the length of an earlier `var s = [1, 2]` in the same function and
// report indices against the wrong array.
func (r *Resolver) forgetArraySize(fn, name string) {
	if fnSizes := r.arraySizes[fn]; fnSizes != nil {
		delete(fnSizes, name)
	}
}

// enterArrayScope snapshots the sizes tracked for `fn`. It exists because a
// declaration inside a block must not outlive it: `if c { let s = [1, 2] }`
// followed by `s[9]` refers to an OUTER `s` whose length is unknown, and a
// name-keyed table would report it against the inner literal.
func (r *Resolver) enterArrayScope(fn string) map[string]int {
	fnSizes := r.arraySizes[fn]
	if fnSizes == nil {
		return nil
	}
	cp := make(map[string]int, len(fnSizes))
	for k, v := range fnSizes {
		cp[k] = v
	}
	return cp
}

// leaveArrayScope restores the snapshot enterArrayScope took.
func (r *Resolver) leaveArrayScope(fn string, saved map[string]int) {
	if saved == nil {
		delete(r.arraySizes, fn)
		return
	}
	r.arraySizes[fn] = saved
}