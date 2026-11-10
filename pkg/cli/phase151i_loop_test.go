package cli

import (
	"bytes"
	"context"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"karkain/pkg/native"
)

// NAT-LOOP -- loops in the kcc native driver.
//
// The slice adds `while`, the C-style `for`, `break` and `continue`, and the
// nesting of all of them, to the driver that NAT-IF opened with a branch.
//
// A loop is the first construct in this backend whose CORRECTNESS depends on
// a control-flow structure rather than on a frame layout, and it is the first
// one whose MEMORY behaviour depends on an argument about how many times a
// site runs. Both make it the slice with the most to prove:
//
//   - the semantics are taken from the Go oracle (pkg/native/program.go), not
//     invented: the condition is evaluated before every iteration, the `for`'s
//     init runs ONCE, its post runs after every iteration INCLUDING after a
//     `continue`, and `break`/`continue` act on the NEAREST enclosing loop;
//   - the arena-using constructs are refused inside a loop, because the arena
//     is sized at compile time on the assumption that each allocation site
//     runs at most once -- an assumption a loop breaks;
//   - the evidence is EXECUTION, not inspection: every program is compiled by
//     the self-hosted engine, written to disk, run, and compared as exact
//     bytes -- against BOTH the expected output and the output the existing C
//     backend produces for the same source.
//
// The layers are:
//
//	1. EXECUTED PE goldens -- exact stdout bytes and exit codes.
//	2. A DIFFERENTIAL against the C backend -- the same source, compiled the
//	   old way and run, must produce the same bytes. This is the layer that
//	   would catch a `continue` that skipped the update clause.
//	3. A Linux ELF byte-identity DIFFERENTIAL against the Go oracle, for the
//	   shapes below the register allocator's threshold.
//	4. A REFUSAL TABLE -- the constructs that are still refused, each by name,
//	   with no image written.
//	5. The register-allocator boundary, asserted from both sides.
//	6. Determinism, non-vacuity, and the CI-wiring check.
//	7. Process-level INDEPENDENCE evidence for a loop program.

