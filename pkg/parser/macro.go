package parser

import (
	"karkain/pkg/lexer"
)

// MacroExpander handles compile-time macro expansion
type MacroExpander struct {
	macros map[string]*MacroDeclStmt
}

// NewMacroExpander creates a new macro expander
func NewMacroExpander() *MacroExpander {
	return &MacroExpander{
		macros: make(map[string]*MacroDeclStmt),
	}
}

// RegisterMacro registers a macro for later expansion
func (me *MacroExpander) RegisterMacro(macro *MacroDeclStmt) {
	me.macros[macro.Name] = macro
}

// ExpandProgram expands all macros in a program
func (me *MacroExpander) ExpandProgram(prog *Program) *Program {
	newProg := &Program{
		Statements: []Node{},
		CImports:   prog.CImports,
		Imports:    prog.Imports,
	}

	// First pass: collect all macro declarations
	for _, stmt := range prog.Statements {
		if macro, ok := stmt.(*MacroDeclStmt); ok {
			me.RegisterMacro(macro)
		}
	}

	// Second pass: expand macros and process statements
	for _, stmt := range prog.Statements {
		switch s := stmt.(type) {
		case *MacroDeclStmt:
			// Skip macro declarations in output
			continue
		case *ComptimeStmt:
			// Execute comptime blocks and expand their results
			expanded := me.expandComptime(s)
			newProg.Statements = append(newProg.Statements, expanded...)
		default:
			// Recursively expand macros in the statement
			expanded := me.expandNode(stmt)
			newProg.Statements = append(newProg.Statements, expanded)
		}
	}

	return newProg
}

// expandNode recursively expands macros in a node
func (me *MacroExpander) expandNode(node Node) Node {
	switch n := node.(type) {
	case *MacroExpandExpr:
		return me.expandMacroCall(n)
	case *QuoteExpr:
		// Quote preserves the AST as data
		return n
	case *UnquoteExpr:
		// Unquote evaluates the expression
		return me.expandNode(n.Expr)
	case *ComptimeExpr:
		// Evaluate comptime expression
		return me.evaluateComptimeExpr(n)
	case *ReflectTypeExpr:
		// Handle reflection expressions
		return me.expandReflection(n)
	case *DeriveExpr:
		// Handle trait derivation
		return me.expandDerive(n)
	case *TagExpr:
		// Handle tag expressions
		return me.expandTag(n)
	case *FuncDecl:
		// Expand function body
		newFunc := &FuncDecl{
			Name:   n.Name,
			Params: n.Params,
			Body:   []Node{},
			Line:   n.Line,
		}
		for _, stmt := range n.Body {
			newFunc.Body = append(newFunc.Body, me.expandNode(stmt))
		}
		return newFunc
	case *VarDeclStmt:
		// Expand variable value
		return &VarDeclStmt{
			Name:     n.Name,
			Value:    me.expandNode(n.Value),
			Type:     n.Type,
			IsMatrix: n.IsMatrix,
			Line:     n.Line,
		}
	case *BinaryExpr:
		return &BinaryExpr{
			Left:     me.expandNode(n.Left),
			Operator: n.Operator,
			Right:    me.expandNode(n.Right),
			Line:     n.Line,
		}
	case *CallExpr:
		newArgs := []Node{}
		for _, arg := range n.Args {
			newArgs = append(newArgs, me.expandNode(arg))
		}
		return &CallExpr{
			Function: n.Function,
			Args:     newArgs,
			IsCFunc:  n.IsCFunc,
			Line:     n.Line,
		}
	case *ArrayLiteral:
		newElements := []Node{}
		for _, elem := range n.Elements {
			newElements = append(newElements, me.expandNode(elem))
		}
		return &ArrayLiteral{Elements: newElements, Line: n.Line}
	case *MapLiteral:
		newKeys := []Node{}
		newValues := []Node{}
		for i, key := range n.Keys {
			newKeys = append(newKeys, me.expandNode(key))
			newValues = append(newValues, me.expandNode(n.Values[i]))
		}
		return &MapLiteral{Keys: newKeys, Values: newValues, Line: n.Line}
	case *IndexExpr:
		return &IndexExpr{
			Left:  me.expandNode(n.Left),
			Index: me.expandNode(n.Index),
			Line:  n.Line,
		}
	case *IfStmt:
		newIf := &IfStmt{
			Condition:   me.expandNode(n.Condition),
			Consequence: []Node{},
			Alternative: []Node{},
			Line:        n.Line,
		}
		for _, stmt := range n.Consequence {
			newIf.Consequence = append(newIf.Consequence, me.expandNode(stmt))
		}
		for _, stmt := range n.Alternative {
			newIf.Alternative = append(newIf.Alternative, me.expandNode(stmt))
		}
		return newIf
	case *ExprStmt:
		return &ExprStmt{Expression: me.expandNode(n.Expression), Line: n.Line}
	case *ReturnStmt:
		return &ReturnStmt{Value: me.expandNode(n.Value), Line: n.Line}
	case *PrintStmt:
		return &PrintStmt{Value: me.expandNode(n.Value), Line: n.Line}
	default:
		// For literals and other nodes, return as-is
		return n
	}
}

