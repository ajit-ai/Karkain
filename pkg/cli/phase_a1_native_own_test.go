// A1 (unnamed v1.2.x slice): kcc owns native build/run.
//
// The baseline state is recorded in docs/audit/A1-NATIVE-BUILD-RUN-BASELINE.md:
// kcc emits every byte of a native image but prints it as hex, and `kcc run`
// always goes through C23 + gcc. The Go CLI performed the remaining work.
// This gate drives kcc DIRECTLY and uses PATH shims so "no gcc/go was called"
// is positive evidence rather than an empty log from a shim that never fired.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"karkain/pkg/native"
)

var a1Once sync.Once
var a1CLI, a1KCC string
var a1Err error

func a1Prepare(t *testing.T) (string, string) {
	t.Helper()
	for _, dir := range []string{
		`C:\Program Files\Git\cmd`,
		`C:\Windows\System32\WindowsPowerShell\v1.0`,
		`C:\msys64\mingw64\bin`,
	} {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		if !a1PathHas(dir) {
			_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
	}
	a1Once.Do(func() {
		var cli string
		if a1Err = buildKarkainErr(t, &cli); a1Err != nil {
			return
		}
		// Force a fresh kcc from the WORKING TREE. phase95KCC cannot be used
		// here: buildBootstrap reuses <root>/karkain.exe if it exists, and
		// this slice changes Go sources (builtin registration and codegen),
		// so a stale root binary would report writeBytes as undefined.
		built := a1BuildKCC(t, cli, repoRoot(t))
		dir, err := os.MkdirTemp("", "karkain-a1-kcc")
		if err != nil {
			a1Err = err
			return
		}
		data, err := os.ReadFile(built)
		if err != nil {
			a1Err = err
			return
		}
		dst := filepath.Join(dir, "kcc")
		if runtime.GOOS == "windows" {
			dst += ".exe"
		}
		if err := os.WriteFile(dst, data, 0o755); err != nil {
			a1Err = err
			return
		}
		a1CLI, a1KCC, a1Err = cli, dst, nil
	})
	if a1Err != nil {
		t.Fatalf("preparing A1 gate: %v", a1Err)
	}
	return a1CLI, a1KCC
}

// a1BuildKCC mirrors phase95KCC but uses the freshly built CLI, so both the
// Go-side builtin table and the .kark sources come from the working tree.
func a1BuildKCC(t *testing.T, cli, root string) string {
	t.Helper()
	hasGCC(t)
	srcDir := filepath.Join(root, "src", "compiler")
	sandbox := gccTempDir(t)
	comp := filepath.Join(sandbox, "compiler")
	if err := os.MkdirAll(comp, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".kark") {
			data, err := os.ReadFile(filepath.Join(srcDir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(comp, e.Name()), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	mainFile := filepath.Join(comp, "main.kark")
	build := exec.Command(cli, "build", mainFile)
	build.Dir = comp
	build.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("stage-1 build failed: %v\n%s", err, string(out))
	}
	bin := filepath.Join(sandbox, "kcc")
	exe := bin
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	link := exec.Command("gcc", "-std=c99", "-x", "c", "-D_POSIX_C_SOURCE=200809L",
		filepath.Join(comp, "main.c"), "-o", exe, "-lgmp", "-lm", winsockLibFlag())
	link.Dir = comp
	if out, err := link.CombinedOutput(); err != nil {
		t.Fatalf("stage-1 link failed: %v\n%s", err, string(out))
	}
	return exe
}

func a1PathHas(dir string) bool {
	for _, p := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if strings.EqualFold(p, dir) {
			return true
		}
	}
	return false
}

// a1Shims logs every gcc/go invocation. The legacy C23 path is the positive
// control: if its log were empty too, a silent native log would prove nothing.
func a1Shims(t *testing.T) (string, string) {
	t.Helper()
	gcc, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("gcc not on PATH")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not on PATH")
	}
	dir, err := os.MkdirTemp("", "karkain-a1-shims")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	log := filepath.Join(dir, "calls.log")
	specs := []struct{ name, real string }{{"gcc", gcc}, {"go", goTool}}
	for _, sp := range specs {
		body := ""
		if runtime.GOOS == "windows" {
			body = fmt.Sprintf("@echo off\r\necho %s %%* >> \"%s\"\r\n\"%s\" %%*\r\nexit /b %%ERRORLEVEL%%\r\n", sp.name, log, sp.real)
		} else {
			body = fmt.Sprintf("#!/bin/sh\necho '%s $*' >> '%s'\nexec '%s' \"$@\"\n", sp.name, log, sp.real)
		}
		p := filepath.Join(dir, sp.name)
		if runtime.GOOS == "windows" {
			p += ".cmd"
		}
		if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir, log
}

func a1Run(t *testing.T, exe string, args []string, shimDir string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = append(os.Environ(), "PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.Env = append(cmd.Env, "KARKAIN_ENGINE=", "KARKAIN_KCC=")
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s %v timed out\nstdout:\n%s\nstderr:\n%s", exe, args, out.String(), errOut.String())
	}
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("%s %v: %v", exe, args, err)
		}
	}
	return out.String(), errOut.String(), code
}

