package lsp

// Phase 136, Slice B — syntactic scope model.
//
// Hover and go-to-definition resolve identifiers against bindings collected
// from the parsed AST: top-level declarations (visible everywhere) plus
// function parameters and let/var bindings (visible inside the owning
// function, after their declaration point). Positions come from the
// parser's own spans (Phase 83: 1-based lines, 0-based byte columns of the
// NAME token) converted to LSP lines and UTF-16 units; declarations that
// carry no column (struct/enum/kernel/actor headers, members) derive it by
// locating the name after its keyword on the source line.
//
// Deliberately syntactic (no pkg/sema dependency): the resolver is a
// whole-program diagnostics engine with no per-position query API, so a
// queryable model lives here. Types shown are annotations when present,
// literal inferences otherwise, and empty when unknown.

import (
	"reflect"
	"strings"
	"unicode/utf8"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// BindingKind classifies a name introduction.
type BindingKind string

const (
	BindFunc      BindingKind = "func"
	BindVar       BindingKind = "var"
	BindParam     BindingKind = "param"
	BindStruct    BindingKind = "struct"
	BindField     BindingKind = "field"
	BindEnum      BindingKind = "enum"
	BindVariant   BindingKind = "variant"
	BindKernel    BindingKind = "kernel"
	BindActor     BindingKind = "actor"
	BindMethod    BindingKind = "method"
	BindTrait     BindingKind = "trait"
	BindMacro     BindingKind = "macro"
	BindCircuit   BindingKind = "circuit"
	BindTensor    BindingKind = "tensor"
	BindCoroutine BindingKind = "coroutine"
	BindQReg      BindingKind = "qreg"
)

// Binding is one resolvable name with its declaration span.
type Binding struct {
	Name     string
	Kind     BindingKind
	Detail   string // hover-ready signature line
	Type     string // annotation or inferred primitive, "" when unknown
	Line     int    // 0-based declaration line
	Col      int    // UTF-16 units of the name start
	EndCol   int    // UTF-16 units just past the name
	ScopeLn  int    // 0-based first line of the owning scope
	ScopeEnd int    // 0-based last line of the owning scope (inclusive)
	Global   bool   // visible at every position (top-level declarations)
	value    parser.Node
	declWord string // `let`/`var`/`const` for detail rebuilds
}

// ScopeModel is the queryable binding set of one document.
type ScopeModel struct {
	Bindings []Binding
	// Structs/Enums map a type name to its member bindings (for `a.b` hover).
	Structs map[string][]Binding
	Enums   map[string][]Binding
	// Imports lists the module names imported by the document.
	Imports []string
}

// containerFields are the AST fields that can hold nested binding scopes.
// An allowlist (not full reflection): expression-only fields are never
// descended, so a `let` inside an unexpected expression shape degrades to
// "unknown" instead of mis-scoping.
var containerFields = map[string]bool{
	"Body": true, "Statements": true, "Consequence": true,
	"Alternative": true, "Init": true, "Post": true,
}

// collectVars gathers *parser.VarDeclStmt nodes from nested scopes.
func collectVars(nodes []parser.Node, out *[]*parser.VarDeclStmt) {
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if vd, ok := n.(*parser.VarDeclStmt); ok {
			*out = append(*out, vd)
			continue // never descend into the value (lambda params are out of scope for B)
		}
		v := reflect.ValueOf(n)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			continue
		}
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			if !containerFields[t.Field(i).Name] {
				continue
			}
			f := v.Field(i)
			switch f.Kind() {
			case reflect.Slice:
				var kids []parser.Node
				for j := 0; j < f.Len(); j++ {
					if nd, ok := f.Index(j).Interface().(parser.Node); ok && nd != nil {
						kids = append(kids, nd)
					}
				}
				collectVars(kids, out)
			default:
				if nd, ok := f.Interface().(parser.Node); ok && nd != nil {
					collectVars([]parser.Node{nd}, out)
				}
			}
		}
	}
}

// stmtLine0 returns the 0-based declaration line of a top-level statement,
// or -1 when the node carries no line.
func stmtLine0(n parser.Node) int {
	v := reflect.ValueOf(n)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return -1
	}
	if f := v.FieldByName("Line"); f.IsValid() && f.Kind() == reflect.Int {
		if ln := int(f.Int()); ln > 0 {
			return ln - 1
		}
	}
	return -1
}

