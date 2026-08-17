package sema

import (
	"fmt"
	"strconv"
	"strings"

	"karkain/pkg/parser"
)

// ============================================================
// Phase 35: Compile-Time Macro & Metaprogramming Engine
// Supports: @unroll, @derive(Equal,Clone), @target_guard(backend),
//           @inline, @const_eval, macro hygiene
// ============================================================

// MacroKind describes the type of macro
type MacroKind int

const (
	MacroUserDefined MacroKind = iota
	MacroBuiltinUnroll
	MacroBuiltinDerive
	MacroBuiltinTargetGuard
	MacroBuiltinInline
	MacroBuiltinConstEval
	MacroBuiltinStaticAssert
)

// MacroDefinition represents a registered macro
type MacroDefinition struct {
	Name       string
	Kind       MacroKind
	Params     []string
	Body       []parser.Node
	IsHygienic bool
	ExpansionCount int
	MaxExpansions  int // 0 = unlimited
}

// MacroExpander handles compile-time macro expansion
type MacroExpander struct {
	Macros       map[string]*MacroDefinition
	ScopeChain   []*MacroScope
	Expanded     []ExpansionRecord
	MaxDepth     int
	CurrentDepth int
	Errors       []string
}

// MacroScope tracks hygiene context
type MacroScope struct {
	Parent     *MacroScope
	Bindings   map[string]parser.Node
	UniqueId   int
	SourceName string
}

// ExpansionRecord tracks macro expansions for debugging
type ExpansionRecord struct {
	MacroName string
	Line      int
	ExpandedTo int // number of AST nodes produced
}

// ============================================================
// NewMacroExpander
// ============================================================

func NewMacroExpander() *MacroExpander {
	me := &MacroExpander{
		Macros:   make(map[string]*MacroDefinition),
		MaxDepth: 64,
	}
	me.ScopeChain = []*MacroScope{{Bindings: make(map[string]parser.Node)}}
	return me
}

// ============================================================
// Macro Registration
// ============================================================

// RegisterMacro adds a user-defined macro
func (me *MacroExpander) RegisterMacro(name string, params []string, body []parser.Node, hygienic bool) {
	me.Macros[name] = &MacroDefinition{
		Name:       name,
		Kind:       MacroUserDefined,
		Params:     params,
		Body:       body,
		IsHygienic: hygienic,
	}
}

// RegisterBuiltinMacro registers a built-in macro
func (me *MacroExpander) RegisterBuiltinMacro(name string, kind MacroKind) {
	me.Macros[name] = &MacroDefinition{
		Name: name,
		Kind: kind,
		MaxExpansions: 0,
	}
}

// GetMacro returns a macro definition by name
func (me *MacroExpander) GetMacro(name string) (*MacroDefinition, bool) {
	m, ok := me.Macros[name]
	return m, ok
}

// ============================================================
// Scope Management
// ============================================================

func (me *MacroExpander) PushScope(source string) {
	parent := me.ScopeChain[len(me.ScopeChain)-1]
	me.ScopeChain = append(me.ScopeChain, &MacroScope{
		Parent:     parent,
		Bindings:   make(map[string]parser.Node),
		UniqueId:   len(me.ScopeChain),
		SourceName: source,
	})
}

func (me *MacroExpander) PopScope() {
	if len(me.ScopeChain) > 1 {
		me.ScopeChain = me.ScopeChain[:len(me.ScopeChain)-1]
	}
}

func (me *MacroExpander) Bind(name string, val parser.Node) {
	top := me.ScopeChain[len(me.ScopeChain)-1]
	top.Bindings[name] = val
}

func (me *MacroExpander) Lookup(name string) (parser.Node, bool) {
	for i := len(me.ScopeChain) - 1; i >= 0; i-- {
		if val, ok := me.ScopeChain[i].Bindings[name]; ok {
			return val, true
		}
	}
	return nil, false
}

// HygieneRename generates a unique name to avoid capture
func (me *MacroExpander) HygieneRename(name string) string {
	scope := me.ScopeChain[len(me.ScopeChain)-1]
	return fmt.Sprintf("__%s_%d_%d", name, scope.UniqueId, me.CurrentDepth)
}

// ============================================================
// Main Expansion Entry Points
// ============================================================

