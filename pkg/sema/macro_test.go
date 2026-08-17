package sema

import (
	"testing"

	"karkain/pkg/parser"
)

func TestMacro_BasicRegistration(t *testing.T) {
	me := NewMacroExpander()
	me.RegisterMacro("swap", []string{"a", "b"}, []parser.Node{
		&parser.VarDeclStmt{Name: "temp", Value: &parser.Identifier{Name: "a"}},
		&parser.VarDeclStmt{Name: "a_val", Value: &parser.Identifier{Name: "b"}},
		&parser.VarDeclStmt{Name: "b_val", Value: &parser.Identifier{Name: "temp"}},
	}, false)

	m, ok := me.GetMacro("swap")
	if !ok {
		t.Fatal("macro 'swap' not found")
	}
	if len(m.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(m.Params))
	}
}

func TestMacro_UndefinedMacroError(t *testing.T) {
	me := NewMacroExpander()
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.MacroExpandExpr{
				MacroName: "nonexistent",
				Args:      []parser.Node{&parser.IntLiteral{Value: "1"}},
			},
		},
	}
	_, err := me.ExpandProgram(prog)
	if err == nil {
		t.Fatal("expected error for undefined macro")
	}
}

func TestMacro_ArgCountMismatch(t *testing.T) {
	me := NewMacroExpander()
	me.RegisterMacro("my_macro", []string{"x", "y"}, []parser.Node{
		&parser.IntLiteral{Value: "42"},
	}, false)

	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.MacroExpandExpr{
				MacroName: "my_macro",
				Args:      []parser.Node{&parser.IntLiteral{Value: "1"}},
			},
		},
	}
	_, err := me.ExpandProgram(prog)
	if err == nil {
		t.Fatal("expected error for arg count mismatch")
	}
}

func TestMacro_UserDefinedExpansion(t *testing.T) {
	me := NewMacroExpander()
	me.RegisterMacro("double_it", []string{"x"}, []parser.Node{
		&parser.BinaryExpr{
			Left:     &parser.Identifier{Name: "x"},
			Operator: "*",
			Right:    &parser.IntLiteral{Value: "2"},
		},
	}, false)

	expr, err := me.expandUserDefined(
		me.Macros["double_it"],
		&parser.MacroExpandExpr{
			MacroName: "double_it",
			Args:      []parser.Node{&parser.IntLiteral{Value: "5"}},
		},
	)
	if err != nil {
		t.Fatalf("expansion failed: %v", err)
	}
	be, ok := expr.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", expr)
	}
	if be.Operator != "*" {
		t.Errorf("expected '*', got '%s'", be.Operator)
	}
}

func TestMacro_ScopePushPop(t *testing.T) {
	me := NewMacroExpander()
	me.PushScope("test")
	me.Bind("x", &parser.IntLiteral{Value: "10"})

	val, ok := me.Lookup("x")
	if !ok {
		t.Fatal("expected to find 'x' in scope")
	}
	il, ok := val.(*parser.IntLiteral)
	if !ok || il.Value != "10" {
		t.Errorf("expected IntLiteral(10), got %v", val)
	}

	me.PopScope()
	_, ok = me.Lookup("x")
	if ok {
		t.Fatal("expected 'x' to be out of scope after pop")
	}
}

func TestMacro_HygieneRename(t *testing.T) {
	me := NewMacroExpander()
	me.PushScope("hygiene_test")
	name1 := me.HygieneRename("temp")
	me.CurrentDepth++
	name2 := me.HygieneRename("temp")

	if name1 == name2 {
		t.Errorf("hygiene names should differ: %s == %s", name1, name2)
	}
}

func TestMacro_MaxExpansionDepth(t *testing.T) {
	me := NewMacroExpander()
	me.MaxDepth = 0

	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.MacroExpandExpr{
				MacroName: "anything",
				Args:      []parser.Node{},
			},
		},
	}
	_, err := me.ExpandProgram(prog)
	if err == nil {
		t.Fatal("expected depth exceeded error")
	}
}

