// Package wit implements Karkain's WIT (WebAssembly Interface Types) subset
// tooling, Phase 139.
//
// Scope (honest MVP): parse the structural WIT surface Karkain component
// authoring needs — package/interface/world declarations, records, variants,
// enums, flags, and function signatures over primitive, named, list, option,
// result and tuple types — and (a) print the canonical summary (round-trip
// deterministic), (b) emit Karkain type stubs (records → `type X struct`,
// variants/enums → `enum X`) that compile on every engine. Function IMPORT
// call lowering (component-import codegen) is future work: the parser keeps
// the signatures so the surface is complete, but stubs cover types only and
// say so. Anything outside the subset is a deterministic parse error, never
// a silent drop.
package wit

import (
	"fmt"
	"strings"
)

// --- model ---------------------------------------------------------------

// Type is a WIT type expression.
type Type struct {
	// Kind is one of: prim, named, list, option, result, tuple.
	Kind   string
	Name   string  // prim name (s32, string, ...) or named reference
	Elem   *Type   // list/option element
	Ok     *Type   // result ok (nil = empty)
	Err    *Type   // result err (nil = empty)
	Fields []*Type // tuple elements
}

// Param is a named function parameter.
type Param struct {
	Name string
	Type *Type
}

// Func is an interface function: name + params + optional result.
type Func struct {
	Name   string
	Params []Param
	Result *Type // nil = no result
}

// Field is a record field.
type Field struct {
	Name string
	Type *Type
}

// Record is a WIT record declaration.
type Record struct {
	Name   string
	Fields []Field
}

// VariantCase is one variant case with an optional payload type.
type VariantCase struct {
	Name    string
	Payload *Type // nil = unit case
}

// Variant is a WIT variant declaration.
type Variant struct {
	Name  string
	Cases []VariantCase
}

// Enum is a WIT enum declaration.
type Enum struct {
	Name  string
	Cases []string
}

// Flags is a WIT flags declaration.
type Flags struct {
	Name  string
	Cases []string
}

// Interface is a WIT interface: named types + functions.
type Interface struct {
	Name     string
	Records  []Record
	Variants []Variant
	Enums    []Enum
	Flags    []Flags
	Funcs    []Func
}

// WorldItem is an import/export entry: either an interface name or an
// inline function.
type WorldItem struct {
	Name string // interface name, or function name when Func != nil
	Func *Func  // non-nil for inline world functions
}

// World is a WIT world: imports + exports.
type World struct {
	Name    string
	Imports []WorldItem
	Exports []WorldItem
}

// Document is a parsed WIT file: one package, its interfaces and worlds.
type Document struct {
	Package    string
	Interfaces []Interface
	Worlds     []World
}

// --- lexer ---------------------------------------------------------------

type token struct {
	kind string // ident, punct, eof
	text string
}

func lex(src string) []token {
	var out []token
	i := 0
	isIdent := func(c byte) bool {
		// Kebab-case WIT names only (letters, digits, _, -). Separators
		// (: ; , ( ) { } < >) and comment slashes lex separately — notably
		// ':' splits `ns:name` into three tokens for the package rule.
		return c == '_' || c == '-' ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
	}
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '-' && i+1 < len(src) && src[i+1] == '>':
			out = append(out, token{"punct", "->"})
			i += 2
		case isIdent(c):
			j := i
			for j < len(src) && isIdent(src[j]) {
				j++
			}
			out = append(out, token{"ident", src[i:j]})
			i = j
		default:
			out = append(out, token{"punct", string(c)})
			i++
		}
	}
	return append(out, token{"eof", ""})
}

// --- parser --------------------------------------------------------------

type witParser struct {
	toks []token
	pos  int
}

func (p *witParser) peek() token { return p.toks[p.pos] }

func (p *witParser) next() token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func (p *witParser) want(text string) (token, error) {
	t := p.next()
	if t.text != text {
		return t, fmt.Errorf("expected '%s', got '%s'", text, t.text)
	}
	return t, nil
}

func (p *witParser) wantIdent(what string) (token, error) {
	t := p.next()
	if t.kind != "ident" {
		return t, fmt.Errorf("expected %s, got '%s'", what, t.text)
	}
	return t, nil
}

