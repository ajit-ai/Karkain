package sema

import (
	"fmt"
	"strings"

	"karkain/pkg/parser"
)

// ============================================================
// Phase 37: Actor System Semantic Analysis
// Validates actor declarations, message handlers, spawn expressions,
// send/receive patterns, and distributed dispatch correctness
// ============================================================

// ActorDef represents a fully analyzed actor definition
type ActorDef struct {
	Name       string
	State      []ActorStateDef
	Handlers   []ActorHandlerDef
	ParamTypes []string
	IsDistributed bool // can be spawned on remote nodes
}

// ActorStateDef is a validated state field
type ActorStateDef struct {
	Name     string
	Type     string
	HasDefault bool
}

// ActorHandlerDef is a validated message handler
type ActorHandlerDef struct {
	MessageType string
	ParamName   string
	ParamType   string
	IsReply     bool
	BodySize    int // number of statements in body
}

// ActorChecker performs semantic analysis on actor declarations
type ActorChecker struct {
	Actors     map[string]*ActorDef
	Errors     []string
	Warnings   []string
	CurrentActor string
}

// NewActorChecker creates a new actor semantic checker
func NewActorChecker() *ActorChecker {
	return &ActorChecker{
		Actors: make(map[string]*ActorDef),
	}
}

// ============================================================
// Program Processing
// ============================================================

// ProcessProgram scans a program for actor declarations
func (ac *ActorChecker) ProcessProgram(prog *parser.Program) error {
	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *parser.ActorDeclStmt:
			if err := ac.ProcessActorDecl(n); err != nil {
				ac.Errors = append(ac.Errors, err.Error())
			}
		case *parser.SpawnExpr:
			if err := ac.ProcessSpawnExpr(n); err != nil {
				ac.Errors = append(ac.Errors, err.Error())
			}
		case *parser.SendExpr:
			if err := ac.ProcessSendExpr(n); err != nil {
				ac.Errors = append(ac.Errors, err.Error())
			}
		}
	}

	if len(ac.Errors) > 0 {
		return fmt.Errorf("actor: %d error(s) found", len(ac.Errors))
	}
	return nil
}

// ============================================================
// Actor Declaration Validation
// ============================================================

// ProcessActorDecl validates an actor declaration
func (ac *ActorChecker) ProcessActorDecl(decl *parser.ActorDeclStmt) error {
	if decl.Name == "" {
		return fmt.Errorf("actor: declaration must have a name")
	}

	// Check for duplicate actor names
	if _, exists := ac.Actors[decl.Name]; exists {
		return fmt.Errorf("actor: duplicate declaration '%s'", decl.Name)
	}

	def := &ActorDef{
		Name: decl.Name,
	}

	// Validate state fields
	for _, field := range decl.State {
		if err := ac.validateStateFieldName(field.Name, decl.Name); err != nil {
			ac.Errors = append(ac.Errors, err.Error())
			continue
		}
		if !isValidActorType(field.Type) {
			ac.Warnings = append(ac.Warnings,
				fmt.Sprintf("actor '%s': unusual state type '%s'", decl.Name, field.Type))
		}
		def.State = append(def.State, ActorStateDef{
			Name:       field.Name,
			Type:       field.Type,
			HasDefault: field.Default != nil,
		})
	}

	// Validate message handlers
	handlerTypes := make(map[string]bool)
	for _, handler := range decl.Handlers {
		if handler.MessageType == "" {
			ac.Errors = append(ac.Errors,
				fmt.Sprintf("actor '%s': handler must have a message type", decl.Name))
			continue
		}
		if handlerTypes[handler.MessageType] {
			ac.Errors = append(ac.Errors,
				fmt.Sprintf("actor '%s': duplicate handler for '%s'", decl.Name, handler.MessageType))
			continue
		}
		handlerTypes[handler.MessageType] = true

		if !isValidActorType(handler.ParamType) {
			ac.Warnings = append(ac.Warnings,
				fmt.Sprintf("actor '%s': handler '%s' has unusual param type '%s'",
					decl.Name, handler.MessageType, handler.ParamType))
		}

		def.Handlers = append(def.Handlers, ActorHandlerDef{
			MessageType: handler.MessageType,
			ParamName:   handler.ParamName,
			ParamType:   handler.ParamType,
			IsReply:     handler.IsReply,
			BodySize:    len(handler.Body),
		})
	}

	if len(def.Handlers) == 0 {
		ac.Warnings = append(ac.Warnings,
			fmt.Sprintf("actor '%s': has no message handlers", decl.Name))
	}

	def.IsDistributed = true // all actors are potentially distributed
	ac.Actors[decl.Name] = def
	return nil
}

func (ac *ActorChecker) validateStateFieldName(name, actorName string) error {
	if name == "" {
		return fmt.Errorf("actor '%s': state field must have a name", actorName)
	}
	reserved := map[string]bool{
		"self": true, "sender": true, "system": true,
		"mailbox": true, "context": true,
	}
	if reserved[name] {
		return fmt.Errorf("actor '%s': '%s' is a reserved state field name", actorName, name)
	}
	return nil
}

// ============================================================
// Spawn Expression Validation
// ============================================================