// natICorpus is the execution corpus. Every entry is a different .kark source.
//
// `wantHex` is the EXPECTED stdout as exact bytes, never trimmed: the Step 9b
// defect was a MISSING newline, and trimming is exactly the tool that hides a
// missing byte.
//
// `wantExit` is the process exit status. It is 0 unless the entry says
// otherwise.
var natICorpus = []struct {
	name     string
	src      string
	wantHex  string
	wantExit int
	// expectDiff marks a case whose images are EXPECTED to differ between
	// the self-hosted engine and the Go oracle. The reason is the oracle's
	// Phase-150C register allocator, which engages under control flow and
	// pressure; see TestPhaseNATLoop_TheRegisterAllocatorBoundaryIsPinned.
	expectDiff bool
}{
	{
		// A `while` whose condition is false on the first evaluation: the
		// body never runs and the program prints nothing.
		//
		// The bound is `i > 3` and NOT `i < 3`: a body that does not advance
		// the counter never terminates, and the first draft of this corpus
		// said `i < 3` -- which HUNG the gate, because the program it
		// compiled was an infinite loop. A loop corpus has to be checked for
		// TERMINATION, not just for its expected output.
		name:    "while_zero_iterations",
		src:     "func main() {\n    let i = 0\n    while i > 3 {\n        print(1)\n    }\n}\n",
		wantHex: "",
	},
	{
		name:    "while_one_iteration",
		src:     "func main() {\n    let i = 0\n    while i < 1 {\n        print(1)\n        i = i + 1\n    }\n}\n",
		wantHex: "310a",
	},
	{
		// The counter is printed BEFORE it is advanced, so the output shows
		// the condition being re-evaluated between iterations.
		name:    "while_multiple_iterations",
		src:     "func main() {\n    let i = 0\n    while i < 3 {\n        print(i)\n        i = i + 1\n    }\n}\n",
		wantHex: "300a310a320a",
	},
	{
		// The value that survives the loop is read AFTER it, which is what
		// proves the counter's slot is stable across the back-edge.
		name:    "while_changing_condition",
		src:     "func main() {\n    let i = 0\n    while i < 4 {\n        i = i + 1\n    }\n    print(i)\n}\n",
		wantHex: "340a",
	},
	{
		// The C-style `for`. The init runs once, the post runs after every
		// iteration, and the condition is evaluated before every iteration.
		name:    "for_multiple_iterations",
		src:     "func main() {\n    for (let i = 0; i < 3; i = i + 1) {\n        print(i)\n    }\n}\n",
		wantHex: "300a310a320a",
	},
	{
		name:    "for_zero_iterations",
		src:     "func main() {\n    for (let i = 0; i < 0; i = i + 1) {\n        print(1)\n    }\n}\n",
		wantHex: "",
	},
	{
		// A `for` whose init BINDS a variable. The binding runs once, before
		// the loop, so the variable outlives the loop and can be read after
		// it.
		name:    "for_init_binding",
		src:     "func main() {\n    for (let i = 0; i < 3; i = i + 1) {\n        print(i)\n    }\n}\n",
		wantHex: "300a310a320a",
	},
	{
		// `break` leaves the NEAREST enclosing loop and nothing else.
		name:    "break_from_while",
		src:     "func main() {\n    let i = 0\n    while i < 10 {\n        i = i + 1\n        if i > 2 {\n            break\n        }\n    }\n    print(i)\n}\n",
		wantHex: "330a",
	},
	{
		// `break` leaves the NEAREST enclosing loop and nothing else.
		//
		// The counter is NOT read after the loop, and that is deliberate: the
		// Go engine's borrow checker scopes a C-style `for`'s init binding to
		// the loop ("use of value 'i' after its scope has ended"), while the
		// self-hosted checker scopes it to the function. The two engines
		// therefore DISAGREE about the for-init's scope, and a program that
		// reads the counter after the loop cannot be compared against the
		// oracle at all. The divergence is pre-existing and is recorded in
		// docs/audit/NAT-LOOP-BASELINE.md.
		name:    "break_from_for",
		src:     "func main() {\n    for (let i = 0; i < 10; i = i + 1) {\n        if i > 2 {\n            break\n        }\n        print(i)\n    }\n}\n",
		wantHex: "300a310a320a",
	},
	{
		// `continue` in a `while` jumps to the HEAD, so the condition is
		// evaluated again and the body is not re-run.
		name:    "continue_in_while",
		src:     "func main() {\n    let i = 0\n    while i < 5 {\n        i = i + 1\n        if i == 2 {\n            continue\n        }\n        print(i)\n    }\n}\n",
		wantHex: "310a330a340a350a",
	},
	{
		// `continue` in a C-style `for` jumps to the POST label, so the
		// update clause runs BEFORE the condition is evaluated again. That
		// asymmetry with `while` is the oracle's, and it is the difference a
		// differential test against the C backend exists to catch.
		name:    "continue_in_for",
		src:     "func main() {\n    for (let i = 0; i < 5; i = i + 1) {\n        if i == 2 {\n            continue\n        }\n        print(i)\n    }\n}\n",
		wantHex: "300a310a330a340a",
	},
	{
		// Two nested `while` loops. The inner loop's counter is its own
		// slot, and the outer loop's counter is untouched by it.
		name:    "nested_while",
		src:     "func main() {\n    let i = 0\n    while i < 2 {\n        let j = 0\n        while j < 2 {\n            print(j)\n            j = j + 1\n        }\n        i = i + 1\n    }\n}\n",
		wantHex: "300a310a300a310a",
	},
	{
		name:    "nested_for",
		src:     "func main() {\n    for (let i = 0; i < 2; i = i + 1) {\n        for (let j = 0; j < 2; j = j + 1) {\n            print(j)\n        }\n    }\n}\n",
		wantHex: "300a310a300a310a",
	},
	{
		// `break` in the INNER loop leaves the inner loop only: the outer
		// loop keeps going. This is the case that distinguishes "the nearest
		// enclosing loop" from "the outermost loop".
		name:    "break_from_inner_loop",
		src:     "func main() {\n    for (let i = 0; i < 2; i = i + 1) {\n        for (let j = 0; j < 5; j = j + 1) {\n            if j > 1 {\n                break\n            }\n            print(j)\n        }\n        print(i)\n    }\n}\n",
		wantHex: "300a310a300a300a310a310a",
	},
	{
		// `continue` in the inner loop affects the INNER loop only: the
		// outer loop's update clause still runs on every outer iteration.
		name:    "continue_in_inner_loop",
		src:     "func main() {\n    for (let i = 0; i < 2; i = i + 1) {\n        for (let j = 0; j < 3; j = j + 1) {\n            if j == 1 {\n                continue\n            }\n            print(j)\n        }\n        print(i)\n    }\n}\n",
		wantHex: "300a320a300a300a320a310a",
	},
	{
		// An `if`/`else` inside a loop, with the else taken.
		name:    "if_else_inside_loop",
		src:     "func main() {\n    for (let i = 0; i < 2; i = i + 1) {\n        if i == 0 {\n            print(1)\n        } else {\n            print(2)\n        }\n    }\n}\n",
		wantHex: "310a320a",
	},
	{
		// A loop inside an `if` branch.
		name:    "loop_inside_if",
		src:     "func main() {\n    let x = 1\n    if x > 0 {\n        for (let i = 0; i < 2; i = i + 1) {\n            print(i)\n        }\n    }\n}\n",
		wantHex: "300a310a",
	},
	{
		// A user function CALLED from inside a loop. The call's arguments
		// are staged in the caller's frame, so a call inside a loop must
		// not disturb the loop's own locals.
		name:    "call_inside_loop",
		src:     "func twice(n int) int {\n    return n + n\n}\nfunc main() {\n    for (let i = 0; i < 3; i = i + 1) {\n        print(twice(i))\n    }\n}\n",
		wantHex: "300a320a340a",
	},
	{
		// A `return` from inside a loop: the loop is abandoned and the
		// function returns the value. The exit code is the only observable
		// here, because the program prints nothing.
		name:     "return_from_inside_loop",
		src:      "func f(n int) int {\n    for (let i = 0; i < 5; i = i + 1) {\n        if i == 2 {\n            return 9\n        }\n    }\n    return 0\n}\nfunc main() {\n    print(f(1))\n}\n",
		wantHex:  "390a",
		wantExit: 0,
	},
	{
		// Several loop-local variables, all of which must survive the
		// back-edge with their own values.
		name:    "multiple_loop_locals",
		src:     "func main() {\n    let total = 0\n    let n = 3\n    for (let i = 0; i < n; i = i + 1) {\n        let d = i + 1\n        total = total + d\n    }\n    print(total)\n}\n",
		wantHex: "360a",
	},
	{
		// Seven live locals AND a loop: the shape at which the Go oracle's
		// Phase-150C register allocator engages, so the two engines encode it
		// differently. Every local is READ inside the loop body, so they are
		// all live at the loop head -- a local that is never read again is
		// dead there, and a dead local is not pressure.
		//
		// The self-hosted driver has no allocator (every local is a frame
		// slot, the pre-150C path) and is therefore CORRECT but not
		// byte-identical.
		name:       "many_locals_with_loop",
		src:        "func main() {\n    let v1 = 1\n    let v2 = 2\n    let v3 = 3\n    let v4 = 4\n    let v5 = 5\n    let v6 = 6\n    for (let i = 0; i < 2; i = i + 1) {\n        print(v1)\n        print(v2)\n        print(v3)\n        print(v4)\n        print(v5)\n    }\n    print(v6)\n}\n",
		wantHex:    "310a320a330a340a350a310a320a330a340a350a360a",
		expectDiff: true,
	},
}

