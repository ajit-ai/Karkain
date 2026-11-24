package cli

import (
	"bytes"
	"strings"
	"testing"
)

// v1.2.7 -- native string EQUALITY (`==`/`!=` over strings, condition position).
// NAT-STREQ, ported from feature/v1.3.0-natstdlib-string (branch fce3282),
// re-gated for v1.2.7.

type v127Case struct {
	name string
	src  string
	want string
}

var v127StreqCases = []v127Case{
	{"literal_eq_true", "func main() { if (\"abc\" == \"abc\") { print(1) } else { print(0) } }\n", "1\n"},
	{"literal_eq_false", "func main() { if (\"abc\" != \"abc\") { print(1) } else { print(0) } }\n", "0\n"},
	{"two_literals_ne", "func main() { if (\"abc\" != \"abd\") { print(1) } else { print(0) } }\n", "1\n"},
	{"two_locals_eq", "func main() { let a = \"x\"\nlet b = \"x\"\nif (a == b) { print(1) } }\n", "1\n"},
	{"two_locals_ne", "func main() { let a = \"x\"\nlet b = \"y\"\nif (a == b) { print(1) } else { print(0) } }\n", "0\n"},
	{"two_empty_locals_eq", "func main() { let a = \"\"\nlet b = \"\"\nif (a == b) { print(1) } }\n", "1\n"},
	{"empty_vs_one_byte", "func main() { let a = \"\"\nlet b = \"x\"\nif (a == b) { print(1) } else { print(0) } }\n", "0\n"},
	{"self_ne_is_false", "func main() { let s = \"abc\"\nif (s != s) { print(1) } else { print(0) } }\n", "0\n"},
	{"for_loop_eq", "func main() { let a = \"xyz\"\nfor (let i = 0; i < 2; i = i + 1) { if (a == \"xyz\") { print(1) } } }\n", "1\n1\n"},
	// The staging-area ledger has to see an equality in every body shape the
	// driver lowers, or the equality's staging overwrites a slot the enclosing
	// loop owns. `for_loop_eq` covers the `for` arm; these cover `while` and a
	// nested `if`. Both were RED before the site counters descended.
	{"while_loop_eq", "func main() { let a = \"xyz\"\nlet i = 0\nwhile (i < 2) { if (a == \"xyz\") { print(1) }\ni = i + 1 } }\n", "1\n1\n"},
	{"nested_if_eq", "func main() { let a = \"x\"\nif (1 == 1) { if (a == \"x\") { print(1) } } }\n", "1\n"},
}

var v127StreqRefusals = []struct {
	name string
	src  string
	want string
}{
	{"ordering_less", "func main() { let a = \"x\"\nif (a < \"y\") { print(1) } }\n",
		"string ordering ('<') is not supported; == and != compare content"},
	{"ordering_greater", "func main() { let a = \"x\"\nif (a > \"y\") { print(1) } }\n",
		"string ordering ('>') is not supported; == and != compare content"},
	{"int_operand", "func main() { let a = \"x\"\nif (a == 1) { print(1) } }\n",
		"int operand in string comparison (no implicit conversion)"},
	{"float_operand", "func main() { let a = \"x\"\nif (a == 1.5) { print(1) } }\n",
		"float operand in string comparison (no implicit conversion)"},
}

func TestPhaseV127Streq_ExecutedPEWithExactBytes(t *testing.T) {
	karkain := v126Setup(t)
	for _, c := range v127StreqCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v127streq.exe")
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

func TestPhaseV127Streq_RefusalsAreByName(t *testing.T) {
	karkain := v126Setup(t)
	for _, c := range v127StreqRefusals {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v127streqref.exe")
			if res.ExitCode == 0 {
				t.Fatalf("expected refusal, got success (image written: %v)\nsource:\n%s", res.WroteImage, c.src)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Fatalf("refusal output does not name %q:\n%s", c.want, res.Output)
			}
		})
	}
}

func TestPhaseV127Streq_Deterministic(t *testing.T) {
	karkain := v126Setup(t)
	src := "func main() { let a = \"xyz\"\nfor (let i = 0; i < 2; i = i + 1) { if (a == \"xyz\") { print(1) } } }\n"
	r1 := v123Build(t, karkain, src, NativeWindowsTarget, "kcc", "v127streqd1.exe")
	r2 := v123Build(t, karkain, src, NativeWindowsTarget, "kcc", "v127streqd2.exe")
	if r1.ExitCode != 0 || r2.ExitCode != 0 || len(r1.Image) == 0 {
		t.Fatalf("builds failed: %d %d (image len %d)", r1.ExitCode, r2.ExitCode, len(r1.Image))
	}
	if !bytes.Equal(r1.Image, r2.Image) {
		t.Fatal("deterministic build: images differ between two builds of the same source")
	}
}