// ProcessSpawnExpr validates a spawn expression
func (ac *ActorChecker) ProcessSpawnExpr(expr *parser.SpawnExpr) error {
	if expr.ActorName == "" {
		return fmt.Errorf("actor: spawn requires an actor name")
	}

	// If remote spawn, validate node address
	if expr.NodeAddr != "" {
		if !isValidNodeAddr(expr.NodeAddr) {
			ac.Warnings = append(ac.Warnings,
				fmt.Sprintf("actor: spawn node address '%s' may be invalid", expr.NodeAddr))
		}
	}

	return nil
}

// ============================================================
// Send Expression Validation
// ============================================================

// ProcessSendExpr validates a send expression
func (ac *ActorChecker) ProcessSendExpr(expr *parser.SendExpr) error {
	if expr.Channel == nil {
		return fmt.Errorf("actor: send requires a target channel/actor")
	}
	if expr.Message == nil {
		return fmt.Errorf("actor: send requires a message")
	}

	// Validate timeout for sync send
	if expr.IsSync && expr.Timeout != nil {
		// Timeout should be a valid expression (compile-time constant preferred)
		if _, ok := expr.Timeout.(*parser.IntLiteral); !ok {
			if _, ok := expr.Timeout.(*parser.Float64Literal); !ok {
				ac.Warnings = append(ac.Warnings,
					"actor: sync send timeout should be a numeric literal")
			}
		}
	}

	return nil
}

// ============================================================
// Handler Lookup
// ============================================================

// GetHandler finds a handler for a message type in an actor
func (ac *ActorChecker) GetHandler(actorName, msgType string) *ActorHandlerDef {
	def, ok := ac.Actors[actorName]
	if !ok {
		return nil
	}
	for i := range def.Handlers {
		if def.Handlers[i].MessageType == msgType {
			return &def.Handlers[i]
		}
	}
	return nil
}

// HasActor checks if an actor is declared
func (ac *ActorChecker) HasActor(name string) bool {
	_, ok := ac.Actors[name]
	return ok
}

// GetActor returns an actor definition
func (ac *ActorChecker) GetActor(name string) *ActorDef {
	return ac.Actors[name]
}

// GetActorNames returns all declared actor names
func (ac *ActorChecker) GetActorNames() []string {
	names := make([]string, 0, len(ac.Actors))
	for name := range ac.Actors {
		names = append(names, name)
	}
	return names
}

// ============================================================
// Type Checking Helpers
// ============================================================

// ValidateMessageType checks that a message type is valid
func (ac *ActorChecker) ValidateMessageType(actorName, msgType string) error {
	def, ok := ac.Actors[actorName]
	if !ok {
		return fmt.Errorf("actor '%s' not declared", actorName)
	}
	for _, h := range def.Handlers {
		if h.MessageType == msgType {
			return nil
		}
	}
	return fmt.Errorf("actor '%s' has no handler for message type '%s'", actorName, msgType)
}

// ValidateReplyTarget checks that a reply handler exists
func (ac *ActorChecker) ValidateReplyTarget(actorName string) error {
	def, ok := ac.Actors[actorName]
	if !ok {
		return fmt.Errorf("actor '%s' not declared", actorName)
	}
	for _, h := range def.Handlers {
		if h.IsReply {
			return nil
		}
	}
	return fmt.Errorf("actor '%s' has no reply-capable handler", actorName)
}

// ============================================================
// Distributed Validation
// ============================================================

// ValidateDistributedSpawn checks if an actor can be spawned remotely
func (ac *ActorChecker) ValidateDistributedSpawn(actorName, nodeAddr string) error {
	def, ok := ac.Actors[actorName]
	if !ok {
		return fmt.Errorf("actor '%s' not declared", actorName)
	}
	if !def.IsDistributed {
		return fmt.Errorf("actor '%s' is not marked as distributed", actorName)
	}
	if nodeAddr == "" {
		return fmt.Errorf("distributed spawn requires a node address")
	}
	return nil
}

// ============================================================
// Type Validation Helpers
// ============================================================

func isValidActorType(t string) bool {
	validTypes := map[string]bool{
		"i8": true, "i16": true, "i32": true, "i64": true,
		"u8": true, "u16": true, "u32": true, "u64": true,
		"f32": true, "f64": true, "bool": true, "string": true,
		"void": true, "*void": true, "*i8": true,
	}
	if validTypes[t] {
		return true
	}
	// Allow generic types like Tensor<f32, [128, 128]>
	if strings.HasPrefix(t, "Tensor<") || strings.HasPrefix(t, "Array<") {
		return true
	}
	return false
}

func isValidNodeAddr(addr string) bool {
	// Simple validation: must contain : for host:port
	return strings.Contains(addr, ":")
}

// ============================================================
// Code Generation Helpers
// ============================================================

// GenerateActorBridge produces runtime bridge code for an actor
func (ac *ActorChecker) GenerateActorBridge(def *ActorDef) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("// Actor Bridge: %s\n", def.Name))
	b.WriteString(fmt.Sprintf("// State fields: %d\n", len(def.State)))
	for i, s := range def.State {
		b.WriteString(fmt.Sprintf("//   [%d] %s: %s\n", i, s.Name, s.Type))
	}
	b.WriteString(fmt.Sprintf("// Handlers: %d\n", len(def.Handlers)))
	for i, h := range def.Handlers {
		reply := ""
		if h.IsReply {
			reply = " [reply]"
		}
		b.WriteString(fmt.Sprintf("//   [%d] %s(%s: %s)%s\n",
			i, h.MessageType, h.ParamName, h.ParamType, reply))
	}
	return b.String()
}
