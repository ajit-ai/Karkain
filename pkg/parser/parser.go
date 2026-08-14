package parser

import (
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
	input     string // Store input for C import parsing
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, input: l.GetInput()}
	p.nextToken()
	p.nextToken()
	return p
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
		} else if p.curToken.Type == lexer.TokenImport {
			if cImport := p.parseCImport(); cImport != nil {
				prog.CImports = append(prog.CImports, cImport)
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
		if p.curToken.Type == lexer.TokenLet || p.curToken.Type == lexer.TokenVar {
			fn.Body = append(fn.Body, p.parseVarDecl())
		} else if p.curToken.Type == lexer.TokenPrint {
			fn.Body = append(fn.Body, p.parsePrint())
		} else if p.curToken.Type == lexer.TokenMatrix {
			fn.Body = append(fn.Body, p.parseMatrixDecl())
		} else if p.curToken.Type == lexer.TokenAlloc {
			fn.Body = append(fn.Body, p.parseAlloc())
		} else if p.curToken.Type == lexer.TokenFree {
			fn.Body = append(fn.Body, p.parseFree())
		} else if p.curToken.Type == lexer.TokenAddr {
			fn.Body = append(fn.Body, p.parseAddrOf())
		} else if p.curToken.Type == lexer.TokenQReg {
			fn.Body = append(fn.Body, p.parseQRegDecl())
		} else if p.curToken.Type == lexer.TokenGate {
			fn.Body = append(fn.Body, p.parseGateApply())
		} else if p.curToken.Type == lexer.TokenMeasure {
			fn.Body = append(fn.Body, &ExprStmt{Expression: p.parseMeasure()})
		} else if p.curToken.Type == lexer.TokenMacro {
			fn.Body = append(fn.Body, p.parseMacro())
		} else if p.curToken.Type == lexer.TokenComptime {
			fn.Body = append(fn.Body, p.parseComptimeStmt())
		} else if p.curToken.Type == lexer.TokenSpawn {
			fn.Body = append(fn.Body, &ExprStmt{Expression: p.parseSpawn()})
		} else if p.curToken.Type == lexer.TokenReceive {
			fn.Body = append(fn.Body, p.parseReceive())
		} else if p.curToken.Type == lexer.TokenIdent {
			// Check for assignment to matrix index or variable
			ident := p.curToken.Literal
			p.nextToken()

			// Check for matrix index assignment
			if p.curToken.Type == lexer.TokenLBracket {
				p.nextToken() // consume '['
				row := p.parseExpr()
				p.nextToken() // consume ','
				col := p.parseExpr()
				p.nextToken() // consume ']'

				// Expect '='
				if p.curToken.Type == lexer.TokenAssign {
					p.nextToken() // consume '='
					val := p.parseExpr()

					// Create assignment statement
					assignExpr := &BinaryExpr{
						Left: &MatrixIndexExpr{
							Matrix: &Identifier{Name: ident},
							Row:    row,
							Col:    col,
						},
						Operator: "=",
						Right:    val,
					}
					fn.Body = append(fn.Body, &ExprStmt{Expression: assignExpr})
				}
			} else {
				// Simple variable assignment
				if p.curToken.Type == lexer.TokenAssign {
					p.nextToken() // consume '='
					val := p.parseExpr()
					fn.Body = append(fn.Body, &VarDeclStmt{Name: ident, Value: val})
				}
			}
		} else {
			p.nextToken()
		}
	}
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
		// Could be a type annotation like `int` or `float64`
		potentialType := p.curToken.Literal
		if potentialType == "int" || potentialType == "float64" || potentialType == "string" {
			typeName = potentialType
			p.nextToken() // consume type
		}
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

func (p *Parser) parseExpr() Node {
	left := p.parsePrimary()

	// Handle dot expression (e.g., C.sqrt)
	if p.peekToken.Type == lexer.TokenDot {
		if ident, ok := left.(*Identifier); ok {
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
					Function: ident.Name + "." + rightIdent,
					Args:     args,
					IsCFunc:  ident.Name == "C",
				}
			}

			return &DotExpr{Left: left, Right: rightIdent}
		}
	}

	// Handle matrix indexing [row, col]
	if p.peekToken.Type == lexer.TokenLBracket {
		p.nextToken() // consume '['
		row := p.parseExpr()
		p.nextToken() // consume ','
		col := p.parseExpr()
		p.nextToken() // consume ']'
		return &MatrixIndexExpr{Matrix: left, Row: row, Col: col}
	}

	// Handle binary operators
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
	case lexer.TokenFloat64:
		return &Float64Literal{Value: p.curToken.Literal}
	case lexer.TokenIdent:
		ident := &Identifier{Name: p.curToken.Literal}

		// Check for dot expression (e.g., C.sqrt)
		if p.peekToken.Type == lexer.TokenStar {
			// This could be a pointer type in variable declaration
			// Handle in parseVarDecl instead
			return ident
		}

		return ident
	}
	return nil
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
	operand := p.parsePrimary()
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

	var control Node = nil
	var params []Node = nil

	// Check for control qubit (CNOT format: CNOT(control, target))
	if p.curToken.Type == lexer.TokenComma {
		p.nextToken() // consume ','
		control = target
		target = p.parseExpr()
	}

	// Check for parameters (rotation gates)
	if p.curToken.Type == lexer.TokenComma {
		p.nextToken() // consume ','
		params = []Node{p.parseExpr()}
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

	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		if p.curToken.Type == lexer.TokenLet || p.curToken.Type == lexer.TokenVar {
			actor.Body = append(actor.Body, p.parseVarDecl())
		} else if p.curToken.Type == lexer.TokenPrint {
			actor.Body = append(actor.Body, p.parsePrint())
		} else if p.curToken.Type == lexer.TokenReceive {
			actor.Body = append(actor.Body, p.parseReceive())
		} else if p.curToken.Type == lexer.TokenSpawn {
			actor.Body = append(actor.Body, &ExprStmt{Expression: p.parseSpawn()})
		} else if p.curToken.Type == lexer.TokenIdent {
			actor.Body = append(actor.Body, &ExprStmt{Expression: p.parseExpr()})
		} else {
			p.nextToken()
		}
	}

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
	matrix := p.parsePrimary()

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
