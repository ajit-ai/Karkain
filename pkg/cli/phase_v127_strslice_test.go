package cli

import (
	"strings"
	"testing"
)

// v1.2.7 -- native STRING SLICE (`s[a:b]` — both-bounds VIEW form).
// NAT-STRSLICE, ported from feature/v1.3.0-natstdlib-string (branch 645d5d3).

type v127SliceCase struct {
	name string
	src  string
	want string
}

var v127StrsliceCases = []v127SliceCase{
	{"literal_full", "func main() { print(\"hello\"[0:5]) }\n", "hello\n"},
	{"literal_single_byte", "func main() { print(\"hello\"[2:3]) }\n", "l\n"},
	{"literal_empty_result", "func main() { print(\"hello\"[2:2]) }\n", "\n"},
	{"local_mid", "func main() { let s = \"hello\"\nprint(s[1:4]) }\n", "ell\n"},
	{"local_from_zero", "func main() { let s = \"hello\"\nprint(s[0:3]) }\n", "hel\n"},
	{"local_to_end", "func main() { let s = \"hello\"\nprint(s[2:5]) }\n", "llo\n"},
	{"local_single_byte", "func main() { let s = \"hello\"\nprint(s[2:3]) }\n", "l\n"},
	{"local_empty", "func main() { let s = \"hello\"\nprint(s[2:2]) }\n", "\n"},
	// Open-ended forms. An absent bound is an EMPTY node, and the emitter
	// substitutes 0 for a missing start and the string's length for a missing
	// end -- the oracle's own rule (program.go:6541-6556). These four were
	// listed as REFUSALS until v1.2.7 measured them: the scanner's
	// `sliceStart(e) == 0` test compared an array with an integer and could
	// never fire, while the emitter already lowered the shapes correctly.
	// Keeping the rows as refusals would have pinned a diagnostic the compiler
	// does not produce.
	{"literal_open_end", "func main() { print(\"hi\"[1:]) }\n", "i\n"},
	{"literal_open_start", "func main() { print(\"hi\"[:1]) }\n", "h\n"},
	{"literal_open_both", "func main() { print(\"hi\"[:]) }\n", "hi\n"},
	{"local_open_end", "func main() { let s = \"hello\"\nprint(s[2:]) }\n", "llo\n"},
	{"local_open_start", "func main() { let s = \"hello\"\nprint(s[:3]) }\n", "hel\n"},
	// A PRESENT bound is an ordinary int expression -- the scanner routes it
	// through natNativeScanIntExpr and the emitter through natEmitIntExpr, so
	// variables and arithmetic must work rather than only literals.
	{"local_bounds_via_vars", "func main() { let s = \"hello\"\nlet a = 1 let b = 4\nprint(s[a:b]) }\n", "ell\n"},
	{"local_bounds_arithmetic", "func main() { let s = \"hello\"\nlet a = 1\nprint(s[a + 1:a * 2 + 3]) }\n", "llo\n"},
}

var v127StrsliceRefusals = []struct {
	name string
	src  string
	want string
}{
	{"slice_of_int", "func main() { print(1[0:1]) }\n", "a string slice target must be a string"},
	{"chained", "func main() { print(\"hi\"[0:2][0:1]) }\n", "chained string slicing is not lowered"},
	// A present bound that is not an int is refused by NAME at the bound,
	// before emission. This is the check that replaced the two dead
	// open-ended refusals: the old rows asserted a diagnostic the compiler
	// never produced, and these assert the one it does.
	{"float_bound", "func main() { print(\"hi\"[0.5:2]) }\n", "is not an integer expression"},
}

func TestPhaseV127Strslice_ExecutedPEWithExactBytes(t *testing.T) {
	karkain := v126Setup(t)
	for _, c := range v127StrsliceCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v127strslice.exe")
			if res.ExitCode != 0 || !res.WroteImage {
				t.Fatalf("kcc produced no image (exit %d):\n%s", res.ExitCode, res.Output)
			}
			if !kccProducedImage(res.Output) {
				t.Fatalf("no kcc provenance:\n%s", res.Output)
			}
			got, code := v123RunImage(t, res.Image)
			if code != 0 {
				t.Fatalf("exit = %d (want 0), stdout = % x", code, got)
			}
			if string(got) != c.want {
				t.Fatalf("stdout = %q, want %q\nsource:\n%s", got, c.want, c.src)
			}
		})
	}
}

func TestPhaseV127Strslice_RefusalsAreByName(t *testing.T) {
	karkain := v126Setup(t)
	for _, c := range v127StrsliceRefusals {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v127strsliceref.exe")
			if res.ExitCode == 0 {
				t.Fatalf("expected refusal, got success (image written: %v)\nsource:\n%s", res.WroteImage, c.src)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Fatalf("refusal output does not name %q:\n%s", c.want, res.Output)
			}
		})
	}
}