// colAfterKeyword locates a name after its declaring keyword on a source
// line, returning the 0-based byte column or -1. Word boundaries guard
// against prefix matches (`x` inside `xyz`).
func colAfterKeyword(line, keyword, name string) int {
	ki := indexWord(line, keyword, 0)
	if ki < 0 {
		return indexWord(line, name, 0)
	}
	return indexWord(line, name, ki+len(keyword))
}

// indexWord finds word at or after `from`, or -1.
func indexWord(line, word string, from int) int {
	for i := from; i+len(word) <= len(line); i++ {
		if line[i:i+len(word)] != word {
			continue
		}
		if i > 0 && isWordChar(line[i-1]) {
			continue
		}
		if i+len(word) < len(line) && isWordChar(line[i+len(word)]) {
			continue
		}
		return i
	}
	return -1
}

func isWordChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// nameSpan converts a byte name span on a 0-based line to UTF-16 columns,
// clamped into the line.
func nameSpan(lines []string, line, byteCol, byteEnd int) (int, int) {
	lb := ""
	if line >= 0 && line < len(lines) {
		lb = lines[line]
	}
	return byteColToUTF16(lb, byteCol), byteColToUTF16(lb, byteEnd)
}

// braceSpan pairs one opening-brace line with its closing-brace line.
type braceSpan struct {
	open, close int // 0-based lines
}

// matchBraces pairs braces in lexer token order. Token-exactness (strings
// and comments never produce brace tokens) makes this immune to the
// classic brace-in-string miscount. Unclosed opens pair with the final
// line so truncated input still scopes sanely.
func matchBraces(text string, lines []string) []braceSpan {
	var spans []braceSpan
	var stack []int
	l := lexer.New(text)
	for {
		t := l.NextToken()
		if t.Type == lexer.TokenEOF {
			break
		}
		ln := int(t.Line) - 1
		switch t.Type {
		case lexer.TokenLBrace:
			stack = append(stack, ln)
		case lexer.TokenRBrace:
			if len(stack) > 0 {
				spans = append(spans, braceSpan{open: stack[len(stack)-1], close: ln})
				stack = stack[:len(stack)-1]
			}
		}
	}
	last := len(lines) - 1
	for _, o := range stack {
		spans = append(spans, braceSpan{open: o, close: last})
	}
	return spans
}

// scopeEndFor returns the closing line of the brace block opened at or
// after headerLine (the function header's own line), or -1 when there is
// no such block (single-line declaration: caller falls back). The span
// with the smallest opening line wins: any later open is nested inside it.
func scopeEndFor(spans []braceSpan, headerLine int) int {
	bestOpen, best := -1, -1
	for _, s := range spans {
		if s.open >= headerLine && (bestOpen < 0 || s.open < bestOpen) {
			bestOpen, best = s.open, s.close
		}
	}
	return best
}

// blockScope narrows a declaration to its innermost brace block: the
// smallest brace span containing declLine. Falls back to
// [fallbackLn, fallbackEnd] for declarations outside every block (headers
// whose brace opens later, single-line declarations).
func blockScope(spans []braceSpan, declLine, fallbackLn, fallbackEnd int) (int, int) {
	best, bln, bend := -1, fallbackLn, fallbackEnd
	for _, s := range spans {
		if s.open <= declLine && declLine <= s.close {
			if best < 0 || (s.close-s.open) < best {
				best = s.close - s.open
				bln, bend = s.open, s.close
			}
		}
	}
	return bln, bend
}

func funcScopeEnd(lines []string, prog *parser.Program, i int) int {
	for j := i + 1; j < len(prog.Statements); j++ {
		if ln := stmtLine0(prog.Statements[j]); ln >= 0 {
			return ln - 1
		}
	}
	return len(lines) - 1
}

// inferNodeType names the type of an initializer expression, or "" when
// the shape carries no obvious type. Binary comparisons yield bool; an
// arithmetic binary/unary over a single known primitive preserves it.
// Identifiers resolve through lookup (nil lookup leaves them unknown).
func inferNodeType(n parser.Node, lookup func(string) string) string {
	switch n.(type) {
	case *parser.IntLiteral, *parser.BigIntLiteral:
		return "int"
	case *parser.StringLiteral:
		return "string"
	case *parser.BoolLiteral:
		return "bool"
	case *parser.ArrayLiteral:
		return "array"
	case *parser.MapLiteral:
		return "map"
	case *parser.StructLiteral:
		if s, ok := n.(*parser.StructLiteral); ok && s.TypeName != "" {
			return s.TypeName
		}
		return "struct"
	case *parser.Identifier:
		if lookup != nil {
			if id, ok := n.(*parser.Identifier); ok {
				return lookup(id.Name)
			}
		}
		return ""
	case *parser.BinaryExpr:
		if b, ok := n.(*parser.BinaryExpr); ok {
			switch b.Operator {
			case "==", "!=", "<", ">", "<=", ">=", "&&", "||", "!":
				return "bool"
			}
			if lt, rt := inferNodeType(b.Left, lookup), inferNodeType(b.Right, lookup); lt != "" && lt == rt {
				return lt
			}
		}
		return ""
	case *parser.UnaryExpr:
		if u, ok := n.(*parser.UnaryExpr); ok {
			if u.Operator == "!" {
				return "bool"
			}
			return inferNodeType(u.Operand, lookup)
		}
		return ""
	default:
		return ""
	}
}

