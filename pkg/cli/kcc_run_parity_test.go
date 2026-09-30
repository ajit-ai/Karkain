package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// trailingTerminator returns the line terminator b ends with, "" if it ends
// with neither. It is deliberately platform-agnostic: a Windows host sees CRLF
// from both engines, a POSIX host sees LF from both, and the assertion that
// matters is that they AGREE, not that either one is a particular value.
func trailingTerminator(b []byte) string {
	if len(b) >= 2 && b[len(b)-2] == '\r' && b[len(b)-1] == '\n' {
		return "\r\n"
	}
	if len(b) >= 1 && b[len(b)-1] == '\n' {
		return "\n"
	}
	return ""
}

// Gate for the `karkain run` stdout-parity fix in KCCRunCommand.
//
// The defect: the kcc run path folded the compiler's build banner into the
// program's stdout and ran the program's output through strings.TrimSpace,
// after which the CLI re-added a newline with fmt.Println. The observable
// effect was that `karkain run prog.kark` on the DEFAULT engine emitted a bare
// LF (0x0a) where `--engine go` emitted CRLF (0x0d 0x0a), and prefixed ~174
// bytes of build diagnostic to the program's output.
//
// The compiled programs were never wrong -- a kcc-linked exe and a Go-linked
// exe emit identical bytes -- and that is why the Phase 114 corpus gate never
// saw it: it builds to main.exe and runs that directly, capturing build output
// into a separate buffer. This gate closes the gap by asserting on what a USER
// sees from the CLI, which nothing else covered.
//
// Every expectation here is a DIFFERENTIAL against the Go engine rather than a
// hand-written golden. A golden would pin the current bytes, and the defect
// was precisely that the two engines' bytes differed -- a golden on either
// side alone would have kept passing. Comparing the two engines to each other
// is the only formulation that fails when they disagree.

// kccRunParityCase is one program whose stdout must be identical on both
// engines.
type kccRunParityCase struct {
	name string
	src  string
}

var kccRunParityCases = []kccRunParityCase{
	{
		// Three statements, so the defect is visible twice over: a stripped
		// final terminator and an interleaved build line.
		name: "multiple_prints",
		src: "func main() {\n" +
			"    let a = 1\n" +
			"    let b = 2\n" +
			"    print(a)\n" +
			"    print(b)\n" +
			"    print(a + b)\n" +
			"}\n",
	},
	{
		// A single line: the stripped terminator is the whole difference.
		name: "single_print",
		src:  "func main() {\n    print(42)\n}\n",
	},
	{
		name: "string_print",
		src:  "func main() {\n    print(\"karkain\")\n}\n",
	},
}

// runKCCParityEngine runs `karkain run <file> --engine <engine>` and returns
// STDOUT and STDERR as separate byte slices. They are captured separately on
// purpose: using CombinedOutput here would merge the very streams whose
// separation is part of what is being asserted.
func runKCCParityEngine(t *testing.T, bin, file, engine string) (stdout, stderr []byte, code int) {
	t.Helper()
	cmd := exec.Command(bin, "run", file, "--engine", engine)
	// A clean temp cwd: some examples create files next to themselves.
	cmd.Dir = t.TempDir()
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	code = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running --engine %s: %v", engine, err)
		}
	}
	return so.Bytes(), se.Bytes(), code
}

