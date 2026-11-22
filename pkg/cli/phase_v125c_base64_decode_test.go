package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// v1.2.5c -- native base64_decode_bytes. Contract is the decoded VALUE,
// checked by exact PE stdout and a C-backend differential; no byte-identity
// with the Go oracle (arithmetic decode vs its lookup table).
type v125cCase struct {
	name string
	src  string
	want string
}

var v125cCases = []v125cCase{
	{"one_group", "func main() { print(base64_decode_bytes(\"Zm9v\")) }\n", "foo\n"},
	{"empty", "func main() { print(base64_decode_bytes(\"\")) }\n", "\n"},
	{"one_pad", "func main() { print(base64_decode_bytes(\"Zg==\")) }\n", "f\n"},
	{"two_pad", "func main() { print(base64_decode_bytes(\"Zm8=\")) }\n", "fo\n"},
	{"two_groups", "func main() { print(base64_decode_bytes(\"Zm9vYmFy\")) }\n", "foobar\n"},
	{"upper_lower_digits", "func main() { print(base64_decode_bytes(\"QUJDRA==\")) }\n", "ABCD\n"},
}

func v125cSetup(t *testing.T) string {
	t.Helper()
	if os.PathSeparator == '/' { t.Skip("PE images only execute on Windows") }
	v123EnsurePATH(t)
	hasGCC(t)
	kccBin := phase95KCC(t)
	selectKCCEngine(t, kccBin)
	return phase130Karkain(t)
}

func TestPhaseV125C_ExecutedPEWithExactBytes(t *testing.T) {
	karkain := v125cSetup(t)
	for _, c := range v125cCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v125c.exe")
			if res.ExitCode != 0 || !res.WroteImage { t.Fatalf("no image (exit %d):\n%s", res.ExitCode, res.Output) }
			if !kccProducedImage(res.Output) { t.Fatalf("no kcc provenance:\n%s", res.Output) }
			got, code := v123RunImage(t, res.Image)
			if code != 0 { t.Fatalf("exit = %d, stdout = % x", code, got) }
			if string(got) != c.want { t.Fatalf("stdout = %q, want %q\nsource:\n%s", got, c.want, c.src) }
		})
	}
}

func TestPhaseV125C_MatchesCBackend(t *testing.T) {
	karkain := v125cSetup(t)
	for _, c := range v125cCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v125c-cmp.exe")
			if res.ExitCode != 0 || !res.WroteImage { t.Fatalf("no image:\n%s", res.Output) }
			nativeOut, nativeCode := v123RunImage(t, res.Image)
			cOut, cCode := natIRunCBackend(t, karkain, c.src)
			cOut = natINormalizeCRLF(cOut)
			if nativeCode != cCode || !bytes.Equal(nativeOut, cOut) {
				t.Fatalf("native/C mismatch:\n  native: % x exit %d\n  c    : % x exit %d", nativeOut, nativeCode, cOut, cCode)
			}
		})
	}
}

func TestPhaseV125C_InvalidInputIsRuntimeError(t *testing.T) {
	karkain := v125cSetup(t)
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"bad_length", "func main() { print(base64_decode_bytes(\"Zm9\")) }\n", "invalid base64 string (length)"},
		{"bad_symbol", "func main() { print(base64_decode_bytes(\"Zm*v\")) }\n", "invalid base64 string"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v125c-bad.exe")
			if res.ExitCode != 0 || !res.WroteImage { t.Fatalf("no image:\n%s", res.Output) }
			got, code := nat9dRun(t, res.Image)
			if code == 0 || len(got) != 0 { t.Fatalf("invalid base64: exit %d stdout % x", code, got) }
			if !strings.Contains(string(res.Image), c.want) { t.Errorf("image lacks diagnostic %q", c.want) }
		})
	}
}

func TestPhaseV125C_CorpusIsNonVacuous(t *testing.T) {
	if len(v125cCases) < 4 { t.Fatal("corpus too small") }
	for _, c := range v125cCases {
		if !strings.Contains(c.src, "base64_decode_bytes") { t.Fatalf("case %q missing call", c.name) }
	}
}
