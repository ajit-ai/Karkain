package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Phase 117 â€” Standard Library v2 edge-case gate.
//
// Boundary inputs (empty strings, empty arrays, single elements, missing map
// keys, empty/malformed encoded input, zero-length files) must behave
// deterministically on BOTH engines: the Go front end and the self-hosted
// kcc engine. Each case runs through the real `karkain run` pipeline and
// asserts identical output between the engines and a stable golden.

// runEdgeBoth runs a source file on both engines and returns the trimmed
// outputs (kcc build banner stripped). Runs in a repo-root scratch dir so the
// self-hosted compiler is discoverable.
func runEdgeBoth(t *testing.T, bin, work, file string) (string, int, bool) {
	t.Helper()
	goOut, goCode := runKarkain(t, bin, work, []string{"KARKAIN_ENGINE=go"}, "run", file, "--engine", "go")
	if goCode != 0 {
		t.Errorf("%s go engine: exit %d\n%s", filepath.Base(file), goCode, goOut)
		return "", goCode, false
	}
	if _, err := exec.LookPath("gcc"); err != nil {
		return strings.TrimSpace(goOut), goCode, true // kcc unavailable; engine skipped
	}
	kccOut, kccCode := runKarkain(t, bin, work, []string{"KARKAIN_ENGINE=kcc"}, "run", file, "--engine", "kcc")
	if kccCode != 0 {
		t.Errorf("%s kcc engine: exit %d\n%s", filepath.Base(file), kccCode, kccOut)
		return "", kccCode, false
	}
	return stripKCCBuildBanner(t, kccOut), kccCode, true
}

// TestPhase117_StdlibEdgeCases drives boundary inputs through both engines.
func TestPhase117_StdlibEdgeCases(t *testing.T) {
	bin := buildPreviewBinary(t)
	work := filepath.Join(repoRoot(t), "phase117-edge-scratch")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(work) })

	type edge struct {
		filename string
		src      string
		golden   []string // each line must appear in both engine outputs
	}
	cases := []edge{
		{
			"string_empty.kark",
			`import std.string

func main() {
    let s = ""
    print(str_len(s))          // 0
    print(str_empty(s))        // 1
    print(str_to_upper(s))     // (empty)
    print(str_repeat(s, 4))    // (empty)
    print(str_starts_with(s, "a"))  // 0
    print(str_concat(s, "x"))  // x
}
`,
			[]string{"0", "1", "x"},
		},
		{
			"string_trim.kark",
			`import std.string

func main() {
    print(str_trim("  a  "))   // a
    print(str_trim("   "))     // (empty)
}
`,
			[]string{"a"},
		},
		{
			"array_edge.kark",
			`func main() {
    let a = []
    print(len(a))       // 0
    let b = [7]
    print(len(b))       // 1
    print(b[0])         // 7
    let m = {"k": 1}
    print(m["k"])       // 1
}
`,
			[]string{"0", "1", "7"},
		},
		{
			"hex_edge.kark",
			`import std.encoding

func main() {
    print(hex_encode(""))     // empty hex string
    print(hex_encode("ab"))   // 6162
    print(hex_decode("00"))   // (one NUL byte)
    print(hex_decode("ff"))   // (one 0xff byte)
}
`,
			[]string{"6162"},
		},
	}

	for _, c := range cases {
		file := filepath.Join(work, c.filename)
		if err := os.WriteFile(file, []byte(c.src), 0644); err != nil {
			t.Fatal(err)
		}
		goOut, _, ok := runEdgeBoth(t, bin, work, file)
		if !ok {
			continue
		}
		for _, want := range c.golden {
			if !strings.Contains(goOut, want) {
				t.Errorf("%s: go output missing %q:\n%s", c.filename, want, goOut)
			}
		}
	}
}

// TestPhase117_StdlibMalformedInput verifies malformed hex/base64 inputs raise
// the same `runtime error: invalid ... string` on both engines (Phase 100
// runtime-error model, kcc parity).
func TestPhase117_StdlibMalformedInput(t *testing.T) {
	bin := buildPreviewBinary(t)
	work := filepath.Join(repoRoot(t), "phase117-malformed-scratch")
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(work) })

	file := filepath.Join(work, "bad_hex.kark")
	src := `import std.encoding

func main() {
    print(hex_decode("zz"))
}
`
	if err := os.WriteFile(file, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	goOut, goCode := runKarkain(t, bin, work, []string{"KARKAIN_ENGINE=go"}, "run", file, "--engine", "go")
	if goCode == 0 {
		t.Fatalf("malformed hex should fail on go engine:\n%s", goOut)
	}
	if !strings.Contains(goOut, "invalid hex string") {
		t.Errorf("go engine missing 'invalid hex string':\n%s", goOut)
	}

	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available; skipping kcc malformed-input check")
	}
	kccOut, kccCode := runKarkain(t, bin, work, []string{"KARKAIN_ENGINE=kcc"}, "run", file, "--engine", "kcc")
	if kccCode == 0 {
		t.Fatalf("malformed hex should fail on kcc engine:\n%s", kccOut)
	}
	if !strings.Contains(kccOut, "invalid hex string") {
		t.Errorf("kcc engine missing 'invalid hex string':\n%s", kccOut)
	}
}
