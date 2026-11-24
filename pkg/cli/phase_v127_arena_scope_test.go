package cli

import (
	"strings"
	"testing"
)

// v1.2.7 -- the ARENA-SCOPE invariant that lets the allocation-site counters
// walk only a function's TOP-LEVEL statement list.
//
// natNativeConcatSites, natNativePushCount and the four codec site counters
// (hex decode, base64 encode/decode, sha256) do not descend into `if`/`while`/
// `for` bodies. That is sound ONLY because every construct they count is
// REFUSED inside a nested non-loop body -- the int-only body rule -- so no
// reachable site is missed. If a later slice ever lowers one of these nested,
// the counter must descend in the same change or the compile-time arena is
// sized too small, which is a memory-safety defect rather than a diagnostic.
//
// This gate pins the reason, not the style: each construct is placed inside an
// `if` body at loopDepth 0 (the one nesting the driver lowers statements in)
// and must be refused BY NAME with no image written. Two of them are also
// built at the top level and must be ACCEPTED, so the assertion is measured as
// a difference in scope rather than assumed.
//
// Measured before it was written: all six nested shapes are refused today, and
// the string-EQUALITY counter is the one that already descends (natNativeStrEqSites
// had to, because an equality IS accepted nested -- see phase_v127_streq_test.go).

type v127ArenaCase struct {
	name string
	src  string
	want string
}

var v127ArenaRefused = []v127ArenaCase{
	{
		"concat_in_if_body",
		"func main() { let a = \"x\"\nlet b = \"y\"\nif (1 == 1) { print(a + b) } }\n",
		"only an int may appear in an arithmetic expression",
	},
	{
		"push_in_if_body",
		"func main() { let a = [1]\nif (1 == 1) { let b = push(a, 2)\nprint(len(b)) } }\n",
		"call to builtin `push` is not lowered",
	},
	{
		"hex_decode_in_if_body",
		"func main() { let s = \"41\"\nif (1 == 1) { let d = hex_decode_bytes(s)\nprint(d) } }\n",
		"call to builtin `hex_decode_bytes` is not lowered",
	},
	{
		"base64_encode_in_if_body",
		"func main() { let s = \"ab\"\nif (1 == 1) { let e = base64_encode_bytes(s)\nprint(e) } }\n",
		"call to builtin `base64_encode_bytes` is not lowered",
	},
	{
		"base64_decode_in_if_body",
		"func main() { let s = \"YWI=\"\nif (1 == 1) { let d = base64_decode_bytes(s)\nprint(d) } }\n",
		"call to builtin `base64_decode_bytes` is not lowered",
	},
	{
		"sha256_in_if_body",
		"func main() { let s = \"abc\"\nif (1 == 1) { let h = sha256_hex(s)\nprint(h) } }\n",
		"call to builtin `sha256_hex` is not lowered",
	},
	{
		"slice_in_if_body",
		"func main() { let s = \"hello\"\nif (1 == 1) { print(s[1:3]) } }\n",
		"not an integer expression",
	},
}

// The same constructs at MAIN's top level are lowered, which is what makes the
// non-descending counters sufficient rather than merely lucky.
var v127ArenaAccepted = []v127Case{
	{
		"concat_at_top_level",
		"func main() { let a = \"x\"\nlet b = \"y\"\nprint(a + b) }\n",
		"xy\n",
	},
	{
		"slice_at_top_level",
		"func main() { let s = \"hello\"\nprint(s[1:3]) }\n",
		"el\n"},
}

func TestPhaseV127ArenaScope_NestedAllocatorsAreRefusedByName(t *testing.T) {
	karkain := v126Setup(t)
	for _, c := range v127ArenaRefused {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v127arenascope.exe")
			if res.ExitCode == 0 {
				t.Fatalf("expected a refusal, got success (image written: %v)\nsource:\n%s",
					res.WroteImage, c.src)
			}
			if res.WroteImage {
				t.Fatalf("a refused program still wrote an image\nsource:\n%s", c.src)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Fatalf("refusal does not name %q:\n%s", c.want, res.Output)
			}
		})
	}
}

func TestPhaseV127ArenaScope_TopLevelAllocatorsStayLowered(t *testing.T) {
	karkain := v126Setup(t)
	for _, c := range v127ArenaAccepted {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v127arenatop.exe")
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