// renderVarDetail builds the hover signature of a variable binding.
func renderVarDetail(declWord, name, typ string) string {
	detail := declWord + " " + name
	if typ != "" {
		detail += ": " + typ
	}
	return detail
}

// declKindWord reads `let`/`var`/`const` from the declaration source line.
func declKindWord(vd *parser.VarDeclStmt, lines []string) string {
	if vd.Const {
		return "const"
	}
	ln := vd.Line - 1
	if ln >= 0 && ln < len(lines) {
		trimmed := strings.TrimSpace(lines[ln])
		for _, kw := range []string{"let ", "var ", "const "} {
			if strings.HasPrefix(trimmed, kw) {
				return strings.TrimSpace(kw)
			}
		}
	}
	return "let"
}

// BuildScopeModel collects every binding of a parsed document.
func BuildScopeModel(prog *parser.Program, text string) *ScopeModel {
	m := &ScopeModel{Structs: map[string][]Binding{}, Enums: map[string][]Binding{}}
	if prog == nil {
		return m
	}
	lines := strings.Split(text, "\n")
	spans := matchBraces(text, lines)
	for _, imp := range prog.Imports {
		if imp != nil && imp.Name != "" {
			m.Imports = append(m.Imports, imp.Name)
		}
	}

	addTop := func(b Binding) {
		b.Global = true
		b.ScopeLn = 0
		b.ScopeEnd = len(lines) - 1
		m.Bindings = append(m.Bindings, b)
	}

	for i, st := range prog.Statements {
		switch n := st.(type) {
		case *parser.FuncDecl:
			line0 := n.Line - 1
			c, e := nameSpan(lines, line0, n.Col, n.EndCol)
			// Brace-matched end (token-exact); falls back to next-decl for
			// single-line declarations without a brace block.
			scopeEnd := scopeEndFor(spans, line0)
			if scopeEnd < 0 {
				scopeEnd = funcScopeEnd(lines, prog, i)
			}
			m.Bindings = append(m.Bindings, Binding{
				Name: n.Name, Kind: BindFunc, Detail: formatFuncDecl(n),
				Line: line0, Col: c, EndCol: e,
				ScopeLn: 0, ScopeEnd: len(lines) - 1, Global: true,
			})
			for pi, p := range n.Params {
				pt := ""
				if pi < len(n.ParamTypes) {
					pt = n.ParamTypes[pi]
				}
				bline := ""
				if line0 >= 0 && line0 < len(lines) {
					bline = lines[line0]
				}
				pcol := indexWordAfterParen(bline, p)
				if pcol < 0 {
					pcol = n.Col
				}
				pcu, peu := nameSpan(lines, line0, pcol, pcol+len(p))
				detail := p
				if pt != "" {
					detail = p + ": " + pt
				}
				psln, psend := blockScope(spans, line0, line0, scopeEnd)
				m.Bindings = append(m.Bindings, Binding{
					Name: p, Kind: BindParam, Detail: detail, Type: pt,
					Line: line0, Col: pcu, EndCol: peu,
					ScopeLn: psln, ScopeEnd: psend, Global: false,
				})
			}
			var vars []*parser.VarDeclStmt
			collectVars(n.Body, &vars)
			for _, vd := range vars {
				m.Bindings = append(m.Bindings, varBinding(vd, lines, spans, line0, scopeEnd))
			}
		case *parser.StructDeclStmt:
			line0 := n.Line - 1
			lb := ""
			if line0 >= 0 && line0 < len(lines) {
				lb = lines[line0]
			}
			bc := colAfterKeyword(lb, "type", n.Name)
			if bc < 0 {
				bc = 0
			}
			c, e := nameSpan(lines, line0, bc, bc+len(n.Name))
			addTop(Binding{Name: n.Name, Kind: BindStruct, Detail: formatStructDecl(n), Line: line0, Col: c, EndCol: e})
			for _, f := range n.Fields {
				fc := indexWord(lb, f.Name, 0)
				if fc < 0 {
					fc = bc
				}
				fcs, fce := nameSpan(lines, line0, fc, fc+len(f.Name))
				fb := Binding{Name: f.Name, Kind: BindField,
					Detail: f.Name + ": " + f.Type,
					Line:   line0, Col: fcs, EndCol: fce,
					ScopeLn: line0, ScopeEnd: line0, Global: false}
				m.Structs[n.Name] = append(m.Structs[n.Name], fb)
			}
		case *parser.EnumDecl:
			line0 := n.Line - 1
			lb := ""
			if line0 >= 0 && line0 < len(lines) {
				lb = lines[line0]
			}
			bc := colAfterKeyword(lb, "enum", n.Name)
			if bc < 0 {
				bc = 0
			}
			c, e := nameSpan(lines, line0, bc, bc+len(n.Name))
			addTop(Binding{Name: n.Name, Kind: BindEnum, Detail: "enum " + n.Name, Line: line0, Col: c, EndCol: e})
			for _, v := range n.Variants {
				vl, vc := findVariantPos(lines, line0, v.Name)
				vcs, vce := nameSpan(lines, vl, vc, vc+len(v.Name))
				detail := n.Name + "." + v.Name
				if v.Payload != "" {
					detail += "(" + v.Payload + ")"
				}
				m.Enums[n.Name] = append(m.Enums[n.Name], Binding{
					Name: v.Name, Kind: BindVariant, Detail: detail,
					Line: vl, Col: vcs, EndCol: vce,
					ScopeLn: line0, ScopeEnd: line0, Global: false,
				})
			}
		case *parser.KernelDeclStmt:
			line0 := n.Line - 1
			lb := ""
			if line0 >= 0 && line0 < len(lines) {
				lb = lines[line0]
			}
			bc := colAfterKeyword(lb, "kernel", n.Name)
			if bc < 0 {
				bc = 0
			}
			c, e := nameSpan(lines, line0, bc, bc+len(n.Name))
			scopeEnd := scopeEndFor(spans, line0)
			if scopeEnd < 0 {
				scopeEnd = funcScopeEnd(lines, prog, i)
			}
			addTop(Binding{Name: n.Name, Kind: BindKernel, Detail: formatKernelDecl(n), Line: line0, Col: c, EndCol: e})
			for _, p := range n.Params {
				pcol := indexWordAfterParen(lb, p.Name)
				if pcol < 0 {
					pcol = bc
				}
				pcu, peu := nameSpan(lines, line0, pcol, pcol+len(p.Name))
				detail := p.Name
				if p.Type != "" {
					detail += ": " + p.Type
				}
				psln, psend := blockScope(spans, line0, line0, scopeEnd)
				m.Bindings = append(m.Bindings, Binding{
					Name: p.Name, Kind: BindParam, Detail: detail, Type: p.Type,
					Line: line0, Col: pcu, EndCol: peu,
					ScopeLn: psln, ScopeEnd: psend, Global: false,
				})
			}
		case *parser.ActorDeclStmt:
			line0 := n.Line - 1
			lb := ""
			if line0 >= 0 && line0 < len(lines) {
				lb = lines[line0]
			}
			bc := colAfterKeyword(lb, "actor", n.Name)
			if bc < 0 {
				bc = 0
			}
			c, e := nameSpan(lines, line0, bc, bc+len(n.Name))
			addTop(Binding{Name: n.Name, Kind: BindActor, Detail: "actor " + n.Name, Line: line0, Col: c, EndCol: e})
			for _, hh := range n.Handlers {
				hl, hc := findVariantPos(lines, line0, hh.MessageType)
				hcs, hce := nameSpan(lines, hl, hc, hc+len(hh.MessageType))
				m.Bindings = append(m.Bindings, Binding{
					Name: hh.MessageType, Kind: BindMethod,
					Detail: "handler " + hh.MessageType + "(" + hh.ParamName + ": " + hh.ParamType + ")",
					Line:   hl, Col: hcs, EndCol: hce,
					ScopeLn: 0, ScopeEnd: len(lines) - 1, Global: true,
				})
			}
		case *parser.TraitDeclStmt:
			addTop(derivedTop(lines, n.Line, "trait", n.Name, BindTrait, "trait "+n.Name))
		case *parser.MacroDeclStmt:
			addTop(derivedTop(lines, n.Line, "macro", n.Name, BindMacro, "macro "+n.Name))
		case *parser.CircuitDecl:
			addTop(derivedTop(lines, n.Line, "circuit", n.Name, BindCircuit, "circuit "+n.Name))
		case *parser.TensorStmt:
			addTop(derivedTop(lines, n.Line, "tensor", n.Name, BindTensor, "tensor "+n.Name))
		case *parser.CoroutineDecl:
			addTop(derivedTop(lines, n.Line, "co", n.Name, BindCoroutine, "coroutine "+n.Name))
		case *parser.QRegDeclStmt:
			addTop(derivedTop(lines, n.Line, "qreg", n.Name, BindQReg, "qreg "+n.Name))
		case *parser.VarDeclStmt:
			// Top-level binding: visible everywhere like other globals.
			b := varBinding(n, lines, spans, 0, len(lines)-1)
			b.Global = true
			b.ScopeLn = 0
			b.ScopeEnd = len(lines) - 1
			m.Bindings = append(m.Bindings, b)
		}
	}
	refineVarTypes(m)
	return m
}

