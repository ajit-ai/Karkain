// v1.2.27 DAP foundation CLI gate: `karkain dap` must speak real DAP framing
// over stdio. This gate does not claim a working debugger — debuggee commands
// are explicit errors in this slice.
package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

type dapWireMsg struct {
	Seq        int             `json:"seq"`
	Type       string          `json:"type"`
	Command    string          `json:"command"`
	Event      string          `json:"event"`
	RequestSeq int             `json:"request_seq"`
	Success    bool            `json:"success"`
	Body       json.RawMessage `json:"body"`
}

func dapFrame(t *testing.T, seq int, command string) string {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"seq": seq, "type": "request", "command": command})
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(b), b)
}

func dapRead(t *testing.T, r *bufio.Reader) dapWireMsg {
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
		t.Fatal("no Content-Length in adapter response")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		t.Fatal(err)
	}
	var m dapWireMsg
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPhaseV127DAP_StdioLifecycle(t *testing.T) {
	cmd := exec.Command(v1226Prepare(t), "dap")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(stdin, dapFrame(t, 1, "initialize")+dapFrame(t, 2, "disconnect"))
	r := bufio.NewReader(stdout)
	init := dapRead(t, r)
	if init.Type != "response" || init.RequestSeq != 1 || !init.Success || init.Command != "initialize" {
		t.Fatalf("bad initialize response: %+v", init)
	}
	ev := dapRead(t, r)
	if ev.Type != "event" || ev.Event != "initialized" {
		t.Fatalf("expected initialized event, got %+v", ev)
	}
	disc := dapRead(t, r)
	if disc.Type != "response" || disc.RequestSeq != 2 || !disc.Success {
		t.Fatalf("bad disconnect response: %+v", disc)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("dap adapter exited with error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("dap adapter did not exit after disconnect")
	}
}