func TestMacro_ExpansionTracking(t *testing.T) {
	me := NewMacroExpander()
	me.RegisterMacro("foo", []string{}, []parser.Node{
		&parser.IntLiteral{Value: "99"},
	}, false)

	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.MacroExpandExpr{
				MacroName: "foo",
				Args:      []parser.Node{},
			},
			&parser.MacroExpandExpr{
				MacroName: "foo",
				Args:      []parser.Node{},
			},
		},
	}
	_, err := me.ExpandProgram(prog)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if me.Macros["foo"].ExpansionCount != 2 {
		t.Errorf("expected 2 expansions, got %d", me.Macros["foo"].ExpansionCount)
	}
}

func TestMacro_MaxExpansionsLimit(t *testing.T) {
	me := NewMacroExpander()
	me.RegisterMacro("limited", []string{}, []parser.Node{
		&parser.IntLiteral{Value: "1"},
	}, false)
	me.Macros["limited"].MaxExpansions = 1
	me.Macros["limited"].ExpansionCount = 1

	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.MacroExpandExpr{
				MacroName: "limited",
				Args:      []parser.Node{},
			},
		},
	}
	_, err := me.ExpandProgram(prog)
	if err == nil {
		t.Fatal("expected max expansion error")
	}
}

func TestMacro_HygienicExpansion(t *testing.T) {
	me := NewMacroExpander()
	me.RegisterMacro("hyg", []string{"val"}, []parser.Node{
		&parser.VarDeclStmt{Name: "x", Value: &parser.Identifier{Name: "val"}},
	}, true)

	macro := me.Macros["hyg"]
	expr, err := me.expandUserDefined(macro, &parser.MacroExpandExpr{
		MacroName: "hyg",
		Args:      []parser.Node{&parser.IntLiteral{Value: "7"}},
	})
	if err != nil {
		t.Fatalf("hygienic expansion failed: %v", err)
	}

	vd, ok := expr.(*parser.VarDeclStmt)
	if !ok {
		t.Fatalf("expected VarDeclStmt, got %T", expr)
	}
	if vd.Name == "x" {
		t.Error("hygienic expansion should rename 'x'")
	}
}

func TestMacro_ProgramExpansion(t *testing.T) {
	me := NewMacroExpander()
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.MacroDeclStmt{
				Name:       "get_zero",
				Params:     []string{},
				Body:       []parser.Node{&parser.IntLiteral{Value: "0"}},
				IsHygienic: false,
			},
			&parser.VarDeclStmt{
				Name:  "z",
				Value: &parser.MacroExpandExpr{MacroName: "get_zero", Args: []parser.Node{}},
				Type:  "int",
			},
		},
	}

	result, err := me.ExpandProgram(prog)
	if err != nil {
		t.Fatalf("program expansion failed: %v", err)
	}

	// MacroDeclStmt should be removed
	if len(result.Statements) != 1 {
		t.Errorf("expected 1 statement (macro decl removed), got %d", len(result.Statements))
	}

	vd, ok := result.Statements[0].(*parser.VarDeclStmt)
	if !ok {
		t.Fatalf("expected VarDeclStmt, got %T", result.Statements[0])
	}
	if vd.Name != "z" {
		t.Errorf("expected var 'z', got '%s'", vd.Name)
	}
}

// ============================================================
// Const Eval Tests
// ============================================================

func TestMacro_ConstEvalInt(t *testing.T) {
	expr := &parser.BinaryExpr{
		Left:     &parser.IntLiteral{Value: "3"},
		Operator: "+",
		Right:    &parser.IntLiteral{Value: "4"},
	}
	result, ok := EvalConstExpr(expr)
	if !ok {
		t.Fatal("eval failed")
	}
	il, ok := result.(*parser.IntLiteral)
	if !ok || il.Value != "7" {
		t.Errorf("expected 7, got %v", result)
	}
}

func TestMacro_ConstEvalMul(t *testing.T) {
	expr := &parser.BinaryExpr{
		Left:     &parser.IntLiteral{Value: "6"},
		Operator: "*",
		Right:    &parser.IntLiteral{Value: "7"},
	}
	result, ok := EvalConstExpr(expr)
	if !ok {
		t.Fatal("eval failed")
	}
	il, ok := result.(*parser.IntLiteral)
	if !ok || il.Value != "42" {
		t.Errorf("expected 42, got %v", result)
	}
}

func TestMacro_ConstEvalDiv(t *testing.T) {
	expr := &parser.BinaryExpr{
		Left:     &parser.IntLiteral{Value: "20"},
		Operator: "/",
		Right:    &parser.IntLiteral{Value: "4"},
	}
	result, ok := EvalConstExpr(expr)
	if !ok {
		t.Fatal("eval failed")
	}
	il, ok := result.(*parser.IntLiteral)
	if !ok || il.Value != "5" {
		t.Errorf("expected 5, got %v", result)
	}
}

