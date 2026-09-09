package parser

import (
	"fmt"
	"karkain/pkg/lexer"
	"strconv"
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
	src       string          // Source text for zero-copy token literal access
	arena     *Arena          // Arena allocator for AST nodes — batch allocation
	Errors    []string        // Phase 19: Parser error tracking
	ErrorCols []int           // Phase 82: 1-based token-start column per Errors entry
	enumNames map[string]bool // Phase 45: known enum type names

	// Phase 81: When true, an identifier directly followed by '{' inside an
	// unparenthesized `if` condition is treated as a plain identifier (the '{'
	// opens the body, not a struct literal). Cleared while parsing any '('
	// group so parenthesized struct literals keep working.
	unparenthesizedIfCondition bool
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, src: l.GetInput(), arena: NewArena(), Errors: []string{}, enumNames: make(map[string]bool)}
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) addError(msg string) {
	p.Errors = append(p.Errors, fmt.Sprintf("line %d: %s", p.curToken.Line, msg))
	p.ErrorCols = append(p.ErrorCols, int(p.curToken.Col)+1)
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

// advanceIfStalled forces forward progress when parsing did not consume the current
// token. parsePrimaryExpr returns nil without calling nextToken() for unexpected tokens
// (see its fall-through), which otherwise lets collection/argument loops append forever
// and exhaust memory. Recording curToken.Start (monotonically increasing source offset)
// detects a genuine stall; when one occurs we report an error and consume a token to
// guarantee every loop terminates.
func (p *Parser) advanceIfStalled(start uint32, context string) {
	if p.curToken.Type == lexer.TokenEOF || p.curToken.Start != start {
		return
	}
	p.addError(fmt.Sprintf("unexpected token '%s' in %s", p.curToken.Literal(p.src), context))
	p.nextToken()
}

func (p *Parser) ParseProgram() *Program {
	prog := p.arena.AllocProgram([]Node{}, []*CImportBlock{})
	for p.curToken.Type != lexer.TokenEOF {
		progressed := false
		if p.curToken.Type == lexer.TokenPub {
			progressed = true
			// public modifier: consume it, parse the next decl, mark as public.
			p.nextToken()
			if p.curToken.Type == lexer.TokenFunc {
				if stmt := p.parseFunc(); stmt != nil {
					stmt.Public = true
					prog.Statements = append(prog.Statements, stmt)
				}
			} else if p.curToken.Type == lexer.TokenTypeDef {
				if stmt := p.parseStructDecl(); stmt != nil {
					stmt.Public = true
					prog.Statements = append(prog.Statements, stmt)
				}
			} else if p.curToken.Type == lexer.TokenEnum {
				if stmt := p.parseEnumDecl(); stmt != nil {
					stmt.Public = true
					prog.Statements = append(prog.Statements, stmt)
				}
			} else {
				p.addError(fmt.Sprintf("unexpected token '%s' after 'public' modifier (expected func/type/enum)", p.curToken.Literal(p.src)))
				p.nextToken()
			}
		} else if p.curToken.Type == lexer.TokenFunc {
			progressed = true
			if stmt := p.parseFunc(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenAt {
			progressed = true
			// Phase 98: @target(npu) func ... — function-level execution target.
			if stmt := p.parseTargetAttr(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenActor {
			progressed = true
			if stmt := p.parseActor(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenMacro {
			progressed = true
			if stmt := p.parseMacro(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenKernel {
			progressed = true
			if stmt := p.parseKernel(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenTypeDef {
			progressed = true
			if stmt := p.parseStructDecl(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenLinear {
			progressed = true
			if stmt := p.parseLinearTypeDecl(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenPacked {
			progressed = true
			if stmt := p.parsePackedStructDecl(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenEnum {
			progressed = true
			if stmt := p.parseEnumDecl(); stmt != nil {
				prog.Statements = append(prog.Statements, stmt)
			}
		} else if p.curToken.Type == lexer.TokenImport {
			progressed = true
			// Peek at next token to distinguish C imports from module imports.
			// C import: import "C" { ... }   → TokenString
			// Module:   import <name>         → TokenIdent
			if p.peekToken.Type == lexer.TokenString {
				if cImport := p.parseCImport(); cImport != nil {
					prog.CImports = append(prog.CImports, cImport)
				}
			} else if p.peekToken.Type == lexer.TokenIdent {
				if modImp := p.parseModuleImport(); modImp != nil {
					prog.Imports = append(prog.Imports, modImp)
				}
			} else {
				p.addError(fmt.Sprintf("expected module name or \"C\" after 'import', got '%s'", p.peekToken.Literal(p.src)))
				p.nextToken()
			}
		} else {
			p.addError(fmt.Sprintf("unexpected token '%s' at top level", p.curToken.Literal(p.src)))
			p.nextToken()
		}
		// Ensure progress even on errors — only fire when no known-top-level
		// token was handled above (prevents double-advancing past C-import
		// tokens, which the main dispatch already consumed).
		if !progressed {
			p.nextToken()
		}
	}
	return prog
}

func (p *Parser) parseTargetAttr() *FuncDecl {
	// curToken is '@'. Parse `@target(<name>) func ...` and attach the target
	// to the following function declaration. Phase 98: only the attribute
	// syntax is owned here; semantic target validation lives in pkg/sema.
	p.nextToken() // consume '@'
	if p.curToken.Type != lexer.TokenIdent || p.curToken.Literal(p.src) != "target" {
		p.addError(fmt.Sprintf("expected '@target(...)' attribute, got '@%s'", p.curToken.Literal(p.src)))
		p.nextToken()
		return nil
	}
	p.nextToken() // consume 'target'
	if p.curToken.Type != lexer.TokenLParen {
		p.addError("expected '(' after '@target'")
		p.nextToken()
		return nil
	}
	p.nextToken() // consume '('
	targetName := ""
	if p.curToken.Type == lexer.TokenIdent {
		targetName = p.curToken.Literal(p.src)
		p.nextToken() // consume target name
	} else {
		p.addError("expected target name in '@target(...)'")
		p.nextToken()
		return nil
	}
	if p.curToken.Type != lexer.TokenRParen {
		p.addError("expected ')' after '@target(...)'")
		p.nextToken()
		return nil
	}
	p.nextToken() // consume ')'
	if p.curToken.Type != lexer.TokenFunc {
		p.addError(fmt.Sprintf("'@target(%s)' must be followed by 'func'", targetName))
		p.nextToken()
		return nil
	}
	fn := p.parseFunc()
	if fn != nil {
		fn.Target = targetName
	}
	return fn
}

func (p *Parser) parseFunc() *FuncDecl {
	p.nextToken() // consume 'func'
	fn := p.arena.AllocFuncDecl(p.curToken.Literal(p.src), nil, nil, nil)
	fn.Line = int(p.curToken.Line)
	fn.Col = int(p.curToken.Col)
	fn.EndCol = fn.Col + len(p.curToken.Literal(p.src))
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

	// Phase 81: An optional single return-type annotation may follow the
	// parameter list (e.g. `func f(x) int { ... }`). It carries no stored
	// semantics yet, but must be consumed so the C-brace that opens the body
	// is properly aligned. Without this skip, the return type was treated as
	// the body opener and the closing brace was left dangling, which swallowed
	// every following top-level statement into the still-open body.
	if p.curToken.Type == lexer.TokenIdent {
		p.nextToken() // consume the return type
	}
	p.nextToken() // consume '{'

	fn.Body = p.parseBlock()
	p.nextToken() // consume '}'

	// Phase 49: Escape analysis — mark variables that escape the function scope
	MarkVarEscaping(fn.Body)

	return fn
}

func (p *Parser) parseVarDecl() *VarDeclStmt {
	line := int(p.curToken.Line)
	p.nextToken() // consume 'let' or 'var'
	name := p.curToken.Literal(p.src)
	nameCol := int(p.curToken.Col)
	nameEndCol := nameCol + len(name)

	p.nextToken() // consume identifier

	// Check for type annotation (e.g., `var x int = 42`, `let p &int = &x`)
	var typeName string
	isSIMD := false
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
	} else if p.curToken.Type == lexer.TokenLBracket {
		// Phase 70: [4]f32 fixed-lane SIMD vector type
		laneType := p.parseSIMDVectorType()
		if laneType != "" {
			typeName = laneType
			isSIMD = true
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
	} else if p.curToken.Type == lexer.TokenColon {
		// Colon type annotation: `let n: int = 7` — consume ':' then parse the
		// type name so the annotation lands in VarDeclStmt.Type instead of being
		// mis-parsed as the left operand of `int = 7`.
		p.nextToken() // consume ':'
		switch {
		case p.curToken.Type == lexer.TokenAmp:
			p.nextToken()
			base := p.curToken.Literal(p.src)
			p.nextToken()
			typeName = "&" + base
		case p.curToken.Type == lexer.TokenStar:
			p.nextToken()
			typeName = "*" + p.curToken.Literal(p.src)
			p.nextToken()
		case p.curToken.Type == lexer.TokenLBracket:
			laneType := p.parseSIMDVectorType()
			if laneType != "" {
				typeName = laneType
				isSIMD = true
			}
		case p.curToken.Type == lexer.TokenIdent:
			typeName = p.curToken.Literal(p.src)
			p.nextToken()
		case p.curToken.Type == lexer.TokenBool:
			typeName = p.curToken.Literal(p.src)
			p.nextToken()
		case p.curToken.Type == lexer.TokenBigInt:
			typeName = "bigint"
			p.nextToken()
		case p.curToken.Type == lexer.TokenBigFloat:
			typeName = "bigfloat"
			p.nextToken()
		default:
			typeName = p.curToken.Literal(p.src)
			p.nextToken()
		}
	}

	align := 0
	if p.curToken.Type == lexer.TokenAt {
		// Phase 70: cache-line alignment attribute between type and value,
		// e.g. `let v [4]f32 @aligned(64) = @simd_splat(1.0, 4)`.
		align = p.tryParseAlignAttr()
	}

	p.nextToken() // consume '='

	// Check if the value is a matrix declaration
	if p.curToken.Type == lexer.TokenMatrix {
		matrixDecl := p.parseMatrixDeclInternal()
		node := &VarDeclStmt{Name: name, Value: matrixDecl, IsMatrix: true, Col: nameCol, EndCol: nameEndCol}
		setNodeLine(node, line)
		return node
	}

	// Phase 48: let x = fn(a, b) { ... } → treat as named function declaration
	if p.curToken.Type == lexer.TokenFn {
		lambda := p.parseLambda()
		fn := p.arena.AllocFuncDecl(name, lambda.Params, lambda.Body, nil)
		fn.ParamTypes = lambda.ParamTypes
		fn.Captures = lambda.Captures // Phase 54: propagate captures to named binding
		node := &VarDeclStmt{Name: name, Value: fn, Col: nameCol, EndCol: nameEndCol}
		setNodeLine(node, line)
		return node
	}

	val := p.parseExpr()

	// If we have a type, store it in the variable declaration
	if typeName != "" {
		node := &VarDeclStmt{Name: name, Value: val, Type: typeName, IsSIMD: isSIMD, Align: align, Col: nameCol, EndCol: nameEndCol}
		setNodeLine(node, line)
		return node
	}

	node := &VarDeclStmt{Name: name, Value: val, Align: align, Col: nameCol, EndCol: nameEndCol}
	setNodeLine(node, line)
	return node
}

// parseSIMDVectorType parses a "[N]T" type after a '[' token and consumes
// through the ']' and element-type identifier. Returns the canonical type
// string (e.g. "[4]f32") or "" if the bracket is not a SIMD vector type.
func (p *Parser) parseSIMDVectorType() string {
	// curToken is '['
	p.nextToken() // consume '['
	if p.curToken.Type != lexer.TokenInt {
		return ""
	}
	lanes, err := strconv.Atoi(p.curToken.Literal(p.src))
	if err != nil || lanes <= 0 || lanes > 64 {
		return ""
	}
	p.nextToken() // consume lane count
	if p.curToken.Type != lexer.TokenRBracket {
		return ""
	}
	p.nextToken() // consume ']'
	if p.curToken.Type != lexer.TokenIdent {
		return ""
	}
	elem := p.curToken.Literal(p.src)
	switch elem {
	case "f32", "float32", "f64", "float64", "i32", "int32", "i64", "int64":
	default:
		return ""
	}
	p.nextToken() // consume element type
	return fmt.Sprintf("[%d]%s", lanes, elem)
}

// tryParseAlignAttr attempts to parse an `@aligned(N)` suffix. Returns the
// alignment or 0 if absent (leaving the cursor unchanged on failure).
func (p *Parser) tryParseAlignAttr() int {
	if p.curToken.Type != lexer.TokenAt {
		return 0
	}
	if p.peekToken.Type != lexer.TokenIdent || p.peekToken.Literal(p.src) != "aligned" {
		return 0
	}
	// consume '@'
	p.nextToken()
	p.nextToken() // consume 'aligned'
	if p.curToken.Type != lexer.TokenLParen {
		return 0
	}
	p.nextToken() // consume '('
	if p.curToken.Type != lexer.TokenInt {
		return 0
	}
	align, err := strconv.Atoi(p.curToken.Literal(p.src))
	if err != nil || align <= 0 {
		return 0
	}
	p.nextToken() // consume N
	if p.curToken.Type == lexer.TokenRParen {
		p.nextToken() // consume ')'
	}
	return align
}

// normalizeOrdering canonicalizes a memory-ordering name to one of
// relaxed/acquire/release/acq_rel/seq_cst. ok is false for unknown strings.
func normalizeOrdering(s string) (string, bool) {
	switch s {
	case "relaxed", "acquire", "release", "acq_rel", "seq_cst":
		return s, true
	}
	return "", false
}

func (p *Parser) parsePrint() *PrintStmt {
	line := int(p.curToken.Line)
	p.nextToken() // consume 'print'
	p.nextToken() // consume '('
	val := p.parseExpr()
	p.nextToken() // consume ')'
	node := &PrintStmt{Value: val}
	setNodeLine(node, line)
	return node
}

func (p *Parser) parseBlock() []Node {
	stmts := []Node{}
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		start := p.curToken.Start
		stmt := p.parseStatement()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
		// Ensure progress even on errors / stalled parses
		if p.curToken.Start == start {
			p.nextToken()
		}
	}
	return stmts
}

func (p *Parser) parseStatement() Node {
	p.arena.SetLine(int(p.curToken.Line))
	line := int(p.curToken.Line)
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
		node := &ExprStmt{Expression: p.parseAlloc()}
		setNodeLine(node, line)
		return node
	case lexer.TokenFree:
		node := &ExprStmt{Expression: p.parseFree()}
		setNodeLine(node, line)
		return node
	case lexer.TokenAddr:
		node := &ExprStmt{Expression: p.parseAddrOf()}
		setNodeLine(node, line)
		return node
	case lexer.TokenQReg:
		return p.parseQRegDecl()
	case lexer.TokenGate:
		return p.parseGateApply()
	case lexer.TokenMeasure:
		node := &ExprStmt{Expression: p.parseMeasure()}
		setNodeLine(node, line)
		return node
	case lexer.TokenMacro:
		return p.parseMacro()
	case lexer.TokenComptime:
		return p.parseComptimeStmt()
	case lexer.TokenSpawn:
		node := &ExprStmt{Expression: p.parseSpawn()}
		setNodeLine(node, line)
		return node
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
		node := &ExprStmt{Expression: p.parseLambda()}
		setNodeLine(node, line)
		return node
	case lexer.TokenIdent:
		return p.parseIdentStatement()
	case lexer.TokenImport:
		cImport := p.parseCImport()
		if cImport != nil {
			node := &ExprStmt{Expression: &StringLiteral{Value: cImport.Content}}
			setNodeLine(node, line)
			return node
		}
		return nil
	case lexer.TokenAt, lexer.TokenAmp, lexer.TokenMove:
		expr := p.parsePrimaryExpr()
		if expr != nil {
			node := &ExprStmt{Expression: expr}
			setNodeLine(node, line)
			return node
		}
		return nil
	case lexer.TokenMatch:
		node := &ExprStmt{Expression: p.parseMatchExpr()}
		setNodeLine(node, line)
		return node
	case lexer.TokenLBrace:
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
			node := &ExprStmt{Expression: expr}
			setNodeLine(node, line)
			return node
		}
		return nil
	default:
		p.addError(fmt.Sprintf("unexpected token '%s' (%s)", p.curToken.Literal(p.src), p.curToken.Type))
		p.nextToken()
		return nil
	}
}

func (p *Parser) parseReturn() *ReturnStmt {
	line := int(p.curToken.Line)
	p.nextToken() // consume 'return'
	val := p.parseExpr()
	node := &ReturnStmt{Value: val}
	setNodeLine(node, line)
	return node
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
	le := p.arena.AllocLambdaExpr(params, paramTypes, body)
	le.Captures = ComputeCaptures(params, body) // Phase 54
	return le
}

func (p *Parser) parseWhile() *WhileStmt {
	line := int(p.curToken.Line)
	p.nextToken() // consume 'while'
	p.nextToken() // consume '('
	condition := p.parseExpr()
	p.nextToken() // consume ')'
	p.nextToken() // consume '{'
	body := p.parseBlock()
	p.nextToken() // consume '}'
	node := &WhileStmt{Condition: condition, Body: body}
	setNodeLine(node, line)
	return node
}

// Phase 19: for (init; cond; post) { body }
func (p *Parser) parseFor() Node {
	line := int(p.curToken.Line)
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

	node := &ForStmt{Init: init, Condition: condition, Post: post, Body: body}
	setNodeLine(node, line)
	return node
}

func (p *Parser) parseIf() *IfStmt {
	line := int(p.curToken.Line)
	p.nextToken() // consume 'if'

	// Phase 81: give conditions both forms — `if x < y { ... }` and the
	// historical `if (x < y) { ... }`. parseExpr treats a '(' group as a
	// primary and continues with any operator that follows ')', so mixed forms
	// like `if (p.x) == 1 { ... }` parse too. The flag suppresses struct-literal
	// interpretation so an identifier right before the body is not confused
	// with `Type{...}` (see parseIdentExpr); it is cleared inside '(' groups,
	// so parenthesized struct literals keep working.
	p.unparenthesizedIfCondition = true
	condition := p.parseExpr()
	p.unparenthesizedIfCondition = false

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

	node := &IfStmt{
		Condition:   condition,
		Consequence: consequence,
		Alternative: alternative,
	}
	setNodeLine(node, line)
	return node
}

func (p *Parser) exprStmtAt(expr Node, line int) *ExprStmt {
	node := &ExprStmt{Expression: expr}
	setNodeLine(node, line)
	return node
}

func (p *Parser) parseIdentStatement() Node {
	line := int(p.curToken.Line)
	ident := p.curToken.Literal(p.src)
	identCol := int(p.curToken.Col)
	p.nextToken() // consume identifier

	// Phase 14: Bare quantum gate syntax — H qr[0], CNOT qr[0], qr[1]
	// A known gate name in statement position followed by an operand (identifier or int)
	// is a gate apply, not an expression statement.
	if isQuantumGate(ident) && (p.curToken.Type == lexer.TokenIdent || p.curToken.Type == lexer.TokenInt) {
		return p.parseBareGateApply(ident)
	}

	// Check for index assignment: arr[i] = val, chained arr[i][j] = val, or
	// matrix arr[r,c] = val. Postfix brackets chain so that nested 2D table
	// writes (dp[i][j] = v) build a single IndexExpr instead of splitting the
	// trailing "[j] = v" into a misparsed ArrayLiteral assignment.
	if p.curToken.Type == lexer.TokenLBracket {
		var target Node = &Identifier{Name: ident}
		for p.curToken.Type == lexer.TokenLBracket {
			p.nextToken() // consume '['
			var first Node
			if p.curToken.Type == lexer.TokenColon { // Phase 55: open start
				first = &IntLiteral{Value: "0"}
			} else {
				first = p.parseExpr()
			}
			if p.curToken.Type == lexer.TokenComma {
				// Matrix index: arr[row, col] = val
				p.nextToken() // consume ','
				col := p.parseExpr()
				p.nextToken() // consume ']'
				matIdx := &MatrixIndexExpr{
					Matrix: target,
					Row:    first,
					Col:    col,
				}
				target = matIdx
				break
			}
			// Phase 55: slice in statement position: s[0:5]
			if p.curToken.Type == lexer.TokenColon {
				p.nextToken() // consume ':'
				var end Node
				if p.curToken.Type != lexer.TokenRBracket {
					end = p.parseExpr()
				}
				p.nextToken() // consume ']'
				target = &SliceExpr{Target: target, Start: first, End: end}
				continue
			}
			// Single index: arr[i] = val or arr[i]
			p.nextToken() // consume ']'
			target = &IndexExpr{Left: target, Index: first, Line: line}
		}
		if p.curToken.Type == lexer.TokenAssign {
			p.nextToken() // consume '='
			val := p.parseExpr()
			return p.exprStmtAt(&BinaryExpr{Left: target, Operator: "=", Right: val, Line: line}, line)
		}
		return p.exprStmtAt(target, line)
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
		return p.exprStmtAt(assignExpr, line)
	}

	// Otherwise it's an expression statement
	// We already consumed the ident, so we need to reconstruct
	// Handle function calls and other expressions starting with ident
	left := &Identifier{Name: ident, Col: identCol, EndCol: identCol + len(ident)}

	// Check for function call
	if p.curToken.Type == lexer.TokenLParen {
		p.nextToken() // consume '('
		args := []Node{}
		for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
			start := p.curToken.Start
			args = append(args, p.parseExpr())
			if p.curToken.Type == lexer.TokenComma {
				p.nextToken()
			}
			p.advanceIfStalled(start, "function call arguments")
		}
		p.nextToken() // consume ')'
		return p.exprStmtAt(&CallExpr{Function: ident, Args: args, Line: line, Col: identCol, EndCol: identCol + len(ident)}, line)
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
				start := p.curToken.Start
				args = append(args, p.parseExpr())
				if p.curToken.Type == lexer.TokenComma {
					p.nextToken()
				}
				p.advanceIfStalled(start, "function call arguments")
			}
			p.nextToken() // consume ')'
			callName := rightIdent
			isCFunc := ident == "C"
			module := ""
			if isCFunc {
				callName = "C." + rightIdent
			} else if ident != "" {
				module = ident
			}
			return p.exprStmtAt(&CallExpr{
				Function: callName,
				Module:   module,
				Args:     args,
				IsCFunc:  isCFunc,
				Line:     line,
				Col:      identCol,
				EndCol:   identCol + len(ident) + 1 + len(rightIdent),
			}, line)
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
			return p.exprStmtAt(&BinaryExpr{Left: dotExpr, Operator: "=", Right: val}, line)
		}
		return p.exprStmtAt(dotExpr, line)
	}

	// Check for binary operator (assignment via expression)
	if p.curToken.Type == lexer.TokenPlus || p.curToken.Type == lexer.TokenMinus ||
		p.curToken.Type == lexer.TokenStar || p.curToken.Type == lexer.TokenSlash ||
		p.curToken.Type == lexer.TokenPercent || p.curToken.Type == lexer.TokenEqual ||
		p.curToken.Type == lexer.TokenNotEqual ||
		p.curToken.Type == lexer.TokenLessThan || p.curToken.Type == lexer.TokenGreaterThan ||
		p.curToken.Type == lexer.TokenLessEqual || p.curToken.Type == lexer.TokenGreaterEqual ||
		p.curToken.Type == lexer.TokenAnd || p.curToken.Type == lexer.TokenOr {
		return p.exprStmtAt(p.parseBinaryExpr(left, 0), line)
	}

	// Phase 19: Handle send operator: channel <- message
	if p.curToken.Type == lexer.TokenSend {
		p.nextToken() // consume '<-'
		message := p.parseExpr()
		return p.exprStmtAt(&SendExpr{Channel: left, Message: message}, line)
	}

	return p.exprStmtAt(left, line)
}

func (p *Parser) parseExpr() Node {
	p.arena.SetLine(int(p.curToken.Line))
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
		// Apply postfix operators (index/matrix/slice, @derive/@tag, ?) before
		// considering a binary operator, and re-check operators afterwards, so
		// that a[i][j] + b and a[i] parse correctly. Postfix binds tighter than
		// binary operators.
		for p.curToken.Type == lexer.TokenLBracket {
			p.nextToken() // consume '['
			var first Node
			if p.curToken.Type == lexer.TokenColon { // Phase 55: open start [:n]
				first = &IntLiteral{Value: "0"}
			} else {
				first = p.parseExpr()
			}
			if p.curToken.Type == lexer.TokenComma {
				// Matrix index: [row, col]
				p.nextToken() // consume ','
				col := p.parseExpr()
				p.nextToken() // consume ']'
				left = p.arena.AllocMatrixIndexExpr(left, first, col)
			} else if p.curToken.Type == lexer.TokenColon { // Phase 55: slice [a:b]
				p.nextToken() // consume ':'
				var end Node
				if p.curToken.Type != lexer.TokenRBracket {
					end = p.parseExpr()
				}
				p.nextToken() // consume ']'
				left = &SliceExpr{Target: left, Start: first, End: end}
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

		// Handle assignment: ident = expr (bind loosest, only at min precedence 0)
		if p.curToken.Type == lexer.TokenAssign && minPrec == 0 && left != nil {
			p.nextToken() // consume '='
			right := p.parseExpr()
			left = p.arena.AllocBinaryExpr(left, "=", right)
			continue
		}

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
	p.arena.SetLine(int(p.curToken.Line))
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
		// Phase 81: clear the unparenthesized-if flag inside '(' groups so
		// struct literals such as `if x < (Point{y: 1}).z { ... }` still parse.
		savedIfCondition := p.unparenthesizedIfCondition
		p.unparenthesizedIfCondition = false
		p.nextToken() // consume '('
		expr := p.parseExpr()
		p.nextToken() // consume ')'
		p.unparenthesizedIfCondition = savedIfCondition
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
	case lexer.TokenMeasure:
		// Phase 14: measure(qr[0]) in expression position — yields a classical bit
		return p.parseMeasure()
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
						start := p.curToken.Start
						args = append(args, p.parseExpr())
						p.advanceIfStalled(start, "simd builtin arguments")
					}
				}
				p.nextToken() // consume ')'
				return &SIMDBuiltinExpr{Op: simdOp, Args: args}
			}
			if strings.HasPrefix(op, "atomic_") {
				atomicOp := strings.TrimPrefix(op, "atomic_")
				// @atomic_load(ptr, ordering), @atomic_store(ptr, v, ordering), etc.
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
				// Last argument may be a memory-ordering string literal.
				order := "seq_cst"
				if n := len(args); n >= 2 {
					if lit, isStr := args[n-1].(*StringLiteral); isStr {
						if o, ok := normalizeOrdering(lit.Value); ok {
							order = o
							args = args[:n-1]
						}
					}
				}
				return &AtomicOp{Op: atomicOp, Args: args, Order: order}
			}
			return nil
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
			start := p.curToken.Start
			elements = append(elements, p.parseExpr())
			p.advanceIfStalled(start, "array literal")
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
			start := p.curToken.Start
			key = p.parseExpr()
			p.nextToken() // consume ':'
			val = p.parseExpr()
			p.advanceIfStalled(start, "map literal")
			keys = append(keys, key)
			values = append(values, val)
		}
	}
	p.nextToken() // consume '}'
	return &MapLiteral{Keys: keys, Values: values}
}

func (p *Parser) parseIdentExpr() Node {
	ident := p.curToken.Literal(p.src)
	identCol := int(p.curToken.Col)
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
				start := p.curToken.Start
				args = append(args, p.parseExpr())
				if p.curToken.Type == lexer.TokenComma {
					p.nextToken()
				}
				p.advanceIfStalled(start, "function call arguments")
			}
			p.nextToken() // consume ')'

			if p.enumNames[ident] {
				var val Node
				if len(args) == 1 {
					val = args[0]
				}
				return &EnumVariantExpr{EnumName: ident, Variant: rightIdent, Value: val}
			}

			// Module-qualified calls (e.g. math.twice(...)) keep their module
			// qualifier so the resolver can validate the call against the
			// module's export set (Phase 103). The flat C symbol model means
			// the emitted function name stays bare. C.* calls keep their
			// qualifier inside the name so codegen can emit raw C invocations.
			callName := rightIdent
			isCFunc := ident == "C"
			module := ""
			if isCFunc {
				callName = "C." + rightIdent
			} else if ident != "" {
				module = ident
			}
			return &CallExpr{
				Function: callName,
				Module:   module,
				Args:     args,
				IsCFunc:  isCFunc,
				Col:      identCol,
				EndCol:   identCol + len(ident) + 1 + len(rightIdent),
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
			start := p.curToken.Start
			args = append(args, p.parseExpr())
			if p.curToken.Type == lexer.TokenComma {
				p.nextToken()
			}
			p.advanceIfStalled(start, "function call arguments")
		}
		p.nextToken() // consume ')'

		call := p.arena.AllocCallExpr(ident, args, false)
		call.Col = identCol
		call.EndCol = identCol + len(ident)
		return call
	}

	// Phase 19: Check for struct literal: TypeName{field: val, ...}
	// Phase 81: inside an unparenthesized `if` condition, a trailing '{' opens
	// the body, not a struct literal — the identifier stays a plain operand so
	// parseIf can consume the body block.
	if p.curToken.Type == lexer.TokenLBrace && !p.unparenthesizedIfCondition {
		return p.parseStructLiteral(ident)
	}

	id := p.arena.AllocIdentifier(ident)
	id.Col = identCol
	id.EndCol = identCol + len(ident)
	return id
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
	nameLine := int(p.curToken.Line)
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
		} else if p.curToken.Type == lexer.TokenSemicolon {
			p.nextToken() // consume ';'
		}
	}
	p.nextToken() // consume '}'

	return &StructDeclStmt{Name: name, Fields: fields, Line: nameLine}
}

// Phase 45: enum declaration parsing: enum Name { Variant, Variant(payload), ... }
func (p *Parser) parseEnumDecl() *EnumDecl {
	p.nextToken() // consume 'enum'
	name := p.curToken.Literal(p.src)
	nameLine := int(p.curToken.Line)
	p.enumNames[name] = true // register enum name
	p.nextToken()            // consume enum name
	p.nextToken()            // consume '{'

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

	return &EnumDecl{Name: name, Variants: variants, Line: nameLine}
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

// parseModuleImport parses `import <module_name>` at the top level. Module
// names may be dotted paths such as `std.core` or `app.mymod`; the full path
// string becomes the import name.
func (p *Parser) parseModuleImport() *ModuleImport {
	start := p.curToken.Start
	p.nextToken() // consume 'import'
	name := p.curToken.Literal(p.src)
	line := int(p.curToken.Line)
	p.nextToken() // consume the first module-path element

	// Consume any dotted path continuation: (. ident)*
	for p.curToken.Type == lexer.TokenDot {
		p.nextToken() // consume '.'
		part := p.curToken.Literal(p.src)
		name += "." + part
		p.advanceIfStalled(start, "module import path")
		p.nextToken() // consume the path element
	}
	return &ModuleImport{Name: name, Line: line}
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

	var qubits Node
	if p.curToken.Type == lexer.TokenLBracket {
		// Bracket form: qreg qr[2]
		p.nextToken() // consume '['
		qubits = p.parseExpr()
		if p.curToken.Type == lexer.TokenRBracket {
			p.nextToken() // consume ']'
		}
	} else {
		// Assignment form: qreg qr = 2
		p.nextToken() // consume '='
		qubits = p.parseExpr()
	}

	return &QRegDeclStmt{Name: name, Qubits: qubits}
}

// quantumGateNames — gates usable with bare statement syntax (H qr[0], CNOT qr[0], qr[1])
var quantumGateNames = map[string]bool{
	"H": true, "X": true, "Y": true, "Z": true,
	"S": true, "T": true, "SX": true,
	"RX": true, "RY": true, "RZ": true, "PHASE": true,
	"CNOT": true, "CZ": true, "SWAP": true, "CCX": true,
}

func isQuantumGate(name string) bool {
	return quantumGateNames[name]
}

// parseBareGateApply parses bare gate syntax: GateName operand [, operand]...
// Convention matches keyword form: for two-qubit gates the FIRST operand is control,
// the SECOND is target (CNOT qr[0], qr[1] → Control=qr[0], Target=qr[1]).
func (p *Parser) parseBareGateApply(gateName string) *GateApplyStmt {
	target := p.parseExpr()

	var control Node = nil
	var params []Node = nil

	// Second operand: two-qubit gate (control) or rotation parameter
	if p.curToken.Type == lexer.TokenComma {
		p.nextToken() // consume ','
		second := p.parseExpr()
		switch gateName {
		case "CNOT", "CZ", "SWAP", "CCX":
			control = target
			target = second
		default:
			params = []Node{second}
		}
	}

	return &GateApplyStmt{
		Gate:    gateName,
		Target:  target,
		Control: control,
		Params:  params,
	}
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

	// Paren form: measure(qr[0])
	if p.curToken.Type == lexer.TokenLParen {
		p.nextToken() // consume '('
		qubit := p.parseExpr()
		if p.curToken.Type == lexer.TokenRParen {
			p.nextToken() // consume ')'
		}
		return &MeasureExpr{Qubit: qubit}
	}

	// Bare form: measure qr[0]
	qubit := p.parseExpr()
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
	p.nextToken()                         // consume type

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
			start := p.curToken.Start
			args = append(args, p.parseExpr())
			if p.curToken.Type == lexer.TokenComma {
				p.nextToken() // consume ','
			}
			p.advanceIfStalled(start, "spawn arguments")
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
	p.nextToken()                         // consume type

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
