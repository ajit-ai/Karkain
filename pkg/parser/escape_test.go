package parser

import (
	"testing"
)

func TestMarkVarEscaping(t *testing.T) {
	// Case 1: variable passed to a function call should escape
	body := []Node{
		&VarDeclStmt{Name: "x", Value: &IntLiteral{Value: "42"}},
		&ExprStmt{Expression: &CallExpr{Function: "print", Args: []Node{&Identifier{Name: "x"}}}},
	}
	MarkVarEscaping(body)
	vd := body[0].(*VarDeclStmt)
	if !vd.Escapes {
		t.Error("x passed to function should escape")
	}

	// Case 2: variable used only in arithmetic should not escape
	body2 := []Node{
		&VarDeclStmt{Name: "a", Value: &IntLiteral{Value: "1"}},
		&VarDeclStmt{Name: "b", Value: &BinaryExpr{Left: &Identifier{Name: "a"}, Operator: "+", Right: &IntLiteral{Value: "2"}}},
	}
	MarkVarEscaping(body2)
	vdA := body2[0].(*VarDeclStmt)
	if vdA.Escapes {
		t.Error("a used only in arithmetic should not escape")
	}

	// Case 3: variable returned should escape
	body3 := []Node{
		&VarDeclStmt{Name: "val", Value: &IntLiteral{Value: "10"}},
		&ReturnStmt{Value: &Identifier{Name: "val"}},
	}
	MarkVarEscaping(body3)
	vdVal := body3[0].(*VarDeclStmt)
	if !vdVal.Escapes {
		t.Error("val returned should escape")
	}
}

func TestMarkVarEscapingLambdaCapture(t *testing.T) {
	// Variable used inside a lambda should escape
	body := []Node{
		&VarDeclStmt{Name: "x", Value: &IntLiteral{Value: "5"}},
		&ExprStmt{Expression: &CallExpr{
			Function: "map",
			Args: []Node{
				&Identifier{Name: "arr"},
				&LambdaExpr{
					Params: []string{"item"},
					Body: []Node{
						&ReturnStmt{Value: &BinaryExpr{
							Left:  &Identifier{Name: "item"},
							Operator: "+",
							Right: &Identifier{Name: "x"},
						}},
					},
				},
			},
		}},
	}
	MarkVarEscaping(body)
	vdX := body[0].(*VarDeclStmt)
	if !vdX.Escapes {
		t.Error("x captured by lambda should escape")
	}
}
