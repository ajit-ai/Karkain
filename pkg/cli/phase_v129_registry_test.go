// v1.2.29 remote package registry gate.
//
// The HTTP client already existed (Phase 135 left the v1 protocol in pkg/pm);
// this slice exposes it through `karkain pkg search` and `karkain pkg info`
// and proves a real HTTP registry can serve a package through the normal
// add/fetch/cache path. The test server is in-process: no public registry is
// contacted and CI stays hermetic.
package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func v129Tarball(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := map[string]string{
		"karkain.toml": "name = \"remotepkg\"\nversion = \"1.0.0\"\n",
		"main.kark":    "public func remote_value() { return 7 }\n",
	}
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func v129Server(t *testing.T) *httptest.Server {
	t.Helper()
	archive := v129Tarball(t)
	sum := sha256.Sum256(archive)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/packages":
			_ = json.NewEncoder(w).Encode([]map[string]string{{
				"name": "remotepkg", "version": "1.0.0",
				"description": "remote test package", "author": "tester",
			}})
		case r.URL.Path == "/v1/packages/remotepkg":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name": "remotepkg", "latest": "1.0.0", "versions": []string{"1.0.0"},
				"description": "remote test package", "author": "tester", "license": "MIT",
			})
		case r.URL.Path == "/v1/packages/remotepkg/1.0.0/download":
			w.Header().Set("X-Karkain-Sha256", hex.EncodeToString(sum[:]))
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestPhaseV129_RemoteDiscoveryBothCommands(t *testing.T) {
	srv := v129Server(t)
	defer srv.Close()
	bin := v1226Prepare(t)
	dir := t.TempDir()

	out, err := runBinEnv(t, bin, dir, []string{"KARKAIN_REGISTRY=" + srv.URL}, "pkg", "search", "remote")
	if err != nil {
		t.Fatalf("pkg search failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Found 1 packages:") || !strings.Contains(out, "remotepkg") ||
		!strings.Contains(out, "1.0.0") || !strings.Contains(out, "remote test package") {
		t.Errorf("search result missing package: %q", out)
	}

	// --registry URL form must work too (not only the env fallback).
	out, err = runBinEnv(t, bin, dir, nil, "pkg", "info", "remotepkg", "--registry", srv.URL)
	if err != nil {
		t.Fatalf("pkg info failed: %v\n%s", err, out)
	}
	for _, want := range []string{"Name:        remotepkg", "Latest:      1.0.0", "remote test package"} {
		if !strings.Contains(out, want) {
			t.Errorf("info missing %q: %q", want, out)
		}
	}
}

func TestPhaseV129_RemoteFetchThroughCLI(t *testing.T) {
	srv := v129Server(t)
	defer srv.Close()
	bin := v1226Prepare(t)
	root := t.TempDir()
	proj := filepath.Join(root, "consumer")
	if out, err := runBin(t, bin, root, "pkg", "init", "consumer"); err != nil {
		t.Fatalf("pkg init failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(proj, "karkain.toml")); err != nil {
		t.Fatalf("project manifest missing: %v", err)
	}

	out, err := runBinEnv(t, bin, proj, []string{"KARKAIN_REGISTRY=" + srv.URL},
		"pkg", "add", "remotepkg", "1.0.0")
	if err != nil {
		t.Fatalf("pkg add from HTTP registry failed: %v\n%s", err, out)
	}
	manifest, readErr := os.ReadFile(filepath.Join(proj, "karkain.toml"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(manifest), "remotepkg") {
		t.Errorf("manifest did not record the dependency: %q", string(manifest))
	}
	found := false
	_ = filepath.Walk(filepath.Join(proj, ".karkain"), func(path string, info os.FileInfo, walkErr error) error {
		if walkErr == nil && !info.IsDir() && strings.HasSuffix(path, "main.kark") {
			found = true
		}
		return nil
	})
	if !found {
		t.Error("fetched package source not present in the local cache")
	}
}
