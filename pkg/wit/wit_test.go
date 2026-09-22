package wit

import (
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

const sampleWIT = `package karkain:geometry;

interface shapes {
  record point {
    x: s32,
    y: s32,
  }
  enum color { red, green, blue }
  variant shape {
    circle(point),
    rect,
  }
  flags opts { fast, safe }
  area: func(s: string) -> f64;
  origin: func() -> point;
}

world app {
  import shapes;
  export run: func() -> s32;
}
`

func TestParseGolden(t *testing.T) {
	doc, err := Parse(sampleWIT)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Package != "karkain:geometry" {
		t.Errorf("package = %q", doc.Package)
	}
	if len(doc.Interfaces) != 1 || doc.Interfaces[0].Name != "shapes" {
		t.Fatalf("interfaces = %+v", doc.Interfaces)
	}
	it := doc.Interfaces[0]
	if len(it.Records) != 1 || it.Records[0].Name != "point" || len(it.Records[0].Fields) != 2 {
		t.Errorf("records = %+v", it.Records)
	}
	if len(it.Enums) != 1 || len(it.Enums[0].Cases) != 3 {
		t.Errorf("enums = %+v", it.Enums)
	}
	if len(it.Variants) != 1 || len(it.Variants[0].Cases) != 2 || it.Variants[0].Cases[0].Payload == nil {
		t.Errorf("variants = %+v", it.Variants)
	}
	if len(it.Flags) != 1 || len(it.Flags[0].Cases) != 2 {
		t.Errorf("flags = %+v", it.Flags)
	}
	if len(it.Funcs) != 2 || it.Funcs[0].Name != "area" || it.Funcs[0].Result == nil {
		t.Errorf("funcs = %+v", it.Funcs)
	}
	if len(doc.Worlds) != 1 || len(doc.Worlds[0].Imports) != 1 || len(doc.Worlds[0].Exports) != 1 {
		t.Errorf("worlds = %+v", doc.Worlds)
	}
	if doc.Worlds[0].Exports[0].Func == nil {
		t.Errorf("world export should be an inline function: %+v", doc.Worlds[0].Exports[0])
	}
}

func TestRoundTripDeterministic(t *testing.T) {
	doc, err := Parse(sampleWIT)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a, b := doc.Summary(), doc.Summary()
	if a != b {
		t.Fatal("summary not deterministic")
	}
	// Summary must re-parse to an identical summary (canonical form).
	doc2, err := Parse(a)
	if err != nil {
		t.Fatalf("re-parse of summary: %v", err)
	}
	if doc2.Summary() != a {
		t.Fatalf("round-trip mismatch:\n%s\n---\n%s", a, doc2.Summary())
	}
	for _, frag := range []string{
		"package karkain:geometry;",
		"record point {",
		"enum color { red, green, blue }",
		"circle(point)",
		"area: func(s: string) -> f64;",
		"world app {",
		"import shapes;",
		"export run: func() -> s32;",
	} {
		if !strings.Contains(a, frag) {
			t.Errorf("summary lacks %q:\n%s", frag, a)
		}
	}
}

func TestStubsGolden(t *testing.T) {
	doc, err := Parse(sampleWIT)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	stubs, err := doc.Stubs()
	if err != nil {
		t.Fatalf("stubs: %v", err)
	}
	for _, frag := range []string{
		"type point struct { x int; y int }",
		"enum color { red, green, blue }",
		"enum shape { circle, rect }",
	} {
		if !strings.Contains(stubs, frag) {
			t.Errorf("stubs lack %q:\n%s", frag, stubs)
		}
	}
	// The stubs must parse as real Karkain (no dead surface).
	l := lexer.New(stubs)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("stubs do not parse as Karkain: %v\n%s", p.Errors, stubs)
	}
	found := map[string]bool{}
	for _, s := range prog.Statements {
		switch n := s.(type) {
		case *parser.StructDeclStmt:
			found["struct:"+n.Name] = true
		case *parser.EnumDecl:
			found["enum:"+n.Name] = true
		}
	}
	for _, want := range []string{"struct:point", "enum:color", "enum:shape"} {
		if !found[want] {
			t.Errorf("stubs missing %s (parsed %+v)\n%s", want, prog.Statements, stubs)
		}
	}
}

func TestParseNegatives(t *testing.T) {
	// Package-only document is valid.
	if _, err := Parse(`package karkain:geometry;`); err != nil {
		t.Errorf("package-only should parse: %v", err)
	}
	// Named (forward) types defer resolution — parseable here.
	if _, err := Parse(`package karkain:geometry; interface i { record r { x: frobnicate } }`); err != nil {
		t.Errorf("named field type should parse: %v", err)
	}
	for _, src := range []string{
		`interface shapes { record point { x: s32 } }`,
		`package karkain:geometry; bogus foo;`,
		`package karkain:geometry; interface i { record r { x: s32 }`, // unterminated
		`package karkain:geometry; world w { consume foo; }`,          // bad world item
	} {
		if _, err := Parse(src); err == nil {
			t.Errorf("expected parse error for %q", src)
		}
	}
}

func TestStubsRejectComposite(t *testing.T) {
	doc, err := Parse(`package karkain:m;
interface i {
  record r { xs: list<s32> }
}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := doc.Stubs(); err == nil {
		t.Error("expected stub error for composite list<s32> field")
	} else if !strings.Contains(err.Error(), "no Karkain stub surface") {
		t.Errorf("wrong stub error: %v", err)
	}
}