// ExpandProgram expands all macros in a program
func (me *MacroExpander) ExpandProgram(prog *parser.Program) (*parser.Program, error) {
	// First pass: register all macro declarations
	for _, stmt := range prog.Statements {
		if md, ok := stmt.(*parser.MacroDeclStmt); ok {
			me.RegisterMacro(md.Name, md.Params, md.Body, md.IsHygienic)
		}
	}

	// Second pass: expand all macro invocations
	newStmts := make([]parser.Node, 0, len(prog.Statements))
	for _, stmt := range prog.Statements {
		if _, ok := stmt.(*parser.MacroDeclStmt); ok {
			continue // remove macro declarations from output
		}
		expanded, err := me.ExpandNode(stmt)
		if err != nil {
			return nil, err
		}
		if nodes, ok := expanded.([]parser.Node); ok {
			newStmts = append(newStmts, nodes...)
		} else {
			newStmts = append(newStmts, expanded)
		}
	}

	prog.Statements = newStmts
	return prog, nil
}

// ExpandNode recursively expands macros in any AST node
func (me *MacroExpander) ExpandNode(node parser.Node) (parser.Node, error) {
	if me.CurrentDepth > me.MaxDepth {
		return nil, fmt.Errorf("macro: maximum expansion depth %d exceeded", me.MaxDepth)
	}
	me.CurrentDepth++
	defer func() { me.CurrentDepth-- }()

	switch n := node.(type) {
	case *parser.MacroExpandExpr:
		return me.expandMacroExpr(n)
	case *parser.DeriveExpr:
		return me.expandDeriveExpr(n)
	case *parser.StmtList:
		return me.expandStmtList(n)
	case *parser.FuncDecl:
		return me.expandFuncDecl(n)
	case *parser.ForStmt:
		return me.expandForStmt(n)
	case *parser.IfStmt:
		return me.expandIfStmt(n)
	case *parser.VarDeclStmt:
		return me.expandVarDecl(n)
	case *parser.BinaryExpr:
		return me.expandBinaryExpr(n)
	case *parser.CallExpr:
		return me.expandCallExpr(n)
	default:
		return node, nil
	}
}

// ============================================================
// @unroll — Loop Unrolling Macro
// ============================================================

// UnrollLoop unrolls a for loop at compile time
// @unroll(4) for i in 0..4 { body(i) }
// → { body(0); body(1); body(2); body(3); }
func UnrollLoop(forStmt *parser.ForStmt, count int, expander *MacroExpander) ([]parser.Node, error) {
	if count < 0 || count > 128 {
		return nil, fmt.Errorf("@unroll: count must be 0..128, got %d", count)
	}

	result := make([]parser.Node, 0, count)

	for i := 0; i < count; i++ {
		expander.PushScope(fmt.Sprintf("unroll_%d", i))
		expander.Bind("_index", &parser.IntLiteral{Value: strconv.Itoa(i)})

		for _, bodyStmt := range forStmt.Body {
			substituted := SubstituteTemplate(bodyStmt, map[string]parser.Node{
				"_i":   &parser.IntLiteral{Value: strconv.Itoa(i)},
				"_idx": &parser.IntLiteral{Value: strconv.Itoa(i)},
			}, expander)
			expanded, err := expander.ExpandNode(substituted)
			if err != nil {
				expander.PopScope()
				return nil, err
			}
			if nodes, ok := expanded.([]parser.Node); ok {
				result = append(result, nodes...)
			} else {
				result = append(result, expanded)
			}
		}
		expander.PopScope()
	}

	return result, nil
}

func (me *MacroExpander) expandMacroExpr(m *parser.MacroExpandExpr) (parser.Node, error) {
	macro, ok := me.Macros[m.MacroName]
	if !ok {
		return nil, fmt.Errorf("macro: undefined macro '%s'", m.MacroName)
	}

	if macro.MaxExpansions > 0 && macro.ExpansionCount >= macro.MaxExpansions {
		return nil, fmt.Errorf("macro: '%s' reached max expansion count %d", m.MacroName, macro.MaxExpansions)
	}

	switch macro.Kind {
	case MacroBuiltinUnroll:
		return me.handleUnroll(m)
	case MacroBuiltinDerive:
		return me.handleDerive(m)
	case MacroBuiltinTargetGuard:
		return me.handleTargetGuard(m)
	case MacroBuiltinInline:
		return me.handleInline(m)
	case MacroBuiltinConstEval:
		return me.handleConstEval(m)
	case MacroBuiltinStaticAssert:
		return me.handleStaticAssert(m)
	case MacroUserDefined:
		return me.expandUserDefined(macro, m)
	default:
		return nil, fmt.Errorf("macro: unknown macro kind for '%s'", m.MacroName)
	}
}

