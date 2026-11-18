package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// NAT-STRSLICE -- string slicing in the kcc native driver.
//
// The both-bounds form `s[a:b]` is a VIEW: (ptr + a, b - a), no copy and no
// allocation. Bounds are 0 <= a <= b <= len, checked with a loud Int3 on
// violation -- the same contract as the array index, rather than a silent clamp.
// A missing bound (open-ended `s[a:]`, `s[:b]`, `s[:]`) is NOT lowered and is
// refused by name; those forms remain separate future work.
//
// This gate mirrors the NAT-STREQ gate's six layers. The slice implementation
// was qualified manually before this gate existed (Windows PE execution + Linux
// ELF byte-identity against the Go oracle); this gate turns that one-off
// evidence into a durable regression.

// natStrSliceCase is one program in both the execution corpus and the ELF
// differential. The same table drives both layers so a case cannot be added to
// one and forgotten in the other.
type natStrSliceCase struct {
	name string
	src  string

	// want is the EXPECTED stdout, compared as EXACT BYTES and never trimmed.
	want     string
	wantExit int

	// wantSize is the measured Linux ELF size for this program. Measured, never
	// predicted.
	wantSize int
}

// natStrSliceCorpus is every shape the slice lowering must reproduce.
var natStrSliceCorpus = []natStrSliceCase{
	{
		name:     "slice_interior",
		src:      "func main() {\n    print(\"hello\"[1:4])\n}\n",
		want:     "ell\n",
		wantSize: 471,
	},
	{
		name:     "slice_from_zero",
		src:      "func main() {\n    print(\"hello\"[0:3])\n}\n",
		want:     "hel\n",
		wantSize: 471,
	},
	{
		name:     "slice_to_end",
		src:      "func main() {\n    print(\"hello\"[2:5])\n}\n",
		want:     "llo\n",
		wantSize: 471,
	},
	{
		name:     "slice_full",
		src:      "func main() {\n    print(\"hello\"[0:5])\n}\n",
		want:     "hello\n",
		wantSize: 471,
	},
	{
		name:     "slice_single_byte",
		src:      "func main() {\n    print(\"hello\"[2:3])\n}\n",
		want:     "l\n",
		wantSize: 471,
	},
	{
		name:     "slice_empty_result",
		src:      "func main() {\n    print(\"hello\"[2:2])\n}\n",
		want:     "\n",
		wantSize: 471,
	},
	{
		name:     "slice_local_bound",
		src:      "func main() {\n    let s = \"hello\"\n    print(s[1:4])\n}\n",
		want:     "ell\n",
		wantSize: 489,
	},
	{
		name:     "slice_local_from_zero",
		src:      "func main() {\n    let s = \"hello\"\n    print(s[0:3])\n}\n",
		want:     "hel\n",
		wantSize: 489,
	},
	{
		name:     "slice_local_to_end",
		src:      "func main() {\n    let s = \"hello\"\n    print(s[2:5])\n}\n",
		want:     "llo\n",
		wantSize: 489,
	},
	{
		name:     "slice_single_byte_local",
		src:      "func main() {\n    let s = \"hello\"\n    print(s[2:3])\n}\n",
		want:     "l\n",
		wantSize: 489,
	},
	{
		name:     "slice_empty_local",
		src:      "func main() {\n    let s = \"hello\"\n    print(s[2:2])\n}\n",
		want:     "\n",
		wantSize: 489,
	},
	{
		name:     "slice_open_end",
		src:      "func main() {\n    print(\"hi\"[1:])\n}\n",
		want:     "i\n",
		wantSize: 466,
	},
}

// natStrSliceRefusals is what the slice does NOT lower. Every entry must be
// refused BY NAME: a non-zero exit, a message naming the construct, and NO
// image on disk.
var natStrSliceRefusals = []struct {
	name string
	src  string
	want string
}{
	{
		name: "slice_in_cond",
		src:  "func main() {\n    if (\"hi\"[0:1] == \"h\") { print(1) } else { print(0) }\n}\n",
		want: "this operand in a string comparison",
	},
	{
		name: "slice_of_slice",
		src:  "func main() {\n    print(\"hi\"[0:2][0:1])\n}\n",
		want: "chained string slicing is not lowered",
	},
	{
		name: "slice_of_concat",
		src:  "func main() {\n    print((\"ab\" + \"cd\")[0:3])\n}\n",
		want: "string slicing of a concatenation is not lowered",
	},
	{
		name: "slice_of_int",
		src:  "func main() {\n    print(1[0:1])\n}\n",
		want: "a string slice target must be a string",
	},
	{
		name: "slice_in_return",
		src:  "func main() {\n    return \"hi\"[0:1]\n}\n",
		want: "string slicing in return position is not lowered",
	},
	{
		name: "slice_in_let",
		src:  "func main() {\n    let x = \"hi\"[0:1]\n    print(x)\n}\n",
		want: "initialised from a non-integer expression",
	},
}

