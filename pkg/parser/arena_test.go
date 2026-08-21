package parser

import (
	"testing"
)

func TestArenaAlloc(t *testing.T) {
	a := NewArena()

	id1 := a.AllocIdentifier("x")
	id2 := a.AllocIdentifier("y")
	id3 := a.AllocIntLiteral("42")

	if a.Len() != 3 {
		t.Fatalf("expected 3 nodes, got %d", a.Len())
	}

	if id1.Name != "x" {
		t.Errorf("expected Identifier{x}, got %v", id1)
	}
	if id2.Name != "y" {
		t.Errorf("expected Identifier{y}, got %v", id2)
	}
	if id3.Value != "42" {
		t.Errorf("expected IntLiteral{42}, got %v", id3)
	}
}

func TestArenaChunkBoundary(t *testing.T) {
	a := NewArena()

	for i := 0; i < arenaChunkSize+100; i++ {
		a.AllocIdentifier("node")
	}

	if a.Len() != uint32(arenaChunkSize+100) {
		t.Fatalf("expected %d nodes, got %d", arenaChunkSize+100, a.Len())
	}
}

func TestArenaReset(t *testing.T) {
	a := NewArena()

	for i := 0; i < 100; i++ {
		a.AllocIdentifier("node")
	}

	if a.Len() != 100 {
		t.Fatalf("expected 100 nodes, got %d", a.Len())
	}

	a.Reset()

	if a.Len() != 0 {
		t.Fatalf("expected 0 nodes after reset, got %d", a.Len())
	}

	a.AllocIdentifier("after_reset")
	if a.Len() != 1 {
		t.Fatalf("expected 1 node after reset, got %d", a.Len())
	}
}

func TestArenaTypedAlloc(t *testing.T) {
	a := NewArena()

	x := a.AllocIdentifier("x")
	y := a.AllocIdentifier("y")
	binExpr := a.AllocBinaryExpr(x, "+", y)
	ret := a.AllocReturnStmt(binExpr)
	decl := a.AllocVarDeclStmt("result", ret, "", false)

	if a.Len() != 5 {
		t.Fatalf("expected 5 nodes, got %d", a.Len())
	}
	if decl.Name != "result" {
		t.Errorf("expected VarDeclStmt name 'result', got %q", decl.Name)
	}
}