func (me *MacroExpander) expandUserDefined(macro *MacroDefinition, call *parser.MacroExpandExpr) (parser.Node, error) {
	if len(call.Args) != len(macro.Params) {
		return nil, fmt.Errorf("macro '%s': expected %d args, got %d",
			macro.Name, len(macro.Params), len(call.Args))
	}

	me.PushScope(macro.Name)
	defer me.PopScope()

	for i, param := range macro.Params {
		me.Bind(param, call.Args[i])
	}

	if macro.IsHygienic {
		// Hygienic: rename all free variables in body
		for _, bodyNode := range macro.Body {
			expanded, err := me.expandHygienic(bodyNode)
			if err != nil {
				return nil, err
			}
			macro.ExpansionCount++
			me.Expanded = append(me.Expanded, ExpansionRecord{
				MacroName: macro.Name, Line: 0, ExpandedTo: 1,
			})
			return expanded, nil
		}
	}

	// Non-hygienic: substitute directly
	for _, bodyNode := range macro.Body {
		substituted := SubstituteBindings(bodyNode, me.ScopeChain[len(me.ScopeChain)-1].Bindings, me)
		expanded, err := me.ExpandNode(substituted)
		if err != nil {
			return nil, err
		}
		macro.ExpansionCount++
		me.Expanded = append(me.Expanded, ExpansionRecord{
			MacroName: macro.Name, Line: 0, ExpandedTo: 1,
		})
		return expanded, nil
	}

	return &parser.StmtList{Statements: []parser.Node{}}, nil
}

func (me *MacroExpander) expandHygienic(node parser.Node) (parser.Node, error) {
	switch n := node.(type) {
	case *parser.Identifier:
		if val, ok := me.Lookup(n.Name); ok {
			return val, nil
		}
		return &parser.Identifier{Name: me.HygieneRename(n.Name)}, nil
	case *parser.VarDeclStmt:
		expanded, err := me.expandHygienic(n.Value)
		if err != nil {
			return nil, err
		}
		newName := me.HygieneRename(n.Name)
		return &parser.VarDeclStmt{Name: newName, Value: expanded, Type: n.Type}, nil
	case *parser.StmtList:
		stmts := make([]parser.Node, len(n.Statements))
		for i, s := range n.Statements {
			expanded, err := me.expandHygienic(s)
			if err != nil {
				return nil, err
			}
			stmts[i] = expanded
		}
		return &parser.StmtList{Statements: stmts}, nil
	default:
		return node, nil
	}
}

// ============================================================
// @derive(Equal, Clone, etc.)
// ============================================================

func (me *MacroExpander) handleDerive(m *parser.MacroExpandExpr) (parser.Node, error) {
	if len(m.Args) == 0 {
		return nil, fmt.Errorf("@derive: at least one trait name required")
	}

	traits := make([]string, 0, len(m.Args))
	for _, arg := range m.Args {
		if id, ok := arg.(*parser.Identifier); ok {
			traits = append(traits, id.Name)
		} else {
			return nil, fmt.Errorf("@derive: trait argument must be an identifier")
		}
	}

	result := &parser.StmtList{Statements: []parser.Node{}}

	for _, trait := range traits {
		switch strings.ToLower(trait) {
		case "equal":
			result.Statements = append(result.Statements, me.generateEqualImpl())
		case "clone":
			result.Statements = append(result.Statements, me.generateCloneImpl())
		case "string":
			result.Statements = append(result.Statements, me.generateStringImpl())
		case "hash":
			result.Statements = append(result.Statements, me.generateHashImpl())
		case "ord":
			result.Statements = append(result.Statements, me.generateOrdImpl())
		case "debug":
			result.Statements = append(result.Statements, me.generateDebugImpl())
		case "jsonserializable":
			result.Statements = append(result.Statements, me.generateJSONImpl())
		case "default":
			result.Statements = append(result.Statements, me.generateDefaultImpl())
		case "copy":
			result.Statements = append(result.Statements, me.generateCopyImpl())
		case "printable":
			result.Statements = append(result.Statements, me.generatePrintableImpl())
		default:
			return nil, fmt.Errorf("@derive: unknown trait '%s'", trait)
		}
	}

	return result, nil
}

