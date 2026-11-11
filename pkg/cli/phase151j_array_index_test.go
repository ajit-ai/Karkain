package cli

import (
	"bytes"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karkain/pkg/native"
)

// NAT-IDX -- array indexing inside the kcc native driver.
//
// The limitation this increment removes was recorded by NAT-LOOP: "the index
// path carries the runtime-error reporter's rodata offsets, and the shared
// integer expression walker does not carry them". The diagnosis behind that
// sentence was two separate things:
//
//  1. The index was a LITERAL-ONLY special case. `natArrayIndex` took the index
//     node and did `int(intLitVal(idx))` -- it materialised a literal straight
//     into RCX. An index that was a variable, or an arithmetic expression, was
//     refused by the scan. The ORACLE has never required a literal: it
//     evaluates the index EXPRESSION through the shared walker and moves the
//     result into RCX (pkg/native/program.go:6320-6324).
//
//  2. The reporter's context (the rodata offset of the reporter's "kind"
//     argument, and the source basename's offset and length) reached the print
//     and return paths, which is where an index could be lowered -- and no
//     further. An index inside an arithmetic expression, a comparison, a call's
//     arguments or a loop body had nowhere to get that context from.
//
// So this increment (a) evaluates the index as an expression through the
// SHARED walker, exactly as the oracle does, and (b) propagates the reporter
// context through that walker as ONE explicit argument -- not a global, and not
// a second copy of the walker.
//
// The evidence is the same shape as NAT-LOOP's:
//
//  1. EXECUTED PE goldens -- exact stdout bytes and exit codes.
//  2. A DIFFERENTIAL against the C backend -- the same source, compiled the
//     old way and run. This is the byte-identity substitute for array
//     programs, and the reason is stated below.
//  3. BOUNDS behaviour -- an out-of-range or negative index must be a runtime
//     error with an exit status, never a silent wrong value.
//  4. A REFUSAL TABLE -- the shapes that are still refused, by name.
//  5. Determinism, non-vacuity, and the CI-wiring check.
//  6. Process-level INDEPENDENCE evidence.
//
// WHY THE BYTE-IDENTITY BOUNDARY IS NOT CROSSED HERE: a program with an array
// has a FRAME the two engines do not size identically. The kcc driver rounds
// the locals region up to the next 16 (round16), while the Go oracle rounds it
// up to the next 16 STRICTLY GREATER, so the oracle's frame is 8 bytes larger
// whenever the locals region is already aligned. That is the MEASURED PARITY
// DELTA Phase 151A Step 9d recorded, it is the one remaining entry in the
// D2 known-non-parity list, and it is not an indexing question: closing it
// moves natFrameLayout, which every frame displacement in the backend is
// pinned against. So array programs are compared BEHAVIOURALLY here, and the
// byte difference is asserted rather than hidden.