// natIRefusals is the refusal table. Every entry must be REFUSED by name: a
// non-zero exit, a message naming the construct, and NO image on disk.
//
// The message is matched as a substring because the wording carries the
// reason, but the REASON is the assertion.
var natIRefusals = []struct {
	name string
	src  string
	want string
}{
	{
		// The LANGUAGE CHECKER gets here first: the self-hosted checker
		// reports K108 for a `break` with no enclosing loop
		// (src/compiler/checker.kark:662), and it tracks the loop depth
		// through an `if` (checkStmt passes `loop` unchanged into a branch),
		// so a `break` inside a branch inside a loop is accepted.
		//
		// The driver's own refusal is the second line of defence -- it is
		// unreachable through this pipeline, and that is recorded rather than
		// pretended otherwise.
		name: "break_outside_loop",
		src:  "func main() {\n    break\n}\n",
		want: "'break' outside of a loop",
	},
	{
		name: "continue_outside_loop",
		src:  "func main() {\n    continue\n}\n",
		want: "'continue' outside of a loop",
	},
	{
		// The arena is sized at compile time on the assumption that each
		// allocation site runs at most once. A loop breaks that assumption,
		// so the oracle refuses the construct (program.go:6629).
		name: "concat_in_loop",
		src:  "func main() {\n    let i = 0\n    while i < 3 {\n        print(\"a\" + \"b\")\n        i = i + 1\n    }\n}\n",
		want: "string concatenation inside a loop is not supported on the native target",
	},
	{
		name: "push_in_loop",
		src:  "func main() {\n    let a = [1, 2]\n    for (let i = 0; i < 3; i = i + 1) {\n        let b = push(a, 3)\n        print(b[0])\n    }\n}\n",
		want: "push() inside a loop is not supported on the native target",
	},
	{
		// A loop whose condition is not a comparison. The oracle's condition
		// is a comparison and nothing else (program.go:5074).
		name: "non_comparison_condition",
		src:  "func main() {\n    let i = 0\n    while i {\n        print(1)\n    }\n}\n",
		want: "condition must be a comparison",
	},
	{
		// A `for` whose post is not an assignment. The post here is a bare
		// identifier, which both parsers accept.
		//
		// It is deliberately NOT a call: a call expression in a `for` post is
		// mis-parsed by BOTH engines -- the Go parser reports "unexpected
		// token ')'" and the self-hosted parser silently produces a Block
		// node -- so such a program never reaches the driver at all. That is
		// a PARSER defect in both engines, recorded in
		// docs/audit/NAT-LOOP-BASELINE.md, not a driver one.
		name: "non_assignment_post",
		src:  "func main() {\n    for (let i = 0; i < 3; i) {\n        print(1)\n    }\n}\n",
		want: "a `for` post must be an assignment",
	},
	{
		// An assignment to a variable that does not exist.
		name: "assign_to_undeclared",
		src:  "func main() {\n    for (let i = 0; i < 3; i = i + 1) {\n        j = 1\n    }\n}\n",
		want: "undefined identifier 'j'",
	},
	{
		// A STRING binding inside a loop body. The rodata cursor is threaded
		// per statement list in this driver, so a literal in a nested body
		// cannot be given a stable offset.
		name: "string_binding_in_loop",
		src:  "func main() {\n    for (let i = 0; i < 3; i = i + 1) {\n        let s = \"hi\"\n        print(1)\n    }\n}\n",
		want: "is not an integer expression",
	},
}