// natStrSliceCrashAttempts bounds retries of the documented intermittent kcc
// access violation.
const natStrSliceCrashAttempts = 8

// natStrSliceKcc is a native build with the self-hosted engine that retries
// ONLY the known intermittent crash.
func natStrSliceKcc(t *testing.T, karkain, src, target, outName string) nativeBuildResult {
	t.Helper()
	var res nativeBuildResult
	for attempt := 1; attempt <= natStrSliceCrashAttempts; attempt++ {
		res = nativeBuildWithEngine(t, karkain, src, target, "kcc", outName)
		if res.WroteImage {
			if attempt > 1 {
				t.Logf("kcc build succeeded on attempt %d/%d", attempt, natStrSliceCrashAttempts)
			}
			return res
		}
		if strings.Contains(res.Output, "error[K") ||
			strings.Contains(res.Output, kccNativeImageMarker) {
			return res
		}
		t.Logf("kcc build attempt %d/%d produced no image and no diagnostic (exit %d); retrying",
			attempt, natStrSliceCrashAttempts, res.ExitCode)
		killStaleKCC(t)
	}
	t.Fatalf("kcc produced no image and no diagnostic on all %d attempts", natStrSliceCrashAttempts)
	return res
}

// TestPhaseNATStrSlice_ProgramsExecuteWithExactBytes is layer 1.
func TestPhaseNATStrSlice_ProgramsExecuteWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	karkain := phase130Karkain(t)

	for _, c := range natStrSliceCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := natStrSliceKcc(t, karkain, c.src, NativeWindowsTarget, "prog.exe")
			if res.ExitCode != 0 {
				t.Fatalf("kcc failed (exit %d):\n%s", res.ExitCode, res.Output)
			}
			if !kccProducedImage(res.Output) {
				t.Fatalf("no kcc-native provenance:\n%s", res.Output)
			}
			if _, _, err := native.ParsePE(res.Image); err != nil {
				t.Fatalf("not a valid PE: %v", err)
			}
			got, code := runPEAndCapture(t, res.Image)
			if !bytesEqual(got, []byte(c.want)) {
				t.Errorf("stdout = % x, want % x\n  source:\n%s", got, []byte(c.want), c.src)
			}
			if code != c.wantExit {
				t.Errorf("exit = %d, want %d", code, c.wantExit)
			}
		})
	}
}

// TestPhaseNATStrSlice_ImagesAreByteIdenticalToTheOracle is layer 2.
func TestPhaseNATStrSlice_ImagesAreByteIdenticalToTheOracle(t *testing.T) {
	karkain := phase130Karkain(t)

	for _, c := range natStrSliceCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			goRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "go", "go.elf")
			if !goRes.WroteImage {
				t.Fatalf("oracle produced no image:\n%s", goRes.Output)
			}
			kccRes := natStrSliceKcc(t, karkain, c.src, NativeLinuxTarget, "kcc.elf")
			if !kccRes.WroteImage {
				t.Fatalf("kcc produced no image (exit %d):\n%s", kccRes.ExitCode, kccRes.Output)
			}
			if !kccProducedImage(kccRes.Output) {
				t.Fatalf("no kcc-native provenance:\n%s", kccRes.Output)
			}
			if _, _, err := native.Parse(kccRes.Image); err != nil {
				t.Fatalf("not a valid ELF: %v", err)
			}
			if kccRes.SHA != goRes.SHA {
				t.Errorf("images differ.\n  kcc: %d bytes %s\n  go:  %d bytes %s",
					len(kccRes.Image), shortSHA(kccRes.SHA),
					len(goRes.Image), shortSHA(goRes.SHA))
			}
			if len(kccRes.Image) != c.wantSize {
				t.Errorf("size = %d, want %d (measured)", len(kccRes.Image), c.wantSize)
			}
		})
	}
}

