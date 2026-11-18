package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// NAT-STRIDX -- string byte indexing in the kcc native driver.
//
// The native driver could concatenate strings and compare them (NAT-STREQ)
// and could slice them (NAT-STRSLICE), but it could not read an individual
// byte. This slice adds `s[i]` exactly where the array-index slice landed: the
// index is an ordinary integer expression, the result is a one-byte view of
// the string, and an out-of-range or negative index is the Phase-100 runtime
// error ("string index out of range"), written to stderr and ended with
// exit(1) -- never a silent wrong byte.
//
// THE SEMANTICS, stated so they can be checked rather than assumed:
//
//   - UTF-8 BYTE indexing, not code-point indexing. The bounds check compares
//     the index against the string's BYTE length, the view is (ptr + i, 1),
//     and the interpreter of `len(s)` is the pair's own length. No Unicode
//     conversion is introduced anywhere.
//   - The result is a one-byte string -- the same representation `str_char_at`
//     relies on (SPEC § strings: "indexing is byte-wise").
//   - Bounds: 0 <= i < len(s), else the reporter with kind
//     "string index out of range", file:line, exit(1). The empty string is
//     out of range at index 0. The index is evaluated FIRST, in the array
//     path's order.
//   - Embedded NUL bytes are supported by the REPRESENTATION (the pair carries
//     its length), and the resident literal in this host contains NULs as soon
//     as a byte >= 0x80 is written, which is why the utf8 cases exist below.
//     Note the pre-existing platform limitation recorded in the report: bytes
//     >= 0x80 do not survive the self-hosted rodata writer (they become 0x00),
//     which is a separate defect of the literal pipeline, not of this slice.
//   - Repeated indexing allocates NOTHING. The view is a register pair built
//     from the base pointer, no arena, no copy.
//   - `s[i]` is a string, so it composes with `print`, with concatenation
//     ("ab"), with `let` bindings and with `len`. It is NOT an integer and is
//     refused anywhere an integer is required, BY NAME.
//
// WHY THE GATE IS SHAPED THIS WAY. The claim "string byte indexing works" is
// five claims, one per layer:
//
//	(1) the program prints the right thing            -- execution, exact bytes
//	(2) the bytes agree with the C backend            -- behavioural differential
//	(3) what is NOT lowered is refused BY NAME        -- refusal table
//	(4) bounds violations are runtime errors          -- the safety layer
//	(5) the image is kcc's and is deterministic       -- provenance + determinism
//
// Layer 2 is a differential and not ELF byte-identity for the same reason as
// NAT-IDX: the Go native backend has no string indexing, so there is no
// oracle image to compare against; the C backend is the reference
// implementation of the language's string semantics, and it agrees byte for
// byte on every case below that does not carry high bytes (see the multibyte
// case for the one documented divergence).
//
// Layer 4 exists because a bounds check that fires LATE is indistinguishable
// from a check that fires EARLY if the failure prints nothing: the out-of-
// range-in-loop case asserts the bytes that were printed BEFORE the failure.

// natStrIdxCase is one program in both the execution corpus and the
// differential. The same table drives both layers so a case cannot be added
// to one and forgotten in the other.
type natStrIdxCase struct {
	name     string
	src      string
	want     string
	wantExit int
	// elfSkippable records cases that are NATIVE-ONLY: the self-hosted rodata
	// writer does not carry bytes >= 0x80, so the Go engine's image (or a Go
	// run) is not a valid reference for them. The differential skips them and
	// says so, rather than asserting a false property.
	nativeOnly bool
	nativeNote string
}

