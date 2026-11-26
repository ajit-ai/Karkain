package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"karkain/pkg/parser"
	"karkain/pkg/target"
	"karkain/runtime/sqlite"
)

// v1.2.22: SQLite runtime is emitted only when sqlite_* builtins are used.
// The large amalgamation is compiled as a separate source by the CLI.

func isSQLiteBuiltin(name string) bool {
	switch name {
	case "sqlite_open", "sqlite_exec", "sqlite_query", "sqlite_close", "sqlite_last_error":
		return true
	}
	return false
}

func (g *Generator) scanNodeForSQLite(node parser.Node) {
	if node == nil || g.needsSQLite {
		return
	}
	switch n := node.(type) {
	case *parser.CallExpr:
		if isSQLiteBuiltin(n.Function) {
			g.needsSQLite = true
			return
		}
		for _, a := range n.Args {
			g.scanNodeForSQLite(a)
		}
	case *parser.IndirectCallExpr:
		g.scanNodeForSQLite(n.Target)
		for _, a := range n.Args {
			g.scanNodeForSQLite(a)
		}
	case *parser.FuncDecl:
		for _, s := range n.Body {
			g.scanNodeForSQLite(s)
		}
	case *parser.IfStmt:
		g.scanNodeForSQLite(n.Condition)
		for _, s := range n.Consequence {
			g.scanNodeForSQLite(s)
		}
		for _, s := range n.Alternative {
			g.scanNodeForSQLite(s)
		}
	case *parser.WhileStmt:
		g.scanNodeForSQLite(n.Condition)
		for _, s := range n.Body {
			g.scanNodeForSQLite(s)
		}
	case *parser.ForStmt:
		g.scanNodeForSQLite(n.Init)
		g.scanNodeForSQLite(n.Condition)
		g.scanNodeForSQLite(n.Post)
		for _, s := range n.Body {
			g.scanNodeForSQLite(s)
		}
	case *parser.ForInStmt:
		g.scanNodeForSQLite(n.Iter)
		for _, s := range n.Body {
			g.scanNodeForSQLite(s)
		}
	case *parser.VarDeclStmt:
		g.scanNodeForSQLite(n.Value)
	case *parser.ReturnStmt:
		g.scanNodeForSQLite(n.Value)
	case *parser.ExprStmt:
		g.scanNodeForSQLite(n.Expression)
	case *parser.PrintStmt:
		g.scanNodeForSQLite(n.Value)
	case *parser.BinaryExpr:
		g.scanNodeForSQLite(n.Left)
		g.scanNodeForSQLite(n.Right)
	case *parser.UnaryExpr:
		g.scanNodeForSQLite(n.Operand)
	case *parser.LambdaExpr:
		for _, s := range n.Body {
			g.scanNodeForSQLite(s)
		}
	case *parser.MatchExpr:
		g.scanNodeForSQLite(n.Value)
		for _, arm := range n.Arms {
			g.scanNodeForSQLite(arm.Body)
		}
	case *parser.IndexExpr:
		g.scanNodeForSQLite(n.Left)
		g.scanNodeForSQLite(n.Index)
	case *parser.SliceExpr:
		g.scanNodeForSQLite(n.Target)
		g.scanNodeForSQLite(n.Start)
		g.scanNodeForSQLite(n.End)
	case *parser.DotExpr:
		g.scanNodeForSQLite(n.Left)
	case *parser.ArrayLiteral:
		for _, e := range n.Elements {
			g.scanNodeForSQLite(e)
		}
	case *parser.MapLiteral:
		for _, k := range n.Keys {
			g.scanNodeForSQLite(k)
		}
		for _, v := range n.Values {
			g.scanNodeForSQLite(v)
		}
	case *parser.StructLiteral:
		for _, v := range n.Fields {
			g.scanNodeForSQLite(v)
		}
	}
}

func stripSQLiteInclude(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), `#include "karkain_sqlite.h"`) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

func sqliteRuntimeC() string {
	var b strings.Builder
	for _, f := range sqlite.Files {
		b.WriteString(fmt.Sprintf("// ==== v1.2.22 runtime source: %s ====\n", f.Name))
		b.WriteString(stripSQLiteInclude(f.Src))
		b.WriteString("\n")
	}
	return b.String()
}

// SQLiteLibFlags returns platform libraries required by the amalgamation.
func SQLiteLibFlags(os target.OS) []string {
	if os == target.OSWindows {
		return nil
	}
	return []string{"-ldl", "-lpthread"}
}

// WriteSQLiteVendor writes the vendored amalgamation beside the generated C
// so a single gcc invocation can compile both sources into one executable.
func WriteSQLiteVendor(dir string) (string, error) {
	path := filepath.Join(dir, "sqlite3.c")
	if err := os.WriteFile(path, []byte(sqlite.Source), 0644); err != nil {
		return "", err
	}
	return path, nil
}