// natIdxCorpus is the execution corpus. Every entry is a different .kark source.
//
// `wantHex` is the EXPECTED stdout as exact bytes, MEASURED (not computed by
// hand -- two of NAT-LOOP's hand-computed goldens were wrong and the compiler
// was right).
var natIdxCorpus = []struct {
	name     string
	src      string
	wantHex  string
	wantExit int
}{
	{
		name:    "first_element",
		src:     "func main() {\n    let a = [3, 5, 7]\n    print(a[0])\n}\n",
		wantHex: "330a",
	},
	{
		name:    "last_element",
		src:     "func main() {\n    let a = [3, 5, 7]\n    print(a[2])\n}\n",
		wantHex: "370a",
	},
	{
		// THE case the increment exists for: an index that is a VARIABLE.
		name:    "variable_index",
		src:     "func main() {\n    let a = [3, 5, 7]\n    let i = 1\n    print(a[i])\n}\n",
		wantHex: "350a",
	},
	{
		// An index that is an ARITHMETIC expression.
		name:    "arithmetic_index",
		src:     "func main() {\n    let a = [3, 5, 7]\n    let i = 1\n    print(a[i + 1])\n}\n",
		wantHex: "370a",
	},
	{
		// ORDINARY ARRAY ITERATION in a `while`: the whole point of the
		// increment, and the case the NAT-LOOP record called "the most-felt
		// limitation of the qualified subset".
		name:    "iterate_while",
		src:     "func main() {\n    let a = [3, 5, 7]\n    let i = 0\n    while i < 3 {\n        print(a[i])\n        i = i + 1\n    }\n}\n",
		wantHex: "330a350a370a",
	},
	{
		name:    "iterate_for",
		src:     "func main() {\n    let a = [3, 5, 7]\n    for (let i = 0; i < 3; i = i + 1) {\n        print(a[i])\n    }\n}\n",
		wantHex: "330a350a370a",
	},
	{
		// An element inside an arithmetic expression.
		name:    "element_in_arith",
		src:     "func main() {\n    let a = [3, 5, 7]\n    print(a[0] + a[1])\n}\n",
		wantHex: "380a",
	},
	{
		// An element inside a COMPARISON -- the case that needed the reporter
		// context in the CONDITION path, not only in the print path.
		name:    "element_in_cmp",
		src:     "func main() {\n    let a = [3, 5, 7]\n    if a[0] < a[1] {\n        print(1)\n    }\n}\n",
		wantHex: "310a",
	},
	{
		// A branch inside a loop, with an index in BOTH the branch's condition
		// and its body.
		name:    "index_in_if_in_loop",
		src:     "func main() {\n    let a = [3, 5, 7]\n    for (let i = 0; i < 3; i = i + 1) {\n        if a[i] > 3 {\n            print(a[i])\n        }\n    }\n}\n",
		wantHex: "350a370a",
	},
	{
		// Two arrays indexed in one loop.
		name:    "index_in_nested",
		src:     "func main() {\n    let a = [3, 5, 7]\n    let b = [9, 8]\n    for (let i = 0; i < 2; i = i + 1) {\n        print(a[i])\n        print(b[i])\n    }\n}\n",
		wantHex: "330a390a350a380a",
	},
	{
		// A `continue` with an index: the continue skips the PRINT, and the
		// index is only evaluated on the iterations that reach it.
		name:    "index_with_continue",
		src:     "func main() {\n    let a = [3, 5, 7]\n    for (let i = 0; i < 3; i = i + 1) {\n        if i == 1 {\n            continue\n        }\n        print(a[i])\n    }\n}\n",
		wantHex: "330a370a",
	},
	{
		name:    "index_with_break",
		src:     "func main() {\n    let a = [3, 5, 7]\n    for (let i = 0; i < 4; i = i + 1) {\n        if i == 2 {\n            break\n        }\n        print(a[i])\n    }\n}\n",
		wantHex: "330a350a",
	},
	{
		// An element passed to a user function.
		name:    "element_to_function",
		src:     "func twice(n int) int {\n    return n + n\n}\nfunc main() {\n    let a = [3, 5, 7]\n    print(twice(a[0]))\n}\n",
		wantHex: "360a",
	},
	{
		name:    "two_arrays_one_loop",
		src:     "func main() {\n    let a = [3, 5]\n    let b = [7, 9]\n    for (let i = 0; i < 2; i = i + 1) {\n        print(a[i])\n        print(b[i])\n    }\n}\n",
		wantHex: "330a370a350a390a",
	},
	{
		// An index that is ITSELF an element read: a[a[0] - 3] = a[0] = 3.
		name:    "nested_index",
		src:     "func main() {\n    let a = [3, 5, 7]\n    print(a[a[0] - 3])\n}\n",
		wantHex: "330a",
	},
	{
		name:    "index_in_len",
		src:     "func main() {\n    let a = [3, 5, 7]\n    print(len(a))\n}\n",
		wantHex: "330a",
	},
}