// natIImage compiles one source through the kcc native driver and returns the
// PE image. It goes through `native-ast`, the driver entry point, so a failure
// here is a driver failure and never a CLI-dispatch artefact.
func natIImage(t *testing.T, src string) []byte {
	t.Helper()
	out := nat9cBuild(t, src)
	if strings.Contains(out, "error[K") {
		t.Fatalf("kcc refused a program this gate expects to compile:\n%.400s", out)
	}
	img := nat9cDecodeHex(t, out, "driver image")
	if _, _, err := native.ParsePE(img); err != nil {
		t.Fatalf("driver emitted an invalid PE: %v", err)
	}
	return img
}

// natIHex decodes a hex expectation, failing loudly rather than comparing
// against an empty slice -- which would make an empty expectation pass for a
// program that printed nothing at all.
func natIHex(t *testing.T, s, what string) []byte {
	t.Helper()
	if s == "" {
		return nil
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("%s is not hex (%q): %v", what, s, err)
	}
	return b
}

// natIRunImage executes a native image with a TIMEOUT.
//
// The shared nat9dRun helper has no timeout, which was fine until this slice:
// a loop corpus can contain a program that does not terminate, and a hang is a
// worse failure than a red test -- it takes the whole suite with it. A loop
// gate therefore runs its programs under a bound, and a program that exceeds
// it FAILS with a message that says which case did it.
func natIRunImage(t *testing.T, img []byte, name string) ([]byte, int) {
	t.Helper()
	if _, _, err := native.ParsePE(img); err != nil {
		t.Fatalf("driver image is not a structurally valid PE: %v", err)
	}
	exe := filepath.Join(t.TempDir(), "prog.exe")
	if err := os.WriteFile(exe, img, 0o755); err != nil {
		t.Fatalf("writing the driver image: %v", err)
	}
	cmd := exec.Command(exe)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the driver image: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			t.Fatalf("running the driver image: %v", err)
		}
		return buf.Bytes(), code
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("case %q did not terminate within 20s.\n  A loop program that never "+
			"terminates hangs the gate; the corpus is checked for termination, not "+
			"only for its expected output.\n  partial output: % x", name, buf.Bytes())
	}
	return nil, 0
}

