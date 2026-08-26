package parser

const arenaChunkSize = 4096

// Arena allocates AST nodes in contiguous memory blocks (chunks) rather
// than issuing individual heap allocations per node. This reduces GC
// pressure and improves cache locality for large ASTs.
//
// Nodes are allocated sequentially within chunks. The arena tracks which
// chunk and offset each node lives at, enabling O(1) lookup by NodeID.
// The arena does NOT support individual deallocation — all nodes are
// freed when the arena itself is garbage-collected.
type Arena struct {
	chunks      [][]Node
	chunk0      [arenaChunkSize]Node // inline first chunk to avoid one pointer indirection
	current     int                  // index within current chunk
	chunkIdx    int                  // current chunk index
	total       uint32               // total nodes allocated
	currentLine int                  // Phase 55b: source line for next allocation
}

// NodeID is a compact 32-bit identifier for an AST node allocated in the arena.
// It replaces 64-bit raw pointers, halving the per-reference memory footprint.
type NodeID uint32

// NewArena creates a new arena with the first chunk pre-allocated inline.
func NewArena() *Arena {
	a := &Arena{
		chunks:   make([][]Node, 0, 16),
		current:  0,
		chunkIdx: 0,
		total:    0,
	}
	// Use the inline chunk0 as the first chunk
	a.chunks = append(a.chunks, a.chunk0[:0])
	return a
}

// SetLine records the source line number for the next allocation.
func (a *Arena) SetLine(line int) { a.currentLine = line }

// Alloc allocates a new AST node in the arena and returns a NodeID.
// The node is constructed by calling the provided function.
func (a *Arena) Alloc(fn func() Node) NodeID {
	if a.current >= arenaChunkSize {
		// Current chunk is full, allocate a new one
		newChunk := make([]Node, 0, arenaChunkSize)
		a.chunks = append(a.chunks, newChunk)
		a.chunkIdx++
		a.current = 0
	}

	node := fn()
	setNodeLine(node, a.currentLine)

	id := NodeID(a.total)
	a.chunks[a.chunkIdx] = append(a.chunks[a.chunkIdx], node)
	a.current++
	a.total++
	return id
}

// Get retrieves the node at the given NodeID. This is O(1).
func (a *Arena) Get(id NodeID) Node {
	chunkIdx := int(id) / arenaChunkSize
	offset := int(id) % arenaChunkSize
	return a.chunks[chunkIdx][offset]
}

// Len returns the total number of nodes allocated in the arena.
func (a *Arena) Len() uint32 {
	return a.total
}

// Reset clears the arena, allowing reuse of allocated memory.
// This is useful between compilation passes.
func (a *Arena) Reset() {
	for i := range a.chunks {
		for j := range a.chunks[i] {
			a.chunks[i][j] = nil
		}
		a.chunks[i] = a.chunks[i][:0]
	}
	// Reuse chunk0
	if len(a.chunks) == 0 {
		a.chunks = append(a.chunks, a.chunk0[:0])
	}
	a.current = 0
	a.chunkIdx = 0
	a.total = 0
}

// --- Helper methods for allocating specific AST node types ---

func (a *Arena) AllocFuncDecl(name string, params []string, body []Node, genericParams []GenericTypeParam) *FuncDecl {
	id := a.Alloc(func() Node {
		return &FuncDecl{Name: name, Params: params, Body: body, GenericParams: genericParams}
	})
	return a.Get(id).(*FuncDecl)
}

func (a *Arena) AllocVarDeclStmt(name string, value Node, typ string, isMatrix bool) *VarDeclStmt {
	id := a.Alloc(func() Node {
		return &VarDeclStmt{Name: name, Value: value, Type: typ, IsMatrix: isMatrix}
	})
	return a.Get(id).(*VarDeclStmt)
}

func (a *Arena) AllocReturnStmt(value Node) *ReturnStmt {
	id := a.Alloc(func() Node {
		return &ReturnStmt{Value: value}
	})
	return a.Get(id).(*ReturnStmt)
}

func (a *Arena) AllocExprStmt(expression Node) *ExprStmt {
	id := a.Alloc(func() Node {
		return &ExprStmt{Expression: expression}
	})
	return a.Get(id).(*ExprStmt)
}

