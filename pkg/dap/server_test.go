package dap

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
)

func frame(t *testing.T, m Message) string {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(b), b)
}

func readFrame(t *testing.T, r *bufio.Reader) Message {
	t.Helper()
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			length, _ = strconv.Atoi(strings.TrimPrefix(line, "Content-Length: "))
		}
	}
	if length < 0 {
		t.Fatal("missing Content-Length in response")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		t.Fatal(err)
	}
	var m Message
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDAP_ServerLifecycle(t *testing.T) {
	in := frame(t, Message{Seq: 1, Type: "request", Command: "initialize"}) +
		frame(t, Message{Seq: 2, Type: "request", Command: "disconnect"})
	var out bytes.Buffer
	s := New(strings.NewReader(in), &out)
	if err := s.Serve(); err != nil {
		t.Fatalf("Serve failed: %v", err)
	}
	r := bufio.NewReader(&out)
	initResp := readFrame(t, r)
	if initResp.Type != "response" || initResp.RequestSeq != 1 || !initResp.Success || initResp.Command != "initialize" {
		t.Fatalf("bad initialize response: %+v", initResp)
	}
	var caps map[string]bool
	if err := json.Unmarshal(initResp.Body, &caps); err != nil {
		t.Fatal(err)
	}
	if !caps["supportsConfigurationDoneRequest"] || caps["supportsStepBack"] {
		t.Fatalf("unexpected capabilities: %v", caps)
	}
	ev := readFrame(t, r)
	if ev.Type != "event" || ev.Event != "initialized" {
		t.Fatalf("expected initialized event, got %+v", ev)
	}
	disc := readFrame(t, r)
	if disc.Type != "response" || disc.RequestSeq != 2 || !disc.Success || disc.Command != "disconnect" {
		t.Fatalf("bad disconnect response: %+v", disc)
	}
}

func TestDAP_UnimplementedCommandsAreExplicit(t *testing.T) {
	in := frame(t, Message{Seq: 7, Type: "request", Command: "launch"})
	var out bytes.Buffer
	if err := New(strings.NewReader(in), &out).Serve(); err != nil {
		t.Fatal(err)
	}
	resp := readFrame(t, bufio.NewReader(&out))
	if resp.Success || !strings.Contains(resp.Message, "not implemented") {
		t.Fatalf("launch must not claim success: %+v", resp)
	}
}

func TestDAP_UnknownCommandAndBadFraming(t *testing.T) {
	in := frame(t, Message{Seq: 3, Type: "request", Command: "bogus"})
	var out bytes.Buffer
	if err := New(strings.NewReader(in), &out).Serve(); err != nil {
		t.Fatal(err)
	}
	resp := readFrame(t, bufio.NewReader(&out))
	if resp.Success || !strings.Contains(resp.Message, "unknown DAP command") {
		t.Fatalf("unknown command not rejected: %+v", resp)
	}

	bad := "Content-Length: nope\r\n\r\n{}"
	if err := New(strings.NewReader(bad), &bytes.Buffer{}).Serve(); err == nil {
		t.Fatal("malformed Content-Length was accepted")
	}
}
