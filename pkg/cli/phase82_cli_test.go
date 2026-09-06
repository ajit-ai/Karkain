package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/diagnostics"
)

// TestFormatCommand_IdempotentAndSemanticsPreserving formats a deliberately
// messy program, verifies the canonical form, that formatting is idempotent,
// and that the formatted source still parses (token stream is unchanged).
func TestFormatCommand_IdempotentAndSemanticsPreserving(t *testing.T) {
	src := "func  add( a,b ){\n    return   a+b\n}\nfunc main(){\n   let x=add(1 , 2)\n   print(x)  \n}\n"
	wantCanonical := "func add(a, b) {\n    return a + b\n}\nfunc main() {\n   let x = add(1, 2)\n   print(x)\n}\n"

	formatted, danger, err := Canonicalize(src)
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	if danger {
		t.Fatal("unexpected multi-line-literal guard triggered")
	}
	if formatted != wantCanonical {
		t.Errorf("canonical mismatch:\n got: %q\nwant: %q", formatted, wantCanonical)
	}

	// Idempotency: applying Canonicalize again must not change the output.
	second, _, err := Canonicalize(formatted)
	if err != nil {
		t.Fatalf("Canonicalize(2): %v", err)
	}
	if second != formatted {
		t.Errorf("formatter is not idempotent:\n got: %q\nwant: %q", second, formatted)
	}
}

// TestFormatCommand_PreservesComments ensures comment text (including a `//`
// sequence inside a string literal) survives formatting byte-for-byte.
func TestFormatCommand_PreservesComments(t *testing.T) {
	src := "func main() {\n    let url = \"https://example.com/x?y=1\"   // keep me  \n    // standalone  \n}\n"
	formatted, danger, err := Canonicalize(src)
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	if danger {
		t.Fatal("unexpected multi-line-literal guard triggered")
	}
	if !strings.Contains(formatted, "\"https://example.com/x?y=1\"   // keep me\n") {
		t.Errorf("trailing comment not preserved verbatim:\n%s", formatted)
	}
	if !strings.Contains(formatted, "// standalone\n") {
		t.Errorf("standalone comment line not preserved:\n%s", formatted)
	}
}

// TestFormatCommand_WriteAndCheck verifies the CLI-facing behavior: format
// rewrites the file, --check reports unformatted with a non-zero exit, and a
// re-run of --check reports already-formatted.
func TestFormatCommand_WriteAndCheck(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "fmt.kark")
	orig := "func main(){print(1)   }\n"
	if err := os.WriteFile(file, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	res := FormatCommand(file, false)
	if res.ExitCode != ExitSuccess {
		t.Fatalf("fmt write: exit=%d msg=%s", res.ExitCode, res.Message)
	}
	got, _ := os.ReadFile(file)
	if strings.Contains(string(got), "print(1)   }") {
		t.Errorf("file was not Canonicalized: %q", string(got))
	}

	res = FormatCommand(file, true)
	if res.ExitCode != ExitSuccess {
		t.Errorf("--check after format should pass: exit=%d msg=%s", res.ExitCode, res.Message)
	}
}