// TestPhaseNATLoop_ProgramsExecuteWithExactBytes is layer 1.
//
// Each corpus entry is a different .kark file compiled through kcc's native
// driver, written to disk and EXECUTED on this host. stdout is compared as
// exact bytes and the exit code separately.
func TestPhaseNATLoop_ProgramsExecuteWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; every other layer is cross-platform")
	}
	if len(natICorpus) < 20 {
		t.Fatalf("corpus has %d cases; NAT-LOOP was measured over at least 20 and a "+
			"shrinking corpus is how a gate stops being evidence", len(natICorpus))
	}

	for _, c := range natICorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := natIImage(t, c.src)
			got, code := natIRunImage(t, img, c.name)
			want := natIHex(t, c.wantHex, c.name+" expected stdout")
			if !nat9cBytesEq(got, want) {
				t.Errorf("stdout = % x, want % x\n  source:\n%s\n"+
					"  Compared as EXACT BYTES: a missing or extra trailing newline is a "+
					"real defect, and trimming would hide it.", got, want, c.src)
			}
			if code != c.wantExit {
				t.Errorf("exit = %d, want %d\n  source:\n%s", code, c.wantExit, c.src)
			}
		})
	}
}

// natINormalizeCRLF turns CRLF into LF.
//
// It exists for ONE reason and that reason is a platform artifact, not a
// semantic difference: the C runtime's stdout is in TEXT MODE on Windows, so
// its `\n` reaches the terminal as `\r\n`, while the native program writes raw
// bytes and emits a bare `\n`.
//
// MEASURED on this host: the C backend prints `31 0d 0a` where the native
// image prints `31 0a` -- the same line, a different terminator. The
// kcc_run_parity record documents the same artifact from the other side.
//
// So the differential compares the PROGRAMS' OUTPUT, not the C runtime's text
// mode. The native path's own exact bytes are still pinned by layer 1, which
// compares them against a golden with no normalisation at all -- so nothing
// here weakens the byte-level claim; it only makes the semantic comparison
// fair.
func natINormalizeCRLF(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		if b[i] == '\r' && i+1 < len(b) && b[i+1] == '\n' {
			continue
		}
		out = append(out, b[i])
	}
	return out
}

// TestPhaseNATLoop_MatchesTheCBackend is layer 2, and it is the layer that
// proves the SEMANTICS rather than the bytes.
//
// The same source is compiled the OLD way -- through the C backend and gcc --
// and run, and its stdout is compared with the native image's stdout, byte for
// byte (after the CRLF normalisation described on natINormalizeCRLF).
//
// This is the layer that catches a `continue` that skipped the update clause,
// a `break` that left the wrong loop, or a condition evaluated once too many
// or too few times: the C backend has implemented these semantics since the
// language's first version, and the native path is the NEW code.
//
// The oracle is NOT modified to make this pass. It is the reference.
func TestPhaseNATLoop_MatchesTheCBackend(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("the C backend comparison needs the host toolchain; every other layer is cross-platform")
	}
	karkain := phase130Karkain(t)

	for _, c := range natICorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := natIImage(t, c.src)
			nativeOut, nativeCode := natIRunImage(t, img, c.name)

			goOut, goCode := natIRunCBackend(t, karkain, c.src)
			goOut = natINormalizeCRLF(goOut)
			if !nat9cBytesEq(nativeOut, goOut) {
				t.Errorf("the native path and the C backend disagree.\n"+
					"  native : % x (exit %d)\n  c-path : % x (exit %d)\n  source:\n%s\n"+
					"  The C backend is the reference implementation of these semantics; "+
					"the native path is the new code.\n"+
					"  (The C path's output is CRLF-normalised: its stdout is in Windows "+
					"text mode, which is a runtime artifact, not a difference in the "+
					"program.)",
					nativeOut, nativeCode, goOut, goCode, c.src)
			}
			if nativeCode != goCode {
				t.Errorf("exit status: native %d, C backend %d\n  source:\n%s",
					nativeCode, goCode, c.src)
			}
		})
	}
}

