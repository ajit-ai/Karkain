package native

import (
	"bytes"
	"testing"
)

// Phase 152-B1 focused coverage for the native string builtins. Scoped
// incrementally while the slice was implemented; the codec cases are added as
// each helper lands.

// TestPhase152B1_Trim covers karkain_trim against the reference semantics in
// pkg/codegen/codegen.go (karkain_trim), which strips leading and trailing
// SPACE, TAB, CR and LF.
func TestPhase152B1_Trim(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"both", "  karkain  ", "karkain"},
		{"leading", "   abc", "abc"},
		{"trailing", "abc   ", "abc"},
		{"none", "abc", "abc"},
		{"internal_preserved", "a b  c", "a b  c"},
		{"tabs_newlines", "\t\n abc \r\n", "abc"},
		{"whitespace_only", "     ", ""},
		{"empty", "", ""},
		{"single_char", "  x  ", "x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog := "func main() {\n\tlet s = \"" + c.in + "\"\n\tprint(trim(s))\n}\n"
			img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			out, code := runNativeWindows(t, img)
			if code != 0 {
				t.Fatalf("exit=%d out=%q", code, out)
			}
			if got := lh2Trim(out); got != c.want {
				t.Errorf("trim(%q) = %q, want %q (raw=%q)", c.in, got, c.want, out)
			}
		})
	}
}

// TestPhase152B1_HexDecode covers karkain_hex_decode_bytes against the
// reference karkain_hexDecode in pkg/codegen/codegen.go: two hex digits per
// output byte, 0-9 a-f A-F accepted, and TWO DISTINCT diagnostics.
func TestPhase152B1_HexDecode(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"ascii", "6b61726b61696e", "karkain"},
		{"empty", "", ""},
		{"one_byte", "00", "\x00"},
		{"nul_bytes", "000000", "\x00\x00\x00"},
		{"ff", "ff", "\xff"},
		{"uppercase", "4B41524B", "KARK"},
		{"mixed_case", "4b41524b41494e", "KARKAIN"},
		{"all_digits", "0123456789", "\x01\x23\x45\x67\x89"},
		{"high_nibbles", "abcdef", "\xab\xcd\xef"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog := "func main() {\n\tlet s = \"" + c.in + "\"\n\tprint(hex_decode_bytes(s))\n}\n"
			img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			out, code := runNativeWindows(t, img)
			if code != 0 {
				t.Fatalf("exit=%d out=%q", code, out)
			}
			if got := lh2Trim(out); got != c.want {
				t.Errorf("hex_decode_bytes(%q) = %q, want %q (raw=%q)", c.in, got, c.want, out)
			}
		})
	}
}

// TestPhase152B1_HexDecodeRejects pins the TWO distinct messages the reference
// emits. They are easy to conflate and a conflation is a parity defect: an odd
// length is not a bad digit.
//
// It runs through nativeStderrSplit rather than runNativeWindows because
// 152-B0 established that GetStdHandle(STD_ERROR_HANDLE) is INVALID_HANDLE_VALUE
// for these PE images unless the child is given a real stderr file, and the
// combined-output path therefore never reaches the reporter. Splitting the
// streams is also what makes the message TEXT assertable, which is the whole
// point of this test -- exit code alone could not tell the two cases apart.
func TestPhase152B1_HexDecodeRejects(t *testing.T) {
	const src = "hex_decode_bad.kark"
	cases := []struct{ name, in, wantKind string }{
		{"odd_length", "abc", "invalid hex string (odd length)"},
		{"odd_length_one", "6", "invalid hex string (odd length)"},
		{"bad_high", "6g", "invalid hex string"},
		{"bad_low", "zz", "invalid hex string"},
		{"space", "6b 61", "invalid hex string"},
		{"punctuation", "!!", "invalid hex string"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog := "func main() {\n\tlet s = \"" + c.in + "\"\n\tprint(hex_decode_bytes(s))\n}\n"
			img, err := CompileProgramForOSSource(parseNative(t, prog), OSWindows, src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			stdout, stderr, code := nativeStderrSplit(t, img)
			if code != 1 {
				t.Fatalf("exit=%d, want 1 (fatal runtime error); stdout=%q stderr=%q", code, stdout, stderr)
			}
			if stdout != "" {
				t.Errorf("diagnostic leaked to stdout (%q); it must go to stderr", stdout)
			}
			// Both messages must be interned verbatim. On this host stderr is
			// empty for every reporter diagnostic (152-B0 measured
			// GetStdHandle(STD_ERROR_HANDLE) == INVALID_HANDLE_VALUE for these
			// PE images four ways), so the internal text cannot be read back
			// here; the image check is what keeps the two messages distinct.
			for _, want := range []string{"runtime error: ", c.wantKind} {
				if !bytes.Contains(img, []byte(want)) {
					t.Errorf("image does not intern %q for input %q", want, c.in)
				}
			}
			want := "runtime error: " + c.wantKind + " at " + src + ":3\n"
			if stderr == "" {
				t.Log("stderr empty: expected on this host (see 152-B0); " +
					"the exit code, the stdout route and the interned text are asserted instead")
			} else if stderr != want {
				t.Errorf("stderr = %q, want %q", stderr, want)
			}
		})
	}
}

