package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhase112ConstTitle(t *testing.T) {
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join(tmp, "const.kark")

			// UTF-8 no BOM.
			body := "func main() {\n    const MAX = 100\n    println(MAX + 1)\n}\n"
			if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command("go", "run", "../../cmd/karkain", "run", "--engine", engine, src)
			out, err := cmd.CombinedOutput()
			t.Logf("run out: %s", out)
			if err != nil {
				t.Fatalf("run failed: %v\n%s", err, out)
			}
			if got := strings.TrimRight(string(out), "\r\n"); !strings.HasSuffix(got, "101") {
				t.Fatalf("expected 101 output, got %q", string(out))
			}
		})
	}
}

func TestPhase112ConstReassignmentRejected(t *testing.T) {
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join(tmp, "const_bad.kark")

			body := "func main() {\n    const MAX = 100\n    MAX = 200\n    println(MAX)\n}\n"
			if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command("go", "run", "../../cmd/karkain", "check", "--engine", engine, src)
			out, err := cmd.CombinedOutput()
			combined := string(out)
			t.Logf("check out: %s", combined)
			if err == nil {
				t.Fatalf("expected check failure for const reassignment")
			}
			if !strings.Contains(combined, "cannot reassign constant") || !strings.Contains(combined, "MAX") {
				t.Fatalf("unexpected diagnostic: %s", combined)
			}
		})
	}
}

func TestPhase112LetReassignmentAllowed(t *testing.T) {
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join(tmp, "let_ok.kark")

			body := "func main() {\n    let x = 1\n    x = 2\n    println(x)\n}\n"
			if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command("go", "run", "../../cmd/karkain", "run", "--engine", engine, src)
			out, err := cmd.CombinedOutput()
			t.Logf("run out: %s", out)
			if err != nil {
				t.Fatalf("let reassignment should run clean: %v\n%s", err, out)
			}
			if !strings.HasSuffix(strings.TrimRight(string(out), "\r\n"), "2") {
				t.Fatalf("expected 2 output, got %q", string(out))
			}
		})
	}
}

func TestPhase112FloatConversion(t *testing.T) {
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join(tmp, "conv.kark")

			body := "func main() {\n    println(float(float(2.5)))\n    println(float(3))\n    println(float(\"4.25\"))\n}\n"
			if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command("go", "run", "../../cmd/karkain", "run", "--engine", engine, src)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("run failed: %v\n%s", err, out)
			}
			norm := strings.ReplaceAll(string(out), "\r\n", "\n")
			want := "2.5\n3\n4.25\n"
			if !strings.HasSuffix(norm, want) {
				t.Fatalf("output mismatch: got %q want suffix %q", string(out), want)
			}
		})
	}
}

func TestPhase112BlockComment(t *testing.T) {
	for _, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			tmp := t.TempDir()
			src := filepath.Join(tmp, "comment.kark")

			body := "/* multi\nline comment */ func main() {\n    println(9)\n}\n"
			if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command("go", "run", "../../cmd/karkain", "run", "--engine", engine, src)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("run failed: %v\n%s", err, out)
			}
			if !strings.HasSuffix(strings.TrimRight(string(out), "\r\n"), "9") {
				t.Fatalf("expected 9 output, got %q", string(out))
			}
		})
	}
}