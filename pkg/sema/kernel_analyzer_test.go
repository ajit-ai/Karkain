package sema

import (
	"karkain/pkg/parser"
	"strings"
	"testing"
)

func TestKernelAnalyzer_ForbiddenOps(t *testing.T) {
	tests := []struct {
		name        string
		kernel      *parser.KernelDeclStmt
		expectError bool
		errorMsg    string
	}{
		{
			name: "spawn inside kernel",
			kernel: &parser.KernelDeclStmt{
				Name:   "test_kernel",
				Params: []parser.Parameter{{Name: "data", Type: "[]float"}},
				Body: []parser.Node{
					&parser.ExprStmt{
						Expression: &parser.CallExpr{
							Function: "spawn",
							Args:     []parser.Node{&parser.Identifier{Name: "MyActor"}},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "actor spawn operation forbidden",
		},
		{
			name: "channel send inside kernel",
			kernel: &parser.KernelDeclStmt{
				Name:   "test_kernel",
				Params: []parser.Parameter{{Name: "data", Type: "[]float"}},
				Body: []parser.Node{
					&parser.ExprStmt{
						Expression: &parser.SendExpr{
							Channel: &parser.Identifier{Name: "ch"},
							Message: &parser.IntLiteral{Value: "42"},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "channel send operation forbidden",
		},
		{
			name: "channel receive inside kernel",
			kernel: &parser.KernelDeclStmt{
				Name:   "test_kernel",
				Params: []parser.Parameter{{Name: "data", Type: "[]float"}},
				Body: []parser.Node{
					&parser.ReceiveStmt{
						Channel: &parser.Identifier{Name: "ch"},
						VarName: "msg",
					},
				},
			},
			expectError: true,
			errorMsg:    "channel receive operation forbidden",
		},
		{
			name: "heap allocation inside kernel",
			kernel: &parser.KernelDeclStmt{
				Name:   "test_kernel",
				Params: []parser.Parameter{{Name: "data", Type: "[]float"}},
				Body: []parser.Node{
					&parser.ExprStmt{
						Expression: &parser.CallExpr{
							Function: "alloc",
							Args: []parser.Node{
								&parser.IntLiteral{Value: "100"},
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "heap allocation",
		},
		{
			name: "file I/O inside kernel",
			kernel: &parser.KernelDeclStmt{
				Name:   "test_kernel",
				Params: []parser.Parameter{{Name: "data", Type: "[]float"}},
				Body: []parser.Node{
					&parser.ExprStmt{
						Expression: &parser.CallExpr{
							Function: "readFile",
							Args: []parser.Node{
								&parser.StringLiteral{Value: "test.txt"},
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "file I/O operation",
		},
		{
			name: "recursive call inside kernel",
			kernel: &parser.KernelDeclStmt{
				Name:   "recursive_kernel",
				Params: []parser.Parameter{{Name: "data", Type: "[]float"}},
				Body: []parser.Node{
					&parser.ExprStmt{
						Expression: &parser.CallExpr{
							Function: "recursive_kernel",
							Args: []parser.Node{
								&parser.Identifier{Name: "data"},
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "recursive call forbidden",
		},
		{
			name: "valid kernel",
			kernel: &parser.KernelDeclStmt{
				Name:   "valid_kernel",
				Params: []parser.Parameter{{Name: "data", Type: "[]float"}},
				Body: []parser.Node{
					&parser.VarDeclStmt{
						Name:  "id",
						Value: &parser.GlobalIdExpr{Dimension: 0},
					},
					&parser.ExprStmt{
						Expression: &parser.BinaryExpr{
							Left:     &parser.Identifier{Name: "data"},
							Operator: "=",
							Right:    &parser.IntLiteral{Value: "42"},
						},
					},
				},
			},
			expectError: false,
		},
	}

	analyzer := NewKernelAnalyzer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := analyzer.AnalyzeKernel(tt.kernel)

			if tt.expectError {
				if len(errors) == 0 {
					t.Errorf("expected error containing '%s', got none", tt.errorMsg)
					return
				}
				found := false
				for _, err := range errors {
					if strings.Contains(err.Error(), tt.errorMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing '%s', got: %v", tt.errorMsg, errors)
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("expected no errors, got: %v", errors)
				}
			}
		})
	}
}

func TestKernelAnalyzer_InvalidParamTypes(t *testing.T) {
	kernel := &parser.KernelDeclStmt{
		Name: "test_kernel",
		Params: []parser.Parameter{
			{Name: "good", Type: "[]float"},
			{Name: "bad", Type: "MyStruct"},
		},
		Body: []parser.Node{},
	}

	analyzer := NewKernelAnalyzer()
	errors := analyzer.AnalyzeKernel(kernel)

	if len(errors) == 0 {
		t.Error("expected error for invalid parameter type, got none")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err.Error(), "invalid type") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about invalid type, got: %v", errors)
	}
}

func TestKernelAnalyzer_AllowedOperations(t *testing.T) {
	kernel := &parser.KernelDeclStmt{
		Name: "compute_kernel",
		Params: []parser.Parameter{
			{Name: "input", Type: "[]float"},
			{Name: "output", Type: "[]float"},
		},
		Body: []parser.Node{
			&parser.VarDeclStmt{
				Name:  "idx",
				Value: &parser.GlobalIdExpr{Dimension: 0},
			},
			&parser.BarrierStmt{},
			&parser.ExprStmt{
				Expression: &parser.BinaryExpr{
					Left: &parser.IndexExpr{
						Left:  &parser.Identifier{Name: "output"},
						Index: &parser.Identifier{Name: "idx"},
					},
					Operator: "=",
					Right: &parser.BinaryExpr{
						Left: &parser.IndexExpr{
							Left:  &parser.Identifier{Name: "input"},
							Index: &parser.Identifier{Name: "idx"},
						},
						Operator: "+",
						Right:    &parser.IntLiteral{Value: "1"},
					},
				},
			},
		},
	}

	analyzer := NewKernelAnalyzer()
	errors := analyzer.AnalyzeKernel(kernel)

	if len(errors) > 0 {
		t.Errorf("expected no errors for valid kernel, got: %v", errors)
	}
}
