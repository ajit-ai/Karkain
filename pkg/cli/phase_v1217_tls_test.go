// v1.2.17 - C23 TLS/HTTPS on both engines.
//
// The gate starts a real TLS server in-process (crypto/tls, self-signed cert)
// and drives the compiled Karkain program through both the Go front end and
// the self-hosted kcc engine. It asserts the decrypted HTTP response body, not
// just that a link succeeded. Byte-identity with the Go native backend is NOT
// claimed: native/WASM have no sockets (v1.2.10 platform decision).
package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var v1217Once sync.Once
var v1217Bin string
var v1217Err error

func v1217Prepare(t *testing.T) string {
	t.Helper()
	v1217Once.Do(func() {
		// The QA harness fix from v1.2.16 is applied here too: without these
		// directories the child cannot find gcc/git on this host.
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
		v1217Err = buildKarkainErr(t, &v1217Bin)
	})
	if v1217Err != nil {
		t.Fatalf("building karkain: %v", v1217Err)
	}
	return v1217Bin
}

func v1217TLSFixture(t *testing.T, body string, verify int) string {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(repoRoot(t), "v1217")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	src := fmt.Sprintf(
		"import std.http\nfunc main() {\n"+
			"\tstring resp = https_get(\"%s\", %s, \"/\", %d)\n"+
			"\tprint(resp)\n"+
			"\tprint(https_last_error())\n"+
			"}\n",
		u.Hostname(), u.Port(), verify)
	path := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func v1217Run(t *testing.T, engine, src string) (string, error) {
	t.Helper()
	karkain := v1217Prepare(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "run", src, "--engine", engine)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("%s engine timed out: %v\n%s", engine, ctx.Err(), string(out))
	}
	return string(out), err
}

// TestPhaseV1217_LoopbackTLSBothEngines proves a real TLS handshake and HTTP
// response on both engines, using verify=0 against the in-process self-signed
// server. The body is exact; the Date header is not golden-tested.
func TestPhaseV1217_LoopbackTLSBothEngines(t *testing.T) {
	const body = "V1217-TLS-BODY"
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			src := v1217TLSFixture(t, body, 0)
			out, err := v1217Run(t, engine, src)
			if err != nil {
				t.Fatalf("%s engine run failed: %v\n%s", engine, err, out)
			}
			if !strings.Contains(out, body) {
				t.Errorf("missing decrypted body %q:\n%s", body, out)
			}
		})
	}
}

// TestPhaseV1217_VerifyRejectsSelfSigned pins the verify flag: certificate
// validation must reject the self-signed loopback server rather than silently
// accepting it, and the failure must be observable through https_last_error.
func TestPhaseV1217_VerifyRejectsSelfSigned(t *testing.T) {
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			src := v1217TLSFixture(t, "SHOULD-NOT-ARRIVE", 1)
			out, err := v1217Run(t, engine, src)
			if err != nil {
				t.Fatalf("%s engine run failed: %v\n%s", engine, err, out)
			}
			if strings.Contains(out, "SHOULD-NOT-ARRIVE") {
				t.Errorf("verify=1 accepted a self-signed certificate:\n%s", out)
			}
			if !strings.Contains(out, "https:") {
				t.Errorf("expected a TLS diagnostic, got:\n%s", out)
			}
		})
	}
}

// TestPhaseV1217_RuntimeWiring proves the generated C references the TLS
// runtime only for programs that call https_request (the bootstrap compiler
// must keep its historical link flags). It uses compile-only c23 output.
func TestPhaseV1217_RuntimeWiring(t *testing.T) {
	karkain := v1217Prepare(t)
	dir, err := os.MkdirTemp(repoRoot(t), "v1217wire")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	src := filepath.Join(dir, "wire.kark")
	body := "func main() {\n\tstring s = \"plain\"\n\tprint(s)\n}\n"
	if err := os.WriteFile(src, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	outC := filepath.Join(dir, "wire.c")
	cmd := exec.CommandContext(ctx, karkain, "build", src, "--engine", "go", "--target", "c23", "-o", outC)
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("plain c23 build failed: %v\n%s", err, string(msg))
	}
	data, err := os.ReadFile(outC)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "karkain_https_request(") {
		t.Errorf("TLS-free program emitted the TLS runtime")
	}
}
