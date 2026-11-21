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

	// Check against tracked array variables. The table is keyed by FUNCTION
	// as well as by name: a bare name key is not sound, because a `Value s =
	// [[], []]` in one function would otherwise report `s[14]` in a DIFFERENT
	// function -- where `s` is a parameter -- as out of range.
	//
	// A name whose size was forgotten by an assignment must stay unknown even
	// after a nested block restores its declaration snapshot. That is the
	// closure factory case: `fns = appendArray(fns, ...)` inside `while`.
	if ident, ok := n.Left.(*parser.Identifier); ok {
		if r.arrayForgotten[callerName][ident.Name] {
			return
		}
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
	// A fresh literal declaration is a new binding with a provable size. It
	// clears any earlier assignment invalidation for that name.
	if r.arrayForgotten == nil {
		r.arrayForgotten = make(map[string]map[string]bool)
	}
	if forgotten := r.arrayForgotten[fn]; forgotten != nil {
		delete(forgotten, name)
	}
}

// forgetArraySize drops a tracked size when the name is reassigned. Without
// it a later `var s = f()` would inherit the length of an earlier `var s =
// [1, 2]` in the same function and report indices against the wrong array.
// The invalidation is recorded separately so a block-scope restore cannot
// resurrect the old size.
func (r *Resolver) forgetArraySize(fn, name string) {
	if r.arrayForgotten == nil {
		r.arrayForgotten = make(map[string]map[string]bool)
	}
	forgotten := r.arrayForgotten[fn]
	if forgotten == nil {
		forgotten = make(map[string]bool)
		r.arrayForgotten[fn] = forgotten
	}
	forgotten[name] = true
	if fnSizes := r.arraySizes[fn]; fnSizes != nil {
		delete(fnSizes, name)
	}
}

// enterArrayScope snapshots the sizes tracked for `fn`. It exists because a
// declaration inside a block must not outlive it: `if c { let s = [1, 2] }`
// followed by `s[9]` refers to an OUTER `s` whose length is unknown, and a
// name-keyed table would report it against the inner literal.
func (r *Resolver) enterArrayScope(fn string) map[string]int {
	r.arrayScopeDepth[fn]++
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

// leaveArrayScope restores the snapshot enterArrayScope took, EXCEPT for
// names invalidated by assignment inside the scope. Assignment invalidation
// must outlive the block; declaration shadowing must not.
func (r *Resolver) leaveArrayScope(fn string, saved map[string]int) {
	if r.arrayScopeDepth[fn] > 0 {
		r.arrayScopeDepth[fn]--
	}
	if saved == nil {
		delete(r.arraySizes, fn)
		return
	}
	restored := make(map[string]int, len(saved))
	for k, v := range saved {
		if r.arrayForgotten[fn][k] {
			continue
		}
		restored[k] = v
	}
	r.arraySizes[fn] = restored
}