var primitives = map[string]bool{
	"u8": true, "s8": true, "u16": true, "s16": true,
	"u32": true, "s32": true, "u64": true, "s64": true,
	"f32": true, "f64": true, "bool": true, "char": true, "string": true,
}

func (p *witParser) parseType() (*Type, error) {
	t, err := p.wantIdent("type")
	if err != nil {
		return nil, err
	}
	switch t.text {
	case "list":
		if _, err := p.want("<"); err != nil {
			return nil, err
		}
		el, err := p.parseType()
		if err != nil {
			return nil, err
		}
		if _, err := p.want(">"); err != nil {
			return nil, err
		}
		return &Type{Kind: "list", Elem: el}, nil
	case "option":
		if _, err := p.want("<"); err != nil {
			return nil, err
		}
		el, err := p.parseType()
		if err != nil {
			return nil, err
		}
		if _, err := p.want(">"); err != nil {
			return nil, err
		}
		return &Type{Kind: "option", Elem: el}, nil
	case "result":
		if _, err := p.want("<"); err != nil {
			return nil, err
		}
		r := &Type{Kind: "result"}
		if p.peek().text != "," && p.peek().text != ">" {
			ok, err := p.parseType()
			if err != nil {
				return nil, err
			}
			r.Ok = ok
		}
		if p.peek().text == "," {
			p.next()
			if p.peek().text != ">" {
				e, err := p.parseType()
				if err != nil {
					return nil, err
				}
				r.Err = e
			}
		}
		if _, err := p.want(">"); err != nil {
			return nil, err
		}
		return r, nil
	case "tuple":
		if _, err := p.want("<"); err != nil {
			return nil, err
		}
		tp := &Type{Kind: "tuple"}
		for {
			el, err := p.parseType()
			if err != nil {
				return nil, err
			}
			tp.Fields = append(tp.Fields, el)
			if p.peek().text == "," {
				p.next()
				continue
			}
			break
		}
		if _, err := p.want(">"); err != nil {
			return nil, err
		}
		return tp, nil
	}
	if primitives[t.text] {
		return &Type{Kind: "prim", Name: t.text}, nil
	}
	return &Type{Kind: "named", Name: t.text}, nil
}

func (p *witParser) parseFunc() (Func, error) {
	var f Func
	name, err := p.wantIdent("function name")
	if err != nil {
		return f, err
	}
	f.Name = name.text
	if _, err := p.want(":"); err != nil {
		return f, err
	}
	if _, err := p.want("func"); err != nil {
		return f, err
	}
	if _, err := p.want("("); err != nil {
		return f, err
	}
	for p.peek().text != ")" {
		pn, err := p.wantIdent("parameter name")
		if err != nil {
			return f, err
		}
		if _, err := p.want(":"); err != nil {
			return f, err
		}
		pt, err := p.parseType()
		if err != nil {
			return f, err
		}
		f.Params = append(f.Params, Param{pn.text, pt})
		if p.peek().text == "," {
			p.next()
		} else {
			break
		}
	}
	if _, err := p.want(")"); err != nil {
		return f, err
	}
	if p.peek().text == "->" {
		p.next()
		r, err := p.parseType()
		if err != nil {
			return f, err
		}
		f.Result = r
	}
	if _, err := p.want(";"); err != nil {
		return f, err
	}
	return f, nil
}

func (p *witParser) parseRecord() (Record, error) {
	var r Record
	name, err := p.wantIdent("record name")
	if err != nil {
		return r, err
	}
	r.Name = name.text
	if _, err := p.want("{"); err != nil {
		return r, err
	}
	for p.peek().text != "}" {
		fn, err := p.wantIdent("field name")
		if err != nil {
			return r, err
		}
		if _, err := p.want(":"); err != nil {
			return r, err
		}
		ft, err := p.parseType()
		if err != nil {
			return r, err
		}
		r.Fields = append(r.Fields, Field{fn.text, ft})
		if p.peek().text == "," {
			p.next()
		} else {
			break
		}
	}
	if _, err := p.want("}"); err != nil {
		return r, err
	}
	return r, nil
}

