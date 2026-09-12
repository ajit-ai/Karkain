package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ============================================================
// Phase 39: LSP Verification Tests
// ============================================================

// testClient is a minimal LSP client for testing
type testClient struct {
	server *Server
	inBuf  *bytes.Buffer
	outBuf *bytes.Buffer
	closed bool
}

func newTestClient() *testClient {
	inBuf := &bytes.Buffer{}
	outBuf := &bytes.Buffer{}
	s := NewServer(inBuf, outBuf, nil)
	tc := &testClient{
		server: s,
		inBuf:  inBuf,
		outBuf: outBuf,
	}
	return tc
}

// sendRequest sends a JSON-RPC request and returns the raw response bytes
func (tc *testClient) sendRequest(id int, method string, params interface{}) *JSONRPCResponse {
	paramsBytes, _ := json.Marshal(params)
	rawParams := json.RawMessage(paramsBytes)
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}
	body, _ := json.Marshal(req)
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), string(body))
	tc.inBuf.WriteString(msg)

	// Reset outBuf so we only read the response to this request
	tc.outBuf.Reset()

	// Read the message through the server
	raw, err := tc.server.readMessage()
	if err != nil {
		t := &testing.T{}
		t.Fatalf("failed to read message: %v", err)
	}
	tc.server.dispatch(raw)

	// Parse the response from outBuf
	return tc.parseResponse()
}

// sendNotification sends a JSON-RPC notification (no response expected)
func (tc *testClient) sendNotification(method string, params interface{}) {
	paramsBytes, _ := json.Marshal(params)
	rawParams := json.RawMessage(paramsBytes)
	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  rawParams,
	}
	body, _ := json.Marshal(notif)
	msg := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), string(body))
	tc.inBuf.WriteString(msg)

	raw, err := tc.server.readMessage()
	if err != nil {
		return
	}
	tc.server.dispatch(raw)
}

// parseResponse reads and parses a JSON-RPC response from outBuf
func (tc *testClient) parseResponse() *JSONRPCResponse {
	data := tc.outBuf.Bytes()
	if len(data) == 0 {
		return nil
	}

	// Find the JSON body after the Content-Length header
	idx := strings.Index(string(data), "\r\n\r\n")
	if idx < 0 {
		return nil
	}
	jsonBody := data[idx+4:]
	var resp JSONRPCResponse
	json.Unmarshal(jsonBody, &resp)
	return &resp
}

// parseNotification reads a JSON-RPC notification from outBuf
func (tc *testClient) parseNotification() *JSONRPCNotification {
	data := tc.outBuf.Bytes()
	if len(data) == 0 {
		return nil
	}
	idx := strings.Index(string(data), "\r\n\r\n")
	if idx < 0 {
		return nil
	}
	jsonBody := data[idx+4:]
	var notif JSONRPCNotification
	json.Unmarshal(jsonBody, &notif)
	return &notif
}

// ============================================================
// TestLSP_InitializeLifecycle
// ============================================================

func TestLSP_InitializeLifecycle(t *testing.T) {
	tc := newTestClient()

	// 1. Send initialize
	resp := tc.sendRequest(1, MethodInitialize, InitializeParams{
		ProcessID: 1234,
		RootURI:   "file:///workspace",
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{},
		},
	})

	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}

	// Parse result
	var initResult InitializeResult
	resultBytes, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultBytes, &initResult)

	if initResult.ServerInfo.Name != "karkain-lsp" {
		t.Errorf("expected server name 'karkain-lsp', got '%s'", initResult.ServerInfo.Name)
	}
	if initResult.Capabilities.TextDocumentSync == nil {
		t.Error("expected textDocumentSync capability")
	}
	if initResult.Capabilities.CompletionProvider == nil {
		t.Error("expected completionProvider capability")
	}
	if !initResult.Capabilities.HoverProvider {
		t.Error("expected hoverProvider capability")
	}
	if !initResult.Capabilities.DefinitionProvider {
		t.Error("expected definitionProvider capability")
	}
	if !initResult.Capabilities.DocumentSymbolProvider {
		t.Error("expected documentSymbolProvider capability")
	}

	// 2. Send initialized notification
	tc.sendNotification(MethodInitialized, map[string]interface{}{})
	if !tc.server.isInitialized() {
		t.Error("server should be initialized after initialized notification")
	}

	// 3. Send shutdown
	resp = tc.sendRequest(2, MethodShutdown, nil)
	if resp == nil {
		t.Fatal("expected response for shutdown, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error on shutdown: %s", resp.Error.Message)
	}

	// Verify shutdown state
	if !tc.server.shutdown {
		t.Error("server should be in shutdown state")
	}

	// 4. Send exit
	tc.sendNotification(MethodExit, nil)
}

