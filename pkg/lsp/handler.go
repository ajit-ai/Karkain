package lsp

import (
	"encoding/json"
	"fmt"
	"strings"

	"karkain/pkg/cli"
	"karkain/pkg/diagnostics"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// ============================================================
// Phase 39: LSP Language Feature Handlers
// Diagnostics, Completion, Hover, Definition, Document Symbols
// ============================================================

// Handler implements LSP request handlers
type Handler struct {
	server *Server
}

// NewHandler creates a new LSP handler
func NewHandler(s *Server) *Handler {
	return &Handler{server: s}
}

// ------------------------------------------------------------
// Lifecycle
// ------------------------------------------------------------

// HandleInitialize processes the initialize request
func (h *Handler) HandleInitialize(params json.RawMessage) (interface{}, *JSONRPCError) {
	var p InitializeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &JSONRPCError{Code: ErrInvalidParams, Message: "invalid params"}
	}

	h.server.SetRootURI(p.RootURI)

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: &TextDocumentSyncOptions{
				OpenClose: true,
				Change:    1, // Full sync
				Save:      &SaveOptions{IncludeText: true},
			},
			CompletionProvider: &CompletionOptions{
				TriggerCharacters: []string{".", ":"},
			},
			HoverProvider:          true,
			DefinitionProvider:     true,
			DocumentSymbolProvider: true,
		},
		ServerInfo: ServerInfo{
			Name:    "karkain-lsp",
			Version: "0.18.0",
		},
	}
	return result, nil
}

// HandleRequest routes LSP requests
func (h *Handler) HandleRequest(method string, params json.RawMessage) (interface{}, *JSONRPCError) {
	switch method {
	case MethodTextDocumentCompletion:
		return h.handleCompletion(params)
	case MethodTextDocumentHover:
		return h.handleHover(params)
	case MethodTextDocumentDefinition:
		return h.handleDefinition(params)
	case MethodTextDocumentDocumentSym:
		return h.handleDocumentSymbol(params)
	default:
		return nil, &JSONRPCError{Code: ErrMethodNotFound, Message: "method not found: " + method}
	}
}

// HandleNotification routes LSP notifications
func (h *Handler) HandleNotification(method string, params json.RawMessage) {
	switch method {
	case MethodTextDocumentDidOpen:
		h.handleDidOpen(params)
	case MethodTextDocumentDidChange:
		h.handleDidChange(params)
	case MethodTextDocumentDidSave:
		h.handleDidSave(params)
	case MethodTextDocumentDidClose:
		h.handleDidClose(params)
	}
}

// ------------------------------------------------------------
// Document Sync
// ------------------------------------------------------------

func (h *Handler) handleDidOpen(params json.RawMessage) {
	var p DidOpenTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return
	}
	h.server.OpenDocument(p.TextDocument.URI, p.TextDocument.Text, p.TextDocument.Version)
	h.publishDiagnostics(p.TextDocument.URI)
}

func (h *Handler) handleDidChange(params json.RawMessage) {
	var p DidChangeTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return
	}
	text := ""
	if len(p.ContentChanges) > 0 {
		text = p.ContentChanges[len(p.ContentChanges)-1].Text
	}
	h.server.ChangeDocument(p.TextDocument.URI, text, p.TextDocument.Version)
	h.publishDiagnostics(p.TextDocument.URI)
}

func (h *Handler) handleDidSave(params json.RawMessage) {
	var p DidSaveTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return
	}
	if p.Text != "" {
		h.server.ChangeDocument(p.TextDocument.URI, p.Text, 0)
	}
	h.publishDiagnostics(p.TextDocument.URI)
}

func (h *Handler) handleDidClose(params json.RawMessage) {
	var p DidCloseTextDocumentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return
	}
	h.server.CloseDocument(p.TextDocument.URI)
	h.publishDiagnostics(p.TextDocument.URI)
}

// ------------------------------------------------------------
// Diagnostics
// ------------------------------------------------------------

func (h *Handler) publishDiagnostics(uri string) {
	doc := h.server.GetDocument(uri)
	if doc == nil {
		h.server.SendNotification(MethodTextDocumentPublishDiag, PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: []Diagnostic{},
		})
		return
	}

	diags := h.runDiagnostics(uri, doc.Text)

	h.server.mu.Lock()
	h.server.diagnostics[uri] = diags
	h.server.mu.Unlock()

	h.server.SendNotification(MethodTextDocumentPublishDiag, PublishDiagnosticsParams{
		URI:         uri,
		Version:     doc.Version,
		Diagnostics: diags,
	})
}