// expandMacroCall expands a macro invocation
func (me *MacroExpander) expandMacroCall(call *MacroExpandExpr) Node {
	macro, exists := me.macros[call.MacroName]
	if !exists {
		// Return the call as-is if macro not found (will be caught later)
		return call
	}

	// Create a simple macro expansion environment
	env := make(map[string]Node)
	if len(macro.Params) != len(call.Args) {
		// Mismatch in parameter count
		return call
	}

	for i, param := range macro.Params {
		env[param] = call.Args[i]
	}

	// Expand the macro body with parameter substitution
	expandedBody := []Node{}
	for _, stmt := range macro.Body {
		expandedBody = append(expandedBody, me.substituteAndExpand(stmt, env))
	}

	// If the macro expands to a single expression, return it directly
	if len(expandedBody) == 1 {
		return expandedBody[0]
	}

	// Otherwise, wrap in a block expression
	return &ExprStmt{Expression: &ArrayLiteral{Elements: expandedBody}}
}

// substituteAndExpand substitutes macro parameters and expands recursively
func (me *MacroExpander) substituteAndExpand(node Node, env map[string]Node) Node {
	switch n := node.(type) {
	case *Identifier:
		if replacement, exists := env[n.Name]; exists {
			return replacement
		}
		return n
	case *UnquoteExpr:
		// Unquote triggers evaluation
		return me.expandNode(n.Expr)
	default:
		// Recursively process child nodes
		return me.expandNode(node)
	}
}

// expandComptime executes a comptime block
func (me *MacroExpander) expandComptime(stmt *ComptimeStmt) []Node {
	// In a real implementation, this would execute the code at compile time
	// For now, we'll just expand the body and return it
	result := []Node{}
	for _, node := range stmt.Body {
		result = append(result, me.expandNode(node))
	}
	return result
}

// evaluateComptimeExpr evaluates a comptime expression
func (me *MacroExpander) evaluateComptimeExpr(expr *ComptimeExpr) Node {
	// In a real implementation, this would evaluate the expression at compile time
	// For now, just expand the inner expression
	return me.expandNode(expr.Expr)
}

// expandReflection handles reflection expressions
func (me *MacroExpander) expandReflection(expr *ReflectTypeExpr) Node {
	// In a real implementation, this would generate reflection metadata
	// For now, return as-is for codegen to handle
	return expr
}

// expandDerive handles trait derivation
func (me *MacroExpander) expandDerive(expr *DeriveExpr) Node {
	// In a real implementation, this would synthesize methods based on the trait
	// For now, return as-is for codegen to handle
	return expr
}

// expandTag handles tag expressions
func (me *MacroExpander) expandTag(expr *TagExpr) Node {
	// In a real implementation, this would attach metadata to the target
	// For now, return as-is for codegen to handle
	return expr
}

// AddMacroToParser adds macro parsing support to the parser
func (p *Parser) parseMacro() *MacroDeclStmt {
	p.nextToken() // consume 'macro'
	name := p.curToken.Literal(p.src)
	p.nextToken() // consume macro name

	// Parse parameters
	params := []string{}
	if p.curToken.Type == lexer.TokenLParen {
		p.nextToken() // consume '('
		for p.curToken.Type != lexer.TokenRParen && p.curToken.Type != lexer.TokenEOF {
			if p.curToken.Type == lexer.TokenIdent {
				params = append(params, p.curToken.Literal(p.src))
				p.nextToken()
			}
			if p.curToken.Type == lexer.TokenComma {
				p.nextToken()
			}
		}
		p.nextToken() // consume ')'
	}

	// Parse body
	p.nextToken() // consume '{'
	body := []Node{}
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		body = append(body, p.parseStmt())
	}
	p.nextToken() // consume '}'

	return &MacroDeclStmt{
		Name:       name,
		Params:     params,
		Body:       body,
		IsHygienic: true, // Default to hygienic macros
	}
}

// parseStmt parses a statement (including macro-related statements)
func (p *Parser) parseStmt() Node {
	switch p.curToken.Type {
	case lexer.TokenMacro:
		return p.parseMacro()
	case lexer.TokenComptime:
		return p.parseComptimeStmt()
	case lexer.TokenLet, lexer.TokenVar:
		return p.parseVarDecl()
	case lexer.TokenPrint:
		return p.parsePrint()
	case lexer.TokenReturn:
		p.nextToken()
		return &ReturnStmt{Value: p.parseExpr()}
	case lexer.TokenIf:
		return p.parseIfStmt()
	default:
		return &ExprStmt{Expression: p.parseExpr()}
	}
}

