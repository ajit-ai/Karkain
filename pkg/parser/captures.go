package parser

// Phase 54: free-variable capture analysis for lambdas.
// ComputeCaptures returns the ordered, deduplicated list of identifiers
// referenced inside the lambda body that are neither its parameters nor
// locally declared within it.

type captureSet struct {
	seen  map[string]bool
	order []string
}

func (c *captureSet) add(name string) {
	if !c.seen[name] {
		c.seen[name] = true
		c.order = append(c.order, name)
	}
}

func (c *captureSet) remove(name string) { delete(c.seen, name) }

func (c *captureSet) has(name string) bool { return c.seen[name] }

func ComputeCaptures(params []string, body []Node) []string {
	cs := &captureSet{seen: make(map[string]bool)}
	for _, st := range body {
		walkCaptures(st, cs)
	}
	for _, p := range params {
		cs.remove(p)
	}
	// Locally declared names are not captures.
	for _, st := range body {
		markLocals(st, cs)
	}
	result := []string{}
	for _, name := range cs.order {
		if cs.seen[name] {
			result = append(result, name)
		}
	}
	return result
}

func markLocals(n Node, cs *captureSet) {
	switch node := n.(type) {
	case *VarDeclStmt:
		cs.remove(node.Name)
	case *IfStmt:
		for _, s := range node.Consequence {
			markLocals(s, cs)
		}
		for _, s := range node.Alternative {
			markLocals(s, cs)
		}
	case *WhileStmt:
		for _, s := range node.Body {
			markLocals(s, cs)
		}
	}
}

func walkCaptures(n Node, cs *captureSet) {
	switch node := n.(type) {
	case nil:
		return
	case *Identifier:
		cs.add(node.Name)
	case *ExprStmt:
		walkCaptures(node.Expression, cs)
	case *ReturnStmt:
		walkCaptures(node.Value, cs)
	case *IfStmt:
		walkCaptures(node.Condition, cs)
		for _, s := range node.Consequence {
			walkCaptures(s, cs)
		}
		for _, s := range node.Alternative {
			walkCaptures(s, cs)
		}
	case *WhileStmt:
		walkCaptures(node.Condition, cs)
		for _, s := range node.Body {
			walkCaptures(s, cs)
		}
	case *BinaryExpr:
		walkCaptures(node.Left, cs)
		walkCaptures(node.Right, cs)
	case *UnaryExpr:
		walkCaptures(node.Operand, cs)
	case *CallExpr:
		if node.Function != "" {
			cs.add(node.Function)
		}
		for _, a := range node.Args {
			walkCaptures(a, cs)
		}
	case *IndexExpr:
		walkCaptures(node.Left, cs)
		walkCaptures(node.Index, cs)
	case *LambdaExpr:
		for _, cap := range ComputeCaptures(node.Params, node.Body) {
			cs.add(cap)
		}
	case *VarDeclStmt:
		walkCaptures(node.Name, cs)
		cs.remove(node.Name)
	}
}

