package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestPhaseC1StdJsonCrossEngine verifies the std.json module produces
// byte-identical output on Go and self-hosted KCC. The module uses only core
// builtins (substr/len/contains/trim) — no imports, OS calls or C interop.
func TestPhaseC1StdJsonCrossEngine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cross-engine std.json gate runs on Linux CI")
	}
	root := repoRoot(t)
	bin := filepath.Join(root, "karkain.exe")
	src := `import std.json

func main() {
    print(json_string("hello"))
    print(json_quote("a\"b"))
    print(json_bool(true))
    print(json_int("42"))
    string o = "{\"name\": \"karkain\", \"n\": 42}"
    print(json_field(o, "name"))
    print(json_field(o, "n"))
}
`
	want := "\"hello\"\na\\\"b\ntrue\n42\n\"karkain\"\n42\n"
	for _, eng := range []string{"go", "kcc"} {
		t.Run(eng, func(t *testing.T) {
			work := filepath.Join(root, ".karkain-dev", "c1-stdjson-"+eng)
			if err := os.MkdirAll(work, 0o755); err != nil {
				t.Fatal(err)
			}
			f := filepath.Join(work, "main.kark")
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
