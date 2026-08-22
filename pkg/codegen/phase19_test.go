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

func TestParserBigIntLiteral(t *testing.T) {
	input := `func main() { let x = 42n }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	bigInt, ok := vd.Value.(*parser.BigIntLiteral)
	if !ok {
		t.Fatalf("expected BigIntLiteral, got %T", vd.Value)
	}
	if bigInt.Value != "42n" {
		t.Errorf("expected '42n', got %q", bigInt.Value)
	}
}

func TestParserBigFloatLiteral(t *testing.T) {
	input := `func main() { let pi = 3.14b }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	bigFloat, ok := vd.Value.(*parser.BigFloatLiteral)
	if !ok {
		t.Fatalf("expected BigFloatLiteral, got %T", vd.Value)
	}
	if bigFloat.Value != "3.14b" {
		t.Errorf("expected '3.14b', got %q", bigFloat.Value)
	}
}

func TestParserBigIntArithmetic(t *testing.T) {
	input := `func main() { let result = 999999999999999999999999999999n + 1n }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	bin, ok := vd.Value.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", vd.Value)
	}
	if bin.Operator != "+" {
		t.Errorf("expected +, got %s", bin.Operator)
	}
	left, ok := bin.Left.(*parser.BigIntLiteral)
	if !ok {
		t.Fatalf("expected BigIntLiteral on left, got %T", bin.Left)
	}
	if left.Value != "999999999999999999999999999999n" {
		t.Errorf("unexpected left value: %q", left.Value)
	}
	right, ok := bin.Right.(*parser.BigIntLiteral)
	if !ok {
		t.Fatalf("expected BigIntLiteral on right, got %T", bin.Right)
	}
	if right.Value != "1n" {
		t.Errorf("unexpected right value: %q", right.Value)
	}
}

func TestParserBigIntLargeNumber(t *testing.T) {
	input := `func main() { let x = 100000000000000000000000000000000000000000000000000000n }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	bigInt, ok := vd.Value.(*parser.BigIntLiteral)
	if !ok {
		t.Fatalf("expected BigIntLiteral, got %T", vd.Value)
	}
	if bigInt.Value != "100000000000000000000000000000000000000000000000000000n" {
		t.Errorf("unexpected value: %q", bigInt.Value)
	}
}

func TestParserMixedBigIntAndInt(t *testing.T) {
	input := `func main() { let x = 42n + 100 }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	bin, ok := vd.Value.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", vd.Value)
	}
	_, leftOk := bin.Left.(*parser.BigIntLiteral)
	_, rightOk := bin.Right.(*parser.IntLiteral)
	if !leftOk || !rightOk {
		t.Errorf("expected BigIntLiteral + IntLiteral, got %T + %T", bin.Left, bin.Right)
	}
}

func TestParserBorrowExpr(t *testing.T) {
	input := `func main() { let x = 42; let r &int = &x }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[1].(*parser.VarDeclStmt)
	if vd.Type != "&int" {
		t.Errorf("expected type '&int', got %q", vd.Type)
	}
	borrow, ok := vd.Value.(*parser.BorrowExpr)
	if !ok {
		t.Fatalf("expected BorrowExpr, got %T", vd.Value)
	}
	if borrow.Mutable {
		t.Error("expected immutable borrow")
	}
}

func TestParserMutableBorrowExpr(t *testing.T) {
	input := `func main() { let x = 42; let r &mut int = &mut x }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[1].(*parser.VarDeclStmt)
	if vd.Type != "&mut int" {
		t.Errorf("expected type '&mut int', got %q", vd.Type)
	}
	borrow, ok := vd.Value.(*parser.BorrowExpr)
	if !ok {
		t.Fatalf("expected BorrowExpr, got %T", vd.Value)
	}
	if !borrow.Mutable {
		t.Error("expected mutable borrow")
	}
}

func TestParserMoveExpr(t *testing.T) {
	input := `func main() { let a = 10; let b = move(a) }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[1].(*parser.VarDeclStmt)
	moveExpr, ok := vd.Value.(*parser.MoveExpr)
	if !ok {
		t.Fatalf("expected MoveExpr, got %T", vd.Value)
	}
	ident, ok := moveExpr.Operand.(*parser.Identifier)
	if !ok {
		t.Fatalf("expected Identifier in move, got %T", moveExpr.Operand)
	}
	if ident.Name != "a" {
		t.Errorf("expected 'a', got %q", ident.Name)
	}
}

