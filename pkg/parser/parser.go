package parser

import (
	"karkain/pkg/lexer"
)

type Parser struct {
	l           *lexer.Lexer
	curToken    lexer.Token
	peekToken   lexer.Token
	structTypes map[string]bool // tracks declared struct type names
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, structTypes: map[string]bool{}}
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseProgram() *Program {
	prog := &Program{Statements: []Node{}}
	for p.curToken.Type != lexer.TokenEOF {
		switch p.curToken.Type {
		case lexer.TokenFunc:
			if stmt := p.parseFunc(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		case lexer.TokenTypeKw:
			if decl := p.parseStructDecl(); decl != nil {
				prog.Statements = append(prog.Statements, decl)
			}
		default:
			p.nextToken()
		}
	}
	return prog
}

// parseStructDecl parses: type Name struct { field type \n ... }
func (p *Parser) parseStructDecl() *StructDecl {
	p.nextToken() // consume 'type'
	name := p.curToken.Literal
	p.nextToken() // consume struct name
	p.nextToken() // consume 'struct'
	p.nextToken() // consume '{'

	decl := &StructDecl{Name: name}
	p.structTypes[name] = true

	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		if p.curToken.Type == lexer.TokenIdent {
			fieldName := p.curToken.Literal
			p.nextToken() // consume field name
			fieldType := p.curToken.Literal
			p.nextToken() // consume field type
			decl.Fields = append(decl.Fields, &StructField{Name: fieldName, Type: fieldType})
		} else {
			p.nextToken()
		}
	}
	p.nextToken() // consume '}'
	return decl
}

func (p *Parser) parseFunc() *FuncDecl {
	p.nextToken() // consume 'func'
	fn := &FuncDecl{Name: p.curToken.Literal, Body: []Node{}}

	p.nextToken() // consume fn name
	p.nextToken() // consume '('

	for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
		if p.curToken.Type == lexer.TokenIdent {
			fn.Params = append(fn.Params, p.curToken.Literal)
		}
		p.nextToken()
		if p.curToken.Type == lexer.TokenComma {
			p.nextToken()
		}
	}
	p.nextToken() // consume ')'
	p.nextToken() // consume '{'

	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		stmt := p.parseStatement()
		if stmt != nil {
			fn.Body = append(fn.Body, stmt)
		}
	}
	p.nextToken() // consume '}'
	return fn
}

func (p *Parser) parseStatement() Node {
	switch p.curToken.Type {
	case lexer.TokenLet:
		return p.parseVarDecl()
	case lexer.TokenPrint:
		return p.parsePrint()
	case lexer.TokenReturn:
		return p.parseReturn()
	case lexer.TokenIf:
		return p.parseIf()
	case lexer.TokenDelete:
		return p.parseDeleteStmt()
	case lexer.TokenIdent:
		// Detect: ident.field = expr  → FieldAssignStmt
		if p.peekToken.Type == lexer.TokenDot {
			return p.parseFieldAssignOrExpr()
		}
		// Detect: ident = expr  → AssignStmt
		if p.peekToken.Type == lexer.TokenAssign {
			return p.parseAssign()
		}
		// Otherwise: expression statement (call, index, etc.)
		expr := p.parseExpr()
		return &ExprStmt{Expression: expr}
	default:
		p.nextToken()
		return nil
	}
}

// parseFieldAssignOrExpr disambiguates  `obj.field = val`  vs  `obj.field` (read/call).
func (p *Parser) parseFieldAssignOrExpr() Node {
	// We need to look further: ident DOT ident ASSIGN
	// Use the expression parser and then check if it produced a FieldAccess
	// followed by an '=' sign.
	expr := p.parseExpr()
	if p.curToken.Type == lexer.TokenAssign {
		// It's a field assignment
		if fa, ok := expr.(*FieldAccess); ok {
			p.nextToken() // consume '='
			val := p.parseExpr()
			return &FieldAssignStmt{Object: fa.Left, Field: fa.Field, Value: val}
		}
	}
	return &ExprStmt{Expression: expr}
}

func (p *Parser) parseVarDecl() *VarDeclStmt {
	p.nextToken() // consume 'let'
	name := p.curToken.Literal
	p.nextToken() // consume identifier
	p.nextToken() // consume '='
	val := p.parseExpr()
	return &VarDeclStmt{Name: name, Value: val}
}

func (p *Parser) parseAssign() *AssignStmt {
	name := p.curToken.Literal
	p.nextToken() // consume identifier
	p.nextToken() // consume '='
	val := p.parseExpr()
	return &AssignStmt{Name: name, Value: val}
}

func (p *Parser) parseReturn() *ReturnStmt {
	p.nextToken() // consume 'return'
	val := p.parseExpr()
	return &ReturnStmt{Value: val}
}

func (p *Parser) parsePrint() *PrintStmt {
	p.nextToken() // consume 'print'
	p.nextToken() // consume '('
	val := p.parseExpr()
	if p.curToken.Type == lexer.TokenRParen {
		p.nextToken() // consume ')'
	}
	return &PrintStmt{Value: val}
}

func (p *Parser) parseDeleteStmt() *DeleteStmt {
	p.nextToken() // consume 'delete'
	p.nextToken() // consume '('
	mapExpr := p.parseExpr()
	if p.curToken.Type == lexer.TokenComma {
		p.nextToken() // consume ','
	}
	keyExpr := p.parseExpr()
	if p.curToken.Type == lexer.TokenRParen {
		p.nextToken() // consume ')'
	}
	return &DeleteStmt{Map: mapExpr, Key: keyExpr}
}