func TestMacro_ConstEvalMod(t *testing.T) {
	expr := &parser.BinaryExpr{
		Left:     &parser.IntLiteral{Value: "17"},
		Operator: "%",
		Right:    &parser.IntLiteral{Value: "5"},
	}
	result, ok := EvalConstExpr(expr)
	if !ok {
		t.Fatal("eval failed")
	}
	il, ok := result.(*parser.IntLiteral)
	if !ok || il.Value != "2" {
		t.Errorf("expected 2, got %v", result)
	}
}

func TestMacro_ConstEvalCompare(t *testing.T) {
	tests := []struct {
		op   string
		want bool
	}{
		{"==", false},
		{"!=", true},
		{"<", true},
		{">", false},
		{"<=", true},
		{">=", false},
	}
	for _, tt := range tests {
		expr := &parser.BinaryExpr{
			Left:     &parser.IntLiteral{Value: "3"},
			Operator: tt.op,
			Right:    &parser.IntLiteral{Value: "5"},
		}
		result, ok := EvalConstExpr(expr)
		if !ok {
			t.Errorf("%s: eval failed", tt.op)
			continue
		}
		bl, ok := result.(*parser.BoolLiteral)
		if !ok {
			t.Errorf("%s: expected BoolLiteral, got %T", tt.op, result)
			continue
		}
		if bl.Value != tt.want {
			t.Errorf("%s: expected %v, got %v", tt.op, tt.want, bl.Value)
		}
	}
}

func TestMacro_ConstEvalBoolLogic(t *testing.T) {
	expr := &parser.BinaryExpr{
		Left:     &parser.BoolLiteral{Value: true},
		Operator: "and",
		Right:    &parser.BoolLiteral{Value: false},
	}
	result, ok := EvalConstExpr(expr)
	if !ok {
		t.Fatal("eval failed")
	}
	bl, ok := result.(*parser.BoolLiteral)
	if !ok || bl.Value != false {
		t.Errorf("expected false, got %v", result)
	}
}

func TestMacro_ConstEvalStringCompare(t *testing.T) {
	expr := &parser.BinaryExpr{
		Left:     &parser.StringLiteral{Value: "hello"},
		Operator: "==",
		Right:    &parser.StringLiteral{Value: "hello"},
	}
	result, ok := EvalConstExpr(expr)
	if !ok {
		t.Fatal("eval failed")
	}
	bl, ok := result.(*parser.BoolLiteral)
	if !ok || bl.Value != true {
		t.Errorf("expected true, got %v", result)
	}
}

func TestMacro_ConstEvalUnary(t *testing.T) {
	expr := &parser.UnaryExpr{
		Operator: "-",
		Operand:  &parser.IntLiteral{Value: "42"},
	}
	result, ok := EvalConstExpr(expr)
	if !ok {
		t.Fatal("eval failed")
	}
	il, ok := result.(*parser.IntLiteral)
	if !ok || il.Value != "-42" {
		t.Errorf("expected -42, got %v", result)
	}
}

func TestMacro_ConstEvalNotBool(t *testing.T) {
	expr := &parser.UnaryExpr{
		Operator: "!",
		Operand:  &parser.BoolLiteral{Value: true},
	}
	result, ok := EvalConstExpr(expr)
	if !ok {
		t.Fatal("eval failed")
	}
	bl, ok := result.(*parser.BoolLiteral)
	if !ok || bl.Value != false {
		t.Errorf("expected false, got %v", result)
	}
}

func TestMacro_ConstEvalDivByZero(t *testing.T) {
	expr := &parser.BinaryExpr{
		Left:     &parser.IntLiteral{Value: "10"},
		Operator: "/",
		Right:    &parser.IntLiteral{Value: "0"},
	}
	_, ok := EvalConstExpr(expr)
	if ok {
		t.Fatal("division by zero should fail")
	}
}

func TestMacro_ConstEvalModByZero(t *testing.T) {
	expr := &parser.BinaryExpr{
		Left:     &parser.IntLiteral{Value: "10"},
		Operator: "%",
		Right:    &parser.IntLiteral{Value: "0"},
	}
	_, ok := EvalConstExpr(expr)
	if ok {
		t.Fatal("mod by zero should fail")
	}
}

