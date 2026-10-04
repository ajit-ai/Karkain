package native

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"testing"
)

// Phase 152-B2 â€” native karkain_sha256_hex.
//
// The harness in sha256_harness_test.go owns the ACCEPTANCE question (does the
// helper compute the published digests?). This file owns everything the harness
// explicitly deferred, per its own A/B/C/D boundary comment:
//
//	A. Message/padding   â€” here
//	B. Message schedule  â€” here (the constant table + the multi-block vector)
//	C. Compression       â€” the vectors
//	D. Final digest      â€” the harness
//
// and the three properties a helper can get wrong while still producing correct
// digests for the four harness vectors:
//
//   - GATING: the helper must not appear in an image that never calls it, or
//     every pre-152 image would grow (the byte-identity criterion).
//   - ARENA BOUND: sha256_hex allocates a FIXED 65 bytes per site, which the
//     proportional codec bound does not cover (see program.go).
//   - strlen PARITY: the C23 reference truncates at the first NUL.

// sha256FirstPrimes returns the first n primes. Used to derive the two constant
// tables from their FIPS 180-4 DEFINITION rather than comparing the shipped
// table against a second transcription of it -- a transcription can share a typo
// with the original, while cube roots of primes cannot.
func sha256FirstPrimes(n int) []int64 {
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

// sha256Frac32 takes the first 32 bits of the fractional part of x, which is
// exactly how FIPS 180-4 defines both the IV (Â§5.3.3: square roots of the first
// 8 primes) and the round constants (Â§4.2.2: cube roots of the first 64).
func sha256Frac32(x float64) uint32 {
	f := x - float64(int64(x))
	return uint32(f * 4294967296.0)
}

// TestPhase152B2_KTableMatchesFIPS derives all 64 round constants and all 8 IV
// words from the FIPS definition and compares them against the shipped tables.
//
// This is the test sha256.go names in the sha256KWords comment, which records a
// real slip this class of work invites: an earlier draft carried 0x80deb01f
// where the table says 0x80deb1fe.
//
// Measured, not assumed: the digest vectors DO also catch that typo (all four
// fail), so this test is NOT claiming to catch something they miss. What it adds
// is LOCALISATION. The harness says "the digest is wrong"; this says "K[13], the
// prime 43, is 80deb01f and FIPS says 80deb1fe" -- which is the difference
// between bisecting a table by eye and reading the answer.
//
// Deriving rather than transcribing is what makes that localisation trustworthy.
// A second copy of the table in the test would agree with the shipped table
// INCLUDING a shared typo; the cube-root derivation is an INDEPENDENT
// computation that a typo in the Go literal cannot match.
func TestPhase152B2_KTableMatchesFIPS(t *testing.T) {
	if len(sha256KWords) != 64 {
		t.Fatalf("sha256KWords has %d entries, FIPS 180-4 defines 64", len(sha256KWords))
	}
	for i, p := range sha256FirstPrimes(64) {
		want := sha256Frac32(math.Cbrt(float64(p)))
		if got := sha256KWords[i]; got != want {
			t.Errorf("K[%d] (prime %d): table has %08x, FIPS definition gives %08x", i, p, got, want)
		}
	}
	if len(sha256IV) != 8 {
		t.Fatalf("sha256IV has %d entries, FIPS 180-4 defines 8", len(sha256IV))
	}
	for i, p := range sha256FirstPrimes(8) {
		want := sha256Frac32(math.Sqrt(float64(p)))
		if got := sha256IV[i]; got != want {
			t.Errorf("IV[%d] (prime %d): table has %08x, FIPS definition gives %08x", i, p, got, want)
		}
	}
	// The packed table is what the helper actually READS (LoadScaled32 at 4*t),
	// so it must be the little-endian packing of the words, byte for byte. An
	// error here would leave the words correct and the emitted image wrong.
	if len(sha256K) != 256 {
		t.Fatalf("packed K table is %d bytes, want 256", len(sha256K))
	}
	for i, w := range sha256KWords {
		want := []byte{byte(w), byte(w >> 8), byte(w >> 16), byte(w >> 24)}
		if got := []byte(sha256K[i*4 : i*4+4]); !bytes.Equal(got, want) {
			t.Errorf("packed K[%d] = % x, want % x (little-endian)", i, got, want)
		}
	}
}

// TestPhase152B2_KTableIsInTheImage proves the derived table reaches the
// emitted bytes. A correct table that is never referenced, or referenced at the
// wrong width, would pass every digest vector only by accident of the fixture
// inputs; this asserts the 256 packed bytes are present verbatim.
func TestPhase152B2_KTableIsInTheImage(t *testing.T) {
	prog := "func main() {\n\tprint(sha256_hex(\"abc\"))\n}\n"
	img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !bytes.Contains(img, []byte(sha256K)) {
		t.Error("the packed K table is not present in the emitted image")
	}
	if !bytes.Contains(img, []byte(sha256HexDigits)) {
		t.Error("the lowercase hex alphabet is not present in the emitted image")
	}
}

// TestPhase152B2_GatedOnUse is the byte-identity criterion as a gate.
//
// sha256_hex is a gated helper, like the three 152-B1 codecs and print_float
// before it: emitted only when the program calls it. If the gate were dropped,
// every pre-152 image would grow by the whole helper and
// TestNativeELFByteIdentity would fail -- but that test only covers 19 specific
// programs, so this asserts the property directly on a pair of images.
func TestPhase152B2_GatedOnUse(t *testing.T) {
	withCall := "func main() {\n\tprint(sha256_hex(\"abc\"))\n}\n"
	without := "func main() {\n\tprint(\"abc\")\n}\n"
	imgWith, err := CompileProgramForOS(parseNative(t, withCall), OSWindows)
	if err != nil {
		t.Fatalf("compile (with sha256_hex): %v", err)
	}
	imgWithout, err := CompileProgramForOS(parseNative(t, without), OSWindows)
	if err != nil {
		t.Fatalf("compile (without sha256_hex): %v", err)
	}
	// A reliable proxy for "the helper was emitted": the K table is 256 bytes
	// of .rodata referenced only by the helper.
	if !bytes.Contains(imgWith, []byte(sha256K)) {
		t.Error("a program calling sha256_hex does not carry the K table")
	}
	if bytes.Contains(imgWithout, []byte(sha256K)) {
		t.Error("a program NOT calling sha256_hex carries the K table: the helper is not gated")
	}
	if len(imgWith) <= len(imgWithout) {
		t.Errorf("image with the helper (%d bytes) is not larger than the one without (%d bytes)",
			len(imgWith), len(imgWithout))
	}
}

// TestPhase152B2_ArenaBoundCoversFixedCost pins the fixed 65-bytes-per-site
// arena contribution.
//
// The proportional codec bound (sites * (2*maxIn + 16)) is derived from the
// INPUT LENGTH, so it does not cover sha256_hex: for a one-byte input it yields
// 18 bytes against a real 65-byte allocation, and the allocator's exhaustion
// trap would fire mid-run. The bound therefore adds 65 per call site.
//
// This asserts the property that matters -- a program with several sha256_hex
// sites runs to completion -- rather than restating the arithmetic, because the
// arithmetic is what a future edit would get wrong silently.
func TestPhase152B2_ArenaBoundCoversFixedCost(t *testing.T) {
	// Three sha256 sites, the last consuming a previous digest, plus a concat,
	// so the arena must cover the codecs AND the fixed sha256 cost together.
	prog := "func main() {\n" +
		"\tlet a = sha256_hex(\"abc\")\n" +
		"\tlet b = sha256_hex(\"hello\")\n" +
		"\tlet c = sha256_hex(a)\n" +
		"\tlet d = \"x\" + \"y\"\n" +
		"\tprint(a)\n\tprint(b)\n\tprint(c)\n\tprint(d)\n}\n"
	img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, code := runNativeWindows(t, img)
	if code != 0 {
		t.Fatalf("exit=%d out=%q (an exhausted arena traps, it does not exit cleanly)", code, out)
	}
	sa := sha256.Sum256([]byte("abc"))
	sb := sha256.Sum256([]byte("hello"))
	sc := sha256.Sum256([]byte(hex.EncodeToString(sa[:])))
	want := strings.Join([]string{
		hex.EncodeToString(sa[:]),
		hex.EncodeToString(sb[:]),
		hex.EncodeToString(sc[:]),
		"xy",
	}, "\n")
	if got := lh2Trim(out); got != want {
		t.Errorf("multi-site output:\n  got  %q\n  want %q", got, want)
	}
}

// TestPhase152B2_StrlenTruncationParity pins the C23 reference's strlen
// contract.
//
// karkain_sha256 in pkg/codegen/codegen.go measures its input with strlen, so
// the digest stops at the first NUL. A native string carries an explicit
// length, so WITHOUT the truncation scan a value containing a NUL would hash
// MORE bytes than the reference and the two engines would silently diverge on
// exactly the inputs binary data produces.
//
// The NUL is introduced through hex_decode_bytes (which yields "a\0b", length
// 3), so the fixture needs no escape syntax the native parser may not have.
// The expected digest is sha256("a") -- the truncated message.
func TestPhase152B2_StrlenTruncationParity(t *testing.T) {
	prog := "func main() {\n\tlet s = hex_decode_bytes(\"610062\")\n\tprint(sha256_hex(s))\n}\n"
	img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, code := runNativeWindows(t, img)
	if code != 0 {
		t.Fatalf("exit=%d out=%q", code, out)
	}
	truncated := sha256.Sum256([]byte("a"))
	full := sha256.Sum256([]byte{'a', 0, 'b'})
	if hex.EncodeToString(truncated[:]) == hex.EncodeToString(full[:]) {
		t.Fatal("fixture is vacuous: the truncated and untruncated digests are equal")
	}
	if got, want := lh2Trim(out), hex.EncodeToString(truncated[:]); got != want {
		t.Errorf("sha256_hex over a string with an embedded NUL:\n  got  %s\n  want %s (strlen semantics)\n  untruncated would be %s",
			got, want, hex.EncodeToString(full[:]))
	}
}

// TestPhase152B2_LengthIsExactly64 pins the (RAX=ptr, RDX=64) return
// convention. The helper allocates 65 bytes (64 digits plus the NUL the
// reference writes) and must return the length 64, so the terminator is never
// part of the value.
//
// It is observable through SLICING rather than len(), because native `len()`
// accepts only arrays and maps (program.go: "len() requires an array or a map
// argument") and a string length is not otherwise exposed. `d[64:64]` is the
// empty slice only when the length really is 64; a returned 65 would make it
// the one-byte NUL. Comparing it to a literal is the check that distinguishes
// those two, since printing a NUL is not reliably visible in captured output.
func TestPhase152B2_LengthIsExactly64(t *testing.T) {
	// The empty slice must compare equal to "" ...
	progEmpty := "func main() {\n\tlet d = sha256_hex(\"abc\")\n\tif d[64:64] == \"\" {\n\t\tprint(\"empty\")\n\t} else {\n\t\tprint(\"notempty\")\n\t}\n}\n"
	// ... and the 64th character must be the last hex digit, so the value is
	// not merely short but exactly the digest.
	progLast := "func main() {\n\tlet d = sha256_hex(\"abc\")\n\tprint(d[63:64])\n}\n"
	img, err := CompileProgramForOS(parseNative(t, progEmpty), OSWindows)
	if err != nil {
		t.Fatalf("compile (empty slice): %v", err)
	}
	out, code := runNativeWindows(t, img)
	if code != 0 {
		t.Fatalf("empty-slice program: exit=%d out=%q", code, out)
	}
	if got, want := lh2Trim(out), "empty"; got != want {
		t.Errorf("d[64:64] is %s, want %s (the returned length is not exactly 64)", got, want)
	}
	img2, err := CompileProgramForOS(parseNative(t, progLast), OSWindows)
	if err != nil {
		t.Fatalf("compile (last char): %v", err)
	}
	out2, code2 := runNativeWindows(t, img2)
	if code2 != 0 {
		t.Fatalf("last-char program: exit=%d out=%q", code2, out2)
	}
	sa := sha256.Sum256([]byte("abc"))
	if got, want := lh2Trim(out2), hex.EncodeToString(sa[:])[63:64]; got != want {
		t.Errorf("d[63:64] = %q, want %q", got, want)
	}
}

// TestPhase152B2_Rejections pins the two K145 refusals on the shared codec call
// site, and asserts the NAMES differ so a single generic message cannot satisfy
// both.
func TestPhase152B2_Rejections(t *testing.T) {
	cases := []struct{ name, prog, want string }{
		{"no_args", "func main() {\n\tprint(sha256_hex())\n}\n", "takes 1 argument"},
		{"two_args", "func main() {\n\tprint(sha256_hex(\"a\", \"b\"))\n}\n", "takes 1 argument"},
		{"int_arg", "func main() {\n\tlet n = 5\n\tprint(sha256_hex(n))\n}\n", "requires a string argument"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := CompileProgramForOS(parseNative(t, c.prog), OSWindows)
			if err == nil {
				t.Fatalf("compiled with no error; want a K145 refusal naming %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not name %q", err.Error(), c.want)
			}
			if !strings.Contains(err.Error(), "sha256_hex") {
				t.Errorf("error %q does not name the builtin", err.Error())
			}
		})
	}
	if cases[0].want == cases[2].want {
		t.Fatal("fixture is vacuous: the arity and type refusals expect the same text")
	}
}

