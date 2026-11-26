// v1.2.21 - UDP datagrams and WebSocket text frames on both engines.
//
// All peers are in-process Go listeners on 127.0.0.1. Karkain initializes the
// exchange by calling a udp_*/ws_* builtin, so no readiness sleep is needed.
// Native and WASM remain excluded by existing socket/concurrency boundaries.
package cli

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var v1221Once sync.Once
var v1221Bin string
var v1221Err error

func v1221Prepare(t *testing.T) string {
	t.Helper()
	v1221Once.Do(func() {
		for _, dir := range []string{
			`C:\Program Files\Git\cmd`,
			`C:\Windows\System32\WindowsPowerShell\v1.0`,
			`C:\msys64\mingw64\bin`,
		} {
			if st, err := os.Stat(dir); err == nil && st.IsDir() {
				parts := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
				found := false
				for _, p := range parts {
					if strings.EqualFold(p, dir) {
						found = true
						break
					}
				}
				if !found {
					_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
				}
			}
		}
		v1221Err = buildKarkainErr(t, &v1221Bin)
	})
	if v1221Err != nil {
		t.Fatalf("building karkain: %v", v1221Err)
	}
	return v1221Bin
}

func v1221Source(t *testing.T, name, body string) string {
	t.Helper()
	dir, err := os.MkdirTemp(repoRoot(t), "v1221")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, name+".kark")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func v1221Run(t *testing.T, engine, path string) string {
	t.Helper()
	karkain := v1221Prepare(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "run", path, "--engine", engine)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s timed out: %v\nstdout:\n%s\nstderr:\n%s", engine, ctx.Err(), stdout.String(), stderr.String())
	}
	if err != nil {
		t.Fatalf("%s failed: %v\nstdout:\n%s\nstderr:\n%s", engine, err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func v1221FreeUDPPort(t *testing.T) int {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	_ = pc.Close()
	return port
}

// TestPhaseV1221_UDPLoopback proves a real datagram round trip: Karkain binds
// a UDP socket, sends to a Go echo peer and receives the peer's reply.
func TestPhaseV1221_UDPLoopback(t *testing.T) {
	peer, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	peerPort := peer.LocalAddr().(*net.UDPAddr).Port

	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := peer.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = peer.WriteTo(append([]byte("pong:"), buf[:n]...), addr)
		}
	}()

	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			port := v1221FreeUDPPort(t)
			src := fmt.Sprintf(`import std.net
func main() {
    let fd = udp_bind_on("127.0.0.1", %d)
    print(fd >= 0)
    let sent = udp_send_from(fd, "127.0.0.1", %d, "ping")
    print(sent)
    let reply = udp_recv_from(fd, 1024)
    print(reply)
    print(udp_error())
}
`, port, peerPort)
			path := v1221Source(t, "udp", src)
			out := v1221Run(t, engine, path)
			lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
			if len(lines) < 4 {
				t.Fatalf("expected 4 UDP lines, got:\n%s", out)
			}
			if strings.TrimSpace(lines[0]) != "1" {
				t.Errorf("bind failed:\n%s", out)
			}
			if strings.TrimSpace(lines[1]) != "4" {
				t.Errorf("send count = %q, want 4:\n%s", lines[1], out)
			}
			if strings.TrimSpace(lines[2]) != "pong:ping" {
				t.Errorf("reply = %q:\n%s", lines[2], out)
			}
			if strings.TrimSpace(lines[3]) != "" {
				t.Errorf("udp_error = %q:\n%s", lines[3], out)
			}
		})
	}
}