func (a *Arena) AllocIfStmt(condition Node, consequence []Node, alternative []Node) *IfStmt {
	id := a.Alloc(func() Node {
		return &IfStmt{Condition: condition, Consequence: consequence, Alternative: alternative}
	})
	return a.Get(id).(*IfStmt)
}

func (a *Arena) AllocWhileStmt(condition Node, body []Node) *WhileStmt {
	id := a.Alloc(func() Node {
		return &WhileStmt{Condition: condition, Body: body}
	})
	return a.Get(id).(*WhileStmt)
}

func (a *Arena) AllocForInStmt(varName string, iter Node, body []Node) *ForInStmt {
	id := a.Alloc(func() Node {
		return &ForInStmt{VarName: varName, Iter: iter, Body: body}
	})
	return a.Get(id).(*ForInStmt)
}

func (a *Arena) AllocForInStmtWithKey(keyName string, varName string, iter Node, body []Node) *ForInStmt {
	id := a.Alloc(func() Node {
		return &ForInStmt{KeyName: keyName, VarName: varName, Iter: iter, Body: body}
	})
	return a.Get(id).(*ForInStmt)
}

func (a *Arena) AllocBreakStmt() *BreakStmt {
	id := a.Alloc(func() Node { return &BreakStmt{} })
	return a.Get(id).(*BreakStmt)
}

func (a *Arena) AllocContinueStmt() *ContinueStmt {
	id := a.Alloc(func() Node { return &ContinueStmt{} })
	return a.Get(id).(*ContinueStmt)
}

func (a *Arena) AllocLambdaExpr(params []string, paramTypes []string, body []Node) *LambdaExpr {
	id := a.Alloc(func() Node {
		return &LambdaExpr{Params: params, ParamTypes: paramTypes, Body: body}
	})
	return a.Get(id).(*LambdaExpr)
}

func (a *Arena) AllocForStmt(init Node, condition Node, post Node, body []Node) *ForStmt {
	id := a.Alloc(func() Node {
		return &ForStmt{Init: init, Condition: condition, Post: post, Body: body}
	})
	return a.Get(id).(*ForStmt)
}

func (a *Arena) AllocPrintStmt(value Node) *PrintStmt {
	id := a.Alloc(func() Node {
		return &PrintStmt{Value: value}
	})
	return a.Get(id).(*PrintStmt)
}

func (a *Arena) AllocBinaryExpr(left Node, operator string, right Node) *BinaryExpr {
	id := a.Alloc(func() Node {
		return &BinaryExpr{Left: left, Operator: operator, Right: right}
	})
	return a.Get(id).(*BinaryExpr)
}

func (a *Arena) AllocUnaryExpr(operator string, operand Node) *UnaryExpr {
	id := a.Alloc(func() Node {
		return &UnaryExpr{Operator: operator, Operand: operand}
	})
	return a.Get(id).(*UnaryExpr)
}

func (a *Arena) AllocCallExpr(function string, args []Node, isCFunc bool) *CallExpr {
	id := a.Alloc(func() Node {
		return &CallExpr{Function: function, Args: args, IsCFunc: isCFunc}
	})
	return a.Get(id).(*CallExpr)
}

func (a *Arena) AllocIdentifier(name string) *Identifier {
	id := a.Alloc(func() Node {
		return &Identifier{Name: name}
	})
	return a.Get(id).(*Identifier)
}

func (a *Arena) AllocIntLiteral(value string) *IntLiteral {
	id := a.Alloc(func() Node {
		return &IntLiteral{Value: value}
	})
	return a.Get(id).(*IntLiteral)
}

func (a *Arena) AllocFloat64Literal(value string) *Float64Literal {
	id := a.Alloc(func() Node {
		return &Float64Literal{Value: value}
	})
	return a.Get(id).(*Float64Literal)
}

func (a *Arena) AllocBigIntLiteral(value string) *BigIntLiteral {
	id := a.Alloc(func() Node {
		return &BigIntLiteral{Value: value}
	})
	return a.Get(id).(*BigIntLiteral)
}