func (me *MacroExpander) generateEqualImpl() parser.Node {
	return &parser.MacroExpandExpr{
		MacroName: "__generated_equal",
		Args:      []parser.Node{&parser.Identifier{Name: "self"}, &parser.Identifier{Name: "other"}},
	}
}

func (me *MacroExpander) generateCloneImpl() parser.Node {
	return &parser.MacroExpandExpr{
		MacroName: "__generated_clone",
		Args:      []parser.Node{&parser.Identifier{Name: "self"}},
	}
}

func (me *MacroExpander) generateStringImpl() parser.Node {
	return &parser.MacroExpandExpr{
		MacroName: "__generated_string",
		Args:      []parser.Node{&parser.Identifier{Name: "self"}},
	}
}

func (me *MacroExpander) generateHashImpl() parser.Node {
	return &parser.MacroExpandExpr{
		MacroName: "__generated_hash",
		Args:      []parser.Node{&parser.Identifier{Name: "self"}},
	}
}

func (me *MacroExpander) generateOrdImpl() parser.Node {
	return &parser.MacroExpandExpr{
		MacroName: "__generated_ord",
		Args:      []parser.Node{&parser.Identifier{Name: "self"}, &parser.Identifier{Name: "other"}},
	}
}

func (me *MacroExpander) generateDebugImpl() parser.Node {
	return &parser.MacroExpandExpr{
		MacroName: "__generated_debug",
		Args:      []parser.Node{&parser.Identifier{Name: "self"}},
	}
}

func (me *MacroExpander) generateJSONImpl() parser.Node {
	return &parser.MacroExpandExpr{
		MacroName: "__generated_json",
		Args:      []parser.Node{&parser.Identifier{Name: "self"}},
	}
}

func (me *MacroExpander) generateDefaultImpl() parser.Node {
	return &parser.MacroExpandExpr{
		MacroName: "__generated_default",
		Args:      []parser.Node{},
	}
}

func (me *MacroExpander) generateCopyImpl() parser.Node {
	return &parser.MacroExpandExpr{
		MacroName: "__generated_copy",
		Args:      []parser.Node{&parser.Identifier{Name: "self"}},
	}
}

func (me *MacroExpander) generatePrintableImpl() parser.Node {
	return &parser.MacroExpandExpr{
		MacroName: "__generated_printable",
		Args:      []parser.Node{&parser.Identifier{Name: "self"}},
	}
}

// ============================================================
// @target_guard — Backend-Specific Code Guard
// ============================================================

// TargetGuard tracks the active compilation target
type TargetGuard struct {
	CurrentTarget string
	Registered    map[string]bool
}

// TargetBackend names
const (
	BackendCPU      = "cpu"
	BackendGPU      = "gpu"
	BackendQPU      = "qpu"
	BackendWASM     = "wasm"
	BackendSPIRV    = "spirv"
	BackendLLVM     = "llvm"
	BackendOpenQASM = "openqasm"
	BackendQIR      = "qir"
	BackendOpenPulse = "openpulse"
)

func (me *MacroExpander) handleTargetGuard(m *parser.MacroExpandExpr) (parser.Node, error) {
	if len(m.Args) < 1 {
		return nil, fmt.Errorf("@target_guard: requires at least 1 argument (backend name)")
	}

	target, ok := m.Args[0].(*parser.StringLiteral)
	if !ok {
		return nil, fmt.Errorf("@target_guard: first argument must be a string literal")
	}
	_ = target // used for future backend filtering

	// For now, return the body as a statement list (actual backend filtering happens at codegen)
	if len(m.Args) > 1 {
		return m.Args[1], nil
	}
	return &parser.StmtList{Statements: []parser.Node{}}, nil
}

