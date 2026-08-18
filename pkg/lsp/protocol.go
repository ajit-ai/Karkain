package lsp

// ============================================================
// Phase 39: JSON-RPC 2.0 & LSP 3.17 Protocol Types
// ============================================================

// ------------------------------------------------------------
// JSON-RPC 2.0 Base
// ------------------------------------------------------------

// JSONRPCRequest is a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError is a JSON-RPC 2.0 error object
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSONRPCNotification is a JSON-RPC 2.0 notification (no id)
type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Standard JSON-RPC error codes
const (
	ErrParseError     = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternalError  = -32603
)

// ------------------------------------------------------------
// LSP 3.17 — Initialize
// ------------------------------------------------------------

// InitializeParams from client
type InitializeParams struct {
	ProcessID             int                `json:"processId,omitempty"`
	RootURI               string             `json:"rootUri,omitempty"`
	Capabilities          ClientCapabilities `json:"capabilities"`
	InitializationOptions interface{}        `json:"initializationOptions,omitempty"`
}

// InitializeResult sent to client
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo,omitempty"`
}

// ServerInfo identifies the server
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerCapabilities declares what the server can do
type ServerCapabilities struct {
	TextDocumentSync           *TextDocumentSyncOptions  `json:"textDocumentSync,omitempty"`
	CompletionProvider         *CompletionOptions        `json:"completionProvider,omitempty"`
	HoverProvider              bool                      `json:"hoverProvider,omitempty"`
	DefinitionProvider         bool                      `json:"definitionProvider,omitempty"`
	DocumentSymbolProvider     bool                      `json:"documentSymbolProvider,omitempty"`
	DiagnosticProvider         interface{}               `json:"diagnosticProvider,omitempty"`
}

// TextDocumentSyncOptions
type TextDocumentSyncOptions struct {
	OpenClose bool                   `json:"openClose,omitempty"`
	Change    int                    `json:"change,omitempty"`
	Save      *SaveOptions           `json:"save,omitempty"`
}

// SaveOptions
type SaveOptions struct {
	IncludeText bool `json:"includeText,omitempty"`
}

// CompletionOptions
type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// ClientCapabilities (minimal)
type ClientCapabilities struct {
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
}

// TextDocumentClientCapabilities (minimal)
type TextDocumentClientCapabilities struct {
	Completion   interface{} `json:"completion,omitempty"`
	Hover        interface{} `json:"hover,omitempty"`
	Definition   interface{} `json:"definition,omitempty"`
	Symbol       interface{} `json:"symbol,omitempty"`
	Diagnostics  interface{} `json:"diagnostic,omitempty"`
}

// ------------------------------------------------------------
// LSP 3.17 — TextDocument Sync
// ------------------------------------------------------------

// TextDocumentItem represents an opened text document
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// VersionedTextDocumentIdentifier
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// TextDocumentIdentifier
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// DidOpenTextDocumentParams
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DidChangeTextDocumentParams
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// TextDocumentContentChangeEvent
type TextDocumentContentChangeEvent struct {
	Range *Range `json:"range,omitempty"`
	Text  string `json:"text"`
}

// DidSaveTextDocumentParams
type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Text         string                 `json:"text,omitempty"`
}

// DidCloseTextDocumentParams
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// ------------------------------------------------------------
// LSP 3.17 — Position / Range / Location
// ------------------------------------------------------------

// Position in a text document (0-based)
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range in a text document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location links a range to a file
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// ------------------------------------------------------------
// LSP 3.17 — Diagnostics
// ------------------------------------------------------------