func (p *witParser) parseVariant() (Variant, error) {
	var v Variant
	name, err := p.wantIdent("variant name")
	if err != nil {
		return v, err
	}
	v.Name = name.text
	if _, err := p.want("{"); err != nil {
		return v, err
	}
	for p.peek().text != "}" {
		cn, err := p.wantIdent("case name")
		if err != nil {
			return v, err
		}
		c := VariantCase{Name: cn.text}
		if p.peek().text == "(" {
			p.next()
			pt, err := p.parseType()
			if err != nil {
				return v, err
			}
			c.Payload = pt
			if _, err := p.want(")"); err != nil {
				return v, err
			}
		}
		v.Cases = append(v.Cases, c)
		if p.peek().text == "," {
			p.next()
		} else {
			break
		}
	}
	if _, err := p.want("}"); err != nil {
		return v, err
	}
	return v, nil
}

func (p *witParser) parseEnumOrFlags(keyword string) ([]string, string, error) {
	name, err := p.wantIdent(keyword + " name")
	if err != nil {
		return nil, "", err
	}
	if _, err := p.want("{"); err != nil {
		return nil, "", err
	}
	var cases []string
	for p.peek().text != "}" {
		cn, err := p.wantIdent("case name")
		if err != nil {
			return nil, "", err
		}
		cases = append(cases, cn.text)
		if p.peek().text == "," {
			p.next()
		} else {
			break
		}
	}
	if _, err := p.want("}"); err != nil {
		return nil, "", err
	}
	return cases, name.text, nil
}

func (p *witParser) parseInterface() (Interface, error) {
	var it Interface
	name, err := p.wantIdent("interface name")
	if err != nil {
		return it, err
	}
	it.Name = name.text
	if _, err := p.want("{"); err != nil {
		return it, err
	}
	for p.peek().text != "}" {
		kw, err := p.wantIdent("declaration")
		if err != nil {
			return it, err
		}
		switch kw.text {
		case "record":
			r, err := p.parseRecord()
			if err != nil {
				return it, err
			}
			it.Records = append(it.Records, r)
		case "variant":
			v, err := p.parseVariant()
			if err != nil {
				return it, err
			}
			it.Variants = append(it.Variants, v)
		case "enum":
			cases, ename, err := p.parseEnumOrFlags("enum")
			if err != nil {
				return it, err
			}
			it.Enums = append(it.Enums, Enum{ename, cases})
		case "flags":
			cases, fname, err := p.parseEnumOrFlags("flags")
			if err != nil {
				return it, err
			}
			it.Flags = append(it.Flags, Flags{fname, cases})
		default:
			// Otherwise it must be "<name>: func..." — rewind one token.
			p.pos--
			f, err := p.parseFunc()
			if err != nil {
				return it, err
			}
			it.Funcs = append(it.Funcs, f)
		}
	}
	if _, err := p.want("}"); err != nil {
		return it, err
	}
	return it, nil
}

func (p *witParser) parseWorldItem() (WorldItem, error) {
	var wi WorldItem
	name, err := p.wantIdent("import/export name")
	if err != nil {
		return wi, err
	}
	if p.peek().text == ":" {
		// Inline function: "name: func(...);" — parseFunc consumes the
		// trailing semicolon itself.
		p.pos--
		f, err := p.parseFunc()
		if err != nil {
			return wi, err
		}
		wi.Name = f.Name
		wi.Func = &f
		return wi, nil
	}
	wi.Name = name.text
	if _, err := p.want(";"); err != nil {
		return wi, err
	}
	return wi, nil
}

func (p *witParser) parseWorld() (World, error) {
	var w World
	name, err := p.wantIdent("world name")
	if err != nil {
		return w, err
	}
	w.Name = name.text
	if _, err := p.want("{"); err != nil {
		return w, err
	}
	for p.peek().text != "}" {
		kw, err := p.wantIdent("import or export")
		if err != nil {
			return w, err
		}
		switch kw.text {
		case "import":
			wi, err := p.parseWorldItem()
			if err != nil {
				return w, err
			}
			w.Imports = append(w.Imports, wi)
		case "export":
			wi, err := p.parseWorldItem()
			if err != nil {
				return w, err
			}
			w.Exports = append(w.Exports, wi)
		default:
			return w, fmt.Errorf("expected 'import' or 'export', got '%s'", kw.text)
		}
	}
	if _, err := p.want("}"); err != nil {
		return w, err
	}
	return w, nil
}