// ============================================================
// @inline — Inline Expansion
// ============================================================

func (me *MacroExpander) handleInline(m *parser.MacroExpandExpr) (parser.Node, error) {
	if len(m.Args) != 1 {
		return nil, fmt.Errorf("@inline: expected 1 argument (function call)")
	}
	// Inline: just return the expression to be inlined by codegen
	return m.Args[0], nil
}

// ============================================================
// @const_eval — Compile-Time Evaluation
// ============================================================

func (me *MacroExpander) handleConstEval(m *parser.MacroExpandExpr) (parser.Node, error) {
	if len(m.Args) != 1 {
		return nil, fmt.Errorf("@const_eval: expected 1 argument")
	}
	// Try to evaluate arithmetic expressions at compile time
	result, ok := EvalConstExpr(m.Args[0])
	if !ok {
		return nil, fmt.Errorf("@const_eval: cannot evaluate expression at compile time")
	}
	return result, nil
}

// ============================================================
// @static_assert — Compile-Time Assertions
// ============================================================

func (me *MacroExpander) handleStaticAssert(m *parser.MacroExpandExpr) (parser.Node, error) {
	if len(m.Args) < 1 {
		return nil, fmt.Errorf("@static_assert: expected condition argument")
	}
	result, ok := EvalConstExpr(m.Args[0])
	if !ok {
		return nil, fmt.Errorf("@static_assert: cannot evaluate assertion at compile time")
	}
	if bl, ok := result.(*parser.BoolLiteral); ok && !bl.Value {
		msg := "assertion failed"
		if len(m.Args) > 1 {
			if sl, ok := m.Args[1].(*parser.StringLiteral); ok {
				msg = sl.Value
			}
		}
		return nil, fmt.Errorf("@static_assert: %s", msg)
	}
	return &parser.StmtList{Statements: []parser.Node{}}, nil
}

// ============================================================
// @for_each — Code Generation Loop Macro
// ============================================================

// ForEach expands a template for each element in a list
// @for_each(item in items) { print(item) }
func ForEach(items []parser.Node, paramName string, template parser.Node, expander *MacroExpander) ([]parser.Node, error) {
	result := make([]parser.Node, 0, len(items))
	for i, item := range items {
		expander.PushScope(fmt.Sprintf("foreach_%d", i))
		expander.Bind(paramName, item)
		substituted := SubstituteBindings(template, expander.ScopeChain[len(expander.ScopeChain)-1].Bindings, expander)
		expanded, err := expander.ExpandNode(substituted)
		if err != nil {
			expander.PopScope()
			return nil, err
		}
		expander.PopScope()
		if nodes, ok := expanded.([]parser.Node); ok {
			result = append(result, nodes...)
		} else {
			result = append(result, expanded)
		}
	}
	return result, nil
}

// ============================================================
// Template Substitution
// ============================================================

// SubstituteTemplate replaces placeholders in an AST node
func SubstituteTemplate(node parser.Node, vars map[string]parser.Node, expander *MacroExpander) parser.Node {
	switch n := node.(type) {
	case *parser.Identifier:
		if replacement, ok := vars[n.Name]; ok {
			return replacement
		}
		return n
	case *parser.BinaryExpr:
		return &parser.BinaryExpr{
			Left:     SubstituteTemplate(n.Left, vars, expander),
			Operator: n.Operator,
			Right:    SubstituteTemplate(n.Right, vars, expander),
		}
	case *parser.UnaryExpr:
		return &parser.UnaryExpr{
			Operator: n.Operator,
			Operand:  SubstituteTemplate(n.Operand, vars, expander),
		}
	case *parser.CallExpr:
		args := make([]parser.Node, len(n.Args))
		for i, arg := range n.Args {
			args[i] = SubstituteTemplate(arg, vars, expander)
		}
		return &parser.CallExpr{Function: n.Function, Args: args, IsCFunc: n.IsCFunc}
	case *parser.VarDeclStmt:
		return &parser.VarDeclStmt{
			Name:  n.Name,
			Value: SubstituteTemplate(n.Value, vars, expander),
			Type:  n.Type,
		}
	case *parser.ReturnStmt:
		return &parser.ReturnStmt{Value: SubstituteTemplate(n.Value, vars, expander)}
	case *parser.IfStmt:
		return &parser.IfStmt{
			Condition:   SubstituteTemplate(n.Condition, vars, expander),
			Consequence: n.Consequence,
			Alternative: n.Alternative,
		}
	case *parser.PrintStmt:
		return &parser.PrintStmt{Value: SubstituteTemplate(n.Value, vars, expander)}
	case *parser.ExprStmt:
		return &parser.ExprStmt{Expression: SubstituteTemplate(n.Expression, vars, expander)}
	case *parser.IntLiteral, *parser.Float64Literal, *parser.StringLiteral, *parser.BoolLiteral:
		return n
	default:
		return n
	}
}