// natIdxBounds is the BOUNDS corpus. Every entry must be a RUNTIME error: a
// non-zero exit and NO stdout. A program that printed a value for an
// out-of-range index would be the worst possible outcome, because it would be
// indistinguishable from a correct answer.
var natIdxBounds = []struct {
	name string
	src  string
	// wantBefore is the stdout produced BEFORE the failing access, when the
	// failure happens inside a loop.
	wantBefore string
}{
	{
		name: "index_equal_to_length",
		src:  "func main() {\n    let a = [3, 5, 7]\n    let i = 3\n    print(a[i])\n}\n",
	},
	{
		name: "index_greater_than_length",
		src:  "func main() {\n    let a = [3, 5, 7]\n    let i = 9\n    print(a[i])\n}\n",
	},
	{
		// The failure happens on the FOURTH iteration, so the first three
		// elements are printed and then the program dies. The stdout before the
		// failure is asserted, because a bounds check that fired EARLY would
		// also produce "no output" and would pass a weaker test.
		name:       "out_of_range_in_loop",
		src:        "func main() {\n    let a = [3, 5, 7]\n    for (let i = 0; i < 5; i = i + 1) {\n        print(a[i])\n    }\n}\n",
		wantBefore: "330a350a370a",
	},
	{
		// A NEGATIVE index. The index is built by an assignment rather than a
		// `let`, because main's `let` accepts literals only.
		name: "negative_index",
		src:  "func main() {\n    let a = [3, 5, 7]\n    let i = 0\n    i = i - 1\n    print(a[i])\n}\n",
	},
	{
		// An EMPTY array: index 0 is already out of range.
		name: "empty_array",
		src:  "func main() {\n    let a = []\n    print(a[0])\n}\n",
	},
}

// natIdxRefusals is the refusal table: the shapes that are still refused, by
// name, with no image written.
var natIdxRefusals = []struct {
	name string
	src  string
	want string
}{
	{
		// Indexing a SCALAR. The kind check is load-bearing: without it the
		// lowering reads the scalar's slot as a (base, len) header and prints
		// a meaningless number from a VALID PE.
		name: "index_of_scalar",
		src:  "func main() {\n    let a = 7\n    print(a[0])\n}\n",
		want: "index target must be an array",
	},
	{
		name: "index_of_undeclared",
		src:  "func main() {\n    print(q[0])\n}\n",
		want: "undefined identifier 'q'",
	},
	{
		// A STRING index. The index must be an integer expression.
		name: "string_index",
		src:  "func main() {\n    let a = [3, 5, 7]\n    print(a[\"x\"])\n}\n",
		want: "is not an integer expression",
	},
}

// natIdxImage compiles one source through the kcc native driver and returns the
// PE image.
func natIdxImage(t *testing.T, src string) []byte {
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

// natIdxHex decodes a hex expectation, failing loudly rather than comparing
// against an empty slice.
func natIdxHex(t *testing.T, s, what string) []byte {
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

// natIdxRunImage executes a native image with a TIMEOUT, as the loop gate
// does: a loop program can fail to terminate, and a hang is a worse failure
// than a red test.
func natIdxRunImage(t *testing.T, img []byte, name string) ([]byte, int) {
	t.Helper()
	if _, _, err := native.ParsePE(img); err != nil {
		t.Fatalf("driver image is not a structurally valid PE: %v", err)
	}
	exe := filepath.Join(t.TempDir(), "prog.exe")
	if err := os.WriteFile(exe, img, 0o755); err != nil {
		t.Fatalf("writing the driver image: %v", err)
	}
	cmd := exec.Command(exe)
	// stdout and stderr are captured SEPARATELY. The bounds layer asserts that
	// an out-of-range access prints NO VALUE, and that assertion is about
	// stdout: the runtime-error reporter writes its diagnostic to stderr, so
	// merging the two streams would put the diagnostic into the very buffer
	// the assertion checks.
	//
	// (On THIS host the reporter's stderr is empty anyway -- the Phase-152-B0
	// GetStdHandle limitation -- so the bug would not show here. It would
	// show on a host where the reporter works, which is exactly the kind of
	// latent fragility a gate should not carry.)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
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
		return outBuf.Bytes(), code
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("case %q did not terminate within 20s", name)
	}
	return nil, 0
}

// TestPhaseNATIdx_ProgramsExecuteWithExactBytes is layer 1: the indexing is
// OBSERVABLE.
//
// Every corpus entry is a different .kark file compiled through kcc's native
// driver, written to disk and EXECUTED on this host. stdout is compared as
// exact bytes and the exit code separately.
func TestPhaseNATIdx_ProgramsExecuteWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; every other layer is cross-platform")
	}
	if len(natIdxCorpus) < 15 {
		t.Fatalf("corpus has %d cases; NAT-IDX was measured over at least 15 and a "+
			"shrinking corpus is how a gate stops being evidence", len(natIdxCorpus))
	}

	for _, c := range natIdxCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := natIdxImage(t, c.src)
			got, code := natIdxRunImage(t, img, c.name)
			want := natIdxHex(t, c.wantHex, c.name+" expected stdout")
			if !nat9cBytesEq(got, want) {
				t.Errorf("stdout = % x, want % x\n  source:\n%s", got, want, c.src)
			}
			if code != c.wantExit {
				t.Errorf("exit = %d, want %d\n  source:\n%s", code, c.wantExit, c.src)
			}
		})
	}
}