// ============================================================
// TestLSP_RealtimeDiagnostics
// ============================================================

func TestLSP_RealtimeDiagnostics(t *testing.T) {
	tc := newTestClient()

	// Initialize first
	tc.sendRequest(1, MethodInitialize, InitializeParams{
		RootURI: "file:///workspace",
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{},
		},
	})
	tc.sendNotification(MethodInitialized, map[string]interface{}{})

	// Open a valid document
	validCode := `func main() {
  let x = 42
  println(x)
}`
	tc.sendNotification(MethodTextDocumentDidOpen, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///workspace/test.kark",
			LanguageID: "karkain",
			Version:    1,
			Text:       validCode,
		},
	})

	// The server should have sent diagnostics (may be empty for valid code)
	diags := tc.server.diagnostics["file:///workspace/test.kark"]
	// Valid code should have no parser errors
	if len(diags) > 0 {
		for _, d := range diags {
			if d.Severity == DiagError {
				t.Errorf("valid code should not have parser errors: %s", d.Message)
			}
		}
	}

	// Now open invalid code
	invalidCode := `func main() {
  let = 42
}`
	tc.sendNotification(MethodTextDocumentDidChange, DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{
			URI:     "file:///workspace/test.kark",
			Version: 2,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: invalidCode},
		},
	})

	// Check diagnostics from server
	diags = tc.server.diagnostics["file:///workspace/test.kark"]
	// The parser should detect some issue with the invalid syntax
	t.Logf("Diagnostics for invalid code: %d", len(diags))
	for _, d := range diags {
		t.Logf("  [%d] line %d:%d - %s", d.Severity, d.Range.Start.Line, d.Range.Start.Character, d.Message)
	}

	// Save document
	tc.sendNotification(MethodTextDocumentDidSave, DidSaveTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/test.kark"},
		Text:         validCode,
	})

	// Close document
	tc.sendNotification(MethodTextDocumentDidClose, DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/test.kark"},
	})
}

// ============================================================
// TestLSP_SharedCanonicalDiagnostics (Phase 83)
// The LSP must publish exactly the diagnostics the CLI's canonical
// pipeline (cli.AnalyzeSource) produces, with 0-based LSP ranges derived
// from the true resolver spans (start..endColumn).
// ============================================================

func TestLSP_SharedCanonicalDiagnostics(t *testing.T) {
	tc := newTestClient()
	tc.sendRequest(1, MethodInitialize, InitializeParams{
		RootURI: "file:///workspace",
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{},
		},
	})
	tc.sendNotification(MethodInitialized, map[string]interface{}{})

	src := "func main() {\n  let result = unknown_name + 5\n}\n"
	tc.sendNotification(MethodTextDocumentDidOpen, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///workspace/diags.kark",
			LanguageID: "karkain",
			Version:    1,
			Text:       src,
		},
	})

	diags := tc.server.diagnostics["file:///workspace/diags.kark"]
	if len(diags) != 1 {
		t.Fatalf("expected 1 canonical diagnostic, got %d (%v)", len(diags), diags)
	}
	d := diags[0]
	// unknown_name is on line 2 (0-based line 1) at byte 15 (0-based char),
	// spanning 12 characters (15..27 0-based) — CLI reports column 16/endColumn 28.
	if d.Range.Start.Line != 1 {
		t.Errorf("expected start line 1, got %d", d.Range.Start.Line)
	}
	if d.Range.Start.Character != 15 {
		t.Errorf("expected start char 15, got %d", d.Range.Start.Character)
	}
	if d.Range.End.Line != 1 {
		t.Errorf("expected end line 1, got %d", d.Range.End.Line)
	}
	if d.Range.End.Character != 27 {
		t.Errorf("expected end char 27, got %d", d.Range.End.Character)
	}
	if d.Severity != DiagError {
		t.Errorf("expected error severity, got %v", d.Severity)
	}
	if !strings.Contains(d.Message, "undefined identifier 'unknown_name'") {
		t.Errorf("expected undefined-identifier message, got: %s", d.Message)
	}
}

