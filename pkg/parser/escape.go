package parser

// Phase 49: Escape Analysis Foundation
//
// Determines which local variables "escape" their declaring scope.
// A variable escapes if it is:
//   - Passed as an argument to any function
//   - Returned from the enclosing function
//   - Captured by a lambda/closure defined in a nested scope
//   - Assigned to a heap-structured container (array, map)
//
// A variable that does NOT escape can theoretically be stack-allocated
// or even kept in a register. This is the foundation for Karkain's
// allocation model: primitives that don't escape never touch the heap.

// MarkVarEscaping walks a function body and sets Escapes=true on any
// VarDeclStmt whose variable is used in an escaping position.
func MarkVarEscaping(body []Node) {
	escaping := make(map[string]bool)

	for _, stmt := range body {
		scanStmtForEscapes(stmt, escaping)
	}

	for _, stmt := range body {
		if vd, ok := stmt.(*VarDeclStmt); ok {
			if escaping[vd.Name] {
				vd.Escapes = true
			}
		}
	}
}

// scanStmtForEscapes finds identifiers that appear in escape positions
// and adds them to the escaping set.
func scanStmtForEscapes(stmt Node, escaping map[string]bool) {
	switch s := stmt.(type) {
	case *VarDeclStmt:
		if s.Value != nil {
			scanExprForEscapes(s.Value, escaping)
		}
	case *ExprStmt:
		if s.Expression != nil {
			scanExprForEscapes(s.Expression, escaping)
		}
	case *ReturnStmt:
		if s.Value != nil {
			collectIdentifiers(s.Value, escaping)
		}
	case *IfStmt:
		scanExprForEscapes(s.Condition, escaping)
		for _, bodyStmt := range s.Consequence {
			scanStmtForEscapes(bodyStmt, escaping)
		}
		for _, bodyStmt := range s.Alternative {
			scanStmtForEscapes(bodyStmt, escaping)
		}
	case *WhileStmt:
		scanExprForEscapes(s.Condition, escaping)
		for _, bodyStmt := range s.Body {
			scanStmtForEscapes(bodyStmt, escaping)
		}
	case *ForStmt:
		if s.Init != nil {
			scanExprForEscapes(s.Init, escaping)
		}
		scanExprForEscapes(s.Condition, escaping)
		if s.Post != nil {
			scanExprForEscapes(s.Post, escaping)
		}
		for _, bodyStmt := range s.Body {
			scanStmtForEscapes(bodyStmt, escaping)
		}
	case *ForInStmt:
		if s.Iter != nil {
			scanExprForEscapes(s.Iter, escaping)
		}
		for _, bodyStmt := range s.Body {
			scanStmtForEscapes(bodyStmt, escaping)
		}
	case *MatchExpr:
		scanExprForEscapes(s.Value, escaping)
		for _, arm := range s.Arms {
			scanStmtForEscapes(arm.Body, escaping)
		}
	case *PrintStmt:
		collectIdentifiers(s.Value, escaping)
	case *SendExpr:
		collectIdentifiers(s.Message, escaping)
		collectIdentifiers(s.Channel, escaping)
	}
}

// scanExprForEscapes checks if an expression causes variables to escape.
// Call arguments and lambda captures are the primary escape positions.
func scanExprForEscapes(expr Node, escaping map[string]bool) {
	switch e := expr.(type) {
	case *CallExpr:
		for _, arg := range e.Args {
			collectIdentifiers(arg, escaping)
		}
	case *IndirectCallExpr:
		// Phase 133: the callee value escapes through the call.
		collectIdentifiers(e.Target, escaping)
		for _, arg := range e.Args {
			collectIdentifiers(arg, escaping)
		}
	case *LambdaExpr:
		for _, bodyStmt := range e.Body {
			scanStmtForEscapes(bodyStmt, escaping)
		}
	case *BinaryExpr:
		scanExprForEscapes(e.Left, escaping)
		scanExprForEscapes(e.Right, escaping)
	case *UnaryExpr:
		scanExprForEscapes(e.Operand, escaping)
	case *IndexExpr:
		collectIdentifiers(e.Left, escaping)
		scanExprForEscapes(e.Index, escaping)
	case *DotExpr:
		collectIdentifiers(e.Left, escaping)
	case *ArrayLiteral:
		for _, elem := range e.Elements {
			collectIdentifiers(elem, escaping)
		}
	case *MapLiteral:
		for _, k := range e.Keys {
			collectIdentifiers(k, escaping)
		}
		for _, v := range e.Values {
			collectIdentifiers(v, escaping)
		}
	case *Identifier:
		// Identifiers in non-escape positions don't cause escaping
		// (e.g., x in "let y = x + 1" — x is just read, not escaped)
	}
}

// collectIdentifiers adds all Identifier names found in expr to the set.
func collectIdentifiers(expr Node, ids map[string]bool) {
	switch e := expr.(type) {
	case *Identifier:
		ids[e.Name] = true
	case *BinaryExpr:
		collectIdentifiers(e.Left, ids)
		collectIdentifiers(e.Right, ids)
	case *UnaryExpr:
		collectIdentifiers(e.Operand, ids)
	case *CallExpr:
		for _, arg := range e.Args {
			collectIdentifiers(arg, ids)
		}
	case *IndirectCallExpr:
		// Phase 133: identifiers flow through computed callees too.
		collectIdentifiers(e.Target, ids)
		for _, arg := range e.Args {
			collectIdentifiers(arg, ids)
		}
	case *IndexExpr:
		collectIdentifiers(e.Left, ids)
		collectIdentifiers(e.Index, ids)
	case *DotExpr:
		collectIdentifiers(e.Left, ids)
	case *LambdaExpr:
		for _, bodyStmt := range e.Body {
			collectIdentsInStmt(bodyStmt, ids)
		}
	}
}

// collectIdentsInStmt adds all Identifier names found in a statement to the set.
func collectIdentsInStmt(stmt Node, ids map[string]bool) {
	switch s := stmt.(type) {
	case *VarDeclStmt:
		if s.Value != nil {
			collectIdentifiers(s.Value, ids)
		}
	case *ExprStmt:
		if s.Expression != nil {
			collectIdentifiers(s.Expression, ids)
		}
	case *ReturnStmt:
		if s.Value != nil {
			collectIdentifiers(s.Value, ids)
		}
	case *IfStmt:
		collectIdentifiers(s.Condition, ids)
		for _, bodyStmt := range s.Consequence {
			collectIdentsInStmt(bodyStmt, ids)
		}
		for _, bodyStmt := range s.Alternative {
			collectIdentsInStmt(bodyStmt, ids)
		}
	case *WhileStmt:
		collectIdentifiers(s.Condition, ids)
		for _, bodyStmt := range s.Body {
			collectIdentsInStmt(bodyStmt, ids)
		}
	case *PrintStmt:
		collectIdentifiers(s.Value, ids)
	}
}
