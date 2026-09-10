package runtime_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Phase 107 gate: the concurrency runtime (scheduler, tasks, channels, actors,
// work stealing) compiles standalone, runs every scenario deterministically,
// and sustains > 1,000,000 channel messages per second.
//
// The gate mirrors phase102_core_test.go: host GCC builds the runtime from a
// scratch directory and the executable must reproduce the exact scenario
// outputs (including the throughput line). KARKAIN_SCEN is set to a prefix so
// a regression that breaks an earlier scenario is bisectable on any host; the
// full run is what we assert against.

const phase107ConcurrencyDir = "../../runtime/concurrency/c"

func TestPhase107_ConcurrencyRuntime(t *testing.T) {
	gcc := findGCC(t)
	dir := t.TempDir()

	sources := []string{
		filepath.Join(phase107ConcurrencyDir, "concurrency_main.c"),
		filepath.Join(phase107ConcurrencyDir, "karkain_channel.c"),
		filepath.Join(phase107ConcurrencyDir, "karkain_actor.c"),
		filepath.Join(phase107ConcurrencyDir, "karkain_scheduler.c"),
	}
	args := []string{
		"-std=c11", "-O2", "-Wall", "-Wextra",
		"-I", phase107ConcurrencyDir,
	}
	for _, s := range sources {
		args = append(args, s)
	}
	exe := "conc"
	if runtime.GOOS == "windows" {
		exe = "conc.exe"
	}
	args = append(args, "-o", filepath.Join(dir, exe))

	if out, err := exec.Command(gcc, args...).CombinedOutput(); err != nil {
		t.Fatalf("concurrency compile failed: %v\n%s", err, out)
	}

	run := exec.Command(filepath.Join(dir, exe))
	got, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("concurrency run failed: %v\n%s", err, got)
	}

	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(got), "\r\n", "\n")), "\n")
	wantPrefix := []string{
		"spawn:0,-1",
		"sum:210",
		"actor:100",
		"actorstop:1",
		"steal:1000",
		"bounded:210",
		"close:1,1,0,0,7,1",
	}
	if len(lines) < len(wantPrefix)+2 {
		t.Fatalf("concurrency output too short:\n%s", got)
	}
	for i, w := range wantPrefix {
		if lines[i] != w {
			t.Fatalf("line %d: got %q want %q\nfull output:\n%s", i, lines[i], w, got)
		}
	}
	if lines[len(lines)-1] != "done:107" {
		t.Fatalf("last line: got %q want %q\n%s", lines[len(lines)-1], "done:107", got)
	}
	if !strings.HasPrefix(lines[len(lines)-2], "thr-sum:") ||
		lines[len(lines)-2] != "thr-sum:1000000" {
		t.Fatalf("thr-sum line: got %q want thr-sum:1000000\n%s", lines[len(lines)-2], got)
	}
	// Throughput gate: > 1,000,000 channel messages per second.
	thr := strings.TrimPrefix(lines[len(lines)-3], "thr:")
	var n int
	if _, err := fmt.Sscanf(thr, "%d", &n); err != nil {
		t.Fatalf("bad throughput line %q: %v", lines[len(lines)-3], err)
	}
	if n < 1000000 {
		t.Fatalf("throughput %d msg/s below the 1,000,000 msg/s gate", n)
	}
}

func TestPhase107_StressDeterminism(t *testing.T) {
	gcc := findGCC(t)
	dir := t.TempDir()

	sources := []string{
		filepath.Join(phase107ConcurrencyDir, "concurrency_main.c"),
		filepath.Join(phase107ConcurrencyDir, "karkain_channel.c"),
		filepath.Join(phase107ConcurrencyDir, "karkain_actor.c"),
		filepath.Join(phase107ConcurrencyDir, "karkain_scheduler.c"),
	}
	args := []string{
		"-std=c11", "-O2", "-Wall", "-Wextra",
		"-I", phase107ConcurrencyDir,
	}
	for _, s := range sources {
		args = append(args, s)
	}
	exe := "conc"
	if runtime.GOOS == "windows" {
		exe = "conc.exe"
	}
	args = append(args, "-o", filepath.Join(dir, exe))
	if out, err := exec.Command(gcc, args...).CombinedOutput(); err != nil {
		t.Fatalf("stress compile failed: %v\n%s", err, out)
	}

	// Three full runs must produce byte-identical scenario output except the
	// measured throughput line.
	var refLines []string
	for i := 0; i < 3; i++ {
		got, err := exec.Command(filepath.Join(dir, exe)).CombinedOutput()
		if err != nil {
			t.Fatalf("stress run %d failed: %v\n%s", i+1, err, got)
		}
		lines := strings.Split(strings.TrimSpace(string(got)), "\n")
		if i > 0 {
			for j, w := range refLines {
				if strings.HasPrefix(w, "thr:") {
					continue
				}
				if lines[j] != w {
					t.Fatalf("run %d diverged at line %d: got %q want %q\n%s",
						i+1, j, lines[j], w, got)
				}
			}
			continue
		}
		refLines = lines
	}
}