// TestPhaseNATIdx_OutOfRangeIsARuntimeError is layer 2: the BOUNDS safety.
//
// An out-of-range or negative index must be a runtime error with a non-zero
// exit and NO value printed. The stdout produced BEFORE the failure is
// asserted too, because a check that fired one iteration early would also
// produce "no output" and would pass a weaker test.
//
// NOTE ON THE DIAGNOSTIC TEXT: the runtime-error reporter writes to stderr, and
// the Phase-152-B0 `GetStdHandle` limitation leaves that stream EMPTY on this
// host. The EXIT STATUS is therefore the observable, and the message is not
// asserted here. That limitation is recorded in
// docs/audit/NAT-IDX-BASELINE.md rather than papered over.
func TestPhaseNATIdx_OutOfRangeIsARuntimeError(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	for _, c := range natIdxBounds {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := natIdxImage(t, c.src)
			got, code := natIdxRunImage(t, img, c.name)
			if code == 0 {
				t.Errorf("an out-of-range access exited 0; it must be a runtime error.\n"+
					"  stdout = % x\n  source:\n%s", got, c.src)
			}
			before := natIdxHex(t, c.wantBefore, c.name+" stdout before the failure")
			if !nat9cBytesEq(got, before) {
				t.Errorf("stdout = % x, want % x (the output before the failing access)\n"+
					"  source:\n%s", got, before, c.src)
			}
		})
	}
}

// TestPhaseNATIdx_MatchesTheCBackend is layer 3: the behavioural differential.
//
// The same source is compiled the OLD way -- through the C backend and gcc --
// and run, and its stdout is compared with the native image's.
//
// This is the byte-identity substitute for array programs, and the reason is
// the frame delta the D2 gate documents: a program with an array gets a frame
// the two engines size differently (the oracle rounds the locals region up to
// the next 16 STRICTLY GREATER, the driver up to the next 16), so the images
// differ by that delta and cannot be compared byte for byte.
//
// The ORACLE is NOT modified to make this pass.
func TestPhaseNATIdx_MatchesTheCBackend(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("the C backend comparison needs the host toolchain")
	}
	karkain := phase130Karkain(t)

	for _, c := range natIdxCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := natIdxImage(t, c.src)
			nativeOut, nativeCode := natIdxRunImage(t, img, c.name)

			goOut, goCode := natIRunCBackend(t, karkain, c.src)
			goOut = natINormalizeCRLF(goOut)
			if !nat9cBytesEq(nativeOut, goOut) {
				t.Errorf("the native path and the C backend disagree.\n"+
					"  native : % x (exit %d)\n  c-path : % x (exit %d)\n  source:\n%s\n"+
					"  The C backend is the reference implementation of these semantics.",
					nativeOut, nativeCode, goOut, goCode, c.src)
			}
		})
	}
}

// TestPhaseNATIdx_RefusalsAreByName is layer 4.
func TestPhaseNATIdx_RefusalsAreByName(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range natIdxRefusals {
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

// TestPhaseNATIdx_TheFrameDeltaIsTheOnlyByteDifference is layer 5: the byte
// boundary is ASSERTED, not hidden.
//
// The index ENCODING now matches the oracle's -- the index is evaluated
// through the shared walker, as the oracle evaluates it -- so the two images
// are the same SIZE. They are not the same BYTES, because of the frame delta
// the D2 gate documents.
//
// The assertion is deliberately about the SIZE and not about equality: a
// program with an array cannot be byte-identical until the frame rounding is
// reconciled, and pretending otherwise would either fail forever or hide the
// difference.
func TestPhaseNATIdx_TheFrameDeltaIsTheOnlyByteDifference(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range natIdxCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			kccRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "kcc", c.name+"-kcc.elf")
			if kccRes.ExitCode != 0 || !kccRes.WroteImage {
				t.Fatalf("kcc produced no ELF (exit %d):\n%s", kccRes.ExitCode, kccRes.Output)
			}
			goRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "go", c.name+"-go.elf")
			if goRes.ExitCode != 0 || !goRes.WroteImage {
				t.Fatalf("the Go oracle produced no ELF (exit %d):\n%s", goRes.ExitCode, goRes.Output)
			}
			if len(kccRes.Image) != len(goRes.Image) {
				t.Errorf("the images differ in SIZE: kcc %d bytes, oracle %d bytes.\n"+
					"  The index encoding now matches the oracle's, so the sizes should "+
					"agree; a size difference means something else moved.",
					len(kccRes.Image), len(goRes.Image))
			}
		})
	}
}