// derivedTop builds a global binding for a header-only declaration whose
// name column derives from its keyword.
func derivedTop(lines []string, line1 int, keyword, name string, kind BindingKind, detail string) Binding {
	line0 := line1 - 1
	lb := ""
	if line0 >= 0 && line0 < len(lines) {
		lb = lines[line0]
	}
	bc := colAfterKeyword(lb, keyword, name)
	if bc < 0 {
		bc = 0
	}
	c, e := nameSpan(lines, line0, bc, bc+len(name))
	return Binding{Name: name, Kind: kind, Detail: detail, Line: line0, Col: c, EndCol: e}
}

// varBinding builds the lexical binding of a let/var/const declaration.
// The scope narrows to the innermost brace block holding the declaration.
// The type starts from the annotation or shallow literal shape; identifier
// and binary operands refine in a second pass (refineVarTypes) once every
// binding exists.
func varBinding(vd *parser.VarDeclStmt, lines []string, spans []braceSpan, fallbackLn, fallbackEnd int) Binding {
	line0 := vd.Line - 1
	c, e := nameSpan(lines, line0, vd.Col, vd.EndCol)
	sln, send := blockScope(spans, line0, fallbackLn, fallbackEnd)
	typ := vd.Type
	if typ == "" {
		typ = inferNodeType(vd.Value, nil)
	}
	word := declKindWord(vd, lines)
	return Binding{
		Name: vd.Name, Kind: BindVar, Detail: renderVarDetail(word, vd.Name, typ), Type: typ,
		Line: line0, Col: c, EndCol: e,
		ScopeLn: sln, ScopeEnd: send, Global: false,
		value: vd.Value, declWord: word,
	}
}