// PublishDiagnosticsParams sent to client
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Version     int          `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// DiagnosticSeverity levels
const (
	DiagError   = 1
	DiagWarning = 2
	DiagInfo    = 3
	DiagHint    = 4
)

// Diagnostic
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

// ------------------------------------------------------------
// LSP 3.17 — Completion
// ------------------------------------------------------------

// CompletionParams
type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      *CompletionContext     `json:"context,omitempty"`
}

// CompletionContext
type CompletionContext struct {
	TriggerKind      int    `json:"triggerKind"`
	TriggerCharacter string `json:"triggerCharacter,omitempty"`
}

// CompletionItemKind values
const (
	CompletionKindText          = 1
	CompletionKindMethod        = 2
	CompletionKindFunction      = 3
	CompletionKindConstructor   = 4
	CompletionKindField         = 5
	CompletionKindVariable      = 6
	CompletionKindClass         = 7
	CompletionKindInterface     = 8
	CompletionKindModule        = 9
	CompletionKindProperty      = 10
	CompletionKindUnit          = 11
	CompletionKindValue         = 12
	CompletionKindEnum          = 13
	CompletionKindKeyword       = 14
	CompletionKindSnippet       = 15
	CompletionKindColor         = 16
	CompletionKindFile          = 17
	CompletionKindReference     = 18
	CompletionKindStruct        = 22
	CompletionKindEvent         = 23
	CompletionKindOperator      = 24
	CompletionKindTypeParameter = 25
)

// CompletionItem
type CompletionItem struct {
	Label               string          `json:"label"`
	Kind                int             `json:"kind,omitempty"`
	Detail              string          `json:"detail,omitempty"`
	Documentation       string          `json:"documentation,omitempty"`
	InsertText          string          `json:"insertText,omitempty"`
	InsertTextFormat    int             `json:"insertTextFormat,omitempty"`
	TextEdit            *TextEdit       `json:"textEdit,omitempty"`
	AdditionalTextEdits []TextEdit      `json:"additionalTextEdits,omitempty"`
}

// TextEdit
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// CompletionList
type CompletionList struct {
	IsIncomplete bool           `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// InsertTextFormat values
const (
	InsertTextPlaintext = 1
	InsertTextSnippet   = 2
)

// CompletionTriggerKind
const (
	TriggerKindInvoked          = 1
	TriggerKindTriggerCharacter = 2
	TriggerKindIncomplete       = 3
)

// ------------------------------------------------------------
// LSP 3.17 — Hover
// ------------------------------------------------------------

// HoverParams
type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// HoverResult
type HoverResult struct {
	Contents MarkupContent `json:"contents"`
	Range    Range         `json:"range,omitempty"`
}

// MarkupKind values
const (
	MarkupPlainText = "plaintext"
	MarkupMarkdown  = "markdown"
)

// MarkupContent
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// ------------------------------------------------------------
// LSP 3.17 — Go-to-Definition
// ------------------------------------------------------------

// DefinitionParams
type DefinitionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// ------------------------------------------------------------
// LSP 3.17 — Document Symbols
// ------------------------------------------------------------

// DocumentSymbolParams
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// SymbolKind values
const (
	SymbolKindFile          = 1
	SymbolKindModule        = 2
	SymbolKindNamespace     = 3
	SymbolKindPackage       = 4
	SymbolKindClass         = 5
	SymbolKindMethod        = 6
	SymbolKindProperty      = 7
	SymbolKindField         = 8
	SymbolKindConstructor   = 9
	SymbolKindEnum          = 10
	SymbolKindInterface     = 11
	SymbolKindFunction      = 12
	SymbolKindVariable      = 13
	SymbolKindConstant      = 14
	SymbolKindString        = 15
	SymbolKindNumber        = 16
	SymbolKindBoolean       = 17
	SymbolKindArray         = 18
	SymbolKindObject        = 19
	SymbolKindKey           = 20
	SymbolKindNull          = 21
	SymbolKindEnumMember    = 22
	SymbolKindStruct        = 23
	SymbolKindEvent         = 24
	SymbolKindOperator      = 25
	SymbolKindTypeParameter = 26
)

// DocumentSymbol
type DocumentSymbol struct {
	Name           string            `json:"name"`
	Kind           int               `json:"kind"`
	Range          Range             `json:"range"`
	SelectionRange Range             `json:"selectionRange"`
	Children       []DocumentSymbol  `json:"children,omitempty"`
	Detail         string            `json:"detail,omitempty"`
}

// ------------------------------------------------------------
// LSP Method Names
// ------------------------------------------------------------
const (
	MethodInitialize                = "initialize"
	MethodInitialized              = "initialized"
	MethodShutdown                 = "shutdown"
	MethodExit                     = "exit"
	MethodTextDocumentDidOpen      = "textDocument/didOpen"
	MethodTextDocumentDidChange    = "textDocument/didChange"
	MethodTextDocumentDidSave      = "textDocument/didSave"
	MethodTextDocumentDidClose     = "textDocument/didClose"
	MethodTextDocumentCompletion   = "textDocument/completion"
	MethodTextDocumentHover        = "textDocument/hover"
	MethodTextDocumentDefinition   = "textDocument/definition"
	MethodTextDocumentDocumentSym  = "textDocument/documentSymbol"
	MethodTextDocumentPublishDiag  = "textDocument/publishDiagnostics"
)