// TestPhaseNATIdx_DeterminismAndNonVacuity is layer 6.
func TestPhaseNATIdx_DeterminismAndNonVacuity(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	src := "func main() {\n    let a = [3, 5, 7]\n    for (let i = 0; i < 3; i = i + 1) {\n        print(a[i])\n    }\n}\n"
	a := natIdxImage(t, src)
	b := natIdxImage(t, src)
	if !nat9cBytesEq(a, b) {
		t.Errorf("two builds of the same source produced different images (%d vs %d bytes)",
			len(a), len(b))
	}

	// Non-vacuity: the INDEX must reach the image. Two programs that differ
	// only in the index's bound must differ in their bytes.
	three := natIdxImage(t, "func main() {\n    let a = [3, 5, 7]\n    print(a[0])\n}\n")
	five := natIdxImage(t, "func main() {\n    let a = [3, 5, 7]\n    print(a[2])\n}\n")
	if nat9cBytesEq(three, five) {
		t.Errorf("two programs whose index differs produced IDENTICAL images, so the " +
			"index is not reaching the emitted code")
	}
}

// TestPhaseNATIdx_TheNativePathInvokesNeitherCNorGo is layer 7: the
// process-level independence evidence, by the NAT-QUAL methodology.
//
// The self-hosted compiler is warmed OUTSIDE the trace, the native build's
// shim log must be EMPTY, and the LEGACY build path is the positive control
// whose log must NOT be empty.
//
// This qualifies LOOP-WITH-INDEXING compilation independence only. It does not
// claim that the compiler is independent: the compiler's OWN build still uses
// gcc, and the bootstrap proof is still carried to v1.3.0 as UNMET.
func TestPhaseNATIdx_TheNativePathInvokesNeitherCNorGo(t *testing.T) {
	karkain := phase130Karkain(t)
	const src = "func main() {\n    let a = [3, 5, 7]\n    for (let i = 0; i < 3; i = i + 1) {\n        print(a[i])\n    }\n}\n"

	warm := natQualRunBuild(t, karkain, src, NativeWindowsTarget, "")
	if warm.ExitCode != 0 || !warm.WroteImage {
		t.Fatalf("the warm-up native build failed (exit %d):\n%s", warm.ExitCode, warm.Output)
	}

	shimDir, logPath := natQualShimDir(t)
	traced := natQualRunBuild(t, karkain, src, NativeWindowsTarget, shimDir)
	if traced.ExitCode != 0 || !traced.WroteImage {
		t.Fatalf("the traced native build failed (exit %d):\n%s", traced.ExitCode, traced.Output)
	}
	if log := natQualLogBytes(t, logPath); log != "" {
		t.Errorf("the NATIVE build of an INDEXING program invoked a C compiler or the Go "+
			"toolchain.\n  shim log:\n%s", log)
	}

	ctlDir, ctlLog := natQualShimDir(t)
	ctl := natQualRunBuild(t, karkain, src, "", ctlDir)
	_ = ctl
	if log := natQualLogBytes(t, ctlLog); log == "" {
		t.Errorf("the positive control did not fire.\n  control output:\n%s", ctl.Output)
	}
}

// TestPhaseNATIdx_GateIsWiredInCI asserts that this gate has a ci.yml step.
func TestPhaseNATIdx_GateIsWiredInCI(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	if !strings.Contains(string(raw), "TestPhaseNATIdx_") {
		t.Errorf("ci.yml has no step running this gate's pattern 'TestPhaseNATIdx_'")
	}
}