func (a *Arena) AllocBigFloatLiteral(value string) *BigFloatLiteral {
	id := a.Alloc(func() Node {
		return &BigFloatLiteral{Value: value}
	})
	return a.Get(id).(*BigFloatLiteral)
}

func (a *Arena) AllocStringLiteral(value string) *StringLiteral {
	id := a.Alloc(func() Node {
		return &StringLiteral{Value: value}
	})
	return a.Get(id).(*StringLiteral)
}

func (a *Arena) AllocBoolLiteral(value bool) *BoolLiteral {
	id := a.Alloc(func() Node {
		return &BoolLiteral{Value: value}
	})
	return a.Get(id).(*BoolLiteral)
}

func (a *Arena) AllocIndexExpr(left Node, index Node) *IndexExpr {
	id := a.Alloc(func() Node {
		return &IndexExpr{Left: left, Index: index}
	})
	return a.Get(id).(*IndexExpr)
}

func (a *Arena) AllocDotExpr(left Node, right string) *DotExpr {
	id := a.Alloc(func() Node {
		return &DotExpr{Left: left, Right: right}
	})
	return a.Get(id).(*DotExpr)
}

func (a *Arena) AllocArrayLiteral(elements []Node) *ArrayLiteral {
	id := a.Alloc(func() Node {
		return &ArrayLiteral{Elements: elements}
	})
	return a.Get(id).(*ArrayLiteral)
}

func (a *Arena) AllocMapLiteral(keys []Node, values []Node) *MapLiteral {
	id := a.Alloc(func() Node {
		return &MapLiteral{Keys: keys, Values: values}
	})
	return a.Get(id).(*MapLiteral)
}

func (a *Arena) AllocMatrixIndexExpr(matrix Node, row Node, col Node) *MatrixIndexExpr {
	id := a.Alloc(func() Node {
		return &MatrixIndexExpr{Matrix: matrix, Row: row, Col: col}
	})
	return a.Get(id).(*MatrixIndexExpr)
}

func (a *Arena) AllocSendExpr(channel Node, message Node, isSync bool, timeout Node) *SendExpr {
	id := a.Alloc(func() Node {
		return &SendExpr{Channel: channel, Message: message, IsSync: isSync, Timeout: timeout}
	})
	return a.Get(id).(*SendExpr)
}

func (a *Arena) AllocStructLiteral(typeName string, fields []Node) *StructLiteral {
	id := a.Alloc(func() Node {
		return &StructLiteral{TypeName: typeName, Fields: fields}
	})
	return a.Get(id).(*StructLiteral)
}

func (a *Arena) AllocStructDeclStmt(name string, fields []StructField, genericParams []GenericTypeParam) *StructDeclStmt {
	id := a.Alloc(func() Node {
		return &StructDeclStmt{Name: name, Fields: fields, GenericParams: genericParams}
	})
	return a.Get(id).(*StructDeclStmt)
}

func (a *Arena) AllocKernelDeclStmt(name string, params []Parameter, body []Node, workGroupX int, workGroupY int, workGroupZ int, genericParams []GenericTypeParam) *KernelDeclStmt {
	id := a.Alloc(func() Node {
		return &KernelDeclStmt{Name: name, Params: params, Body: body, WorkGroupX: workGroupX, WorkGroupY: workGroupY, WorkGroupZ: workGroupZ, GenericParams: genericParams}
	})
	return a.Get(id).(*KernelDeclStmt)
}

func (a *Arena) AllocProgram(statements []Node, cImports []*CImportBlock) *Program {
	id := a.Alloc(func() Node {
		return &Program{Statements: statements, CImports: cImports}
	})
	return a.Get(id).(*Program)
}