// runDiagnostics is a thin adapter over the canonical CLI diagnostic driver
// (cli.AnalyzeSource). LSP and `karkain check` now share one front-end
// pipeline, so editor squiggles and CLI reports can never diverge again.
// Errors and warnings both publish; LSP severities map from the structured
// field (error → red, warning → yellow, anything else → info).
func (h *Handler) runDiagnostics(uri, text string) []Diagnostic {
	errDiags, warnDiags, _ := cli.AnalyzeSource("", text, nil)
	lspDiags := make([]Diagnostic, 0, len(errDiags)+len(warnDiags))
	appendDiags := func(diags []diagnostics.Diagnostic, severity int) {
		for _, d := range diags {
			startLine := d.Line - 1
			if startLine < 0 {
				startLine = 0
			}
			startChar := d.Column - 1
			if startChar < 0 {
				startChar = 0
			}
			endLine := startLine
			endChar := startChar + 1
			if d.EndColumn > d.Column {
				endChar = d.EndColumn - 1
			}
			lspDiags = append(lspDiags, Diagnostic{
				Range: Range{
					Start: Position{Line: startLine, Character: startChar},
					End:   Position{Line: endLine, Character: endChar},
				},
				Severity: severity,
				Source:   "karkain",
				Message:  d.Message,
			})
		}
	}
	appendDiags(errDiags, lspDiagSeverity(diagnostics.SeverityError))
	appendDiags(warnDiags, lspDiagSeverity(diagnostics.SeverityWarning))
	return lspDiags
}

// lspDiagSeverity maps a structured diagnostic severity onto the LSP
// DiagnosticSeverity scale.
func lspDiagSeverity(s diagnostics.Severity) int {
	switch s {
	case diagnostics.SeverityError:
		return DiagError
	case diagnostics.SeverityWarning:
		return DiagWarning
	default:
		return DiagInfo
	}
}

// ------------------------------------------------------------
// Completion
// ------------------------------------------------------------