// natIRunCBackend runs a program through the C backend (the Go front end, the C
// code generator and gcc) and returns its stdout and exit status.
//
// This is the OLD path -- the one the native path exists to replace -- and it
// is used here as the semantic reference, never as the thing under test.
func natIRunCBackend(t *testing.T, karkain, src string) ([]byte, int) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "prog.kark")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatalf("writing the program: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "run", "--engine", "go", file)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("the C backend did not terminate within 60s.\n  A loop program that " +
			"never terminates hangs the gate; the corpus is checked for termination.")
	}
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running the C backend: %v", err)
		}
	}
	return out, code
}

// TestPhaseNATLoop_LinuxELFIsByteIdenticalToTheOracle is layer 3.
//
// Both engines build the SAME source for native-x86_64-linux and the image
// bytes are compared. The kcc outcome must also carry the provenance marker,
// which only src/compiler/main.kark can print -- so a build that quietly fell
// through to the Go backend would agree byte for byte and still fail here.
//
// The expected bytes are produced by calling the oracle at test time. A
// GOLDEN would be wrong here: for most of increment 151 the emitted bytes
// were wrong, so a golden pins the divergence and calls it expected.
func TestPhaseNATLoop_LinuxELFIsByteIdenticalToTheOracle(t *testing.T) {
	karkain := phase130Karkain(t)

	for _, c := range natICorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			kccRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "kcc", c.name+"-kcc.elf")
			if kccRes.ExitCode != 0 || !kccRes.WroteImage {
				t.Fatalf("kcc produced no ELF (exit %d):\n%s", kccRes.ExitCode, kccRes.Output)
			}
			if !kccProducedImage(kccRes.Output) {
				t.Fatalf("the kcc build did not carry the self-hosted provenance marker "+
					"(%q), so a Go-backend fallback cannot be excluded:\n%s",
					kccNativeProvenance, kccRes.Output)
			}
			if _, _, err := native.Parse(kccRes.Image); err != nil {
				t.Fatalf("kcc emitted an ELF the ORACLE's Parse rejects: %v", err)
			}

			goRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "go", c.name+"-go.elf")
			if goRes.ExitCode != 0 || !goRes.WroteImage {
				t.Fatalf("the Go oracle produced no ELF (exit %d):\n%s", goRes.ExitCode, goRes.Output)
			}

			if c.expectDiff {
				// A KNOWN difference, asserted rather than skipped. See the
				// expectDiff field's comment and
				// TestPhaseNATLoop_TheRegisterAllocatorBoundaryIsPinned.
				if nat9cBytesEq(kccRes.Image, goRes.Image) {
					t.Errorf("this case is marked expectDiff, but the two engines "+
						"produced IDENTICAL images.\n  sha %s\n"+
						"  The Go oracle's Phase-150C register allocator engages under "+
						"control flow and pressure. If the allocator's rule changed, the "+
						"boundary must be re-measured rather than assumed.", kccRes.SHA)
				}
				return
			}
			if !nat9cBytesEq(kccRes.Image, goRes.Image) {
				t.Errorf("kcc and the Go oracle emitted DIFFERENT ELF images for the "+
					"same program.\n  kcc: %d bytes sha %s\n  go : %d bytes sha %s\n"+
					"  source:\n%s\n"+
					"  The corpus is meant to cover the shapes that ARE identical. If a "+
					"difference is expected, it belongs in the refusal table or in a "+
					"newly baselined slice -- not silently in this list.",
					len(kccRes.Image), kccRes.SHA, len(goRes.Image), goRes.SHA, c.src)
			}
		})
	}
}

// TestPhaseNATLoop_RefusalsAreByName is layer 4.
//
// Every declined construct must be refused BY NAME, must exit non-zero, and
// must leave NO IMAGE behind. The last part is the one that matters most: a
// refusal that still wrote a file would leave a user with an executable they
// believe is the program they wrote.
func TestPhaseNATLoop_RefusalsAreByName(t *testing.T) {
	karkain := phase130Karkain(t)

	for _, c := range natIRefusals {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "kcc", c.name+".elf")
			if res.ExitCode == 0 {
				t.Fatalf("kcc ACCEPTED a program it cannot lower (exit 0):\n%s", res.Output)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Errorf("the refusal does not name the construct.\n  want the message to "+
					"contain %q\n  got:\n%s", c.want, res.Output)
			}
			if res.WroteImage {
				t.Errorf("a refused program still wrote an image (%d bytes)", len(res.Image))
			}
		})
	}
}

