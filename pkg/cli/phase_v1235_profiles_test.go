// v1.2.35 slice 1: user-visible build profiles.
//
// The baseline (docs/audit/V1.2.35-BUILD-PROFILES-BASELINE.md) records that
// there is no --profile flag and that the C23 link flags are hard-coded to -O0.
// This gate uses a gcc PATH shim so "the flag reached the compiler" is observed
// rather than inferred from binary size, and it asserts that a profile never
// changes what the program prints.
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
)

var v1235Once sync.Once
var v1235CLI string
var v1235Err error

func v1235Prepare(t *testing.T) string {
	t.Helper()
	for _, dir := range []string{
		`C:\Program Files\Git\cmd`,
		`C:\Windows\System32\WindowsPowerShell\v1.0`,
		`C:\msys64\mingw64\bin`,
	} {
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			continue
		}
		if !v1235PathHas(dir) {
			_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		}
	}
	v1235Once.Do(func() { v1235Err = buildKarkainErr(t, &v1235CLI) })
	if v1235Err != nil {
		t.Fatalf("building karkain: %v", v1235Err)
	}
	return v1235CLI
}

func v1235PathHas(dir string) bool {
	for _, p := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
		if strings.EqualFold(p, dir) {
			return true
		}
	}
	return false
}

func v1235Shims(t *testing.T) (string, string) {
	t.Helper()
	gcc, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("gcc not on PATH")
	}
	dir, err := os.MkdirTemp("", "karkain-v1235-shims")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	log := filepath.Join(dir, "gcc-calls.log")
	name := "gcc"
	// Double quotes are required here: inside single quotes the shell writes
	// the literal characters "$*" and the log proves nothing (CI caught this
	// on the Linux leg while the Windows .cmd shim expanded %* correctly).
	body := fmt.Sprintf("#!/bin/sh\necho \"gcc $*\" >> '%s'\nexec '%s' \"$@\"\n", log, gcc)
	if runtime.GOOS == "windows" {
		name += ".cmd"
		body = fmt.Sprintf("@echo off\r\necho gcc %%* >> \"%s\"\r\n\"%s\" %%*\r\nexit /b %%ERRORLEVEL%%\r\n", log, gcc)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir, log
}

func v1235Run(t *testing.T, shimDir string, args ...string) (string, string, int) {
	t.Helper()
	cli := v1235Prepare(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, args...)
	cmd.Env = append(os.Environ(), "PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("command timed out: %v", args)
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

// v1235HostTriple selects a concrete same-machine triple. A plain `karkain
// build` is compile-only by design (commands.go:178-187), so the gate uses
// `run` with a triple to exercise the real C23 link.
func v1235HostTriple() string {
	switch runtime.GOOS {
	case "windows":
		return "x86_64-windows"
	case "darwin":
		return "x86_64-macos"
	default:
		return "x86_64-linux"
	}
}

func v1235RunRetry(t *testing.T, shimDir string, args ...string) (string, string, int) {
	t.Helper()
	for attempt := 0; attempt < 2; attempt++ {
		stdout, stderr, code := v1235Run(t, shimDir, args...)
		if code != 0 && attempt == 0 && strings.Contains(stdout+stderr, "C compilation failed") {
			continue // documented gcc-kill / pagefile flake, not a profile defect
		}
		return stdout, stderr, code
	}
	panic("unreachable")
}

func v1235Calls(t *testing.T, log string) string {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatal(err)
	}
	return string(data)
}

const v1235Hello = `func main() {
    print("hello profiles")
}
`

func TestPhaseV1235_UnknownProfileRejected(t *testing.T) {
	shimDir, _ := v1235Shims(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "hello.kark")
	out := filepath.Join(dir, "hello.exe")
	if err := os.WriteFile(src, []byte(v1235Hello), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := v1235Run(t, shimDir, "build", src, "--engine", "go", "--profile", "bogus", "-o", out)
	if code == 0 {
		t.Errorf("unknown profile accepted (exit 0)\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
	all := stdout + stderr
	if !strings.Contains(all, "profile") {
		t.Errorf("diagnostic does not name the bad profile:\n%s", all)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("unknown profile still produced an executable")
	}
}

func TestPhaseV1235_ProfileFlagsReachCompiler(t *testing.T) {
	shimDir, log := v1235Shims(t)
	cases := []struct {
		profile, flag string
	}{
		{"dev", "-O0"},
		{"release", "-O2"},
		{"size", "-Os"},
	}
	for _, tc := range cases {
		t.Run(tc.profile, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "hello.kark")
			if err := os.WriteFile(src, []byte(v1235Hello), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(log, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{"run", src, "--engine", "go", "--target", v1235HostTriple(), "--profile", tc.profile}
			start := time.Now()
			stdout, stderr, code := v1235RunRetry(t, shimDir, args...)
			elapsed := time.Since(start)
			if code != 0 {
				t.Fatalf("run exit = %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			calls := v1235Calls(t, log)
			if !strings.Contains(calls, tc.flag) {
				t.Errorf("gcc log does not contain %s:\n%s", tc.flag, calls)
			}
			got := strings.ReplaceAll(stdout, "\r\n", "\n")
			if got != "hello profiles\n" {
				t.Errorf("stdout changed under profile %s: %q", tc.profile, got)
			}

			// Determinism: same profile, same source, same observable result.
			stdout2, _, code2 := v1235RunRetry(t, shimDir, args...)
			if code2 != 0 {
				t.Fatalf("second run exit = %d", code2)
			}
			if strings.ReplaceAll(stdout2, "\r\n", "\n") != got {
				t.Errorf("profile %s is not deterministic: %q vs %q", tc.profile, got, stdout2)
			}
			t.Logf("profile %s: flag %s, elapsed %s, stdout %q", tc.profile, tc.flag, elapsed, got)
		})
	}
}
