// Phase v1.2.9 - no-C `stdlib_v2` build-and-run (Condition 2).
//
// Pins what the native path can consume from an UNMODIFIED .kark
// module today, measured on this host, and what it cannot: every
// unsupported shape is refused BY NAME.
//
// What WORKS, end to end, on the Go native backend, through the real CLI:
//
//   - `import std.string` / `std.crypto` / `std.encoding` through the normal
//     toolchain (pure modules whose helpers take typed string/int params).
//   - str_len / str_concat / str_trim / str_starts_with / str_empty
//   - sha256 / base64_encode / base64_decode / hex_encode / hex_decode
//
// What does NOT work (documented boundaries, not failures):
//
//   - split / str_split (string arrays) - refused with K145
//   - str_to_upper / str_repeat (concat inside a loop) - refused with K145
//   - hasKey / map_keys_of - refused with K145
//   - std.collections array_* helpers take untyped params; the Go native
//     backend does not yet infer array params from call sites (the kcc
//     branch added this inference; the Go backend has not). Collections is
//     therefore a documented boundary for v1.2.9, not a failure.
//   - kcc native parity for stdlib is not claimed (kcc string params are
//     a separate, parked slice - NAT-STRPARAM).
package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func writeV129Probe(t *testing.T, name, src string) string {
	t.Helper()
	probe := filepath.Join(t.TempDir(), name+".kark")
	if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
		t.Fatalf("writing probe: %v", err)
	}
	return probe
}

var v129KarkainOnce sync.Once
var v129KarkainBin string
var v129KarkainErr error

func v129Karkain(t *testing.T) string {
	t.Helper()
	v129KarkainOnce.Do(func() {
		v129KarkainErr = buildKarkainErr(t, &v129KarkainBin)
	})
	if v129KarkainErr != nil {
		t.Fatalf("building karkain: %v", v129KarkainErr)
	}
	return v129KarkainBin
}

// TestPhaseV129_NativeStdlibExample builds and runs the shipped
// examples/stdlib_v2_native/main.kark through the C-free native path and
// checks exact stdout bytes.
func TestPhaseV129_NativeStdlibExample(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PE execution requires windows/amd64")
	}
	karkain := v129Karkain(t)
	example := filepath.Join(repoRoot(t), "examples", "stdlib_v2_native", "main.kark")
	out := filepath.Join(t.TempDir(), "native129.exe")
	cmd := exec.Command(karkain, "build", example, "--engine", "go", "--target", "native-x86_64-windows", "-o", out)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	if msg, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build should succeed:\n%s", string(msg))
	}
	run := exec.Command(out)
	got, err := run.Output()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, string(got))
	}
	want := "5\nabcd\n00e0cba20c10cac449eb885a9926a4b646f0ac163ed7fbc5704d9d8a057ef44d\ncm91bmR0cmlw\n"
	if string(got) != want {
		t.Errorf("golden mismatch:\nwant: %q\ngot:  %q", want, string(got))
	}
}

// TestPhaseV129_NativeStdlibRefusals pins the unsupported shapes by name.
func TestPhaseV129_NativeStdlibRefusals(t *testing.T) {
	karkain := v129Karkain(t)
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			"split",
			"func main() {\n\tstring[] p = split(\"a-b\", \"-\")\n\tprint(len(p))\n}\n",
			"split() is not yet supported on the native target",
		},
		{
			"concat_in_loop",
			"func main() {\n\tstring r = \"\"\n\tint i = 0\n\twhile (i < 3) {\n\t\tr = r + \"x\"\n\t\ti = i + 1\n\t}\n\tprint(r)\n}\n",
			"string concatenation inside a loop",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			probe := writeV129Probe(t, c.name, c.src)
			out := filepath.Join(t.TempDir(), c.name+".exe")
			cmd := exec.Command(karkain, "build", probe, "--engine", "go", "--target", "native-x86_64-windows", "-o", out)
			cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
			msg, _ := cmd.CombinedOutput()
			if !strings.Contains(string(msg), c.want) {
				t.Errorf("expected refusal %q, got:\n%s", c.want, string(msg))
			}
			if _, err := os.Stat(out); err == nil {
				t.Errorf("refusal should not write an image")
			}
		})
	}
}

// TestPhaseV129_NativeStdlibCollections builds an int-array stdlib program.