// refineVarTypes upgrades untyped variable bindings whose initializers
// resolve through already-known bindings (single ordered pass: bindings
// append in source order, so earlier declarations are final).
func refineVarTypes(m *ScopeModel) {
	lookupFor := func(useLine, useChar int) func(string) string {
		return func(name string) string {
			var best *Binding
			bestScope := -2
			for i := range m.Bindings {
				b := &m.Bindings[i]
				if b.Name != name || b.Type == "" {
					continue
				}
				if b.Global {
					if best == nil {
						best = b
					}
					continue
				}
				if useLine < b.ScopeLn || useLine > b.ScopeEnd {
					continue
				}
				if b.Line > useLine || (b.Line == useLine && b.EndCol > useChar) {
					continue
				}
				if b.ScopeLn > bestScope {
					bestScope = b.ScopeLn
					best = b
				}
			}
			if best == nil {
				return ""
			}
			return best.Type
		}
	}
	for i := range m.Bindings {
		b := &m.Bindings[i]
		if b.Kind != BindVar || b.Type != "" || b.value == nil {
			continue
		}
		if t := inferNodeType(b.value, lookupFor(b.Line, b.Col)); t != "" {
			b.Type = t
			b.Detail = renderVarDetail(b.declWord, b.Name, t)
		}
	}
}

// indexWordAfterParen locates a parameter name after the opening paren of a
// declaration source line, or -1.
func indexWordAfterParen(line, name string) int {
	open := strings.Index(line, "(")
	if open < 0 {
		return indexWord(line, name, 0)
	}
	return indexWord(line, name, open+1)
}