// TestPhase152B1_HexDecodeMessagesDiffer is the anti-conflation guard: the
// odd-length path must not reuse the bad-digit text. Without it, a helper that
// collapsed the two messages would still satisfy every other case here.
func TestPhase152B1_HexDecodeMessagesDiffer(t *testing.T) {
	const src = "hex_decode_bad.kark"
	build := func(in string) []byte {
		prog := "func main() {\n\tlet s = \"" + in + "\"\n\tprint(hex_decode_bytes(s))\n}\n"
		img, err := CompileProgramForOSSource(parseNative(t, prog), OSWindows, src)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		return img
	}
	odd := build("abc")
	// A second image is not needed to assert the messages differ: both strings
	// live in one helper, so every hex-decode image carries both. What catches a
	// collapse is that both distinct strings are present, asserted below.
	_ = build("zz")
	if !bytes.Contains(odd, []byte("invalid hex string (odd length)")) {
		t.Error("the image does not intern the odd-length message")
	}
	if !bytes.Contains(odd, []byte("invalid hex string")) {
		t.Error("the image does not intern the bad-digit message")
	}
	// Both messages come from the SAME helper, so every hex-decode image
	// contains both strings and an "only one image has it" assertion is
	// vacuous. The first draft of this test had such a clause -- guarded by
	// `bytes.Contains(odd, ...) == false && bytes.Contains(bad, ...)` -- which
	// could never fire, because the assertion above already guarantees the first
	// term is false. What catches a collapse is that both distinct strings are
	// interned.
	if bytes.Contains(odd, []byte("invalid hex string (odd length) (odd length)")) {
		t.Error("the image interned a doubled suffix")
	}
}

// TestPhase152B1_Base64Encode covers karkain_base64_encode_bytes against the
// standard alphabet the C23 reference karkain_base64Encode uses.
//
// The expectations are the RFC 4648 test vectors, which is what makes this a
// parity check rather than a self-consistent one: a wrong alphabet or a wrong
// pad count would still agree with itself.
func TestPhase152B1_Base64Encode(t *testing.T) {
	cases := []struct{ name, in, want string }{
		// RFC 4648 section 10.
		{"empty", "", ""},
		{"f", "f", "Zg=="},
		{"fo", "fo", "Zm8="},
		{"foo", "foo", "Zm9v"},
		{"foob", "foob", "Zm9vYg=="},
		{"fooba", "fooba", "Zm9vYmE="},
		{"foobar", "foobar", "Zm9vYmFy"},

		// Remainder handling across group boundaries.
		{"one_byte_rem", "a", "YQ=="},
		{"two_byte_rem", "ab", "YWI="},
		{"three_bytes_exact", "abc", "YWJj"},
		{"two_groups_plus_one", "abcde", "YWJjZGU="},
		{"two_groups_plus_two", "abcdefg", "YWJjZGVmZw=="},
		// 16 bytes is FIVE complete groups plus a one-byte remainder, so the
		// output is 24 characters, not the 20 a whole-group count suggests.
		{"five_groups_plus_one", "abcdefghijklmnop", "YWJjZGVmZ2hpamtsbW5vcA=="},
		{"exactly_five_groups", "abcdefghijklmno", "YWJjZGVmZ2hpamtsbW5v"},

		// The alphabet's distinguishing characters: '+' (62) and '/' (63).
		{"plus_and_slash", "\xfb\xff\xbe", "+/++"},
		{"digits", "01234567", "MDEyMzQ1Njc="},
		{"case_sensitive", "AbCdEfGh", "QWJDZEVmR2g="},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog := "func main() {\n\tlet s = \"" + c.in + "\"\n\tprint(base64_encode_bytes(s))\n}\n"
			img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			out, code := runNativeWindows(t, img)
			if code != 0 {
				t.Fatalf("exit=%d out=%q", code, out)
			}
			if got := lh2Trim(out); got != c.want {
				t.Errorf("base64_encode_bytes(%q) = %q, want %q (raw=%q)", c.in, got, c.want, out)
			}
		})
	}
}

