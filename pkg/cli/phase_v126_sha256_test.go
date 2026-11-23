package cli

import (
	"bytes"
	"math"
	"os"
	"strings"
	"testing"
)

// v1.2.6 -- native sha256_hex in the self-hosted driver.
//
// The contract is the digest VALUE: 64 lowercase hex characters, with the C
// runtime's strlen truncation at the first NUL. The Go native backend already
// has its own SHA-256 helper, so this gate can compare three independent
// implementations: kcc's native PE, the Go native PE, and the C backend.
// Byte-identity between kcc and Go is NOT claimed: kcc materialises the K table
// in the helper frame and uses true 32-bit ALU forms, while Go uses rodata and
// 64-bit ops with a mask. The observable contract is stdout.
type v126Case struct {
	name string
	src  string
	want string // digest without the trailing newline
}

var v126Cases = []v126Case{
	{"empty", "func main() { print(sha256_hex(\"\")) }\n", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	{"abc", "func main() { print(sha256_hex(\"abc\")) }\n", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	{"karkain_bound", "func main() { let s = \"karkain\" print(sha256_hex(s)) }\n", "00e0cba20c10cac449eb885a9926a4b646f0ac163ed7fbc5704d9d8a057ef44d"},
	{"hello", "func main() { print(sha256_hex(\"hello\")) }\n", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
	{"pad55", "func main() { print(sha256_hex(\"" + strings.Repeat("a", 55) + "\")) }\n", "9f4390f8d30c2dd92ec9f095b65e2b9ae9b0a925a5258e241c9f1e910f734318"},
	{"pad56", "func main() { print(sha256_hex(\"" + strings.Repeat("a", 56) + "\")) }\n", "b35439a4ac6f0948b6d6f9e3c6af0f5f590ce20f1bde7090ef7970686ec6738a"},
	{"block64", "func main() { print(sha256_hex(\"" + strings.Repeat("a", 64) + "\")) }\n", "ffe054fe7ae0cb6dc65c3af9b61d5209f439851db43d0ba5997337df154668eb"},
	{"multi85", "func main() { print(sha256_hex(\"The quick brown fox jumps over the lazy dog. Pack my box with five dozen liquor jugs.\")) }\n", "d51712a8d1852b5acf942c19caddf168f80120d2f3a72c2d917227fd37f22788"},
	{"digest_of_digest", "func main() { let a = sha256_hex(\"abc\") print(sha256_hex(a)) }\n", "dfe7a23fefeea519e9bbfdd1a6be94c4b2e4529dd6b7cbea83f9959c2621b13c"},
	{"mixed_concat_sha", "func main() { let a = sha256_hex(\"abc\") let b = a + \"x\" print(sha256_hex(b)) }\n", "f1d9eefa15d6125b72ddf704ef43d2d689486b6044469282d1fd170f31bd6774"},
	{"nul_truncation", "func main() { let s = hex_decode_bytes(\"610062\") print(sha256_hex(s)) }\n", "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"},
}

func v126Setup(t *testing.T) string {
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

func TestPhaseV126_ExecutedPEWithExactBytes(t *testing.T) {
	karkain := v126Setup(t)
	for _, c := range v126Cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v126.exe")
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
			want := c.want + "\n"
			if string(got) != want {
				t.Fatalf("stdout = %q, want %q\nsource:\n%s", got, want, c.src)
			}
		})
	}
}

func TestPhaseV126_MatchesCBackend(t *testing.T) {
	karkain := v126Setup(t)
	for _, c := range v126Cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v126-cmp.exe")
			if res.ExitCode != 0 || !res.WroteImage {
				t.Fatalf("kcc produced no image:\n%s", res.Output)
			}
			nativeOut, nativeCode := v123RunImage(t, res.Image)
			cOut, cCode := natIRunCBackend(t, karkain, c.src)
			cOut = natINormalizeCRLF(cOut)
			if nativeCode != cCode || !bytes.Equal(nativeOut, cOut) {
				t.Fatalf("native/C mismatch:\n  native: % x exit %d\n  c    : % x exit %d\nsource:\n%s",
					nativeOut, nativeCode, cOut, cCode, c.src)
			}
		})
	}
}

func TestPhaseV126_MatchesGoNativeBackend(t *testing.T) {
	karkain := v126Setup(t)
	// A representative subset: the Go native helper is a separate
	// implementation, so agreement on these shapes is a strong cross-check.
	subset := []string{"abc", "multi85", "digest_of_digest", "nul_truncation"}
	for _, name := range subset {
		var c v126Case
		for _, cc := range v126Cases {
			if cc.name == name {
				c = cc
				break
			}
		}
		if c.name == "" {
			t.Fatalf("missing case %q", name)
		}
		t.Run(c.name, func(t *testing.T) {
			kccRes := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v126-kcc.exe")
			goRes := v123Build(t, karkain, c.src, NativeWindowsTarget, "go", "v126-go.exe")
			if kccRes.ExitCode != 0 || !kccRes.WroteImage {
				t.Fatalf("kcc build failed:\n%s", kccRes.Output)
			}
			if goRes.ExitCode != 0 || !goRes.WroteImage {
				t.Fatalf("go native build failed:\n%s", goRes.Output)
			}
			kOut, kCode := v123RunImage(t, kccRes.Image)
			gOut, gCode := v123RunImage(t, goRes.Image)
			if kCode != gCode || !bytes.Equal(kOut, gOut) {
				t.Fatalf("kcc/go-native mismatch:\n  kcc: % x exit %d\n  go : % x exit %d\nsource:\n%s",
					kOut, kCode, gOut, gCode, c.src)
			}
		})
	}
}

