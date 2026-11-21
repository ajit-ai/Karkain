package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestPhaseC1StdPathCrossEngine verifies the std.path module produces
// byte-identical output on Go and self-hosted KCC. The module itself uses only
// core substr/len builtins — no OS calls, C interop or Go dependency.
func TestPhaseC1StdPathCrossEngine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cross-engine std.path gate runs on Linux CI")
	}
	root := repoRoot(t)
	bin := filepath.Join(root, "karkain.exe")
	src := `import std.path

func main() {
    print(path_base("/a/b/c.txt"))
    print(path_dir("/a/b/c.txt"))
    print(path_ext("/a/b/c.txt"))
    print(path_stem("/a/b/c.txt"))
    print(path_join("a/b", "c.txt"))
    print(path_is_abs("/a/b"))
    print(path_normalize("a\\b\\c"))
}
`
	want := "c.txt\n/a/b\n.txt\nc\na/b/c.txt\n1\na/b/c\n"
	for _, eng := range []string{"go", "kcc"} {
		t.Run(eng, func(t *testing.T) {
			work := t.TempDir()
			f := filepath.Join(work, "main.kark")
			if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			// Place the file under the repo so stdlib resolution walks up to it.
			work = filepath.Join(root, ".karkain-dev", "c1-stdpath-"+eng)
			if err := os.MkdirAll(work, 0o755); err != nil {
				t.Fatal(err)
			}
			f = filepath.Join(work, "main.kark")
			if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bin, "run", f, "--engine", eng)
			cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+eng)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s run: %v\n%s", eng, err, string(out))
			}
			got := stripKCCBuildBanner(t, string(out))
			got = strings.ReplaceAll(got, "\r\n", "\n")
			if got != want {
				t.Fatalf("%s output mismatch:\n got %q\nwant %q", eng, got, want)
			}
		})
	}
}
