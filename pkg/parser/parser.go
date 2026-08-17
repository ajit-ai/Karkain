package parser

import (
	"fmt"
	"karkain/pkg/lexer"
	"strings"
)

// ...

const (
	TokenLet lexer.TokenType = "LET"
)

// ...
type Parser struct {
	l         *lexer.Lexer
	curToken  lexer.Token
	peekToken lexer.Token
	input     string   // Store input for C import parsing
	Errors    []string // Phase 19: Parser error tracking
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, input: l.GetInput(), Errors: []string{}}
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) addError(msg string) {
	p.Errors = append(p.Errors, fmt.Sprintf("line %d: %s", p.curToken.Line, msg))
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseProgram() *Program {
	prog := &Program{Statements: []Node{}, CImports: []*CImportBlock{}}
	for p.curToken.Type != lexer.TokenEOF {
		if p.curToken.Type == lexer.TokenFunc {
			if stmt := p.parseFunc(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenActor {
			if stmt := p.parseActor(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenMacro {
			if stmt := p.parseMacro(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenKernel {
		if stmt := p.parseKernel(); stmt != nil {
			prog.Statements = append(prog.Statements, stmt)
		}
	} else if p.curToken.Type == lexer.TokenTypeDef {
		if stmt := p.parseStructDecl(); stmt != nil {
			prog.Statements = append(prog.Statements, stmt)
		}
	} else if p.curToken.Type == lexer.TokenImport {
			if cImport := p.parseCImport(); cImport != nil {
				prog.CImports = append(prog.CImports, cImport)
			}
		} else {
			p.addError(fmt.Sprintf("unexpected token '%s' at top level", p.curToken.Literal))
			p.nextToken()
		}
	}
	return prog
}

func (p *Parser) parseFunc() *FuncDecl {
	p.nextToken() // consume 'func'
	fn := &FuncDecl{Name: p.curToken.Literal, Body: []Node{}}

	p.nextToken() // consume fn name
	p.nextToken() // consume '('

	// Parse parameters
	for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
		if p.curToken.Type == lexer.TokenIdent {
			paramName := p.curToken.Literal
			fn.Params = append(fn.Params, paramName)
			p.nextToken() // consume param name
			if p.curToken.Type == lexer.TokenComma {
				p.nextToken() // consume ','
			}
		} else {
			p.nextToken()
		}
	}
	p.nextToken() // consume ')'
	p.nextToken() // consume '{'

	fn.Body = p.parseBlock()
	p.nextToken() // consume '}'
	return fn
}

func (p *Parser) parseVarDecl() *VarDeclStmt {
	p.nextToken() // consume 'let' or 'var'
	name := p.curToken.Literal

	p.nextToken() // consume identifier

	// Check for type annotation (e.g., `var x int = 42`)
	var typeName string
	if p.curToken.Type == lexer.TokenStar {
		typeName = "*" // Pointer type
		p.nextToken()  // consume '*'
		if p.curToken.Type == lexer.TokenIdent {
			typeName += p.curToken.Literal
			p.nextToken() // consume base type
		}
	} else if p.curToken.Type == lexer.TokenIdent {
		typeName = p.curToken.Literal
		p.nextToken() // consume type name
	} else if p.curToken.Type == lexer.TokenBool {
		typeName = p.curToken.Literal
		p.nextToken() // consume bool type
	}

	p.nextToken() // consume '='

	// Check if the value is a matrix declaration
	if p.curToken.Type == lexer.TokenMatrix {
		matrixDecl := p.parseMatrixDeclInternal()
		return &VarDeclStmt{Name: name, Value: matrixDecl, IsMatrix: true}
	}

	val := p.parseExpr()

	// If we have a type, store it in the variable declaration
	if typeName != "" {
		return &VarDeclStmt{Name: name, Value: val, Type: typeName}
	}

	return &VarDeclStmt{Name: name, Value: val}
}

func (p *Parser) parsePrint() *PrintStmt {
	p.nextToken() // consume 'print'
	p.nextToken() // consume '('
	val := p.parseExpr()
	p.nextToken() // consume ')'
	return &PrintStmt{Value: val}
}

func (p *Parser) parseBlock() []Node {
	stmts := []Node{}
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		stmt := p.parseStatement()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}
	return stmts
}

func (p *Parser) parseStatement() Node {
	switch p.curToken.Type {
	case lexer.TokenLet, lexer.TokenVar:
		return p.parseVarDecl()
	case lexer.TokenPrint:
		return p.parsePrint()
	case lexer.TokenReturn:
		return p.parseReturn()
	case lexer.TokenIf:
		return p.parseIf()
	case lexer.TokenWhile:
		return p.parseWhile()
	case lexer.TokenFor:
		return p.parseFor()
	case lexer.TokenMatrix:
		return p.parseMatrixDecl()
	case lexer.TokenAlloc:
		return &ExprStmt{Expression: p.parseAlloc()}
	case lexer.TokenFree:
		return &ExprStmt{Expression: p.parseFree()}
	case lexer.TokenAddr:
		return &ExprStmt{Expression: p.parseAddrOf()}
	case lexer.TokenQReg:
		return p.parseQRegDecl()
	case lexer.TokenGate:
		return p.parseGateApply()
	case lexer.TokenMeasure:
		return &ExprStmt{Expression: p.parseMeasure()}
	case lexer.TokenMacro:
		return p.parseMacro()
	case lexer.TokenComptime:
		return p.parseComptimeStmt()
	case lexer.TokenSpawn:
		return &ExprStmt{Expression: p.parseSpawn()}
	case lexer.TokenReceive:
		return p.parseReceive()
	case lexer.TokenBarrier:
		return p.parseBarrierStmt()
	case lexer.TokenIdent:
		return p.parseIdentStatement()
	case lexer.TokenImport:
		// Skip import blocks inside functions (they're handled at top level)
		cImport := p.parseCImport()
		if cImport != nil {
			return &ExprStmt{Expression: &StringLiteral{Value: cImport.Content}}
		}
		return nil
	default:
		p.addError(fmt.Sprintf("unexpected token '%s' (%s)", p.curToken.Literal, p.curToken.Type))
		p.nextToken()
		return nil
	}
}

func (p *Parser) parseReturn() *ReturnStmt {
	p.nextToken() // consume 'return'
	val := p.parseExpr()
	return &ReturnStmt{Value: val}
}

func (p *Parser) parseWhile() *WhileStmt {
	p.nextToken() // consume 'while'
	p.nextToken() // consume '('
	condition := p.parseExpr()
	p.nextToken() // consume ')'
	p.nextToken() // consume '{'
	body := p.parseBlock()
	p.nextToken() // consume '}'
	return &WhileStmt{Condition: condition, Body: body}
}

// Phase 19: for (init; cond; post) { body }
func (p *Parser) parseFor() *ForStmt {
	p.nextToken() // consume 'for'
	p.nextToken() // consume '('

	// Parse init (can be empty)
	var init Node
	if p.curToken.Type != lexer.TokenSemicolon {
		init = p.parseStatement()
	}
	// Consume ';' after init
	if p.curToken.Type == lexer.TokenSemicolon {
		p.nextToken()
	}

	// Parse condition
	condition := p.parseExpr()
	// Consume ';' after condition
	if p.curToken.Type == lexer.TokenSemicolon {
		p.nextToken()
	}

	// Parse post (can be empty)
	var post Node
	if p.curToken.Type != lexer.TokenRParen {
		post = p.parseExpr()
	}
	if p.curToken.Type == lexer.TokenRParen {
		p.nextToken() // consume ')'
	}
	if p.curToken.Type == lexer.TokenLBrace {
		p.nextToken() // consume '{'
	}
	body := p.parseBlock()
	p.nextToken() // consume '}'

	return &ForStmt{Init: init, Condition: condition, Post: post, Body: body}
}

func (p *Parser) parseIf() *IfStmt {
	p.nextToken() // consume 'if'
	p.nextToken() // consume '('
	condition := p.parseExpr()
	p.nextToken() // consume ')'
	p.nextToken() // consume '{'
	consequence := p.parseBlock()
	p.nextToken() // consume '}'

	var alternative []Node
	if p.curToken.Type == lexer.TokenElse {
		p.nextToken() // consume 'else'
		if p.curToken.Type == lexer.TokenIf {
			// else if -> wrap in a single-statement alternative
			elseIf := p.parseIf()
			alternative = []Node{elseIf}
		} else {
			p.nextToken() // consume '{'
			alternative = p.parseBlock()
			p.nextToken() // consume '}'
		}
	}

	return &IfStmt{
		Condition:   condition,
		Consequence: consequence,
		Alternative: alternative,
	}
}

func (p *Parser) parseIdentStatement() Node {
	ident := p.curToken.Literal
	p.nextToken() // consume identifier

	// Check for matrix index assignment
	if p.curToken.Type == lexer.TokenLBracket {
		p.nextToken() // consume '['
		row := p.parseExpr()
		p.nextToken() // consume ','
		col := p.parseExpr()
		p.nextToken() // consume ']'

		if p.curToken.Type == lexer.TokenAssign {
			p.nextToken() // consume '='
			val := p.parseExpr()
			assignExpr := &BinaryExpr{
				Left: &MatrixIndexExpr{
					Matrix: &Identifier{Name: ident},
					Row:    row,
					Col:    col,
				},
				Operator: "=",
				Right:    val,
			}
			return &ExprStmt{Expression: assignExpr}
		}
	}

	// Simple variable assignment (reassignment, not declaration)
	if p.curToken.Type == lexer.TokenAssign {
		p.nextToken() // consume '='
		val := p.parseExpr()
		assignExpr := &BinaryExpr{
			Left:     &Identifier{Name: ident},
			Operator: "=",
			Right:    val,
		}
		return &ExprStmt{Expression: assignExpr}
	}

	// Otherwise it's an expression statement
	// We already consumed the ident, so we need to reconstruct
	// Handle function calls and other expressions starting with ident
	left := &Identifier{Name: ident}

	// Check for function call
	if p.curToken.Type == lexer.TokenLParen {
		p.nextToken() // consume '('
		args := []Node{}
		for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
			args = append(args, p.parseExpr())
			if p.curToken.Type == lexer.TokenComma {
				p.nextToken()
			}
		}
		p.nextToken() // consume ')'
		return &ExprStmt{Expression: &CallExpr{Function: ident, Args: args}}
	}

	// Check for dot expression
	if p.curToken.Type == lexer.TokenDot {
		p.nextToken() // consume '.'
		rightIdent := p.curToken.Literal
		p.nextToken() // consume right identifier
		if p.curToken.Type == lexer.TokenLParen {
			p.nextToken() // consume '('
			args := []Node{}
			for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
				args = append(args, p.parseExpr())
				if p.curToken.Type == lexer.TokenComma {
					p.nextToken()
				}
			}
			p.nextToken() // consume ')'
			return &ExprStmt{Expression: &CallExpr{
				Function: ident + "." + rightIdent,
				Args:     args,
				IsCFunc:  ident == "C",
			}}
		}
		return &ExprStmt{Expression: &DotExpr{Left: left, Right: rightIdent}}
	}

	// Check for binary operator (assignment via expression)
	if p.curToken.Type == lexer.TokenPlus || p.curToken.Type == lexer.TokenMinus ||
		p.curToken.Type == lexer.TokenStar || p.curToken.Type == lexer.TokenSlash ||
		p.curToken.Type == lexer.TokenPercent || p.curToken.Type == lexer.TokenEqual ||
		p.curToken.Type == lexer.TokenNotEqual ||
		p.curToken.Type == lexer.TokenLessThan || p.curToken.Type == lexer.TokenGreaterThan ||
		p.curToken.Type == lexer.TokenLessEqual || p.curToken.Type == lexer.TokenGreaterEqual ||
		p.curToken.Type == lexer.TokenAnd || p.curToken.Type == lexer.TokenOr {
		return &ExprStmt{Expression: p.parseBinaryExpr(left, 0)}
	}

	// Phase 19: Handle send operator: channel <- message
	if p.curToken.Type == lexer.TokenSend {
		p.nextToken() // consume '<-'
		message := p.parseExpr()
		return &ExprStmt{Expression: &SendExpr{Channel: left, Message: message}}
	}

	return &ExprStmt{Expression: left}
}

func (p *Parser) parseExpr() Node {
	return p.parseBinaryExpr(nil, 0)
}

func precedence(op string) int {
	switch op {
	case "||":
		return 0
	case "&&":
		return 1
	case "==", "!=", "<", ">", "<=", ">=":
		return 2
	case "+", "-":
		return 3
	case "*", "/", "%":
		return 4
	default:
		return -1
	}
}

func (p *Parser) parseBinaryExpr(left Node, minPrec int) Node {
	if left == nil {
		left = p.parseUnary()
	}

	for {
		op := ""
		switch p.curToken.Type {
		case lexer.TokenPlus:
			op = "+"
		case lexer.TokenMinus:
			op = "-"
		case lexer.TokenStar:
			op = "*"
		case lexer.TokenSlash:
			op = "/"
		case lexer.TokenPercent:
			op = "%"
		case lexer.TokenEqual:
			op = "=="
		case lexer.TokenNotEqual:
			op = "!="
		case lexer.TokenLessThan:
			op = "<"
		case lexer.TokenGreaterThan:
			op = ">"
		case lexer.TokenLessEqual:
			op = "<="
		case lexer.TokenGreaterEqual:
			op = ">="
		case lexer.TokenAnd:
			op = "&&"
		case lexer.TokenOr:
			op = "||"
		}

		if op == "" || precedence(op) < minPrec {
			break
		}

		p.nextToken() // consume operator
		right := p.parseBinaryExpr(nil, precedence(op)+1)
		left = &BinaryExpr{Left: left, Operator: op, Right: right}
	}

	// Handle assignment: ident = expr (only at precedence 0)
	if p.curToken.Type == lexer.TokenAssign && left != nil {
		p.nextToken() // consume '='
		right := p.parseExpr()
		return &BinaryExpr{Left: left, Operator: "=", Right: right}
	}

	// Handle index expression [index] or matrix index [row, col]
	if p.curToken.Type == lexer.TokenLBracket {
		p.nextToken() // consume '['
		first := p.parseExpr()
		if p.curToken.Type == lexer.TokenComma {
			// Matrix index: [row, col]
			p.nextToken() // consume ','
			col := p.parseExpr()
			p.nextToken() // consume ']'
			left = &MatrixIndexExpr{Matrix: left, Row: first, Col: col}
		} else {
			// Array/map index: [index]
			p.nextToken() // consume ']'
			left = &IndexExpr{Left: left, Index: first}
		}
	}

	// Phase 19: Handle @derive(Trait) and @tag(name, value) as postfix operators
	if p.curToken.Type == lexer.TokenAt {
		p.nextToken() // consume '@'
		ident := p.curToken.Literal
		p.nextToken() // consume identifier
		if ident == "derive" && p.curToken.Type == lexer.TokenLParen {
			p.nextToken() // consume '('
			trait := p.curToken.Literal
			p.nextToken() // consume trait name
			p.nextToken() // consume ')'
			left = &DeriveExpr{Trait: trait, Target: left}
		} else if ident == "tag" && p.curToken.Type == lexer.TokenLParen {
			p.nextToken() // consume '('
			tagName := p.curToken.Literal
			p.nextToken() // consume tag name
			p.nextToken() // consume ','
			tagValue := p.curToken.Literal
			p.nextToken() // consume tag value
			p.nextToken() // consume ')'
			left = &TagExpr{Target: left, TagName: tagName, TagValue: tagValue}
		}
	}

	return left
}

func (p *Parser) parseUnary() Node {
	// Phase 19: Handle unary negation (-x), logical NOT (!x), and dereference (*ptr)
	if p.curToken.Type == lexer.TokenMinus {
		p.nextToken() // consume '-'
		operand := p.parseUnary()
		return &UnaryExpr{Operator: "-", Operand: operand}
	}
	if p.curToken.Type == lexer.TokenNot {
		p.nextToken() // consume '!'
		operand := p.parseUnary()
		return &UnaryExpr{Operator: "!", Operand: operand}
	}
	if p.curToken.Type == lexer.TokenStar {
		p.nextToken() // consume '*'
		operand := p.parseUnary()
		return &Dereference{Operand: operand}
	}
	return p.parsePrimaryExpr()
}

func (p *Parser) parsePrimaryExpr() Node {
	switch p.curToken.Type {
	case lexer.TokenString:
		val := p.curToken.Literal
		p.nextToken()
		return &StringLiteral{Value: val}
	case lexer.TokenInt:
		val := p.curToken.Literal
		p.nextToken()
		return &IntLiteral{Value: val}
	case lexer.TokenFloat64:
		val := p.curToken.Literal
		p.nextToken()
		return &Float64Literal{Value: val}
	case lexer.TokenIdent:
		return p.parseIdentExpr()
	case lexer.TokenLParen:
		p.nextToken() // consume '('
		expr := p.parseExpr()
		p.nextToken() // consume ')'
		return expr
	case lexer.TokenLBracket:
		return p.parseArrayLiteral()
	case lexer.TokenLBrace:
		return p.parseMapLiteral()
	case lexer.TokenGlobalID:
		return p.parseGlobalIDExpr()
	case lexer.TokenTrue:
		p.nextToken()
		return &BoolLiteral{Value: true}
	case lexer.TokenFalse:
		p.nextToken()
		return &BoolLiteral{Value: false}
	case lexer.TokenQuote:
		p.nextToken() // consume 'quote'
		p.nextToken() // consume '('
		expr := p.parseExpr()
		p.nextToken() // consume ')'
		return &QuoteExpr{Expr: expr}
	case lexer.TokenUnquote:
		p.nextToken() // consume 'unquote'
		p.nextToken() // consume '('
		expr := p.parseExpr()
		p.nextToken() // consume ')'
		return &UnquoteExpr{Expr: expr}
	}
	return nil
}

func (p *Parser) parseArrayLiteral() *ArrayLiteral {
	p.nextToken() // consume '['
	elements := []Node{}
	if p.curToken.Type != lexer.TokenRBracket {
		elements = append(elements, p.parseExpr())
		for p.curToken.Type == lexer.TokenComma {
			p.nextToken() // consume ','
			elements = append(elements, p.parseExpr())
		}
	}
	p.nextToken() // consume ']'
	return &ArrayLiteral{Elements: elements}
}

func (p *Parser) parseMapLiteral() *MapLiteral {
	p.nextToken() // consume '{'
	keys := []Node{}
	values := []Node{}
	if p.curToken.Type != lexer.TokenRBrace {
		key := p.parseExpr()
		p.nextToken() // consume ':'
		val := p.parseExpr()
		keys = append(keys, key)
		values = append(values, val)
		for p.curToken.Type == lexer.TokenComma {
			p.nextToken() // consume ','
			key = p.parseExpr()
			p.nextToken() // consume ':'
			val = p.parseExpr()
			keys = append(keys, key)
			values = append(values, val)
		}
	}
	p.nextToken() // consume '}'
	return &MapLiteral{Keys: keys, Values: values}
}

func (p *Parser) parseIdentExpr() Node {
	ident := p.curToken.Literal
	p.nextToken() // consume identifier

	// Check for dot expression (e.g., C.sqrt, matrix.method)
	if p.curToken.Type == lexer.TokenDot {
		p.nextToken() // consume '.'
		rightIdent := p.curToken.Literal
		p.nextToken() // consume right identifier

		// Check if this is a function call
		if p.curToken.Type == lexer.TokenLParen {
			p.nextToken() // consume '('
			args := []Node{}
			for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
				args = append(args, p.parseExpr())
				if p.curToken.Type == lexer.TokenComma {
					p.nextToken()
				}
			}
			p.nextToken() // consume ')'

			return &CallExpr{
				Function: ident + "." + rightIdent,
				Args:     args,
				IsCFunc:  ident == "C",
			}
		}

		return &DotExpr{Left: &Identifier{Name: ident}, Right: rightIdent}
	}

	// Check for function call
	if p.curToken.Type == lexer.TokenLParen {
		p.nextToken() // consume '('
		args := []Node{}
		for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
			args = append(args, p.parseExpr())
			if p.curToken.Type == lexer.TokenComma {
				p.nextToken()
			}
		}
		p.nextToken() // consume ')'

		return &CallExpr{
			Function: ident,
			Args:     args,
			IsCFunc:  false,
		}
	}

	// Phase 19: Check for struct literal: TypeName{field: val, ...}
	if p.curToken.Type == lexer.TokenLBrace {
		return p.parseStructLiteral(ident)
	}

	return &Identifier{Name: ident}
}

// Phase 19: Struct literal parsing: Name{field: val, ...}
func (p *Parser) parseStructLiteral(typeName string) *StructLiteral {
	p.nextToken() // consume '{'
	fields := []Node{}
	if p.curToken.Type != lexer.TokenRBrace {
		// Parse field: value pairs
		fieldName := p.curToken.Literal
		p.nextToken() // consume field name
		p.nextToken() // consume ':'
		val := p.parseExpr()
		fields = append(fields, &BinaryExpr{
			Left:     &Identifier{Name: fieldName},
			Operator: "=",
			Right:    val,
		})
		for p.curToken.Type == lexer.TokenComma {
			p.nextToken() // consume ','
			fieldName = p.curToken.Literal
			p.nextToken() // consume field name
			p.nextToken() // consume ':'
			val = p.parseExpr()
			fields = append(fields, &BinaryExpr{
				Left:     &Identifier{Name: fieldName},
				Operator: "=",
				Right:    val,
			})
		}
	}
	p.nextToken() // consume '}'
	return &StructLiteral{TypeName: typeName, Fields: fields}
}

// Phase 19: Struct declaration parsing: type Name struct { field type, ... }
func (p *Parser) parseStructDecl() *StructDeclStmt {
	p.nextToken() // consume 'type'
	name := p.curToken.Literal
	p.nextToken() // consume struct name
	p.nextToken() // consume 'struct'
	p.nextToken() // consume '{'

	fields := []StructField{}
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		fieldName := p.curToken.Literal
		p.nextToken() // consume field name
		fieldType := p.curToken.Literal
		p.nextToken() // consume field type
		fields = append(fields, StructField{Name: fieldName, Type: fieldType})
		if p.curToken.Type == lexer.TokenComma {
			p.nextToken() // consume ','
		}
	}
	p.nextToken() // consume '}'

	return &StructDeclStmt{Name: name, Fields: fields}
}

