package sema

import (
	"strings"
	"testing"
)

// Phase 109 gate: the eight byte-level runtime builtins behind the stdlib v2
// encoding/crypto/collections modules (hex/base64/utf8/sha256/sha512/
// map_keys_of) must be known to the resolver so std.* module wrappers compile
// and — critically — user diagnostics never flag them as undefined. Direct
// calls must resolve clean on both literal and variable operands.

var phase109Builtins = []string{
	"hex_encode_bytes",
	"hex_decode_bytes",
	"base64_encode_bytes",
	"base64_decode_bytes",
	"sha256_hex",
	"sha512_hex",
	"map_keys_of",
	"utf8_valid_bytes",
}

func TestPhase109_BuiltinCallsResolve(t *testing.T) {
	for _, name := range phase109Builtins {
		t.Run(name, func(t *testing.T) {
			src := "func main() {\n  " + name + "(\"x\")\n}\n"
			prog := parseTestProg(t, src)
			r := NewResolver(prog, nil)
			errs := r.Resolve()
			if len(errs) > 0 {
				t.Fatalf("builtin '%s' flagged as error: %s", name, errLines(errs))
			}
		})
	}
}

func TestPhase109_BuiltinCallsResolveWithVariables(t *testing.T) {
	// The stdlib v2 module wrappers pass variables (not string literals) into
	// the builtins; the resolver must treat the new names as known builtins in
	// that shape too, never as undefined identifiers.
	for _, name := range phase109Builtins {
		t.Run(name, func(t *testing.T) {
			src := "func main() {\n  let buf: string = \"x\"\n  " + name + "(buf)\n}\n"
			prog := parseTestProg(t, src)
			r := NewResolver(prog, nil)
			errs := r.Resolve()
			for _, e := range errs {
				if strings.Contains(e.Msg, "not defined") || strings.Contains(e.Msg, "undefined") {
					t.Errorf("builtin '%s' flagged as undefined: %s", name, errLines(errs))
					return
				}
			}
		})
	}
}