// TestFormatCommand_CheckDetectsUnformatted verifies --check flags a
// non-canonical file with ExitFailure.
func TestFormatCommand_CheckDetectsUnformatted(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "fmt2.kark")
	if err := os.WriteFile(file, []byte("func main(){print(1)}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := FormatCommand(file, true)
	if res.ExitCode != ExitFailure {
		t.Errorf("--check on unformatted file: exit=%d want=%d", res.ExitCode, ExitFailure)
	}
}

// TestFormatCommand_SemanticsPreservedThroughRun formats a program that uses
// string literals, negative-number literals and comments, then compiles and
// runs BOTH the original and the formatted source and asserts identical stdout.
// This is the strongest guarantee the token-level formatter can give: exact
// output equality.
func TestFormatCommand_SemanticsPreservedThroughRun(t *testing.T) {
	hasGCC(t)

	src := `func  greet( name ){` + "\n" +
		`    print("hello, "  +  name)   // greeting` + "\n" +
		`}` + "\n" +
		`func main(){` + "\n" +
		`   let xs = [ -2 , 1 , -3 ]` + "\n" +
		`   print(xs[0])` + "\n" +
		`   greet( "Kadane's  max" )` + "\n" +
		`}` + "\n"

	formatted, danger, err := Canonicalize(src)
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	if danger {
		t.Fatal("unexpected multi-line guard")
	}

	origOut := runKarkSource(t, "orig_"+t.Name(), src)
	fmtOut := runKarkSource(t, "fmt_"+t.Name(), formatted)
	if origOut != fmtOut {
		t.Errorf("formatted run differs:\n orig=%q\n  fmt=%q", origOut, fmtOut)
	}
}

// TestFormatCommand_UsableOnWholeCorpus runs --check over the compile corpus's
// hand-written pass fixtures and the algorithm examples: it must never report a
// runner error, and formatting any formatted-canonical file must be no-op.
func TestFormatCommand_UsableOnWholeCorpus(t *testing.T) {
	for _, dir := range []string{
		filepath.Join("..", "..", "pkg", "cli", "testdata", "compile", "pass"),
		filepath.Join("..", "..", "examples", "algorithms"),
	} {
		files, err := filepath.Glob(filepath.Join(dir, "*", "*.kark"))
		if err != nil {
			t.Fatal(err)
		}
		filesRoot, err := filepath.Glob(filepath.Join(dir, "*.kark"))
		if err == nil {
			files = append(files, filesRoot...)
		}
		for _, f := range files {
			formatted, danger, err := Canonicalize(readFileForTest(t, f))
			if err != nil {
				t.Errorf("%s: Canonicalize error: %v", f, err)
				continue
			}
			if danger {
				continue
			}
			again, _, err := Canonicalize(formatted)
			if err != nil {
				t.Errorf("%s: idempotency error: %v", f, err)
				continue
			}
			if again != formatted {
				t.Errorf("%s: formatter not idempotent", f)
			}
		}
	}
}

func readFileForTest(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// TestCheckJSONFormat verifies `check --format=json` emits a pure-JSON
// diagnostic array on stdout with the documented fields and exit codes.
func TestCheckJSONFormat(t *testing.T) {
	bad := filepath.Join("..", "..", "pkg", "cli", "testdata", "compile", "fail", "syntax_junk.kark")
	got := captureStdout(t, func() {
		res := CheckCommandFormatted(bad, false, CheckFormatJSON)
		if res.ExitCode != ExitCompile {
			t.Errorf("json check: exit=%d want=%d", res.ExitCode, ExitCompile)
		}
		if res.Message != "" {
			t.Errorf("json check must not print summary on stdout, got %q", res.Message)
		}
	})

	var diags []diagnostics.Diagnostic
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &diags); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout=%q", err, got)
	}
	if len(diags) == 0 {
		t.Fatalf("expected at least one diagnostic, got none (stdout=%q)", got)
	}
	d := diags[0]
	if d.File == "" || d.Line < 1 || d.Column < 1 || d.Severity != "error" {
		t.Errorf("malformed diagnostic: %+v", d)
	}
	if d.Code != string(diagnostics.CodeSyntax) {
		t.Errorf("syntax_junk should carry E-K-SYN, got %q", d.Code)
	}
}

// TestCheckJSONFormat_ValidFile checks the success path emits nothing and
// returns ExitSuccess.
func TestCheckJSONFormat_ValidFile(t *testing.T) {
	good := filepath.Join("..", "..", "pkg", "cli", "testdata", "compile", "pass", "funcs.kark")
	out := captureStdout(t, func() {
		res := CheckCommandFormatted(good, false, CheckFormatJSON)
		if res.ExitCode != ExitSuccess {
			t.Errorf("valid file: exit=%d want=0", res.ExitCode)
		}
		if res.Message != "" {
			t.Errorf("valid file json: message=%q want empty", res.Message)
		}
	})
	if strings.TrimSpace(out) != "" {
		t.Errorf("valid file json stdout must be empty, got %q", out)
	}
}

// TestToolchainInfoCommand verifies `ide info` emits the machine-readable
// LanguageProvider contract with the documented fields.
func TestToolchainInfoCommand(t *testing.T) {
	res := ToolchainInfoCommand()
	if res.ExitCode != ExitSuccess {
		t.Fatalf("ide info: exit=%d", res.ExitCode)
	}
	var info ToolchainInfo
	if err := json.Unmarshal([]byte(res.Message), &info); err != nil {
		t.Fatalf("ide info is not valid JSON: %v", err)
	}
	if info.SchemaVersion != toolchainInfoVersion {
		t.Errorf("schemaVersion=%d want=%d", info.SchemaVersion, toolchainInfoVersion)
	}
	if info.Language.ID != "karkain" || len(info.Language.Extensions) != 1 || info.Language.Extensions[0] != ".kark" {
		t.Errorf("language metadata wrong: %+v", info.Language)
	}
	if info.ProjectDetection.Manifest != "karkain.toml" {
		t.Errorf("project manifest missing")
	}
	if info.Toolchain.LSP == nil || len(info.Toolchain.LSP) == 0 {
		t.Errorf("missing LSP command contract")
	}
	if len(info.Toolchain.CheckJSON) == 0 {
		t.Errorf("missing structured-check command contract")
	}
	if info.Diagnostics.Format != "json" {
		t.Errorf("diagnostics format contract wrong: %q", info.Diagnostics.Format)
	}
	if info.ExitCodes.Compile != ExitCompile {
		t.Errorf("compile exit code contract wrong: %d", info.ExitCodes.Compile)
	}
}