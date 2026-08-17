package sema

import (
	"fmt"
	"strings"

	"karkain/pkg/parser"
)

// ============================================================
// Phase 38: Coroutine/Async Runtime Semantic Analysis
// ============================================================

// CoroutineDef represents a validated coroutine definition
type CoroutineDef struct {
	Name       string
	Params     []CoroutineParamDef
	RetType    string
	IsAsync    bool
	BodySize   int
	IsGenerator bool
	YieldType  string
}

// CoroutineParamDef is a validated coroutine parameter
type CoroutineParamDef struct {
	Name string
	Type string
}

// ChannelDef represents a validated channel declaration
type ChannelDef struct {
	Name        string
	ElementType string
	BufferSize  int
	Direction   string
	IsInfer     bool
}

// SelectCaseDef is a validated select case
type SelectCaseDef struct {
	ChannelName string
	Direction   string
	VarName     string
	BodySize    int
}

// CoroutineChecker performs semantic analysis on coroutine constructs
type CoroutineChecker struct {
	Coroutines   map[string]*CoroutineDef
	Channels     map[string]*ChannelDef
	Errors       []string
	Warnings     []string
	inAsync      bool
	inCoroutine  bool
	inSelect     bool
	currentCo    string
	nestingLevel int
}

// NewCoroutineChecker creates a new coroutine semantic checker
func NewCoroutineChecker() *CoroutineChecker {
	return &CoroutineChecker{
		Coroutines: make(map[string]*CoroutineDef),
		Channels:   make(map[string]*ChannelDef),
	}
}

// ============================================================
// Program Processing
// ============================================================

func (cc *CoroutineChecker) ProcessProgram(prog *parser.Program) error {
	for _, stmt := range prog.Statements {
		cc.processNode(stmt)
	}
	if len(cc.Errors) > 0 {
		return fmt.Errorf("coroutine: %d error(s) found", len(cc.Errors))
	}
	return nil
}

func (cc *CoroutineChecker) processNode(node parser.Node) {
	switch n := node.(type) {
	case *parser.CoroutineDecl:
		cc.ProcessCoroutineDecl(n)
	case *parser.AsyncExpr:
		cc.ProcessAsyncExpr(n)
	case *parser.AwaitExpr:
		cc.ProcessAwaitExpr(n)
	case *parser.YieldExpr:
		cc.ProcessYieldExpr(n)
	case *parser.ChDeclExpr:
		cc.ProcessChDeclExpr(n)
	case *parser.ChSendExpr:
		cc.ProcessChSendExpr(n)
	case *parser.ChRecvExpr:
		cc.ProcessChRecvExpr(n)
	case *parser.SelectStmt:
		cc.ProcessSelectStmt(n)
	case *parser.GreenSpawnExpr:
		cc.ProcessGreenSpawnExpr(n)
	case *parser.AwaitAllExpr:
		cc.ProcessAwaitAllExpr(n)
	case *parser.FuncDecl:
		cc.processBody(n.Body)
	case *parser.ActorDeclStmt:
		cc.processBody(n.Body)
	}
}

func (cc *CoroutineChecker) processBody(body []parser.Node) {
	for _, stmt := range body {
		cc.processNode(stmt)
	}
}

// ============================================================
// Coroutine Declaration Validation
// ============================================================

func (cc *CoroutineChecker) ProcessCoroutineDecl(decl *parser.CoroutineDecl) error {
	if decl.Name == "" {
		cc.Errors = append(cc.Errors, "coroutine: missing name")
		return fmt.Errorf("coroutine: missing name")
	}

	if _, exists := cc.Coroutines[decl.Name]; exists {
		cc.Errors = append(cc.Errors, fmt.Sprintf("coroutine: duplicate name '%s'", decl.Name))
		return fmt.Errorf("coroutine: duplicate name '%s'", decl.Name)
	}

	params := make([]CoroutineParamDef, len(decl.Params))
	for i, p := range decl.Params {
		params[i] = CoroutineParamDef{Name: p.Name, Type: p.Type}
	}

	def := &CoroutineDef{
		Name:       decl.Name,
		Params:     params,
		RetType:    decl.RetType,
		IsAsync:    decl.IsAsync,
		BodySize:   len(decl.Body),
	}

	cc.Coroutines[decl.Name] = def

	// Process body to check for yield/await usage
	prevCoroutine := cc.inCoroutine
	prevCo := cc.currentCo
	cc.inCoroutine = true
	cc.currentCo = decl.Name
	cc.processBody(decl.Body)
	cc.inCoroutine = prevCoroutine
	cc.currentCo = prevCo

	return nil
}

// ============================================================
// Async/Await Validation
// ============================================================

func (cc *CoroutineChecker) ProcessAsyncExpr(expr *parser.AsyncExpr) error {
	cc.nestingLevel++
	prevAsync := cc.inAsync
	cc.inAsync = true
	cc.processBody(expr.Body)
	cc.inAsync = prevAsync
	cc.nestingLevel--
	return nil
}

func (cc *CoroutineChecker) ProcessAwaitExpr(expr *parser.AwaitExpr) error {
	if expr.Operand == nil {
		cc.Errors = append(cc.Errors, "await: missing operand")
		return fmt.Errorf("await: missing operand")
	}
	if !cc.inAsync && !cc.inCoroutine {
		cc.Warnings = append(cc.Warnings, "await: used outside async/coroutine context")
	}
	return nil
}

