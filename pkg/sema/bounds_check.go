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
func (r *Resolver) checkBoundsSafety(n *parser.IndexExpr) {
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
	if ident, ok := n.Left.(*parser.Identifier); ok {
		if size, ok := r.arraySizes[ident.Name]; ok {
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
func (r *Resolver) trackArraySize(name string, size int) {
	if r.arraySizes == nil {
		r.arraySizes = make(map[string]int)
	}
	r.arraySizes[name] = size
}