// v1.2.35 slice 2: native as the default target for programs the self-hosted
// compiler says it can lower. The base line and design are in
// docs/audit/V1.2.35-NATIVE-DEFAULT-BASELINE.md.
//
// An explicit --target is never rewritten. A program the native driver refuses
// keeps the historical C23 path, and the choice is made by kcc's non-emitting
// native-check probe rather than by a heuristic in the Go CLI.
package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const v1235bHello = `func main() {
    print("hello default")
}
`

const v1235bUnsupported = `import std.env
func main() {
    print(env_get("KARKAIN_V1235B_UNSET"))
}
`

func v1235bSource(t *testing.T, name, body string) string {
	t.Helper()
	dir, err := os.MkdirTemp(repoRoot(t), "v1235b")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, name+".kark")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func v1235bRun(t *testing.T, cliBin, kccBin, shimDir, log, src string, args ...string) (string, string, int) {
	t.Helper()
	if err := os.WriteFile(log, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, cliBin, append([]string{"run", src, "--engine", "kcc"}, args...)...)
	cmd.Env = append(os.Environ(),
		"PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"KARKAIN_ENGINE=kcc", "KARKAIN_KCC="+kccBin)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("timed out: %v", args)
	}
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("%v: %v", args, err)
		}
	}
	return out.String(), errOut.String(), code
}

func v1235bGccCalls(t *testing.T, log string) []string {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var calls []string
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) != "" {
			calls = append(calls, l)
		}
	}
	return calls
}

func TestPhaseV1235B_NativeDefaultForSupportedProgram(t *testing.T) {
	cliBin, kccBin := a1Prepare(t)
	shimDir, log := a1Shims(t)
	src := v1235bSource(t, "hello", v1235bHello)

	stdout, stderr, code := v1235bRun(t, cliBin, kccBin, shimDir, log, src)
	if code != 0 {
		t.Fatalf("default run exit = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if got := strings.ReplaceAll(stdout, "\r\n", "\n"); got != "hello default\n" {
		t.Errorf("stdout = %q, want %q", got, "hello default\n")
	}
	if calls := v1235bGccCalls(t, log); len(calls) != 0 {
		t.Errorf("native default invoked gcc: %v", calls)
	}
}

func TestPhaseV1235B_C23DefaultForUnsupportedProgram(t *testing.T) {
	cliBin, kccBin := a1Prepare(t)
	shimDir, log := a1Shims(t)
	src := v1235bSource(t, "unsupported", v1235bUnsupported)

	stdout, stderr, code := v1235bRun(t, cliBin, kccBin, shimDir, log, src)
	if code != 0 {
		t.Fatalf("default run exit = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if got := strings.ReplaceAll(stdout, "\r\n", "\n"); got != "\n" {
		t.Errorf("stdout = %q, want a single empty line", got)
	}
	if calls := v1235bGccCalls(t, log); len(calls) == 0 {
		t.Error("C23 fallback did not invoke gcc; native probe may have lied")
	}
}

func TestPhaseV1235B_ExplicitTargetsAreUnchanged(t *testing.T) {
	cliBin, kccBin := a1Prepare(t)
	shimDir, log := a1Shims(t)
	src := v1235bSource(t, "explicit", v1235bHello)

	_, _, c23Code := v1235bRun(t, cliBin, kccBin, shimDir, log, src, "--target", "c23")
	if c23Code != 0 {
		t.Fatalf("explicit c23 exit = %d", c23Code)
	}
	if calls := v1235bGccCalls(t, log); len(calls) == 0 {
		t.Error("explicit c23 did not invoke gcc")
	}
	_, _, nativeCode := v1235bRun(t, cliBin, kccBin, shimDir, log, src, "--target", NativeHostTarget())
	if nativeCode != 0 {
		t.Fatalf("explicit native exit = %d", nativeCode)
	}
	if calls := v1235bGccCalls(t, log); len(calls) != 0 {
		t.Errorf("explicit native invoked gcc: %v", calls)
	}
}

func TestPhaseV1235B_ProbeWritesNoImage(t *testing.T) {
	_, kccBin := a1Prepare(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "probe.kark")
	if err := os.WriteFile(src, []byte(v1235bHello), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(kccBin, "native-check", src, "--target", NativeHostTarget())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native-check failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "[ok] native supported") {
		t.Errorf("probe output = %q", string(out))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".exe") || strings.HasSuffix(e.Name(), ".c23") {
			t.Errorf("probe wrote an artifact: %s", e.Name())
		}
	}
}