// Phase 11: Native C Interop parsing
func (p *Parser) parseCImport() *CImportBlock {
	p.nextToken() // consume 'import'

	// Expect string literal "C"
	if p.curToken.Type != lexer.TokenString || p.curToken.Literal != "C" {
		return nil
	}
	p.nextToken() // consume "C"

	// Expect '{'
	if p.curToken.Type != lexer.TokenLBrace {
		return nil
	}

	// Build content manually by reading tokens until matching '}'
	p.nextToken() // consume '{'

	// Use the raw input approach to extract C code
	input := p.l.GetInput()
	openBracePos := strings.Index(input[p.l.Position:], "{")
	if openBracePos == -1 {
		return nil
	}

	currentPos := p.l.Position + openBracePos + 1
	braceDepth := 1
	startPos := currentPos

	for currentPos < len(input) && braceDepth > 0 {
		ch := input[currentPos]
		if ch == '{' {
			braceDepth++
		} else if ch == '}' {
			braceDepth--
			if braceDepth == 0 {
				break
			}
		}
		currentPos++
	}

	// Extract content (excluding the final '}')
	content := input[startPos:currentPos]

	// Advance lexer to after the closing brace
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		p.nextToken()
	}
	p.nextToken() // consume final '}'

	// Clean up the content
	content = strings.TrimSpace(content)

	return &CImportBlock{Content: content}
}