func TestMacro_ConstEvalFloat(t *testing.T) {
	expr := &parser.BinaryExpr{
		Left:     &parser.Float64Literal{Value: "3.14"},
		Operator: "+",
		Right:    &parser.Float64Literal{Value: "2.86"},
	}
	result, ok := EvalConstExpr(expr)
	if !ok {
		t.Fatal("eval failed")
	}
	fl, ok := result.(*parser.Float64Literal)
	if !ok || fl.Value != "6" {
		t.Errorf("expected 6, got %v", result)
	}
}

func TestMacro_ConstEvalNested(t *testing.T) {
	expr := &parser.BinaryExpr{
		Left: &parser.BinaryExpr{
			Left:     &parser.IntLiteral{Value: "2"},
			Operator: "+",
			Right:    &parser.IntLiteral{Value: "3"},
		},
		Operator: "*",
		Right:    &parser.IntLiteral{Value: "4"},
	}
	result, ok := EvalConstExpr(expr)
	if !ok {
		t.Fatal("eval failed")
	}
	il, ok := result.(*parser.IntLiteral)
	if !ok || il.Value != "20" {
		t.Errorf("expected 20, got %v", result)
	}
}

// ============================================================
// Template Substitution Tests
// ============================================================

func TestMacro_SubstituteTemplate(t *testing.T) {
	me := NewMacroExpander()
	me.PushScope("subst_test")

	template := &parser.BinaryExpr{
		Left:     &parser.Identifier{Name: "x"},
		Operator: "+",
		Right:    &parser.IntLiteral{Value: "1"},
	}

	vars := map[string]parser.Node{
		"x": &parser.IntLiteral{Value: "10"},
	}

	result := SubstituteTemplate(template, vars, me)
	be, ok := result.(*parser.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", result)
	}

	leftLit, ok := be.Left.(*parser.IntLiteral)
	if !ok || leftLit.Value != "10" {
		t.Errorf("expected substituted value 10, got %v", be.Left)
	}
}

func TestMacro_SubstituteCallExpr(t *testing.T) {
	me := NewMacroExpander()
	me.PushScope("subst_call")

	template := &parser.CallExpr{
		Function: "print",
		Args:     []parser.Node{&parser.Identifier{Name: "val"}},
	}

	vars := map[string]parser.Node{
		"val": &parser.StringLiteral{Value: "hello"},
	}

	result := SubstituteTemplate(template, vars, me)
	ce, ok := result.(*parser.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", result)
	}

	arg, ok := ce.Args[0].(*parser.StringLiteral)
	if !ok || arg.Value != "hello" {
		t.Errorf("expected 'hello', got %v", ce.Args[0])
	}
}

// ============================================================
// Loop Unrolling Tests
// ============================================================

func TestMacro_UnrollLoop(t *testing.T) {
	me := NewMacroExpander()
	me.PushScope("unroll")

	forStmt := &parser.ForStmt{
		Init:      &parser.VarDeclStmt{Name: "i", Value: &parser.IntLiteral{Value: "0"}, Type: "int"},
		Condition: &parser.BinaryExpr{Left: &parser.Identifier{Name: "i"}, Operator: "<", Right: &parser.IntLiteral{Value: "4"}},
		Post:      &parser.BinaryExpr{Left: &parser.Identifier{Name: "i"}, Operator: "+", Right: &parser.IntLiteral{Value: "1"}},
		Body: []parser.Node{
			&parser.PrintStmt{Value: &parser.Identifier{Name: "_i"}},
		},
	}

	result, err := UnrollLoop(forStmt, 4, me)
	if err != nil {
		t.Fatalf("unroll failed: %v", err)
	}
	if len(result) != 4 {
		t.Errorf("expected 4 unrolled statements, got %d", len(result))
	}

	for i, node := range result {
		ps, ok := node.(*parser.PrintStmt)
		if !ok {
			t.Errorf("expected PrintStmt at %d, got %T", i, node)
			continue
		}
		il, ok := ps.Value.(*parser.IntLiteral)
		if !ok || il.Value != string(rune('0'+i)) {
			t.Errorf("expected %d, got %v", i, ps.Value)
		}
	}
}