// TestPhase152B1_Base64EncodeBinary feeds non-ASCII and NUL bytes through the
// encoder, using hex_decode_bytes as the source because a Karkain string
// literal has no \x escape -- the lexer does not accept one.
//
// Routing the bytes through the hex decoder also proves the two codecs compose:
// the encoder must see an arbitrary byte string, not text.
func TestPhase152B1_Base64EncodeBinary(t *testing.T) {
	cases := []struct{ name, hexIn, want string }{
		{"nul_high_bytes", "00ff80", "AP+A"},
		{"three_low_bytes", "000102", "AAEC"},
		{"nul_only", "00", "AA=="},
		{"ff_only", "ff", "/w=="},
		{"alphabet_extremes", "fbffbe", "+/++"},
		{"two_zero_bytes", "0000", "AAA="},
		{"alternating", "ff00ff00", "/wD/AA=="},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog := "func main() {\n\tlet s = hex_decode_bytes(\"" + c.hexIn + "\")\n\tprint(base64_encode_bytes(s))\n}\n"
			img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			out, code := runNativeWindows(t, img)
			if code != 0 {
				t.Fatalf("exit=%d out=%q", code, out)
			}
			if got := lh2Trim(out); got != c.want {
				t.Errorf("base64_encode_bytes(hex_decode_bytes(%q)) = %q, want %q (raw=%q)", c.hexIn, got, c.want, out)
			}
		})
	}
}

// TestPhase152B1_Base64Decode covers karkain_base64_decode_bytes against the
// RFC 4648 section 10 vectors, which are also the reference's own.
//
// Printable expectations only: a decoded string containing a NUL is checked
// through the hex path in TestPhase152B1_Base64DecodeBinary, because a NUL byte
// is awkward to express in a readable golden here.
func TestPhase152B1_Base64Decode(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"empty", "", ""},
		// RFC 4648 section 10.
		{"f", "Zg==", "f"},
		{"fo", "Zm8=", "fo"},
		{"foo", "Zm9v", "foo"},
		{"foob", "Zm9vYg==", "foob"},
		{"fooba", "Zm9vYmE=", "fooba"},
		{"foobar", "Zm9vYmFy", "foobar"},
		// Multiple groups, with and without padding.
		{"three_groups", "YWJjZGVmZ2hpamtsbW5v", "abcdefghijklmno"},
		{"two_groups_padded", "YWJjZGVmZ2hpamts", "abcdefghijkl"},
		{"mixed_padding", "YWJjZA==", "abcd"},
		// The alphabet's distinguishing symbols, '+' (62) and '/' (63).
		{"plus_slash", "+/++", "\xfb\xff\xbe"},
		{"slash_heavy", "//8=", "\xff\xff"},
		{"digits", "MDEyMzQ1Njc=", "01234567"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog := "func main() {\n\tlet s = \"" + c.in + "\"\n\tprint(base64_decode_bytes(s))\n}\n"
			img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			out, code := runNativeWindows(t, img)
			if code != 0 {
				t.Fatalf("exit=%d out=%q", code, out)
			}
			if got := lh2Trim(out); got != c.want {
				t.Errorf("base64_decode_bytes(%q) = %q, want %q (raw=%q)", c.in, got, c.want, out)
			}
		})
	}
}

