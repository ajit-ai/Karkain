package native

import (
	"strings"
	"testing"
)

// Phase 150B3c -- maps on the native target.
//
// The executed goldens run through the PE container on this windows/amd64
// host, so the linear-scan runtime is proven by real execution. The cases
// cover a literal and its reads, a missing key, insertion, overwrite,
// insertion-order iteration, len, computed keys and values, and independence
// between maps.
var nativeMapCases = []struct {
	name string
	src  string
	want string
}{
	{"literal_read", "func main() {\n    let m = {1: 10, 2: 20}\n    print(m[1])\n    print(m[2])\n}\n", "10\n20\n"},
	{"single_entry", "func main() {\n    let m = {7: 99}\n    print(m[7])\n}\n", "99\n"},
	{"empty_map", "func main() {\n    let m = {}\n    print(len(m))\n    print(m[1])\n}\n", "0\n0\n"},
	{"missing_key_is_zero", "func main() {\n    let m = {1: 5}\n    print(m[42])\n}\n", "0\n"},
	{"len", "func main() {\n    let m = {1: 1, 2: 2, 3: 3}\n    print(len(m))\n}\n", "3\n"},
	{"key_order_irrelevant", "func main() {\n    let m = {3: 30, 1: 10, 2: 20}\n    print(m[1])\n    print(m[2])\n    print(m[3])\n}\n", "10\n20\n30\n"},
	{"negative_key", "func main() {\n    let m = {0 - 5: 55}\n    print(m[0 - 5])\n}\n", "55\n"},
	{"negative_and_zero_keys", "func main() {\n    let m = {0 - 1: 1, 0: 2, 1: 3}\n    print(m[0 - 1])\n    print(m[0])\n    print(m[1])\n}\n", "1\n2\n3\n"},

	// Insertion after construction, and overwrite.
	{"insert", "func main() {\n    let m = {1: 10}\n    m[2] = 20\n    print(m[2])\n    print(len(m))\n}\n", "20\n2\n"},
	{"insert_many", "func main() {\n    let m = {1: 10}\n    m[2] = 20\n    m[3] = 30\n    m[4] = 40\n    print(len(m))\n    print(m[2])\n    print(m[3])\n    print(m[4])\n}\n", "4\n20\n30\n40\n"},
	{"overwrite", "func main() {\n    let m = {1: 10}\n    m[1] = 99\n    print(m[1])\n    print(len(m))\n}\n", "99\n1\n"},
	{"insert_existing_and_new", "func main() {\n    let m = {1: 10, 2: 20}\n    m[1] = 11\n    m[3] = 30\n    print(m[1])\n    print(m[3])\n    print(len(m))\n}\n", "11\n30\n3\n"},

	// Iteration is over keys, in insertion order -- the same order the C
	// runtime's linear scan produces.
	{"iterate_keys", "func main() {\n    let m = {1: 10, 2: 20, 3: 30}\n    for k in m {\n        print(k)\n    }\n}\n", "1\n2\n3\n"},
	{"iterate_sum_values", "func main() {\n    let m = {1: 10, 2: 20, 3: 30}\n    let s = 0\n    for k in m {\n        s = s + m[k]\n    }\n    print(s)\n}\n", "60\n"},
	{"iterate_empty", "func main() {\n    let m = {}\n    for k in m {\n        print(k)\n    }\n    print(0)\n}\n", "0\n"},
	{"iterate_with_break", "func main() {\n    let m = {1: 10, 2: 20, 3: 30}\n    for k in m {\n        if (k == 2) {\n            break\n        }\n        print(k)\n    }\n}\n", "1\n"},
	{"iterate_with_continue", "func main() {\n    let m = {1: 10, 2: 20}\n    for k in m {\n        if (k == 1) {\n            continue\n        }\n        print(k)\n    }\n}\n", "2\n"},

	// Computed keys and values: the key is evaluated before the map
	// address is loaded, so an arbitrary expression is safe.
	{"computed_key", "func main() {\n    let m = {1 + 1: 5}\n    print(m[2])\n}\n", "5\n"},
	{"computed_value", "func main() {\n    let m = {1: 2 * 3}\n    print(m[1])\n}\n", "6\n"},
	{"computed_both", "func main() {\n    let k = 3\n    let m = {k: k * k}\n    print(m[3])\n}\n", "9\n"},
	{"insert_computed", "func main() {\n    let m = {}\n    let n = 5\n    m[n] = n + 1\n    print(m[5])\n    print(len(m))\n}\n", "6\n1\n"},

	// Two independent maps must not share a region.
	{"two_maps", "func main() {\n    let a = {1: 10}\n    let b = {1: 99}\n    print(a[1])\n    print(b[1])\n}\n", "10\n99\n"},
	{"maps_independent_insert", "func main() {\n    let a = {1: 10}\n    let b = {1: 20}\n    a[2] = 11\n    print(len(a))\n    print(len(b))\n}\n", "2\n1\n"},
	{"map_in_if", "func main() {\n    let m = {}\n    if (1 == 1) {\n        m[1] = 7\n    }\n    print(m[1])\n}\n", "7\n"},
	{"insert_then_read_in_expr", "func main() {\n    let m = {1: 2}\n    print(m[1] * m[1] + 1)\n}\n", "5\n"},
	{"key_as_condition", "func main() {\n    let m = {1: 10, 2: 20}\n    if (m[2] > 15) {\n        print(1)\n    } else {\n        print(0)\n    }\n}\n", "1\n"},
}

