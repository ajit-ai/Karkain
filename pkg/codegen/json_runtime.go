package codegen

import (
	"fmt"
	"strings"

	"karkain/pkg/parser"
	"karkain/pkg/target"
	"karkain/runtime/json"
)

// v1.2.20: the JSON DOM runtime is emitted only when a json_* builtin is
// used. Metric/TLS-free programs (including the bootstrap compiler) keep
// their exact historical preamble and link flags.

func isJSONBuiltin(name string) bool {
	switch name {
	case "json_parse", "json_type", "json_serialize", "json_get_path", "json_last_error":
		return true
	}
	return false
}

func (g *Generator) scanNodeForJSON(node parser.Node) {
	if node == nil || g.needsJSON {
		return
	}
	switch n := node.(type) {
	case *parser.CallExpr:
		if isJSONBuiltin(n.Function) {
			g.needsJSON = true
			return
		}
		for _, a := range n.Args {
			g.scanNodeForJSON(a)
		}
	case *parser.IndirectCallExpr:
		g.scanNodeForJSON(n.Target)
		for _, a := range n.Args {
			g.scanNodeForJSON(a)
		}
	case *parser.FuncDecl:
		for _, s := range n.Body {
			g.scanNodeForJSON(s)
		}
	case *parser.IfStmt:
		g.scanNodeForJSON(n.Condition)
		for _, s := range n.Consequence {
			g.scanNodeForJSON(s)
		}
		for _, s := range n.Alternative {
			g.scanNodeForJSON(s)
		}
	case *parser.WhileStmt:
		g.scanNodeForJSON(n.Condition)
		for _, s := range n.Body {
			g.scanNodeForJSON(s)
		}
	case *parser.ForStmt:
		g.scanNodeForJSON(n.Init)
		g.scanNodeForJSON(n.Condition)
		g.scanNodeForJSON(n.Post)
		for _, s := range n.Body {
			g.scanNodeForJSON(s)
		}
	case *parser.ForInStmt:
		g.scanNodeForJSON(n.Iter)
		for _, s := range n.Body {
			g.scanNodeForJSON(s)
		}
	case *parser.VarDeclStmt:
		g.scanNodeForJSON(n.Value)
	case *parser.ReturnStmt:
		g.scanNodeForJSON(n.Value)
	case *parser.ExprStmt:
		g.scanNodeForJSON(n.Expression)
	case *parser.PrintStmt:
		g.scanNodeForJSON(n.Value)
	case *parser.BinaryExpr:
		g.scanNodeForJSON(n.Left)
		g.scanNodeForJSON(n.Right)
	case *parser.UnaryExpr:
		g.scanNodeForJSON(n.Operand)
	case *parser.LambdaExpr:
		for _, s := range n.Body {
			g.scanNodeForJSON(s)
		}
	case *parser.MatchExpr:
		g.scanNodeForJSON(n.Value)
		for _, arm := range n.Arms {
			g.scanNodeForJSON(arm.Body)
		}
	case *parser.IndexExpr:
		g.scanNodeForJSON(n.Left)
		g.scanNodeForJSON(n.Index)
	case *parser.SliceExpr:
		g.scanNodeForJSON(n.Target)
		g.scanNodeForJSON(n.Start)
		g.scanNodeForJSON(n.End)
	case *parser.DotExpr:
		g.scanNodeForJSON(n.Left)
	case *parser.ArrayLiteral:
		for _, e := range n.Elements {
			g.scanNodeForJSON(e)
		}
	case *parser.MapLiteral:
		for _, k := range n.Keys {
			g.scanNodeForJSON(k)
		}
		for _, v := range n.Values {
			g.scanNodeForJSON(v)
		}
	case *parser.StructLiteral:
		for _, v := range n.Fields {
			g.scanNodeForJSON(v)
		}
	}
}

func stripJSONInclude(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), `#include "karkain_json.h"`) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// jsonRuntimeC emits the compiler-neutral C23 JSON DOM runtime. The Go engine
// embeds the same bytes the self-hosted compiler emits from
// json_runtime.kark.
func jsonRuntimeC() string {
	var b strings.Builder
	for _, f := range json.Files {
		b.WriteString(fmt.Sprintf("// ==== v1.2.20 runtime source: %s ====\n", f.Name))
		b.WriteString(stripJSONInclude(f.Src))
		b.WriteString("\n")
	}
	return b.String()
}

// JSONLibFlags is deliberately empty: json parsing needs only libc helpers
// already present in every generated-C link path.
func JSONLibFlags(_ target.OS) []string { return nil }
