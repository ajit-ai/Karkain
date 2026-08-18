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
	server   *Server
	inBuf    *bytes.Buffer
	outBuf   *bytes.Buffer
	closed   bool
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
			URI:        "file:///workspace/test.kar",
			LanguageID: "karkain",
			Version:    1,
			Text:       validCode,
		},
	})

	// The server should have sent diagnostics (may be empty for valid code)
	diags := tc.server.diagnostics["file:///workspace/test.kar"]
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
			URI:     "file:///workspace/test.kar",
			Version: 2,
		},
		ContentChanges: []TextDocumentContentChangeEvent{
			{Text: invalidCode},
		},
	})

	// Check diagnostics from server
	diags = tc.server.diagnostics["file:///workspace/test.kar"]
	// The parser should detect some issue with the invalid syntax
	t.Logf("Diagnostics for invalid code: %d", len(diags))
	for _, d := range diags {
		t.Logf("  [%d] line %d:%d - %s", d.Severity, d.Range.Start.Line, d.Range.Start.Character, d.Message)
	}

	// Save document
	tc.sendNotification(MethodTextDocumentDidSave, DidSaveTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/test.kar"},
		Text:         validCode,
	})

	// Close document
	tc.sendNotification(MethodTextDocumentDidClose, DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/test.kar"},
	})
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
			URI:        "file:///workspace/test.kar",
			LanguageID: "karkain",
			Version:    1,
			Text:       code,
		},
	})

	// Test completion at position 0,0 (should return keywords)
	resp := tc.sendRequest(2, MethodTextDocumentCompletion, CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/test.kar"},
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
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/test.kar"},
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
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/test.kar"},
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
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/test.kar"},
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
			URI:        "file:///workspace/main.kar",
			LanguageID: "karkain",
			Version:    1,
			Text:       code,
		},
	})

	// Go-to-definition on "helper" at line 5, col 2 (the call site)
	resp := tc.sendRequest(2, MethodTextDocumentDefinition, DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/main.kar"},
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

	if loc.URI != "file:///workspace/main.kar" {
		t.Errorf("expected URI 'file:///workspace/main.kar', got '%s'", loc.URI)
	}
	// Definition should map back to the function declaration
	if loc.Range.Start.Line != 0 {
		t.Errorf("expected definition at line 0, got line %d", loc.Range.Start.Line)
	}

	// Test go-to-definition on "func" keyword (no definition expected)
	resp = tc.sendRequest(3, MethodTextDocumentDefinition, DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/main.kar"},
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
			URI:        "file:///workspace/symbols.kar",
			LanguageID: "karkain",
			Version:    1,
			Text:       code,
		},
	})

	// Request document symbols
	resp := tc.sendRequest(2, MethodTextDocumentDocumentSym, DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///workspace/symbols.kar"},
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