func TestMacro_UnrollLoopZeroCount(t *testing.T) {
	me := NewMacroExpander()
	me.PushScope("unroll_zero")

	forStmt := &parser.ForStmt{
		Init:      &parser.VarDeclStmt{Name: "i", Value: &parser.IntLiteral{Value: "0"}, Type: "int"},
		Condition: &parser.BinaryExpr{Left: &parser.Identifier{Name: "i"}, Operator: "<", Right: &parser.IntLiteral{Value: "10"}},
		Body: []parser.Node{
			&parser.PrintStmt{Value: &parser.Identifier{Name: "_i"}},
		},
	}

	result, err := UnrollLoop(forStmt, 0, me)
	if err != nil {
		t.Fatalf("unroll with 0 count should succeed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 unrolled statements, got %d", len(result))
	}
}

func TestMacro_UnrollLoopMaxExceeded(t *testing.T) {
	me := NewMacroExpander()
	me.PushScope("unroll_max")

	forStmt := &parser.ForStmt{
		Init:      &parser.VarDeclStmt{Name: "i", Value: &parser.IntLiteral{Value: "0"}, Type: "int"},
		Condition: &parser.BinaryExpr{Left: &parser.Identifier{Name: "i"}, Operator: "<", Right: &parser.IntLiteral{Value: "200"}},
		Body: []parser.Node{
			&parser.PrintStmt{Value: &parser.Identifier{Name: "_i"}},
		},
	}

	_, err := UnrollLoop(forStmt, 200, me)
	if err == nil {
		t.Fatal("expected error for count > 128")
	}
}

// ============================================================
// @derive Tests
// ============================================================

func TestMacro_DeriveEqual(t *testing.T) {
	me := NewMacroExpander()
	node := me.generateEqualImpl()
	meExpr, ok := node.(*parser.MacroExpandExpr)
	if !ok {
		t.Fatalf("expected MacroExpandExpr, got %T", node)
	}
	if meExpr.MacroName != "__generated_equal" {
		t.Errorf("expected __generated_equal, got %s", meExpr.MacroName)
	}
	if len(meExpr.Args) != 2 {
		t.Errorf("expected 2 args (self, other), got %d", len(meExpr.Args))
	}
}

func TestMacro_DeriveClone(t *testing.T) {
	me := NewMacroExpander()
	node := me.generateCloneImpl()
	meExpr, ok := node.(*parser.MacroExpandExpr)
	if !ok {
		t.Fatalf("expected MacroExpandExpr, got %T", node)
	}
	if meExpr.MacroName != "__generated_clone" {
		t.Errorf("expected __generated_clone, got %s", meExpr.MacroName)
	}
}

func TestMacro_DeriveUnknownTrait(t *testing.T) {
	me := NewMacroExpander()
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.DeriveExpr{Trait: "UnknownTrait"},
		},
	}
	_, err := me.ExpandProgram(prog)
	if err == nil {
		t.Fatal("expected error for unknown trait")
	}
}

func TestMacro_DeriveAllKnownTraits(t *testing.T) {
	me := NewMacroExpander()
	traits := []string{"equal", "clone", "string", "hash", "ord", "debug", "jsonserializable", "default", "copy", "printable"}
	for _, trait := range traits {
		node, err := me.expandDeriveExpr(&parser.DeriveExpr{Trait: trait})
		if err != nil {
			t.Errorf("@derive(%s) failed: %v", trait, err)
			continue
		}
		if node == nil {
			t.Errorf("@derive(%s) returned nil", trait)
		}
	}
}

// ============================================================
// @target_guard Tests
// ============================================================

func TestMacro_TargetGuard(t *testing.T) {
	me := NewMacroExpander()
	node, err := me.handleTargetGuard(&parser.MacroExpandExpr{
		MacroName: "target_guard",
		Args: []parser.Node{
			&parser.StringLiteral{Value: "gpu"},
			&parser.IntLiteral{Value: "42"},
		},
	})
	if err != nil {
		t.Fatalf("target_guard failed: %v", err)
	}
	il, ok := node.(*parser.IntLiteral)
	if !ok || il.Value != "42" {
		t.Errorf("expected IntLiteral(42), got %v", node)
	}
}

