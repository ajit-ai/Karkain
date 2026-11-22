package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// v1.2.3 -- native string byte indexing.
//
// The current driver refuses `s[i]` when s is a string. This gate executes the
// Windows PE on this host with exact stdout bytes and compares the same source
// with the C backend. The Go NATIVE backend has no string-index lowering, so
// there is no ELF byte oracle for this construct; claiming one would be false.
// It intentionally avoids string equality, slicing, trim, split, codecs and
// SHA: those are later v1.2.x slices.
//
// The semantics are byte-wise and the result is a one-byte string view. The
// C backend is the language oracle for stdout.
type v123Case struct {
	name string
	src  string
	want string
}

var v123Cases = []v123Case{
	{
		name: "literal_zero",
		src:  "func main() { let s = \"abc\" print(s[0]) }\n",
		want: "a\n",
	},
	{
		name: "literal_middle",
		src:  "func main() { let s = \"abc\" print(s[1]) }\n",
		want: "b\n",
	},
	{
		name: "literal_last",
		src:  "func main() { let s = \"abc\" print(s[2]) }\n",
		want: "c\n",
	},
	{
		name: "variable_index",
		src:  "func main() { let s = \"abc\" let i = 1 print(s[i]) }\n",
		want: "b\n",
	},
	{
		name: "expression_index",
		src:  "func main() { let s = \"abc\" let i = 1 print(s[i + 1]) }\n",
		want: "c\n",
	},
}

func v123EnsurePATH(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}
	if _, err := exec.LookPath("gcc"); err == nil {
		return
	}
	for _, dir := range []string{`C:\msys64\ucrt64\bin`, `C:\msys64\mingw64\bin`} {
		if _, err := os.Stat(filepath.Join(dir, "gcc.exe")); err == nil {
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			return
		}
	}
}

func v123Build(t *testing.T, karkain, src, target, engine, outName string) nativeBuildResult {
	t.Helper()
	return nativeBuildWithEngine(t, karkain, src, target, engine, outName)
}

func v123RunImage(t *testing.T, img []byte) ([]byte, int) {
	t.Helper()
	if _, _, err := native.ParsePE(img); err != nil {
		t.Fatalf("not a valid PE: %v", err)
	}
	return nat9dRun(t, img)
}

func TestPhaseV123_ExecutedPEWithExactBytes(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	v123EnsurePATH(t)
	hasGCC(t)
	kccBin := phase95KCC(t)
	selectKCCEngine(t, kccBin)
	karkain := phase130Karkain(t)

	for _, c := range v123Cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v123.exe")
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
				t.Fatalf("stdout = %q, want %q\n  source:\n%s", got, c.want, c.src)
			}
		})
	}
}

func TestPhaseV123_MatchesCBackend(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("C backend comparison needs the host toolchain")
	}
	v123EnsurePATH(t)
	hasGCC(t)
	kccBin := phase95KCC(t)
	selectKCCEngine(t, kccBin)
	karkain := phase130Karkain(t)

	for _, c := range v123Cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v123-cmp.exe")
			if res.ExitCode != 0 || !res.WroteImage {
				t.Fatalf("kcc produced no image:\n%s", res.Output)
			}
			nativeOut, nativeCode := v123RunImage(t, res.Image)
			cOut, cCode := natIRunCBackend(t, karkain, c.src)
			cOut = natINormalizeCRLF(cOut)
			if nativeCode != cCode || !bytes.Equal(nativeOut, cOut) {
				t.Fatalf("native/C mismatch:\n  native: % x exit %d\n  c    : % x exit %d",
					nativeOut, nativeCode, cOut, cCode)
			}
		})
	}
}

func TestPhaseV123_BoundsAreRuntimeErrors(t *testing.T) {
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	v123EnsurePATH(t)
	hasGCC(t)
	kccBin := phase95KCC(t)
	selectKCCEngine(t, kccBin)
	karkain := phase130Karkain(t)

	cases := []struct {
		name       string
		src        string
		wantBefore string
	}{
		{"index_equal_to_length", "func main() { let s = \"abc\" print(s[3]) }\n", ""},
		{"index_above_length", "func main() { let s = \"abc\" print(s[9]) }\n", ""},
		{"negative_index", "func main() { let s = \"abc\" let i = 0 i = i - 1 print(s[i]) }\n", ""},
		{"empty_string", "func main() { let s = \"\" print(s[0]) }\n", ""},
		{
			"out_of_range_in_loop",
			"func main() { let s = \"abc\" for (let i = 0; i < 5; i = i + 1) { print(s[i]) } }\n",
			"a\nb\nc\n",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v123-bounds.exe")
			if res.ExitCode != 0 || !res.WroteImage {
				t.Fatalf("kcc produced no image:\n%s", res.Output)
			}
			got, code := nat9dRun(t, res.Image)
			if code == 0 {
				t.Fatalf("an out-of-range access exited 0; stdout = % x", got)
			}
			if string(got) != c.wantBefore {
				t.Fatalf("stdout before failure = %q, want %q", got, c.wantBefore)
			}
			if !strings.Contains(string(res.Image), "string index out of range") {
				t.Errorf("the image does not carry the string-index diagnostic text")
			}
		})
	}
}

func TestPhaseV123_RefusalsAreByName(t *testing.T) {
	v123EnsurePATH(t)
	hasGCC(t)
	kccBin := phase95KCC(t)
	selectKCCEngine(t, kccBin)
	karkain := phase130Karkain(t)

	cases := []struct {
		name string
		src  string
		want string
	}{
		{"index_of_int", "func main() { let a = 7 print(a[0]) }\n", "index target must be an array"},
		{"float_index", "func main() { let s = \"abc\" print(s[1.5]) }\n", "`FloatLiteral` is not an integer expression"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v123-refuse.exe")
			if res.ExitCode == 0 {
				t.Fatalf("kcc ACCEPTED a program it cannot lower:\n%s", res.Output)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Errorf("refusal does not name %q:\n%s", c.want, res.Output)
			}
			if res.WroteImage {
				t.Errorf("refused program wrote an image (%d bytes)", len(res.Image))
			}
		})
	}
}

func TestPhaseV123_CIWiringIsPresent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	if !strings.Contains(string(data), "TestPhaseV123_") {
		t.Errorf("ci.yml has no TestPhaseV123_ step")
	}
}