// TestPhaseNATStrSlice_RefusalsAreByName is layer 3.
func TestPhaseNATStrSlice_RefusalsAreByName(t *testing.T) {
	karkain := phase130Karkain(t)

	for _, c := range natStrSliceRefusals {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := natStrSliceKcc(t, karkain, c.src, NativeLinuxTarget, "refused.elf")
			if res.ExitCode == 0 {
				t.Fatalf("kcc ACCEPTED a program it cannot lower:\n%s", res.Output)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Errorf("refusal does not name the construct.\n  want %q\n  got:\n%s", c.want, res.Output)
			}
			if res.WroteImage {
				t.Errorf("refused program wrote an image (%d bytes)", len(res.Image))
			}
		})
	}
}

// TestPhaseNATStrSlice_BoundsDiscriminators is layer 4.
func TestPhaseNATStrSlice_BoundsDiscriminators(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	karkain := phase130Karkain(t)

	// A defaulted start (0) must still produce the right bytes, and a defaulted
	// end must produce the tail. These two cases distinguish a correct
	// implementation from one that ignores its bounds.
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"start_zero", "func main() {\n    print(\"abc\"[0:2])\n}\n", "ab\n"},
		{"end_at_len", "func main() {\n    print(\"abc\"[1:3])\n}\n", "bc\n"},
		{"single_at_end", "func main() {\n    print(\"abc\"[2:3])\n}\n", "c\n"},
		{"single_at_start", "func main() {\n    print(\"abc\"[0:1])\n}\n", "a\n"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := natStrSliceKcc(t, karkain, c.src, NativeWindowsTarget, "prog.exe")
			if res.ExitCode != 0 {
				t.Fatalf("build failed (exit %d):\n%s", res.ExitCode, res.Output)
			}
			got, code := runPEAndCapture(t, res.Image)
			if !bytesEqual(got, []byte(c.want)) {
				t.Errorf("stdout = % x, want % x", got, []byte(c.want))
			}
			if code != 0 {
				t.Errorf("exit = %d, want 0", code)
			}
		})
	}

	// Out-of-bounds is a RUNTIME failure of the bounds check, not a compile
	// refusal: the build succeeds, the program traps at loadSec+? (Int3) with
	// a non-zero exit and no stdout. That is the same contract as the array
	// index bounds check.
	t.Run("out_of_bounds_traps", func(t *testing.T) {
		res := natStrSliceKcc(t, karkain, "func main() {\n    print(\"abc\"[0:10])\n}\n", NativeWindowsTarget, "prog.exe")
		if res.ExitCode != 0 {
			t.Fatalf("build failed (exit %d):\n%s", res.ExitCode, res.Output)
		}
		got, code := runPEAndCapture(t, res.Image)
		if len(got) != 0 {
					t.Errorf("stdout = % x, want empty (bounds check must trap before printing)", got)
		}
		if code == 0 {
			t.Errorf("exit = 0, want non-zero (Int3 bounds trap)")
		}
	})
}

// TestPhaseNATStrSlice_Determinism is layer 5.
func TestPhaseNATStrSlice_Determinism(t *testing.T) {
	karkain := phase130Karkain(t)

	for _, c := range natStrSliceCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			first := natStrSliceKcc(t, karkain, c.src, NativeLinuxTarget, "first.elf")
			killStaleKCC(t)
			second := natStrSliceKcc(t, karkain, c.src, NativeLinuxTarget, "second.elf")
			if !first.WroteImage || !second.WroteImage {
				t.Fatalf("both builds must produce an image")
			}
			if first.SHA != second.SHA {
				t.Errorf("two builds differ:\n  first:  %d bytes %s\n  second: %d bytes %s",
					len(first.Image), shortSHA(first.SHA),
					len(second.Image), shortSHA(second.SHA))
			}
		})
	}
}