// SubstituteBindings replaces variables using scope bindings
func SubstituteBindings(node parser.Node, bindings map[string]parser.Node, expander *MacroExpander) parser.Node {
	return SubstituteTemplate(node, bindings, expander)
}

// ============================================================
// Compile-Time Expression Evaluation
// ============================================================

// EvalConstExpr evaluates a constant expression at compile time
func EvalConstExpr(node parser.Node) (parser.Node, bool) {
	switch n := node.(type) {
	case *parser.IntLiteral:
		return n, true
	case *parser.Float64Literal:
		return n, true
	case *parser.BoolLiteral:
		return n, true
	case *parser.StringLiteral:
		return n, true
	case *parser.BinaryExpr:
		return evalBinaryConst(n)
	case *parser.UnaryExpr:
		return evalUnaryConst(n)
	default:
		return nil, false
	}
}

func evalBinaryConst(expr *parser.BinaryExpr) (parser.Node, bool) {
	left, lok := EvalConstExpr(expr.Left)
	right, rok := EvalConstExpr(expr.Right)
	if !lok || !rok {
		return nil, false
	}

	switch expr.Operator {
	case "+", "-", "*", "/", "%":
		return evalArith(left, right, expr.Operator)
	case "==", "!=", "<", ">", "<=", ">=":
		return evalCompare(left, right, expr.Operator)
	case "&&", "and":
		lb, lok := left.(*parser.BoolLiteral)
		rb, rok := right.(*parser.BoolLiteral)
		if lok && rok {
			return &parser.BoolLiteral{Value: lb.Value && rb.Value}, true
		}
	case "||", "or":
		lb, lok := left.(*parser.BoolLiteral)
		rb, rok := right.(*parser.BoolLiteral)
		if lok && rok {
			return &parser.BoolLiteral{Value: lb.Value || rb.Value}, true
		}
	}
	return nil, false
}

func evalUnaryConst(expr *parser.UnaryExpr) (parser.Node, bool) {
	operand, ok := EvalConstExpr(expr.Operand)
	if !ok {
		return nil, false
	}
	switch expr.Operator {
	case "-":
		if il, ok := operand.(*parser.IntLiteral); ok {
			v, _ := strconv.Atoi(il.Value)
			return &parser.IntLiteral{Value: strconv.Itoa(-v)}, true
		}
		if fl, ok := operand.(*parser.Float64Literal); ok {
			v, _ := strconv.ParseFloat(fl.Value, 64)
			return &parser.Float64Literal{Value: strconv.FormatFloat(-v, 'f', -1, 64)}, true
		}
	case "!", "not":
		if bl, ok := operand.(*parser.BoolLiteral); ok {
			return &parser.BoolLiteral{Value: !bl.Value}, true
		}
	}
	return nil, false
}

func evalArith(left, right parser.Node, op string) (parser.Node, bool) {
	lil, lok := left.(*parser.IntLiteral)
	ril, rok := right.(*parser.IntLiteral)
	if lok && rok {
		lv, _ := strconv.Atoi(lil.Value)
		rv, _ := strconv.Atoi(ril.Value)
		var result int
		switch op {
		case "+":
			result = lv + rv
		case "-":
			result = lv - rv
		case "*":
			result = lv * rv
		case "/":
			if rv == 0 {
				return nil, false
			}
			result = lv / rv
		case "%":
			if rv == 0 {
				return nil, false
			}
			result = lv % rv
		default:
			return nil, false
		}
		return &parser.IntLiteral{Value: strconv.Itoa(result)}, true
	}

	lfl, lok := left.(*parser.Float64Literal)
	rfl, rok := right.(*parser.Float64Literal)
	if lok && rok {
		lv, _ := strconv.ParseFloat(lfl.Value, 64)
		rv, _ := strconv.ParseFloat(rfl.Value, 64)
		var result float64
		switch op {
		case "+":
			result = lv + rv
		case "-":
			result = lv - rv
		case "*":
			result = lv * rv
		case "/":
			if rv == 0 {
				return nil, false
			}
			result = lv / rv
		default:
			return nil, false
		}
		return &parser.Float64Literal{Value: strconv.FormatFloat(result, 'f', -1, 64)}, true
	}

	return nil, false
}