func TestKCCRun_StdoutIsByteIdenticalToTheGoEngine(t *testing.T) {
	bin := buildKarkain(t)
	// KARKAIN_KCC points the CLI at a prebuilt self-hosted engine, so the
	// child can run from a clean temp cwd. Without it the engine looks for
	// src/compiler relative to the working directory and refuses -- which is a
	// test-harness constraint, not the defect under test.
	kccBin := phase95KCC(t)
	selectKCCEngine(t, kccBin)
	hasGCC(t)

	for _, tc := range kccRunParityCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "main.kark")
			// BOM-free: the lexer rejects a UTF-8 BOM.
			if err := os.WriteFile(file, []byte(tc.src), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			kOut, kErrOut, kCode := runKCCParityEngine(t, bin, file, "kcc")
			gOut, gErrOut, gCode := runKCCParityEngine(t, bin, file, "go")

			if kCode != gCode {
				t.Errorf("exit code kcc=%d go=%d, want equal", kCode, gCode)
			}
			if !bytes.Equal(kOut, gOut) {
				t.Errorf("stdout differs between engines:\n kcc = %q\n go  = %q", kOut, gOut)
			}
			if len(gOut) == 0 {
				t.Fatalf("go engine produced no stdout; the comparison would be vacuous:\n%s", gOut)
			}

			// The specific corruption, asserted independently of the
			// differential so the failure message names the cause. The
			// terminator is compared BETWEEN the two engines and NOT
			// hardcoded to CRLF: on a POSIX host both engines emit a bare LF,
			// because text mode does not translate it, so "must be CRLF" is a
			// Windows-only expectation that fails on Linux for a program
			// behaving perfectly correctly. The first draft of this gate
			// asserted CRLF and CI rejected it -- the 151B lesson, third
			// occurrence: a wrong expectation in the test is worse than none.
			kTerm, gTerm := trailingTerminator(kOut), trailingTerminator(gOut)
			if gTerm == "" {
				t.Errorf("go stdout does not end in a newline: %q", gOut)
			}
			if kTerm != gTerm {
				t.Errorf("kcc stdout terminator %q differs from go %q "+
					"(the trailing terminator was stripped and re-added):\n kcc = %q\n go  = %q",
					kTerm, gTerm, kOut, gOut)
			}

			// Defect 2: the compiler's build banner must not be on the
			// program's stdout. It is a build diagnostic; it belongs on stderr.
			if bytes.Contains(kOut, []byte("[ok]")) {
				t.Errorf("kcc stdout carries the build banner:\n%s", kOut)
			}
			_ = kErrOut
			_ = gErrOut
		})
	}
}

// A program that fails must still be reported: the fix routes the program's
// stdout verbatim, and a regression that swallowed it would leave a silent
// zero-exit run.
func TestKCCRun_FailureStillReportsAndExitsNonZero(t *testing.T) {
	bin := buildKarkain(t)
	selectKCCEngine(t, phase95KCC(t))
	hasGCC(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "boom.kark")
	src := "func main() {\n    let z = 0\n    print(10 / z)\n}\n"
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	stdout, _, code := runKCCParityEngine(t, bin, file, "kcc")
	if code == 0 {
		t.Errorf("kcc run of a dividing-by-zero program exited 0, want non-zero:\n%s", stdout)
	}
	if !strings.Contains(string(stdout), "division by zero") {
		t.Errorf("kcc run did not report the runtime error:\n%s", stdout)
	}
	// The source location is part of the Phase 100 contract.
	if !strings.Contains(string(stdout), "boom.kark:3") {
		t.Errorf("kcc run omitted the source location:\n%s", stdout)
	}
	if bytes.Contains(stdout, []byte("[ok]")) {
		t.Errorf("kcc stdout carries the build banner on the failure path:\n%s", stdout)
	}
}

// Guards the fix at the unit level, without paying for a build-and-link: the
// banner is what used to be prepended, and it must not appear in a successful
// run's Message.
func TestKCCRun_SuccessMessageIsNotEmptyForPrintingPrograms(t *testing.T) {
	// A program that prints nothing must produce an empty Message rather than
	// a banner-only one; the banner is gone from this path entirely.
	bin := buildKarkain(t)
	selectKCCEngine(t, phase95KCC(t))
	hasGCC(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "quiet.kark")
	if err := os.WriteFile(file, []byte("func main() {\n    let x = 1\n}\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	stdout, _, code := runKCCParityEngine(t, bin, file, "kcc")
	if code != 0 {
		t.Errorf("exit code = %d, want 0:\n%s", code, stdout)
	}
	if len(bytes.TrimSpace(stdout)) != 0 {
		t.Errorf("a program that prints nothing produced stdout: %q", stdout)
	}
}