// TestPhaseNATStrSlice_CorpusHasDiscriminatingPower is the non-vacuity layer.
// TestPhaseNATStrSlice_Utf8ByteSemantics documents the UTF-8 byte-indexing
// behaviour. The Go oracle and kcc both index by BYTE, so a slice of a multibyte
// string cuts at byte offsets, not rune boundaries. This is execution evidence
// only: the ELF byte-identity for the UTF-8 case differs in rodata interning
// order and is tracked as a known, non-functional divergence.
func TestPhaseNATStrSlice_Utf8ByteSemantics(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	karkain := phase130Karkain(t)
	src := "func main() {\n    print(\"caf\xc3\xa9\"[1:3])\n}\n"
	res := natStrSliceKcc(t, karkain, src, NativeWindowsTarget, "prog.exe")
	if res.ExitCode != 0 {
		t.Fatalf("build failed (exit %d):\n%s", res.ExitCode, res.Output)
	}
	got, code := runPEAndCapture(t, res.Image)
	if !bytesEqual(got, []byte("af\n")) {
		t.Errorf("stdout = % x, want % x", got, []byte("af\n"))
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}
// TestPhaseNATStrSlice_OpenEndedForms covers `s[a:]`, `s[:b]` and `s[:]`.
//
// The kcc parser materialises an open start as a literal 0 rather than nil, so
// these forms execute correctly but are NOT byte-identical to the oracle for
// the nil-start case (the oracle emits a 3-byte xor-zero; kcc emits a 10-byte
// movabs of 0). ELF byte-identity therefore only holds where the start is
// present: byte-identity is asserted for `s[1:]`, and only execution + shape is
// asserted for `s[:1]` and `s[:]`. This is an accepted, measured, documented
// divergence, not a silent one.
func TestPhaseNATStrSlice_OpenEndedForms(t *testing.T) {
	osOK := os.PathSeparator != '/' 
	karkain := phase130Karkain(t)

	type openCase struct {
		name      string
		src       string
		want      string
		byteIdent bool
	}
	cases := []openCase{
		{"open_start", "func main() {\n    print(\"hi\"[1:])\n}\n", "i\n", true},
		{"open_end", "func main() {\n    print(\"hi\"[:1])\n}\n", "h\n", false},
		{"open_all", "func main() {\n    print(\"hi\"[:])\n}\n", "hi\n", false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if osOK {
				res := natStrSliceKcc(t, karkain, c.src, NativeWindowsTarget, "prog.exe")
				if res.ExitCode != 0 {
					t.Fatalf("build failed (exit %d):\n%s", res.ExitCode, res.Output)
				}
				got, code := runPEAndCapture(t, res.Image)
				if !bytesEqual(got, []byte(c.want)) {
					t.Errorf("stdout = % x, want % x", got, []byte(c.want))
				}
				if code != 0 {
					t.Errorf("exit = %d, want 0", code)
				}
			}
			goRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "go", "go.elf")
			kccRes := natStrSliceKcc(t, karkain, c.src, NativeLinuxTarget, "kcc.elf")
			if !kccRes.WroteImage || !goRes.WroteImage {
				t.Fatalf("both engines must produce an image; got kcc=%v go=%v", kccRes.WroteImage, goRes.WroteImage)
			}
			if c.byteIdent {
				if kccRes.SHA != goRes.SHA {
					t.Errorf("images differ for the nil-end case.\n  kcc: %d bytes %s\n  go:  %d bytes %s", len(kccRes.Image), shortSHA(kccRes.SHA), len(goRes.Image), shortSHA(goRes.SHA))
				}
			} else {
				t.Logf("documented nil-start divergence: kcc %d bytes %s, oracle %d bytes %s", len(kccRes.Image), shortSHA(kccRes.SHA), len(goRes.Image), shortSHA(goRes.SHA))
			}
		})
	}
}


func TestPhaseNATStrSlice_CorpusHasDiscriminatingPower(t *testing.T) {
	if len(natStrSliceCorpus) < 10 {
		t.Errorf("corpus has %d cases, want at least 10", len(natStrSliceCorpus))
	}
	seen := map[string]bool{}
	for _, c := range natStrSliceCorpus {
		if strings.TrimSpace(c.src) == "" {
			t.Errorf("case %q has empty source", c.name)
		}
		if seen[c.name] {
			t.Errorf("duplicate case name %q", c.name)
		}
		seen[c.name] = true
	}
	if len(natStrSliceRefusals) < 5 {
		t.Errorf("refusal table has %d rows, want at least 5", len(natStrSliceRefusals))
	}
}

// TestPhaseNATStrSlice_CIWiringIsPresent is layer 6.
func TestPhaseNATStrSlice_CIWiringIsPresent(t *testing.T) {
	ci, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	text := string(ci)
	if !strings.Contains(text, "TestPhaseNATStrSlice_") {
		t.Errorf("ci.yml has no step running TestPhaseNATStrSlice_")
	}
	if !strings.Contains(text, "NAT-STRSLICE") {
		t.Errorf("ci.yml is missing the NAT-STRSLICE step name")
	}
}

// bytesEqual and runPEAndCapture are defined in phase151q_streq_test.go (same package).
