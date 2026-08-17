package codegen

import (
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"strings"
	"testing"
)

func TestLexer_NewTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected lexer.TokenType
	}{
		{"struct", lexer.TokenStruct},
		{"type", lexer.TokenTypeDef},
		{"bool", lexer.TokenBool},
		{"true", lexer.TokenTrue},
		{"false", lexer.TokenFalse},
		{"for", lexer.TokenFor},
		{"%", lexer.TokenPercent},
		{"&&", lexer.TokenAnd},
		{"||", lexer.TokenOr},
		{"!", lexer.TokenNot},
		{";", lexer.TokenSemicolon},
	}
	for _, tt := range tests {
		l := lexer.New(tt.input)
		tok := l.NextToken()
		if tok.Type != tt.expected {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, tok.Type)
		}
	}
}

func TestLexer_LogicalOperators(t *testing.T) {
	l := lexer.New("a && b || !c")
	tokens := []lexer.TokenType{}
	for {
		tok := l.NextToken()
		if tok.Type == lexer.TokenEOF {
			break
		}
		tokens = append(tokens, tok.Type)
	}
	expected := []lexer.TokenType{
		lexer.TokenIdent, lexer.TokenAnd, lexer.TokenIdent,
		lexer.TokenOr, lexer.TokenNot, lexer.TokenIdent,
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, tok := range tokens {
		if tok != expected[i] {
			t.Errorf("token %d: expected %s, got %s", i, expected[i], tok)
		}
	}
}

func TestLexer_ModuloAndSemicolon(t *testing.T) {
	l := lexer.New("10 % 3; x = 1")
	tokens := []lexer.TokenType{}
	for {
		tok := l.NextToken()
		if tok.Type == lexer.TokenEOF {
			break
		}
		tokens = append(tokens, tok.Type)
	}
	expected := []lexer.TokenType{
		lexer.TokenInt, lexer.TokenPercent, lexer.TokenInt,
		lexer.TokenSemicolon,
		lexer.TokenIdent, lexer.TokenAssign, lexer.TokenInt,
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, tok := range tokens {
		if tok != expected[i] {
			t.Errorf("token %d: expected %s, got %s", i, expected[i], tok)
		}
	}
}

func TestGenStructDecl(t *testing.T) {
	g := New(Config{})
	node := &parser.StructDeclStmt{
		Name: "Person",
		Fields: []parser.StructField{
			{Name: "name", Type: "string"},
			{Name: "age", Type: "int"},
		},
	}
	result := g.genStructDecl(node)
	if !strings.Contains(result, "typedef struct {") {
		t.Errorf("expected typedef struct, got:\n%s", result)
	}
	if !strings.Contains(result, "char* name;") {
		t.Errorf("expected char* name;, got:\n%s", result)
	}
	if !strings.Contains(result, "int64_t age;") {
		t.Errorf("expected int64_t age;, got:\n%s", result)
	}
	if !strings.Contains(result, "} Person;") {
		t.Errorf("expected } Person;, got:\n%s", result)
	}
}

func TestGenForStmt(t *testing.T) {
	g := New(Config{})
	node := &parser.ForStmt{
		Init: &parser.VarDeclStmt{
			Name:  "i",
			Value: &parser.IntLiteral{Value: "0"},
			Type:  "int",
		},
		Condition: &parser.BinaryExpr{
			Left:     &parser.Identifier{Name: "i"},
			Operator: "<",
			Right:    &parser.IntLiteral{Value: "10"},
		},
		Post: &parser.ExprStmt{
			Expression: &parser.BinaryExpr{
				Left:     &parser.Identifier{Name: "i"},
				Operator: "=",
				Right: &parser.BinaryExpr{
					Left:     &parser.Identifier{Name: "i"},
					Operator: "+",
					Right:    &parser.IntLiteral{Value: "1"},
				},
			},
		},
		Body: []parser.Node{
			&parser.PrintStmt{Value: &parser.Identifier{Name: "i"}},
		},
	}
	result := g.genForStmt(node)
	if !strings.Contains(result, "for (") {
		t.Errorf("expected for (, got:\n%s", result)
	}
	if !strings.Contains(result, "is_truthy") {
		t.Errorf("expected is_truthy in condition, got:\n%s", result)
	}
}

func TestGenBoolLiteral(t *testing.T) {
	g := New(Config{})
	trueExpr := &parser.BoolLiteral{Value: true}
	falseExpr := &parser.BoolLiteral{Value: false}

	trueResult := g.genExpr(trueExpr)
	falseResult := g.genExpr(falseExpr)

	if trueResult != "make_int(1)" {
		t.Errorf("expected make_int(1), got %s", trueResult)
	}
	if falseResult != "make_int(0)" {
		t.Errorf("expected make_int(0), got %s", falseResult)
	}
}

func TestGenUnaryExpr(t *testing.T) {
	g := New(Config{})
	negExpr := &parser.UnaryExpr{
		Operator: "-",
		Operand:  &parser.IntLiteral{Value: "42"},
	}
	notExpr := &parser.UnaryExpr{
		Operator: "!",
		Operand:  &parser.BoolLiteral{Value: true},
	}

	negResult := g.genExpr(negExpr)
	notResult := g.genExpr(notExpr)

	if !strings.Contains(negResult, "karkain_negate") {
		t.Errorf("expected karkain_negate, got %s", negResult)
	}
	if !strings.Contains(notResult, "karkain_not") {
		t.Errorf("expected karkain_not, got %s", notResult)
	}
}

func TestGenLogicalOperators(t *testing.T) {
	g := New(Config{})
	andExpr := &parser.BinaryExpr{
		Left:     &parser.BoolLiteral{Value: true},
		Operator: "&&",
		Right:    &parser.BoolLiteral{Value: false},
	}
	orExpr := &parser.BinaryExpr{
		Left:     &parser.BoolLiteral{Value: true},
		Operator: "||",
		Right:    &parser.BoolLiteral{Value: false},
	}
	modExpr := &parser.BinaryExpr{
		Left:     &parser.IntLiteral{Value: "10"},
		Operator: "%",
		Right:    &parser.IntLiteral{Value: "3"},
	}

	andResult := g.genExpr(andExpr)
	orResult := g.genExpr(orExpr)
	modResult := g.genExpr(modExpr)

	if !strings.Contains(andResult, "is_truthy") {
		t.Errorf("expected is_truthy in &&, got %s", andResult)
	}
	if !strings.Contains(orResult, "is_truthy") {
		t.Errorf("expected is_truthy in ||, got %s", orResult)
	}
	if !strings.Contains(modResult, "karkain_mod") {
		t.Errorf("expected karkain_mod, got %s", modResult)
	}
}

func TestGenStructLiteral(t *testing.T) {
	g := New(Config{})
	node := &parser.StructLiteral{
		TypeName: "Person",
		Fields: []parser.Node{
			&parser.BinaryExpr{
				Left:     &parser.Identifier{Name: "name"},
				Operator: "=",
				Right:    &parser.StringLiteral{Value: "Alice"},
			},
			&parser.BinaryExpr{
				Left:     &parser.Identifier{Name: "age"},
				Operator: "=",
				Right:    &parser.IntLiteral{Value: "30"},
			},
		},
	}
	result := g.genStructLiteral(node)
	if !strings.Contains(result, "Person") {
		t.Errorf("expected Person struct type, got %s", result)
	}
	if !strings.Contains(result, "_s.name =") {
		t.Errorf("expected _s.name assignment, got %s", result)
	}
	if !strings.Contains(result, "_s.age =") {
		t.Errorf("expected _s.age assignment, got %s", result)
	}
}

func TestMapKarkainTypeToC_Bool(t *testing.T) {
	g := New(Config{})
	result := g.mapKarkainTypeToC("bool")
	if result != "int" {
		t.Errorf("expected int for bool, got %s", result)
	}
}

func TestParserStructDecl(t *testing.T) {
	input := `type Person struct { name string, age int }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	structDecl, ok := prog.Statements[0].(*parser.StructDeclStmt)
	if !ok {
		t.Fatalf("expected StructDeclStmt, got %T", prog.Statements[0])
	}
	if structDecl.Name != "Person" {
		t.Errorf("expected Person, got %s", structDecl.Name)
	}
	if len(structDecl.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(structDecl.Fields))
	}
}

func TestParserForLoop(t *testing.T) {
	input := `func main() { for (let i int = 0; i < 10; i = i + 1) { print(i) } }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	fn, ok := prog.Statements[0].(*parser.FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl, got %T", prog.Statements[0])
	}
	if len(fn.Body) != 1 {
		t.Fatalf("expected 1 body statement, got %d", len(fn.Body))
	}
	forStmt, ok := fn.Body[0].(*parser.ForStmt)
	if !ok {
		t.Fatalf("expected ForStmt, got %T", fn.Body[0])
	}
	if forStmt.Init == nil {
		t.Error("expected non-nil Init")
	}
	if forStmt.Condition == nil {
		t.Error("expected non-nil Condition")
	}
	if forStmt.Post == nil {
		t.Error("expected non-nil Post")
	}
	if len(forStmt.Body) != 1 {
		t.Errorf("expected 1 body statement, got %d", len(forStmt.Body))
	}
}

func TestParserBoolLiterals(t *testing.T) {
	input := `func main() { let x bool = true; let y bool = false }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	if len(fn.Body) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(fn.Body))
	}
	vd1 := fn.Body[0].(*parser.VarDeclStmt)
	vd2 := fn.Body[1].(*parser.VarDeclStmt)

	b1, ok := vd1.Value.(*parser.BoolLiteral)
	if !ok {
		t.Fatalf("expected BoolLiteral, got %T", vd1.Value)
	}
	if !b1.Value {
		t.Error("expected true")
	}

	b2, ok := vd2.Value.(*parser.BoolLiteral)
	if !ok {
		t.Fatalf("expected BoolLiteral, got %T", vd2.Value)
	}
	if b2.Value {
		t.Error("expected false")
	}
}

func TestParserUnaryNegation(t *testing.T) {
	input := `func main() { let x int = -5 }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	unary, ok := vd.Value.(*parser.UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", vd.Value)
	}
	if unary.Operator != "-" {
		t.Errorf("expected -, got %s", unary.Operator)
	}
}

func TestParserLogicalNot(t *testing.T) {
	input := `func main() { let x int = !true }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	unary, ok := vd.Value.(*parser.UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", vd.Value)
	}
	if unary.Operator != "!" {
		t.Errorf("expected !, got %s", unary.Operator)
	}
}

func TestParserModuloOperator(t *testing.T) {
	input := `func main() { let x int = 10 % 3 }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	bin, ok := vd.Value.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", vd.Value)
	}
	if bin.Operator != "%" {
		t.Errorf("expected %%, got %s", bin.Operator)
	}
}

func TestParserLogicalOperators(t *testing.T) {
	input := `func main() { let x int = true && false || !true }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	bin, ok := vd.Value.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", vd.Value)
	}
	if bin.Operator != "||" {
		t.Errorf("expected ||, got %s", bin.Operator)
	}
}

func TestParserStructLiteral(t *testing.T) {
	input := `func main() { let p Person = Person{name: "Alice", age: 30} }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	sl, ok := vd.Value.(*parser.StructLiteral)
	if !ok {
		t.Fatalf("expected StructLiteral, got %T", vd.Value)
	}
	if sl.TypeName != "Person" {
		t.Errorf("expected Person, got %s", sl.TypeName)
	}
	if len(sl.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(sl.Fields))
	}
}