func (p *Parser) parseIf() *IfStmt {
	p.nextToken() // consume 'if'
	p.nextToken() // consume '('
	cond := p.parseExpr()
	if p.curToken.Type == lexer.TokenRParen {
		p.nextToken() // consume ')'
	}
	p.nextToken() // consume '{'

	var consequence []Node
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		stmt := p.parseStatement()
		if stmt != nil {
			consequence = append(consequence, stmt)
		}
	}
	p.nextToken() // consume '}'

	var alternative []Node
	if p.curToken.Type == lexer.TokenElse {
		p.nextToken() // consume 'else'
		p.nextToken() // consume '{'
		for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
			stmt := p.parseStatement()
			if stmt != nil {
				alternative = append(alternative, stmt)
			}
		}
		p.nextToken() // consume '}'
	}
	return &IfStmt{Condition: cond, Consequence: consequence, Alternative: alternative}
}

// parseExpr handles binary operations and comparisons.
func (p *Parser) parseExpr() Node {
	left := p.parsePostfix()

	switch p.curToken.Type {
	case lexer.TokenPlus, lexer.TokenMinus, lexer.TokenStar, lexer.TokenSlash,
		lexer.TokenEqual, lexer.TokenLessThan, lexer.TokenGreaterThan:
		op := p.curToken.Literal
		p.nextToken()
		right := p.parseExpr()
		return &BinaryExpr{Left: left, Operator: op, Right: right}
	}

	return left
}

// parsePostfix handles index expressions (x[k]) and field access (x.f) after a primary.
func (p *Parser) parsePostfix() Node {
	left := p.parsePrimary()

	for {
		if p.curToken.Type == lexer.TokenLBracket {
			p.nextToken() // consume '['
			index := p.parseExpr()
			if p.curToken.Type == lexer.TokenRBracket {
				p.nextToken() // consume ']'
			}
			left = &IndexExpr{Left: left, Index: index}
		} else if p.curToken.Type == lexer.TokenDot {
			p.nextToken() // consume '.'
			field := p.curToken.Literal
			p.nextToken() // consume field name
			left = &FieldAccess{Left: left, Field: field}
		} else {
			break
		}
	}
	return left
}

// parsePrimary handles the innermost expressions.
func (p *Parser) parsePrimary() Node {
	tok := p.curToken
	p.nextToken() // consume the current token

	switch tok.Type {
	case lexer.TokenString:
		return &StringLiteral{Value: tok.Literal}

	case lexer.TokenInt:
		return &IntLiteral{Value: tok.Literal}

	case lexer.TokenMinus:
		if p.curToken.Type == lexer.TokenInt {
			val := p.curToken.Literal
			p.nextToken() // consume TokenInt
			return &IntLiteral{Value: "-" + val}
		}
		return nil

	case lexer.TokenIdent:
		// Struct literal: TypeName { field: val, ... }  — only if TypeName is a known struct
		if p.curToken.Type == lexer.TokenLBrace && p.structTypes[tok.Literal] {
			p.nextToken() // consume '{'
			fields := map[string]Node{}
			for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
				if p.curToken.Type == lexer.TokenIdent {
					fieldName := p.curToken.Literal
					p.nextToken() // consume field name
					if p.curToken.Type == lexer.TokenColon {
						p.nextToken() // consume ':'
					}
					val := p.parseExpr()
					fields[fieldName] = val
				}
				if p.curToken.Type == lexer.TokenComma {
					p.nextToken() // consume ','
				}
			}
			if p.curToken.Type == lexer.TokenRBrace {
				p.nextToken() // consume '}'
			}
			return &StructLiteral{TypeName: tok.Literal, Fields: fields}
		}
		// Function call: ident(args...)
		if p.curToken.Type == lexer.TokenLParen {
			p.nextToken() // consume '('
			args := p.parseCallArgs()
			if p.curToken.Type == lexer.TokenRParen {
				p.nextToken() // consume ')'
			}
			return &CallExpr{Function: tok.Literal, Args: args}
		}
		return &Identifier{Name: tok.Literal}

	case lexer.TokenHasKey:
		p.nextToken() // consume '('
		args := p.parseCallArgs()
		if p.curToken.Type == lexer.TokenRParen {
			p.nextToken() // consume ')'
		}
		return &CallExpr{Function: "hasKey", Args: args}

	case lexer.TokenLBracket:
		// Array literal: [elem, elem, ...]
		var elems []Node
		for p.curToken.Type != lexer.TokenRBracket && p.curToken.Type != lexer.TokenEOF {
			elems = append(elems, p.parseExpr())
			if p.curToken.Type == lexer.TokenComma {
				p.nextToken()
			}
		}
		if p.curToken.Type == lexer.TokenRBracket {
			p.nextToken()
		}
		return &ArrayLiteral{Elements: elems}

	case lexer.TokenLBrace:
		// Map literal: {"key": val, ...}
		var keys, vals []Node
		for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
			key := p.parseExpr()
			keys = append(keys, key)
			if p.curToken.Type == lexer.TokenColon {
				p.nextToken()
			}
			val := p.parseExpr()
			vals = append(vals, val)
			if p.curToken.Type == lexer.TokenComma {
				p.nextToken()
			}
		}
		if p.curToken.Type == lexer.TokenRBrace {
			p.nextToken()
		}
		return &MapLiteral{Keys: keys, Values: vals}

	case lexer.TokenLParen:
		expr := p.parseExpr()
		if p.curToken.Type == lexer.TokenRParen {
			p.nextToken()
		}
		return expr
	}

	return nil
}

func (p *Parser) parseCallArgs() []Node {
	var args []Node
	for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
		args = append(args, p.parseExpr())
		if p.curToken.Type == lexer.TokenComma {
			p.nextToken()
		}
	}
	return args
}
