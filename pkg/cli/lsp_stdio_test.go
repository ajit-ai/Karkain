package cli

// Phase 136, Slice E gate — stdio end-to-end through the real binary.
//
// Spawns `karkain lsp` and speaks JSON-RPC over pipes: initialize (pins the
// advertised capabilities), didOpen (collects the diagnostics push),
// semanticTokens/full, hover, definition, completion, shutdown/exit (clean
// process exit). This proves the whole server path outside unit harnesses.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// stdioClient drives a framed JSON-RPC conversation with a child process.
type stdioClient struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
	nextID int
}

func startLSPServer(t *testing.T, bin string) *stdioClient {
	t.Helper()
	cmd := exec.Command(bin, "lsp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start karkain lsp: %v", err)
	}
	c := &stdioClient{t: t, cmd: cmd, stdin: stdin, stdout: stdout}
	t.Cleanup(func() {
		stdin.Close()
		_ = cmd.Wait()
		if stderr.Len() > 0 {
			t.Logf("server stderr: %s", stderr.String())
		}
	})
	return c
}

func (c *stdioClient) sendFrame(body []byte) {
	c.t.Helper()
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(c.stdin, header); err != nil {
		c.t.Fatalf("write header: %v", err)
	}
	if _, err := c.stdin.Write(body); err != nil {
		c.t.Fatalf("write body: %v", err)
	}
}

// readFrame reads one Content-Length frame with a hard deadline so a hung
// server fails the test instead of the suite.
func (c *stdioClient) readFrame() []byte {
	c.t.Helper()
	type result struct {
		body []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		body, err := readOneFrame(c.stdout)
		ch <- result{body, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			c.t.Fatalf("read frame: %v", r.err)
		}
		return r.body
	case <-time.After(30 * time.Second):
		c.t.Fatal("timed out waiting for server frame")
		return nil
	}
}

func readOneFrame(r io.Reader) ([]byte, error) {
	var length = -1
	buf := make([]byte, 1)
	var line []byte
	for {
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		if buf[0] == '\n' {
			text := strings.TrimSpace(string(line))
			if text == "" {
				break
			}
			if strings.HasPrefix(text, "Content-Length:") {
				n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(text, "Content-Length:")))
				if err != nil {
					return nil, err
				}
				length = n
			}
			line = line[:0]
			continue
		}
		line = append(line, buf[0])
	}
	if length < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(r, body)
	return body, err
}

func (c *stdioClient) request(method string, params interface{}) map[string]interface{} {
	c.t.Helper()
	c.nextID++
	pb, _ := json.Marshal(params)
	msg, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "id": c.nextID, "method": method, "params": json.RawMessage(pb),
	})
	c.sendFrame(msg)
	want := c.nextID
	for {
		var body map[string]interface{}
		if err := json.Unmarshal(c.readFrame(), &body); err != nil {
			c.t.Fatalf("decode frame: %v", err)
		}
		if id, ok := body["id"]; ok && id != nil {
			if int(id.(float64)) == want {
				return body
			}
			continue // stale response (should not happen sequentially)
		}
		// Server notification (e.g. publishDiagnostics): drained, not lost.
		c.t.Logf("drained notification: %v", body["method"])
	}
}

func (c *stdioClient) notify(method string, params interface{}) {
	c.t.Helper()
	pb, _ := json.Marshal(params)
	msg, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0", "method": method, "params": json.RawMessage(pb),
	})
	c.sendFrame(msg)
}

func resultMap(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	if errVal, ok := resp["error"]; ok && errVal != nil {
		t.Fatalf("server error: %v", errVal)
	}
	res, _ := resp["result"].(map[string]interface{})
	if res == nil {
		t.Fatalf("null result where object expected: %v", resp)
	}
	return res
}

func TestLSP_StdioEndToEnd(t *testing.T) {
	bin := buildKarkain(t)
	c := startLSPServer(t, bin)

	// initialize: pin the Phase-136 capability set.
	resp := c.request("initialize", map[string]interface{}{
		"processId": 1, "rootUri": "file:///workspace", "capabilities": map[string]interface{}{},
	})
	caps, _ := resultMap(t, resp)["capabilities"].(map[string]interface{})
	for _, want := range []string{"completionProvider", "hoverProvider", "definitionProvider", "semanticTokensProvider"} {
		if _, ok := caps[want]; !ok {
			t.Fatalf("capability %s missing: %v", want, caps)
		}
	}
	c.notify("initialized", map[string]interface{}{})

	// didOpen a program exercising every Slice A-D surface.
	uri := "file:///workspace/e2e.kark"
	text := "import math\n\nfunc helper() {\n    let answer = math.twice(21)\n    print(answer)\n}\n"
	c.notify("textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri, "languageId": "karkain", "version": 1, "text": text,
		},
	})

	// semanticTokens/full: well-formed delta stream covering the document.
	resp = c.request("textDocument/semanticTokens/full", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
	})
	data, _ := resultMap(t, resp)["data"].([]interface{})
	if len(data) == 0 || len(data)%5 != 0 {
		t.Fatalf("token data malformed: %d units", len(data))
	}

	// hover on `func` keyword: non-empty markdown mentioning func.
	resp = c.request("textDocument/hover", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": 2, "character": 1},
	})
	hover := resultMap(t, resp)
	contents, _ := hover["contents"].(map[string]interface{})
	if v, _ := contents["value"].(string); !strings.Contains(v, "func") {
		t.Fatalf("hover should mention func, got %v", hover)
	}

	// definition on the local `answer` (line 3 `print(answer)`, col 11):
	// jumps to its `let` on line 3... use the call-adjacent line instead:
	// `math` qualifier is unresolved without math.kark open, so assert the
	// local: position line 4 col 11 (`answer` in print) -> line 3.
	resp = c.request("textDocument/definition", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": 4, "character": 11},
	})
	def := resultMap(t, resp)
	rng, _ := def["range"].(map[string]interface{})
	start, _ := rng["start"].(map[string]interface{})
	if int(start["line"].(float64)) != 3 {
		t.Fatalf("definition should land on line 3, got %v", def)
	}

	// completion: non-empty list anywhere.
	resp = c.request("textDocument/completion", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": uri},
		"position":     map[string]interface{}{"line": 4, "character": 4},
	})
	items, _ := resultMap(t, resp)["items"].([]interface{})
	if len(items) == 0 {
		t.Fatal("completion should offer items")
	}

	// Clean shutdown: shutdown response carries a null result (no error),
	// then exit ends the process. The server exits on stdin EOF, so the
	// client closes its end (standard stdio lifecycle) before waiting.
	resp = c.request("shutdown", nil)
	if errVal, ok := resp["error"]; ok && errVal != nil {
		t.Fatalf("shutdown error: %v", errVal)
	}
	c.notify("exit", nil)
	_ = c.stdin.Close()
	done := make(chan error, 1)
	go func() { done <- c.cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server should exit 0, got %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("server did not exit after exit notification")
	}
}
