package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/codegen"
	"karkain/pkg/pm"
)

// buildWorkspaceDir creates a temp workspace with the given members, writing a
// kark manifest with an entrypoint into each, and returns the root.
func buildWorkspaceDir(t *testing.T, members []string, memberDeps map[string][]string) string {
	t.Helper()
	root := t.TempDir()
	if err := pm.InitWorkspace(root); err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if err := pm.AddWorkspaceMember(root, m); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(root, m, "src")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		entry := filepath.Join(dir, "main.kark")
		body := ""
		if len(memberDeps[m]) == 0 {
			body = "func main() { print(\"hi from " + m + "\") }\n"
		} else {
			calls := ""
			for _, dep := range memberDeps[m] {
				calls += "  " + dep + "()\n"
			}
			body = "func main() {\n" + calls + "  print(\"hi\")\n}\n"
		}
		if err := os.WriteFile(entry, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		// Workspace members that are depended on must expose their API from a
		// non-main module (the assembly keeps only the root file's `func main`;
		// a dependency's main.kark is dropped and `dep()` calls would be
		// undefined). Phase 117 build/run semantic gating rejects exactly that.
		for _, deps := range memberDeps {
			for _, d := range deps {
				if d == m {
					api := filepath.Join(dir, "api.kark")
					if err := os.WriteFile(api, []byte(
						"func "+m+"() { print(\"hi from "+m+" api\") }\n"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		mm := &pm.Manifest{Name: m, Version: "1.0.0", Dependencies: map[string]pm.Dependency{}}
		if deps, ok := memberDeps[m]; ok {
			for _, d := range deps {
				mm.Dependencies[d] = pm.Dependency{Name: d, Version: "1.0.0", Source: "workspace", URL: d}
			}
		}
		if err := pm.WriteManifest(filepath.Join(root, m, pm.ManifestFile), mm); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// captureStdout runs fn with os.Stdout redirected and returns captured output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	fn()

	w.Close()
	<-done
	os.Stdout = old
	return buf.String()
}

// TestWorkspaceBuild_OrderE2E verifies WorkspaceBuild compiles members in
// dependency order by observing the build output sequence.
func TestWorkspaceBuild_OrderE2E(t *testing.T) {
	root := buildWorkspaceDir(t,
		[]string{"lib", "app"},
		map[string][]string{"lib": {}, "app": {"lib"}},
	)

	cfg := codegen.NewConfig()
	cfg.CompileOnly = true

	out := captureStdout(t, func() {
		if err := WorkspaceBuild(root, cfg, false); err != nil {
			t.Errorf("WorkspaceBuild failed: %v", err)
		}
	})
	idxLib := strings.Index(out, "BUILD [0] lib")
	idxApp := strings.Index(out, "BUILD [1] app")
	if idxLib == -1 || idxApp == -1 {
		t.Fatalf("expected BUILD lines for lib.app, got:\n%s", out)
	}
	if idxLib > idxApp {
		t.Errorf("lib must build before app, got:\n%s", out)
	}
}

// TestWorkspaceBuild_CycleFails verifies a member cycle is rejected before any
// build occurs.
func TestWorkspaceBuild_CycleFails(t *testing.T) {
	root := t.TempDir()
	if err := pm.InitWorkspace(root); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"a", "b"} {
		if err := pm.AddWorkspaceMember(root, m); err != nil {
			t.Fatal(err)
		}
	}
	// a depends on b, b depends on a
	writePM(t, filepath.Join(root, "a", pm.ManifestFile), map[string]string{"b": "b"})
	writePM(t, filepath.Join(root, "b", pm.ManifestFile), map[string]string{"a": "a"})

	cfg := codegen.NewConfig()
	err := WorkspaceBuild(root, cfg, false)
	if err == nil {
		t.Fatal("expected cycle error from WorkspaceBuild")
	}
	if !strings.Contains(err.Error(), pm.ErrWsCycle) {
		t.Errorf("expected cycle code, got: %v", err)
	}
}

// TestWorkspaceTest_DiscoveryE2E verifies WorkspaceTest runs each member's test
// files in dependency order.
func TestWorkspaceTest_DiscoveryE2E(t *testing.T) {
	root := buildWorkspaceDir(t,
		[]string{"lib", "app"},
		map[string][]string{"lib": {}, "app": {"lib"}},
	)
	// add a passing test file to each member
	for _, m := range []string{"lib", "app"} {
		tf := filepath.Join(root, m, "x_test.kark")
		if err := os.WriteFile(tf, []byte("func test_pass() { print(1) }\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := codegen.NewConfig()
	cfg.CompileOnly = true

	out := captureStdout(t, func() {
		if err := WorkspaceTest(root, cfg, false); err != nil {
			t.Errorf("WorkspaceTest failed: %v", err)
		}
	})
	if !strings.Contains(out, "TEST  [0] lib") || !strings.Contains(out, "TEST  [1] app") {
		t.Fatalf("expected TEST lines for lib,app, got:\n%s", out)
	}
}

func writePM(t *testing.T, path string, deps map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	mm := &pm.Manifest{Name: filepath.Base(filepath.Dir(path)), Version: "1.0.0", Dependencies: map[string]pm.Dependency{}}
	for name, url := range deps {
		mm.Dependencies[name] = pm.Dependency{Name: name, Version: "1.0.0", Source: "workspace", URL: url}
	}
	if err := pm.WriteManifest(path, mm); err != nil {
		t.Fatal(err)
	}
}