var karkainKeywords = []CompletionItem{
	{Label: "func", Kind: CompletionKindKeyword, Detail: "Function declaration", Documentation: "Declare a function: func name(params) { body }"},
	{Label: "let", Kind: CompletionKindKeyword, Detail: "Immutable binding", Documentation: "Bind a value to a name: let x = expr"},
	{Label: "var", Kind: CompletionKindKeyword, Detail: "Mutable variable", Documentation: "Declare a mutable variable: var x = expr"},
	{Label: "return", Kind: CompletionKindKeyword, Detail: "Return statement", Documentation: "Return a value from a function"},
	{Label: "if", Kind: CompletionKindKeyword, Detail: "Conditional", Documentation: "If-else conditional: if cond { ... } else { ... }"},
	{Label: "else", Kind: CompletionKindKeyword, Detail: "Else branch", Documentation: "Else branch of an if statement"},
	{Label: "while", Kind: CompletionKindKeyword, Detail: "While loop", Documentation: "While loop: while cond { body }"},
	{Label: "for", Kind: CompletionKindKeyword, Detail: "For loop", Documentation: "C-style for loop: for (init; cond; post) { body }"},
	{Label: "kernel", Kind: CompletionKindKeyword, Detail: "GPU kernel", Documentation: "Declare a GPU compute kernel: kernel name(params) { body }"},
	{Label: "circuit", Kind: CompletionKindKeyword, Detail: "Quantum circuit", Documentation: "Declare a quantum circuit: circuit name(q: Qubit[N]) -> Bit[N] { body }"},
	{Label: "tensor", Kind: CompletionKindKeyword, Detail: "Tensor function", Documentation: "Declare a tensor computation: tensor name(x: Tensor<f32, [shape]>) { body }"},
	{Label: "actor", Kind: CompletionKindKeyword, Detail: "Actor declaration", Documentation: "Declare an actor: actor Name { handler msg_type(param) { ... } }"},
	{Label: "spawn", Kind: CompletionKindKeyword, Detail: "Spawn actor", Documentation: "Spawn a new actor instance"},
	{Label: "send", Kind: CompletionKindKeyword, Detail: "Send message", Documentation: "Send a message to an actor"},
	{Label: "receive", Kind: CompletionKindKeyword, Detail: "Receive message", Documentation: "Receive a message from an actor mailbox"},
	{Label: "struct", Kind: CompletionKindKeyword, Detail: "Struct declaration", Documentation: "Declare a struct type: type Name { field: Type }"},
	{Label: "type", Kind: CompletionKindKeyword, Detail: "Type alias", Documentation: "Declare a type alias"},
	{Label: "import", Kind: CompletionKindKeyword, Detail: "C import block", Documentation: "Import C code: import { ... }"},
	{Label: "alloc", Kind: CompletionKindKeyword, Detail: "Allocate memory", Documentation: "Allocate memory: alloc Type[count]"},
	{Label: "free", Kind: CompletionKindKeyword, Detail: "Free memory", Documentation: "Free allocated memory"},
	{Label: "qreg", Kind: CompletionKindKeyword, Detail: "Qubit register", Documentation: "Allocate a qubit register: qreg name = N"},
	{Label: "gate", Kind: CompletionKindKeyword, Detail: "Quantum gate", Documentation: "Apply a quantum gate: gate H(target)"},
	{Label: "measure", Kind: CompletionKindKeyword, Detail: "Qubit measurement", Documentation: "Measure a qubit: measure qubit"},
	{Label: "global_id", Kind: CompletionKindKeyword, Detail: "GPU thread ID", Documentation: "Get GPU thread index: global_id(dim)"},
	{Label: "barrier", Kind: CompletionKindKeyword, Detail: "GPU barrier", Documentation: "Synchronize GPU threads: barrier()"},
	{Label: "true", Kind: CompletionKindKeyword, Detail: "Boolean true", Documentation: "Boolean literal true"},
	{Label: "false", Kind: CompletionKindKeyword, Detail: "Boolean false", Documentation: "Boolean literal false"},
	{Label: "print", Kind: CompletionKindKeyword, Detail: "Print function", Documentation: "Print a value: print(expr)"},
	{Label: "println", Kind: CompletionKindKeyword, Detail: "Print line", Documentation: "Print a value with newline: println(expr)"},
	{Label: "matrix", Kind: CompletionKindKeyword, Detail: "Matrix type", Documentation: "Matrix allocation: matrix Rows x Cols of Type"},
	{Label: "macro", Kind: CompletionKindKeyword, Detail: "Macro declaration", Documentation: "Declare a compile-time macro"},
	{Label: "comptime", Kind: CompletionKindKeyword, Detail: "Compile-time", Documentation: "Compile-time evaluation block"},
	{Label: "async", Kind: CompletionKindKeyword, Detail: "Async block", Documentation: "Async expression block: async { ... }"},
	{Label: "await", Kind: CompletionKindKeyword, Detail: "Await expression", Documentation: "Wait for async result: await(future)"},
	{Label: "yield", Kind: CompletionKindKeyword, Detail: "Yield expression", Documentation: "Yield a value from a coroutine"},
	{Label: "co", Kind: CompletionKindKeyword, Detail: "Coroutine", Documentation: "Declare a coroutine: co name(params) { body }"},
	{Label: "gospawn", Kind: CompletionKindKeyword, Detail: "Green thread spawn", Documentation: "Spawn a green thread: gospawn(fn(args))"},
	{Label: "select", Kind: CompletionKindKeyword, Detail: "Select statement", Documentation: "Channel multiplexer: select { case v <- ch: ... }"},
	{Label: "chan", Kind: CompletionKindKeyword, Detail: "Channel", Documentation: "Create a channel: chan<T>(buffer_size)"},
	{Label: "trait", Kind: CompletionKindKeyword, Detail: "Trait declaration", Documentation: "Declare a trait: trait Name { method signatures }"},
	{Label: "impl", Kind: CompletionKindKeyword, Detail: "Impl block", Documentation: "Implement a trait: impl Trait for Type { ... }"},
	{Label: "derive", Kind: CompletionKindKeyword, Detail: "Derive macro", Documentation: "Derive a trait implementation"},
	{Label: "address_of", Kind: CompletionKindKeyword, Detail: "Address-of", Documentation: "Get address of a variable: addr(x)"},
	{Label: "dereference", Kind: CompletionKindKeyword, Detail: "Dereference", Documentation: "Dereference a pointer: *(ptr)"},
}

