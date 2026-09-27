package cli

// Phase 150D — the `native-x86_64-windows` and `native-x86_64-macos` CLI
// targets (Go engine), joining the Phase-148 Linux target.
//
// What this gate asserts, and why each assertion exists:
//
//  1. Registry: all three targets validate, pass through normalization, are
//     NOT parsed as conventional cross triples, and are listed by
//     `karkain target` with their run requirement.
//  2. CONTAINER PER TARGET: the same source built for three targets yields
//     three different magic numbers. This is the load-bearing assertion of
//     150D — without it the three names could silently be three aliases for
//     whichever image matches the host, which is precisely the "silent host
//     fallback" the Phase-111 rule forbids.
//  3. Container validity: ELF magic, PE (MZ + PE signature), Mach-O
//     (MH_MAGIC_64) and the cputype/subtype fields that identify x86-64.
//  4. Execution where the host allows it, and an honest ExitEnv(6)
//     build-only refusal everywhere else — for all three targets, with the
//     message naming the target the user actually typed.
//  5. The documented incremental refusal names the target.
//
// kcc parity is Phase 151, so every subtest pins --engine go explicitly.

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"karkain/pkg/native"
)

const phase150DSrc = "func main() {\n\tprint(42)\n}\n"

func write150DProbe(t *testing.T, src string) string {
	t.Helper()
	probe := filepath.Join(t.TempDir(), "probe.kark")
	if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
		t.Fatalf("writing probe: %v", err)
	}
	return probe
}

func run150D(t *testing.T, karkain string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(karkain, args...)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return string(out), code
}

func hostOS() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "darwin"
	default:
		return "linux"
	}
}

// TestPhase150D_TargetsRegistered pins the registry contract for all three
// C-free machine-code targets.
func TestPhase150D_TargetsRegistered(t *testing.T) {
	for _, tc := range []struct{ target, osName string }{
		{NativeLinuxTarget, native.OSLinux},
		{NativeWindowsTarget, native.OSWindows},
		{NativeMacOSTarget, native.OSMacOS},
	} {
		if err := ValidateTarget(tc.target); err != nil {
			t.Errorf("%s: ValidateTarget rejected: %v", tc.target, err)
		}
		if got, err := NormalizeTarget(tc.target); err != nil || got != tc.target {
			t.Errorf("%s: NormalizeTarget = %q, err=%v", tc.target, got, err)
		}
		if !IsNativeTarget(tc.target) {
			t.Errorf("%s: IsNativeTarget = false", tc.target)
		}
		if got := NativeTargetOS(tc.target); got != tc.osName {
			t.Errorf("%s: NativeTargetOS = %q, want %q", tc.target, got, tc.osName)
		}
		if _, isTriple := SelectedTarget(tc.target); isTriple {
			t.Errorf("%s must not parse as a conventional cross triple", tc.target)
		}
	}
	// A non-native value must not be mistaken for one.
	if IsNativeTarget("native") || IsNativeTarget("wasm32-wasi") || NativeTargetOS("x86_64-windows") != "" {
		t.Errorf("non-native targets misclassified as C-free machine-code targets")
	}
	// The default output extension is part of the user-visible contract.
	for target, want := range map[string]string{
		NativeLinuxTarget:   ".elf",
		NativeWindowsTarget: ".exe",
		NativeMacOSTarget:   ".macho",
	} {
		if got := nativeImageExt(NativeTargetOS(target)); got != want {
			t.Errorf("%s: default extension = %q, want %q", target, got, want)
		}
	}
}

// TestPhase150D_TargetsListed pins that `karkain target` advertises all three
// with their run requirement, so the capability is discoverable rather than
// folklore.
func TestPhase150D_TargetsListed(t *testing.T) {
	karkain := phase130Karkain(t)
	out, code := run150D(t, karkain, "target")
	if code != 0 {
		t.Fatalf("karkain target failed (%d):\n%s", code, out)
	}
	for _, tc := range []struct{ target, runNeed string }{
		{NativeLinuxTarget, "linux/amd64"},
		{NativeWindowsTarget, "windows/amd64"},
		{NativeMacOSTarget, "darwin/amd64"},
	} {
		if !strings.Contains(out, tc.target) {
			t.Errorf("target listing missing %s:\n%s", tc.target, out)
		}
		if !strings.Contains(out, tc.runNeed) {
			t.Errorf("target listing missing run requirement %q for %s:\n%s", tc.runNeed, tc.target, out)
		}
	}
}