// Phase 11: Pointer operations parsing
func (p *Parser) parseAddrOf() *AddressOf {
	p.nextToken() // consume 'addr'
	p.nextToken() // consume '('
	operand := p.parseExpr()
	p.nextToken() // consume ')'
	return &AddressOf{Operand: operand}
}

func (p *Parser) parseDeref() *Dereference {
	p.nextToken() // consume '*'
	operand := p.parsePrimaryExpr()
	return &Dereference{Operand: operand}
}

func (p *Parser) parseAlloc() *AllocExpr {
	p.nextToken() // consume 'alloc'
	p.nextToken() // consume '('

	// Type parameter
	typeName := p.curToken.Literal
	p.nextToken() // consume type
	p.nextToken() // consume ','

	count := p.parseExpr()
	p.nextToken() // consume ')'

	return &AllocExpr{Type: typeName, Count: count}
}

func (p *Parser) parseFree() *FreeExpr {
	p.nextToken() // consume 'free'
	p.nextToken() // consume '('
	ptr := p.parseExpr()
	p.nextToken() // consume ')'
	return &FreeExpr{Ptr: ptr}
}

// Phase 14: Quantum computing parsing
func (p *Parser) parseQRegDecl() *QRegDeclStmt {
	p.nextToken() // consume 'qreg'
	name := p.curToken.Literal
	p.nextToken() // consume name
	p.nextToken() // consume '='
	qubits := p.parseExpr()
	return &QRegDeclStmt{Name: name, Qubits: qubits}
}

