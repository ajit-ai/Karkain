package sema

import "karkain/pkg/parser"

type VarOwnership int

const (
	Owned VarOwnership = iota
	Moved
	Borrowed
	BorrowedMut
)

type BorrowError struct {
	Message string
	Line    uint16
}

type varEntry struct {
	ownership VarOwnership
	typeName  string
	line      uint16
	// Phase 51: the declaring scope. Lets borrow reversion resolve the exact
	// binding that was borrowed even when the name is shadowed in nested scopes.
	declScope *scope
	// Phase 51: set when the variable escapes its declaring scope (Phase 49
	// escape analysis). Escaping values are heap-backed and outlive the scope;
	// non-escaping values may die with the scope.
	escapes bool
	// Phase 1.2 (v1.3.0): true when this binding is a function parameter.
	// Parameters outlive the function body's local scopes, so a reference to
	// a parameter is never dangling. A reference to a non-parameter local
	// returned from the function is a lifetime violation.
	isParam bool
	// Phase 1.2 (v1.3.0): if this binding was initialized from a borrow
	// (&x or &mut x) or address-of (&x), the name of the borrowed owner.
	// Empty if the binding is not a reference binding. Used to detect
	// dangling references through intermediate bindings.
	borrowsFrom string
	// v1.2.23 Phase 1 (linear types): true when this binding's type is a
	// `linear type`. Linear bindings must be consumed exactly once.
	linear bool
	// v1.2.23 Phase 1 (linear types): number of times this binding has been
	// used (identifier reads + move + borrow + field access). Checked at
	// scope exit against exactly 1 for linear bindings.
	uses int
}

type borrowRecord struct {
	name     string
	prev     VarOwnership
	// Phase 51: exact declaring scope of the borrowed binding. If nil, the
	// record is resolved by name within the current scope on pop.
	declScope *scope
}

type scope struct {
	parent    *scope
	vars      map[string]*varEntry
	usage     map[string]int
	borrows   []borrowRecord
	// Phase 51: names introduced by this scope. Used to mark them as dead
	// (lifetime ended) when the scope exits.
	decls []string
	// Phase 51: names whose binding lifetime ended at this scope. An identifier
	// resolving to a dead name (with no live shadow) is a use-after-scope-end.
	dead map[string]bool
	// fnRoot marks the top-level scope of a function declaration. Popping a
	// function root scope must NOT propagate dead names into the enclosing
	// (program/global) scope: otherwise a parameter of an earlier function
	// (e.g. "balance") would poison later identifiers with the same name
	// anywhere in the assembled program (struct field keys, member names),
	// producing false use-after-scope-end diagnostics across functions.
	fnRoot bool
}

func (s *scope) lookup(name string) (*varEntry, *scope) {
	if e, ok := s.vars[name]; ok {
		return e, s
	}
	if s.parent != nil {
		return s.parent.lookup(name)
	}
	return nil, nil
}

func (s *scope) lookupOwn(name string) *varEntry {
	if e, ok := s.vars[name]; ok {
		return e
	}
	return nil
}

type BorrowChecker struct {
	scope       *scope
	linearTypes map[string]bool
	errors      []BorrowError
}

func NewBorrowChecker() *BorrowChecker {
	return &BorrowChecker{
		linearTypes: make(map[string]bool),
	}
}

func (bc *BorrowChecker) pushScope() {
	bc.scope = &scope{
		parent: bc.scope,
		vars:   make(map[string]*varEntry),
		usage:  make(map[string]int),
		dead:   make(map[string]bool),
	}
}

