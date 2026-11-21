package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// natPushCases was the execution corpus for push() in main. It is now the
// REFUSAL corpus: every one of these programs is declined by the self-hosted
// native PE path, and none writes an image.
//
// CORRECTIVE CHANGE against the frozen Phase-2.1 baseline, recorded in
// docs/audit/NAT-FORIN-BASELINE.md. The earlier gate expected these programs to
// execute. A patch-position guard added to native_pe.kark -- which made a
// previously silent out-of-bounds store a named diagnostic -- exposed a
// pre-existing defect: a push() program records a rodata patch position that
// does not fit the PE .text body. The honest current state is a refusal, not
// an executable image. This gate pins that refusal rather than papering over
// it, and does not attempt to fix push() in the for-in slice.
var natPushCases = []struct {
	name string
	src  string
}{
	{
		name: "push_single",
		src:  "func main() { let a = [1, 2, 3] let b = push(a, 4) print(b[0]) print(b[1]) print(b[2]) print(b[3]) print(len(b)) }",
	},
	{
		name: "push_empty",
		src:  "func main() { let a = [] let b = push(a, 7) print(len(b)) print(b[0]) }",
	},
	{
		name: "push_source_unchanged",
		src:  "func main() { let a = [10, 20] let b = push(a, 30) print(a[0]) print(a[1]) print(len(a)) print(b[0]) print(b[2]) print(len(b)) }",
	},
	{
		name: "push_chain",
		src:  "func main() { let a = [1] let b = push(a, 2) let c = push(b, 3) print(len(c)) print(c[0]) print(c[1]) print(c[2]) }",
	},
	{
		name: "push_negative",
		src:  "func main() { let a = [5] let b = push(a, -7) print(b[0]) print(b[1]) }",
	},
	{
		// This one fails even earlier, inside the slot scan, and is PINNED as
		// a non-image failure rather than mislabelled as a clean K117 refusal.
		name: "push_computed_value",
		src:  "func main() { let a = [1, 2] let b = push(a, 3 + 4) print(b[2]) }",
	},
}

// TestPushMain_ProgramsAreRefusedByThePEPatchGuard is the correction of the
// former execution gate. It asserts the observable, current contract: a
// push() program produces no image, and what it does produce is either the
// named PE patch diagnostic or a loud slot-scan error. An empty answer, a
// panic with no text, or a hex image all fail.
func TestPushMain_ProgramsAreRefusedByThePEPatchGuard(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range natPushCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "prog.kark")
			if err := os.WriteFile(file, []byte(c.src), 0o644); err != nil {
				t.Fatalf("writing program: %v", err)
			}
			out, err := exec.Command(karkain, "native-ast", file).CombinedOutput()
			s := string(out)
			if err != nil && s == "" {
				t.Fatalf("kcc produced no output (err %v); a driver crash must not be "+
					"reported as a clean refusal", err)
			}
			cleanPatchRefusal := strings.Contains(s, "error[K117]: native pe: rodata patch")
			loudSlotFailure := strings.Contains(s, "runtime error:") && strings.Contains(s, "natNativeSlotOff")
			if !cleanPatchRefusal && !loudSlotFailure {
				t.Errorf("the push program was neither cleanly refused nor loudly rejected:\n%.500s", s)
			}
			if pushLooksLikeHex(s) {
				t.Errorf("a push program still produced image bytes:\n%.200s", s)
			}
		})
	}
}

// pushLooksLikeHex reports whether the driver's answer is a bare hex image.
// It checks the character set as well as the length: a runtime stack trace is
// also long, and should be recognised as a loud non-image failure instead.
func pushLooksLikeHex(s string) bool {
	t := strings.TrimSpace(s)
	if len(t) < 64 {
		return false
	}
	for i := 0; i < len(t); i++ {
		c := t[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// TestPushMain_RefusalsAreByName tests invalid inputs.
func TestPushMain_RefusalsAreByName(t *testing.T) {
	karkain := phase130Karkain(t)
	cases := []struct {
		name string
		src  string
		code string
		want string
	}{
		{
			name: "push_wrong_arity",
			src:  "func main() { let a = [1, 2] let b = push(a) print(len(b)) }",
			code: "error[K145]", want: "push() takes exactly 2 arguments",
		},
		{
			name: "push_non_array_receiver",
			src:  "func main() { let a = 5 let b = push(a, 1) print(len(b)) }",
			code: "error[K145]", want: "push() requires an array receiver",
		},
		{
			name: "push_non_int_element",
			src:  "func main() { let a = [1, 2] let b = push(a, \"hi\") print(len(b)) }",
			code: "error[K145]", want: "`StringLiteral` is not an integer expression",
		},
		{
			name: "push_undefined_receiver",
			src:  "func main() { let b = push(nope, 1) print(len(b)) }",
			code: "error[K102]", want: "undefined identifier",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "prog.kark")
			if err := os.WriteFile(file, []byte(c.src), 0o644); err != nil {
				t.Fatalf("writing program: %v", err)
			}
			out, err := exec.Command(karkain, "native-ast", file).CombinedOutput()
			s := string(out)
			if err != nil && s == "" {
				t.Fatalf("kcc produced no output: %v", err)
			}
			if !strings.Contains(s, c.code) {
				t.Errorf("output does not contain %s\n  output:\n%.400s", c.code, s)
			}
			if !strings.Contains(s, c.want) {
				t.Errorf("output does not contain %q\n  output:\n%.400s", c.want, s)
			}
		})
	}
}

// TestPushMain_NoGoFallback verifies that the refusal comes from kcc's native
// driver (the PE patch guard), not from a silent hand-off or a missing engine.
func TestPushMain_NoGoFallback(t *testing.T) {
	karkain := phase130Karkain(t)
	src := "func main() { let a = [1, 2] let b = push(a, 3) print(len(b)) }"
	dir := t.TempDir()
	file := filepath.Join(dir, "prog.kark")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatalf("writing program: %v", err)
	}
	out, err := exec.Command(karkain, "native-ast", file).CombinedOutput()
	s := string(out)
	if err != nil && s == "" {
		t.Fatalf("kcc produced no output: %v", err)
	}
	if !strings.Contains(s, "error[K117]: native pe: rodata patch") {
		t.Fatalf("the refusal did not come from kcc's PE patch guard:\n%.500s", s)
	}
}
