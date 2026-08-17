package sema

import (
	"testing"

	"karkain/pkg/parser"
)

func TestCoroutine_ProcessCoroutineDecl(t *testing.T) {
	checker := NewCoroutineChecker()
	decl := &parser.CoroutineDecl{
		Name:    "fetchData",
		IsAsync: false,
		Params: []parser.Parameter{
			{Name: "url", Type: "string"},
		},
		RetType: "string",
		Body:    []parser.Node{},
	}

	err := checker.ProcessCoroutineDecl(decl)
	if err != nil {
		t.Fatalf("process coroutine decl failed: %v", err)
	}

	co := checker.GetCoroutine("fetchData")
	if co == nil {
		t.Fatal("expected coroutine 'fetchData'")
	}
	if co.RetType != "string" {
		t.Errorf("expected rettype 'string', got '%s'", co.RetType)
	}
	if len(co.Params) != 1 {
		t.Errorf("expected 1 param, got %d", len(co.Params))
	}
}

func TestCoroutine_EmptyName(t *testing.T) {
	checker := NewCoroutineChecker()
	decl := &parser.CoroutineDecl{
		Name: "",
	}

	err := checker.ProcessCoroutineDecl(decl)
	if err == nil {
		t.Fatal("empty name should fail")
	}
}

func TestCoroutine_DuplicateName(t *testing.T) {
	checker := NewCoroutineChecker()
	decl := &parser.CoroutineDecl{Name: "dup"}

	err := checker.ProcessCoroutineDecl(decl)
	if err != nil {
		t.Fatalf("first declaration should succeed: %v", err)
	}

	err = checker.ProcessCoroutineDecl(decl)
	if err == nil {
		t.Fatal("duplicate name should fail")
	}
}

func TestCoroutine_ProcessProgram(t *testing.T) {
	checker := NewCoroutineChecker()
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.CoroutineDecl{Name: "co1", Body: []parser.Node{}},
			&parser.CoroutineDecl{Name: "co2", Body: []parser.Node{}},
			&parser.FuncDecl{Name: "main", Body: []parser.Node{}},
		},
	}

	err := checker.ProcessProgram(prog)
	if err != nil {
		t.Fatalf("process program failed: %v", err)
	}

	if len(checker.Coroutines) != 2 {
		t.Errorf("expected 2 coroutines, got %d", len(checker.Coroutines))
	}
}

func TestCoroutine_AsyncExpr(t *testing.T) {
	checker := NewCoroutineChecker()

	if checker.IsAsyncContext() {
		t.Error("should not start in async context")
	}

	err := checker.ProcessAsyncExpr(&parser.AsyncExpr{
		Body: []parser.Node{},
	})
	if err != nil {
		t.Fatalf("async expr should succeed: %v", err)
	}

	if checker.IsAsyncContext() {
		t.Error("should not remain in async context after processing")
	}
}

func TestCoroutine_AwaitExpr(t *testing.T) {
	checker := NewCoroutineChecker()

	err := checker.ProcessAwaitExpr(&parser.AwaitExpr{
		Operand: &parser.Identifier{Name: "future1"},
	})
	if err != nil {
		t.Fatalf("await should succeed: %v", err)
	}

	if len(checker.Warnings) == 0 {
		t.Error("expected warning for await outside async context")
	}
}

func TestCoroutine_AwaitExprNilOperand(t *testing.T) {
	checker := NewCoroutineChecker()

	err := checker.ProcessAwaitExpr(&parser.AwaitExpr{Operand: nil})
	if err == nil {
		t.Fatal("nil operand should fail")
	}
}

func TestCoroutine_YieldOutsideCoroutine(t *testing.T) {
	checker := NewCoroutineChecker()

	err := checker.ProcessYieldExpr(&parser.YieldExpr{
		Value: &parser.IntLiteral{Value: "42"},
	})
	if err == nil {
		t.Fatal("yield outside coroutine should fail")
	}
}

func TestCoroutine_YieldInsideCoroutine(t *testing.T) {
	checker := NewCoroutineChecker()

	decl := &parser.CoroutineDecl{
		Name: "gen",
		Body: []parser.Node{
			&parser.YieldExpr{Value: &parser.IntLiteral{Value: "1"}},
			&parser.YieldExpr{Value: &parser.IntLiteral{Value: "2"}},
		},
	}
	checker.ProcessCoroutineDecl(decl)

	co := checker.GetCoroutine("gen")
	if !co.IsGenerator {
		t.Error("expected generator to be true")
	}
}