// ============================================================
// TestLSP_UndefinedFunctionSpans (Phase 83)
// Statement-position undefined function calls must carry the same true
// span this phase delivers to `karkain check --format=json`.
// ============================================================

func TestLSP_UndefinedFunctionSpans(t *testing.T) {
	tc := newTestClient()
	tc.sendRequest(1, MethodInitialize, InitializeParams{
		RootURI: "file:///workspace",
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{},
		},
	})
	tc.sendNotification(MethodInitialized, map[string]interface{}{})

	src := "func main() {\n  undefined_call(1)\n}\n"
	tc.sendNotification(MethodTextDocumentDidOpen, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///workspace/undef.kark",
			LanguageID: "karkain",
			Version:    1,
			Text:       src,
		},
	})

	diags := tc.server.diagnostics["file:///workspace/undef.kark"]
	found := false
	for _, d := range diags {
		if strings.Contains(d.Message, "undefined function 'undefined_call'") {
			found = true
			// undefined_call at 0-based line 1, char 2, spans 14 chars (2..16).
			if d.Range.Start.Line != 1 || d.Range.Start.Character != 2 {
				t.Errorf("expected range start (1,2), got (%d,%d)", d.Range.Start.Line, d.Range.Start.Character)
			}
			if d.Range.End.Character != 16 {
				t.Errorf("expected range end char 16, got %d", d.Range.End.Character)
			}
		}
	}
	if !found {
		t.Fatalf("expected undefined-function diagnostic, got: %v", diags)
	}
}

// ============================================================
// TestLSP_RealTimeSync (Phase 83)
// didChange must invalidate diagnostics and republish them from the shared
// driver; fixing the buffer clears them again.
// ============================================================

func TestLSP_RealTimeSync(t *testing.T) {
	tc := newTestClient()
	tc.sendRequest(1, MethodInitialize, InitializeParams{
		RootURI: "file:///workspace",
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{},
		},
	})
	tc.sendNotification(MethodInitialized, map[string]interface{}{})

	uri := "file:///workspace/sync.kark"
	tc.sendNotification(MethodTextDocumentDidOpen, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "karkain", Version: 1, Text: "func main() {\n}\n"},
	})
	if diags := tc.server.diagnostics[uri]; len(diags) != 0 {
		t.Fatalf("valid buffer should have no diagnostics, got %d", len(diags))
	}

	invalid := "func main() {\n  let = 42\n}\n"
	tc.sendNotification(MethodTextDocumentDidChange, DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 2},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: invalid},
		},
	})
	if diags := tc.server.diagnostics[uri]; len(diags) == 0 {
		t.Fatal("expected diagnostics after introducing a syntax error")
	} else if diags[0].Source != "karkain" {
		t.Errorf("expected canonical source 'karkain', got %q", diags[0].Source)
	}

	// Fix the buffer: a valid program must clear the diagnostics.
	tc.sendNotification(MethodTextDocumentDidChange, DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 3},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: "func main() {\n  let x = 42\n  println(x)\n}\n"},
		},
	})
	if diags := tc.server.diagnostics[uri]; len(diags) != 0 {
		t.Fatalf("fixed buffer should clear diagnostics, got %d", len(diags))
	}

	// Version must track the latest didChange version.
	doc := tc.server.GetDocument(uri)
	if doc == nil || doc.Version != 3 {
		t.Errorf("expected document version 3, got %v", doc)
	}
}

// ============================================================
// TestLSP_CompletionAndHover
// ============================================================

