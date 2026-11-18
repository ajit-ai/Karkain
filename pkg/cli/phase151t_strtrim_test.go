package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// NAT-STRTRIM -- native trim as a NON-ALLOCATING string view.
//
// The actual standard-library API discovered in the repository:
//
//   - stdlib/string/string.kark: `str_trim(s)` is `trim(s)` -- it delegates to
//     the builtin; `str_trim_left`/`str_trim_right` pin the same whitespace
//     set (space, tab, LF, CR) as byte comparisons against single bytes.
//   - the C backend (pkg/codegen/codegen.go, karkain_trim): strips exactly
//     ' ', '\t', '\n', '\r' from both ends; everything else, including every
//     byte >= 0x7f, is preserved; internal whitespace is preserved;
//     all-whitespace and empty inputs produce ""; a non-string argument
//     produces "" there (the native scan refuses it by name instead).
//   - SPEC.md: strings are UTF-8 byte strings; indexing and `len` are
//     byte-wise; case conversion is ASCII-only.
//
// THE CONTRACT THIS INCREMENT IMPLEMENTS (identical set on both engines):
//
//   - leading AND trailing ASCII whitespace bytes are removed;
//   - whitespace is exactly 0x20, 0x09, 0x0a, 0x0d -- no NBSP, no Unicode,
//     no locale;
//   - internal whitespace is preserved;
//   - empty input -> zero-length view; all-whitespace input -> zero-length
//     view; the view is never a copy and never allocates;
//   - a view that trims nothing IS the source string (ptr, len unchanged),
//     not a new string;
//   - UTF-8 high bytes are preserved as bytes (they are never treated as
//     whitespace); with the kcc rodata-writer limitation (bytes >= 0x80
//     become NULs, see NAT-STRIDX-IMPLEMENTATION-REPORT.md), the resident
//     image still trims byte-accurately on the resident bytes.
//
// THE IMPLEMENTATION (see natEmitTrim in src/compiler/native_value.kark):
//
//   (ptr + start, trimmed_len) of the source bytes. No allocation, no copy,
//   no external runtime, no C or Go production-source change. The argument
//   lowers as an ordinary string pair (RDI=ptr, RSI=len) -- the oracle's own
//   shape -- and the two scan loops only move registers. Repeated trim calls
//   are safe (each level is a view of a view of the same storage).
//
//   VIEW LIFETIME: a trim view can only ever reference bytes no local frame
//   owns -- rodata literals (interned into the image, immortal), the bump
//   arena (lived for the process), or a view of those. The argument scan
//   refuses slices and concatenations (which would produced freed-then-moved
//   bytes or unbounded storage), so no trim view can alias a frame. The
//   callee-boundary cases are executed by this gate.
//
// THE ORACLE IS THE C BACKEND. The Go native backend has no trim, so there
// is no ELF byte-identity target; the differential compiles the same source
// through the C backend (--engine go) and compares stdout exactly, with
// CRLF normalized. The multibyte case is native-only by design (the kcc
// rodata writer corrupts high bytes; see below).

type natTrimCase struct {
	name     string
	src      string
	want     string
	wantExit int
	// nativeOnly excludes the case from the C-backend differential. The
	// resident literal's high bytes do not survive the kcc rodata writer
	// (they become 0x00), so the Go engine's output is not a valid reference.
	nativeOnly bool
	nativeNote string
}

