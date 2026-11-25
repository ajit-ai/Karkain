package codegen

import (
	"fmt"
	"strings"

	"karkain/pkg/parser"
	"karkain/pkg/target"
	"karkain/runtime/tls"
)

// v1.2.17: https_request is dispatched like the net builtins. It is gated by
// needsTLS so programs that do not use it never reference OpenSSL/WinHTTP
// symbols and therefore never require the TLS link libraries.

func (g *Generator) scanNodeForTLS(node parser.Node) {
	if node == nil || g.needsTLS {
		return
	}
	switch n := node.(type) {
	case *parser.CallExpr:
		if n.Function == "https_request" {
			g.needsTLS = true
			return
		}
		for _, a := range n.Args {
			g.scanNodeForTLS(a)
		}
	case *parser.IndirectCallExpr:
		g.scanNodeForTLS(n.Target)
		for _, a := range n.Args {
			g.scanNodeForTLS(a)
		}
	case *parser.FuncDecl:
		for _, s := range n.Body {
			g.scanNodeForTLS(s)
		}
	case *parser.IfStmt:
		g.scanNodeForTLS(n.Condition)
		for _, s := range n.Consequence {
			g.scanNodeForTLS(s)
		}
		for _, s := range n.Alternative {
			g.scanNodeForTLS(s)
		}
	case *parser.WhileStmt:
		g.scanNodeForTLS(n.Condition)
		for _, s := range n.Body {
			g.scanNodeForTLS(s)
		}
	case *parser.ForStmt:
		g.scanNodeForTLS(n.Init)
		g.scanNodeForTLS(n.Condition)
		g.scanNodeForTLS(n.Post)
		for _, s := range n.Body {
			g.scanNodeForTLS(s)
		}
	case *parser.ForInStmt:
		g.scanNodeForTLS(n.Iter)
		for _, s := range n.Body {
			g.scanNodeForTLS(s)
		}
	case *parser.VarDeclStmt:
		g.scanNodeForTLS(n.Value)
	case *parser.ReturnStmt:
		g.scanNodeForTLS(n.Value)
	case *parser.ExprStmt:
		g.scanNodeForTLS(n.Expression)
	case *parser.PrintStmt:
		g.scanNodeForTLS(n.Value)
	case *parser.BinaryExpr:
		g.scanNodeForTLS(n.Left)
		g.scanNodeForTLS(n.Right)
	case *parser.UnaryExpr:
		g.scanNodeForTLS(n.Operand)
	case *parser.LambdaExpr:
		for _, s := range n.Body {
			g.scanNodeForTLS(s)
		}
	case *parser.MatchExpr:
		g.scanNodeForTLS(n.Value)
		for _, arm := range n.Arms {
			g.scanNodeForTLS(arm.Body)
		}
	case *parser.IndexExpr:
		g.scanNodeForTLS(n.Left)
		g.scanNodeForTLS(n.Index)
	case *parser.SliceExpr:
		g.scanNodeForTLS(n.Target)
		g.scanNodeForTLS(n.Start)
		g.scanNodeForTLS(n.End)
	case *parser.DotExpr:
		g.scanNodeForTLS(n.Left)
	case *parser.ArrayLiteral:
		for _, e := range n.Elements {
			g.scanNodeForTLS(e)
		}
	case *parser.MapLiteral:
		for _, k := range n.Keys {
			g.scanNodeForTLS(k)
		}
		for _, v := range n.Values {
			g.scanNodeForTLS(v)
		}
	case *parser.StructLiteral:
		for _, v := range n.Fields {
			g.scanNodeForTLS(v)
		}
	}
}

// stripTlsInclude removes the runtime's self-include so the single generated
// translation unit defines everything exactly once.
func stripTlsInclude(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), `#include "karkain_tls.h"`) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// tlsRuntimeC emits the compiler-neutral C23 TLS runtime. The Go engine embeds
// the same bytes the self-hosted compiler emits from tls_runtime.kark.
func tlsRuntimeC() string {
	var b strings.Builder
	for _, f := range tls.Files {
		b.WriteString(fmt.Sprintf("// ==== v1.2.17 runtime source: %s ====\n", f.Name))
		b.WriteString(stripTlsInclude(f.Src))
		b.WriteString("\n")
	}
	return b.String()
}

// TLSLibFlags returns the platform libraries required by the TLS runtime for
// the given target OS. They are added to every generated-C link path that can
// produce a program using https_request. Schannel is reached through WinHTTP
// on Windows; POSIX uses OpenSSL.
func TLSLibFlags(os target.OS) []string {
	if os == target.OSWindows {
		return []string{"-lwinhttp"}
	}
	return []string{"-lssl", "-lcrypto"}
}