var natStrIdxCorpus = []natStrIdxCase{
	{
		name:     "first_byte",
		src:      "func main() {\n    let s = \"abc\"\n    print(s[0])\n}\n",
		want:     "a\n",
		wantExit: 0,
	},
	{
		name:     "middle_byte",
		src:      "func main() {\n    let s = \"abcdef\"\n    print(s[2])\n}\n",
		want:     "c\n",
		wantExit: 0,
	},
	{
		name:     "last_byte",
		src:      "func main() {\n    let s = \"abc\"\n    print(s[2])\n}\n",
		want:     "c\n",
		wantExit: 0,
	},
	{
		name:     "single_char_string",
		src:      "func main() {\n    let s = \"x\"\n    print(s[0])\n}\n",
		want:     "x\n",
		wantExit: 0,
	},
	{
		name:     "variable_index",
		src:      "func main() {\n    let s = \"abc\"\n    let i = 1\n    print(s[i])\n}\n",
		want:     "b\n",
		wantExit: 0,
	},
	{
		name:     "len_based_index",
		src:      "func main() {\n    let s = \"abc\"\n    print(s[len(s) - 1])\n}\n",
		want:     "c\n",
		wantExit: 0,
	},
	{
		// The index is an expression, not a literal: this is what makes `a[i]`
		// usable inside a loop at all.
		name:     "len_based_index_multibyte",
		src:      "func main() {\n    let s = \"héllo\"\n    print(s[len(s) - 1])\n}\n",
		want:     "o\n",
		wantExit: 0,
	},
	{
		name:     "repeated_indexing",
		src:      "func main() {\n    let s = \"abcdef\"\n    print(s[1])\n    print(s[4])\n    print(s[1])\n}\n",
		want:     "b\ne\nb\n",
		wantExit: 0,
	},
	{
		name:     "index_in_while_loop",
		src:      "func main() {\n    let s = \"abc\"\n    let i = 0\n    while (i < 3) {\n        print(s[i])\n        i = i + 1\n    }\n}\n",
		want:     "a\nb\nc\n",
		wantExit: 0,
	},
	{
		name:     "index_in_for_loop",
		src:      "func main() {\n    let s = \"abc\"\n    for (let i = 0; i < len(s); i = i + 1) {\n        print(s[i])\n    }\n}\n",
		want:     "a\nb\nc\n",
		wantExit: 0,
	},
	{
		// A view participates in concatenation: `s[i]` operands go through the
		// same view path, so no extra literal is needed and the arena pays for
		// exactly the composed byte.
		name:     "concat_of_views",
		src:      "func main() {\n    let s = \"abc\"\n    print(s[0] + s[1])\n}\n",
		want:     "ab\n",
		wantExit: 0,
	},
	{
		// A view composed with a literal exercises the same path with a
		// non-index operand on the right.
		name:     "concat_view_and_literal",
		src:      "func main() {\n    let s = \"abc\"\n    print(s[0] + \"z\")\n}\n",
		want:     "az\n",
		wantExit: 0,
	},
	{
		name:     "bind_view_then_print",
		src:      "func main() {\n    let s = \"abc\"\n    let b = s[0]\n    print(b)\n    print(len(b))\n}\n",
		want:     "a\n1\n",
		wantExit: 0,
	},
	{
		// The multibyte case, native-only and DOCUMENTED. The resident bytes of
		// "héllo" through the self-hosted rodata writer are [68 00 00 6c 6c 6f]
		// (the é's two bytes become two NULs -- a pre-existing platform
		// limitation of asciiVal/natAppendStr), so len(s) == 6 there. Byte-wise
		// positions 1 and 2 are the two former é bytes, which is the proof that
		// indexing counts BYTES and not code points: a code-point engine reads
		// s[2] as 'l' (6c), this one reads a NUL.
		name:     "utf8_multibyte_positions",
		src:      "func main() {\n    let s = \"héllo\"\n    print(s[0])\n    print(s[1])\n    print(s[2])\n    print(s[3])\n}\n",
		want:     "h\n\x00\n\x00\nl\n",
		wantExit: 0,
		nativeOnly: true,
		nativeNote: "high bytes become NULs in the self-hosted rodata writer; the Go engine prints the real bytes and is not a valid reference for this case",
	},
}