func (bc *BorrowChecker) popScope() {
	if bc.scope == nil {
		return
	}
	// Revert borrows that were initiated in this scope. Resolve each borrowed
	// binding to its exact declaring scope so a shadowed name never reverts the
	// wrong (outer/restored) binding.
	for i := len(bc.scope.borrows) - 1; i >= 0; i-- {
		rec := bc.scope.borrows[i]
		var entry *varEntry
		if rec.declScope != nil {
			entry = rec.declScope.lookupOwn(rec.name)
		}
		if entry != nil {
			entry.ownership = rec.prev
		}
	}
	// Mark every binding introduced in this scope as dead in the parent. Any
	// later reference to these names (with no live shadow) is a use-after-scope.
	parent := bc.scope.parent
	if parent != nil && !bc.scope.fnRoot {
		for _, name := range bc.scope.decls {
			parent.dead[name] = true
		}
	}
	// v1.2.23 Phase 1 (linear types): every linear binding declared in this
	// scope must have been consumed exactly once. Counts live on the binding
	// itself (entry.uses), not on the scope map, so shadowed bindings never
	// conflate. This runs at every scope exit, including the global scope,
	// so the old whole-program checkLinearTypes pass is no longer needed.
	for name, entry := range bc.scope.vars {
		if !entry.linear {
			continue
		}
		if entry.uses == 0 {
			bc.errors = append(bc.errors, BorrowError{
				Message: "error[K160] linear type variable '" + name + "' of type '" + entry.typeName + "' was never used",
				Line:    entry.line,
			})
		} else if entry.uses > 1 {
			bc.errors = append(bc.errors, BorrowError{
				Message: "error[K160] linear type variable '" + name + "' of type '" + entry.typeName + "' was used " + itoa(entry.uses) + " times (must be exactly 1)",
				Line:    entry.line,
			})
		}
	}
	bc.scope = parent
}

func (bc *BorrowChecker) declare(name string, typeName string, line uint16) {
	bc.scope.vars[name] = &varEntry{
		ownership: Owned,
		typeName:  typeName,
		line:      line,
		declScope: bc.scope,
		linear:    bc.linearTypes[typeName],
	}
	bc.scope.decls = append(bc.scope.decls, name)
}

func (bc *BorrowChecker) lookup(name string) *varEntry {
	if bc.scope == nil {
		return nil
	}
	entry, _ := bc.scope.lookup(name)
	return entry
}

func (bc *BorrowChecker) setOwnership(name string, state VarOwnership) {
	if bc.scope == nil {
		return
	}
	entry, _ := bc.scope.lookup(name)
	if entry != nil {
		entry.ownership = state
	}
}

func (bc *BorrowChecker) trackUsage(name string) {
	if bc.scope == nil {
		return
	}
	// v1.2.23 Phase 1 (linear types): count on the resolved binding, not by
	// name, so shadowed bindings never conflate. The scope-level usage map is
	// retained for compatibility but is no longer the authority.
	if entry := bc.lookup(name); entry != nil {
		entry.uses++
	}
	bc.scope.usage[name]++
}