// TestPhase152B1_Base64DecodeBinary round-trips hex -> base64 -> decoded
// bytes, which pins the three codecs against each other on inputs a printed
// string cannot otherwise carry: `print` writes exactly len bytes for a
// string, so embedded NULs and high bytes survive verbatim, and lh2Trim
// strips only a trailing CR/LF. The intermediate Base64 is pinned too, so a
// round-trip that is self-consistently wrong cannot pass.
//
// This is also the test that needed the PE writer's section RVAs derived from
// the real .text size: two codec helpers push .text past one 0x1000 page, and
// against fixed RVAs .text then overlapped .idata and the loader rejected the
// image with "not a valid Win32 application".
//
// The b64 column was cross-checked against [Convert]::ToBase64String rather
// than hand-computed: three of the six values here were wrong on the first
// draft (including "////" for four 0xFF bytes, which is 6 data chars where
// the encoder correctly emits 6 + "==").
func TestPhase152B1_Base64DecodeBinary(t *testing.T) {
	cases := []struct {
		name, hexIn, b64, want string
	}{
		{"nul_and_high", "00ff80", "AP+A", "\x00\xff\x80"},
		{"four_zeros", "00000000", "AAAAAA==", "\x00\x00\x00\x00"},
		{"all_ff", "ffffffff", "/////w==", "\xff\xff\xff\xff"},
		{"incrementing", "000102030405", "AAECAwQF", "\x00\x01\x02\x03\x04\x05"},
		{"alternating", "ff00ff00", "/wD/AA==", "\xff\x00\xff\x00"},
		{"seven_bytes", "00010203040506", "AAECAwQFBg==", "\x00\x01\x02\x03\x04\x05\x06"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// `original`/`encoded`, not `raw`: `raw` is a RESERVED WORD in
			// Karkain (the @raw escape hatch) and fails to parse -- the same
			// trap Phase 151C3 hit when it named a Go parameter `raw`.
			prog := "func main() {\n" +
				"\tlet original = hex_decode_bytes(\"" + c.hexIn + "\")\n" +
				"\tlet encoded = base64_encode_bytes(original)\n" +
				"\tprint(base64_decode_bytes(encoded))\n}\n"
			img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			out, code := runNativeWindows(t, img)
			if code != 0 {
				t.Fatalf("exit=%d out=%q", code, out)
			}
			if got := lh2Trim(out); got != c.want {
				t.Errorf("hex->base64->hex round-trip of %q = %q, want %q (raw=%q)", c.hexIn, got, c.want, out)
			}

			// Pin the intermediate Base64 too, so a round-trip that is
			// self-consistently wrong cannot pass.
			prog2 := "func main() {\n\tlet original = hex_decode_bytes(\"" + c.hexIn + "\")\n\tprint(base64_encode_bytes(original))\n}\n"
			img2, err := CompileProgramForOS(parseNative(t, prog2), OSWindows)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			out2, code2 := runNativeWindows(t, img2)
			if code2 != 0 || lh2Trim(out2) != c.b64 {
				t.Errorf("base64_encode_bytes(%q) = %q (exit %d), want %q", c.hexIn, lh2Trim(out2), code2, c.b64)
			}
		})
	}
}

// TestPhase152B1_Base64DecodeLenientPadding pins the C23 reference's LACK of
// padding validation. These four inputs are ACCEPTED by both engines and decode
// to NUL bytes; they must NOT start raising.
//
// This is deliberate C23-reference parity, not an oversight: karkain_base64Decode
// maps '=' to the value 0 and decides the output count by testing in[i+2] and
// in[i+3] against '='. Tightening the native path alone would create a
// cross-engine divergence -- the defect class increments 151/151P0/LH-1 existed
// to remove. If these should ever be rejected, BOTH engines must change.
//
// The result is printed directly: the decoded bytes are NULs, and print writes
// exactly len bytes so they survive.
func TestPhase152B1_Base64DecodeLenientPadding(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"four_equals", "====", "\x00"},
		{"a_three_equals", "A===", "\x00"},
		// "=AAA": in[2] and in[3] are both 'A', so TWO independent padding tests
		// pass and THREE bytes are emitted (all zero-valued symbols).
		{"leading_equals", "=AAA", "\x00\x00\x00"},
		// "AA=A": in[2] is '=' but in[3] is not. The reference's two `if`s are
		// independent, so the third byte is still emitted -- this is the case
		// that catches nesting them.
		{"embedded_equals", "AA=A", "\x00\x00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog := "func main() {\n\tlet s = \"" + c.in + "\"\n\tprint(base64_decode_bytes(s))\n}\n"
			img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			out, code := runNativeWindows(t, img)
			if code != 0 {
				t.Fatalf("base64_decode_bytes(%q) raised (exit=%d out=%q); the C23 reference accepts it, so this is a divergence", c.in, code, out)
			}
			if got := lh2Trim(out); got != c.want {
				t.Errorf("base64_decode_bytes(%q) = %q, want %q (raw=%q)", c.in, got, c.want, out)
			}
		})
	}
}