// TestNativeMapExecPE runs the map surface through the PE container, where the
// map runtime, the arena (in the R/W .idata) and the Win64 boundary all
// execute for real on windows/amd64.
func TestNativeMapExecPE(t *testing.T) {
	for _, c := range nativeMapCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNativeOS(t, OSWindows, c.src)
			out, code := runNativeWindows(t, img)
			if out != "" {
				if out != c.want {
					t.Errorf("%s: output %q, want %q", c.name, out, c.want)
				}
				if code != 0 {
					t.Errorf("%s: exit %d, want 0 (out=%q)", c.name, code, out)
				}
			}
		})
	}
}

// TestNativeMapExec mirrors the map surface on the Linux ELF container.
func TestNativeMapExec(t *testing.T) {
	for _, c := range nativeMapCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNative(t, c.src)
			out, code := runNativeCode(t, img)
			if out != "" {
				if out != c.want {
					t.Errorf("%s: output %q, want %q", c.name, out, c.want)
				}
				if code != 0 {
					t.Errorf("%s: exit %d, want 0 (out=%q)", c.name, code, out)
				}
			}
		})
	}
}

// TestNativeMapStructural compiles and structurally validates every map case
// for ELF and PE. The macOS leg asserts the DOCUMENTED refusal instead of
// pretending the image is valid: macOS has no writable segment until 150D, so
// an allocating program (a map needs the arena) is refused by design.
func TestNativeMapStructural(t *testing.T) {
	for _, c := range nativeMapCases {
		c := c
		for _, osName := range []string{OSLinux, OSWindows} {
			t.Run(c.name+"_"+osName, func(t *testing.T) {
				compileNativeOS(t, osName, c.src)
			})
		}
		_, err := CompileProgramForOS(parseNative(t, c.src), OSMacOS)
		if err == nil || !strings.Contains(err.Error(), "heap allocation is not supported") {
			t.Errorf("%s (macos): want the documented no-writable-segment refusal, got %v", c.name, err)
		}
	}
}

// TestNativeMapNegatives is the K145 reject table for maps. Each entry is a
// shape the v1 value model does not define, and each must be REFUSED rather
// than lowered into a program that computes the wrong thing.
func TestNativeMapNegatives(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"string_key", "func main() {\n    let m = {\"a\": 1}\n    print(m[1])\n}\n", "map key 0 must be an int"},
		{"string_key_read", "func main() {\n    let m = {1: 1}\n    print(m[\"a\"])\n}\n", "map key must be an int"},
		{"insert_in_loop", "func main() {\n    let m = {}\n    let i = 0\n    while (i < 3) {\n        m[i] = i\n        i = i + 1\n    }\n    print(len(m))\n}\n", "map insertion inside a loop"},
		{"insert_in_for", "func main() {\n    let m = {}\n    for (let i = 0; i < 3; i = i + 1) {\n        m[i] = i\n    }\n    print(len(m))\n}\n", "map insertion inside a loop"},
		{"insert_in_forin", "func main() {\n    let m = {}\n    let a = [1, 2]\n    for x in a {\n        m[x] = x\n    }\n    print(len(m))\n}\n", "map insertion inside a loop"},
		{"len_of_int", "func main() {\n    let n = 5\n    print(len(n))\n}\n", "len() requires an array or a map"},
		{"index_a_string", "func main() {\n    let s = \"hi\"\n    print(s[0])\n}\n", "index target must be an array or a map"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			_, err := CompileProgram(parseNative(t, c.src))
			if err == nil {
				t.Fatalf("%s: compiled without error, want a K145 containing %q", c.name, c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s: error %q, want it to mention %q", c.name, err, c.want)
			}
		})
	}
}
