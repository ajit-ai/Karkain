package wasm

import (
	"fmt"

	"karkain/pkg/parser"
)

// Struct (composite) values on the wasm32-wasi backend, Phase 139.
//
// Representation (honest MVP): a struct with N fields is an N-element boxed
// cell in linear memory — the exact container the backend already uses for
// arrays (rt.MakeArray / rt.Set / rt.Get). Field names resolve to cell
// indices from the struct declaration; the physical layout is therefore
// deterministic and wasmtime-observable, with no WASM-GC-proposal opcodes
// involved. Real struct.new/get/set ISA lowering is future work (documented
// boundary); the K108 gate names it if asked about.
//
// Name resolution is declaration-directed, not flow-directed: a field name
// must identify exactly one cell index across every struct declared in the
// program. Unknown or ambiguous fields are deterministic K108 errors at scan
// time, never miscompiles. Method calls are untouched (still K108 — the
// parser carries no receiver node, so they never reach DotExpr handling).

// collectStructLayouts maps each declared struct name to its ordered field
// names. Duplicate struct names resolve last-wins (the sema checker rejects
// duplicates with K107 on real CLI paths long before the backend runs).
func collectStructLayouts(prog *parser.Program) map[string][]string {
	layouts := map[string][]string{}
	for _, stmt := range prog.Statements {
		decl, ok := stmt.(*parser.StructDeclStmt)
		if !ok {
			continue
		}
		fields := make([]string, 0, len(decl.Fields))
		for _, f := range decl.Fields {
			fields = append(fields, f.Name)
		}
		layouts[decl.Name] = fields
	}
	return layouts
}

// fieldIndex resolves a field name to its cell index. It errors when the
// field is declared in no struct (unknown) or at different indices in
// different structs (ambiguous) — both are scan-time K108 diagnostics.
func fieldIndex(layouts map[string][]string, field string) (int, error) {
	idx := -1
	for _, fields := range layouts {
		for i, name := range fields {
			if name != field {
				continue
			}
			if idx >= 0 && idx != i {
				return 0, fmt.Errorf("ambiguous field '%s'", field)
			}
			idx = i
		}
	}
	if idx < 0 {
		return 0, fmt.Errorf("unknown field '%s'", field)
	}
	return idx, nil
}

// isStructTypeName reports whether a type annotation names a declared struct
// (annotations for int/string/array/bool/"" are handled by the caller).
func isStructTypeName(layouts map[string][]string, t string) bool {
	_, ok := layouts[t]
	return ok
}

// checkStructLiteral validates a struct literal against its declaration:
// the type must be declared and the literal must supply exactly the declared
// field set (order is free — codegen places values by declared index).
// Entry shapes are BinaryExpr with an Identifier field name on the left.
func checkStructLiteral(layouts map[string][]string, lit *parser.StructLiteral) error {
	fields, ok := layouts[lit.TypeName]
	if !ok {
		return fmt.Errorf("unknown struct '%s'", lit.TypeName)
	}
	if len(lit.Fields) != len(fields) {
		return fmt.Errorf("struct '%s' field mismatch", lit.TypeName)
	}
	seen := map[string]bool{}
	for _, entry := range lit.Fields {
		be, ok := entry.(*parser.BinaryExpr)
		if !ok || be.Right == nil {
			return fmt.Errorf("struct '%s' field mismatch", lit.TypeName)
		}
		id, ok := be.Left.(*parser.Identifier)
		if !ok {
			return fmt.Errorf("struct '%s' field mismatch", lit.TypeName)
		}
		known := false
		for _, name := range fields {
			if name == id.Name {
				known = true
			}
		}
		if !known || seen[id.Name] {
			return fmt.Errorf("struct '%s' field mismatch", lit.TypeName)
		}
		seen[id.Name] = true
	}
	return nil
}

// structFieldIndex resolves a DotExpr field at codegen time. The scan
// guarantees resolution, so failure here is an unreachable backend bug
// (the file's panic style for impossible states).
func (g *gen) structFieldIndex(x *parser.DotExpr) int {
	idx, err := fieldIndex(g.layouts, x.Right)
	if err != nil {
		panic(fmt.Sprintf("wasm backend: struct field unresolvable at codegen: %v", err))
	}
	return idx
}

// genStructLiteral materializes `Type { f: v, ... }` as a boxed cell with
// values placed by DECLARED field index (literal order is free). Mirrors the
// ArrayLiteral arm exactly: MakeArray + Set per slot, cell left on the stack.
func (g *gen) genStructLiteral(x *parser.StructLiteral) {
	vals := structLiteralValues(g.layouts, x)
	g.e.I32Const(len(vals))
	g.e.Call(g.rt.MakeArray)
	tmp := g.allocTemp()
	g.e.LocalSet(tmp)
	for i, v := range vals {
		g.e.LocalGet(tmp)
		g.e.I64Const(int64(i) << 1)
		g.genExpr(v)
		g.e.I32Const(g.ko.filePtr)
		g.e.I32Const(g.ko.fileLen)
		g.e.I32Const(x.Line)
		g.e.Call(g.rt.Set)
		g.e.Drop()
	}
	g.e.LocalGet(tmp)
}

// genFieldGet emits `p.field` as a cell load at the declaration-directed
// index (scan-validated). Mirrors the IndexExpr arm.
func (g *gen) genFieldGet(x *parser.DotExpr) {
	g.genExpr(x.Left)
	g.e.I64Const(int64(g.structFieldIndex(x)) << 1)
	g.e.I32Const(g.ko.filePtr)
	g.e.I32Const(g.ko.fileLen)
	g.e.I32Const(x.Line)
	g.e.Call(g.rt.Get)
}

// structLiteralValues returns the literal's field values ordered by declared
// index. The caller must have run checkStructLiteral first (scan time always
// precedes codegen); an entry that cannot be placed is a backend bug.
func structLiteralValues(layouts map[string][]string, lit *parser.StructLiteral) []parser.Node {
	fields := layouts[lit.TypeName]
	byName := map[string]parser.Node{}
	for _, entry := range lit.Fields {
		be := entry.(*parser.BinaryExpr)
		byName[be.Left.(*parser.Identifier).Name] = be.Right
	}
	vals := make([]parser.Node, len(fields))
	for i, name := range fields {
		vals[i] = byName[name]
	}
	return vals
}