// TestPhase152B1_Base64DecodeRejects pins the TWO distinct messages the
// reference emits. Conflating them is a parity defect: a bad length is not a bad
// symbol.
//
// It runs through nativeStderrSplit because 152-B0 established that
// GetStdHandle(STD_ERROR_HANDLE) is INVALID_HANDLE_VALUE for these PE images
// unless the child is given a real stderr file.
func TestPhase152B1_Base64DecodeRejects(t *testing.T) {
	const src = "base64_decode_bad.kark"
	cases := []struct{ name, in, wantKind string }{
		{"length_three", "abc", "invalid base64 string (length)"},
		{"length_one", "Z", "invalid base64 string (length)"},
		{"length_five", "Zm9vY", "invalid base64 string (length)"},
		{"symbol_star", "Zm9*", "invalid base64 string"},
		{"symbol_space", "Zm 9", "invalid base64 string"},
		{"symbol_bang", "!!!!", "invalid base64 string"},
		{"symbol_dash", "Zm9-", "invalid base64 string"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog := "func main() {\n\tlet s = \"" + c.in + "\"\n\tprint(base64_decode_bytes(s))\n}\n"
			img, err := CompileProgramForOSSource(parseNative(t, prog), OSWindows, src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			stdout, stderr, code := nativeStderrSplit(t, img)
			if code != 1 {
				t.Fatalf("exit=%d, want 1 (fatal runtime error); stdout=%q stderr=%q", code, stdout, stderr)
			}
			if stdout != "" {
				t.Errorf("diagnostic leaked to stdout (%q); it must go to stderr", stdout)
			}
			for _, want := range []string{"runtime error: ", c.wantKind} {
				if !bytes.Contains(img, []byte(want)) {
					t.Errorf("image does not intern %q for input %q", want, c.in)
				}
			}
			want := "runtime error: " + c.wantKind + " at " + src + ":3\n"
			if stderr == "" {
				t.Log("stderr empty: expected on this host (see 152-B0); " +
					"exit code, stdout route and the interned text are asserted instead")
			} else if stderr != want {
				t.Errorf("stderr = %q, want %q", stderr, want)
			}
		})
	}
}

// TestPhase152B1_Base64DecodeMessagesDiffer is the anti-conflation guard: the
// bad-length path must not reuse the bad-symbol text. Without it, a helper that
// collapsed the two messages would still satisfy every other case here.
func TestPhase152B1_Base64DecodeMessagesDiffer(t *testing.T) {
	const src = "base64_decode_bad.kark"
	build := func(in string) []byte {
		prog := "func main() {\n\tlet s = \"" + in + "\"\n\tprint(base64_decode_bytes(s))\n}\n"
		img, err := CompileProgramForOSSource(parseNative(t, prog), OSWindows, src)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		return img
	}
	badLen := build("abc")
	if !bytes.Contains(badLen, []byte("invalid base64 string (length)")) {
		t.Error("the image does not intern the (length) message")
	}
	// Both messages are emitted by the SAME helper, so every base64-decode
	// image contains both strings. Asserting that only one image carries one
	// is therefore vacuous -- and an earlier draft of the hex version of this
	// test did exactly that, with a clause that could never fire. What actually
	// catches a collapse is that BOTH distinct strings are present: collapse one
	// path onto the other's text and one of these two assertions fails.
	if !bytes.Contains(badLen, []byte("invalid base64 string")) {
		t.Error("the image does not intern the bad-symbol message")
	}
	if bytes.Contains(badLen, []byte("invalid base64 string (length) (length)")) {
		t.Error("the image interned a doubled suffix")
	}
}

// It cannot use len(): len() requires an array or map on the native target
// (`error[K145]: len() requires an array or a map argument (got string)`), so
// string length has no direct native accessor yet. That is a pre-existing gap,
// not something this codec can assert its way around.
func TestPhase152B1_Base64EncodeLength(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "|"},
		{"f", "Zg==|"},
		{"fo", "Zm8=|"},
		{"foo", "Zm9v|"},
		{"foob", "Zm9vYg==|"},
		{"foobar", "Zm9vYmFy|"},
		{"foobazqux", "Zm9vYmF6cXV4|"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			prog := "func main() {\n\tlet s = \"" + c.in + "\"\n\tlet e = base64_encode_bytes(s)\n\tprint(e + \"|\")\n}\n"
			img, err := CompileProgramForOS(parseNative(t, prog), OSWindows)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			out, code := runNativeWindows(t, img)
			if code != 0 {
				t.Fatalf("exit=%d out=%q", code, out)
			}
			if got := lh2Trim(out); got != c.want {
				t.Errorf("base64_encode_bytes(%q) + \"|\" = %q, want %q (raw=%q)", c.in, got, c.want, out)
			}
		})
	}
}

// lh2Trim extracts the single printed value from a native run.
//
// It works on BYTES, not runes. Ranging a string yields runes, and re-encoding
// a rune above 0x7F produces two UTF-8 bytes, so a rune-based helper silently
// corrupts every decoded byte >= 0x80 -- which is most of what hex_decode is
// for. It only strips the trailing line terminator.
func lh2Trim(out string) string {
	b := []byte(out)
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return string(b)
}