var builtinOps = []CompletionItem{
	{Label: "ops.matmul", Kind: CompletionKindFunction, Detail: "Tensor<T> matmul(Tensor, Tensor)", Documentation: "Matrix multiplication of two tensors"},
	{Label: "ops.relu", Kind: CompletionKindFunction, Detail: "Tensor<T> relu(Tensor)", Documentation: "Rectified linear unit activation"},
	{Label: "ops.softmax", Kind: CompletionKindFunction, Detail: "Tensor<T> softmax(Tensor)", Documentation: "Softmax normalization"},
	{Label: "ops.conv2d", Kind: CompletionKindFunction, Detail: "Tensor<T> conv2d(Tensor, Tensor)", Documentation: "2D convolution operation"},
	{Label: "ops.transpose", Kind: CompletionKindFunction, Detail: "Tensor<T> transpose(Tensor)", Documentation: "Transpose a tensor"},
	{Label: "ops.reshape", Kind: CompletionKindFunction, Detail: "Tensor<T> reshape(Tensor, shape)", Documentation: "Reshape a tensor"},
	{Label: "ops.sigmoid", Kind: CompletionKindFunction, Detail: "Tensor<T> sigmoid(Tensor)", Documentation: "Sigmoid activation function"},
	{Label: "ops.tanh", Kind: CompletionKindFunction, Detail: "Tensor<T> tanh(Tensor)", Documentation: "Tanh activation function"},
	{Label: "ops.gradient", Kind: CompletionKindFunction, Detail: "Tensor<T> gradient(Tensor)", Documentation: "Compute autograd gradient"},
	{Label: "shape_of", Kind: CompletionKindFunction, Detail: "[int] shape_of(Tensor)", Documentation: "Get the shape of a tensor"},
}

var builtinQPU = []CompletionItem{
	{Label: "qpu.h", Kind: CompletionKindFunction, Detail: "void h(Qubit)", Documentation: "Hadamard gate"},
	{Label: "qpu.x", Kind: CompletionKindFunction, Detail: "void x(Qubit)", Documentation: "Pauli-X (NOT) gate"},
	{Label: "qpu.y", Kind: CompletionKindFunction, Detail: "void y(Qubit)", Documentation: "Pauli-Y gate"},
	{Label: "qpu.z", Kind: CompletionKindFunction, Detail: "void z(Qubit)", Documentation: "Pauli-Z gate"},
	{Label: "qpu.rx", Kind: CompletionKindFunction, Detail: "void rx(angle, Qubit)", Documentation: "Rotation-X gate"},
	{Label: "qpu.ry", Kind: CompletionKindFunction, Detail: "void ry(angle, Qubit)", Documentation: "Rotation-Y gate"},
	{Label: "qpu.rz", Kind: CompletionKindFunction, Detail: "void rz(angle, Qubit)", Documentation: "Rotation-Z gate"},
	{Label: "qpu.cx", Kind: CompletionKindFunction, Detail: "void cx(Qubit, Qubit)", Documentation: "Controlled-NOT gate"},
	{Label: "qpu.cz", Kind: CompletionKindFunction, Detail: "void cz(Qubit, Qubit)", Documentation: "Controlled-Z gate"},
	{Label: "qpu.swap", Kind: CompletionKindFunction, Detail: "void swap(Qubit, Qubit)", Documentation: "SWAP gate"},
	{Label: "qpu.measure", Kind: CompletionKindFunction, Detail: "Bit measure(Qubit)", Documentation: "Measure a qubit"},
	{Label: "qpu.reset", Kind: CompletionKindFunction, Detail: "void reset(Qubit)", Documentation: "Reset a qubit"},
}

var builtinTypes = []CompletionItem{
	{Label: "i32", Kind: CompletionKindTypeParameter, Detail: "32-bit integer"},
	{Label: "i64", Kind: CompletionKindTypeParameter, Detail: "64-bit integer"},
	{Label: "u32", Kind: CompletionKindTypeParameter, Detail: "32-bit unsigned integer"},
	{Label: "u64", Kind: CompletionKindTypeParameter, Detail: "64-bit unsigned integer"},
	{Label: "f32", Kind: CompletionKindTypeParameter, Detail: "32-bit float"},
	{Label: "f64", Kind: CompletionKindTypeParameter, Detail: "64-bit float"},
	{Label: "float64", Kind: CompletionKindTypeParameter, Detail: "64-bit float (alias)"},
	{Label: "string", Kind: CompletionKindTypeParameter, Detail: "String type"},
	{Label: "bool", Kind: CompletionKindTypeParameter, Detail: "Boolean type"},
	{Label: "Qubit", Kind: CompletionKindTypeParameter, Detail: "Quantum bit type"},
	{Label: "Bit", Kind: CompletionKindTypeParameter, Detail: "Classical bit type"},
	{Label: "Tensor", Kind: CompletionKindTypeParameter, Detail: "Parametric tensor type"},
}

func (h *Handler) handleCompletion(params json.RawMessage) (interface{}, *JSONRPCError) {
	var p CompletionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &JSONRPCError{Code: ErrInvalidParams, Message: "invalid params"}
	}

	doc := h.server.GetDocument(p.TextDocument.URI)
	if doc == nil {
		return CompletionList{IsIncomplete: false, Items: h.allCompletions(nil)}, nil
	}

	prefix := h.getPrefixAt(doc.Text, p.Position)
	items := h.allCompletions(&prefix)

	// Also add document symbols
	h.server.mu.Lock()
	for _, sym := range h.server.symbolIndex[p.TextDocument.URI] {
		items = append(items, CompletionItem{
			Label:    sym.Name,
			Kind:     sym.Kind,
			Detail:   sym.Detail,
			InsertText: sym.Name,
		})
	}
	h.server.mu.Unlock()

	return CompletionList{IsIncomplete: false, Items: items}, nil
}