func (p *Parser) parseGateApply() *GateApplyStmt {
	p.nextToken() // consume 'gate'
	gateName := p.curToken.Literal
	p.nextToken() // consume gate name
	p.nextToken() // consume '('

	target := p.parseExpr()
	p.nextToken() // advance past primary expression

	var control Node = nil
	var params []Node = nil

	// Check for control qubit (CNOT format: CNOT(control, target))
	if p.curToken.Type == lexer.TokenComma {
		p.nextToken() // consume ','
		control = target
		target = p.parseExpr()
		p.nextToken() // advance past primary expression
	}

	// Check for parameters (rotation gates)
	if p.curToken.Type == lexer.TokenComma {
		p.nextToken() // consume ','
		params = []Node{p.parseExpr()}
		p.nextToken() // advance past primary expression
	}

	p.nextToken() // consume ')'

	return &GateApplyStmt{
		Gate:    gateName,
		Target:  target,
		Control: control,
		Params:  params,
	}
}

func (p *Parser) parseMeasure() *MeasureExpr {
	p.nextToken() // consume 'measure'
	p.nextToken() // consume '('
	qubit := p.parseExpr()
	p.nextToken() // consume ')'
	return &MeasureExpr{Qubit: qubit}
}

// Phase 11: Matrix declaration parsing (standalone)
func (p *Parser) parseMatrixDecl() *VarDeclStmt {
	p.nextToken() // consume 'matrix'
	p.nextToken() // consume '['

	rows := p.parseExpr()
	p.nextToken() // consume ','

	cols := p.parseExpr()
	p.nextToken() // consume ']'

	dataType := p.curToken.Literal // e.g., "float64"
	p.nextToken()                  // consume type

	// Variable name should come next
	name := p.curToken.Literal
	p.nextToken() // consume variable name

	matrixDecl := &MatrixDecl{
		Rows:     rows,
		Cols:     cols,
		DataType: dataType,
	}

	return &VarDeclStmt{Name: name, Value: matrixDecl, IsMatrix: true}
}