// TestPhase152B2_Deterministic guards the 152-NFR-2 property at the slice's own
// boundary: the same program must produce byte-identical images twice. Digest
// CORRECTNESS does not imply determinism -- a helper that emitted an
// uninitialised padding byte could hash correctly by luck on one build.
func TestPhase152B2_Deterministic(t *testing.T) {
	prog := "func main() {\n\tprint(sha256_hex(\"The quick brown fox jumps over the lazy dog.\"))\n}\n"
	var first []byte
	for i := 0; i < 2; i++ {
		img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
		if err != nil {
			t.Fatalf("compile %d: %v", i, err)
		}
		if i == 0 {
			first = img
			continue
		}
		if !bytes.Equal(first, img) {
			t.Errorf("image differs between runs: %d vs %d bytes", len(first), len(img))
		}
	}
}

// TestPhase152B2_AntiVacuity fails if the helper ever stops being emitted. The
// harness SKIPS when the helper is absent (correctly, so it can predate it), so
// without this a regression that removed the helper would turn the acceptance
// test into a skip that reads as green.
func TestPhase152B2_AntiVacuity(t *testing.T) {
	if !sha256NativeAvailable(t) {
		t.Fatal("sha256_hex does not compile natively; the harness would silently SKIP its acceptance test")
	}
	if len(sha256K) != 256 || len(sha256KWords) != 64 || len(sha256IV) != 8 {
		t.Fatal("a constant table is empty or the wrong size; the digest vectors would be measuring nothing")
	}
	for _, v := range sha256AllVectors() {
		if len(v.wantHex) != 64 {
			t.Fatalf("vector %q has a %d-character digest; the fixtures are not 64-hex", v.name, len(v.wantHex))
		}
	}
}