func (bc *BorrowChecker) Check(prog *parser.Program) []BorrowError {
	bc.errors = nil
	bc.scope = nil
	bc.linearTypes = make(map[string]bool)

	// First pass: collect linear type declarations
	for _, stmt := range prog.Statements {
		if ltd, ok := stmt.(*parser.LinearTypeDecl); ok {
			bc.linearTypes[ltd.Name] = true
		}
	}

	// Push global scope
	bc.pushScope()

	// Second pass: check ownership and borrow rules
	for _, stmt := range prog.Statements {
		bc.checkNode(stmt)
	}

	// v1.2.23 Phase 1: linear usage is verified per-scope at every popScope
	// (including this global scope), so no separate whole-program pass remains.

	bc.popScope()
	return bc.errors
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

func (bc *BorrowChecker) checkNode(node parser.Node) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parser.FuncDecl:
		bc.checkFuncDecl(n)
	case *parser.VarDeclStmt:
		bc.checkVarDecl(n)
	case *parser.ReturnStmt:
		bc.checkNode(n.Value)
		// Phase 1.2 (v1.3.0): lifetime analysis. A reference returned from a
		// function must not point at a local that dies when the function
		// returns. Parameters and escaping values are exempt.
		bc.checkReturnLifetime(n.Value, uint16(n.Line))
	case *parser.ExprStmt:
		bc.checkNode(n.Expression)
	case *parser.IfStmt:
		bc.checkIfStmt(n)
	case *parser.WhileStmt:
		bc.checkNode(n.Condition)
		bc.pushScope()
		bc.checkBlock(n.Body)
		bc.popScope()
	case *parser.ForStmt:
		bc.pushScope()
		bc.checkNode(n.Init)
		bc.checkNode(n.Condition)
		bc.checkBlock(n.Body)
		bc.checkNode(n.Post)
		bc.popScope()
	case *parser.BreakStmt, *parser.ContinueStmt:
		// no-op for borrow checking
	case *parser.PrintStmt:
		bc.checkNode(n.Value)
	case *parser.BinaryExpr:
		bc.checkBinaryExpr(n)
	case *parser.UnaryExpr:
		bc.checkNode(n.Operand)
	case *parser.CallExpr:
		for _, arg := range n.Args {
			bc.checkNode(arg)
		}
	case *parser.IndirectCallExpr:
		// Phase 133: borrow-check the computed callee and the arguments.
		bc.checkNode(n.Target)
		for _, arg := range n.Args {
			bc.checkNode(arg)
		}
	case *parser.ArrayLiteral:
		for _, elem := range n.Elements {
			bc.checkNode(elem)
		}
	case *parser.MapLiteral:
		for i, key := range n.Keys {
			bc.checkNode(key)
			bc.checkNode(n.Values[i])
		}
	case *parser.IndexExpr:
		bc.checkNode(n.Left)
		bc.checkNode(n.Index)
	case *parser.DotExpr:
		bc.checkNode(n.Left)
	case *parser.Identifier:
		bc.checkIdentifierUse(n)
	case *parser.BorrowExpr:
		bc.checkBorrow(n)
	case *parser.MoveExpr:
		bc.checkMove(n)
	case *parser.RawAccessExpr:
		bc.checkNode(n.Address)
		bc.checkNode(n.Value)
	case *parser.OptionSomeExpr:
		bc.checkNode(n.Value)
	case *parser.OptionNoneExpr:
		// no-op
	case *parser.ResultOkExpr:
		bc.checkNode(n.Value)
	case *parser.ResultErrExpr:
		bc.checkNode(n.Error)
	case *parser.MatchExpr:
		bc.checkMatchExpr(n)
	case *parser.PropagateExpr:
		bc.checkNode(n.Operand)
	case *parser.SIMDBuiltinExpr:
		for _, arg := range n.Args {
			bc.checkNode(arg)
		}
	case *parser.AtomicOp:
		for _, arg := range n.Args {
			bc.checkNode(arg)
		}
	case *parser.StructLiteral:
		for _, field := range n.Fields {
			// StructLiteral.Fields are BinaryExpr{Left: Identifier(fieldName),
			// Operator: "=", Right: value}. Only the value side counts as a
			// use; the field-name identifier is a label, not a binding.
			if be, ok := field.(*parser.BinaryExpr); ok && be.Operator == "=" {
				bc.checkNode(be.Right)
			} else {
				bc.checkNode(field)
			}
		}
	case *parser.AddressOf:
		bc.checkNode(n.Operand)
	case *parser.Dereference:
		bc.checkNode(n.Operand)
	case *parser.EnumVariantExpr:
		if n.Value != nil {
			bc.checkNode(n.Value)
		}
	case *parser.BlockStmt:
		bc.checkBlock(n.Statements)
	case *parser.ForInStmt:
		bc.checkNode(n.Iter)
		bc.pushScope()
		bc.declare(n.VarName, "", uint16(n.Line))
		if n.KeyName != "" {
			bc.declare(n.KeyName, "", uint16(n.Line))
		}
		bc.checkBlock(n.Body)
		bc.popScope()
	case *parser.LambdaExpr:
		bc.pushScope()
		bc.scope.fnRoot = true
		for i, param := range n.Params {
			typeName := ""
			if i < len(n.ParamTypes) {
				typeName = n.ParamTypes[i]
			}
			bc.declare(param, typeName, uint16(n.Line))
		}
		for _, stmt := range n.Body {
			bc.checkNode(stmt)
		}
		bc.popScope()
	}
}