// Phase 16: Actor parsing functions
func (p *Parser) parseActor() *ActorDeclStmt {
	p.nextToken() // consume 'actor'
	actor := &ActorDeclStmt{Name: p.curToken.Literal, Body: []Node{}}

	p.nextToken() // consume actor name
	p.nextToken() // consume '('
	p.nextToken() // consume ')'
	p.nextToken() // consume '{'

	actor.Body = p.parseBlock()

	p.nextToken() // consume '}'
	return actor
}

func (p *Parser) parseSpawn() *SpawnExpr {
	p.nextToken() // consume 'spawn'
	p.nextToken() // consume '('
	actorName := p.curToken.Literal
	p.nextToken() // consume actor name
	p.nextToken() // consume ')'

	args := []Node{}
	if p.curToken.Type == lexer.TokenLParen {
		p.nextToken() // consume '('
		for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
			args = append(args, p.parseExpr())
			if p.curToken.Type == lexer.TokenComma {
				p.nextToken() // consume ','
			}
		}
		p.nextToken() // consume ')'
	}

	return &SpawnExpr{ActorName: actorName, Args: args}
}

func (p *Parser) parseReceive() *ReceiveStmt {
	p.nextToken() // consume 'receive'
	p.nextToken() // consume '('
	channel := p.parseExpr()
	p.nextToken() // consume ')'

	varName := ""
	if p.curToken.Type == lexer.TokenIdent {
		varName = p.curToken.Literal
		p.nextToken() // consume variable name
	}

	return &ReceiveStmt{Channel: channel, VarName: varName}
}