func evalCompare(left, right parser.Node, op string) (parser.Node, bool) {
	lil, lok := left.(*parser.IntLiteral)
	ril, rok := right.(*parser.IntLiteral)
	if lok && rok {
		lv, _ := strconv.Atoi(lil.Value)
		rv, _ := strconv.Atoi(ril.Value)
		var result bool
		switch op {
		case "==":
			result = lv == rv
		case "!=":
			result = lv != rv
		case "<":
			result = lv < rv
		case ">":
			result = lv > rv
		case "<=":
			result = lv <= rv
		case ">=":
			result = lv >= rv
		default:
			return nil, false
		}
		return &parser.BoolLiteral{Value: result}, true
	}

	lfl, lok := left.(*parser.Float64Literal)
	rfl, rok := right.(*parser.Float64Literal)
	if lok && rok {
		lv, _ := strconv.ParseFloat(lfl.Value, 64)
		rv, _ := strconv.ParseFloat(rfl.Value, 64)
		var result bool
		switch op {
		case "==":
			result = lv == rv
		case "!=":
			result = lv != rv
		case "<":
			result = lv < rv
		case ">":
			result = lv > rv
		case "<=":
			result = lv <= rv
		case ">=":
			result = lv >= rv
		default:
			return nil, false
		}
		return &parser.BoolLiteral{Value: result}, true
	}

	ls, lok := left.(*parser.StringLiteral)
	rs, rok := right.(*parser.StringLiteral)
	if lok && rok {
		switch op {
		case "==":
			return &parser.BoolLiteral{Value: ls.Value == rs.Value}, true
		case "!=":
			return &parser.BoolLiteral{Value: ls.Value != rs.Value}, true
		}
	}

	return nil, false
}

// ============================================================
// Helper Expansion Functions
// ============================================================

func (me *MacroExpander) expandForStmt(n *parser.ForStmt) (parser.Node, error) {
	body := make([]parser.Node, len(n.Body))
	for i, stmt := range n.Body {
		expanded, err := me.ExpandNode(stmt)
		if err != nil {
			return nil, err
		}
		body[i] = expanded
	}
	return &parser.ForStmt{
		Init:      n.Init,
		Condition: n.Condition,
		Post:      n.Post,
		Body:      body,
	}, nil
}

func (me *MacroExpander) expandIfStmt(n *parser.IfStmt) (parser.Node, error) {
	cond, err := me.ExpandNode(n.Condition)
	if err != nil {
		return nil, err
	}
	cons := make([]parser.Node, len(n.Consequence))
	for i, stmt := range n.Consequence {
		expanded, err := me.ExpandNode(stmt)
		if err != nil {
			return nil, err
		}
		cons[i] = expanded
	}
	alt := make([]parser.Node, len(n.Alternative))
	for i, stmt := range n.Alternative {
		expanded, err := me.ExpandNode(stmt)
		if err != nil {
			return nil, err
		}
		alt[i] = expanded
	}
	return &parser.IfStmt{
		Condition:   cond,
		Consequence: cons,
		Alternative: alt,
	}, nil
}

func (me *MacroExpander) expandVarDecl(n *parser.VarDeclStmt) (parser.Node, error) {
	val, err := me.ExpandNode(n.Value)
	if err != nil {
		return nil, err
	}
	return &parser.VarDeclStmt{Name: n.Name, Value: val, Type: n.Type}, nil
}