func TestParserQuoteUnquote(t *testing.T) {
	input := `func main() { let x int = quote(1 + 2) }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	qe, ok := vd.Value.(*parser.QuoteExpr)
	if !ok {
		t.Fatalf("expected QuoteExpr, got %T", vd.Value)
	}
	if qe.Expr == nil {
		t.Error("expected non-nil Expr in QuoteExpr")
	}
}

func TestParserSendOperator(t *testing.T) {
	input := `func main() { ch <- 42 }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	es, ok := fn.Body[0].(*parser.ExprStmt)
	if !ok {
		t.Fatalf("expected ExprStmt, got %T", fn.Body[0])
	}
	se, ok := es.Expression.(*parser.SendExpr)
	if !ok {
		t.Fatalf("expected SendExpr, got %T", es.Expression)
	}
	if se.Channel == nil {
		t.Error("expected non-nil Channel")
	}
}

func TestParserErrorReporting(t *testing.T) {
	input := `func main() { 123 }`
	l := lexer.New(input)
	p := parser.New(l)
	p.ParseProgram()
	// Should not crash; may or may not produce errors depending on recovery
}

func TestParserDereference(t *testing.T) {
	input := `func main() { let x int = *ptr }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	deref, ok := vd.Value.(*parser.Dereference)
	if !ok {
		t.Fatalf("expected Dereference, got %T", vd.Value)
	}
	if deref.Operand == nil {
		t.Error("expected non-nil Operand")
	}
}

func TestGenHasKeyAndDelete(t *testing.T) {
	g := New(Config{})
	hasKeyCall := &parser.CallExpr{
		Function: "hasKey",
		Args:     []parser.Node{&parser.Identifier{Name: "m"}, &parser.StringLiteral{Value: "key"}},
	}
	deleteCall := &parser.CallExpr{
		Function: "delete",
		Args:     []parser.Node{&parser.Identifier{Name: "m"}, &parser.StringLiteral{Value: "key"}},
	}

	hasKeyResult := g.genExpr(hasKeyCall)
	deleteResult := g.genExpr(deleteCall)

	if !strings.Contains(hasKeyResult, "karkain_hasKey") {
		t.Errorf("expected karkain_hasKey, got %s", hasKeyResult)
	}
	if !strings.Contains(deleteResult, "karkain_delete") {
		t.Errorf("expected karkain_delete, got %s", deleteResult)
	}
}