// a1HostTarget picks the container the CURRENT host can actually execute.
// A fixed Windows target made the gate fail on Linux CI with "exec format
// error": kcc correctly emits a PE for that target, and Linux correctly
// refuses to run it. That is a gate defect, not a compiler defect.
func a1HostTarget() string {
	switch runtime.GOOS {
	case "windows":
		return "native-x86_64-windows"
	case "darwin":
		return "native-x86_64-macos"
	default:
		return "native-x86_64-linux"
	}
}

func a1ValidateImage(t *testing.T, img []byte) {
	t.Helper()
	var err error
	switch runtime.GOOS {
	case "windows":
		_, _, err = native.ParsePE(img)
	case "darwin":
		_, _, err = native.ParseMachO(img)
	default:
		_, _, err = native.Parse(img)
	}
	if err != nil {
		t.Fatalf("image is not a valid %s container: %v", runtime.GOOS, err)
	}
}

func a1Calls(t *testing.T, log string) []string {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

const a1Hello = `func main() {
    print("hello")
}
`

func TestPhaseA1_KccBuildWritesNativeImageDirectly(t *testing.T) {
	_, kcc := a1Prepare(t)
	shimDir, log := a1Shims(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "hello.kark")
	out := filepath.Join(dir, "hello-built")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	if err := os.WriteFile(src, []byte(a1Hello), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := a1Run(t, kcc, []string{"build", src, "--target", a1HostTarget(), "--output", out}, shimDir)
	if code != 0 {
		t.Fatalf("kcc build exit = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	img, err := os.ReadFile(out)
	if err != nil {
		// The hex is kilobytes and would bury the assertion in noise.
		head := stdout
		if len(head) > 200 {
			head = head[:200] + "..."
		}
		t.Fatalf("kcc did not write the image itself: %v\nstdout (truncated):\n%s", err, head)
	}
	if len(img) == 0 {
		t.Fatal("kcc wrote an empty image")
	}
	a1ValidateImage(t, img)

	// Execute it to prove the bytes are a runnable program, not just a
	// structurally valid container.
	proc := exec.Command(out)
	var runOut bytes.Buffer
	proc.Stdout = &runOut
	if err := proc.Run(); err != nil {
		t.Fatalf("executing kcc-produced image: %v", err)
	}
	if runOut.String() != "hello\n" {
		t.Errorf("image stdout = %q, want %q", runOut.String(), "hello\n")
	}

	if calls := a1Calls(t, log); len(calls) != 0 {
		t.Errorf("kcc native build invoked external tools: %v", calls)
	}

	// Positive control, with the SAME shims: the legacy path must call gcc.
	cli, _ := a1Prepare(t)
	legacySrc := filepath.Join(dir, "legacy.kark")
	if err := os.WriteFile(legacySrc, []byte(a1Hello), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(log, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, legacyCode := a1Run(t, cli, []string{"run", legacySrc, "--engine", "go"}, shimDir)
	if legacyCode != 0 {
		t.Fatalf("legacy Go-engine run exit = %d (positive control must work)", legacyCode)
	}
	if calls := a1Calls(t, log); len(calls) == 0 {
		t.Error("positive control did not invoke gcc/go; an empty native log would prove nothing")
	}
}

func TestPhaseA1_KccRunNativeImageDirectly(t *testing.T) {
	_, kcc := a1Prepare(t)
	shimDir, log := a1Shims(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "hello.kark")
	if err := os.WriteFile(src, []byte(a1Hello), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := a1Run(t, kcc, []string{"run", src, "--target", a1HostTarget()}, shimDir)
	if code != 0 {
		t.Fatalf("kcc native run exit = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if stdout != "hello\n" {
		t.Errorf("kcc run stdout = %q, want %q", stdout, "hello\n")
	}
	if calls := a1Calls(t, log); len(calls) != 0 {
		t.Errorf("kcc native run invoked external tools: %v", calls)
	}
}