func TestLSP_CompletionAndHover(t *testing.T) {
	tc := newTestClient()

	// Initialize
	tc.sendRequest(1, MethodInitialize, InitializeParams{
		RootURI: "file:///workspace",
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{},
		},
	})
	tc.sendNotification(MethodInitialized, map[string]interface{}{})

	// Open a document with some code
	code := `func helper() { }
func main() {
  let x = 42
  println(x)
}`
	tc.sendNotification(MethodTextDocumentDidOpen, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///workspace/test.kark",
			LanguageID: "karkain",
			Version:    1,
			Text:       code,
		},
	})

	// Test completion at position 0,0 (should return keywords)
	resp := tc.sendRequest(2, MethodTextDocumentCompletion, CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/test.kark"},
		Position:     Position{Line: 0, Character: 0},
	})

	if resp == nil {
		t.Fatal("expected completion response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("completion error: %s", resp.Error.Message)
	}

	var complList CompletionList
	resultBytes, _ := json.Marshal(resp.Result)
	json.Unmarshal(resultBytes, &complList)

	if len(complList.Items) == 0 {
		t.Error("expected completion items, got none")
	}

	// Verify we have keywords
	hasKeyword := false
	for _, item := range complList.Items {
		if item.Label == "func" || item.Label == "kernel" || item.Label == "actor" {
			hasKeyword = true
			break
		}
	}
	if !hasKeyword {
		t.Error("expected at least one keyword in completions")
	}

	// Verify we have ops.* and qpu.*
	hasOps := false
	hasQpu := false
	for _, item := range complList.Items {
		if strings.HasPrefix(item.Label, "ops.") {
			hasOps = true
		}
		if strings.HasPrefix(item.Label, "qpu.") {
			hasQpu = true
		}
	}
	if !hasOps {
		t.Error("expected ops.* completions")
	}
	if !hasQpu {
		t.Error("expected qpu.* completions")
	}

	// Test hover on "func" keyword at line 0
	resp = tc.sendRequest(3, MethodTextDocumentHover, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/test.kark"},
		Position:     Position{Line: 0, Character: 2},
	})

	if resp == nil {
		t.Fatal("expected hover response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("hover error: %s", resp.Error.Message)
	}

	var hoverResult HoverResult
	resultBytes, _ = json.Marshal(resp.Result)
	json.Unmarshal(resultBytes, &hoverResult)

	if hoverResult.Contents.Value == "" {
		t.Error("expected non-empty hover content")
	}
	if !strings.Contains(hoverResult.Contents.Value, "func") {
		t.Errorf("hover content should mention 'func', got: %s", hoverResult.Contents.Value)
	}

	// Test hover on "func"
	resp = tc.sendRequest(4, MethodTextDocumentHover, HoverParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/test.kark"},
		Position:     Position{Line: 2, Character: 2}, // "func" at col 2
	})

	if resp == nil {
		t.Fatal("expected hover response for func, got nil")
	}

	resultBytes, _ = json.Marshal(resp.Result)
	json.Unmarshal(resultBytes, &hoverResult)

	if hoverResult.Contents.Value == "" {
		t.Error("expected hover content for 'func'")
	}

	// Test dot-trigger completion
	resp = tc.sendRequest(5, MethodTextDocumentCompletion, CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/test.kark"},
		Position:     Position{Line: 3, Character: 12},
	})

	if resp == nil {
		t.Fatal("expected completion response for dot-trigger")
	}
	if resp.Error != nil {
		t.Logf("dot-trigger completion error (expected for no context): %s", resp.Error.Message)
	}
}

// ============================================================
// TestLSP_GoToDefinition
// ============================================================