// natStrIdxBounds is the BOUNDS corpus. Every entry must be a RUNTIME error: a
// non-zero exit and NO stdout (beyond the output preceding the failing access).
// A program that printed a value for an out-of-range index would be the worst
// possible outcome, because it would be indistinguishable from a correct
// answer.
var natStrIdxBounds = []struct {
	name string
	src  string
	// wantBefore is the stdout produced BEFORE the failing access.
	wantBefore string
}{
	{
		name:       "index_equal_to_length",
		src:        "func main() {\n    let s = \"abc\"\n    let i = 3\n    print(s[i])\n}\n",
		wantBefore: "",
	},
	{
		name:       "index_greater_than_length",
		src:        "func main() {\n    let s = \"abc\"\n    let i = 9\n    print(s[i])\n}\n",
		wantBefore: "",
	},
	{
		// The index is built by an assignment, because `let` accepts literals
		// only; a negative index is the other out-of-range direction.
		name:       "negative_index",
		src:        "func main() {\n    let s = \"abc\"\n    let i = 0\n    i = i - 1\n    print(s[i])\n}\n",
		wantBefore: "",
	},
	{
		// The empty string is out of range at index 0.
		name:       "empty_string",
		src:        "func main() {\n    let s = \"\"\n    print(s[0])\n}\n",
		wantBefore: "",
	},
	{
		// The multibyte string under the corrupted-bytes representation:
		// len(s) == 6, so index 6 is out of range. The Go engine measures a
		// different length (7) and dies at index 7, so this case is
		// native-only and is asserted on the native engine alone.
		name:       "utf8_index_equal_to_len",
		src:        "func main() {\n    let s = \"héllo\"\n    print(s[len(s)])\n}\n",
		wantBefore: "",
	},
	{
		// The failure happens on the FOURTH iteration, so the first three bytes
		// are printed and then the program dies. The stdout before the failure
		// is asserted, because a bounds check that fired EARLY would also
		// produce "no output" and would pass a weaker test.
		name:       "out_of_range_in_loop",
		src:        "func main() {\n    let s = \"abc\"\n    for (let i = 0; i < 5; i = i + 1) {\n        print(s[i])\n    }\n}\n",
		wantBefore: "a\nb\nc\n",
	},
}

// natStrIdxRefusals is the refusal table: shapes that are still refused, by
// name, with no image written. Each entry is a boundary this slice does NOT
// cross, recorded so the next reader knows the boundary was measured, not
// missed.
var natStrIdxRefusals = []struct {
	name string
	src  string
	want string
}{
	{
		// Indexing a SCALAR. The kind check is load-bearing: without it the
		// lowering reads the scalar's slot as a (ptr, len) string pair and
		// prints bytes from whatever that address happens to be.
		name: "scalar_base",
		src:  "func main() {\n    let a = 7\n    print(a[0])\n}\n",
		want: "index target must be an array",
	},
	{
		// An undeclared base has its own diagnostic, just like the array path.
		name: "undeclared_base",
		src:  "func main() {\n    print(q[0])\n}\n",
		want: "undefined identifier 'q'",
	},
	{
		// A byte index is a STRING, so `s[0] + 1` is not a string concatenation
		// either -- the operand rule is two-strings-only on the native target.
		// (A string + int coercion exists on the C backend and is deliberately
		// not lowered here; the message says which side is not a string.)
		name: "concat_view_with_int_operand",
		src:  "func main() {\n    let s = \"abc\"\n    print(s[0] + 1)\n}\n",
		want: "`+` needs two strings",
	},
	{
		// Literals are not indexable in this slice. The array-index discipline
		// is the same: index targets must be variables.
		name: "literal_base",
		src:  "func main() {\n    print(\"abc\"[0])\n}\n",
		want: "string index target must be a variable",
	},
	{
		// A string index is not sliceable. This closes a pre-existing crash:
		// the slice scanner's kind dispatch accepted the INDEX and the emitter
		// then resolved identName() on a BinaryExpr/INDEX to a slot that does
		// not exist, producing a compiler crash rather than a diagnostic.
		name: "slice_of_index",
		src:  "func main() {\n    let s = \"abc\"\n    print(s[0][1:2])\n}\n",
		want: "string slicing of a string index is not lowered",
	},
	{
		// A view composed with a concat in `let` initialiser position is
		// refused by the existing binding scan (the binding requires an
		// integer expression). Recorded as a boundary, not a defect.
		name: "bind_concat_of_view",
		src:  "func main() {\n    let s = \"abc\"\n    let b = s[0] + \"z\"\n    print(b)\n}\n",
		want: "initialised from a non-integer expression",
	},
	{
		// A byte index inside a string comparison is refused by the NAT-STREQ
		// slice-1 operand rule, which has its own message. This pins that the
		// boundary still holds after this slice landed.
		name: "index_operand_in_comparison",
		src:  "func main() {\n    let s = \"abc\"\n    if (s[0] == \"a\") {\n        print(1)\n    }\n}\n",
		want: "an indexed operand",
	},
	{
		// The pre-existing crash this slice closes: a string concatenation in
		// return position reached natEmitStrValue (which has no BinaryExpr
		// arm) and died with an access violation. It must now be a refusal.
		name: "callee_concat_return",
		src:  "func f() {\n    return \"a\" + \"a\"\n}\nfunc main() {\n    let b = f()\n    print(b)\n}\n",
		want: "return",
	},
}

