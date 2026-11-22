package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// v1.2.5b -- native base64_encode_bytes in the self-hosted driver.
// Same contract as hex: the encoded VALUE, not the helper's bytes. The PE is
// executed with exact stdout bytes and compared with the C backend.
type v125bCase struct {
	name string
	src  string
	want string
}

var v125bCases = []v125bCase{
	{"one_group", "func main() { print(base64_encode_bytes(\"foo\")) }\n", "Zm9v\n"},
	{"empty", "func main() { print(base64_encode_bytes(\"\")) }\n", "\n"},
	{"one_byte", "func main() { print(base64_encode_bytes(\"f\")) }\n", "Zg==\n"},
	{"two_bytes", "func main() { print(base64_encode_bytes(\"fo\")) }\n", "Zm8=\n"},
	{"four_bytes", "func main() { print(base64_encode_bytes(\"foob\")) }\n", "Zm9vYg==\n"},
	{"three_groups", "func main() { print(base64_encode_bytes(\"foobar\")) }\n", "Zm9vYmFy\n"},
}

func v125bSetup(t *testing.T) string {
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

func TestPhaseV125B_ExecutedPEWithExactBytes(t *testing.T) {
	karkain := v125bSetup(t)
	for _, c := range v125bCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v125b.exe")
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

func TestPhaseV125B_MatchesCBackend(t *testing.T) {
	karkain := v125bSetup(t)
	for _, c := range v125bCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v125b-cmp.exe")
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

func TestPhaseV125B_CorpusIsNonVacuous(t *testing.T) {
	if len(v125bCases) < 4 {
		t.Fatal("the base64-encode execution corpus is too small")
	}
	for _, c := range v125bCases {
		if !strings.Contains(c.src, "base64_encode_bytes") {
			t.Fatalf("case %q does not call base64_encode_bytes", c.name)
		}
	}
}