func TestParserRawAccessRead(t *testing.T) {
	input := `func main() { let val = @raw(4096) }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	vd := fn.Body[0].(*parser.VarDeclStmt)
	raw, ok := vd.Value.(*parser.RawAccessExpr)
	if !ok {
		t.Fatalf("expected RawAccessExpr, got %T", vd.Value)
	}
	if raw.Value != nil {
		t.Error("expected nil Value for read-only @raw")
	}
}

func TestParserRawAccessWrite(t *testing.T) {
	input := `func main() { @raw(4096, 42) }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	exprStmt, ok := fn.Body[0].(*parser.ExprStmt)
	if !ok {
		t.Fatalf("expected ExprStmt, got %T", fn.Body[0])
	}
	raw, ok := exprStmt.Expression.(*parser.RawAccessExpr)
	if !ok {
		t.Fatalf("expected RawAccessExpr, got %T", exprStmt.Expression)
	}
	if raw.Value == nil {
		t.Error("expected non-nil Value for write @raw")
	}
}

func TestLexer_FatArrow(t *testing.T) {
	l := lexer.New("=>")
	tok := l.NextToken()
	if tok.Type != lexer.TokenFatArrow {
		t.Errorf("expected TokenFatArrow, got %s", tok.Type)
	}
}

func TestLexer_Phase42Keywords(t *testing.T) {
	tests := []struct {
		input    string
		expected lexer.TokenType
	}{
		{"Some", lexer.TokenSome},
		{"None", lexer.TokenNone},
		{"Ok", lexer.TokenOk},
		{"Err", lexer.TokenErr},
		{"match", lexer.TokenMatch},
		{"linear", lexer.TokenLinear},
		{"packed", lexer.TokenPacked},
	}
	for _, tt := range tests {
		l := lexer.New(tt.input)
		tok := l.NextToken()
		if tok.Type != tt.expected {
			t.Errorf("input %q: expected %s, got %s", tt.input, tt.expected, tok.Type)
		}
	}
}

func TestParserOptionSomeExpr(t *testing.T) {
	input := `func main() { let x = Some(42) }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	stmt := fn.Body[0].(*parser.VarDeclStmt)
	some, ok := stmt.Value.(*parser.OptionSomeExpr)
	if !ok {
		t.Fatalf("expected OptionSomeExpr, got %T", stmt.Value)
	}
	lit, ok := some.Value.(*parser.IntLiteral)
	if !ok {
		t.Fatalf("expected IntLiteral, got %T", some.Value)
	}
	if lit.Value != "42" {
		t.Errorf("expected 42, got %s", lit.Value)
	}
}

func TestParserOptionNoneExpr(t *testing.T) {
	input := `func main() { let x = None }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	stmt := fn.Body[0].(*parser.VarDeclStmt)
	_, ok := stmt.Value.(*parser.OptionNoneExpr)
	if !ok {
		t.Fatalf("expected OptionNoneExpr, got %T", stmt.Value)
	}
}

func TestParserResultOkExpr(t *testing.T) {
	input := `func main() { let r = Ok("hello") }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	stmt := fn.Body[0].(*parser.VarDeclStmt)
	okExpr, ok := stmt.Value.(*parser.ResultOkExpr)
	if !ok {
		t.Fatalf("expected ResultOkExpr, got %T", stmt.Value)
	}
	strLit, ok := okExpr.Value.(*parser.StringLiteral)
	if !ok {
		t.Fatalf("expected StringLiteral, got %T", okExpr.Value)
	}
	if strLit.Value != "hello" {
		t.Errorf("expected hello, got %s", strLit.Value)
	}
}

func TestParserResultErrExpr(t *testing.T) {
	input := `func main() { let e = Err("fail") }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	stmt := fn.Body[0].(*parser.VarDeclStmt)
	errExpr, ok := stmt.Value.(*parser.ResultErrExpr)
	if !ok {
		t.Fatalf("expected ResultErrExpr, got %T", stmt.Value)
	}
	strLit, ok := errExpr.Error.(*parser.StringLiteral)
	if !ok {
		t.Fatalf("expected StringLiteral, got %T", errExpr.Error)
	}
	if strLit.Value != "fail" {
		t.Errorf("expected fail, got %s", strLit.Value)
	}
}