// Parse parses a WIT source file into a Document. Anything outside the
// Phase-139 subset is a deterministic error naming the location.
func Parse(src string) (*Document, error) {
	p := &witParser{toks: lex(src)}
	doc := &Document{}
	for p.peek().kind != "eof" {
		kw, err := p.wantIdent("declaration")
		if err != nil {
			return nil, err
		}
		switch kw.text {
		case "package":
			ns, err := p.wantIdent("package namespace")
			if err != nil {
				return nil, err
			}
			if _, err := p.want(":"); err != nil {
				return nil, err
			}
			nm, err := p.wantIdent("package name")
			if err != nil {
				return nil, err
			}
			doc.Package = ns.text + ":" + nm.text
			if _, err := p.want(";"); err != nil {
				return nil, err
			}
		case "interface":
			it, err := p.parseInterface()
			if err != nil {
				return nil, err
			}
			doc.Interfaces = append(doc.Interfaces, it)
		case "world":
			w, err := p.parseWorld()
			if err != nil {
				return nil, err
			}
			doc.Worlds = append(doc.Worlds, w)
		default:
			return nil, fmt.Errorf("unexpected top-level declaration '%s' (want package, interface or world)", kw.text)
		}
	}
	if doc.Package == "" {
		return nil, fmt.Errorf("WIT document lacks a package declaration")
	}
	return doc, nil
}

// --- canonical summary ---------------------------------------------------

func typeString(t *Type) string {
	switch t.Kind {
	case "prim", "named":
		return t.Name
	case "list":
		return "list<" + typeString(t.Elem) + ">"
	case "option":
		return "option<" + typeString(t.Elem) + ">"
	case "result":
		s := "result"
		if t.Ok != nil || t.Err != nil {
			s += "<"
			if t.Ok != nil {
				s += typeString(t.Ok)
			} else {
				s += "_"
			}
			if t.Err != nil {
				s += ", " + typeString(t.Err)
			}
			s += ">"
		}
		return s
	case "tuple":
		parts := make([]string, len(t.Fields))
		for i, f := range t.Fields {
			parts[i] = typeString(f)
		}
		return "tuple<" + strings.Join(parts, ", ") + ">"
	}
	return "?"
}

func funcString(f Func) string {
	var b strings.Builder
	b.WriteString(f.Name + ": func(")
	for i, p := range f.Params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name + ": " + typeString(p.Type))
	}
	b.WriteString(")")
	if f.Result != nil {
		b.WriteString(" -> " + typeString(f.Result))
	}
	b.WriteString(";")
	return b.String()
}

// Summary renders the deterministic canonical summary: package, then each
// interface (types then functions) and world, all in source order.
func (d *Document) Summary() string {
	var b strings.Builder
	b.WriteString("package " + d.Package + ";\n")
	for _, it := range d.Interfaces {
		b.WriteString("interface " + it.Name + " {\n")
		for _, r := range it.Records {
			b.WriteString("  record " + r.Name + " {\n")
			for _, f := range r.Fields {
				b.WriteString("    " + f.Name + ": " + typeString(f.Type) + ",\n")
			}
			b.WriteString("  }\n")
		}
		for _, v := range it.Variants {
			b.WriteString("  variant " + v.Name + " {\n")
			for _, c := range v.Cases {
				b.WriteString("    " + c.Name)
				if c.Payload != nil {
					b.WriteString("(" + typeString(c.Payload) + ")")
				}
				b.WriteString(",\n")
			}
			b.WriteString("  }\n")
		}
		for _, e := range it.Enums {
			b.WriteString("  enum " + e.Name + " { " + strings.Join(e.Cases, ", ") + " }\n")
		}
		for _, f := range it.Flags {
			b.WriteString("  flags " + f.Name + " { " + strings.Join(f.Cases, ", ") + " }\n")
		}
		for _, f := range it.Funcs {
			b.WriteString("  " + funcString(f) + "\n")
		}
		b.WriteString("}\n")
	}
	for _, w := range d.Worlds {
		b.WriteString("world " + w.Name + " {\n")
		for _, im := range w.Imports {
			if im.Func != nil {
				b.WriteString("  import " + funcString(*im.Func) + "\n")
			} else {
				b.WriteString("  import " + im.Name + ";\n")
			}
		}
		for _, ex := range w.Exports {
			if ex.Func != nil {
				b.WriteString("  export " + funcString(*ex.Func) + "\n")
			} else {
				b.WriteString("  export " + ex.Name + ";\n")
			}
		}
		b.WriteString("}\n")
	}
	return b.String()
}

