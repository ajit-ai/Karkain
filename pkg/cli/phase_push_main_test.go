package cli

import (
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// natPushCases is the execution corpus for push() in main.
var natPushCases = []struct {
	name    string
	src     string
	wantOut string
	wantExit int
}{
	{
		name:     "push_single",
		src:      "func main() { let a = [1, 2, 3] let b = push(a, 4) print(b[0]) print(b[1]) print(b[2]) print(b[3]) print(len(b)) }",
		wantOut:  "1\n2\n3\n4\n4\n",
		wantExit: 0,
	},
	{
		name:     "push_empty",
		src:      "func main() { let a = [] let b = push(a, 7) print(len(b)) print(b[0]) }",
		wantOut:  "1\n7\n",
		wantExit: 0,
	},
	{
		name:     "push_source_unchanged",
		src:      "func main() { let a = [10, 20] let b = push(a, 30) print(a[0]) print(a[1]) print(len(a)) print(b[0]) print(b[2]) print(len(b)) }",
		wantOut:  "10\n20\n2\n10\n30\n3\n",
		wantExit: 0,
	},
	{
		name:     "push_chain",
		src:      "func main() { let a = [1] let b = push(a, 2) let c = push(b, 3) print(len(c)) print(c[0]) print(c[1]) print(c[2]) }",
		wantOut:  "3\n1\n2\n3\n",
		wantExit: 0,
	},
	{
		name:     "push_negative",
		src:      "func main() { let a = [5] let b = push(a, -7) print(b[0]) print(b[1]) }",
		wantOut:  "5\n-7\n",
		wantExit: 0,
	},
	{
		name:     "push_computed_value",
		src:      "func main() { let a = [1, 2] let b = push(a, 3 + 4) print(b[2]) }",
		wantOut:  "7\n",
		wantExit: 0,
	},
}

// TestPushMain_ProgramsExecuteWithExactBytes is layer 1: push programs execute.
func TestPushMain_ProgramsExecuteWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	for _, c := range natPushCases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "prog.kark")
			if err := os.WriteFile(file, []byte(c.src), 0o644); err != nil {
				t.Fatalf("writing program: %v", err)
			}
			karkain := phase130Karkain(t)
			out, err := exec.Command(karkain, "native-ast", file).CombinedOutput()
			s := strings.TrimSpace(string(out))
			if err != nil && s == "" {
				t.Fatalf("kcc produced no output: %v", err)
			}
			if strings.Contains(s, "error[K") {
				t.Fatalf("kcc refused a program this gate expects to compile:\n%.400s", s)
			}
			img, err := hex.DecodeString(s)
			if err != nil {
				t.Fatalf("output is not hex: %v\n%.200s", err, s)
			}
			if _, _, err := native.ParsePE(img); err != nil {
				t.Fatalf("image is not a valid PE: %v", err)
			}
			exe := filepath.Join(dir, "prog.exe")
			if err := os.WriteFile(exe, img, 0o755); err != nil {
				t.Fatalf("writing image: %v", err)
			}
			got, err := exec.Command(exe).CombinedOutput()
			code := 0
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else if err != nil {
				t.Fatalf("running image: %v", err)
			}
			if string(got) != c.wantOut {
				t.Errorf("stdout = %q, want %q\n  source: %s", got, c.wantOut, c.src)
			}
			if code != c.wantExit {
				t.Errorf("exit = %d, want %d", code, c.wantExit)
			}
		})
	}
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
			if err == nil {
				// Might still have refused — check output
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

// TestPushMain_NoGoFallback verifies kcc provenance.
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
	if strings.Contains(s, "error[K") {
		t.Fatalf("kcc refused a program this gate expects to compile:\n%.400s", s)
	}
	img, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil {
		t.Fatalf("output is not hex: %v", err)
	}
	if _, _, err := native.ParsePE(img); err != nil {
		t.Fatalf("kcc emitted an invalid PE: %v", err)
	}
}