func TestMacro_TargetGuardNoBody(t *testing.T) {
	me := NewMacroExpander()
	node, err := me.handleTargetGuard(&parser.MacroExpandExpr{
		MacroName: "target_guard",
		Args: []parser.Node{
			&parser.StringLiteral{Value: "wasm"},
		},
	})
	if err != nil {
		t.Fatalf("target_guard failed: %v", err)
	}
	sl, ok := node.(*parser.StmtList)
	if !ok {
		t.Fatalf("expected StmtList, got %T", node)
	}
	if len(sl.Statements) != 0 {
		t.Errorf("expected empty statement list, got %d", len(sl.Statements))
	}
}

func TestMacro_TargetGuardRequiresString(t *testing.T) {
	me := NewMacroExpander()
	_, err := me.handleTargetGuard(&parser.MacroExpandExpr{
		MacroName: "target_guard",
		Args: []parser.Node{
			&parser.IntLiteral{Value: "42"},
		},
	})
	if err == nil {
		t.Fatal("expected error for non-string argument")
	}
}

// ============================================================
// @inline Tests
// ============================================================

func TestMacro_InlineExpansion(t *testing.T) {
	me := NewMacroExpander()
	node, err := me.handleInline(&parser.MacroExpandExpr{
		MacroName: "inline",
		Args: []parser.Node{
			&parser.CallExpr{Function: "add", Args: []parser.Node{
				&parser.IntLiteral{Value: "1"},
				&parser.IntLiteral{Value: "2"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("inline failed: %v", err)
	}
	ce, ok := node.(*parser.CallExpr)
	if !ok {
		t.Fatalf("expected CallExpr, got %T", node)
	}
	if ce.Function != "add" {
		t.Errorf("expected 'add', got '%s'", ce.Function)
	}
}

func TestMacro_InlineRequiresOneArg(t *testing.T) {
	me := NewMacroExpander()
	_, err := me.handleInline(&parser.MacroExpandExpr{
		MacroName: "inline",
		Args:      []parser.Node{},
	})
	if err == nil {
		t.Fatal("expected error for 0 args")
	}
}

// ============================================================
// @const_eval Tests
// ============================================================

func TestMacro_ConstEvalSuccess(t *testing.T) {
	me := NewMacroExpander()
	node, err := me.handleConstEval(&parser.MacroExpandExpr{
		MacroName: "const_eval",
		Args: []parser.Node{
			&parser.BinaryExpr{
				Left:     &parser.IntLiteral{Value: "2"},
				Operator: "+",
				Right:    &parser.IntLiteral{Value: "3"},
			},
		},
	})
	if err != nil {
		t.Fatalf("const_eval failed: %v", err)
	}
	il, ok := node.(*parser.IntLiteral)
	if !ok || il.Value != "5" {
		t.Errorf("expected 5, got %v", node)
	}
}

func TestMacro_ConstEvalCannotEval(t *testing.T) {
	me := NewMacroExpander()
	_, err := me.handleConstEval(&parser.MacroExpandExpr{
		MacroName: "const_eval",
		Args: []parser.Node{
			&parser.Identifier{Name: "runtime_var"},
		},
	})
	if err == nil {
		t.Fatal("expected error for non-const expression")
	}
}

// ============================================================
// ForEach Macro Tests
// ============================================================

func TestMacro_ForEach(t *testing.T) {
	me := NewMacroExpander()
	me.PushScope("foreach")

	items := []parser.Node{
		&parser.IntLiteral{Value: "1"},
		&parser.IntLiteral{Value: "2"},
		&parser.IntLiteral{Value: "3"},
	}

	template := &parser.PrintStmt{Value: &parser.Identifier{Name: "item"}}

	result, err := ForEach(items, "item", template, me)
	if err != nil {
		t.Fatalf("foreach failed: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(result))
	}

	for i, node := range result {
		ps, ok := node.(*parser.PrintStmt)
		if !ok {
			t.Errorf("expected PrintStmt at %d, got %T", i, node)
			continue
		}
		il, ok := ps.Value.(*parser.IntLiteral)
		if !ok || il.Value != string(rune('1'+i)) {
			t.Errorf("expected %d, got %v", i+1, ps.Value)
		}
	}
}

func TestMacro_ForEachEmpty(t *testing.T) {
	me := NewMacroExpander()
	me.PushScope("foreach_empty")

	result, err := ForEach([]parser.Node{}, "item",
		&parser.PrintStmt{Value: &parser.Identifier{Name: "item"}}, me)
	if err != nil {
		t.Fatalf("foreach empty failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}
