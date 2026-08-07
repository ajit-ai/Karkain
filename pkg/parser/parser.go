package parser

import (
	"karkain/pkg/lexer"
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
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l}
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
		if p.curToken.Type == lexer.TokenFunc {
			if stmt := p.parseFunc(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else {
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
	p.nextToken() // consume ')'
	p.nextToken() // consume '{'

	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		if p.curToken.Type == lexer.TokenLet {
			fn.Body = append(fn.Body, p.parseVarDecl())
		} else if p.curToken.Type == lexer.TokenPrint {
			fn.Body = append(fn.Body, p.parsePrint())
		} else {
			p.nextToken()
		}
	}
	p.nextToken() // consume '}'
	return fn
}

func (p *Parser) parseVarDecl() *VarDeclStmt {
	p.nextToken() // consume 'let'
	name := p.curToken.Literal

	p.nextToken() // consume identifier
	p.nextToken() // consume '='

	val := p.parseExpr()
	return &VarDeclStmt{Name: name, Value: val}
}

func (p *Parser) parsePrint() *PrintStmt {
	p.nextToken() // consume 'print'
	p.nextToken() // consume '('
	val := p.parseExpr()
	p.nextToken() // consume ')'
	return &PrintStmt{Value: val}
}

func (p *Parser) parseExpr() Node {
	left := p.parsePrimary()

	if p.peekToken.Type == lexer.TokenPlus || p.peekToken.Type == lexer.TokenMinus ||
		p.peekToken.Type == lexer.TokenStar || p.peekToken.Type == lexer.TokenSlash {
		p.nextToken()
		op := p.curToken.Literal
		p.nextToken()
		right := p.parseExpr()
		return &BinaryExpr{Left: left, Operator: op, Right: right}
	}

	return left
}

func (p *Parser) parsePrimary() Node {
	switch p.curToken.Type {
	case lexer.TokenString:
		return &StringLiteral{Value: p.curToken.Literal}
	case lexer.TokenInt:
		return &IntLiteral{Value: p.curToken.Literal}
	case lexer.TokenIdent:
		return &Identifier{Name: p.curToken.Literal}
	}
	return nil
}