// setNodeLine sets the Line field on any AST node that has one.
// This is called by Alloc to propagate source line numbers from tokens.
func setNodeLine(node Node, line int) {
	if line == 0 || node == nil {
		return
	}
	switch n := node.(type) {
	case *Program: n.Line = line
	case *FuncDecl: n.Line = line
	case *VarDeclStmt: n.Line = line
	case *ReturnStmt: n.Line = line
	case *ExprStmt: n.Line = line
	case *IfStmt: n.Line = line
	case *WhileStmt: n.Line = line
	case *PrintStmt: n.Line = line
	case *BlockStmt: /* no Line field */
	case *StringLiteral: n.Line = line
	case *IntLiteral: n.Line = line
	case *Float64Literal: n.Line = line
	case *BigIntLiteral: n.Line = line
	case *BigFloatLiteral: n.Line = line
	case *Identifier: n.Line = line
	case *ArrayLiteral: n.Line = line
	case *MapLiteral: n.Line = line
	case *IndexExpr: n.Line = line
	case *SliceExpr: n.Line = line
	case *BinaryExpr: n.Line = line
	case *CallExpr: n.Line = line
	case *RawAccessExpr: n.Line = line
	case *BorrowExpr: n.Line = line
	case *MoveExpr: n.Line = line
	case *PropagateExpr: n.Line = line
	case *OptionSomeExpr: n.Line = line
	case *OptionNoneExpr: n.Line = line
	case *ResultOkExpr: n.Line = line
	case *ResultErrExpr: n.Line = line
	case *MatchExpr: n.Line = line
	case *SIMDBuiltinExpr: n.Line = line
	case *LinearTypeDecl: n.Line = line
	case *PackedStructDecl: n.Line = line
	case *EnumDecl: n.Line = line
	case *EnumVariantExpr: n.Line = line
	case *CImportBlock: n.Line = line
	case *AddressOf: n.Line = line
	case *Dereference: n.Line = line
	case *AllocExpr: n.Line = line
	case *FreeExpr: n.Line = line
	case *MatrixDecl: n.Line = line
	case *MatrixIndexExpr: n.Line = line
	case *DotExpr: n.Line = line
	case *QRegDeclStmt: n.Line = line
	case *GateApplyStmt: n.Line = line
	case *MeasureExpr: n.Line = line
	case *ActorDeclStmt: n.Line = line
	case *SpawnExpr: n.Line = line
	case *ReceiveStmt: n.Line = line
	case *SendExpr: n.Line = line
	case *MacroDeclStmt: n.Line = line
	case *MacroExpandExpr: n.Line = line
	case *QuoteExpr: n.Line = line
	case *UnquoteExpr: n.Line = line
	case *ComptimeExpr: n.Line = line
	case *ComptimeStmt: n.Line = line
	case *ReflectTypeExpr: n.Line = line
	case *DeriveExpr: n.Line = line
	case *TagExpr: n.Line = line
	case *KernelDeclStmt: n.Line = line
	case *GlobalIdExpr: n.Line = line
	case *BarrierStmt: n.Line = line
	case *StructDeclStmt: n.Line = line
	case *StructLiteral: n.Line = line
	case *BoolLiteral: n.Line = line
	case *ForStmt: n.Line = line
	case *ForInStmt: n.Line = line
	case *BreakStmt: n.Line = line
	case *ContinueStmt: n.Line = line
	case *LambdaExpr: n.Line = line
	case *FuncRefExpr: n.Line = line
	case *UnaryExpr: n.Line = line
	case *TraitDeclStmt: n.Line = line
	case *ImplDeclStmt: n.Line = line
	case *TensorStmt: n.Line = line
	case *TensorOpExpr: n.Line = line
	case *TensorReturnStmt: n.Line = line
	case *TensorIndexExpr: n.Line = line
	case *TensorShapeOfExpr: n.Line = line
	case *CircuitDecl: n.Line = line
	case *QPUOpExpr: n.Line = line
	case *CircuitReturnStmt: n.Line = line
	case *QubitIndexExpr: n.Line = line
	case *QubitAssignStmt: n.Line = line
	case *StmtList: n.Line = line
	case *CoroutineDecl: n.Line = line
	case *AsyncExpr: n.Line = line
	case *AwaitExpr: n.Line = line
	case *YieldExpr: n.Line = line
	case *ChSendExpr: n.Line = line
	case *ChRecvExpr: n.Line = line
	case *ChDeclExpr: n.Line = line
	case *SelectStmt: n.Line = line
	case *GreenSpawnExpr: n.Line = line
	case *AwaitAllExpr: n.Line = line
	}
}