func (h *Handler) allCompletions(prefix *string) []CompletionItem {
	var all []CompletionItem
	all = append(all, karkainKeywords...)
	all = append(all, builtinOps...)
	all = append(all, builtinQPU...)
	all = append(all, builtinTypes...)

	if prefix == nil {
		return all
	}

	p := strings.ToLower(*prefix)
	var filtered []CompletionItem
	for _, item := range all {
		if strings.HasPrefix(strings.ToLower(item.Label), p) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (h *Handler) getPrefixAt(text string, pos Position) string {
	lines := strings.Split(text, "\n")
	if pos.Line >= len(lines) {
		return ""
	}
	line := lines[pos.Line]
	if pos.Character > len(line) {
		return ""
	}

	// Walk backwards to find word boundary
	start := pos.Character
	for start > 0 {
		ch := line[start-1]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			start--
		} else {
			break
		}
	}
	return line[start:pos.Character]
}

// ------------------------------------------------------------
// Hover
// ------------------------------------------------------------

var hoverDocs = map[string]string{
	"func":      "```karkain\nfunc name(param: Type) -> ReturnType { ... }\n```\nDeclare a named function with parameters and return type.",
	"let":       "```karkain\nlet name = value\n```\nCreate an immutable value binding.",
	"var":       "```karkain\nvar name = value\n```\nDeclare a mutable variable.",
	"return":    "```karkain\nreturn value\n```\nReturn a value from the current function.",
	"if":        "```karkain\nif condition { ... } else { ... }\n```\nConditional branching.",
	"while":     "```karkain\nwhile condition { body }\n```\nLoop while a condition is true.",
	"for":       "```karkain\nfor (init; condition; post) { body }\n```\nC-style for loop.",
	"kernel":    "```karkain\nkernel name(param: Type) { global_id(0); ... }\n```\nGPU compute kernel function.",
	"circuit":   "```karkain\ncircuit name(q: Qubit[N]) -> Bit[N] { qpu.h(q[0]); ... }\n```\nQuantum circuit declaration.",
	"tensor":    "```karkain\ntensor name(x: Tensor<f32, [B, D]>) -> Tensor<f32, [B, D]> { ... }\n```\nTensor computation block with autograd.",
	"actor":     "```karkain\nactor Name { handler msg_type(param: Type) { ... } }\n```\nActor declaration with message handlers.",
	"struct":    "```karkain\ntype Name {\n  field: Type\n}\n```\nStruct type declaration.",
	"trait":     "```karkain\ntrait Name {\n  fn method(self, arg: Type) -> ReturnType\n}\n```\nTrait (interface) declaration.",
	"impl":      "```karkain\nimpl TraitName for TypeName { ... }\n```\nImplement a trait for a concrete type.",
	"spawn":     "```karkain\nlet ref = spawn ActorName()\n```\nSpawn a new actor instance.",
	"send":      "```karkain\nactor_ref ! message\n```\nSend an asynchronous message to an actor.",
	"receive":   "```karkain\nreceive from actor_ref { ... }\n```\nReceive a message from an actor.",
	"import":    "```karkain\nimport {\n  #include <stdio.h>\n}\n```\nImport C code for native interop.",
	"alloc":     "```karkain\nlet ptr = alloc Type[count]\n```\nAllocate heap memory for an array.",
	"free":      "```karkain\nfree ptr\n```\nFree previously allocated memory.",
	"qreg":      "```karkain\nqreg q = 2\n```\nAllocate a quantum register with N qubits.",
	"gate":      "```karkain\ngate H(q[0])\ngate CNOT(q[0], q[1])\n```\nApply a quantum gate to qubits.",
	"measure":   "```karkain\nlet result = measure q[0]\n```\nMeasure a qubit, collapsing superposition.",
	"matrix":    "```karkain\nmatrix M = 3 x 4 of float64\n```\nAllocate a 2D matrix.",
	"async":     "```karkain\nasync {\n  let result = await future\n}\n```\nAsynchronous computation block.",
	"await":     "```karkain\nlet value = await async_future\n```\nWait for an asynchronous result.",
	"yield":     "```karkain\nyield value\n```\nYield a value from a coroutine/generator.",
	"co":        "```karkain\nco generator() {\n  yield 1\n  yield 2\n}\n```\nDeclare a coroutine (generator or async function).",
	"gospawn":   "```karkain\ngospawn heavy_computation(args)\n```\nSpawn a lightweight green thread.",
	"select":    "```karkain\nselect {\n  case v = <-ch1: handle(v)\n  case ch2 <- val: sent()\n  default: idle()\n}\n```\nMultiplex over multiple channel operations.",
	"chan":      "```karkain\nlet ch = chan<int>(10)\n```\nCreate a typed channel with optional buffer size.",
	"ops.matmul":    "```karkain\nops.matmul(A, B)\n```\nMatrix multiplication of two tensors.",
	"ops.relu":      "```karkain\nops.relu(tensor)\n```\nRectified Linear Unit: max(0, x).",
	"ops.softmax":   "```karkain\nops.softmax(tensor)\n```\nSoftmax normalization across a dimension.",
	"ops.conv2d":    "```karkain\nops.conv2d(input, kernel)\n```\n2D convolution operation.",
	"ops.transpose": "```karkain\nops.transpose(tensor)\n```\nTranspose a tensor.",
	"ops.gradient":  "```karkain\nops.gradient(tensor)\n```\nCompute autograd gradient.",
	"qpu.h":     "```karkain\nqpu.h(q[0])\n```\nHadamard gate — creates superposition.",
	"qpu.x":     "```karkain\nqpu.x(q[0])\n```\nPauli-X gate — quantum NOT.",
	"qpu.cx":    "```karkain\nqpu.cx(q[0], q[1])\n```\nControlled-NOT — entangling gate.",
	"qpu.measure":"```karkain\nqpu.measure(q[0])\n```\nMeasure qubit in computational basis.",
}

func (h *Handler) handleHover(params json.RawMessage) (interface{}, *JSONRPCError) {
	var p HoverParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &JSONRPCError{Code: ErrInvalidParams, Message: "invalid params"}
	}

	doc := h.server.GetDocument(p.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}

	word := h.getWordAt(doc.Text, p.Position)
	if word == "" {
		return nil, nil
	}

	// Check hover docs map
	if docStr, ok := hoverDocs[word]; ok {
		return HoverResult{
			Contents: MarkupContent{Kind: MarkupMarkdown, Value: docStr},
			Range:    h.wordRange(doc.Text, p.Position, word),
		}, nil
	}

	// Check symbol index
	h.server.mu.Lock()
	syms := h.server.symbolIndex[p.TextDocument.URI]
	h.server.mu.Unlock()

	for _, sym := range syms {
		if sym.Name == word {
			detail := sym.Detail
			if detail == "" {
				detail = fmt.Sprintf("Symbol: %s", sym.Name)
			}
			return HoverResult{
				Contents: MarkupContent{Kind: MarkupMarkdown, Value: "```karkain\n" + detail + "\n```"},
				Range:    sym.Range,
			}, nil
		}
	}

	// Check if it's a type name
	if h.isBuiltinType(word) {
		return HoverResult{
			Contents: MarkupContent{
				Kind:  MarkupMarkdown,
				Value: fmt.Sprintf("```karkain\n%s\n```\nBuilt-in type", word),
			},
		}, nil
	}

	return nil, nil
}