func (cc *CoroutineChecker) ProcessYieldExpr(expr *parser.YieldExpr) error {
	if !cc.inCoroutine {
		cc.Errors = append(cc.Errors, "yield: used outside coroutine")
		return fmt.Errorf("yield: used outside coroutine")
	}
	// Mark current coroutine as generator
	if co, ok := cc.Coroutines[cc.currentCo]; ok {
		co.IsGenerator = true
	}
	return nil
}

// ============================================================
// Channel Validation
// ============================================================

func (cc *CoroutineChecker) ProcessChDeclExpr(expr *parser.ChDeclExpr) error {
	if expr.ElementType == "" {
		cc.Errors = append(cc.Errors, "chan: missing element type")
		return fmt.Errorf("chan: missing element type")
	}
	return nil
}

func (cc *CoroutineChecker) ProcessChSendExpr(expr *parser.ChSendExpr) error {
	if expr.Channel == nil {
		cc.Errors = append(cc.Errors, "ch send: missing channel")
		return fmt.Errorf("ch send: missing channel")
	}
	if expr.Value == nil {
		cc.Errors = append(cc.Errors, "ch send: missing value")
		return fmt.Errorf("ch send: missing value")
	}
	return nil
}

func (cc *CoroutineChecker) ProcessChRecvExpr(expr *parser.ChRecvExpr) error {
	if expr.Channel == nil {
		cc.Errors = append(cc.Errors, "ch recv: missing channel")
		return fmt.Errorf("ch recv: missing channel")
	}
	return nil
}

// ============================================================
// Select Validation
// ============================================================

func (cc *CoroutineChecker) ProcessSelectStmt(stmt *parser.SelectStmt) error {
	if len(stmt.Cases) == 0 && len(stmt.Default) == 0 {
		cc.Errors = append(cc.Errors, "select: empty select statement")
		return fmt.Errorf("select: empty select statement")
	}

	prevSelect := cc.inSelect
	cc.inSelect = true
	for _, sc := range stmt.Cases {
		if sc.Channel == nil {
			cc.Errors = append(cc.Errors, "select: case missing channel")
		}
		cc.processBody(sc.Body)
	}
	cc.processBody(stmt.Default)
	cc.inSelect = prevSelect

	return nil
}

// ============================================================
// Green Spawn & Await All Validation
// ============================================================

func (cc *CoroutineChecker) ProcessGreenSpawnExpr(expr *parser.GreenSpawnExpr) error {
	if expr.Function == "" {
		cc.Errors = append(cc.Errors, "gospawn: missing function name")
		return fmt.Errorf("gospawn: missing function name")
	}
	return nil
}

func (cc *CoroutineChecker) ProcessAwaitAllExpr(expr *parser.AwaitAllExpr) error {
	if len(expr.Futures) == 0 {
		cc.Errors = append(cc.Errors, "await_all: no futures provided")
		return fmt.Errorf("await_all: no futures provided")
	}
	return nil
}

// ============================================================
// Query API
// ============================================================

func (cc *CoroutineChecker) HasCoroutine(name string) bool {
	_, ok := cc.Coroutines[name]
	return ok
}

func (cc *CoroutineChecker) GetCoroutine(name string) *CoroutineDef {
	return cc.Coroutines[name]
}

func (cc *CoroutineChecker) GetCoroutineNames() []string {
	names := make([]string, 0, len(cc.Coroutines))
	for name := range cc.Coroutines {
		names = append(names, name)
	}
	return names
}

func (cc *CoroutineChecker) IsAsyncContext() bool {
	return cc.inAsync
}

func (cc *CoroutineChecker) IsCoroutineContext() bool {
	return cc.inCoroutine
}

func (cc *CoroutineChecker) IsSelectContext() bool {
	return cc.inSelect
}

func (cc *CoroutineChecker) ValidateAsyncUsage(name string) error {
	co, ok := cc.Coroutines[name]
	if !ok {
		return fmt.Errorf("coroutine '%s' not found", name)
	}
	if !co.IsAsync {
		return fmt.Errorf("coroutine '%s' is not async", name)
	}
	return nil
}

func (cc *CoroutineChecker) ValidateYieldInCoroutine(name string) error {
	co, ok := cc.Coroutines[name]
	if !ok {
		return fmt.Errorf("coroutine '%s' not found", name)
	}
	if !co.IsGenerator {
		return fmt.Errorf("coroutine '%s' does not yield values", name)
	}
	return nil
}

func (cc *CoroutineChecker) RegisterChannel(name, elementType string, bufferSize int) {
	cc.Channels[name] = &ChannelDef{
		Name:        name,
		ElementType: elementType,
		BufferSize:  bufferSize,
		Direction:   "bidirectional",
	}
}

func (cc *CoroutineChecker) GetChannel(name string) *ChannelDef {
	return cc.Channels[name]
}

func (cc *CoroutineChecker) HasChannel(name string) bool {
	_, ok := cc.Channels[name]
	return ok
}

// GenerateBridge produces Go runtime bridge code for a coroutine
func (cc *CoroutineChecker) GenerateCoroutineBridge(def *CoroutineDef) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("func spawn_%s() *runtime.Coroutine {\n", def.Name))
	b.WriteString(fmt.Sprintf("\treturn runtime.NewCoroutine(%q, func(co *runtime.Coroutine) (runtime.CoroutineState, error) {\n", def.Name))
	b.WriteString("\t\tswitch co.GetCurrentState() {\n")
	b.WriteString("\t\tcase 0:\n")
	b.WriteString("\t\t\t// initial state\n")
	b.WriteString("\t\t\treturn 1, nil\n")
	b.WriteString("\t\tdefault:\n")
	b.WriteString("\t\t\treturn 0, nil // completed\n")
	b.WriteString("\t\t}\n")
	b.WriteString("\t})\n}\n")
	return b.String()
}
