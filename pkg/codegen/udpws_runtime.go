package codegen

import (
	"fmt"
	"strings"

	"karkain/pkg/parser"
	"karkain/pkg/target"
	"karkain/runtime/udpws"
)

// v1.2.21: UDP/WebSocket runtime is emitted only when a udp_* or ws_* builtin
// is used. Metric/TLS/JSON-free programs keep their historical preamble.

func isUDPWSBuiltin(name string) bool {
	switch name {
	case "udp_send", "udp_send_on", "udp_bind", "udp_recv", "udp_last_error",
		"ws_accept", "ws_send_text", "ws_recv_text", "ws_last_error":
		return true
	}
	return false
}

func (g *Generator) scanNodeForUDPWS(node parser.Node) {
	if node == nil || g.needsUDPWS {
		return
	}
	switch n := node.(type) {
	case *parser.CallExpr:
		if isUDPWSBuiltin(n.Function) {
			g.needsUDPWS = true
			return
		}
		for _, a := range n.Args {
			g.scanNodeForUDPWS(a)
		}
	case *parser.IndirectCallExpr:
		g.scanNodeForUDPWS(n.Target)
		for _, a := range n.Args {
			g.scanNodeForUDPWS(a)
		}
	case *parser.FuncDecl:
		for _, s := range n.Body {
			g.scanNodeForUDPWS(s)
		}
	case *parser.IfStmt:
		g.scanNodeForUDPWS(n.Condition)
		for _, s := range n.Consequence {
			g.scanNodeForUDPWS(s)
		}
		for _, s := range n.Alternative {
			g.scanNodeForUDPWS(s)
		}
	case *parser.WhileStmt:
		g.scanNodeForUDPWS(n.Condition)
		for _, s := range n.Body {
			g.scanNodeForUDPWS(s)
		}
	case *parser.ForStmt:
		g.scanNodeForUDPWS(n.Init)
		g.scanNodeForUDPWS(n.Condition)
		g.scanNodeForUDPWS(n.Post)
		for _, s := range n.Body {
			g.scanNodeForUDPWS(s)
		}
	case *parser.ForInStmt:
		g.scanNodeForUDPWS(n.Iter)
		for _, s := range n.Body {
			g.scanNodeForUDPWS(s)
		}
	case *parser.VarDeclStmt:
		g.scanNodeForUDPWS(n.Value)
	case *parser.ReturnStmt:
		g.scanNodeForUDPWS(n.Value)
	case *parser.ExprStmt:
		g.scanNodeForUDPWS(n.Expression)
	case *parser.PrintStmt:
		g.scanNodeForUDPWS(n.Value)
	case *parser.BinaryExpr:
		g.scanNodeForUDPWS(n.Left)
		g.scanNodeForUDPWS(n.Right)
	case *parser.UnaryExpr:
		g.scanNodeForUDPWS(n.Operand)
	case *parser.LambdaExpr:
		for _, s := range n.Body {
			g.scanNodeForUDPWS(s)
		}
	case *parser.MatchExpr:
		g.scanNodeForUDPWS(n.Value)
		for _, arm := range n.Arms {
			g.scanNodeForUDPWS(arm.Body)
		}
	case *parser.IndexExpr:
		g.scanNodeForUDPWS(n.Left)
		g.scanNodeForUDPWS(n.Index)
	case *parser.SliceExpr:
		g.scanNodeForUDPWS(n.Target)
		g.scanNodeForUDPWS(n.Start)
		g.scanNodeForUDPWS(n.End)
	case *parser.DotExpr:
		g.scanNodeForUDPWS(n.Left)
	case *parser.ArrayLiteral:
		for _, e := range n.Elements {
			g.scanNodeForUDPWS(e)
		}
	case *parser.MapLiteral:
		for _, k := range n.Keys {
			g.scanNodeForUDPWS(k)
		}
		for _, v := range n.Values {
			g.scanNodeForUDPWS(v)
		}
	case *parser.StructLiteral:
		for _, v := range n.Fields {
			g.scanNodeForUDPWS(v)
		}
	}
}

func stripUDPWSInclude(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), `#include "karkain_udpws.h"`) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// udpwsRuntimeC emits the compiler-neutral C23 UDP/WebSocket runtime.
func udpwsRuntimeC() string {
	var b strings.Builder
	for _, f := range udpws.Files {
		b.WriteString(fmt.Sprintf("// ==== v1.2.21 runtime source: %s ====\n", f.Name))
		b.WriteString(stripUDPWSInclude(f.Src))
		b.WriteString("\n")
	}
	return b.String()
}

// UDPWSLibFlags is empty: Winsock is already linked via -lws2_32 and POSIX
// sockets are libc.
func UDPWSLibFlags(_ target.OS) []string { return nil }