func TestCoroutine_YieldOutsideContext(t *testing.T) {
	checker := NewCoroutineChecker()

	// Manually set inCoroutine to test the error path
	checker.inCoroutine = false
	err := checker.ProcessYieldExpr(&parser.YieldExpr{Value: &parser.IntLiteral{Value: "1"}})
	if err == nil {
		t.Error("yield outside coroutine should error")
	}
}

func TestCoroutine_ChDeclExpr(t *testing.T) {
	checker := NewCoroutineChecker()

	err := checker.ProcessChDeclExpr(&parser.ChDeclExpr{
		ElementType: "int",
		BufferSize:  &parser.IntLiteral{Value: "10"},
	})
	if err != nil {
		t.Fatalf("channel decl should succeed: %v", err)
	}

	err = checker.ProcessChDeclExpr(&parser.ChDeclExpr{
		ElementType: "",
	})
	if err == nil {
		t.Fatal("empty element type should fail")
	}
}

func TestCoroutine_ChSendExpr(t *testing.T) {
	checker := NewCoroutineChecker()

	err := checker.ProcessChSendExpr(&parser.ChSendExpr{
		Channel: &parser.Identifier{Name: "ch"},
		Value:   &parser.IntLiteral{Value: "42"},
	})
	if err != nil {
		t.Fatalf("ch send should succeed: %v", err)
	}

	err = checker.ProcessChSendExpr(&parser.ChSendExpr{Channel: nil})
	if err == nil {
		t.Fatal("nil channel should fail")
	}

	err = checker.ProcessChSendExpr(&parser.ChSendExpr{
		Channel: &parser.Identifier{Name: "ch"},
		Value:   nil,
	})
	if err == nil {
		t.Fatal("nil value should fail")
	}
}

func TestCoroutine_ChRecvExpr(t *testing.T) {
	checker := NewCoroutineChecker()

	err := checker.ProcessChRecvExpr(&parser.ChRecvExpr{
		Channel: &parser.Identifier{Name: "ch"},
	})
	if err != nil {
		t.Fatalf("ch recv should succeed: %v", err)
	}

	err = checker.ProcessChRecvExpr(&parser.ChRecvExpr{Channel: nil})
	if err == nil {
		t.Fatal("nil channel should fail")
	}
}

func TestCoroutine_SelectStmt(t *testing.T) {
	checker := NewCoroutineChecker()

	err := checker.ProcessSelectStmt(&parser.SelectStmt{
		Cases: []parser.SelectCase{
			{
				Channel: &parser.Identifier{Name: "ch1"},
				Dir:     "recv",
				VarName: "v",
				Body:    []parser.Node{},
			},
		},
	})
	if err != nil {
		t.Fatalf("select should succeed: %v", err)
	}
}

func TestCoroutine_SelectStmtEmpty(t *testing.T) {
	checker := NewCoroutineChecker()

	err := checker.ProcessSelectStmt(&parser.SelectStmt{})
	if err == nil {
		t.Fatal("empty select should fail")
	}
}

func TestCoroutine_SelectStmtNilChannel(t *testing.T) {
	checker := NewCoroutineChecker()

	err := checker.ProcessSelectStmt(&parser.SelectStmt{
		Cases: []parser.SelectCase{
			{Channel: nil, Body: []parser.Node{}},
		},
	})
	if err != nil {
		// It should add an error but not return error from ProcessSelectStmt
		// since it accumulates errors
	}
	if len(checker.Errors) == 0 {
		t.Error("expected error for nil channel in select case")
	}
}

func TestCoroutine_GreenSpawnExpr(t *testing.T) {
	checker := NewCoroutineChecker()

	err := checker.ProcessGreenSpawnExpr(&parser.GreenSpawnExpr{
		Function: "worker",
		Args:     []parser.Node{&parser.IntLiteral{Value: "1"}},
	})
	if err != nil {
		t.Fatalf("gospawn should succeed: %v", err)
	}

	err = checker.ProcessGreenSpawnExpr(&parser.GreenSpawnExpr{Function: ""})
	if err == nil {
		t.Fatal("empty function should fail")
	}
}

func TestCoroutine_AwaitAllExpr(t *testing.T) {
	checker := NewCoroutineChecker()

	err := checker.ProcessAwaitAllExpr(&parser.AwaitAllExpr{
		Futures: []parser.Node{
			&parser.Identifier{Name: "f1"},
			&parser.Identifier{Name: "f2"},
		},
	})
	if err != nil {
		t.Fatalf("await_all should succeed: %v", err)
	}

	err = checker.ProcessAwaitAllExpr(&parser.AwaitAllExpr{Futures: []parser.Node{}})
	if err == nil {
		t.Fatal("empty futures should fail")
	}
}

