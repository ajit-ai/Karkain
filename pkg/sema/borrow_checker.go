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
	// Propagate usage counts to parent for linear type checking
	if parent != nil {
		for name, count := range bc.scope.usage {
			parent.usage[name] += count
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

	// Third pass: verify linear types were used exactly once
	bc.checkLinearTypes()

	bc.popScope()
	return bc.errors
}

func (bc *BorrowChecker) checkLinearTypes() {
	if bc.scope == nil {
		return
	}
	for name, count := range bc.scope.usage {
		entry := bc.scope.lookupOwn(name)
		if entry == nil {
			continue
		}
		typeName := entry.typeName
		if typeName == "" {
			continue
		}
		if !bc.linearTypes[typeName] {
			continue
		}
		if count == 0 {
			bc.errors = append(bc.errors, BorrowError{
				Message: "linear type variable '" + name + "' of type '" + typeName + "' was never used",
				Line:    entry.line,
			})
		} else if count > 1 {
			bc.errors = append(bc.errors, BorrowError{
				Message: "linear type variable '" + name + "' of type '" + typeName + "' was used " + itoa(count) + " times (must be exactly 1)",
				Line:    entry.line,
			})
		}
	}
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
			bc.checkNode(field)
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
		bc.declare(n.VarName, "", 0)
		if n.KeyName != "" {
			bc.declare(n.KeyName, "", 0)
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
			bc.declare(param, typeName, 0)
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
	bc.declare(n.Name, typeName, 0)
	// Phase 49 escape analysis marks n.Escapes; capture it so the borrow
	// checker records whether the value outlives its declaring scope.
	if bc.scope != nil {
		if e := bc.scope.lookupOwn(n.Name); e != nil {
			e.escapes = n.Escapes
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
			bc.declare(arm.Pattern.Binding, "", 0)
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
	case *parser.CallExpr:
		// Could be a function returning Option/Result
	}
	return ""
}
