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
	src       string   // Source text for zero-copy token literal access
	arena     *Arena   // Arena allocator for AST nodes — batch allocation
	Errors    []string // Phase 19: Parser error tracking
	enumNames map[string]bool // Phase 45: known enum type names
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, src: l.GetInput(), arena: NewArena(), Errors: []string{}, enumNames: make(map[string]bool)}
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
	prog := p.arena.AllocProgram([]Node{}, []*CImportBlock{})
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
		} else if p.curToken.Type == lexer.TokenLinear {
			if stmt := p.parseLinearTypeDecl(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenPacked {
			if stmt := p.parsePackedStructDecl(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenEnum {
			if stmt := p.parseEnumDecl(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenImport {
			if cImport := p.parseCImport(); cImport != nil {
				prog.CImports = append(prog.CImports, cImport)
			}
		} else {
			p.addError(fmt.Sprintf("unexpected token '%s' at top level", p.curToken.Literal(p.src)))
			p.nextToken()
		}
		// Ensure progress even on errors
		if len(prog.Statements) == 0 && len(prog.CImports) == 0 {
			p.nextToken()
		}
	}
	return prog
}

func (p *Parser) parseFunc() *FuncDecl {
	p.nextToken() // consume 'func'
	fn := p.arena.AllocFuncDecl(p.curToken.Literal(p.src), nil, nil, nil)
	fn.Params = []string{}

	p.nextToken() // consume fn name
	p.nextToken() // consume '('

	// Parse parameters
	for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
		if p.curToken.Type == lexer.TokenIdent {
			paramName := p.curToken.Literal(p.src)
			fn.Params = append(fn.Params, paramName)
			p.nextToken() // consume param name

			// Phase 46: Parse optional type annotation (e.g., `a int`, `b string`)
			if p.curToken.Type == lexer.TokenIdent {
				typeName := p.curToken.Literal(p.src)
				fn.ParamTypes = append(fn.ParamTypes, typeName)
				p.nextToken() // consume type name
			} else {
				fn.ParamTypes = append(fn.ParamTypes, "")
			}

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

	// Phase 49: Escape analysis — mark variables that escape the function scope
	MarkVarEscaping(fn.Body)

	return fn
}

func (p *Parser) parseVarDecl() *VarDeclStmt {
	p.nextToken() // consume 'let' or 'var'
	name := p.curToken.Literal(p.src)

	p.nextToken() // consume identifier

	// Check for type annotation (e.g., `var x int = 42`, `let p &int = &x`)
	var typeName string
	if p.curToken.Type == lexer.TokenAmp {
		// Reference type: &T or &mut T
		p.nextToken() // consume '&'
		mutable := false
		if p.curToken.Type == lexer.TokenMut {
			mutable = true
			p.nextToken() // consume 'mut'
		}
		if p.curToken.Type == lexer.TokenIdent {
			typeName = "&" + p.curToken.Literal(p.src)
			if mutable {
				typeName = "&mut " + p.curToken.Literal(p.src)
			}
			p.nextToken() // consume base type
		}
	} else if p.curToken.Type == lexer.TokenStar {
		typeName = "*" // Pointer type
		p.nextToken()  // consume '*'
		if p.curToken.Type == lexer.TokenIdent {
			typeName += p.curToken.Literal(p.src)
			p.nextToken() // consume base type
		}
	} else if p.curToken.Type == lexer.TokenIdent {
		typeName = p.curToken.Literal(p.src)
		p.nextToken() // consume type name
	} else if p.curToken.Type == lexer.TokenBool {
		typeName = p.curToken.Literal(p.src)
		p.nextToken() // consume bool type
	} else if p.curToken.Type == lexer.TokenBigInt {
		typeName = "bigint"
		p.nextToken() // consume bigint type
	} else if p.curToken.Type == lexer.TokenBigFloat {
		typeName = "bigfloat"
		p.nextToken() // consume bigfloat type
	}

	p.nextToken() // consume '='

	// Check if the value is a matrix declaration
	if p.curToken.Type == lexer.TokenMatrix {
		matrixDecl := p.parseMatrixDeclInternal()
		return &VarDeclStmt{Name: name, Value: matrixDecl, IsMatrix: true}
	}

	// Phase 48: let x = fn(a, b) { ... } → treat as named function declaration
	if p.curToken.Type == lexer.TokenFn {
		lambda := p.parseLambda()
		fn := p.arena.AllocFuncDecl(name, lambda.Params, lambda.Body, nil)
		fn.ParamTypes = lambda.ParamTypes
		return &VarDeclStmt{Name: name, Value: fn}
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
		// Ensure progress even on errors
		if len(stmts) == 0 {
			p.nextToken()
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
	case lexer.TokenBreak:
		p.nextToken() // consume 'break'
		return p.arena.AllocBreakStmt()
	case lexer.TokenContinue:
		p.nextToken() // consume 'continue'
		return p.arena.AllocContinueStmt()
	case lexer.TokenFn:
		return &ExprStmt{Expression: p.parseLambda()}
	case lexer.TokenIdent:
		return p.parseIdentStatement()
	case lexer.TokenImport:
		// Skip import blocks inside functions (they're handled at top level)
		cImport := p.parseCImport()
		if cImport != nil {
			return &ExprStmt{Expression: &StringLiteral{Value: cImport.Content}}
		}
		return nil
	case lexer.TokenAt, lexer.TokenAmp, lexer.TokenMove:
		// Phase 41: Expression statements starting with @, &, or move
		expr := p.parsePrimaryExpr()
		if expr != nil {
			return &ExprStmt{Expression: expr}
		}
		return nil
	case lexer.TokenMatch:
		// Phase 42: match value { pattern => expr, ... }
		return &ExprStmt{Expression: p.parseMatchExpr()}
	case lexer.TokenLBrace:
		// Phase 51: bare block { stmts }
		p.nextToken() // consume '{'
		stmts := p.parseBlock()
		p.nextToken() // consume '}'
		return &BlockStmt{Statements: stmts}
	case lexer.TokenInt, lexer.TokenFloat64, lexer.TokenString, lexer.TokenBigInt, lexer.TokenBigFloat,
		lexer.TokenSome, lexer.TokenNone, lexer.TokenOk, lexer.TokenErr,
		lexer.TokenLParen, lexer.TokenLBracket,
		lexer.TokenTrue, lexer.TokenFalse:
		expr := p.parseExpr()
		if expr != nil {
			return &ExprStmt{Expression: expr}
		}
		return nil
	default:
		p.addError(fmt.Sprintf("unexpected token '%s' (%s)", p.curToken.Literal(p.src), p.curToken.Type))
		p.nextToken()
		return nil
	}
}

func (p *Parser) parseReturn() *ReturnStmt {
	p.nextToken() // consume 'return'
	val := p.parseExpr()
	return &ReturnStmt{Value: val}
}

// Phase 48: Lambda expressions — fn(a, b) { return a + b }
func (p *Parser) parseLambda() *LambdaExpr {
	p.nextToken() // consume 'fn'
	p.nextToken() // consume '('

	params := []string{}
	paramTypes := []string{}
	for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
		if p.curToken.Type == lexer.TokenIdent {
			paramName := p.curToken.Literal(p.src)
			params = append(params, paramName)
			p.nextToken() // consume param name
			if p.curToken.Type == lexer.TokenIdent {
				paramTypes = append(paramTypes, p.curToken.Literal(p.src))
				p.nextToken() // consume type
			} else {
				paramTypes = append(paramTypes, "")
			}
		} else {
			p.nextToken()
		}
		if p.curToken.Type == lexer.TokenComma {
			p.nextToken() // consume ','
		}
	}
	p.nextToken() // consume ')'
	p.nextToken() // consume '{'
	body := p.parseBlock()
	p.nextToken() // consume '}'
	return p.arena.AllocLambdaExpr(params, paramTypes, body)
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
func (p *Parser) parseFor() Node {
	p.nextToken() // consume 'for'

	// Phase 47: Detect for-in syntax: for x in expr { ... }
	// After consuming 'for', curToken = variable name, peekToken = 'in'
	if p.curToken.Type == lexer.TokenIdent && p.peekToken.Type == lexer.TokenIn {
		name := p.curToken.Literal(p.src)
		p.nextToken() // consume variable name
		p.nextToken() // consume 'in'
		// Parse iter expression manually to avoid parseIdentExpr consuming '{' as struct literal
		var iter Node
		switch p.curToken.Type {
		case lexer.TokenIdent:
			iter = p.arena.AllocIdentifier(p.curToken.Literal(p.src))
			p.nextToken()
		case lexer.TokenInt:
			iter = p.arena.AllocIntLiteral(p.curToken.Literal(p.src))
			p.nextToken()
		case lexer.TokenLParen:
			p.nextToken() // consume '('
			iter = p.parseExpr()
			p.nextToken() // consume ')'
		default:
			iter = p.parseExpr()
		}
		if p.curToken.Type == lexer.TokenLBrace {
			p.nextToken() // consume '{'
		}
		body := p.parseBlock()
		p.nextToken() // consume '}'
		return p.arena.AllocForInStmt(name, iter, body)
	}

	// Phase 48: for-in with map key: for k, v in map { ... }
	if p.curToken.Type == lexer.TokenIdent && p.peekToken.Type == lexer.TokenComma {
		keyName := p.curToken.Literal(p.src)
		p.nextToken() // consume key name
		p.nextToken() // consume ','
		valName := p.curToken.Literal(p.src)
		p.nextToken() // consume value name
		p.nextToken() // consume 'in'
		var iter Node
		switch p.curToken.Type {
		case lexer.TokenIdent:
			iter = p.arena.AllocIdentifier(p.curToken.Literal(p.src))
			p.nextToken()
		default:
			iter = p.parseExpr()
		}
		if p.curToken.Type == lexer.TokenLBrace {
			p.nextToken() // consume '{'
		}
		body := p.parseBlock()
		p.nextToken() // consume '}'
		return p.arena.AllocForInStmtWithKey(keyName, valName, iter, body)
	}

	// C-style for loop: for (init; cond; post) { ... }
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
	ident := p.curToken.Literal(p.src)
	p.nextToken() // consume identifier

	// Check for index assignment: arr[i] = val or matrix arr[r,c] = val
	if p.curToken.Type == lexer.TokenLBracket {
		p.nextToken() // consume '['
		first := p.parseExpr()
		if p.curToken.Type == lexer.TokenComma {
			// Matrix index: arr[row, col] = val
			p.nextToken() // consume ','
			col := p.parseExpr()
			p.nextToken() // consume ']'
			matIdx := &MatrixIndexExpr{
				Matrix: &Identifier{Name: ident},
				Row:    first,
				Col:    col,
			}
			if p.curToken.Type == lexer.TokenAssign {
				p.nextToken() // consume '='
				val := p.parseExpr()
				return &ExprStmt{Expression: &BinaryExpr{Left: matIdx, Operator: "=", Right: val}}
			}
			return &ExprStmt{Expression: matIdx}
		}
		// Single index: arr[i] = val or arr[i]
		p.nextToken() // consume ']'
		idxExpr := &IndexExpr{Left: &Identifier{Name: ident}, Index: first}
		if p.curToken.Type == lexer.TokenAssign {
			p.nextToken() // consume '='
			val := p.parseExpr()
			return &ExprStmt{Expression: &BinaryExpr{Left: idxExpr, Operator: "=", Right: val}}
		}
		return &ExprStmt{Expression: idxExpr}
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
		rightIdent := p.curToken.Literal(p.src)
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
		dotExpr := &DotExpr{Left: left, Right: rightIdent}
		// Phase 52: Chain nested field access — a.b.c
		for p.curToken.Type == lexer.TokenDot {
			p.nextToken() // consume '.'
			nextField := p.curToken.Literal(p.src)
			p.nextToken() // consume field identifier
			dotExpr = &DotExpr{Left: dotExpr, Right: nextField}
		}
		// Check for dot assignment: p.name = value
		if p.curToken.Type == lexer.TokenAssign {
			p.nextToken() // consume '='
			val := p.parseExpr()
			return &ExprStmt{Expression: &BinaryExpr{Left: dotExpr, Operator: "=", Right: val}}
		}
		return &ExprStmt{Expression: dotExpr}
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
		left = p.arena.AllocBinaryExpr(left, op, right)
	}

	// Handle assignment: ident = expr (only at precedence 0)
	if p.curToken.Type == lexer.TokenAssign && left != nil {
		p.nextToken() // consume '='
		right := p.parseExpr()
		return p.arena.AllocBinaryExpr(left, "=", right)
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
			left = p.arena.AllocMatrixIndexExpr(left, first, col)
		} else {
			// Array/map index: [index]
			p.nextToken() // consume ']'
			left = p.arena.AllocIndexExpr(left, first)
		}
	}

	// Phase 19: Handle @derive(Trait) and @tag(name, value) as postfix operators
	if p.curToken.Type == lexer.TokenAt {
		p.nextToken() // consume '@'
		ident := p.curToken.Literal(p.src)
		p.nextToken() // consume identifier
		if ident == "derive" && p.curToken.Type == lexer.TokenLParen {
			p.nextToken() // consume '('
			trait := p.curToken.Literal(p.src)
			p.nextToken() // consume trait name
			p.nextToken() // consume ')'
			left = &DeriveExpr{Trait: trait, Target: left}
		} else if ident == "tag" && p.curToken.Type == lexer.TokenLParen {
			p.nextToken() // consume '('
			tagName := p.curToken.Literal(p.src)
			p.nextToken() // consume tag name
			p.nextToken() // consume ','
			tagValue := p.curToken.Literal(p.src)
			p.nextToken() // consume tag value
			p.nextToken() // consume ')'
			left = &TagExpr{Target: left, TagName: tagName, TagValue: tagValue}
		}
	}

	// Phase 44: Handle ? postfix operator for error propagation
	if p.curToken.Type == lexer.TokenQuestion {
		p.nextToken() // consume '?'
		left = &PropagateExpr{Operand: left}
	}

	return left
}

func (p *Parser) parseUnary() Node {
	// Phase 19: Handle unary negation (-x), logical NOT (!x), and dereference (*ptr)
	if p.curToken.Type == lexer.TokenMinus {
		p.nextToken() // consume '-'
		operand := p.parseUnary()
		return p.arena.AllocUnaryExpr("-", operand)
	}
	if p.curToken.Type == lexer.TokenNot {
		p.nextToken() // consume '!'
		operand := p.parseUnary()
		return p.arena.AllocUnaryExpr("!", operand)
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
		val := p.curToken.Literal(p.src)
		p.nextToken()
		return p.arena.AllocStringLiteral(val)
	case lexer.TokenInt:
		val := p.curToken.Literal(p.src)
		p.nextToken()
		return p.arena.AllocIntLiteral(val)
	case lexer.TokenFloat64:
		val := p.curToken.Literal(p.src)
		p.nextToken()
		return p.arena.AllocFloat64Literal(val)
	case lexer.TokenBigInt:
		val := p.curToken.Literal(p.src)
		p.nextToken()
		return p.arena.AllocBigIntLiteral(val)
	case lexer.TokenBigFloat:
		val := p.curToken.Literal(p.src)
		p.nextToken()
		return p.arena.AllocBigFloatLiteral(val)
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
		return p.arena.AllocBoolLiteral(true)
	case lexer.TokenFalse:
		p.nextToken()
		return p.arena.AllocBoolLiteral(false)
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
	case lexer.TokenAt:
		// Phase 41: @raw(addr) or @raw(addr, val) — hardware memory access
		p.nextToken() // consume '@'
		if p.curToken.Type == lexer.TokenRaw {
			p.nextToken() // consume 'raw'
			p.nextToken() // consume '('
			addr := p.parseExpr()
			var val Node
			if p.curToken.Type == lexer.TokenComma {
				p.nextToken() // consume ','
				val = p.parseExpr()
			}
			p.nextToken() // consume ')'
			return &RawAccessExpr{Address: addr, Value: val}
		}
		// Phase 42: @simd_add(a, b), @simd_mul(a, b), etc.
		if p.curToken.Type == lexer.TokenIdent {
			op := p.curToken.Literal(p.src)
			p.nextToken() // consume simd op name
			if strings.HasPrefix(op, "simd_") {
				simdOp := strings.TrimPrefix(op, "simd_")
				p.nextToken() // consume '('
				args := []Node{}
				if p.curToken.Type != lexer.TokenRParen {
					args = append(args, p.parseExpr())
					for p.curToken.Type == lexer.TokenComma {
						p.nextToken() // consume ','
						args = append(args, p.parseExpr())
					}
				}
				p.nextToken() // consume ')'
				return &SIMDBuiltinExpr{Op: simdOp, Args: args}
			}
		}
		return nil
	case lexer.TokenSome:
		// Phase 42: Some(value)
		p.nextToken() // consume 'Some'
		p.nextToken() // consume '('
		val := p.parseExpr()
		p.nextToken() // consume ')'
		return &OptionSomeExpr{Value: val}
	case lexer.TokenNone:
		// Phase 42: None
		p.nextToken() // consume 'None'
		return &OptionNoneExpr{}
	case lexer.TokenOk:
		// Phase 42: Ok(value)
		p.nextToken() // consume 'Ok'
		p.nextToken() // consume '('
		val := p.parseExpr()
		p.nextToken() // consume ')'
		return &ResultOkExpr{Value: val}
	case lexer.TokenErr:
		// Phase 42: Err(error)
		p.nextToken() // consume 'Err'
		p.nextToken() // consume '('
		err := p.parseExpr()
		p.nextToken() // consume ')'
		return &ResultErrExpr{Error: err}
	case lexer.TokenAmp:
		// Phase 41: &x or &mut x — borrow expression
		p.nextToken() // consume '&'
		mutable := false
		if p.curToken.Type == lexer.TokenMut {
			mutable = true
			p.nextToken() // consume 'mut'
		}
		// Parse operand as simple expression (not struct literal)
		var operand Node
		if p.curToken.Type == lexer.TokenIdent {
			operand = &Identifier{Name: p.curToken.Literal(p.src)}
			p.nextToken()
		} else {
			operand = p.parsePrimaryExpr()
		}
		return &BorrowExpr{Operand: operand, Mutable: mutable}
	case lexer.TokenMove:
		// Phase 41: move(x) — explicit ownership transfer
		p.nextToken() // consume 'move'
		p.nextToken() // consume '('
		operand := p.parseExpr()
		p.nextToken() // consume ')'
		return &MoveExpr{Operand: operand}
	case lexer.TokenMatch:
		// Phase 42: match value { pattern => expr, ... }
		return p.parseMatchExpr()
	case lexer.TokenFn:
		return p.parseLambda()
	}
	return nil
}

// Phase 42: match value { pattern => expr, ... }
func (p *Parser) parseMatchExpr() *MatchExpr {
	p.nextToken() // consume 'match'

	// Parse match value — manually to avoid parseIdentExpr consuming '{' as struct literal
	var value Node
	switch p.curToken.Type {
	case lexer.TokenIdent:
		value = p.arena.AllocIdentifier(p.curToken.Literal(p.src))
		p.nextToken()
	case lexer.TokenInt:
		value = p.arena.AllocIntLiteral(p.curToken.Literal(p.src))
		p.nextToken()
	case lexer.TokenString:
		value = p.arena.AllocStringLiteral(p.curToken.Literal(p.src))
		p.nextToken()
	case lexer.TokenFloat64:
		value = p.arena.AllocFloat64Literal(p.curToken.Literal(p.src))
		p.nextToken()
	case lexer.TokenLParen:
		p.nextToken() // consume '('
		value = p.parseExpr()
		p.nextToken() // consume ')'
	default:
		value = p.parseUnary()
	}

	p.nextToken() // consume '{'

	arms := []MatchArm{}
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		// Parse pattern
		pattern := MatchPattern{}
		switch p.curToken.Type {
		case lexer.TokenSome:
			pattern.Type = "Some"
			p.nextToken() // consume 'Some'
			if p.curToken.Type == lexer.TokenLParen {
				p.nextToken() // consume '('
				pattern.Binding = p.curToken.Literal(p.src)
				p.nextToken() // consume binding name
				p.nextToken() // consume ')'
			}
		case lexer.TokenNone:
			pattern.Type = "None"
			p.nextToken() // consume 'None'
		case lexer.TokenOk:
			pattern.Type = "Ok"
			p.nextToken() // consume 'Ok'
			if p.curToken.Type == lexer.TokenLParen {
				p.nextToken() // consume '('
				pattern.Binding = p.curToken.Literal(p.src)
				p.nextToken() // consume binding name
				p.nextToken() // consume ')'
			}
		case lexer.TokenErr:
			pattern.Type = "Err"
			p.nextToken() // consume 'Err'
			if p.curToken.Type == lexer.TokenLParen {
				p.nextToken() // consume '('
				pattern.Binding = p.curToken.Literal(p.src)
				p.nextToken() // consume binding name
				p.nextToken() // consume ')'
			}
		case lexer.TokenInt, lexer.TokenString, lexer.TokenFloat64, lexer.TokenBigInt, lexer.TokenBigFloat:
			pattern.Type = "literal"
			pattern.Value = p.parsePrimaryExpr()
		case lexer.TokenTrue:
			pattern.Type = "literal"
			pattern.Value = &BoolLiteral{Value: true}
			p.nextToken()
		case lexer.TokenFalse:
			pattern.Type = "literal"
			pattern.Value = &BoolLiteral{Value: false}
			p.nextToken()
		default:
			// Wildcard, variable binding, or enum variant pattern (Color.Red)
			if p.curToken.Type == lexer.TokenIdent {
				name := p.curToken.Literal(p.src)
				p.nextToken() // consume identifier

				// Phase 45: Check for Enum.Variant pattern
				if p.curToken.Type == lexer.TokenDot {
					p.nextToken() // consume '.'
					variantName := p.curToken.Literal(p.src)
					p.nextToken() // consume variant name
					pattern.Type = "enum_variant"
					pattern.Binding = name + "." + variantName
				} else if name == "_" {
					pattern.Type = "wildcard"
				} else {
					pattern.Type = "binding"
					pattern.Binding = name
					p.nextToken()
				}
			}
		}

		if p.curToken.Type != lexer.TokenFatArrow {
			p.addError(fmt.Sprintf("expected '=>' in match arm, got '%s'", p.curToken.Literal(p.src)))
		}
		p.nextToken() // consume '=>'
		var body Node
		if p.curToken.Type == lexer.TokenLBrace {
			p.nextToken() // consume '{'
			stmts := p.parseBlock()
			p.nextToken() // consume '}'
			body = &BlockStmt{Statements: stmts}
		} else {
			body = p.parseStatement()
		}
		arms = append(arms, MatchArm{Pattern: pattern, Body: body})

		if p.curToken.Type == lexer.TokenComma {
			p.nextToken() // consume optional ','
		}
	}
	p.nextToken() // consume '}'
	return &MatchExpr{Value: value, Arms: arms}
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
	ident := p.curToken.Literal(p.src)
	p.nextToken() // consume identifier

	// Check for dot expression (e.g., C.sqrt, matrix.method)
	if p.curToken.Type == lexer.TokenDot {
		p.nextToken() // consume '.'
		rightIdent := p.curToken.Literal(p.src)
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

			if p.enumNames[ident] {
				var val Node
				if len(args) == 1 {
					val = args[0]
				}
				return &EnumVariantExpr{EnumName: ident, Variant: rightIdent, Value: val}
			}

			return &CallExpr{
				Function: ident + "." + rightIdent,
				Args:     args,
				IsCFunc:  ident == "C",
			}
		}

		if p.enumNames[ident] {
			return &EnumVariantExpr{EnumName: ident, Variant: rightIdent, Value: nil}
		}

		// Phase 52: Chain nested field access — a.b.c → DotExpr{Left: DotExpr{Left: a, Right: b}, Right: c}
		dotExpr := &DotExpr{Left: p.arena.AllocIdentifier(ident), Right: rightIdent}
		for p.curToken.Type == lexer.TokenDot {
			p.nextToken() // consume '.'
			nextField := p.curToken.Literal(p.src)
			p.nextToken() // consume field identifier
			dotExpr = &DotExpr{Left: dotExpr, Right: nextField}
		}
		return dotExpr
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

		return p.arena.AllocCallExpr(ident, args, false)
	}

	// Phase 19: Check for struct literal: TypeName{field: val, ...}
	if p.curToken.Type == lexer.TokenLBrace {
		return p.parseStructLiteral(ident)
	}

	return p.arena.AllocIdentifier(ident)
}

// Phase 19: Struct literal parsing: Name{field: val, ...}
func (p *Parser) parseStructLiteral(typeName string) *StructLiteral {
	p.nextToken() // consume '{'
	fields := []Node{}
	if p.curToken.Type != lexer.TokenRBrace {
		// Parse field: value pairs
		fieldName := p.curToken.Literal(p.src)
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
			fieldName = p.curToken.Literal(p.src)
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
	name := p.curToken.Literal(p.src)
	p.nextToken() // consume struct name
	p.nextToken() // consume 'struct'
	p.nextToken() // consume '{'

	fields := []StructField{}
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		fieldName := p.curToken.Literal(p.src)
		p.nextToken() // consume field name
		fieldType := p.curToken.Literal(p.src)
		p.nextToken() // consume field type
		fields = append(fields, StructField{Name: fieldName, Type: fieldType})
		if p.curToken.Type == lexer.TokenComma {
			p.nextToken() // consume ','
		}
	}
	p.nextToken() // consume '}'

	return &StructDeclStmt{Name: name, Fields: fields}
}

// Phase 45: enum declaration parsing: enum Name { Variant, Variant(payload), ... }
func (p *Parser) parseEnumDecl() *EnumDecl {
	p.nextToken() // consume 'enum'
	name := p.curToken.Literal(p.src)
	p.enumNames[name] = true // register enum name
	p.nextToken() // consume enum name
	p.nextToken() // consume '{'

	variants := []EnumVariant{}
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		variant := EnumVariant{}
		variant.Name = p.curToken.Literal(p.src)
		p.nextToken() // consume variant name

		if p.curToken.Type == lexer.TokenLParen {
			p.nextToken() // consume '('
			variant.Payload = p.curToken.Literal(p.src)
			p.nextToken() // consume payload type
			p.nextToken() // consume ')'
		}

		variants = append(variants, variant)
		if p.curToken.Type == lexer.TokenComma {
			p.nextToken() // consume ','
		}
	}
	p.nextToken() // consume '}'

	return &EnumDecl{Name: name, Variants: variants}
}

// Phase 45: linear type declaration parsing
func (p *Parser) parseLinearTypeDecl() *LinearTypeDecl {
	p.nextToken() // consume 'linear'
	p.nextToken() // consume 'type'
	name := p.curToken.Literal(p.src)
	p.nextToken() // consume type name
	p.nextToken() // consume '{'

	fields := []StructField{}
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		fieldName := p.curToken.Literal(p.src)
		p.nextToken() // consume field name
		fieldType := p.curToken.Literal(p.src)
		p.nextToken() // consume field type
		fields = append(fields, StructField{Name: fieldName, Type: fieldType})
		if p.curToken.Type == lexer.TokenComma {
			p.nextToken() // consume ','
		}
	}
	p.nextToken() // consume '}'

	return &LinearTypeDecl{Name: name, Fields: fields}
}

// Phase 45: packed struct declaration parsing
func (p *Parser) parsePackedStructDecl() *PackedStructDecl {
	p.nextToken() // consume 'packed'
	p.nextToken() // consume 'struct'
	name := p.curToken.Literal(p.src)
	p.nextToken() // consume struct name
	p.nextToken() // consume '{'

	fields := []StructField{}
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		fieldName := p.curToken.Literal(p.src)
		p.nextToken() // consume field name
		fieldType := p.curToken.Literal(p.src)
		p.nextToken() // consume field type
		fields = append(fields, StructField{Name: fieldName, Type: fieldType})
		if p.curToken.Type == lexer.TokenComma {
			p.nextToken() // consume ','
		}
	}
	p.nextToken() // consume '}'

	return &PackedStructDecl{Name: name, Fields: fields}
}