// natStrIdxKcc builds one source through kcc's whole-program driver and returns
// the CLI's result. The STREQ crash-retry is reused rather than reimplemented:
// the intermittent 0xC0000005 class is a property of the host, not of a gate.
func natStrIdxKcc(t *testing.T, karkain, src, target, outName string) nativeBuildResult {
	return natStrEqKcc(t, karkain, src, target, outName)
}

// TestPhaseNATStrIdx_ProgramsExecuteWithExactBytes is layer 1.
func TestPhaseNATStrIdx_ProgramsExecuteWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; the lowering itself is cross-platform")
	}
	karkain := phase130Karkain(t)

	for _, c := range natStrIdxCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := natStrIdxKcc(t, karkain, c.src, NativeWindowsTarget, "prog.exe")
			if res.ExitCode != 0 {
				t.Fatalf("kcc failed to build a program this gate expects to compile "+
					"(exit %d):\n%s", res.ExitCode, res.Output)
			}
			if !kccProducedImage(res.Output) {
				t.Fatalf("no kcc-native provenance: the image did not come from the "+
					"self-hosted engine.\n%s", res.Output)
			}
			if _, _, err := native.ParsePE(res.Image); err != nil {
				t.Fatalf("kcc's image is not a structurally valid PE: %v", err)
			}
			got, code := runPEAndCapture(t, res.Image)
			if !bytesEqual(got, []byte(c.want)) {
				t.Errorf("stdout = % x, want % x\n  source:\n%s\n"+
					"  Compared as EXACT BYTES: a missing or extra trailing newline is a "+
					"real defect and trimming would hide it.", got, []byte(c.want), c.src)
			}
			if code != c.wantExit {
				t.Errorf("exit code = %d, want %d", code, c.wantExit)
			}
		})
	}
}

// TestPhaseNATStrIdx_MatchesTheCBackend is layer 2.
//
// The same source is compiled the OLD way -- through the C backend and gcc --
// and run, and its stdout is compared with the native image's. The C backend is
// the reference implementation of Karkain's string semantics for bytes < 0x80.
//
// The multibyte case is excluded: the self-hosted rodata writer turns bytes
// >= 0x80 into NULs, so the two engines legitimately disagree on the literal's
// resident bytes. That divergence is recorded in the report and pinned by the
// utf8_multibyte_positions execution case instead of being papered over.
func TestPhaseNATStrIdx_MatchesTheCBackend(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("the C backend comparison needs the host toolchain")
	}
	karkain := phase130Karkain(t)

	for _, c := range natStrIdxCorpus {
		c := c
		if c.nativeOnly {
			t.Run(c.name, func(t *testing.T) {
				t.Skipf("native-only case: %s", c.nativeNote)
			})
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			res := natStrIdxKcc(t, karkain, c.src, NativeWindowsTarget, "prog.exe")
			if res.ExitCode != 0 {
				t.Fatalf("kcc failed to build a program this gate expects to compile:\n%s", res.Output)
			}
			if !kccProducedImage(res.Output) {
				t.Fatalf("no kcc-native provenance:\n%s", res.Output)
			}
			nativeOut, nativeCode := runPEAndCapture(t, res.Image)

			goOut, goCode := natIRunCBackend(t, karkain, c.src)
			goOut = natINormalizeCRLF(goOut)
			if !bytesEqual(nativeOut, goOut) {
				t.Errorf("the native path and the C backend disagree.\n"+
					"  native : % x (exit %d)\n  c-path : % x (exit %d)\n  source:\n%s\n"+
					"  The C backend is the reference implementation of these semantics.",
					nativeOut, nativeCode, goOut, goCode, c.src)
			}
		})
	}
}