func TestLSP_GoToDefinition(t *testing.T) {
	tc := newTestClient()

	// Initialize
	tc.sendRequest(1, MethodInitialize, InitializeParams{
		RootURI: "file:///workspace",
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{},
		},
	})
	tc.sendNotification(MethodInitialized, map[string]interface{}{})

	// Open document with function definition
	code := `func helper() {
  println("helping")
}

func main() {
  helper()
}`
	tc.sendNotification(MethodTextDocumentDidOpen, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///workspace/main.kark",
			LanguageID: "karkain",
			Version:    1,
			Text:       code,
		},
	})

	// Go-to-definition on "helper" at line 5, col 2 (the call site)
	resp := tc.sendRequest(2, MethodTextDocumentDefinition, DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/main.kark"},
		Position:     Position{Line: 5, Character: 2},
	})

	if resp == nil {
		t.Fatal("expected definition response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("definition error: %s", resp.Error.Message)
	}

	var loc Location
	resultBytes, _ := json.Marshal(resp.Result)
	err := json.Unmarshal(resultBytes, &loc)
	if err != nil {
		t.Fatalf("failed to unmarshal location: %v", err)
	}

	if loc.URI != "file:///workspace/main.kark" {
		t.Errorf("expected URI 'file:///workspace/main.kark', got '%s'", loc.URI)
	}
	// Definition should map back to the function declaration
	if loc.Range.Start.Line != 0 {
		t.Errorf("expected definition at line 0, got line %d", loc.Range.Start.Line)
	}

	// Test go-to-definition on "func" keyword (no definition expected)
	resp = tc.sendRequest(3, MethodTextDocumentDefinition, DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/main.kark"},
		Position:     Position{Line: 0, Character: 1}, // "func" keyword
	})

	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	// Keywords typically don't have definitions
	t.Logf("definition for keyword: %v", resp.Result)
}

// ============================================================
// TestLSP_DocumentSymbols
// ============================================================

func TestLSP_DocumentSymbols(t *testing.T) {
	tc := newTestClient()

	// Initialize
	tc.sendRequest(1, MethodInitialize, InitializeParams{
		RootURI: "file:///workspace",
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{},
		},
	})
	tc.sendNotification(MethodInitialized, map[string]interface{}{})

	// Open document with multiple symbol types
	code := `func helper() { }

type Point struct {
  x f64
  y f64
}

func compute(a i32) {
  println(a)
}`
	tc.sendNotification(MethodTextDocumentDidOpen, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///workspace/symbols.kark",
			LanguageID: "karkain",
			Version:    1,
			Text:       code,
		},
	})

	// Request document symbols
	resp := tc.sendRequest(2, MethodTextDocumentDocumentSym, DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/symbols.kark"},
	})

	if resp == nil {
		t.Fatal("expected document symbol response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("document symbol error: %s", resp.Error.Message)
	}

	var symbols []DocumentSymbol
	resultBytes, _ := json.Marshal(resp.Result)
	err := json.Unmarshal(resultBytes, &symbols)
	if err != nil {
		t.Fatalf("failed to unmarshal symbols: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("expected at least one document symbol, got none")
	}

	// Verify we found all expected symbols
	foundSymbols := make(map[string]bool)
	for _, sym := range symbols {
		foundSymbols[sym.Name] = true
		t.Logf("Symbol: %s (kind=%d)", sym.Name, sym.Kind)
	}

	expectedSymbols := []struct {
		name string
		kind int
	}{
		{"helper", SymbolKindFunction},
		{"Point", SymbolKindStruct},
		{"compute", SymbolKindFunction},
	}

	for _, exp := range expectedSymbols {
		if !foundSymbols[exp.name] {
			t.Errorf("expected symbol '%s' not found", exp.name)
		}
	}

	// Verify struct has children (fields)
	for _, sym := range symbols {
		if sym.Name == "Point" {
			if len(sym.Children) != 2 {
				t.Errorf("expected Point to have 2 field children, got %d", len(sym.Children))
			}
		}
	}
}

// ============================================================
// TestLSP_WarningsUseWarningSeverity (Phase 83)
// Resolve-clean programs can still carry warnings (unused variables); those
// must publish at LSP warning severity, not error, with the true span.
// ============================================================

func TestLSP_WarningsUseWarningSeverity(t *testing.T) {
	tc := newTestClient()
	tc.sendRequest(1, MethodInitialize, InitializeParams{
		RootURI: "file:///workspace",
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{},
		},
	})
	tc.sendNotification(MethodInitialized, map[string]interface{}{})

	src := "func main() {\n  let count = 10\n  println(\"hi\")\n}\n"
	tc.sendNotification(MethodTextDocumentDidOpen, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///workspace/warn.kark",
			LanguageID: "karkain",
			Version:    1,
			Text:       src,
		},
	})

	diags := tc.server.diagnostics["file:///workspace/warn.kark"]
	if len(diags) != 1 {
		t.Fatalf("expected 1 warning diagnostic, got %d (%v)", len(diags), diags)
	}
	d := diags[0]
	if d.Severity != DiagWarning {
		t.Errorf("expected warning severity, got %v", d.Severity)
	}
	// `count` is on line 2 at byte 6, spanning 6..11 (0-based).
	if d.Range.Start.Line != 1 || d.Range.Start.Character != 6 {
		t.Errorf("warning range start wrong: %+v", d.Range.Start)
	}
	if d.Range.End.Line != 1 || d.Range.End.Character != 11 {
		t.Errorf("warning range end wrong: %+v", d.Range.End)
	}
	if !strings.Contains(d.Message, "unused variable `count`") {
		t.Errorf("expected unused-variable message, got: %s", d.Message)
	}
}