// Phase 11: Native C Interop parsing
func (p *Parser) parseCImport() *CImportBlock {
	p.nextToken() // consume 'import'

	// Expect string literal "C"
	if p.curToken.Type != lexer.TokenString || p.curToken.Literal(p.src) != "C" {
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
	typeName := p.curToken.Literal(p.src)
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
	name := p.curToken.Literal(p.src)
	p.nextToken() // consume name
	p.nextToken() // consume '='
	qubits := p.parseExpr()
	return &QRegDeclStmt{Name: name, Qubits: qubits}
}

func (p *Parser) parseGateApply() *GateApplyStmt {
	p.nextToken() // consume 'gate'
	gateName := p.curToken.Literal(p.src)
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

	dataType := p.curToken.Literal(p.src) // e.g., "float64"
	p.nextToken()                  // consume type

	// Variable name should come next
	name := p.curToken.Literal(p.src)
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
	actor := &ActorDeclStmt{Name: p.curToken.Literal(p.src), Body: []Node{}}

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
	actorName := p.curToken.Literal(p.src)
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
		varName = p.curToken.Literal(p.src)
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

	dataType := p.curToken.Literal(p.src) // e.g., "float64"
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
	kernel := &KernelDeclStmt{Name: p.curToken.Literal(p.src), Body: []Node{}}
	p.nextToken() // consume kernel name
	p.nextToken() // consume '('

	// Parse typed parameters: name: type
	for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
		if p.curToken.Type == lexer.TokenIdent {
			paramName := p.curToken.Literal(p.src)
			p.nextToken() // consume param name
			if p.curToken.Type == lexer.TokenColon {
				p.nextToken() // consume ':'
			}
			paramType := p.curToken.Literal(p.src)
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
		for _, ch := range p.curToken.Literal(p.src) {
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