func (p *Parser) parseSend() *SendExpr {
	channel := p.parseExpr()
	p.nextToken() // consume '<'
	p.nextToken() // consume '-'
	message := p.parseExpr()
	return &SendExpr{Channel: channel, Message: message}
}

// Phase 11: Matrix declaration parsing (internal, without consuming variable name)
func (p *Parser) parseMatrixDeclInternal() *MatrixDecl {
	p.nextToken() // consume 'matrix'
	p.nextToken() // consume '['

	rows := p.parseExpr()
	p.nextToken() // consume ','

	cols := p.parseExpr()
	p.nextToken() // consume ']'

	dataType := p.curToken.Literal // e.g., "float64"
	p.nextToken()                  // consume type

	return &MatrixDecl{
		Rows:     rows,
		Cols:     cols,
		DataType: dataType,
	}
}

// Phase 11: Matrix indexing parsing
func (p *Parser) parseMatrixIndex() *MatrixIndexExpr {
	matrix := p.parsePrimaryExpr()

	p.nextToken() // consume '['
	row := p.parseExpr()
	p.nextToken() // consume ','
	col := p.parseExpr()
	p.nextToken() // consume ']'

	return &MatrixIndexExpr{
		Matrix: matrix,
		Row:    row,
		Col:    col,
	}
}

