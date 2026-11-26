package codegen

import (
	"fmt"
	"strings"

	"karkain/pkg/parser"
	"karkain/pkg/target"
	"karkain/runtime/rc"
)

// v1.2.24: RC builtins are gated by needsRC so RC-free programs (including the
// bootstrap compiler) keep their exact historical preamble and link flags.

func (g *Generator) scanNodeForRC(node parser.Node) {
	if node == nil || g.needsRC {
		return
	}
	switch n := node.(type) {
	case *parser.CallExpr:
		if isRCBuiltin(n.Function) {
			g.needsRC = true
			return
		}
		for _, a := range n.Args {
			g.scanNodeForRC(a)
		}
	case *parser.IndirectCallExpr:
		g.scanNodeForRC(n.Target)
		for _, a := range n.Args {
			g.scanNodeForRC(a)
		}
	case *parser.FuncDecl:
		for _, s := range n.Body {
			g.scanNodeForRC(s)
		}
	case *parser.IfStmt:
		g.scanNodeForRC(n.Condition)
		for _, s := range n.Consequence {
			g.scanNodeForRC(s)
		}
		for _, s := range n.Alternative {
			g.scanNodeForRC(s)
		}
	case *parser.WhileStmt:
		g.scanNodeForRC(n.Condition)
		for _, s := range n.Body {
			g.scanNodeForRC(s)
		}
	case *parser.ForStmt:
		g.scanNodeForRC(n.Init)
		g.scanNodeForRC(n.Condition)
		g.scanNodeForRC(n.Post)
		for _, s := range n.Body {
			g.scanNodeForRC(s)
		}
	case *parser.ForInStmt:
		g.scanNodeForRC(n.Iter)
		for _, s := range n.Body {
			g.scanNodeForRC(s)
		}
	case *parser.VarDeclStmt:
		g.scanNodeForRC(n.Value)
	case *parser.ReturnStmt:
		g.scanNodeForRC(n.Value)
	case *parser.ExprStmt:
		g.scanNodeForRC(n.Expression)
	case *parser.PrintStmt:
		g.scanNodeForRC(n.Value)
	case *parser.BinaryExpr:
		g.scanNodeForRC(n.Left)
		g.scanNodeForRC(n.Right)
	case *parser.UnaryExpr:
		g.scanNodeForRC(n.Operand)
	case *parser.LambdaExpr:
		for _, s := range n.Body {
			g.scanNodeForRC(s)
		}
	case *parser.MatchExpr:
		g.scanNodeForRC(n.Value)
		for _, arm := range n.Arms {
			g.scanNodeForRC(arm.Body)
		}
	case *parser.IndexExpr:
		g.scanNodeForRC(n.Left)
		g.scanNodeForRC(n.Index)
	case *parser.SliceExpr:
		g.scanNodeForRC(n.Target)
		g.scanNodeForRC(n.Start)
		g.scanNodeForRC(n.End)
	case *parser.DotExpr:
		g.scanNodeForRC(n.Left)
	case *parser.ArrayLiteral:
		for _, e := range n.Elements {
			g.scanNodeForRC(e)
		}
	case *parser.MapLiteral:
		for _, k := range n.Keys {
			g.scanNodeForRC(k)
		}
		for _, v := range n.Values {
			g.scanNodeForRC(v)
		}
	case *parser.StructLiteral:
		for _, v := range n.Fields {
			g.scanNodeForRC(v)
		}
	}
}

func isRCBuiltin(name string) bool {
	switch name {
	case "rc_new", "rc_retain", "rc_release", "rc_strong_count", "rc_payload":
		return true
	}
	return false
}

func stripRCInclude(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), `#include "karkain_rc.h"`) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

func rcRuntimeC() string {
	var b strings.Builder
	for _, f := range rc.Files {
		b.WriteString(fmt.Sprintf("// ==== v1.2.24 runtime source: %s ====\n", f.Name))
		b.WriteString(stripRCInclude(f.Src))
		b.WriteString("\n")
	}
	return b.String()
}

func RCLibFlags(_ target.OS) []string { return nil }