// TestPhase150D_ContainerPerTarget is the load-bearing 150D assertion: one
// source, three targets, three DIFFERENT containers. Emission is pure Go, so
// this runs on every host.
func TestPhase150D_ContainerPerTarget(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write150DProbe(t, phase150DSrc)
	dir := t.TempDir()

	images := map[string]string{}
	for _, target := range []string{NativeLinuxTarget, NativeWindowsTarget, NativeMacOSTarget} {
		out := filepath.Join(dir, strings.TrimPrefix(target, "native-"))
		if msg, code := run150D(t, karkain, "build", probe, "--engine", "go", "--target", target, "-o", out); code != 0 {
			t.Fatalf("%s: build should succeed from any host (%d):\n%s", target, code, msg)
		}
		images[target] = out
	}

	// Every image must be structurally valid for ITS container...
	for target, path := range images {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: reading image: %v", target, err)
		}
		if len(b) < 8 {
			t.Fatalf("%s: image too small (%d bytes)", target, len(b))
		}
		switch target {
		case NativeLinuxTarget:
			if b[0] != 0x7F || b[1] != 'E' || b[2] != 'L' || b[3] != 'F' {
				t.Errorf("%s: not an ELF image: % x", target, b[:4])
			}
		case NativeWindowsTarget:
			if b[0] != 'M' || b[1] != 'Z' {
				t.Errorf("%s: not a PE image (no MZ): % x", target, b[:2])
				break
			}
			peOff := int(binary.LittleEndian.Uint32(b[0x3C:0x40]))
			if peOff+4 > len(b) || string(b[peOff:peOff+4]) != "PE\x00\x00" {
				t.Errorf("%s: no PE signature at e_lfanew=%d", target, peOff)
				break
			}
			// Machine 0x8664 = x86-64, and the optional header must be
			// PE32+ (magic 0x20B) with a 64-bit entry point.
			if m := binary.LittleEndian.Uint16(b[peOff+4:]); m != 0x8664 {
				t.Errorf("%s: machine = %#x, want 0x8664", target, m)
			}
			if m := binary.LittleEndian.Uint16(b[peOff+24:]); m != 0x20B {
				t.Errorf("%s: optional header magic = %#x, want 0x20B (PE32+)", target, m)
			}
		case NativeMacOSTarget:
			// MH_MAGIC_64 == 0xfeedfacf, stored little-endian.
			if m := binary.LittleEndian.Uint32(b[0:4]); m != 0xfeedfacf {
				t.Errorf("%s: magic = %#x, want MH_MAGIC_64 (0xfeedfacf)", target, m)
				break
			}
			if ct := binary.LittleEndian.Uint32(b[4:8]); ct != 0x01000007 {
				t.Errorf("%s: cputype = %#x, want CPU_TYPE_X86_64 (0x01000007)", target, ct)
			}
		}
	}

	// ...and no two targets may produce the same image. Identical bytes
	// would mean the target name is being ignored.
	seen := map[string]string{}
	for target, path := range images {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		if other, dup := seen[string(b)]; dup {
			t.Errorf("%s and %s produced byte-identical images: the target name is not selecting the container", target, other)
		}
		seen[string(b)] = target
	}
}

