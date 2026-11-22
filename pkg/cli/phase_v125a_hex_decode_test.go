package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// v1.2.5a -- native hex_decode_bytes in the self-hosted driver.
//
// The contract is the decoded VALUE, not the helper's bytes: the oracle uses a
// 256-entry lookup table, which Karkain cannot embed as a byte-string literal,
// so kcc decodes nibbles arithmetically. Binary identity is deliberately NOT
// claimed. The Windows PE is executed with exact stdout bytes and the same
// source is compared with the C backend, the language oracle for stdout.
type v125aCase struct {
	name string
	src  string
	want string
}

var v125aCases = []v125aCase{
	{"lowercase", "func main() { print(hex_decode_bytes(\"68656c6c6f\")) }\n", "hello\n"},
	{"empty", "func main() { print(hex_decode_bytes(\"\")) }\n", "\n"},
	{"uppercase", "func main() { print(hex_decode_bytes(\"4849\")) }\n", "HI\n"},
	{"mixed_case", "func main() { print(hex_decode_bytes(\"4869\")) }\n", "Hi\n"},
	{"single_byte", "func main() { print(hex_decode_bytes(\"41\")) }\n", "A\n"},
	{"bound_literal", "func main() { let s = \"6869\" print(hex_decode_bytes(s)) }\n", "hi\n"},
}

func v125aSetup(t *testing.T) string {
	t.Helper()
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	v123EnsurePATH(t)
	hasGCC(t)
	kccBin := phase95KCC(t)
	selectKCCEngine(t, kccBin)
	return phase130Karkain(t)
}

func TestPhaseV125A_ExecutedPEWithExactBytes(t *testing.T) {
	karkain := v125aSetup(t)
	for _, c := range v125aCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v125a.exe")
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

func TestPhaseV125A_MatchesCBackend(t *testing.T) {
	karkain := v125aSetup(t)
	for _, c := range v125aCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v125a-cmp.exe")
			if res.ExitCode != 0 || !res.WroteImage {
				t.Fatalf("kcc produced no image:\n%s", res.Output)
			}
			nativeOut, nativeCode := v123RunImage(t, res.Image)
			cOut, cCode := natIRunCBackend(t, karkain, c.src)
			cOut = natINormalizeCRLF(cOut)
			if nativeCode != cCode || !bytes.Equal(nativeOut, cOut) {
				t.Fatalf("native/C mismatch:\n  native: % x exit %d\n  c    : % x exit %d",
					nativeOut, nativeCode, cOut, cCode)
			}
		})
	}
}

func TestPhaseV125A_InvalidInputIsRuntimeError(t *testing.T) {
	karkain := v125aSetup(t)
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"non_hex_letter", "func main() { print(hex_decode_bytes(\"zz\")) }\n", "invalid hex string"},
		{"odd_length", "func main() { print(hex_decode_bytes(\"6\")) }\n", "invalid hex string (odd length)"},
		{"bad_second_digit", "func main() { print(hex_decode_bytes(\"0g\")) }\n", "invalid hex string"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v125a-bad.exe")
			if res.ExitCode != 0 || !res.WroteImage {
				t.Fatalf("kcc produced no image:\n%s", res.Output)
			}
			got, code := nat9dRun(t, res.Image)
			if code == 0 {
				t.Fatalf("invalid hex exited 0; stdout = % x", got)
			}
			if len(got) != 0 {
				t.Fatalf("invalid hex printed a value before failing: % x", got)
			}
			if !strings.Contains(string(res.Image), c.want) {
				t.Errorf("the image does not carry the diagnostic text %q", c.want)
			}
		})
	}
}

func TestPhaseV125A_CorpusIsNonVacuous(t *testing.T) {
	if len(v125aCases) < 4 {
		t.Fatal("the hex-decode execution corpus is too small to be evidence")
	}
	for _, c := range v125aCases {
		if c.name == "" || c.src == "" || c.want == "" {
			t.Fatalf("incomplete corpus case: %+v", c)
		}
		if !strings.Contains(c.src, "hex_decode_bytes") {
			t.Fatalf("corpus case %q does not call hex_decode_bytes", c.name)
		}
	}
}
