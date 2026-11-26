package codegen

import (
	"fmt"
	"strings"

	"karkain/pkg/parser"
	"karkain/pkg/target"
	"karkain/runtime/arena"
)

// v1.2.25: arena builtins are gated by needsArena so arena-free programs
// (including the bootstrap compiler) keep their exact historical preamble.

func (g *Generator) scanNodeForArena(node parser.Node) {
	if node == nil || g.needsArena {
		return
	}
	switch n := node.(type) {
	case *parser.CallExpr:
		if isArenaBuiltin(n.Function) {
			g.needsArena = true
			return
		}
		for _, a := range n.Args {
			g.scanNodeForArena(a)
		}
	case *parser.IndirectCallExpr:
		g.scanNodeForArena(n.Target)
		for _, a := range n.Args {
			g.scanNodeForArena(a)
		}
	case *parser.FuncDecl:
		for _, s := range n.Body {
			g.scanNodeForArena(s)
		}
	case *parser.IfStmt:
		g.scanNodeForArena(n.Condition)
		for _, s := range n.Consequence {
			g.scanNodeForArena(s)
		}
		for _, s := range n.Alternative {
			g.scanNodeForArena(s)
		}
	case *parser.WhileStmt:
		g.scanNodeForArena(n.Condition)
		for _, s := range n.Body {
			g.scanNodeForArena(s)
		}
	case *parser.ForStmt:
		g.scanNodeForArena(n.Init)
		g.scanNodeForArena(n.Condition)
		g.scanNodeForArena(n.Post)
		for _, s := range n.Body {
			g.scanNodeForArena(s)
		}
	case *parser.ForInStmt:
		g.scanNodeForArena(n.Iter)
		for _, s := range n.Body {
			g.scanNodeForArena(s)
		}
	case *parser.VarDeclStmt:
		g.scanNodeForArena(n.Value)
	case *parser.ReturnStmt:
		g.scanNodeForArena(n.Value)
	case *parser.ExprStmt:
		g.scanNodeForArena(n.Expression)
	case *parser.PrintStmt:
		g.scanNodeForArena(n.Value)
	case *parser.BinaryExpr:
		g.scanNodeForArena(n.Left)
		g.scanNodeForArena(n.Right)
	case *parser.UnaryExpr:
		g.scanNodeForArena(n.Operand)
	case *parser.LambdaExpr:
		for _, s := range n.Body {
			g.scanNodeForArena(s)
		}
	case *parser.MatchExpr:
		g.scanNodeForArena(n.Value)
		for _, arm := range n.Arms {
			g.scanNodeForArena(arm.Body)
		}
	case *parser.IndexExpr:
		g.scanNodeForArena(n.Left)
		g.scanNodeForArena(n.Index)
	case *parser.SliceExpr:
		g.scanNodeForArena(n.Target)
		g.scanNodeForArena(n.Start)
		g.scanNodeForArena(n.End)
	case *parser.DotExpr:
		g.scanNodeForArena(n.Left)
	case *parser.ArrayLiteral:
		for _, e := range n.Elements {
			g.scanNodeForArena(e)
		}
	case *parser.MapLiteral:
		for _, k := range n.Keys {
			g.scanNodeForArena(k)
		}
		for _, v := range n.Values {
			g.scanNodeForArena(v)
		}
	case *parser.StructLiteral:
		for _, v := range n.Fields {
			g.scanNodeForArena(v)
		}
	}
}

func isArenaBuiltin(name string) bool {
	switch name {
	case "arena_new", "arena_alloc", "arena_reset", "arena_free",
		"arena_capacity", "arena_used":
		return true
	}
	return false
}

func stripArenaInclude(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), `#include "karkain_arena.h"`) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

func arenaRuntimeC() string {
	var b strings.Builder
	for _, f := range arena.Files {
		b.WriteString(fmt.Sprintf("// ==== v1.2.25 runtime source: %s ====\n", f.Name))
		b.WriteString(stripArenaInclude(f.Src))
		b.WriteString("\n")
	}
	return b.String()
}

func ArenaLibFlags(_ target.OS) []string { return nil }