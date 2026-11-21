package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// v1.2.1 -- cross-engine gates for std.path and std.json.
//
// These two modules are present and documented as importable on develop, but
// the read-only library assessment found no located gate/executed evidence.
// This file supplies that evidence through the REAL CLI on both engines.
//
// The comparison is raw stdout first: Go and kcc must agree byte for byte.
// A second, normalised comparison checks the semantic golden, because the C
// runtime writes CRLF on Windows while the fixture's expected text uses LF.
// Normalisation is never used to hide a difference BETWEEN engines.

type v121Case struct {
	name string
	src  string
	want string
}

var v121Cases = []v121Case{
	{
		name: "path",
		src: `import std.path

func main() {
    print(path_join("a/b", "c.txt"))
    print(path_basename("/a/b/c.txt"))
    print(path_dirname("/a/b/c.txt"))
    print(path_extension("/a/b/c.txt"))
    print(path_stem("/a/b/c.txt"))
    print(path_is_absolute("/a/b"))
    print(path_is_absolute("a/b"))
    print(path_normalize("a\\b\\c"))
    print(path_has_extension("/a/b/c.txt", "txt"))
    print(path_replace_extension("/a/b/c.txt", ".md"))
}
`,
		want: "a/b/c.txt\nc.txt\n/a/b\n.txt\nc\ntrue\nfalse\na/b/c\n1\n/a/b/c.md\n",
	},
	{
		name: "json",
		src: `import std.json

func main() {
    string o = "{\"name\": \"karkain\", \"n\": 42, \"ok\": true}"
    print(json_get_field(o, "name"))
    print(json_get_int(o, "n"))
    print(json_get_bool(o, "ok"))
    print(json_has_field(o, "name"))
    print(json_stringify("hi"))
    print(json_array_length("[1, 2, 3]"))
}
`,
		want: "karkain\n42\n1\n1\n\"hi\"\n3\n",
	},
}

// v121EnsurePATH makes gcc resolvable on the Windows development host. CI has
// gcc on PATH already; this is the same MinGW location used by the native gates.
func v121EnsurePATH(t *testing.T) {
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

func v121Normalize(b []byte) string {
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func v121Run(t *testing.T, bin, src, engine, name string) []byte {
	t.Helper()
	root := repoRoot(t)
	dir := filepath.Join(root, ".karkain-dev", "v121-"+name+"-"+engine)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	file := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	cmd := exec.Command(bin, "run", file, "--engine", engine)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s engine run failed: %v\nstdout: %s\nstderr: %s",
			engine, err, stdout.String(), stderr.String())
	}
	return stdout.Bytes()
}

// TestPhaseV121_StdlibPathAndJsonBothEngines is the gate.
func TestPhaseV121_StdlibPathAndJsonBothEngines(t *testing.T) {
	v121EnsurePATH(t)
	hasGCC(t)
	kccBin := phase95KCC(t)
	selectKCCEngine(t, kccBin)
	bin := phase130Karkain(t)

	for _, c := range v121Cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			goOut := v121Run(t, bin, c.src, "go", c.name)
			kccOut := v121Run(t, bin, c.src, "kcc", c.name)

			// Engine parity is checked on the RAW bytes. Trimming or
			// normalising here would hide exactly the class of defect the
			// kcc-run stdout parity corrective change was written to catch.
			if !bytes.Equal(goOut, kccOut) {
				t.Fatalf("Go and kcc stdout differ for %s:\n  go : %q (% x)\n  kcc: %q (% x)",
					c.name, goOut, goOut, kccOut, kccOut)
			}
			if got := v121Normalize(goOut); got != c.want {
				t.Fatalf("%s golden mismatch:\n got %q\nwant %q", c.name, got, c.want)
			}
		})
	}
}

// TestPhaseV121_CorpusIsNonVacuous protects the gate itself.
func TestPhaseV121_CorpusIsNonVacuous(t *testing.T) {
	if len(v121Cases) < 2 {
		t.Fatalf("only %d stdlib cases; a two-module gate must carry both", len(v121Cases))
	}
	for _, c := range v121Cases {
		if strings.TrimSpace(c.want) == "" {
			t.Errorf("case %s has an empty golden", c.name)
		}
		if !strings.Contains(c.src, "import std.") {
			t.Errorf("case %s does not import a stdlib module", c.name)
		}
	}
}
