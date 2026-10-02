package native

import (
	"strings"
	"testing"
)

// Increment 152-A — native safety boundary and capability boundary.
//
// 152-A is deliberately small: it closes one latent SOUNDNESS defect and draws
// one CAPABILITY line. It implements no new stdlib function.
//
//  1. concatInLoop — the arena bound is `sites * totalLiteralBytes`, whose
//     stated proof requires each concat site to execute at most once. A concat
//     reached from a loop body can execute once per iteration, so the bound is
//     not an upper bound and the bump allocator exhausts mid-loop.
//
//  2. net_* — the native backend has no socket syscalls (only write and exit),
//     so the socket builtins are a deliberate capability boundary and are
//     rejected by name rather than reported as an undefined function.

// TestNativeStrConcatInLoopRefused pins the refusal for a concat inside every
// loop form. The detection mirrors scanPushSites: While/For/ForIn bodies are
// depth+1, an `if` body is NOT (a conditional runs at most once per entry,
// which the site count already bounds).
func TestNativeStrConcatInLoopRefused(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"while", "func main() {\n    let i = 0\n    while (i < 3) {\n        print(\"it\" + \"er\")\n        i = i + 1\n    }\n}\n"},
		{"for", "func main() {\n    for (let i = 0; i < 3; i = i + 1) {\n        print(\"it\" + \"er\")\n    }\n}\n"},
		{"for_in", "func main() {\n    let xs = [1, 2]\n    for x in xs {\n        print(\"it\" + \"er\")\n    }\n}\n"},
		{"nested", "func main() {\n    let i = 0\n    while (i < 2) {\n        if (i > 0) {\n            print(\"it\" + \"er\")\n        }\n        i = i + 1\n    }\n}\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := CompileProgramForOS(parseNative(t, c.src), OSWindows)
			if err == nil {
				t.Fatalf("concat inside a %s loop compiled; the arena bound is unsound there and it must be refused", c.name)
			}
			msg := err.Error()
			if !strings.Contains(msg, "concatenation inside a loop") {
				t.Errorf("refusal does not name the construct: %q", msg)
			}
			// The diagnostic must say WHY, so a user can tell this from an
			// unrelated native limitation.
			if !strings.Contains(msg, "arena") {
				t.Errorf("refusal does not state the reason (arena bounding): %q", msg)
			}
		})
	}
}

// TestNativeStrConcatOutsideLoopStillWorks is the anti-vacuity guard for the
// refusal above: ordinary concatenation, including inside an `if` and inside a
// helper called from a loop, must be UNAFFECTED. Without this the refusal
// could be over-broad and still pass every rejection case.
func TestNativeStrConcatOutsideLoopStillWorks(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"plain", "func main() {\n    print(\"ab\" + \"cd\")\n}\n", "abcd\n"},
		// An `if` body is deliberately NOT a loop: this must keep working.
		{"in_if", "func main() {\n    let i = 0\n    if (i == 0) {\n        print(\"a\" + \"0\")\n    } else {\n        print(\"b\" + \"1\")\n    }\n}\n", "a0\n"},
		// A concat in a CALLEE, called from a loop: the concat site itself is
		// not in a loop body, so the bound holds (one call, one allocation).
		{"callee_from_loop", "func j() {\n    return \"it\" + \"er\"\n}\nfunc main() {\n    let i = 0\n    while (i < 2) {\n        print(j())\n        i = i + 1\n    }\n}\n", "iter\niter\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			img, err := CompileProgramForOS(parseNative(t, c.src), OSWindows)
			if err != nil {
				t.Fatalf("ordinary concat was refused (%s): %v", c.name, err)
			}
			out, code := runNativeWindows(t, img)
			requireNativeOut(t, "pe "+c.name, out, c.want, code, 0)
		})
	}
}

// TestNativeSocketBuiltinsRefused pins the 152-A capability boundary. The
// diagnostic must name the primitive AND the reason, so a caller can tell
// "this target has no sockets" from "this program is invalid" — which the
// generic "undefined function" message could not.
func TestNativeSocketBuiltinsRefused(t *testing.T) {
	for _, fn := range []string{
		"net_connect", "net_listen", "net_accept",
		"net_read", "net_write", "net_close", "net_last_error",
	} {
		t.Run(fn, func(t *testing.T) {
			src := "func main() {\n    let a = \"x\"\n    print(" + fn + "(a))\n}\n"
			_, err := CompileProgramForOS(parseNative(t, src), OSWindows)
			if err == nil {
				t.Fatalf("%s compiled on the native target; there is no socket syscall layer to lower it onto", fn)
			}
			msg := err.Error()
			if !strings.Contains(msg, fn) {
				t.Errorf("refusal does not name %s: %q", fn, msg)
			}
			if !strings.Contains(msg, "socket") {
				t.Errorf("refusal does not state the capability reason: %q", msg)
			}
			// It must NOT degrade into the generic undefined-function message.
			if strings.Contains(msg, "undefined function") {
				t.Errorf("refusal degraded to 'undefined function', which cannot be distinguished from an invalid program: %q", msg)
			}
		})
	}
}

// TestNativeOrdinaryUndefinedFunctionStillGeneric is the boundary of the
// boundary: an unknown name must STILL report the generic undefined-function
// diagnostic. If the capability refusal swallowed every unknown call, the
// socket message would prove nothing.
func TestNativeOrdinaryUndefinedFunctionStillGeneric(t *testing.T) {
	src := "func main() {\n    print(definitely_not_a_builtin(1))\n}\n"
	_, err := CompileProgramForOS(parseNative(t, src), OSWindows)
	if err == nil {
		t.Fatal("an unknown function compiled")
	}
	msg := err.Error()
	if !strings.Contains(msg, "undefined function") {
		t.Errorf("unknown function lost its generic diagnostic: %q", msg)
	}
	if strings.Contains(msg, "socket") {
		t.Errorf("an unknown function was misreported as a socket capability limit: %q", msg)
	}
}

// TestNativeNonSocketBuiltinsUnaffected proves the capability table is scoped:
// it must name ONLY the seven socket builtins, so an unrelated name is not
// swept into the boundary.
func TestNativeNonSocketBuiltinsUnaffected(t *testing.T) {
	if len(nativeSocketBuiltins) != 7 {
		t.Errorf("nativeSocketBuiltins has %d entries, want exactly the 7 socket builtins", len(nativeSocketBuiltins))
	}
	for _, name := range []string{"trim", "split", "sha256_hex", "len", "push", "print", "base64_encode_bytes"} {
		if nativeSocketBuiltins[name] {
			t.Errorf("%s is wrongly inside the socket capability boundary", name)
		}
	}
	for name := range nativeSocketBuiltins {
		if err := nativeCapabilityRefusal(name); err == nil {
			t.Errorf("nativeCapabilityRefusal(%s) = nil, want a refusal", name)
		}
	}
	if err := nativeCapabilityRefusal("definitely_not_a_builtin"); err != nil {
		t.Errorf("nativeCapabilityRefusal on an unknown name = %v, want nil", err)
	}
}
