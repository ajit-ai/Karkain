package cli

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplainCommand_KnownCodes(t *testing.T) {
	for _, code := range []string{"E-K-SYN", "e-k-syn", "E-K-RES", "E-K-BRW", "E-K-SEM", "E-K-TYP", "E-K-CG", "E-K-PKG", "E-K-ENV", "E-PKG-LOCK", "E-PKG-GIT-REVISION"} {
		res := ExplainCommand(code)
		if res.ExitCode != ExitSuccess {
			t.Errorf("explain %s: got %d, want %d", code, res.ExitCode, ExitSuccess)
		}
	}
}

func TestExplainCommand_UnknownAndMalformed(t *testing.T) {
	if res := ExplainCommand("E-K-NOPE"); res.ExitCode != ExitUsage {
		t.Errorf("unknown code: got %d, want Usage", res.ExitCode)
	}
	if res := ExplainCommand("bogus"); res.ExitCode != ExitUsage {
		t.Errorf("malformed code: got %d, want Usage", res.ExitCode)
	}
	if res := ExplainCommand(""); res.ExitCode != ExitUsage {
		t.Errorf("missing code: got %d, want Usage", res.ExitCode)
	}
}

func TestExplainListCommand_ContainsAllCodes(t *testing.T) {
	out := captureStdout(t, func() {
		if res := ExplainListCommand(); res.ExitCode != ExitSuccess {
			t.Errorf("explain --list: got %d", res.ExitCode)
		}
	})
	for _, want := range []string{"E-K-SYN", "E-K-RES", "E-K-BRW", "E-K-SEM", "E-K-TYP", "E-K-CG", "E-K-PKG", "E-K-ENV", "E-PKG-LOCK"} {
		if !strings.Contains(out, want) {
			t.Errorf("explain --list missing %q", want)
		}
	}
}

func TestCLI_Explain_E2E(t *testing.T) {
	bin := buildKarkain(t)
	root := t.TempDir()

	out, err := runBin(t, bin, root, "explain", "E-K-SYN")
	if err != nil {
		t.Fatalf("explain failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "lexer") && !strings.Contains(out, "parser") {
		t.Errorf("explain E-K-SYN output unexpected:\n%s", out)
	}

	_, err = runBin(t, bin, root, "explain", "E-K-NOPE")
	if err == nil {
		t.Fatal("unknown code should fail")
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != ExitUsage {
		t.Errorf("unknown code should exit %d", ExitUsage)
	}
}

func TestLintCommand_PassesCleanFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "ok.kark")
	writeFile(t, file, "func main() { print(1) }\n")

	res := LintCommand(file, false)
	if res.ExitCode != ExitSuccess {
		t.Fatalf("lint clean file: got %d, want 0: %s", res.ExitCode, res.Message)
	}
}

func TestLintCommand_SyntaxErrorClassified(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "bad.kark")
	writeFile(t, file, "func main() { print(1) } extrajunk\n")

	res := LintCommand(file, false)
	if res.ExitCode != ExitCompile {
		t.Fatalf("lint syntax error: got %d, want %d", res.ExitCode, ExitCompile)
	}
	out := captureStdout(t, func() { LintCommand(file, false) })
	if !strings.Contains(out, "E-K-SYN") {
		t.Errorf("lint should tag syntax issues with E-K-SYN, got:\n%s", out)
	}
}

func TestLintCommand_NameResErrorClassified(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "dup.kark")
	writeFile(t, file, "func foo() { return 1 }\nfunc foo() { return 2 }\n")

	res := LintCommand(file, false)
	if res.ExitCode != ExitCompile {
		t.Fatalf("lint name-res error: got %d, want %d", res.ExitCode, ExitCompile)
	}
	out := captureStdout(t, func() { LintCommand(file, false) })
	if !strings.Contains(out, "E-K-RES") {
		t.Errorf("lint should tag name-resolution issues with E-K-RES, got:\n%s", out)
	}
}

func TestCLI_Lint_E2E(t *testing.T) {
	bin := buildKarkain(t)
	root := t.TempDir()
	file := filepath.Join(root, "lint.kark")
	writeFile(t, file, "func main() { print(1) }\n")

	out, err := runBin(t, bin, root, "lint", file)
	if err != nil {
		t.Fatalf("lint failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Lint passed") {
		t.Errorf("expected lint pass message, got:\n%s", out)
	}
}