func TestCoroutine_HasCoroutine(t *testing.T) {
	checker := NewCoroutineChecker()
	checker.Coroutines["exists"] = &CoroutineDef{Name: "exists"}

	if !checker.HasCoroutine("exists") {
		t.Error("should have 'exists'")
	}
	if checker.HasCoroutine("missing") {
		t.Error("should not have 'missing'")
	}
}

func TestCoroutine_GetCoroutineNames(t *testing.T) {
	checker := NewCoroutineChecker()
	checker.Coroutines["a"] = &CoroutineDef{Name: "a"}
	checker.Coroutines["b"] = &CoroutineDef{Name: "b"}

	names := checker.GetCoroutineNames()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

func TestCoroutine_IsContexts(t *testing.T) {
	checker := NewCoroutineChecker()

	if checker.IsAsyncContext() {
		t.Error("should not start async")
	}
	if checker.IsCoroutineContext() {
		t.Error("should not start coroutine")
	}
	if checker.IsSelectContext() {
		t.Error("should not start select")
	}
}

func TestCoroutine_ValidateAsyncUsage(t *testing.T) {
	checker := NewCoroutineChecker()
	checker.Coroutines["async_co"] = &CoroutineDef{Name: "async_co", IsAsync: true}
	checker.Coroutines["sync_co"] = &CoroutineDef{Name: "sync_co", IsAsync: false}

	err := checker.ValidateAsyncUsage("async_co")
	if err != nil {
		t.Errorf("async usage should pass: %v", err)
	}

	err = checker.ValidateAsyncUsage("sync_co")
	if err == nil {
		t.Error("sync usage should fail")
	}

	err = checker.ValidateAsyncUsage("missing")
	if err == nil {
		t.Error("missing should fail")
	}
}

func TestCoroutine_ValidateYieldInCoroutine(t *testing.T) {
	checker := NewCoroutineChecker()
	checker.Coroutines["gen"] = &CoroutineDef{Name: "gen", IsGenerator: true}
	checker.Coroutines["not_gen"] = &CoroutineDef{Name: "not_gen", IsGenerator: false}

	err := checker.ValidateYieldInCoroutine("gen")
	if err != nil {
		t.Errorf("generator yield should pass: %v", err)
	}

	err = checker.ValidateYieldInCoroutine("not_gen")
	if err == nil {
		t.Error("non-generator yield should fail")
	}

	err = checker.ValidateYieldInCoroutine("missing")
	if err == nil {
		t.Error("missing should fail")
	}
}

func TestCoroutine_ChannelRegistration(t *testing.T) {
	checker := NewCoroutineChecker()

	checker.RegisterChannel("ch1", "int", 10)
	if !checker.HasChannel("ch1") {
		t.Error("should have ch1")
	}

	ch := checker.GetChannel("ch1")
	if ch == nil {
		t.Fatal("expected ch1")
	}
	if ch.ElementType != "int" {
		t.Errorf("expected 'int', got '%s'", ch.ElementType)
	}
	if ch.BufferSize != 10 {
		t.Errorf("expected buffer 10, got %d", ch.BufferSize)
	}

	if checker.HasChannel("missing") {
		t.Error("should not have missing")
	}

	if checker.GetChannel("missing") != nil {
		t.Error("expected nil for missing channel")
	}
}

func TestCoroutine_GenerateBridge(t *testing.T) {
	checker := NewCoroutineChecker()
	def := &CoroutineDef{
		Name:    "MyCo",
		RetType: "int",
	}

	bridge := checker.GenerateCoroutineBridge(def)
	if bridge == "" {
		t.Error("bridge should not be empty")
	}
	if !containsStr(bridge, "MyCo") {
		t.Error("bridge should contain coroutine name")
	}
}

func TestCoroutine_ProcessNodeDispatch(t *testing.T) {
	checker := NewCoroutineChecker()

	// Test that processNode dispatches correctly
	checker.processNode(&parser.CoroutineDecl{Name: "co1"})
	checker.processNode(&parser.AsyncExpr{Body: []parser.Node{}})
	checker.processNode(&parser.GreenSpawnExpr{Function: "f"})
	checker.processNode(&parser.AwaitAllExpr{Futures: []parser.Node{&parser.IntLiteral{Value: "1"}}})

	if !checker.HasCoroutine("co1") {
		t.Error("expected co1 to be registered")
	}
}

func TestCoroutine_NestedAsyncWarning(t *testing.T) {
	checker := NewCoroutineChecker()

	// Process an await inside async — should NOT warn
	checker.inAsync = true
	checker.ProcessAwaitExpr(&parser.AwaitExpr{
		Operand: &parser.Identifier{Name: "future1"},
	})
	checker.inAsync = false

	for _, w := range checker.Warnings {
		if w == "await: used outside async/coroutine context" {
			t.Error("should not warn inside async context")
		}
	}
}