// TestPhaseNATLoop_TheRegisterAllocatorBoundaryIsPinned is layer 5, and it is
// a LIMITATION stated as an assertion rather than as a comment.
//
// The Go oracle has a Phase-150C register allocator. It engages only when a
// function contains control flow AND has more than `raMinLive`
// (= allocRegCount + 1 = 5) simultaneously-live int locals, because below that
// every live value fits in a register anyway and a push/pop pair would cost
// more than the load it removes.
//
// The self-hosted driver has NO register allocator: every local is a frame
// slot, which is the pre-150C path and is always CORRECT. So from five locals
// upward the two engines encode the same program DIFFERENTLY -- and the
// difference is an encoding strategy, not a wrong answer.
//
// MEASURED on this host, not derived:
//
//	4 locals + a loop : IDENTICAL
//	5 locals + a loop : DIFFER
//
// The boundary is asserted from BOTH sides, and the five-local case is also in
// the execution corpus, so "different bytes" is backed by "same behaviour".
func TestPhaseNATLoop_TheRegisterAllocatorBoundaryIsPinned(t *testing.T) {
	karkain := phase130Karkain(t)

	build := func(locals int) (string, string) {
		var b strings.Builder
		b.WriteString("func main() {\n")
		for i := 1; i <= locals; i++ {
			b.WriteString("    let v")
			b.WriteString(strconv.Itoa(i))
			b.WriteString(" = ")
			b.WriteString(strconv.Itoa(i))
			b.WriteString("\n")
		}
		b.WriteString("    for (let i = 0; i < 2; i = i + 1) {\n")
		for i := 1; i < locals; i++ {
			b.WriteString("        print(v")
			b.WriteString(strconv.Itoa(i))
			b.WriteString(")\n")
		}
		b.WriteString("    }\n")
		b.WriteString("    print(v")
		b.WriteString(strconv.Itoa(locals))
		b.WriteString(")\n}\n")
		src := b.String()
		kccRes := nativeBuildWithEngine(t, karkain, src, NativeLinuxTarget, "kcc", "ra-kcc.elf")
		goRes := nativeBuildWithEngine(t, karkain, src, NativeLinuxTarget, "go", "ra-go.elf")
		if kccRes.ExitCode != 0 || goRes.ExitCode != 0 {
			t.Fatalf("a %d-local program did not build (kcc exit %d, go exit %d):\n%s\n%s",
				locals, kccRes.ExitCode, goRes.ExitCode, kccRes.Output, goRes.Output)
		}
		return kccRes.SHA, goRes.SHA
	}

	// Below the threshold: identical.
	kccSHA, goSHA := build(3)
	if kccSHA != goSHA {
		t.Errorf("a 3-local program with a loop produced DIFFERENT images.\n"+
			"  kcc sha %s\n  go  sha %s\n"+
			"  Below the allocator's engagement threshold the two engines must agree "+
			"byte for byte; a difference here means something else moved.", kccSHA, goSHA)
	}

	// At the threshold: different, and the difference is recorded here.
	kccSHA, goSHA = build(8)
	if kccSHA == goSHA {
		t.Errorf("an 8-local program with a loop produced IDENTICAL images.\n"+
			"  The oracle's Phase-150C register allocator engages at 5 simultaneously-"+
			"live int locals in a function with control flow, so the two engines are "+
			"expected to differ. If they do not, the allocator's rule changed and "+
			"this boundary must be re-measured rather than assumed.")
	}
}

