package codegen

import (
	"fmt"
	"strings"

	"karkain/pkg/parser"
	"karkain/pkg/target"
	"karkain/runtime/osmetrics"
)

// v1.2.19: OS metric/process builtins are gated by needsOS so TLS-free and
// metric-free programs (including the bootstrap compiler) keep their exact
// historical preamble and link flags.

func (g *Generator) scanNodeForOS(node parser.Node) {
	if node == nil || g.needsOS {
		return
	}
	switch n := node.(type) {
	case *parser.CallExpr:
		if isOSBuiltin(n.Function) {
			g.needsOS = true
			return
		}
		for _, a := range n.Args {
			g.scanNodeForOS(a)
		}
	case *parser.IndirectCallExpr:
		g.scanNodeForOS(n.Target)
		for _, a := range n.Args {
			g.scanNodeForOS(a)
		}
	case *parser.FuncDecl:
		for _, s := range n.Body {
			g.scanNodeForOS(s)
		}
	case *parser.IfStmt:
		g.scanNodeForOS(n.Condition)
		for _, s := range n.Consequence {
			g.scanNodeForOS(s)
		}
		for _, s := range n.Alternative {
			g.scanNodeForOS(s)
		}
	case *parser.WhileStmt:
		g.scanNodeForOS(n.Condition)
		for _, s := range n.Body {
			g.scanNodeForOS(s)
		}
	case *parser.ForStmt:
		g.scanNodeForOS(n.Init)
		g.scanNodeForOS(n.Condition)
		g.scanNodeForOS(n.Post)
		for _, s := range n.Body {
			g.scanNodeForOS(s)
		}
	case *parser.ForInStmt:
		g.scanNodeForOS(n.Iter)
		for _, s := range n.Body {
			g.scanNodeForOS(s)
		}
	case *parser.VarDeclStmt:
		g.scanNodeForOS(n.Value)
	case *parser.ReturnStmt:
		g.scanNodeForOS(n.Value)
	case *parser.ExprStmt:
		g.scanNodeForOS(n.Expression)
	case *parser.PrintStmt:
		g.scanNodeForOS(n.Value)
	case *parser.BinaryExpr:
		g.scanNodeForOS(n.Left)
		g.scanNodeForOS(n.Right)
	case *parser.UnaryExpr:
		g.scanNodeForOS(n.Operand)
	case *parser.LambdaExpr:
		for _, s := range n.Body {
			g.scanNodeForOS(s)
		}
	case *parser.MatchExpr:
		g.scanNodeForOS(n.Value)
		for _, arm := range n.Arms {
			g.scanNodeForOS(arm.Body)
		}
	case *parser.IndexExpr:
		g.scanNodeForOS(n.Left)
		g.scanNodeForOS(n.Index)
	case *parser.SliceExpr:
		g.scanNodeForOS(n.Target)
		g.scanNodeForOS(n.Start)
		g.scanNodeForOS(n.End)
	case *parser.DotExpr:
		g.scanNodeForOS(n.Left)
	case *parser.ArrayLiteral:
		for _, e := range n.Elements {
			g.scanNodeForOS(e)
		}
	case *parser.MapLiteral:
		for _, k := range n.Keys {
			g.scanNodeForOS(k)
		}
		for _, v := range n.Values {
			g.scanNodeForOS(v)
		}
	case *parser.StructLiteral:
		for _, v := range n.Fields {
			g.scanNodeForOS(v)
		}
	}
}

func isOSBuiltin(name string) bool {
	switch name {
	case "os_pid", "os_cpu_count", "os_total_mem_mb", "os_available_mem_mb",
		"os_uptime_ms", "os_process_list", "os_last_error":
		return true
	}
	return false
}

func stripOSInclude(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), `#include "karkain_osmetrics.h"`) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// osmetricsRuntimeC emits the compiler-neutral C23 OS runtime. The Go engine
// embeds the same bytes the self-hosted compiler emits from the generated
// osmetrics_runtime.kark.
func osmetricsRuntimeC() string {
	var b strings.Builder
	for _, f := range osmetrics.Files {
		b.WriteString(fmt.Sprintf("// ==== v1.2.19 runtime source: %s ====\n", f.Name))
		b.WriteString(stripOSInclude(f.Src))
		b.WriteString("\n")
	}
	return b.String()
}

// OSLibFlags returns the platform libraries required by the OS runtime. Both
// implementations are in kernel32/libc already, so this is deliberately empty;
// it exists as a named seam so a future platform change is explicit.
func OSLibFlags(_ target.OS) []string { return nil }