func TestPhaseV126_RefusalsAreByName(t *testing.T) {
	karkain := v126Setup(t)
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"no_args", "func main() { print(sha256_hex()) }\n", "sha256_hex"},
		{"two_args", "func main() { print(sha256_hex(\"a\", \"b\")) }\n", "sha256_hex"},
		{"int_arg", "func main() { let n = 5 print(sha256_hex(n)) }\n", "requires a string argument"},
		{"return_string", "func main() { return sha256_hex(\"x\") }\n", "`return` of a string"},
		{"nested_codec_arg", "func main() { print(sha256_hex(hex_decode_bytes(\"616263\"))) }\n", "requires a string argument"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v126-refuse.exe")
			if res.ExitCode == 0 || res.WroteImage {
				t.Fatalf("expected refusal with no image; exit=%d wrote=%v\n%s", res.ExitCode, res.WroteImage, res.Output)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Fatalf("refusal does not name %q:\n%s", c.want, res.Output)
			}
		})
	}
}

func TestPhaseV126_Deterministic(t *testing.T) {
	karkain := v126Setup(t)
	src := "func main() { print(sha256_hex(\"abc\")) }\n"
	a := v123Build(t, karkain, src, NativeWindowsTarget, "kcc", "v126-det1.exe")
	b := v123Build(t, karkain, src, NativeWindowsTarget, "kcc", "v126-det2.exe")
	if a.ExitCode != 0 || b.ExitCode != 0 || !a.WroteImage || !b.WroteImage {
		t.Fatalf("build failed:\n%s\n%s", a.Output, b.Output)
	}
	if !bytes.Equal(a.Image, b.Image) {
		t.Fatalf("image differs between runs: %d vs %d bytes", len(a.Image), len(b.Image))
	}
}

func TestPhaseV126_CorpusIsNonVacuous(t *testing.T) {
	if len(v126Cases) < 8 {
		t.Fatal("the sha256 execution corpus is too small to be evidence")
	}
	seen := map[string]bool{}
	for _, c := range v126Cases {
		if c.name == "" || c.src == "" || len(c.want) != 64 {
			t.Fatalf("incomplete corpus case: %+v", c)
		}
		if !strings.Contains(c.src, "sha256_hex") {
			t.Fatalf("case %q does not call sha256_hex", c.name)
		}
		if seen[c.want] {
			t.Fatalf("duplicate digest in corpus makes a constant-returning helper harder to catch: %q", c.name)
		}
		seen[c.want] = true
	}
}

// v126FirstPrimes and v126Frac32 derive the FIPS 180-4 constants from their
// definition rather than transcribing them. A shared typo in a hand-written
// decimal table cannot match the cube/square roots of primes.
func v126FirstPrimes(n int) []int64 {
	out := []int64{}
	for c := int64(2); len(out) < n; c++ {
		prime := true
		for d := int64(2); d*d <= c; d++ {
			if c%d == 0 {
				prime = false
				break
			}
		}
		if prime {
			out = append(out, c)
		}
	}
	return out
}

func v126Frac32(x float64) uint32 {
	f := x - float64(int64(x))
	return uint32(f * 4294967296.0)
}

func TestPhaseV126_KAndIVConstantsAreInTheImage(t *testing.T) {
	karkain := v126Setup(t)
	src := "func main() { print(sha256_hex(\"abc\")) }\n"
	res := v123Build(t, karkain, src, NativeWindowsTarget, "kcc", "v126-const.exe")
	if res.ExitCode != 0 || !res.WroteImage {
		t.Fatalf("kcc produced no image:\n%s", res.Output)
	}
	check := func(label string, words []uint32) {
		for i, w := range words {
			le := []byte{byte(w), byte(w >> 8), byte(w >> 16), byte(w >> 24)}
			if !bytes.Contains(res.Image, le) {
				t.Errorf("%s[%d] = %08x is not present in the emitted image as little-endian bytes", label, i, w)
			}
		}
	}
	k := make([]uint32, 0, 64)
	for _, p := range v126FirstPrimes(64) {
		k = append(k, v126Frac32(math.Cbrt(float64(p))))
	}
	iv := make([]uint32, 0, 8)
	for _, p := range v126FirstPrimes(8) {
		iv = append(iv, v126Frac32(math.Sqrt(float64(p))))
	}
	check("K", k)
	check("IV", iv)
}
