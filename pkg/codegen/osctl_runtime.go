package codegen

import (
	"fmt"
	"strings"

	"karkain/pkg/parser"
	"karkain/pkg/target"
	"karkain/runtime/osctl"
)

// v1.2.34: Portable OS/process control builtins — env_get/set/unset/args,
// fs_exists/is_dir/stat_size/stat_mtime/mkdir/remove, proc_spawn/kill/wait.
// Gated by needsOSCtl so programs that use none of these keep their exact
// historical preamble and link flags.

func (g *Generator) scanNodeForOSCtl(node parser.Node) {
	if node == nil || g.needsOSCtl {
		return
	}
	switch n := node.(type) {
	case *parser.CallExpr:
		if isOSCtlBuiltin(n.Function) {
			g.needsOSCtl = true
			return
		}
		for _, a := range n.Args {
			g.scanNodeForOSCtl(a)
		}
	case *parser.IndirectCallExpr:
		g.scanNodeForOSCtl(n.Target)
		for _, a := range n.Args {
			g.scanNodeForOSCtl(a)
		}
	case *parser.FuncDecl:
		for _, s := range n.Body {
			g.scanNodeForOSCtl(s)
		}
	case *parser.IfStmt:
		g.scanNodeForOSCtl(n.Condition)
		for _, s := range n.Consequence {
			g.scanNodeForOSCtl(s)
		}
		for _, s := range n.Alternative {
			g.scanNodeForOSCtl(s)
		}
	case *parser.WhileStmt:
		g.scanNodeForOSCtl(n.Condition)
		for _, s := range n.Body {
			g.scanNodeForOSCtl(s)
		}
	case *parser.ForStmt:
		g.scanNodeForOSCtl(n.Init)
		g.scanNodeForOSCtl(n.Condition)
		g.scanNodeForOSCtl(n.Post)
		for _, s := range n.Body {
			g.scanNodeForOSCtl(s)
		}
	case *parser.ForInStmt:
		g.scanNodeForOSCtl(n.Iter)
		for _, s := range n.Body {
			g.scanNodeForOSCtl(s)
		}
	case *parser.VarDeclStmt:
		g.scanNodeForOSCtl(n.Value)
	case *parser.ReturnStmt:
		g.scanNodeForOSCtl(n.Value)
	case *parser.ExprStmt:
		g.scanNodeForOSCtl(n.Expression)
	case *parser.PrintStmt:
		g.scanNodeForOSCtl(n.Value)
	case *parser.BinaryExpr:
		g.scanNodeForOSCtl(n.Left)
		g.scanNodeForOSCtl(n.Right)
	case *parser.UnaryExpr:
		g.scanNodeForOSCtl(n.Operand)
	case *parser.LambdaExpr:
		for _, s := range n.Body {
			g.scanNodeForOSCtl(s)
		}
	case *parser.MatchExpr:
		g.scanNodeForOSCtl(n.Value)
		for _, arm := range n.Arms {
			g.scanNodeForOSCtl(arm.Body)
		}
	case *parser.IndexExpr:
		g.scanNodeForOSCtl(n.Left)
		g.scanNodeForOSCtl(n.Index)
	case *parser.SliceExpr:
		g.scanNodeForOSCtl(n.Target)
		g.scanNodeForOSCtl(n.Start)
		g.scanNodeForOSCtl(n.End)
	case *parser.DotExpr:
		g.scanNodeForOSCtl(n.Left)
	case *parser.ArrayLiteral:
		for _, e := range n.Elements {
			g.scanNodeForOSCtl(e)
		}
	case *parser.MapLiteral:
		for _, k := range n.Keys {
			g.scanNodeForOSCtl(k)
		}
		for _, v := range n.Values {
			g.scanNodeForOSCtl(v)
		}
	case *parser.StructLiteral:
		for _, v := range n.Fields {
			g.scanNodeForOSCtl(v)
		}
	}
}

// isOSCtlBuiltin returns true for the 12 portable OS/process control builtins
// introduced by v1.2.34.
func isOSCtlBuiltin(name string) bool {
	switch name {
	case "env_get", "env_set", "env_unset", "env_args",
		"fs_exists", "fs_is_dir", "fs_stat_size", "fs_stat_mtime",
		"fs_mkdir", "fs_remove",
		"proc_spawn", "proc_kill", "proc_wait",
		// v1.2.34 integration: stdlib wrappers call internal osctl_* names
		// so they cannot shadow themselves. The prescan must recognise those.
		"osctl_env_get", "osctl_env_set", "osctl_env_unset", "osctl_env_args",
		"osctl_fs_exists", "osctl_fs_is_dir", "osctl_fs_stat_size", "osctl_fs_stat_mtime",
		"osctl_fs_mkdir", "osctl_fs_remove",
		"osctl_proc_spawn", "osctl_proc_kill", "osctl_proc_wait",
		"osctl_sig_ignore", "osctl_sig_default", "osctl_sig_raise":
		return true
	}
	return false
}

func stripOSCtlInclude(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), `#include "karkain_osctl.h"`) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// osctlRuntimeC emits the compiler-neutral C23 OS control runtime. The Go
// engine embeds the same bytes the self-hosted compiler emits from the
// generated osctl_runtime.kark.
func osctlRuntimeC() string {
	var b strings.Builder
	for _, f := range osctl.Files {
		b.WriteString(fmt.Sprintf("// ==== v1.2.34 runtime source: %s ====\n", f.Name))
		b.WriteString(stripOSCtlInclude(f.Src))
		b.WriteString("\n")
	}
	return b.String()
}

// OSCtlLibFlags returns the platform libraries required by the OS control
// runtime. All Win32 calls live in kernel32 (already linked), and POSIX uses
// libc, so this is deliberately empty; it exists as a named seam so a future
// platform change is explicit.
func OSCtlLibFlags(_ target.OS) []string { return nil }