func TestParserMatchExpr(t *testing.T) {
	input := `func main() { let x = 5; let v = match x { 5 => 10, _ => 0 } }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	stmt := fn.Body[1].(*parser.VarDeclStmt)
	match, ok := stmt.Value.(*parser.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr, got %T", stmt.Value)
	}
	if len(match.Arms) != 2 {
		t.Fatalf("expected 2 arms, got %d", len(match.Arms))
	}
	if match.Arms[0].Pattern.Type != "literal" {
		t.Errorf("expected literal pattern, got %s", match.Arms[0].Pattern.Type)
	}
	if match.Arms[1].Pattern.Type != "wildcard" {
		t.Errorf("expected wildcard pattern, got %s", match.Arms[1].Pattern.Type)
	}
}

func TestParserMatchWithBinding(t *testing.T) {
	input := `func main() { let v = match 42 { x => x } }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	stmt := fn.Body[0].(*parser.VarDeclStmt)
	match, ok := stmt.Value.(*parser.MatchExpr)
	if !ok {
		t.Fatalf("expected MatchExpr, got %T", stmt.Value)
	}
	if len(match.Arms) != 1 {
		t.Fatalf("expected 1 arm, got %d", len(match.Arms))
	}
	if match.Arms[0].Pattern.Type != "binding" {
		t.Errorf("expected binding pattern, got %s", match.Arms[0].Pattern.Type)
	}
	if match.Arms[0].Pattern.Binding != "x" {
		t.Errorf("expected binding 'x', got '%s'", match.Arms[0].Pattern.Binding)
	}
}

func TestParserSIMDBuiltin(t *testing.T) {
	input := `func main() { let r = @simd_add(1, 2) }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()

	fn := prog.Statements[0].(*parser.FuncDecl)
	stmt := fn.Body[0].(*parser.VarDeclStmt)
	simd, ok := stmt.Value.(*parser.SIMDBuiltinExpr)
	if !ok {
		t.Fatalf("expected SIMDBuiltinExpr, got %T", stmt.Value)
	}
	if simd.Op != "add" {
		t.Errorf("expected add, got %s", simd.Op)
	}
	if len(simd.Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(simd.Args))
	}
}

func TestGenMatchExpr(t *testing.T) {
	g := New(Config{})
	matchNode := &parser.MatchExpr{
		Value: &parser.Identifier{Name: "x"},
		Arms: []parser.MatchArm{
			{
				Pattern: parser.MatchPattern{Type: "literal", Value: &parser.IntLiteral{Value: "5"}},
				Body:    &parser.IntLiteral{Value: "10"},
			},
			{
				Pattern: parser.MatchPattern{Type: "wildcard"},
				Body:    &parser.IntLiteral{Value: "0"},
			},
		},
	}
	cCode := g.genExpr(matchNode)

	if !strings.Contains(cCode, "_match_val") {
		t.Error("expected _match_val in generated C")
	}
	if !strings.Contains(cCode, "_match_result") {
		t.Error("expected _match_result in generated C")
	}
	if !strings.Contains(cCode, "is_truthy") {
		t.Error("expected is_truthy comparison in generated C")
	}
}

func TestGenOptionSomeNone(t *testing.T) {
	g := New(Config{})
	someExpr := &parser.OptionSomeExpr{Value: &parser.IntLiteral{Value: "42"}}
	noneExpr := &parser.OptionNoneExpr{}

	someResult := g.genExpr(someExpr)
	noneResult := g.genExpr(noneExpr)

	if someResult != "make_int(42)" {
		t.Errorf("expected make_int(42), got %s", someResult)
	}
	if noneResult != "make_int(0)" {
		t.Errorf("expected make_int(0), got %s", noneResult)
	}
}

func TestParserTypedParams(t *testing.T) {
	input := `func add(a int, b int) { print(a + b) } func main() { add(3, 7) }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(prog.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Statements))
	}
	fn, ok := prog.Statements[0].(*parser.FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl, got %T", prog.Statements[0])
	}
	if len(fn.Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(fn.Params))
	}
	if fn.Params[0] != "a" || fn.ParamTypes[0] != "int" {
		t.Errorf("param 0: expected (a, int), got (%s, %s)", fn.Params[0], fn.ParamTypes[0])
	}
	if fn.Params[1] != "b" || fn.ParamTypes[1] != "int" {
		t.Errorf("param 1: expected (b, int), got (%s, %s)", fn.Params[1], fn.ParamTypes[1])
	}
}

