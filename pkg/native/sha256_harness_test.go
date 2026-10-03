package native

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// Phase 152-B1: verification harness for the future native karkain_sha256_hex
// helper. THIS FILE CONTAINS NO SHA-256 IMPLEMENTATION -- it is data, a probe
// and a comparison, so that the helper can be landed and proven one boundary at
// a time without the tests becoming a second implementation to debug.
//
// The helper it is written to accept is
//
//	karkain_sha256_hex(ptr, len, filePtr, fileLen, line) -> (RAX=ptr, RDX=64)
//
// and the builtin name is `sha256_hex`, matching the C23 reference
// `karkain_sha256` in pkg/codegen/codegen.go.
//
// Where the incremental boundaries plug in:
//
//	A. Message/padding -- NOT implemented here. Verifying the pad buffer
//	   (0x80 marker, big-endian bit length, 512-bit block sizing, first-NUL
//	   truncation) means reading the helper's own frame, which is only
//	   reachable from inside it. Land those assertions in emitSHA256Helper's
//	   tests, not here.
//	B. Message schedule -- NOT implemented here. Checking W[0..63] would mean
//	   reproducing the schedule, which is precisely the second implementation
//	   this file must not become.
//	C. Compression -- same reason.
//	D. Final digest -- THIS FILE. The only externally observable state is the
//	   64 hex characters, and a wrong digest says *that* it is wrong but never
//	   *where*, so A-C are the only way to localise a failure once D goes red.
//
// C23 parity: pkg/native has no C23 runner -- that path needs gcc and lives at
// CLI level. Parity is therefore established by transitivity: the C23
// `karkain_sha256` and the native helper are independent implementations of the
// same FIPS 180-4 algorithm, so both matching these same published vectors
// pins them to the same behaviour. A direct native-vs-C23 differential belongs
// beside the existing both-engine corpus gates, not here.

// sha256KATs are the published FIPS 180-4 known-answer digests. They are
// hardcoded deliberately: TestSHA256Harness_VectorsAreAuthoritative proves
// them against crypto/sha256, so the native comparison below cannot be made
// tautological by both sides deriving from the same Go call.
var sha256KATs = []struct{ name, in, wantHex string }{
	{"empty", "", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	{"abc", "abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	{"hello", "hello", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
}

// The multi-block vector is 85 bytes. SHA-256 pads to the next 512-bit
// boundary after a 1-byte 0x80 marker and an 8-byte length, so any message
// longer than 55 bytes needs a second block -- this one crosses that boundary
// (55 < 85 <= 119) and is the case a single-block implementation would get
// wrong silently.
const (
	sha256MultiBlockIn  = "The quick brown fox jumps over the lazy dog. Pack my box with five dozen liquor jugs."
	sha256MultiBlockOut = "d51712a8d1852b5acf942c19caddf168f80120d2f3a72c2d917227fd37f22788"
)

// sha256AllVectors returns the four required cases with their expected digests.
func sha256AllVectors() []struct{ name, in, wantHex string } {
	v := append([]struct{ name, in, wantHex string }{}, sha256KATs...)
	return append(v, struct{ name, in, wantHex string }{"multi_block", sha256MultiBlockIn, sha256MultiBlockOut})
}

// TestSHA256Harness_VectorsAreAuthoritative checks the harness's own test data
// against Go's crypto/sha256. It needs no native code, so it runs today and
// will keep passing after the helper lands: it guards the fixtures, not the
// implementation.
func TestSHA256Harness_VectorsAreAuthoritative(t *testing.T) {
	for _, v := range sha256AllVectors() {
		t.Run(v.name, func(t *testing.T) {
			sum := sha256.Sum256([]byte(v.in))
			if got := hex.EncodeToString(sum[:]); got != v.wantHex {
				t.Fatalf("fixture digest for %q is wrong:\n  harness: %s\n  crypto : %s", v.name, v.wantHex, got)
			}
		})
	}
}

// sha256NativeAvailable reports whether the native backend can compile a
// program that calls the builtin. Until emitSHA256Helper lands it cannot, and
// the harness says so loudly rather than passing quietly.
func sha256NativeAvailable(t *testing.T) bool {
	t.Helper()
	_, err := CompileProgramForOS(parseNative(t, "func main() {\n\tprint(sha256_hex(\"x\"))\n}\n"), OSWindows)
	return err == nil
}

// runSHA256Native compiles and executes a program that prints sha256_hex(in)
// and returns its stdout with the trailing newline removed. This is the single
// entry point the acceptance test uses: it exercises helper invocation, the
// (RAX, RDX) return convention and the allocated output contents together.
func runSHA256Native(t *testing.T, in string) string {
	t.Helper()
	prog := "func main() {\n\tlet s = \"" + in + "\"\n\tprint(sha256_hex(s))\n}\n"
	img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, code := runNativeWindows(t, img)
	if code != 0 {
		t.Fatalf("exit=%d out=%q", code, out)
	}
	return strings.TrimRight(out, "\r\n")
}

// TestSHA256Harness_NativeDigest is the acceptance test for
// karkain_sha256_hex. It is skipped -- not passed -- while the helper is
// absent, so an unimplemented helper can never read as green.
func TestSHA256Harness_NativeDigest(t *testing.T) {
	if !sha256NativeAvailable(t) {
		t.Skip("native karkain_sha256_hex helper not implemented yet; " +
			"harness and vectors are in place and TestSHA256Harness_VectorsAreAuthoritative still runs")
	}
	for _, v := range sha256AllVectors() {
		t.Run(v.name, func(t *testing.T) {
			got := runSHA256Native(t, v.in)
			// The (RAX, RDX) convention promises RDX == 64 exactly: 64
			// lowercase hex digits, never the 65th NUL byte.
			if len(got) != 64 {
				t.Fatalf("digest length = %d, want 64 (raw=%q)", len(got), got)
			}
			if got != v.wantHex {
				t.Errorf("sha256_hex(%q)\n  native : %s\n  expect : %s", v.name, got, v.wantHex)
			}
		})
	}
}