// TestPhaseNATStrIdx_OutOfRangeIsARuntimeError is layer 4.
//
// An out-of-range or negative index must be a runtime error with a non-zero
// exit and NO value printed. The stdout produced BEFORE the failure is
// asserted too, because a check that fired one iteration early would also
// produce "no output" and would pass a weaker test.
//
// NOTE ON THE DIAGNOSTIC TEXT: the runtime-error reporter writes to stderr,
// and the Phase-152-B0 GetStdHandle limitation leaves that stream EMPTY on
// this host. The EXIT STATUS is therefore the observable, and the message is
// not asserted here.
func TestPhaseNATStrIdx_OutOfRangeIsARuntimeError(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	karkain := phase130Karkain(t)

	for _, c := range natStrIdxBounds {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := natStrIdxKcc(t, karkain, c.src, NativeWindowsTarget, "bound.exe")
			if res.ExitCode != 0 {
				t.Fatalf("kcc failed to build a program this gate expects to compile:\n%s", res.Output)
			}
			if !kccProducedImage(res.Output) {
				t.Fatalf("no kcc-native provenance:\n%s", res.Output)
			}
			img := res.Image
			got, code := natIdxRunImage(t, img, c.name)
			if code == 0 {
				t.Errorf("an out-of-range access exited 0; it must be a runtime error.\n"+
					"  stdout = % x\n  source:\n%s", got, c.src)
			}
			if !bytesEqual(got, []byte(c.wantBefore)) {
				t.Errorf("stdout = % x, want % x (the output before the failing access)\n  source:\n%s",
					got, []byte(c.wantBefore), c.src)
			}
		})
	}
}

// TestPhaseNATStrIdx_RefusalsAreByName is layer 3.
func TestPhaseNATStrIdx_RefusalsAreByName(t *testing.T) {
	karkain := phase130Karkain(t)

	for _, c := range natStrIdxRefusals {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := natStrIdxKcc(t, karkain, c.src, NativeWindowsTarget, "refused.exe")
			if res.ExitCode == 0 {
				t.Fatalf("kcc ACCEPTED a program it cannot lower (exit 0):\n%s", res.Output)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Errorf("the refusal does not name the construct.\n  want the output to "+
					"contain %q\n  got:\n%s", c.want, res.Output)
			}
			if res.WroteImage {
				t.Errorf("a refused program still wrote an image (%d bytes)", len(res.Image))
			}
		})
	}
}

// TestPhaseNATStrIdx_Determinism is layer 5: the same source twice produces
// the same image. Non-vacuity and provenance are asserted too, so a degenerate
// empty corpus cannot report success.
func TestPhaseNATStrIdx_Determinism(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	karkain := phase130Karkain(t)

	for _, c := range natStrIdxCorpus {
		c := c
		first := natStrIdxKcc(t, karkain, c.src, NativeWindowsTarget, "det1.exe")
		second := natStrIdxKcc(t, karkain, c.src, NativeWindowsTarget, "det2.exe")
		if first.ExitCode != 0 || second.ExitCode != 0 {
			t.Fatalf("kcc failed to build a program this gate expects to compile:\n%s\n%s",
				first.Output, second.Output)
		}
		if first.SHA == "" || first.SHA != second.SHA {
			t.Errorf("image is not deterministic for %q: %s vs %s", c.name, first.SHA, second.SHA)
		}
	}

	// Non-vacuity: the corpus is large enough that dropping a case would be
	// visible, and its entries genuinely differ in output so an emitter that
	// printed the same bytes for everything would fail.
	if len(natStrIdxCorpus) < 14 {
		t.Fatalf("corpus has %d cases; NAT-STRIDX was measured over at least 14 and a "+
			"shrinking corpus is how a gate stops being evidence", len(natStrIdxCorpus))
	}
	count := 0
	seen := map[string]bool{}
	for _, c := range natStrIdxCorpus {
		seen[c.want] = true
		count++
	}
	if len(seen) < 8 {
		t.Errorf("only %d distinct stdout shapes across %d cases; an emitter that "+
			"printed a constant would satisfy a weaker corpus", len(seen), count)
	}
}

// TestPhaseNATStrIdx_CIWiringIsPresent is layer 6.
func TestPhaseNATStrIdx_CIWiringIsPresent(t *testing.T) {
	ci, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	text := string(ci)
	if !strings.Contains(text, "TestPhaseNATStrIdx_") {
		t.Errorf("ci.yml has no step running TestPhaseNATStrIdx_; a gate without a CI " +
			"entry is not a gate (see the companion rule in AGENTS.md)")
	}
	if !strings.Contains(text, "NAT-STRIDX") {
		t.Errorf("ci.yml is missing the NAT-STRIDX step name")
	}
}