func (bc *BorrowChecker) checkBlock(block []parser.Node) {
	bc.pushScope()
	for _, stmt := range block {
		bc.checkNode(stmt)
	}
	bc.popScope()
}

func (bc *BorrowChecker) checkFuncDecl(n *parser.FuncDecl) {
	bc.pushScope()
	bc.scope.fnRoot = true
	// Declare every owned function parameter as a live binding in the
	// function's scope. A parameter is alive for the entire function body.
	// Without this, a reference to a parameter would be treated as an unknown
	// name; if that name had been marked dead by an earlier (independent)
	// function's scope exit, it would be a false "use after scope has ended".
	for i, name := range n.Params {
		var typ string
		if i < len(n.ParamTypes) {
			typ = n.ParamTypes[i]
		}
		bc.declare(name, typ, uint16(n.Line))
		// Phase 1.2 (v1.3.0): mark function parameters as non-dangling.
		// A parameter outlives the function body's local scopes, so a
		// reference to a parameter returned from the function is safe.
		if e := bc.scope.lookupOwn(name); e != nil {
			e.isParam = true
		}
	}
	for _, stmt := range n.Body {
		bc.checkNode(stmt)
	}
	bc.popScope()
}

func (bc *BorrowChecker) checkVarDecl(n *parser.VarDeclStmt) {
	bc.checkNode(n.Value)
	typeName := n.Type
	if typeName == "" {
		typeName = bc.inferMatchType(n.Value)
	}
	bc.declare(n.Name, typeName, uint16(n.Line))
	// Phase 49 escape analysis marks n.Escapes; capture it so the borrow
	// checker records whether the value outlives its declaring scope.
	if bc.scope != nil {
		if e := bc.scope.lookupOwn(n.Name); e != nil {
			e.escapes = n.Escapes
			// Phase 1.2 (v1.3.0): record if this binding is a reference
			// to another value, for dangling-reference detection.
			if be, ok := n.Value.(*parser.BorrowExpr); ok {
				if id, ok := be.Operand.(*parser.Identifier); ok {
					e.borrowsFrom = id.Name
				}
			}
			if ao, ok := n.Value.(*parser.AddressOf); ok {
				if id, ok := ao.Operand.(*parser.Identifier); ok {
					e.borrowsFrom = id.Name
				}
			}
		}
	}
}

func (bc *BorrowChecker) checkIfStmt(n *parser.IfStmt) {
	bc.checkNode(n.Condition)
	bc.checkBlock(n.Consequence)
	bc.checkBlock(n.Alternative)
}

func (bc *BorrowChecker) checkBinaryExpr(n *parser.BinaryExpr) {
	bc.checkNode(n.Left)
	bc.checkNode(n.Right)
}

func (bc *BorrowChecker) checkMatchExpr(n *parser.MatchExpr) {
	bc.checkNode(n.Value)
	for _, arm := range n.Arms {
		bc.pushScope()
		// Bind pattern variables
		if arm.Pattern.Binding != "" {
			bc.declare(arm.Pattern.Binding, "", uint16(n.Line))
		}
		bc.checkNode(arm.Body)
		bc.popScope()
	}
	bc.checkExhaustiveMatch(n)
}

func (bc *BorrowChecker) checkIdentifierUse(n *parser.Identifier) {
	entry := bc.lookup(n.Name)
	if entry == nil {
		// Phase 51: a reference to a name whose binding lifetime already ended
		// (no live shadow exists) is a use-after-scope-end diagnostic.
		if bc.isDeadName(n.Name) {
			bc.errors = append(bc.errors, BorrowError{
				Message: "use of value '" + n.Name + "' after its scope has ended",
				Line:    uint16(n.Line),
			})
		}
		return
	}
	switch entry.ownership {
	case Moved:
		bc.errors = append(bc.errors, BorrowError{
			Message: "use of moved value: '" + n.Name + "'",
		})
	case BorrowedMut:
		bc.errors = append(bc.errors, BorrowError{
			Message: "cannot use '" + n.Name + "' while mutably borrowed",
		})
	}
	bc.trackUsage(n.Name)
}