var natTrimCorpus = []natTrimCase{
	{name: "no_trim_needed", src: "func main() {\n    print(trim(\"hi\"))\n}\n", want: "hi\n", wantExit: 0},
	{name: "leading_whitespace", src: "func main() {\n    print(trim(\"  hi\"))\n}\n", want: "hi\n", wantExit: 0},
	{name: "trailing_whitespace", src: "func main() {\n    print(trim(\"hi  \"))\n}\n", want: "hi\n", wantExit: 0},
	{name: "leading_and_trailing", src: "func main() {\n    print(trim(\"  hi  \"))\n}\n", want: "hi\n", wantExit: 0},
	{name: "internal_whitespace_preserved", src: "func main() {\n    print(trim(\"  a b  \"))\n}\n", want: "a b\n", wantExit: 0},
	{name: "empty_string", src: "func main() {\n    print(trim(\"\"))\n}\n", want: "\n", wantExit: 0},
	{name: "all_whitespace", src: "func main() {\n    print(trim(\"    \"))\n}\n", want: "\n", wantExit: 0},
	{name: "one_byte_string", src: "func main() {\n    print(trim(\"a\"))\n}\n", want: "a\n", wantExit: 0},
	{name: "one_byte_whitespace", src: "func main() {\n    print(trim(\" \"))\n}\n", want: "\n", wantExit: 0},
	{
		// Raw tab (0x09) and CR (0x0d) bytes. Escapes are NOT interpreted by
		// this language (see the lexer), so the corpus writes raw bytes.
		name:     "raw_tab_and_cr",
		src:      "func main() {\n    let s = \"\u0009\u000d hi \u0009\u000d\"\n    print(trim(s))\n    print(len(trim(s)))\n}\n",
		want:     "hi\n2\n",
		wantExit: 0,
	},
	{name: "many_consecutive_whitespace", src: "func main() {\n    print(trim(\"      hi      \"))\n}\n", want: "hi\n", wantExit: 0},
	{
		name:     "repeated_trim_calls",
		src:      "func main() {\n    let s = \"  hi  \"\n    print(trim(trim(s)))\n    print(trim(trim(trim(s))))\n}\n",
		want:     "hi\nhi\n",
		wantExit: 0,
	},
	{name: "len_of_trim", src: "func main() {\n    print(len(trim(\"  hi  \")))\n}\n", want: "2\n", wantExit: 0},
	{
		name: "bind_view_then_print",
		src:  "func main() {\n    let b = trim(\"  hi  \")\n    print(b)\n    print(len(b))\n}\n",
		want: "hi\n2\n",
		wantExit: 0,
	},
	{name: "concat_view_plus_literal", src: "func main() {\n    print(trim(\"  hi  \") + \"!\")\n}\n", want: "hi!\n", wantExit: 0},
	{name: "concat_literal_plus_view", src: "func main() {\n    print(\"!\" + trim(\"  hi  \"))\n}\n", want: "!hi\n", wantExit: 0},
	{name: "concat_view_plus_view", src: "func main() {\n    print(trim(\"  h\") + trim(\"i  \"))\n}\n", want: "hi\n", wantExit: 0},
	{
		name: "trim_returned_from_callee",
		src:  "func f() {\n    return trim(\"  hi  \")\n}\nfunc main() {\n    let b = f()\n    print(b)\n}\n",
		want: "hi\n",
		wantExit: 0,
	},
	{
		name: "trim_from_param_returned",
		src:  "func f(s) {\n    return trim(s)\n}\nfunc main() {\n    let b = f(\"  hi  \")\n    print(b)\n    print(len(b))\n}\n",
		want: "hi\n2\n",
		wantExit: 0,
	},
	{
		name: "nested_trim_in_callee",
		src:  "func trim2(s) {\n    return trim(trim(s))\n}\nfunc main() {\n    let b = trim2(\"  hi  \")\n    print(b)\n}\n",
		want: "hi\n",
		wantExit: 0,
	},
	{
		// The resident bytes of this literal through the self-hosted rodata
		// writer are [20 20 00 00 20 20] (the é's two bytes become two NULs --
		// see NAT-STRIDX-IMPLEMENTATION-REPORT.md). The boundaries are computed
		// on the RESIDENT bytes, so the view is [00 00], len 2 -- byte-accurate
		// trimming of the corrupted-but-resident representation. The Go engine
		// sees the real é bytes and is not a valid reference here.
		name:       "utf8_high_bytes_native_only",
		src:        "func main() {\n    let s = \"  \u00e9  \"\n    print(trim(s))\n    print(len(trim(s)))\n}\n",
		want:       "\x00\x00\n2\n",
		wantExit:   0,
		nativeOnly: true,
		nativeNote: "high bytes become NULs in the self-hosted rodata writer; the resident bytes are what trim sees",
	},
}

