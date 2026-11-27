// v1.2.26 S4b-1: trait/impl declaration validation (Go engine first slice).
//
// A trait declares required method signatures; an impl must provide every
// method with the same arity. This slice has no trait dispatch — impl methods
// are also appended as ordinary functions by the parser, so they compile and
// can be called directly. kcc parity (new AST nodes, parser and checker in
// src/compiler) is a separate slice.
package sema

import (
	"fmt"

	"karkain/pkg/parser"
)

func (r *Resolver) validateTraits(stmts []parser.Node) {
	traits := map[string]*parser.TraitDeclStmt{}
	var impls []*parser.ImplDeclStmt
	for _, s := range stmts {
		switch n := s.(type) {
		case *parser.TraitDeclStmt:
			if _, dup := traits[n.Name]; dup {
				r.errors = append(r.errors, ResolveError{
					Line: n.Line, Msg: fmt.Sprintf("duplicate trait '%s'", n.Name),
				})
				continue
			}
			traits[n.Name] = n
		case *parser.ImplDeclStmt:
			impls = append(impls, n)
		}
	}
	for _, impl := range impls {
		tr, ok := traits[impl.TraitName]
		if !ok {
			r.errors = append(r.errors, ResolveError{
				Line: impl.Line,
				Msg:  fmt.Sprintf("impl references unknown trait '%s'", impl.TraitName),
			})
			continue
		}
		have := map[string]int{}
		for _, m := range impl.Methods {
			have[m.Name] = len(m.Params)
		}
		for _, m := range tr.Methods {
			arity, found := have[m.Name]
			if !found {
				r.errors = append(r.errors, ResolveError{
					Line: impl.Line,
					Msg: fmt.Sprintf("type '%s' does not implement trait '%s': missing method '%s'",
						impl.ForType, impl.TraitName, m.Name),
				})
			} else if arity != len(m.Params) {
				r.errors = append(r.errors, ResolveError{
					Line: impl.Line,
					Msg: fmt.Sprintf("type '%s' method '%s' has %d parameters, trait '%s' requires %d",
						impl.ForType, m.Name, arity, impl.TraitName, len(m.Params)),
				})
			}
		}
	}
}