// isDeadName reports whether name refers to a binding whose lexical scope has
// already ended, with no live shadow binding covering it.
func (bc *BorrowChecker) isDeadName(name string) bool {
	for s := bc.scope; s != nil; s = s.parent {
		if s.dead[name] {
			return true
		}
	}
	return false
}

func (bc *BorrowChecker) checkBorrow(n *parser.BorrowExpr) {
	ident, ok := n.Operand.(*parser.Identifier)
	if !ok {
		bc.checkNode(n.Operand)
		return
	}
	name := ident.Name
	entry := bc.lookup(name)
	if entry == nil {
		return
	}
	if entry.ownership == Moved {
		bc.errors = append(bc.errors, BorrowError{
			Message: "cannot borrow moved value: '" + name + "'",
		})
		return
	}
	if n.Mutable {
		if entry.ownership == Borrowed || entry.ownership == BorrowedMut {
			bc.errors = append(bc.errors, BorrowError{
				Message: "cannot mutably borrow '" + name + "' — already borrowed",
			})
			return
		}
		prev := entry.ownership
		entry.ownership = BorrowedMut
		bc.scope.borrows = append(bc.scope.borrows, borrowRecord{name: name, prev: prev, declScope: entry.declScope})
	} else {
		if entry.ownership == BorrowedMut {
			bc.errors = append(bc.errors, BorrowError{
				Message: "cannot borrow '" + name + "' — mutably borrowed",
			})
			return
		}
		prev := entry.ownership
		entry.ownership = Borrowed
		bc.scope.borrows = append(bc.scope.borrows, borrowRecord{name: name, prev: prev, declScope: entry.declScope})
	}
}

func (bc *BorrowChecker) checkMove(n *parser.MoveExpr) {
	ident, ok := n.Operand.(*parser.Identifier)
	if !ok {
		bc.checkNode(n.Operand)
		return
	}
	name := ident.Name
	entry := bc.lookup(name)
	if entry == nil {
		return
	}
	if entry.ownership == Moved {
		bc.errors = append(bc.errors, BorrowError{
			Message: "cannot move '" + name + "' — already moved",
		})
		return
	}
	if entry.ownership == Borrowed || entry.ownership == BorrowedMut {
		bc.errors = append(bc.errors, BorrowError{
			Message: "cannot move '" + name + "' — value is borrowed",
		})
		return
	}
	entry.ownership = Moved
	// v1.2.23 Phase 1 (linear types): a move consumes the binding.
	entry.uses++
}

// checkReturnLifetime (Phase 1.2, v1.3.0) walks a return value and rejects
// references that would outlive the data they point at. A borrow (&T / &mut T)
// or address-of (&x) returned from a function must reference a value that
// outlives the function's local scope: a parameter or an escaping (heap-backed)
// binding. A reference to a plain local is dangling and is rejected.
//
// This is a conservative, lexical check. It does not track borrows stored in
// structs or passed through intermediate bindings; those are future work.
func (bc *BorrowChecker) checkReturnLifetime(expr parser.Node, line uint16) {
	if expr == nil {
		return
	}
	switch n := expr.(type) {
	case *parser.BorrowExpr:
		bc.checkBorrowLifetime(n.Operand, n.Mutable, line)
	case *parser.AddressOf:
		bc.checkBorrowLifetime(n.Operand, false, line)
	case *parser.Identifier:
		// Phase 1.2 (v1.3.0): if this identifier is a reference binding
		// (initialized from &x or &mut x), check that the owner outlives
		// the function.
		if entry := bc.lookup(n.Name); entry != nil && entry.borrowsFrom != "" {
			ownerEntry := bc.lookup(entry.borrowsFrom)
			if ownerEntry != nil && !ownerEntry.isParam && !ownerEntry.escapes {
				bc.errors = append(bc.errors, BorrowError{
					Message: "cannot return reference '" + n.Name +
						"' — it points at local variable '" +
						entry.borrowsFrom + "' which does not outlive the function",
					Line: line,
				})
			}
		}
	case *parser.BinaryExpr:
		bc.checkReturnLifetime(n.Left, line)
		bc.checkReturnLifetime(n.Right, line)
	case *parser.UnaryExpr:
		bc.checkReturnLifetime(n.Operand, line)
	case *parser.IndexExpr:
		bc.checkReturnLifetime(n.Left, line)
		bc.checkReturnLifetime(n.Index, line)
	case *parser.CallExpr:
		for _, arg := range n.Args {
			bc.checkReturnLifetime(arg, line)
		}
	case *parser.IndirectCallExpr:
		bc.checkReturnLifetime(n.Target, line)
		for _, arg := range n.Args {
			bc.checkReturnLifetime(arg, line)
		}
	case *parser.StructLiteral:
		for _, field := range n.Fields {
			bc.checkReturnLifetime(field, line)
		}
	case *parser.ArrayLiteral:
		for _, elem := range n.Elements {
			bc.checkReturnLifetime(elem, line)
		}
	}
}

