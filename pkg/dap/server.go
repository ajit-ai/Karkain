// Package dap implements the Debug Adapter Protocol transport foundation.
//
// This is a FOUNDATION slice: it speaks real DAP framing (Content-Length) and
// a deterministic request/response/event lifecycle over stdio, but it does
// NOT claim to debug a program. Commands that would require a live debuggee
// (launch, attach, setBreakpoints, continue, stepping, stackTrace, variables)
// return explicit "not implemented" error responses rather than pretending
// success. Phase 140's batch gdb path remains the only working debugger.
package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
)

type Message struct {
	Seq       int             `json:"seq"`
	Type      string          `json:"type"`
	Command   string          `json:"command,omitempty"`
	Event     string          `json:"event,omitempty"`
	RequestSeq int            `json:"request_seq,omitempty"`
	Success   bool            `json:"success,omitempty"`
	Body      json.RawMessage `json:"body,omitempty"`
	Message   string          `json:"message,omitempty"`
}

type Server struct {
	r   *bufio.Reader
	w   io.Writer
	mu  sync.Mutex
	seq int
}

func New(r io.Reader, w io.Writer) *Server {
	return &Server{r: bufio.NewReader(r), w: w}
}

func (s *Server) nextSeq() int {
	s.seq++
	return s.seq
}

func (s *Server) send(m Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

// readMessage accepts only DAP's Content-Length framing with a CRLF blank line.
func (s *Server) readMessage() (Message, error) {
	var m Message
	length := -1
	for {
		line, err := s.r.ReadString('\n')
		if err != nil {
			return m, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			n, convErr := strconv.Atoi(strings.TrimPrefix(line, "Content-Length: "))
			if convErr != nil || n < 0 {
				return m, fmt.Errorf("invalid Content-Length header %q", line)
			}
			length = n
		}
	}
	if length < 0 {
		return m, fmt.Errorf("missing Content-Length header")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(s.r, body); err != nil {
		return m, err
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return m, err
	}
	if m.Type != "request" {
		return m, fmt.Errorf("expected request message, got %q", m.Type)
	}
	return m, nil
}

func (s *Server) respond(req Message, success bool, body any, msg string) error {
	var raw json.RawMessage
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		raw = b
	}
	return s.send(Message{
		Seq:        s.nextSeq(),
		Type:       "response",
		RequestSeq: req.Seq,
		Command:    req.Command,
		Success:    success,
		Body:       raw,
		Message:    msg,
	})
}

func (s *Server) handle(req Message) (stop bool, err error) {
	switch req.Command {
	case "initialize":
		caps := map[string]bool{
			"supportsConfigurationDoneRequest": true,
			"supportsFunctionBreakpoints":      false,
			"supportsConditionalBreakpoints":   false,
			"supportsEvaluateForHovers":        false,
			"supportsStepBack":                 false,
			"supportsRestartRequest":           false,
		}
		if err := s.respond(req, true, caps, ""); err != nil {
			return false, err
		}
		// DAP: the adapter announces readiness after the initialize response.
		return false, s.send(Message{Seq: s.nextSeq(), Type: "event", Event: "initialized"})
	case "threads":
		return false, s.respond(req, true, map[string]any{"threads": []any{}}, "")
	case "disconnect":
		err := s.respond(req, true, nil, "")
		return true, err
	case "launch", "attach", "setBreakpoints", "continue", "next", "stepIn",
		"stepOut", "stackTrace", "scopes", "variables", "evaluate":
		return false, s.respond(req, false, nil,
			"DAP "+req.Command+" is not implemented in this foundation slice")
	default:
		return false, s.respond(req, false, nil, "unknown DAP command: "+req.Command)
	}
}

// Serve runs the DAP loop until EOF or a disconnect request. A malformed
// frame is reported to the caller; it does not silently terminate as success.
func (s *Server) Serve() error {
	for {
		req, err := s.readMessage()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		stop, err := s.handle(req)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
}