// natTrimRefusals is the refusal table: shapes that are still refused, by
// name, with no image written. Each entry is a boundary this slice does NOT
// cross.
var natTrimRefusals = []struct {
	name string
	src  string
	want string
}{
	{
		name: "trim_of_int_scalar",
		src:  "func main() {\n    let a = 7\n    print(trim(a))\n}\n",
		want: "trim() argument must be a string",
	},
	{
		name: "trim_of_undeclared",
		src:  "func main() {\n    print(trim(q))\n}\n",
		want: "undefined identifier 'q'",
	},
	{
		name: "trim_of_concat",
		src:  "func main() {\n    print(trim(\"a\" + \"b\"))\n}\n",
		want: "trim() of a concatenation is not lowered",
	},
	{
		name: "trim_of_slice",
		src:  "func main() {\n    print(trim(\"abc\"[1:2]))\n}\n",
		want: "trim() of a string slice is not lowered",
	},
	{
		name: "trim_of_non_string_index",
		src:  "func main() {\n    let a = [1, 2]\n    print(trim(a[0]))\n}\n",
		want: "the index target must be a string",
	},
	{
		// len(trim(s)) nested inside arithmetic would need the string emitter
		// inside the int walker. Refused by name rather than emitting a wrong
		// answer; direct print and return positions lower it fine.
		name: "len_of_trim_nested_in_arithmetic",
		src:  "func main() {\n    let s = \"  hi  \"\n    print(len(trim(s)) + 1)\n}\n",
		want: "len(trim(s)) inside arithmetic is not lowered",
	},
	{
		name: "concat_view_with_int_operand",
		src:  "func main() {\n    let s = \"  hi  \"\n    print(trim(s) + 1)\n}\n",
		want: "`+` needs two strings",
	},
	{
		name: "bind_concat_of_trim_view",
		src:  "func main() {\n    let s = \"  hi  \"\n    let b = trim(s) + \"x\"\n    print(b)\n}\n",
		want: "initialised from a non-integer expression",
	},
	{
		// The two-unit (ptr, len) pair cannot become the process exit status;
		// identical boundary to `return "x"` and `return s[i]` in main.
		name: "callee_trim_return_in_main",
		src:  "func main() {\n    return trim(\"  hi  \")\n}\n",
		want: "`return` of a string from `main`",
	},
	{
		// A trim view composed into a concat in a callee's return position:
		// the callee must return a value, and a concat has no lowering there.
		name: "callee_concat_return",
		src:  "func f() {\n    return trim(\"  hi  \") + \"!\"\n}\nfunc main() {\n    let b = f()\n    print(b)\n}\n",
		want: "`return` of a string concatenation",
	},
}

// TestPhaseNATStrTrim_ProgramsExecuteWithExactBytes is layer 1.
func TestPhaseNATStrTrim_ProgramsExecuteWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows; the lowering itself is cross-platform")
	}
	karkain := phase130Karkain(t)

	for _, c := range natTrimCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := natStrEqKcc(t, karkain, c.src, NativeWindowsTarget, "prog.exe")
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