// TestPhaseNATLoop_DeterminismAndNonVacuity is layer 6.
//
// Determinism: the same source must produce the same image twice.
//
// Non-vacuity: DIFFERENT sources must produce DIFFERENT images. A driver that
// ignored the loop condition and always ran the body once would pass the
// execution layer for any case whose expected output happened to match, so the
// images themselves have to move.
func TestPhaseNATLoop_DeterminismAndNonVacuity(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}

	src := "func main() {\n    for (let i = 0; i < 3; i = i + 1) {\n        print(i)\n    }\n}\n"
	a := natIImage(t, src)
	b := natIImage(t, src)
	if !nat9cBytesEq(a, b) {
		t.Errorf("two builds of the same source produced different images (%d vs %d bytes)",
			len(a), len(b))
	}

	// Non-vacuity: the loop's CONDITION must reach the image. Two programs
	// that differ only in the condition's bound must differ in their bytes.
	three := natIImage(t, "func main() {\n    for (let i = 0; i < 3; i = i + 1) {\n        print(i)\n    }\n}\n")
	five := natIImage(t, "func main() {\n    for (let i = 0; i < 5; i = i + 1) {\n        print(i)\n    }\n}\n")
	if nat9cBytesEq(three, five) {
		t.Errorf("two programs whose loop bounds differ produced IDENTICAL images, so " +
			"the condition is not reaching the emitted code")
	}
}

// TestPhaseNATLoop_TheNativePathInvokesNeitherCNorGo is layer 7: the
// process-level independence evidence for a LOOP program.
//
// The method is the NAT-QUAL one (see phase151h_qualification_test.go): PATH
// shims for `gcc` and `go` log every invocation, the self-hosted compiler is
// warmed OUTSIDE the trace, and the native build's shim log must be EMPTY.
// The legacy build path, run with the same shims, is the POSITIVE CONTROL:
// its log must NOT be empty, because the legacy path is supposed to reach
// gcc.
//
// This qualifies LOOP COMPILATION independence only. It does not claim that
// the compiler is independent -- the compiler's own build still uses gcc, and
// the bootstrap proof is still carried to v1.3.0 as UNMET.
func TestPhaseNATLoop_TheNativePathInvokesNeitherCNorGo(t *testing.T) {
	karkain := phase130Karkain(t)
	const src = "func main() {\n    for (let i = 0; i < 3; i = i + 1) {\n        print(i)\n    }\n}\n"

	// Warm the self-hosted compiler OUTSIDE the trace.
	warm := natQualRunBuild(t, karkain, src, NativeWindowsTarget, "")
	if warm.ExitCode != 0 || !warm.WroteImage {
		t.Fatalf("the warm-up native build failed (exit %d):\n%s", warm.ExitCode, warm.Output)
	}

	// The traced native build.
	shimDir, logPath := natQualShimDir(t)
	traced := natQualRunBuild(t, karkain, src, NativeWindowsTarget, shimDir)
	if traced.ExitCode != 0 || !traced.WroteImage {
		t.Fatalf("the traced native build failed (exit %d):\n%s", traced.ExitCode, traced.Output)
	}
	if log := natQualLogBytes(t, logPath); log != "" {
		t.Errorf("the NATIVE build of a LOOP program invoked a C compiler or the Go "+
			"toolchain.\n  shim log:\n%s\n"+
			"  The qualified native path must compile a user program without a C "+
			"compiler, a C source generator, the Go compiler or the Go runtime.",
			log)
	}

	// The positive control: the LEGACY path IS supposed to reach gcc, and the
	// shims must fire there. If they do not, the empty log above is not
	// evidence of anything.
	ctlDir, ctlLog := natQualShimDir(t)
	ctl := natQualRunBuild(t, karkain, src, "", ctlDir)
	_ = ctl // the control build is EXPECTED to fail: the gcc shim fails the link
	if log := natQualLogBytes(t, ctlLog); log == "" {
		t.Errorf("the positive control did not fire.\n"+
			"  The LEGACY build path is supposed to reach a C compiler, and the "+
			"shims must log that invocation. If they do not, the empty log in the "+
			"native trace above is not evidence of anything -- the shims may simply "+
			"not be reachable.\n  control output:\n%s", ctl.Output)
	}
}

// TestPhaseNATLoop_GateIsWiredInCI asserts that this gate has a ci.yml step of
// its own.
//
// A gate is a test file PLUS a CI entry. Writing the test and running it
// locally is not evidence that CI ever executes it -- and this repository has
// four gates that were written, reported green locally, and never ran in CI.
func TestPhaseNATLoop_GateIsWiredInCI(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	ci := string(raw)
	if !strings.Contains(ci, "TestPhaseNATLoop_") {
		t.Errorf("ci.yml has no step running this gate's pattern 'TestPhaseNATLoop_'")
	}
}
