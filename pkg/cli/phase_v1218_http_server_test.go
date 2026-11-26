// v1.2.18 - Concurrent HTTP server on the C23 path, both engines.
//
// The library exposes http_serve_echo(host, port): a blocking accept loop that
// spawns a named worker per connection. This gate proves CONCURRENCY, not just
// two sequential requests: the first client connects but deliberately sends
// nothing, so a serial server blocks in its first read; the second client must
// still receive its response within a short deadline.
//
// Native/WASM remain excluded by the v1.2.10 socket decision.
package cli

import (
	"context"
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

var v1218Once sync.Once
var v1218Bin string
var v1218Err error

func v1218Prepare(t *testing.T) string {
	t.Helper()
	v1218Once.Do(func() {
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
		v1218Err = buildKarkainErr(t, &v1218Bin)
	})
	if v1218Err != nil {
		t.Fatalf("building karkain: %v", v1218Err)
	}
	return v1218Bin
}

func v1218FreeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func v1218StartServer(t *testing.T, engine, addr string) {
	t.Helper()
	karkain := v1218Prepare(t)
	dir, err := os.MkdirTemp(repoRoot(t), "v1218")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	src := fmt.Sprintf(
		"import std.http\nfunc main() {\n\thttp_serve_echo(\"%s\", %s)\n}\n",
		host, port)
	path := filepath.Join(dir, "server.kark")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(karkain, "run", path, "--engine", engine)
	// nil (not io.Discard) is load-bearing: io.Discard makes os/exec create
	// pipe-copy goroutines, and the compiled program inherits the compiler's
	// stdout. Killing karkain then leaves those goroutines blocked forever and
	// Wait() never returns. nil connects directly to the null device.
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.WaitDelay = 3 * time.Second
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})
	// Poll the listener instead of stdout: C stdio buffering makes a pipe
	// readiness line unreliable, and a bound port is the real contract.
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		if c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond); err == nil {
			_ = c.Close()
			return
		}
		if cmd.ProcessState != nil {
			t.Fatalf("%s server exited before binding", engine)
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("%s server did not bind %s before deadline", engine, addr)
}

func v1218Request(t *testing.T, conn net.Conn, path, want string) {
	t.Helper()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n", path)
	if _, err := io.WriteString(conn, req); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	data, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read %s (serial server would block here): %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Errorf("response for %s missing %q:\n%s", path, want, string(data))
	}
	_ = conn.Close()
}

// TestPhaseV1218_ConcurrentLoopback proves that a silent first connection
// cannot block a second one. This is the property that distinguishes a
// concurrent server from two sequential requests.
func TestPhaseV1218_ConcurrentLoopback(t *testing.T) {
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			addr := v1218FreeAddr(t)
			v1218StartServer(t, engine, addr)

			// Client A connects but sends NO request. A serial accept loop is
			// now stuck inside http_read_request for this socket.
			first, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer first.Close()

			// Client B must be served while A is still silent.
			second, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			v1218Request(t, second, "/second", "hello /second")

			// A is served only after B has completed, proving the order was
			// not an artifact of a serial loop that happened to finish fast.
			v1218Request(t, first, "/first", "hello /first")
		})
	}
}

// TestPhaseV1218_RuntimeWiring pins that the concurrency runtime is emitted
// for the spawning server but not for an otherwise identical TLS-free,
// spawn-free program. It keeps the bootstrap compiler's runtime unchanged.
func TestPhaseV1218_RuntimeWiring(t *testing.T) {
	karkain := v1218Prepare(t)
	dir, err := os.MkdirTemp(repoRoot(t), "v1218wire")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	spawnSrc := "import std.http\nfunc main() { http_serve_echo(\"127.0.0.1\", 18099) }\n"
	// A plain program must not import std.http: the library itself now
	// contains http_serve_echo, whose spawn legitimately pulls the runtime
	// into every translation unit that imports it.
	plainSrc := "func main() { print(\"plain\") }\n"
	for name, src := range map[string]string{"spawn": spawnSrc, "plain": plainSrc} {
		path := filepath.Join(dir, name+".kark")
		out := filepath.Join(dir, name+".c")
		if err := os.WriteFile(path, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		cmd := exec.CommandContext(ctx, karkain, "build", path, "--engine", "go", "--target", "c23", "-o", out)
		if msg, err := cmd.CombinedOutput(); err != nil {
			cancel()
			t.Fatalf("%s c23 build failed: %v\n%s", name, err, string(msg))
		}
		cancel()
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		hasRuntime := strings.Contains(string(data), "static void karkain_run_")
		if name == "spawn" && !hasRuntime {
			t.Errorf("spawning server did not emit the concurrency wrapper")
		}
		if name == "plain" && hasRuntime {
			t.Errorf("spawn-free program emitted the concurrency wrapper")
		}
	}
}