func TestLSP_Formatting(t *testing.T) {
	tc := newTestClient()
	tc.sendRequest(1, MethodInitialize, InitializeParams{
		RootURI: "file:///workspace",
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{},
		},
	})
	tc.sendNotification(MethodInitialized, map[string]interface{}{})

	// Open an unformatted document
	src := "func  add( a,b ){\n  return   a+b\n}\n"
	tc.sendNotification(MethodTextDocumentDidOpen, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///workspace/format.kark",
			LanguageID: "karkain",
			Version:    1,
			Text:       src,
		},
	})

	// Request formatting
	resp := tc.sendRequest(2, MethodTextDocumentFormatting, DocumentFormattingParams{
		TextDocument: struct {
			URI string `json:"uri"`
		}{URI: "file:///workspace/format.kark"},
	})

	if resp.Error != nil {
		t.Fatalf("formatting request failed: %v", resp.Error.Message)
	}

	// Result is []TextEdit marshaled as []interface{}
	editsRaw, ok := resp.Result.([]interface{})
	if !ok {
		t.Fatalf("expected array result, got %T: %v", resp.Result, resp.Result)
	}
	if len(editsRaw) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(editsRaw))
	}

	// Re-marshal and unmarshal to get TextEdit
	editBytes, _ := json.Marshal(editsRaw[0])
	var edit TextEdit
	if err := json.Unmarshal(editBytes, &edit); err != nil {
		t.Fatalf("failed to unmarshal edit: %v", err)
	}

	// Verify the edit replaces the entire document
	if edit.Range.Start.Line != 0 || edit.Range.Start.Character != 0 {
		t.Errorf("edit start wrong: %+v", edit.Range.Start)
	}

	// Verify the formatted text is canonical
	formatted := edit.NewText
	if formatted == src {
		t.Error("expected formatting to change source")
	}
	// Verify no double spaces between tokens
	for i := 0; i < len(formatted)-1; i++ {
		if formatted[i] == ' ' && formatted[i+1] == ' ' {
			// Skip indentation (leading spaces at start of line)
			if i > 0 && formatted[i-1] != '\n' {
				t.Error("formatted text still has double spaces between tokens")
				break
			}
		}
	}
}

func TestLSP_Formatting_AlreadyFormatted(t *testing.T) {
	tc := newTestClient()
	tc.sendRequest(1, MethodInitialize, InitializeParams{
		RootURI: "file:///workspace",
		Capabilities: ClientCapabilities{
			TextDocument: &TextDocumentClientCapabilities{},
		},
	})
	tc.sendNotification(MethodInitialized, map[string]interface{}{})

	// Open a properly formatted document
	src := "func add(a, b) {\n    return a + b\n}\n"
	tc.sendNotification(MethodTextDocumentDidOpen, DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        "file:///workspace/formatted.kark",
			LanguageID: "karkain",
			Version:    1,
			Text:       src,
		},
	})

	// Request formatting
	resp := tc.sendRequest(2, MethodTextDocumentFormatting, DocumentFormattingParams{
		TextDocument: struct {
			URI string `json:"uri"`
		}{URI: "file:///workspace/formatted.kark"},
	})

	if resp.Error != nil {
		t.Fatalf("formatting request failed: %v", resp.Error.Message)
	}

	// Result should be an empty array
	editsRaw, ok := resp.Result.([]interface{})
	if !ok {
		t.Fatalf("expected array result, got %T: %v", resp.Result, resp.Result)
	}
	if len(editsRaw) != 0 {
		t.Errorf("expected 0 edits for already-formatted document, got %d", len(editsRaw))
	}
}