// --- Karkain stubs -------------------------------------------------------

var witToKarkain = map[string]string{
	"u8": "int", "s8": "int", "u16": "int", "s16": "int",
	"u32": "int", "s32": "int", "u64": "int", "s64": "int",
	"f32": "float", "f64": "float",
	"bool": "bool", "char": "string", "string": "string",
}

// karkainType maps a WIT type to a Karkain type name for stubs. Only scalar
// mappings and same-document named types are emittable; composite nesting
// (list/option/result/tuple) has no Karkain surface yet and is an error.
func karkainType(t *Type, known map[string]bool) (string, error) {
	switch t.Kind {
	case "prim":
		if k, ok := witToKarkain[t.Name]; ok {
			return k, nil
		}
		return "", fmt.Errorf("no Karkain mapping for WIT type '%s'", t.Name)
	case "named":
		if known[t.Name] {
			return t.Name, nil
		}
		return "", fmt.Errorf("unknown WIT type '%s'", t.Name)
	}
	return "", fmt.Errorf("composite WIT type '%s' has no Karkain stub surface yet", t.Kind)
}

// Stubs emits compilable Karkain type declarations for the document's named
// types: records → `type X struct`, variants/enums → `enum X` (payloads are
// documented, not lowered — construction stays future work), flags → bitmask
// constants. Function signatures are intentionally NOT emitted: interface
// call lowering (component imports) is the documented future slice, and
// emitting dead funcs would lie about linkability.
func (d *Document) Stubs() (string, error) {
	known := map[string]bool{}
	for _, it := range d.Interfaces {
		for _, r := range it.Records {
			known[r.Name] = true
		}
		for _, v := range it.Variants {
			known[v.Name] = true
		}
		for _, e := range it.Enums {
			known[e.Name] = true
		}
	}
	var b strings.Builder
	b.WriteString("// Generated by `karkain wit --stubs` from " + d.Package + " — DO NOT EDIT.\n")
	b.WriteString("// Function call lowering is future work; only types are emitted.\n")
	for _, it := range d.Interfaces {
		for _, r := range it.Records {
			// Proven struct form (see the wasm struct golden): fields
			// semicolon-separated; layout order is declaration order.
			parts := make([]string, 0, len(r.Fields))
			for _, f := range r.Fields {
				kt, err := karkainType(f.Type, known)
				if err != nil {
					return "", err
				}
				parts = append(parts, f.Name+" "+kt)
			}
			b.WriteString("type " + r.Name + " struct { " + strings.Join(parts, "; ") + " }\n")
		}
		for _, e := range it.Enums {
			// Proven enum form (conformance/012_enums_test.kark).
			b.WriteString("enum " + e.Name + " { " + strings.Join(e.Cases, ", ") + " }\n")
		}
		for _, v := range it.Variants {
			b.WriteString("// variant " + v.Name + ": payload construction is future work\n")
			names := make([]string, 0, len(v.Cases))
			for _, c := range v.Cases {
				names = append(names, c.Name)
			}
			b.WriteString("enum " + v.Name + " { " + strings.Join(names, ", ") + " }\n")
		}
		for _, f := range it.Flags {
			b.WriteString("// flags " + f.Name + ": no Karkain surface yet (no const bitmask)\n")
		}
	}
	return b.String(), nil
}