// TestPhaseNATStrTrim_MatchesTheCBackend is layer 2.
//
// The same source is compiled the OLD way -- through the C backend and gcc --
// and run; stdout is compared after CRLF normalization. The multibyte case is
// excluded by design: the self-hosted rodata writer corrupts bytes >= 0x80.
func TestPhaseNATStrTrim_MatchesTheCBackend(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("the C backend comparison needs the host toolchain")
	}
	karkain := phase130Karkain(t)

	for _, c := range natTrimCorpus {
		c := c
		if c.nativeOnly {
			t.Run(c.name, func(t *testing.T) {
				t.Skipf("native-only case: %s", c.nativeNote)
			})
			continue
		}
		t.Run(c.name, func(t *testing.T) {
			res := natStrEqKcc(t, karkain, c.src, NativeWindowsTarget, "prog.exe")
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

// TestPhaseNATStrTrim_RefusalsAreByName is layer 3.
func TestPhaseNATStrTrim_RefusalsAreByName(t *testing.T) {
	karkain := phase130Karkain(t)

	for _, c := range natTrimRefusals {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := natStrEqKcc(t, karkain, c.src, NativeWindowsTarget, "refused.exe")
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

// TestPhaseNATStrTrim_StrTrimWorks is layer 4: the actual standard-library
// export -- `import std.string` + `str_trim`, whose body returns the builtin
// `trim(s)`. The whole delegation path (module assembly, callee scan, callee
// return lowering, string parameter staging) is exercised.
//
// The project lives under .karkain-dev/ because the stdlib resolver walks UP
// from the source file; an isolated t.TempDir() outside the repository finds
// no stdlib. The directory is unique and removed on completion; .karkain-dev
// is gitignored, so nothing can leak into a commit.
func TestPhaseNATStrTrim_StrTrimWorks(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	karkain := phase130Karkain(t)
	root := repoRoot(t)
	base := filepath.Join(root, ".karkain-dev")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("creating .karkain-dev: %v", err)
	}
	dir, err := os.MkdirTemp(base, "strtrim-gate-*")
	if err != nil {
		t.Fatalf("creating the temp project: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	src := "import std.string\nfunc main() {\n    let s = \"  hi  \"\n    print(str_trim(s))\n}\n"
	file := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatalf("writing the test program: %v", err)
	}
	outPath := filepath.Join(dir, "strtrim-gate.exe")
	cmd := exec.Command(karkain, "build", file, "--target", NativeWindowsTarget, "-o", outPath)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=kcc")
	out, err := cmd.CombinedOutput()
	s := string(out)
	if err != nil {
		t.Fatalf("kcc failed to build a stdlib-importing program this gate expects to compile:\n%s", out)
	}
	// Provenance: kcc must have been consulted AND must have produced bytes.
	// The two markers are declared on both sides so they cannot drift.
	if !strings.Contains(s, kccNativeProvenance) {
		t.Fatalf("no kcc-native provenance marker in the build output:\n%s", s)
	}
	if !strings.Contains(s, kccNativeImageMarker) {
		t.Fatalf("kcc printed no image marker:\n%s", s)
	}
	img, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading the image kcc wrote: %v", err)
	}
	if _, _, err := native.ParsePE(img); err != nil {
		t.Fatalf("kcc's image is not a structurally valid PE: %v", err)
	}
	got, code := runPEAndCapture(t, img)
	if !bytesEqual(got, []byte("hi\n")) {
		t.Errorf("stdout = % x, want 68 69 0a", got)
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

// TestPhaseNATStrTrim_Determinism is layer 5: the same source twice produces
// the same image; non-vacuity is asserted for the corpus, so an empty or
// shrunken table cannot report success.
func TestPhaseNATStrTrim_Determinism(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	karkain := phase130Karkain(t)

	for _, c := range natTrimCorpus {
		c := c
		first := natStrEqKcc(t, karkain, c.src, NativeWindowsTarget, "det1.exe")
		second := natStrEqKcc(t, karkain, c.src, NativeWindowsTarget, "det2.exe")
		if first.ExitCode != 0 || second.ExitCode != 0 {
			t.Fatalf("kcc failed to build a program this gate expects to compile:\n%s\n%s",
				first.Output, second.Output)
		}
		if first.SHA == "" || first.SHA != second.SHA {
			t.Errorf("image is not deterministic for %q: %s vs %s", c.name, first.SHA, second.SHA)
		}
	}
	if len(natTrimCorpus) < 20 {
		t.Fatalf("corpus has %d cases; NAT-STRTRIM was measured over at least 20 and a "+
			"shrinking corpus is how a gate stops being evidence", len(natTrimCorpus))
	}
	seen := map[string]bool{}
	for _, c := range natTrimCorpus {
		seen[c.want] = true
	}
	if len(seen) < 8 {
		t.Errorf("only %d distinct stdout shapes across %d cases; an emitter that "+
			"printed a constant would satisfy a weaker corpus", len(seen), len(natTrimCorpus))
	}
}

// TestPhaseNATStrTrim_CIWiringIsPresent is layer 6.
func TestPhaseNATStrTrim_CIWiringIsPresent(t *testing.T) {
	ci, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	text := string(ci)
	if !strings.Contains(text, "TestPhaseNATStrTrim_") {
		t.Errorf("ci.yml has no step running TestPhaseNATStrTrim_; a gate without a CI " +
			"entry is not a gate (see the companion rule in AGENTS.md)")
	}
	if !strings.Contains(text, "NAT-STRTRIM") {
		t.Errorf("ci.yml is missing the NAT-STRTRIM step name")
	}
}