func (h *Handler) getWordAt(text string, pos Position) string {
	lines := strings.Split(text, "\n")
	if pos.Line >= len(lines) {
		return ""
	}
	line := lines[pos.Line]
	if pos.Character > len(line) {
		return ""
	}

	start := pos.Character
	end := pos.Character

	// Find word start
	for start > 0 {
		ch := line[start-1]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			start--
		} else {
			break
		}
	}
	// Find word end
	for end < len(line) {
		ch := line[end]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			end++
		} else {
			break
		}
	}

	if start >= end {
		return ""
	}
	return line[start:end]
}

func (h *Handler) wordRange(text string, pos Position, word string) Range {
	return Range{
		Start: Position{Line: pos.Line, Character: pos.Character - len(word)},
		End:   Position{Line: pos.Line, Character: pos.Character},
	}
}

func (h *Handler) isBuiltinType(word string) bool {
	types := []string{"i32", "i64", "u32", "u64", "f32", "f64", "float64", "string", "bool", "Qubit", "Bit", "Tensor"}
	for _, t := range types {
		if t == word {
			return true
		}
	}
	return false
}

// ------------------------------------------------------------
// Go-to-Definition
// ------------------------------------------------------------

func (h *Handler) handleDefinition(params json.RawMessage) (interface{}, *JSONRPCError) {
	var p DefinitionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &JSONRPCError{Code: ErrInvalidParams, Message: "invalid params"}
	}

	doc := h.server.GetDocument(p.TextDocument.URI)
	if doc == nil {
		return nil, nil
	}

	word := h.getWordAt(doc.Text, p.Position)
	if word == "" {
		return nil, nil
	}

	// Search all document symbols for the definition
	h.server.mu.Lock()
	defer h.server.mu.Unlock()

	// Check current document first
	if syms, ok := h.server.symbolIndex[p.TextDocument.URI]; ok {
		for _, sym := range syms {
			if sym.Name == word {
				return Location{
					URI:   p.TextDocument.URI,
					Range: sym.Range,
				}, nil
			}
		}
	}

	// Search all other documents
	for uri, syms := range h.server.symbolIndex {
		for _, sym := range syms {
			if sym.Name == word {
				return Location{
					URI:   uri,
					Range: sym.Range,
				}, nil
			}
		}
	}

	return nil, nil
}

