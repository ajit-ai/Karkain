// v1.2.35 slice 3: profile-selected SSA passes. The baseline is
// docs/audit/V1.2.35-SSA-PASSES-BASELINE.md.
package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"karkain/pkg/ir/ssa"
)

const v1235cProgram = `func main() {
    let a = 20
    let b = 22
    print(a + b)
}
`

func TestPhaseV1235C_PassSetsAreExplicit(t *testing.T) {
	dev := ssa.NewPipelineForProfile("dev").PassNames()
	if len(dev) != 0 {
		t.Errorf("dev must add no profile passes (codegen keeps its historical sequence), got %v", dev)
	}
	release := ssa.NewPipelineForProfile("release").PassNames()
	wantRelease := []string{"fold-const", "cse", "dce", "licm", "sroa", "simplify-cfg"}
	if !reflect.DeepEqual(release, wantRelease) {
		t.Errorf("release passes = %v, want %v", release, wantRelease)
	}
	if count(release, "mem2reg") != 0 {
		t.Error("mem2reg is unsound per Phase 141 and must never be selected")
	}
	size := ssa.NewPipelineForProfile("size").PassNames()
	if !reflect.DeepEqual(size, []string{"fold-const", "dce", "cse", "licm", "sroa", "dce", "simplify-cfg"}) {
		t.Errorf("size passes = %v", size)
	}
	if count(size, "dce") != 2 {
		t.Errorf("size must run DCE twice, got %d", count(size, "dce"))
	}
	if len(size) <= len(release) {
		t.Errorf("size profile must select more passes than release: %d vs %d", len(size), len(release))
	}
}

func count(items []string, want string) int {
	n := 0
	for _, s := range items {
		if s == want {
			n++
		}
	}
	return n
}

// A same-machine triple is required: a hard-coded Windows triple fails on
// the Linux CI leg (no stdio.h for cross C compilation, and cross-run is
// correctly refused by the CLI).
func v1235cHostTriple() string {
	switch runtime.GOOS {
	case "windows":
		return "x86_64-windows"
	case "darwin":
		return "x86_64-macos"
	default:
		return "x86_64-linux"
	}
}

func v1235cBuildC(t *testing.T, profile string) []byte {
	t.Helper()
	cli := v1235Prepare(t)
	dir, err := os.MkdirTemp(repoRoot(t), "v1235c")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	src := filepath.Join(dir, "p.kark")
	if err := os.WriteFile(src, []byte(v1235cProgram), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(cli, "build", src, "--engine", "go", "--target", v1235cHostTriple(), "--profile", profile, "-c")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s failed: %v\n%s", profile, err, string(out))
	}
	data, err := os.ReadFile(filepath.Join(dir, "p.c"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPhaseV1235C_GeneratedCIsDeterministicPerProfile(t *testing.T) {
	for _, profile := range []string{"dev", "release", "size"} {
		t.Run(profile, func(t *testing.T) {
			a := v1235cBuildC(t, profile)
			b := v1235cBuildC(t, profile)
			if !bytes.Equal(a, b) {
				t.Errorf("%s generated C is not byte-deterministic (%d vs %d bytes)", profile, len(a), len(b))
			}
			t.Logf("profile %s: %d C bytes", profile, len(a))
		})
	}
}

func TestPhaseV1235C_ProfilesKeepProgramMeaning(t *testing.T) {
	cli := v1235Prepare(t)
	dir, err := os.MkdirTemp(repoRoot(t), "v1235c")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	src := filepath.Join(dir, "p.kark")
	if err := os.WriteFile(src, []byte(v1235cProgram), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"dev", "release", "size"} {
		t.Run(profile, func(t *testing.T) {
			cmd := exec.Command(cli, "run", src, "--engine", "go", "--target", v1235cHostTriple(), "--profile", profile)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("run %s failed: %v\n%s", profile, err, string(out))
			}
			if got := strings.ReplaceAll(string(out), "\r\n", "\n"); got != "42\n" {
				t.Errorf("%s stdout = %q, want %q", profile, got, "42\n")
			}
		})
	}
}