// Phase 18: GPU kernel parsing
func (p *Parser) parseKernel() *KernelDeclStmt {
	p.nextToken() // consume 'kernel'
	kernel := &KernelDeclStmt{Name: p.curToken.Literal, Body: []Node{}}
	p.nextToken() // consume kernel name
	p.nextToken() // consume '('

	// Parse typed parameters: name: type
	for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
		if p.curToken.Type == lexer.TokenIdent {
			paramName := p.curToken.Literal
			p.nextToken() // consume param name
			if p.curToken.Type == lexer.TokenColon {
				p.nextToken() // consume ':'
			}
			paramType := p.curToken.Literal
			p.nextToken() // consume type
			kernel.Params = append(kernel.Params, Parameter{Name: paramName, Type: paramType})
			if p.curToken.Type == lexer.TokenComma {
				p.nextToken() // consume ','
			}
		} else {
			p.nextToken()
		}
	}
	p.nextToken() // consume ')'
	p.nextToken() // consume '{'

	kernel.Body = p.parseBlock()
	p.nextToken() // consume '}'
	return kernel
}

// Phase 18: barrier() statement parsing
func (p *Parser) parseBarrierStmt() *BarrierStmt {
	p.nextToken() // consume 'barrier'
	if p.curToken.Type == lexer.TokenLParen {
		p.nextToken() // consume '('
		if p.curToken.Type == lexer.TokenRParen {
			p.nextToken() // consume ')'
		}
	}
	return &BarrierStmt{}
}

// Phase 18: global_id(dim) expression parsing
func (p *Parser) parseGlobalIDExpr() *GlobalIdExpr {
	p.nextToken() // consume 'global_id'
	p.nextToken() // consume '('
	dim := 0
	if p.curToken.Type == lexer.TokenInt {
		dim = 0
		for _, ch := range p.curToken.Literal {
			if ch >= '0' && ch <= '9' {
				dim = dim*10 + int(ch-'0')
			}
		}
		p.nextToken() // consume dimension literal
	}
	if p.curToken.Type == lexer.TokenRParen {
		p.nextToken() // consume ')'
	}
	return &GlobalIdExpr{Dimension: dim}
}