// ------------------------------------------------------------
// Document Symbols
// ------------------------------------------------------------

func (h *Handler) handleDocumentSymbol(params json.RawMessage) (interface{}, *JSONRPCError) {
	var p DocumentSymbolParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &JSONRPCError{Code: ErrInvalidParams, Message: "invalid params"}
	}

	h.server.mu.Lock()
	defer h.server.mu.Unlock()

	syms := h.server.symbolIndex[p.TextDocument.URI]

	// Build parent map for nesting
	childrenMap := make(map[string][]DocumentSymbol)
	var result []DocumentSymbol

	for _, sym := range syms {
		ds := DocumentSymbol{
			Name:           sym.Name,
			Kind:           sym.Kind,
			Range:          sym.Range,
			SelectionRange: sym.Range,
			Detail:         sym.Detail,
		}
		if sym.Parent != "" {
			childrenMap[sym.Parent] = append(childrenMap[sym.Parent], ds)
		} else {
			result = append(result, ds)
		}
	}

	// Attach children to parents
	for i := range result {
		if children, ok := childrenMap[result[i].Name]; ok {
			result[i].Children = children
		}
	}

	if result == nil {
		result = []DocumentSymbol{}
	}
	return result, nil
}

// ------------------------------------------------------------
// Symbol Index Builder
// ------------------------------------------------------------

func (h *Server) parseAndIndex(uri, text string) {
	l := lexer.New(text)
	p := parser.New(l)
	prog := p.ParseProgram()

	h.parseAndIndexWith(uri, text, prog)
}

func (s *Server) parseAndIndexWith(uri, text string, prog *parser.Program) {
	h := s.handler
	symbols := h.extractSymbols(uri, prog)

	s.mu.Lock()
	s.symbolIndex[uri] = symbols
	s.mu.Unlock()
}

func (h *Handler) buildSymbolIndex(uri string, prog *parser.Program) {
	symbols := h.extractSymbols(uri, prog)
	h.server.mu.Lock()
	h.server.symbolIndex[uri] = symbols
	h.server.mu.Unlock()
}

