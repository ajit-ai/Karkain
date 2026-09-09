package runtime_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Phase 101 gate: the libc-free runtime compiles standalone with a freestanding
// toolchain and produces a working executable that prints "hello" and exits 0.

const phase101FreestandingDir = "../../runtime/freestanding"

func findGCC(t *testing.T) string {
	t.Helper()
	if gcc, err := exec.LookPath("gcc"); err == nil {
		return gcc
	}
	for _, cand := range []string{
		`C:\msys64\ucrt64\bin\gcc.exe`,
		`C:\msys64\mingw64\bin\gcc.exe`,
	} {
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	t.Skip("gcc not found; freestanding compile gate requires a freestanding toolchain")
	return ""
}

func TestPhase101_FreestandingHello(t *testing.T) {
	gcc := findGCC(t)
	dir := t.TempDir()

	sources := []string{
		"hello.c",
		"karkain_runtime.c",
		"karkain_memory.c",
		"karkain_io.c",
		"karkain_string.c",
		"karkain_math.c",
		"karkain_platform.c",
	}
	args := []string{"-std=c11", "-ffreestanding", "-nostdlib", "-fno-builtin", "-O2"}
	for _, s := range sources {
		args = append(args, filepath.Join(phase101FreestandingDir, s))
	}
	exe := "hello"
	if runtime.GOOS == "windows" {
		exe = "hello.exe"
		args = append(args, "-Wl,-e,_start", "-lkernel32")
	}
	args = append(args, "-o", filepath.Join(dir, exe))

	out, err := exec.Command(gcc, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("freestanding compile failed: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(dir, "data.bin"), []byte("xyz"), 0o644); err != nil {
		t.Fatalf("writing data.bin: %v", err)
	}
	run := exec.Command(filepath.Join(dir, exe))
	run.Dir = dir
	got, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("freestanding run failed: %v\n%s", err, got)
	}

	want := "hello\n" +
		"i=-42\n" +
		"sqrt2=1.414214\n" +
		"floor2.7=2\n" +
		"fmod=1.500000\n" +
		"pow=1.414214\n" +
		"parse=-42\n" +
		"parse2=42\n" +
		"cmpabc=0\n" +
		"cmpl=-1\n" +
		"arena: ok\n" +
		"file=xyz\n"
	if string(got) != want {
		t.Fatalf("unexpected freestanding output:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}