package cli

import (
	"os"
	"strings"
	"testing"
)

// v1.2.4b -- corrective gate for the four latent trim defects found during
// the v1.2.5 attempt. Each case targets one path that was structurally broken
// in the committed v1.2.4 code:
//   1. print(trim(...))   -- unreachable scan case (after an unconditional return)
//   2. let t = trim(...)  -- scan case nested inside the push branch
//   3. return trim(...)   -- malformed return-position validator block
//   4. let a = [..]       -- dropped array-literal branch header in decl lowering
//
// The gate executes kcc-produced PEs with exact stdout bytes and requires kcc
// provenance, so a silent Go fallback cannot satisfy it.
type v124Case struct {
	name string
	src  string
	want string
}

var v124Cases = []v124Case{
	{"print_trim", "func main() { print(trim(\"  hi  \")) }\n", "hi\n"},
	{"decl_binding", "func main() { let t = trim(\"  hi  \") print(t) }\n", "hi\n"},
	{"array_binding", "func main() { let a = [1, 2] print(a[0]) }\n", "1\n"},
}

func v124Setup(t *testing.T) string {
	t.Helper()
	if os.PathSeparator == '/' {
		t.Skip("PE images only execute on Windows")
	}
	v123EnsurePATH(t)
	hasGCC(t)
	kccBin := phase95KCC(t)
	selectKCCEngine(t, kccBin)
	return phase130Karkain(t)
}

func TestPhaseV124_TrimAndArrayPathsExecute(t *testing.T) {
	karkain := v124Setup(t)
	for _, c := range v124Cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := v123Build(t, karkain, c.src, NativeWindowsTarget, "kcc", "v124.exe")
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

func TestPhaseV124_ReturnOfTrimIsRefused(t *testing.T) {
	karkain := v124Setup(t)
	src := "func main() { return trim(\"x\") }\n"
	res := v123Build(t, karkain, src, NativeWindowsTarget, "kcc", "v124-ret.exe")
	if res.ExitCode == 0 {
		t.Fatal("a string return was accepted by the native driver")
	}
	if res.WroteImage {
		t.Fatal("a refused program still produced an image")
	}
	if !strings.Contains(res.Output, "return` of a string") && !strings.Contains(res.Output, "must be an int") {
		t.Fatalf("refusal did not name the construct:\n%s", res.Output)
	}
}