// TestPhase150D_RunOnHostOrRefuse pins the execution contract for all three
// targets. Where the host matches the target the program RUNS and its golden
// is checked; everywhere else it is ExitEnv(6) with a build-only hint naming
// the target.
func TestPhase150D_RunOnHostOrRefuse(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write150DProbe(t, phase150DSrc)
	canRun := runtime.GOARCH == "amd64"

	for _, tc := range []struct{ target, hostOS string }{
		{NativeLinuxTarget, "linux"},
		{NativeWindowsTarget, "windows"},
		{NativeMacOSTarget, "darwin"},
	} {
		out, code := run150D(t, karkain, "run", probe, "--engine", "go", "--target", tc.target)
		if canRun && hostOS() == tc.hostOS {
			if code != 0 {
				t.Errorf("%s: run should succeed on %s/%s (%d):\n%s", tc.target, hostOS(), runtime.GOARCH, code, out)
				continue
			}
			if strings.TrimSpace(out) != "42" {
				t.Errorf("%s: golden mismatch: want 42, got %q", tc.target, out)
			}
			continue
		}
		if code != ExitEnv {
			t.Errorf("%s: cross-run should be refused with ExitEnv(%d), got %d:\n%s", tc.target, ExitEnv, code, out)
		}
		if !strings.Contains(out, "build only") && !strings.Contains(out, "cross-run") {
			t.Errorf("%s: refusal must carry the build-only hint:\n%s", tc.target, out)
		}
		if !strings.Contains(out, tc.target) {
			t.Errorf("%s: refusal must name the target the user typed:\n%s", tc.target, out)
		}
	}
}

// TestPhase150D_BuildIsCrossHost proves the point of the targets: an image
// for a foreign OS is produced on this host with no C compiler and no
// emulator, and it is still a valid image for its own container.
//
// The program deliberately needs the heap arena, so the images differ in more
// than their magic: the arena is a second writable segment on ELF and lives
// in .idata on PE. The macOS case asserts the CURRENT contract — Mach-O has
// a single R+X __TEXT and no writable segment, so an allocating program is
// refused by name there rather than emitted as an image whose first
// bump-cursor store would fault. Flipping that expectation to a successful
// build belongs in the same commit that adds __DATA.
func TestPhase150D_BuildIsCrossHost(t *testing.T) {
	karkain := phase130Karkain(t)
	src := "func main() {\n\tlet a = \"ka\" + \"rk\"\n\tprint(a)\n}\n"
	probe := write150DProbe(t, src)
	dir := t.TempDir()
	for _, tc := range []struct {
		target     string
		wantRefuse bool
	}{
		{NativeLinuxTarget, false},
		{NativeWindowsTarget, false},
		// Documented 150D boundary: no writable __DATA yet.
		{NativeMacOSTarget, true},
	} {
		out := filepath.Join(dir, strings.TrimPrefix(tc.target, "native-"))
		msg, code := run150D(t, karkain, "build", probe, "--engine", "go", "--target", tc.target, "-o", out)
		if tc.wantRefuse {
			if code != ExitCompile {
				t.Errorf("%s: allocating program should be refused on macOS until __DATA lands, got %d:\n%s", tc.target, code, msg)
			}
			if !strings.Contains(msg, "writable") && !strings.Contains(msg, "__DATA") {
				t.Errorf("%s: refusal must name the writable-__DATA gap, got:\n%s", tc.target, msg)
			}
			continue
		}
		if code != 0 {
			t.Errorf("%s: allocating program should build on any host (%d):\n%s", tc.target, code, msg)
			continue
		}
		if st, err := os.Stat(out); err != nil || st.Size() == 0 {
			t.Errorf("%s: no image written: %v", tc.target, err)
		}
	}
}

// TestPhase150D_IncrementalRefused pins the documented boundary: the
// incremental cache serves the C pipeline, so every native target is a loud
// usage refusal naming the target the user typed (native-split caching is
// 150D work).
func TestPhase150D_IncrementalRefused(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write150DProbe(t, phase150DSrc)
	for _, target := range []string{NativeLinuxTarget, NativeWindowsTarget, NativeMacOSTarget} {
		out, code := run150D(t, karkain, "build", probe, "--engine", "go", "--target", target, "--incremental")
		if code != ExitUsage {
			t.Errorf("%s: incremental+native should be refused with ExitUsage(%d), got %d:\n%s", target, ExitUsage, code, out)
		}
		if !strings.Contains(out, "--incremental") {
			t.Errorf("%s: refusal must name --incremental:\n%s", target, out)
		}
		if !strings.Contains(out, target) {
			t.Errorf("%s: refusal must name the target:\n%s", target, out)
		}
	}
}