// parseComptimeStmt parses a comptime statement block
func (p *Parser) parseComptimeStmt() *ComptimeStmt {
	p.nextToken() // consume 'comptime'
	p.nextToken() // consume '{'

	body := []Node{}
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		body = append(body, p.parseStmt())
	}
	p.nextToken() // consume '}'

	return &ComptimeStmt{Body: body}
}

// parseExprExtended extends expression parsing to handle macro constructs
func (p *Parser) parseExprExtended() Node {
	left := p.parsePrimaryExpr()

	// Handle @derive and @tag
	if p.curToken.Type == lexer.TokenAt {
		p.nextToken() // consume '@'
		ident := p.curToken.Literal(p.src)
		p.nextToken() // consume identifier

		if ident == "derive" {
			p.nextToken() // consume '('
			trait := p.curToken.Literal(p.src)
			p.nextToken() // consume trait name
			p.nextToken() // consume ')'
			return &DeriveExpr{
				Trait:  trait,
				Target: left,
			}
		} else if ident == "tag" {
			p.nextToken() // consume '('
			tagName := p.curToken.Literal(p.src)
			p.nextToken() // consume tag name
			p.nextToken() // consume ','
			tagValue := p.curToken.Literal(p.src)
			p.nextToken() // consume tag value
			p.nextToken() // consume ')'
			return &TagExpr{
				Target:   left,
				TagName:  tagName,
				TagValue: tagValue,
			}
		}
	}

	// Handle quote and unquote
	if p.curToken.Type == lexer.TokenQuote {
		p.nextToken() // consume 'quote'
		p.nextToken() // consume '('
		expr := p.parseExpr()
		p.nextToken() // consume ')'
		return &QuoteExpr{Expr: expr}
	}

	if p.curToken.Type == lexer.TokenUnquote {
		p.nextToken() // consume 'unquote'
		p.nextToken() // consume '('
		expr := p.parseExpr()
		p.nextToken() // consume ')'
		return &UnquoteExpr{Expr: expr}
	}

	// Handle reflect.typeof()
	if ident, ok := left.(*Identifier); ok && ident.Name == "reflect" {
		if p.curToken.Type == lexer.TokenDot {
			p.nextToken() // consume '.'
			method := p.curToken.Literal(p.src)
			p.nextToken() // consume method name
			if method == "typeof" {
				p.nextToken() // consume '('
				typeExpr := p.parseExpr()
				p.nextToken() // consume ')'
				return &ReflectTypeExpr{TypeExpr: typeExpr}
			}
		}
	}

	return left
}

// Helper function to integrate with existing parser
func (p *Parser) parseIfStmt() *IfStmt {
	p.nextToken() // consume 'if'
	p.nextToken() // consume '('
	condition := p.parseExpr()
	p.nextToken() // consume ')'
	p.nextToken() // consume '{'

	consequence := []Node{}
	for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
		consequence = append(consequence, p.parseStmt())
	}
	p.nextToken() // consume '}'

	alternative := []Node{}
	if p.curToken.Type == lexer.TokenElse {
		p.nextToken() // consume 'else'
		p.nextToken() // consume '{'
		for p.curToken.Type != lexer.TokenRBrace && p.curToken.Type != lexer.TokenEOF {
			alternative = append(alternative, p.parseStmt())
		}
		p.nextToken() // consume '}'
	}

	return &IfStmt{
		Condition:   condition,
		Consequence: consequence,
		Alternative: alternative,
	}
}

// ApplyMacroExpansion applies macro expansion to a parsed program
func ApplyMacroExpansion(prog *Program) *Program {
	expander := NewMacroExpander()
	return expander.ExpandProgram(prog)
}

// Built-in macro: assert_eq
func AssertEqMacro() *MacroDeclStmt {
	return &MacroDeclStmt{
		Name:   "assert_eq",
		Params: []string{"left", "right"},
		Body: []Node{
			&IfStmt{
				Condition: &BinaryExpr{
					Left:     &Identifier{Name: "left"},
					Operator: "!=",
					Right:    &Identifier{Name: "right"},
				},
				Consequence: []Node{
					&PrintStmt{Value: &StringLiteral{Value: "Assertion failed:"}},
					&PrintStmt{Value: &Identifier{Name: "left"}},
					&PrintStmt{Value: &StringLiteral{Value: "!="}},
					&PrintStmt{Value: &Identifier{Name: "right"}},
				},
				Alternative: []Node{},
			},
		},
		IsHygienic: true,
	}
}

// RegisterBuiltinMacros registers standard library macros
func RegisterBuiltinMacros(expander *MacroExpander) {
	expander.RegisterMacro(AssertEqMacro())
}