func TestParserEnumMatchPattern(t *testing.T) {
	input := `enum Color { Red, Green, Blue } func main() { let c = Color.Green; let label = match c { Color.Red => 1, Color.Green => 2, Color.Blue => 3, _ => 0 } }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(prog.Statements) != 2 {
		t.Fatalf("expected 2 statements (enum + func), got %d", len(prog.Statements))
	}
	enumDecl, ok := prog.Statements[0].(*parser.EnumDecl)
	if !ok {
		t.Fatalf("expected EnumDecl, got %T", prog.Statements[0])
	}
	if enumDecl.Name != "Color" {
		t.Errorf("expected enum name Color, got %s", enumDecl.Name)
	}
	if len(enumDecl.Variants) != 3 {
		t.Errorf("expected 3 variants, got %d", len(enumDecl.Variants))
	}
}

func TestParserForInLoop(t *testing.T) {
	input := `func main() { let arr = [10, 20, 30]; let sum = 0; for x in arr { sum = sum + x } }`
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
	// Body should have: let arr = ..., let sum = 0, for x in arr { ... }
	found := false
	for _, stmt := range fn.Body {
		if forIn, ok := stmt.(*parser.ForInStmt); ok {
			found = true
			if forIn.VarName != "x" {
				t.Errorf("expected iterator var 'x', got '%s'", forIn.VarName)
			}
			if len(forIn.Body) != 1 {
				t.Errorf("expected 1 body statement, got %d", len(forIn.Body))
			}
		}
	}
	if !found {
		t.Error("expected ForInStmt in function body")
	}
}

func TestParserBreakContinue(t *testing.T) {
	input := `func main() { let i = 0; while (i < 10) { i = i + 1; if (i == 3) { continue } if (i == 5) { break } } }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	fn, ok := prog.Statements[0].(*parser.FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl, got %T", prog.Statements[0])
	}
	var whileStmt *parser.WhileStmt
	for _, stmt := range fn.Body {
		if w, ok := stmt.(*parser.WhileStmt); ok {
			whileStmt = w
		}
	}
	if whileStmt == nil {
		t.Fatal("expected WhileStmt in function body")
	}
	if len(whileStmt.Body) != 3 {
		t.Errorf("expected 3 body statements in while (assign, if-continue, if-break), got %d", len(whileStmt.Body))
	}
	foundBreak := false
	foundContinue := false
	for _, stmt := range whileStmt.Body {
		if ifStmt, ok := stmt.(*parser.IfStmt); ok {
			for _, c := range ifStmt.Consequence {
				if _, ok := c.(*parser.BreakStmt); ok {
					foundBreak = true
				}
				if _, ok := c.(*parser.ContinueStmt); ok {
					foundContinue = true
				}
			}
		}
	}
	if !foundBreak {
		t.Error("expected BreakStmt inside if in while body")
	}
	if !foundContinue {
		t.Error("expected ContinueStmt inside if in while body")
	}
}

func TestParserLambda(t *testing.T) {
	input := `func main() { let add = fn(a, b) { return a + b }; print(add(1, 2)) }`
	l := lexer.New(input)
	p := parser.New(l)
	prog := p.ParseProgram()
	fn, ok := prog.Statements[0].(*parser.FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl, got %T", prog.Statements[0])
	}
	if len(fn.Body) != 2 {
		t.Fatalf("expected 2 body statements, got %d", len(fn.Body))
	}
	// Lambda desugars to FuncDecl via let x = fn(...)
	lambdaFn, ok := fn.Body[0].(*parser.VarDeclStmt)
	if !ok {
		t.Fatalf("expected VarDeclStmt for lambda, got %T", fn.Body[0])
	}
	if lambdaFn.Name != "add" {
		t.Errorf("expected lambda name 'add', got '%s'", lambdaFn.Name)
	}
	decl, ok := lambdaFn.Value.(*parser.FuncDecl)
	if !ok {
		t.Fatalf("expected FuncDecl inside VarDeclStmt, got %T", lambdaFn.Value)
	}
	if len(decl.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(decl.Params))
	}
}

func TestBreakContinueCodegen(t *testing.T) {
	g := New(Config{})
	breakNode := &parser.BreakStmt{}
	continueNode := &parser.ContinueStmt{}
	breakCode := g.genStatement(breakNode)
	continueCode := g.genStatement(continueNode)
	if !strings.Contains(breakCode, "break;") {
		t.Errorf("expected 'break;' in generated C, got %s", breakCode)
	}
	if !strings.Contains(continueCode, "continue;") {
		t.Errorf("expected 'continue;' in generated C, got %s", continueCode)
	}
}

func TestWhileCodegen(t *testing.T) {
	g := New(Config{})
	whileNode := &parser.WhileStmt{
		Condition: &parser.BinaryExpr{
			Left:     &parser.Identifier{Name: "i"},
			Operator: "<",
			Right:    &parser.IntLiteral{Value: "10"},
		},
		Body: []parser.Node{
			&parser.BreakStmt{},
		},
	}
	code := g.genStatement(whileNode)
	if !strings.Contains(code, "while (is_truthy(binary_op(i, \"<\", make_int(10))))") {
		t.Errorf("unexpected while condition: %s", code)
	}
	if !strings.Contains(code, "break;") {
		t.Errorf("expected break in while body: %s", code)
	}
}