func v1221WSKey(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := io.ReadFull(bytes.NewReader(b), b); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func v1221StartWSPeer(t *testing.T, key string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	accept := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	wantAccept := base64.StdEncoding.EncodeToString(accept[:])
	errCh := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
		r := bufio.NewReader(conn)
		var req bytes.Buffer
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			req.WriteString(line)
			if strings.HasSuffix(req.String(), "\r\n\r\n") {
				break
			}
		}
		if !strings.Contains(req.String(), wantAccept) && !strings.Contains(req.String(), "Sec-WebSocket-Key: "+key) {
			errCh <- fmt.Errorf("missing websocket key in request:\n%s", req.String())
			return
		}
		resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + wantAccept + "\r\n\r\n"
		if _, err := io.WriteString(conn, resp); err != nil {
			errCh <- err
			return
		}
		head := make([]byte, 2)
		if _, err := io.ReadFull(r, head); err != nil {
			errCh <- err
			return
		}
		n := int(head[1] & 0x7f)
		mask := make([]byte, 4)
		if _, err := io.ReadFull(r, mask); err != nil {
			errCh <- err
			return
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(r, payload); err != nil {
			errCh <- err
			return
		}
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
		if string(payload) != "hello" {
			errCh <- fmt.Errorf("client frame = %q, want hello", payload)
			return
		}
		body := []byte("world")
		frame := append([]byte{0x81, byte(len(body))}, body...)
		if _, err := conn.Write(frame); err != nil {
			errCh <- err
			return
		}
		_, _ = conn.Write([]byte{0x88, 0x00})
		errCh <- nil
	}()
	t.Cleanup(func() {
		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("websocket peer: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("websocket peer did not finish")
		}
	})
	return ln.Addr().String()
}

// TestPhaseV1221_WebSocketLoopback proves a real RFC 6455 handshake and text
// frame exchange over a loopback TCP connection.
func TestPhaseV1221_WebSocketLoopback(t *testing.T) {
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			key := v1221WSKey(t)
			addr := v1221StartWSPeer(t, key)
			host, portStr, err := net.SplitHostPort(addr)
			if err != nil {
				t.Fatal(err)
			}
			src := fmt.Sprintf(`import std.http
func main() {
    let fd = ws_connect("%s", %s, "/", "%s")
    print(fd >= 0)
    print(ws_send(fd, "hello"))
    print(ws_recv(fd, 1024))
    print(ws_error())
}
`, host, portStr, key)
			path := v1221Source(t, "ws", src)
			out := v1221Run(t, engine, path)
			lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
			if len(lines) < 4 {
				t.Fatalf("expected 4 WS lines, got:\n%s", out)
			}
			if strings.TrimSpace(lines[0]) != "1" {
				t.Errorf("handshake failed:\n%s", out)
			}
			if strings.TrimSpace(lines[2]) != "world" {
				t.Errorf("frame = %q:\n%s", lines[2], out)
			}
			if strings.TrimSpace(lines[3]) != "" {
				t.Errorf("ws_error = %q:\n%s", lines[3], out)
			}
		})
	}
}

// TestPhaseV1221_RuntimeWiring pins that a clean program emits neither UDP nor
// WebSocket runtime symbols.
func TestPhaseV1221_RuntimeWiring(t *testing.T) {
	karkain := v1221Prepare(t)
	dir, err := os.MkdirTemp(repoRoot(t), "v1221wire")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	plain := filepath.Join(dir, "plain.kark")
	outC := filepath.Join(dir, "plain.c")
	if err := os.WriteFile(plain, []byte("func main() { print(\"plain\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "build", plain, "--engine", "go", "--target", "c23", "-o", outC)
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, string(msg))
	}
	data, err := os.ReadFile(outC)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "karkain_udp_send(") || strings.Contains(string(data), "karkain_ws_send_text(") {
		t.Errorf("metric-free program emitted UDP/WS runtime")
	}
}

// TestPhaseV1221_NativeRefused pins the C23-only boundary.
func TestPhaseV1221_NativeRefused(t *testing.T) {
	karkain := v1221Prepare(t)
	path := v1221Source(t, "native", "func main() { let fd = udp_bind(\"127.0.0.1\", 0) }\n")
	out := filepath.Join(filepath.Dir(path), "native.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "build", path, "--engine", "go",
		"--target", "native-x86_64-windows", "-o", out)
	msg, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("native build unexpectedly succeeded:\n%s", string(msg))
	}
	if !strings.Contains(string(msg), "udp_") {
		t.Errorf("refusal must name a udp_* builtin:\n%s", string(msg))
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Errorf("native refusal should not write an image")
	}
}