func (me *MacroExpander) expandBinaryExpr(n *parser.BinaryExpr) (parser.Node, error) {
	left, err := me.ExpandNode(n.Left)
	if err != nil {
		return nil, err
	}
	right, err := me.ExpandNode(n.Right)
	if err != nil {
		return nil, err
	}
	return &parser.BinaryExpr{Left: left, Operator: n.Operator, Right: right}, nil
}

func (me *MacroExpander) expandCallExpr(n *parser.CallExpr) (parser.Node, error) {
	args := make([]parser.Node, len(n.Args))
	for i, arg := range n.Args {
		expanded, err := me.ExpandNode(arg)
		if err != nil {
			return nil, err
		}
		args[i] = expanded
	}
	return &parser.CallExpr{Function: n.Function, Args: args, IsCFunc: n.IsCFunc}, nil
}

func (me *MacroExpander) expandFuncDecl(n *parser.FuncDecl) (parser.Node, error) {
	body := make([]parser.Node, len(n.Body))
	for i, stmt := range n.Body {
		expanded, err := me.ExpandNode(stmt)
		if err != nil {
			return nil, err
		}
		body[i] = expanded
	}
	return &parser.FuncDecl{
		Name:          n.Name,
		Params:        n.Params,
		Body:          body,
		GenericParams: n.GenericParams,
	}, nil
}

func (me *MacroExpander) expandStmtList(n *parser.StmtList) (parser.Node, error) {
	stmts := make([]parser.Node, 0, len(n.Statements))
	for _, stmt := range n.Statements {
		expanded, err := me.ExpandNode(stmt)
		if err != nil {
			return nil, err
		}
		if nodes, ok := expanded.([]parser.Node); ok {
			stmts = append(stmts, nodes...)
		} else {
			stmts = append(stmts, expanded)
		}
	}
	return &parser.StmtList{Statements: stmts}, nil
}

// expandDeriveExpr handles @derive(Trait) on a struct — generates impl blocks
func (me *MacroExpander) expandDeriveExpr(n *parser.DeriveExpr) (parser.Node, error) {
	traits := []string{n.Trait}
	result := &parser.StmtList{Statements: []parser.Node{}}
	for _, trait := range traits {
		switch strings.ToLower(trait) {
		case "equal":
			result.Statements = append(result.Statements, me.generateEqualImpl())
		case "clone":
			result.Statements = append(result.Statements, me.generateCloneImpl())
		case "string":
			result.Statements = append(result.Statements, me.generateStringImpl())
		case "hash":
			result.Statements = append(result.Statements, me.generateHashImpl())
		case "ord":
			result.Statements = append(result.Statements, me.generateOrdImpl())
		case "debug":
			result.Statements = append(result.Statements, me.generateDebugImpl())
		case "jsonserializable":
			result.Statements = append(result.Statements, me.generateJSONImpl())
		case "default":
			result.Statements = append(result.Statements, me.generateDefaultImpl())
		case "copy":
			result.Statements = append(result.Statements, me.generateCopyImpl())
		case "printable":
			result.Statements = append(result.Statements, me.generatePrintableImpl())
		default:
			return nil, fmt.Errorf("@derive: unknown trait '%s'", trait)
		}
	}
	return result, nil
}

// handleUnroll processes @unroll(count) on the next for-loop
func (me *MacroExpander) handleUnroll(m *parser.MacroExpandExpr) (parser.Node, error) {
	if len(m.Args) < 1 {
		return nil, fmt.Errorf("@unroll: requires count argument")
	}
	countNode, ok := EvalConstExpr(m.Args[0])
	if !ok {
		return nil, fmt.Errorf("@unroll: count must be a compile-time constant")
	}
	countLit, ok := countNode.(*parser.IntLiteral)
	if !ok {
		return nil, fmt.Errorf("@unroll: count must be an integer")
	}
	count, err := strconv.Atoi(countLit.Value)
	if err != nil {
		return nil, fmt.Errorf("@unroll: invalid count: %s", countLit.Value)
	}
	_ = count
	return m, nil
}

// handleInline marks an expression for inline expansion by codegen
func (me *MacroExpander) handleInlineOriginal(m *parser.MacroExpandExpr) (parser.Node, error) {
	if len(m.Args) != 1 {
		return nil, fmt.Errorf("@inline: expected 1 argument (function call)")
	}
	return m.Args[0], nil
}