// checkBorrowLifetime checks whether a single borrow/address-of operand
// references a value that will outlive the current function. If the operand is
// an identifier naming a non-parameter, non-escaping local, it is dangling.
func (bc *BorrowChecker) checkBorrowLifetime(operand parser.Node, mutable bool, line uint16) {
	ident, ok := operand.(*parser.Identifier)
	if !ok {
		bc.checkNode(operand)
		return
	}
	name := ident.Name
	entry := bc.lookup(name)
	if entry == nil {
		return
	}
	if entry.isParam {
		return // parameters outlive the function
	}
	if entry.escapes {
		return // escaping (heap-backed) values outlive the scope
	}
	// Non-parameter, non-escaping local: dangling reference.
	bc.errors = append(bc.errors, BorrowError{
		Message: "cannot return reference to local variable '" + name +
			"' — the reference would outlive the data it points at",
		Line:    line,
	})
}

// checkExhaustiveMatch verifies that match expressions cover all variants
// for Option<T> (Some/None) and Result<T,E> (Ok/Err) types
func (bc *BorrowChecker) checkExhaustiveMatch(node *parser.MatchExpr) {
	valueType := bc.inferMatchType(node.Value)
	if valueType == "" {
		return
	}
	covered := make(map[string]bool)
	for _, arm := range node.Arms {
		covered[arm.Pattern.Type] = true
	}
	switch valueType {
	case "Option":
		if !covered["Some"] || !covered["None"] {
			missing := ""
			if !covered["Some"] {
				missing += "Some "
			}
			if !covered["None"] {
				missing += "None"
			}
			bc.errors = append(bc.errors, BorrowError{
				Message: "non-exhaustive match on Option: missing pattern(s): " + missing,
			})
		}
	case "Result":
		if !covered["Ok"] || !covered["Err"] {
			missing := ""
			if !covered["Ok"] {
				missing += "Ok "
			}
			if !covered["Err"] {
				missing += "Err"
			}
			bc.errors = append(bc.errors, BorrowError{
				Message: "non-exhaustive match on Result: missing pattern(s): " + missing,
			})
		}
	}
}

// inferMatchType attempts to determine the type of a matched expression
func (bc *BorrowChecker) inferMatchType(node parser.Node) string {
	switch n := node.(type) {
	case *parser.Identifier:
		entry := bc.lookup(n.Name)
		if entry == nil {
			return ""
		}
		return entry.typeName
	case *parser.OptionSomeExpr:
		return "Option"
	case *parser.OptionNoneExpr:
		return "Option"
	case *parser.ResultOkExpr:
		return "Result"
	case *parser.ResultErrExpr:
		return "Result"
	case *parser.StructLiteral:
		// v1.2.23 Phase 1: a linear struct literal's type is its TypeName,
		// so `let h = Handle{id: 7}` infers typeName "Handle".
		return n.TypeName
	case *parser.CallExpr:
		// Could be a function returning Option/Result
	case *parser.IndirectCallExpr:
		// Phase 133: same — a computed call may return Option/Result.
	}
	return ""
}