func (h *Handler) extractSymbols(uri string, prog *parser.Program) []SymbolEntry {
	if prog == nil {
		return nil
	}

	var symbols []SymbolEntry

	for _, stmt := range prog.Statements {
		switch n := stmt.(type) {
		case *parser.FuncDecl:
			symbols = append(symbols, SymbolEntry{
				Name:   n.Name,
				Kind:   SymbolKindFunction,
				Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(n.Name)}},
				Detail: formatFuncDecl(n),
				URI:    uri,
			})
		case *parser.ActorDeclStmt:
			symbols = append(symbols, SymbolEntry{
				Name:   n.Name,
				Kind:   SymbolKindClass,
				Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(n.Name)}},
				Detail: fmt.Sprintf("actor %s", n.Name),
				URI:    uri,
			})
			for _, handler := range n.Handlers {
				symbols = append(symbols, SymbolEntry{
					Name:   handler.MessageType,
					Kind:   SymbolKindMethod,
					Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(handler.MessageType)}},
					Detail: fmt.Sprintf("handler %s(%s: %s)", handler.MessageType, handler.ParamName, handler.ParamType),
					Parent: n.Name,
					URI:    uri,
				})
			}
		case *parser.KernelDeclStmt:
			symbols = append(symbols, SymbolEntry{
				Name:   n.Name,
				Kind:   SymbolKindFunction,
				Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(n.Name)}},
				Detail: formatKernelDecl(n),
				URI:    uri,
			})
		case *parser.StructDeclStmt:
			symbols = append(symbols, SymbolEntry{
				Name:   n.Name,
				Kind:   SymbolKindStruct,
				Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(n.Name)}},
				Detail: formatStructDecl(n),
				URI:    uri,
			})
			for _, field := range n.Fields {
				symbols = append(symbols, SymbolEntry{
					Name:   field.Name,
					Kind:   SymbolKindField,
					Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(field.Name)}},
					Detail: fmt.Sprintf("%s: %s", field.Name, field.Type),
					Parent: n.Name,
					URI:    uri,
				})
			}
		case *parser.TraitDeclStmt:
			symbols = append(symbols, SymbolEntry{
				Name:   n.Name,
				Kind:   SymbolKindInterface,
				Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(n.Name)}},
				Detail: fmt.Sprintf("trait %s", n.Name),
				URI:    uri,
			})
		case *parser.ImplDeclStmt:
			symbols = append(symbols, SymbolEntry{
				Name:   fmt.Sprintf("%s for %s", n.TraitName, n.ForType),
				Kind:   SymbolKindClass,
				Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(n.TraitName)}},
				Detail: fmt.Sprintf("impl %s for %s", n.TraitName, n.ForType),
				URI:    uri,
			})
		case *parser.MacroDeclStmt:
			symbols = append(symbols, SymbolEntry{
				Name:   n.Name,
				Kind:   SymbolKindEvent,
				Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(n.Name)}},
				Detail: fmt.Sprintf("macro %s", n.Name),
				URI:    uri,
			})
		case *parser.CircuitDecl:
			symbols = append(symbols, SymbolEntry{
				Name:   n.Name,
				Kind:   SymbolKindClass,
				Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(n.Name)}},
				Detail: formatCircuitDecl(n),
				URI:    uri,
			})
		case *parser.TensorStmt:
			symbols = append(symbols, SymbolEntry{
				Name:   n.Name,
				Kind:   SymbolKindFunction,
				Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(n.Name)}},
				Detail: fmt.Sprintf("tensor %s", n.Name),
				URI:    uri,
			})
		case *parser.CoroutineDecl:
			symbols = append(symbols, SymbolEntry{
				Name:   n.Name,
				Kind:   SymbolKindFunction,
				Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(n.Name)}},
				Detail: formatCoroutineDecl(n),
				URI:    uri,
			})
		case *parser.VarDeclStmt:
			symbols = append(symbols, SymbolEntry{
				Name:   n.Name,
				Kind:   SymbolKindVariable,
				Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(n.Name)}},
				Detail: fmt.Sprintf("var %s", n.Name),
				URI:    uri,
			})
		case *parser.QRegDeclStmt:
			symbols = append(symbols, SymbolEntry{
				Name:   n.Name,
				Kind:   SymbolKindVariable,
				Range:  Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: len(n.Name)}},
				Detail: fmt.Sprintf("qreg %s", n.Name),
				URI:    uri,
			})
		}
	}

	return symbols
}

// ------------------------------------------------------------
// Formatting Helpers
// ------------------------------------------------------------

func formatFuncDecl(f *parser.FuncDecl) string {
	params := strings.Join(f.Params, ", ")
	return fmt.Sprintf("func %s(%s)", f.Name, params)
}

func formatKernelDecl(k *parser.KernelDeclStmt) string {
	var params []string
	for _, p := range k.Params {
		params = append(params, fmt.Sprintf("%s: %s", p.Name, p.Type))
	}
	return fmt.Sprintf("kernel %s(%s)", k.Name, strings.Join(params, ", "))
}

func formatStructDecl(s *parser.StructDeclStmt) string {
	var fields []string
	for _, f := range s.Fields {
		fields = append(fields, fmt.Sprintf("%s: %s", f.Name, f.Type))
	}
	return fmt.Sprintf("type %s { %s }", s.Name, strings.Join(fields, "; "))
}

func formatCircuitDecl(c *parser.CircuitDecl) string {
	var params []string
	for _, p := range c.Params {
		params = append(params, fmt.Sprintf("%s: Qubit[%d]", p.Name, p.Type.Size))
	}
	ret := ""
	if c.ReturnType != nil {
		ret = fmt.Sprintf(" -> Bit[%d]", c.ReturnType.Size)
	}
	return fmt.Sprintf("circuit %s(%s)%s", c.Name, strings.Join(params, ", "), ret)
}

func formatCoroutineDecl(c *parser.CoroutineDecl) string {
	prefix := "co"
	if c.IsAsync {
		prefix = "async co"
	}
	var params []string
	for _, p := range c.Params {
		params = append(params, fmt.Sprintf("%s: %s", p.Name, p.Type))
	}
	return fmt.Sprintf("%s %s(%s)", prefix, c.Name, strings.Join(params, ", "))
}
