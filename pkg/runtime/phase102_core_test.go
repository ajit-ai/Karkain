package runtime_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// Phase 102 gate: the Native Runtime Core (values, strings, arrays, memory,
// I/O) compiles standalone on the Phase 101 freestanding layer and produces a
// working native binary with deterministic output.

const phase102CoreDir = "../../runtime/core"

func TestPhase102_NativeRuntimeCore(t *testing.T) {
	gcc := findGCC(t)
	dir := t.TempDir()

	sources := []string{
		filepath.Join(phase102CoreDir, "core_main.c"),
		filepath.Join(phase102CoreDir, "karkain_mem.c"),
		filepath.Join(phase102CoreDir, "karkain_value.c"),
		filepath.Join(phase102CoreDir, "karkain_nstr.c"),
		filepath.Join(phase102CoreDir, "karkain_narr.c"),
		filepath.Join(phase102CoreDir, "karkain_core_io.c"),
		filepath.Join(phase101FreestandingDir, "karkain_runtime.c"),
		filepath.Join(phase101FreestandingDir, "karkain_memory.c"),
		filepath.Join(phase101FreestandingDir, "karkain_io.c"),
		filepath.Join(phase101FreestandingDir, "karkain_string.c"),
		filepath.Join(phase101FreestandingDir, "karkain_math.c"),
		filepath.Join(phase101FreestandingDir, "karkain_platform.c"),
	}
	args := []string{
		"-std=c11", "-ffreestanding", "-nostdlib", "-fno-builtin", "-O2",
		"-I", phase102CoreDir, "-I", phase101FreestandingDir,
	}
	for _, s := range sources {
		args = append(args, s)
	}
	exe := "core"
	if runtime.GOOS == "windows" {
		exe = "core.exe"
		args = append(args, "-Wl,-e,_start", "-lkernel32")
	}
	args = append(args, "-o", filepath.Join(dir, exe))

	out, err := exec.Command(gcc, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("core compile failed: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(dir, "data.bin"), []byte("xyz"), 0o644); err != nil {
		t.Fatalf("writing data.bin: %v", err)
	}
	run := exec.Command(filepath.Join(dir, exe))
	run.Dir = dir
	got, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("core run failed: %v\n%s", err, got)
	}

	want := "phase102:core\n" +
		"int:42\n" +
		"float:2.500000\n" +
		"bool:true\n" +
		"nil:0\n" +
		"kind:int\n" +
		"tag:TYPE_FLOAT64\n" +
		"truthy int:1\n" +
		"truthy zero:0\n" +
		"eq:1\n" +
		"neq:0\n" +
		"asint:3\n" +
		"str:hello\n" +
		"nlen:5\n" +
		"ncat:hello world\n" +
		"nslice:ell\n" +
		"ncmp:-1\n" +
		"neqs:0\n" +
		"nstart:1\n" +
		"strvalue:hello world\n" +
		"arr len:3\n" +
		"arr[0]:10\n" +
		"arr set:1\n" +
		"pop:30\n" +
		"arr len:2\n" +
		"arrval:[99, 20]\n" +
		"realloc:1\n" +
		"mem free:1\n" +
		"calloc:1\n" +
		"reset alloc:1\n" +
		"file:xyz\n"
	if string(got) != want {
		t.Fatalf("unexpected core output:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}