// findVariantPos locates an enum variant name at or after the enum header,
// returning its 0-based line and byte column (header line scanned too — a
// variant name never equals the type name, so the first match is the
// variant itself).
func findVariantPos(lines []string, fromLine int, name string) (int, int) {
	for ln := fromLine; ln < len(lines); ln++ {
		if c := indexWord(lines[ln], name, 0); c >= 0 {
			return ln, c
		}
	}
	return fromLine, 0
}

// ResolveAt finds the binding referenced by the identifier at (line, char).
// Dotted words (`a.b`) resolve the member against the qualifier's struct or
// enum; anything unresolvable yields nil (never an error, never a guess).
func ResolveAt(m *ScopeModel, text, uri string, line, char int) *Binding {
	_ = uri
	lines := strings.Split(text, "\n")
	word := wordAtPos(lines, line, char)
	if word == "" {
		return nil
	}
	if dot := strings.LastIndex(word, "."); dot >= 0 {
		qual, member := word[:dot], word[dot+1:]
		if fields, ok := m.Structs[qual]; ok {
			for i := range fields {
				if fields[i].Name == member {
					b := fields[i]
					return &b
				}
			}
		}
		if variants, ok := m.Enums[qual]; ok {
			for i := range variants {
				if variants[i].Name == member {
					b := variants[i]
					return &b
				}
			}
		}
		return nil
	}
	var best *Binding
	bestScope := -2
	bestLine := -1
	for i := range m.Bindings {
		b := &m.Bindings[i]
		if b.Name != word {
			continue
		}
		if b.Global {
			if best == nil {
				best = b
			}
			continue
		}
		if line < b.ScopeLn || line > b.ScopeEnd {
			continue
		}
		if b.Line > line || (b.Line == line && b.EndCol > char) {
			continue // declared after the use point
		}
		// Narrowest scope wins; same-scope redeclarations resolve to the
		// nearest preceding one.
		if b.ScopeLn > bestScope || (b.ScopeLn == bestScope && b.Line > bestLine) {
			bestScope = b.ScopeLn
			bestLine = b.Line
			best = b
		}
	}
	return best
}

// visibleBindings returns every lexically completable binding at a
// position: globals plus in-scope declarations ordered by declaration
// (narrowest scope and nearest preceding declaration first). Struct
// fields, enum variants and actor handlers are member-only (reachable
// through `a.b`) and never complete as bare identifiers.
func visibleBindings(m *ScopeModel, line, char int) []Binding {
	if m == nil {
		return nil
	}
	var scoped, globals []Binding
	for i := range m.Bindings {
		b := m.Bindings[i]
		if b.Kind == BindField || b.Kind == BindVariant || b.Kind == BindMethod {
			continue
		}
		if b.Global {
			globals = append(globals, b)
			continue
		}
		if line < b.ScopeLn || line > b.ScopeEnd {
			continue
		}
		if b.Line > line || (b.Line == line && b.EndCol > char) {
			continue
		}
		scoped = append(scoped, b)
	}
	// Narrowest scope first, then nearest preceding declaration.
	for i := 1; i < len(scoped); i++ {
		j := i
		for j > 0 && (scoped[j-1].ScopeLn < scoped[j].ScopeLn ||
			(scoped[j-1].ScopeLn == scoped[j].ScopeLn && scoped[j-1].Line < scoped[j].Line)) {
			scoped[j-1], scoped[j] = scoped[j], scoped[j-1]
			j--
		}
	}
	return append(scoped, globals...)
}

// wordAtPos extracts the dotted-identifier word at a position, where char
// is in UTF-16 units (protocol semantics).
func wordAtPos(lines []string, line, char int) string {
	if line < 0 || line >= len(lines) || char < 0 {
		return ""
	}
	ln := lines[line]
	// Convert the UTF-16 column to a byte index.
	byteIdx := 0
	units := 0
	for byteIdx < len(ln) && units < char {
		r, size := utf8.DecodeRuneInString(ln[byteIdx:])
		byteIdx += size
		if r < 0x10000 {
			units++
		} else {
			units += 2
		}
	}
	start, end := byteIdx, byteIdx
	for start > 0 && isWordCharDot(ln[start-1]) {
		start--
	}
	for end < len(ln) && isWordCharDot(ln[end]) {
		end++
	}
	if start >= end {
		return ""
	}
	return ln[start:end]
}

func isWordCharDot(c byte) bool {
	return isWordChar(c) || c == '.'
}
