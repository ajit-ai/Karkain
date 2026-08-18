package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
)

// ============================================================
// Phase 39: LSP Server Lifecycle
// JSON-RPC 2.0 over stdio / TCP
// ============================================================

// Server is the Karkain Language Server
type Server struct {
	mu sync.Mutex

	// Transport
	reader *bufio.Reader
	writer io.Writer
	closer io.Closer

	// State
	initialized bool
	shutdown    bool
	rootURI     string

	// Document store
	documents map[string]*DocumentState

	// Symbol index (rebuilt on parse)
	symbolIndex map[string][]SymbolEntry

	// Diagnostics cache
	diagnostics map[string][]Diagnostic

	// Handler
	handler *Handler
}

// DocumentState holds the current state of an open document
type DocumentState struct {
	URI     string
	Version int
	Text    string
}

// SymbolEntry is a symbol in the index
type SymbolEntry struct {
	Name     string
	Kind     int
	Range    Range
	Detail   string
	Parent   string
	URI      string
}

// NewServer creates a new LSP server reading/writing to the given streams
func NewServer(reader io.Reader, writer io.Writer, closer io.Closer) *Server {
	s := &Server{
		reader:      bufio.NewReader(reader),
		writer:      writer,
		closer:      closer,
		documents:   make(map[string]*DocumentState),
		symbolIndex: make(map[string][]SymbolEntry),
		diagnostics: make(map[string][]Diagnostic),
	}
	s.handler = NewHandler(s)
	return s
}

// NewTCPServer creates a server listening on TCP
func NewTCPServer(addr string) (*Server, net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	conn, err := ln.Accept()
	if err != nil {
		ln.Close()
		return nil, nil, err
	}
	s := NewServer(conn, conn, conn)
	return s, ln, nil
}

// Run enters the main message loop. Blocks until shutdown or exit.
func (s *Server) Run() error {
	for {
		msg, err := s.readMessage()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if msg == nil {
			continue
		}

		s.dispatch(msg)
	}
}

// dispatch routes a JSON-RPC message to the appropriate handler
func (s *Server) dispatch(msg *rawMessage) {
	if msg.ID != nil {
		// Request
		s.handleRequest(msg)
	} else {
		// Notification
		s.handleNotification(msg)
	}
}

// handleRequest processes a JSON-RPC request and sends a response
func (s *Server) handleRequest(msg *rawMessage) {
	var result interface{}
	var jsonErr *JSONRPCError

	switch msg.Method {
	case MethodInitialize:
		result, jsonErr = s.handler.HandleInitialize(msg.Params)
	case MethodShutdown:
		s.mu.Lock()
		s.shutdown = true
		s.mu.Unlock()
		result = nil
	default:
		if !s.isInitialized() {
			jsonErr = &JSONRPCError{Code: ErrInvalidRequest, Message: "server not initialized"}
		} else {
			result, jsonErr = s.handler.HandleRequest(msg.Method, msg.Params)
		}
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      msg.ID,
		Result:  result,
		Error:   jsonErr,
	}
	s.sendResponse(resp)
}

// handleNotification processes a JSON-RPC notification
func (s *Server) handleNotification(msg *rawMessage) {
	switch msg.Method {
	case MethodInitialized:
		s.mu.Lock()
		s.initialized = true
		s.mu.Unlock()
	case MethodExit:
		if s.closer != nil {
			s.closer.Close()
		}
		return
	default:
		if !s.isInitialized() {
			return
		}
		s.handler.HandleNotification(msg.Method, msg.Params)
	}
}

func (s *Server) isInitialized() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.initialized
}

// ------------------------------------------------------------
// JSON-RPC Message Reading (Content-Length framing)
// ------------------------------------------------------------

// rawMessage is the intermediate parsed form
type rawMessage struct {
	ID     interface{}     `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func (s *Server) readMessage() (*rawMessage, error) {
	// Read Content-Length header
	contentLength := -1
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			val := strings.TrimPrefix(line, "Content-Length: ")
			contentLength, _ = strconv.Atoi(val)
		}
	}

	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(s.reader, body)
	if err != nil {
		return nil, err
	}

	var msg rawMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ------------------------------------------------------------
// JSON-RPC Response Writing (Content-Length framing)
// ------------------------------------------------------------

func (s *Server) sendResponse(resp JSONRPCResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	body, err := json.Marshal(resp)
	if err != nil {
		return
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	s.writer.Write([]byte(header))
	s.writer.Write(body)
}

// SendNotification sends a JSON-RPC notification to the client
func (s *Server) SendNotification(method string, params interface{}) {
	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(notif)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	s.writer.Write([]byte(header))
	s.writer.Write(body)
}

// ------------------------------------------------------------
// Document Store
// ------------------------------------------------------------

// OpenDocument adds or updates a document in the store
func (s *Server) OpenDocument(uri, text string, version int) {
	s.mu.Lock()
	s.documents[uri] = &DocumentState{
		URI:     uri,
		Version: version,
		Text:    text,
	}
	s.mu.Unlock()
	s.parseAndIndex(uri, text)
}

// ChangeDocument applies an incremental change
func (s *Server) ChangeDocument(uri, text string, version int) {
	s.mu.Lock()
	doc, ok := s.documents[uri]
	if !ok {
		s.documents[uri] = &DocumentState{URI: uri, Version: version, Text: text}
		s.mu.Unlock()
		s.parseAndIndex(uri, text)
		return
	}
	doc.Version = version
	doc.Text = text
	s.mu.Unlock()
	s.parseAndIndex(uri, text)
}

// CloseDocument removes a document from the store
func (s *Server) CloseDocument(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.documents, uri)
	delete(s.symbolIndex, uri)
	delete(s.diagnostics, uri)
}

// GetDocument returns the current state of a document
func (s *Server) GetDocument(uri string) *DocumentState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.documents[uri]
}

// GetRootURI returns the workspace root URI
func (s *Server) GetRootURI() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rootURI
}

// SetRootURI sets the workspace root
func (s *Server) SetRootURI(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rootURI = uri
}

// ------------------------------------------------------------
// Close
// ------------------------------------------------------------

// Close shuts down the server and releases resources
func (s *Server) Close() error {
	if s.closer != nil {
		return s.closer.Close()
	}
	return nil
}
