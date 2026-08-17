package sema

import (
	"testing"

	"karkain/pkg/parser"
)

func TestActor_ProcessActorDecl(t *testing.T) {
	checker := NewActorChecker()
	decl := &parser.ActorDeclStmt{
		Name:   "ComputeNode",
		Params: []string{},
		State: []parser.ActorStateField{
			{Name: "state", Type: "i32", Default: &parser.IntLiteral{Value: "0"}},
			{Name: "count", Type: "u64"},
		},
		Handlers: []parser.ActorHandler{
			{
				MessageType: "process_tensor",
				ParamName:   "data",
				ParamType:   "Tensor<f32, [128, 128]>",
				Body:        []parser.Node{&parser.IntLiteral{Value: "42"}},
				IsReply:     true,
			},
			{
				MessageType: "ping",
				ParamName:   "msg",
				ParamType:   "string",
				Body:        []parser.Node{},
				IsReply:     false,
			},
		},
	}

	err := checker.ProcessActorDecl(decl)
	if err != nil {
		t.Fatalf("process actor decl failed: %v", err)
	}

	def := checker.GetActor("ComputeNode")
	if def == nil {
		t.Fatal("expected actor 'ComputeNode'")
	}
	if len(def.State) != 2 {
		t.Errorf("expected 2 state fields, got %d", len(def.State))
	}
	if len(def.Handlers) != 2 {
		t.Errorf("expected 2 handlers, got %d", len(def.Handlers))
	}
	if def.Handlers[0].MessageType != "process_tensor" {
		t.Errorf("expected handler 'process_tensor', got '%s'", def.Handlers[0].MessageType)
	}
	if !def.Handlers[0].IsReply {
		t.Error("expected process_tensor to be reply handler")
	}
}

func TestActor_DuplicateName(t *testing.T) {
	checker := NewActorChecker()
	decl := &parser.ActorDeclStmt{
		Name:   "Dup",
		Params: []string{},
	}

	err := checker.ProcessActorDecl(decl)
	if err != nil {
		t.Fatalf("first declaration should succeed: %v", err)
	}

	err = checker.ProcessActorDecl(decl)
	if err == nil {
		t.Fatal("duplicate declaration should fail")
	}
}

func TestActor_EmptyName(t *testing.T) {
	checker := NewActorChecker()
	decl := &parser.ActorDeclStmt{
		Name:   "",
		Params: []string{},
	}

	err := checker.ProcessActorDecl(decl)
	if err == nil {
		t.Fatal("empty name should fail")
	}
}

func TestActor_DuplicateHandler(t *testing.T) {
	checker := NewActorChecker()
	decl := &parser.ActorDeclStmt{
		Name: "TestActor",
		Handlers: []parser.ActorHandler{
			{MessageType: "ping", ParamName: "x", ParamType: "i32"},
			{MessageType: "ping", ParamName: "y", ParamType: "i32"},
		},
	}

	checker.ProcessActorDecl(decl)
	if len(checker.Errors) == 0 {
		t.Fatal("duplicate handler should produce error")
	}
}

func TestActor_EmptyHandlerType(t *testing.T) {
	checker := NewActorChecker()
	decl := &parser.ActorDeclStmt{
		Name: "TestActor",
		Handlers: []parser.ActorHandler{
			{MessageType: "", ParamName: "x", ParamType: "i32"},
		},
	}

	checker.ProcessActorDecl(decl)
	if len(checker.Errors) == 0 {
		t.Fatal("empty handler message type should produce error")
	}
}

func TestActor_ProcessProgram(t *testing.T) {
	checker := NewActorChecker()
	prog := &parser.Program{
		Statements: []parser.Node{
			&parser.ActorDeclStmt{
				Name: "NodeA",
				Handlers: []parser.ActorHandler{
					{MessageType: "data", ParamName: "d", ParamType: "i32"},
				},
			},
			&parser.ActorDeclStmt{
				Name: "NodeB",
				Handlers: []parser.ActorHandler{
					{MessageType: "ping", ParamName: "p", ParamType: "string"},
				},
			},
			&parser.SpawnExpr{ActorName: "NodeA"},
		},
	}

	err := checker.ProcessProgram(prog)
	if err != nil {
		t.Fatalf("process program failed: %v", err)
	}

	if len(checker.Actors) != 2 {
		t.Errorf("expected 2 actors, got %d", len(checker.Actors))
	}
}

func TestActor_ProcessSpawnExpr(t *testing.T) {
	checker := NewActorChecker()

	// Valid spawn
	err := checker.ProcessSpawnExpr(&parser.SpawnExpr{
		ActorName: "ComputeNode",
	})
	if err != nil {
		t.Errorf("valid spawn should succeed: %v", err)
	}

	// Empty name
	err = checker.ProcessSpawnExpr(&parser.SpawnExpr{
		ActorName: "",
	})
	if err == nil {
		t.Error("empty actor name should fail")
	}
}

func TestActor_ProcessSendExpr(t *testing.T) {
	checker := NewActorChecker()

	// Valid send
	err := checker.ProcessSendExpr(&parser.SendExpr{
		Channel: &parser.Identifier{Name: "actor1"},
		Message: &parser.IntLiteral{Value: "42"},
	})
	if err != nil {
		t.Errorf("valid send should succeed: %v", err)
	}

	// Missing channel
	err = checker.ProcessSendExpr(&parser.SendExpr{
		Channel: nil,
		Message: &parser.IntLiteral{Value: "42"},
	})
	if err == nil {
		t.Error("nil channel should fail")
	}

	// Missing message
	err = checker.ProcessSendExpr(&parser.SendExpr{
		Channel: &parser.Identifier{Name: "actor1"},
		Message: nil,
	})
	if err == nil {
		t.Error("nil message should fail")
	}
}

func TestActor_GetHandler(t *testing.T) {
	checker := NewActorChecker()
	checker.Actors["test"] = &ActorDef{
		Name: "test",
		Handlers: []ActorHandlerDef{
			{MessageType: "ping", ParamName: "x", ParamType: "i32"},
			{MessageType: "data", ParamName: "d", ParamType: "string"},
		},
	}

	h := checker.GetHandler("test", "ping")
	if h == nil {
		t.Fatal("expected ping handler")
	}
	if h.ParamType != "i32" {
		t.Errorf("expected i32, got %s", h.ParamType)
	}

	h = checker.GetHandler("test", "unknown")
	if h != nil {
		t.Error("expected nil for unknown handler")
	}

	h = checker.GetHandler("nonexistent", "ping")
	if h != nil {
		t.Error("expected nil for nonexistent actor")
	}
}

func TestActor_GetActorNames(t *testing.T) {
	checker := NewActorChecker()
	checker.Actors["alpha"] = &ActorDef{Name: "alpha"}
	checker.Actors["beta"] = &ActorDef{Name: "beta"}

	names := checker.GetActorNames()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

func TestActor_ValidateMessageType(t *testing.T) {
	checker := NewActorChecker()
	checker.Actors["node"] = &ActorDef{
		Name: "node",
		Handlers: []ActorHandlerDef{
			{MessageType: "process"},
		},
	}

	err := checker.ValidateMessageType("node", "process")
	if err != nil {
		t.Errorf("valid message type should pass: %v", err)
	}

	err = checker.ValidateMessageType("node", "unknown")
	if err == nil {
		t.Error("unknown message type should fail")
	}

	err = checker.ValidateMessageType("nonexistent", "process")
	if err == nil {
		t.Error("nonexistent actor should fail")
	}
}

func TestActor_ValidateReplyTarget(t *testing.T) {
	checker := NewActorChecker()
	checker.Actors["replier"] = &ActorDef{
		Name: "replier",
		Handlers: []ActorHandlerDef{
			{MessageType: "req", IsReply: true},
		},
	}
	checker.Actors["no_reply"] = &ActorDef{
		Name: "no_reply",
		Handlers: []ActorHandlerDef{
			{MessageType: "fire", IsReply: false},
		},
	}

	err := checker.ValidateReplyTarget("replier")
	if err != nil {
		t.Errorf("replier should have reply: %v", err)
	}

	err = checker.ValidateReplyTarget("no_reply")
	if err == nil {
		t.Error("no_reply should fail")
	}

	err = checker.ValidateReplyTarget("nonexistent")
	if err == nil {
		t.Error("nonexistent should fail")
	}
}

func TestActor_ValidateDistributedSpawn(t *testing.T) {
	checker := NewActorChecker()
	checker.Actors["dist"] = &ActorDef{
		Name:          "dist",
		IsDistributed: true,
	}
	checker.Actors["local_only"] = &ActorDef{
		Name:          "local_only",
		IsDistributed: false,
	}

	err := checker.ValidateDistributedSpawn("dist", "192.168.1.1:7900")
	if err != nil {
		t.Errorf("distributed spawn should pass: %v", err)
	}

	err = checker.ValidateDistributedSpawn("dist", "")
	if err == nil {
		t.Error("empty addr should fail")
	}

	err = checker.ValidateDistributedSpawn("local_only", "192.168.1.1:7900")
	if err == nil {
		t.Error("non-distributed actor should fail")
	}

	err = checker.ValidateDistributedSpawn("nonexistent", "addr")
	if err == nil {
		t.Error("nonexistent actor should fail")
	}
}

func TestActor_GenerateBridge(t *testing.T) {
	checker := NewActorChecker()
	def := &ActorDef{
		Name: "BridgeActor",
		State: []ActorStateDef{
			{Name: "val", Type: "i32"},
		},
		Handlers: []ActorHandlerDef{
			{MessageType: "increment", ParamName: "amount", ParamType: "i32", IsReply: false},
			{MessageType: "get_value", ParamName: "", ParamType: "", IsReply: true},
		},
	}

	bridge := checker.GenerateActorBridge(def)
	if bridge == "" {
		t.Error("bridge should not be empty")
	}
	if !containsStr(bridge, "BridgeActor") {
		t.Error("bridge should contain actor name")
	}
}

func TestActor_NoHandlersWarning(t *testing.T) {
	checker := NewActorChecker()
	decl := &parser.ActorDeclStmt{
		Name:     "EmptyActor",
		Handlers: []parser.ActorHandler{},
	}

	err := checker.ProcessActorDecl(decl)
	if err != nil {
		t.Errorf("empty handlers should not error: %v", err)
	}
	if len(checker.Warnings) == 0 {
		t.Error("expected warning for no handlers")
	}
}

func TestActor_HasActor(t *testing.T) {
	checker := NewActorChecker()
	checker.Actors["exists"] = &ActorDef{Name: "exists"}

	if !checker.HasActor("exists") {
		t.Error("should have 'exists'")
	}
	if checker.HasActor("missing") {
		t.Error("should not have 'missing'")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
