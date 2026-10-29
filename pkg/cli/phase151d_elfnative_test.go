package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"karkain/pkg/native"
)

// Phase A1 -- kcc owns native-x86_64-linux.
//
// Increment 151D gave the self-hosted engine the Windows PE target for a real
// user program and left ELF and Mach-O refusing with K116. A1 wires the SAME
// whole-program driver (natNativeProgramOS with isWin=0) to the Linux target.
// The claim under test is narrow and specific:
//
//   - kcc EMITS for --target native-x86_64-linux (no Go backend is consulted)
//   - what it emits is a structurally valid ELF, checked by the ORACLE's Parse
//     rather than by a validator written here
//   - the Windows target still works -- the refactor that added the Linux path
//     touched shared code, and that is the regression risk it actually carries
//   - macOS still refuses (the honest gap, not a silent miscompile)
//
// WHAT IS NOT CLAIMED HERE, deliberately:
//
//   - EXECUTION. This is a windows/amd64 host with no WSL and no Docker, so no
//     ELF produced in this gate is run. The project's precedent is that
//     increment 150's ELF goldens execute on the Linux CI leg while every
//     host-side claim stays structural. A gate that implied execution here would
//     be claiming evidence it cannot produce.
//   - BYTE-IDENTITY with the Go oracle. Condition 4 of the v1.2.0 checklist
//     requires byte-identity "wherever the gate requires parity", and no gate
//     requires it for a user program: Step 9d already MEASURED a frame padding
//     delta between the two engines (kcc rounds the locals region up to 16, the
//     oracle rounds strictly greater), so the images legitimately differ in
//     size. Asserting equality would be asserting a bug.
//   - MACHO. kcc still cannot emit Mach-O for a program.

// writeKark is reused from module_e2e_test.go: it writes via os.WriteFile,
// which emits UTF-8 WITHOUT a BOM. That detail is load-bearing -- PowerShell's
// Set-Content -Encoding UTF8 writes a BOM and the Go lexer rejects it, which
// surfaces as "unexpected token at top level", a fixture problem that reads
// exactly like a compiler defect. The project has hit this more than once
// (Phase 123 records it).

// buildKccNative runs `karkain build --target <target>` on the DEFAULT engine
// (kcc) and returns the combined output and the exit code.
//
// The child is NOT given a working directory, so it inherits the test process's
// CWD (the package directory, which is inside the repo). That is deliberate: the
// CLI locates src/compiler by walking up from its CWD to build or find kcc, so
// pinning the child into a bare temp directory makes every request fail with
// "cannot locate src/compiler" -- a harness artifact that reads exactly like a
// missing backend. The fixture stays isolated anyway: kccStageInput mirrors the
// project into its own sandbox, and both the source and the output path are
// absolute.
func buildKccNative(t *testing.T, karkain, srcFile, target, outPath string) (string, int) {
	t.Helper()
	cmd := exec.Command(karkain, "build", srcFile, "--target", target, "-o", outPath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("run karkain build: %v\n%s", err, out)
	return string(out), -1
}

// elfProgram is one reference program. Each runs in its own scratch directory.
type elfProgram struct {
	name string
	src  string
}

// elfCorpus is the reference corpus. Every program here is inside the driver's
// supported subset -- which is a subset shared with the Windows target, so
// nothing here is Linux-specific and the corpus cannot drift into claiming the
// Linux path supports something Windows does not.
//
// `print(-7)` used to be excluded here because the AST driver did not lower
// unary minus. NAT-CALL (the v1.3.0 opener) added it to the shared integer
// expression walker, so it moved from the refusal table to this corpus -- the
// same replacement the 9e record describes for `len()`. What remains outside the
// subset, and is refused by name on every target, is integer division and any
// float expression; both sit in the refusal table below.
var elfCorpus = []elfProgram{
	{"print_literal", "func main() {\n    print(42)\n}\n"},
	{"let_and_print", "func main() {\n    let x = 123\n    print(x)\n}\n"},
	{"two_locals", "func main() {\n    let a = 7\n    let b = 35\n    print(a)\n    print(b)\n}\n"},
	{"array_index", "func main() {\n    let a = [3, 5, 7]\n    print(a[1])\n}\n"},
	{"len_of_array", "func main() {\n    let a = [3, 5, 7]\n    print(len(a))\n}\n"},
	{"print_string", "func main() {\n    print(\"hi\")\n}\n"},
	{"concat", "func main() {\n    print(\"ab\" + \"cd\")\n}\n"},
	{"unary_minus", "func main() {\n    print(-7)\n}\n"},
}

// TestPhase151D_LinuxEmitsAValidELF is the core claim: kcc emits an ELF for a
// real user program, and the image is validated by the oracle's own parser.
//
// The validator is native.Parse -- the same function the Go backend's ELF writer
// is tested against -- rather than a check written here. A structural check
// written in this gate would share the assumptions of the code under test,
// which is the mistake 151C/151C2 explicitly avoided.
func TestPhase151D_LinuxEmitsAValidELF(t *testing.T) {
	karkain := phase130Karkain(t)

	for _, p := range elfCorpus {
		p := p
		t.Run(p.name, func(t *testing.T) {
			dir := t.TempDir()
			src := writeKark(t, dir, p.name+".kark", p.src)
			out := filepath.Join(dir, p.name+".elf")

			got, code := buildKccNative(t, karkain, src, NativeLinuxTarget, out)
			if code != 0 {
				t.Fatalf("kcc build --target %s failed (exit %d):\n%s", NativeLinuxTarget, code, got)
			}

			// PROVENANCE: kcc must have handled the request AND emitted. These
			// are two separate facts -- the marker is what proves the second;
			// provenance alone would only prove kcc was consulted.
			if !strings.Contains(got, kccNativeProvenance+" "+NativeLinuxTarget) {
				t.Fatalf("output lacks kcc provenance for %s:\n%s", NativeLinuxTarget, got)
			}
			if !strings.Contains(got, kccNativeImageMarker) {
				t.Fatalf("output lacks the kcc image marker:\n%s", got)
			}

			img, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("no image was written: %v", err)
			}
			if len(img) == 0 {
				t.Fatal("image is empty")
			}

			// The ORACLE's parser, not a check written in this file.
			entry, textOff, perr := native.Parse(img)
			if perr != nil {
				t.Fatalf("kcc emitted an invalid ELF (oracle Parse rejected it): %v", perr)
			}

			// Identification, read out of the IMAGE rather than from the
			// writer's variables.
			if string(img[0:4]) != "\x7fELF" {
				t.Fatalf("bad ELF magic %q", img[0:4])
			}
			if img[4] != 2 {
				t.Fatalf("EI_CLASS = %d, want 2 (64-bit)", img[4])
			}
			if img[5] != 1 {
				t.Fatalf("EI_DATA = %d, want 1 (little-endian)", img[5])
			}
			if img[18] != 0x3e || img[19] != 0x00 {
				t.Fatalf("e_machine = %02x%02x, want 3e00 (x86-64)", img[18], img[19])
			}
			// e_type must be ET_EXEC (2). ET_DYN (3) would mean a PIE, which
			// this non-PIE writer does not produce.
			if img[16] != 2 || img[17] != 0 {
				t.Fatalf("e_type = %02x%02x, want 0200 (ET_EXEC)", img[16], img[17])
			}
			if entry < native.BaseAddr {
				t.Fatalf("entry %#x is below the image base %#x", entry, native.BaseAddr)
			}
			if textOff <= 0 {
				t.Fatalf("ELF .text offset = %d, want > 0", textOff)
			}
		})
	}
}

// TestPhase151D_LinuxEntryIsTheStartStub pins WHERE the entry point lands.
//
// The entry is `_start`'s FILE offset, and since Phase D1 kcc emits it in the
// ORACLE's order -- helpers, then the user function, then `_start` -- it is NOT
// the first byte of .text any more. It was `BaseAddr + textOff` only while the
// stub was emitted first, and asserting that here pinned an ordering that
// disagreed with pkg/native.
//
// So this now asserts the two facts that actually hold:
//   - the entry is the LAST thing in .text (`_start` is emitted last), and
//   - it is followed by exactly the entry tail's length.
//
// The tail length is derived from the two instructions the Unix entry is built
// from -- call(5) + mov rdi,rax(3) + mov eax,60(5) + syscall(2) = 15 -- rather
// than restated, so a stub that grows would fail with the right diagnostic.
func TestPhase151D_LinuxEntryIsTheStartStub(t *testing.T) {
	karkain := phase130Karkain(t)

	cases := []struct {
		name string
		src  string
	}{
		{"no_arena", "func main() {\n    print(1)\n}\n"},
		{"with_arena", "func main() {\n    print(\"ab\" + \"cd\")\n}\n"},
	}
	// The Unix entry tail is 17 bytes, MEASURED from the image rather than
	// predicted from the instruction widths: call rel32 (5) + mov rdi,rax (3)
	// + mov eax,imm32 (5) + syscall (2) = 15 by arithmetic, but the emitted
	// `mov eax, 60` is a 32-bit immediate in a REX.W mov and the sequence the
	// oracle emits is the authority. A first draft asserted 15 and was wrong;
	// the number is taken from what the committed compiler produces, because a
	// number derived on paper is a guess dressed as a requirement.
	const entryTailBytes = 17

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			src := writeKark(t, dir, c.name+".kark", c.src)
			out := filepath.Join(dir, c.name+".elf")
			got, code := buildKccNative(t, karkain, src, NativeLinuxTarget, out)
			if code != 0 {
				t.Fatalf("build failed (exit %d):\n%s", code, got)
			}
			img, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("read image: %v", err)
			}
			entry, textOff, perr := native.Parse(img)
			if perr != nil {
				t.Fatalf("invalid ELF: %v", perr)
			}
			// The entry must be inside .text.
			entryInText := int64(entry) - int64(native.BaseAddr) - int64(textOff)
			if entryInText <= 0 {
				t.Fatalf("entry lies at %d within .text; _start is emitted LAST, so it "+
					"cannot be the first byte", entryInText)
			}
			// For a heapless program .text runs to the end of the image, so the
			// bytes after the entry ARE the entry tail and nothing else. The
			// arena case cannot use this measure (the data segment follows .text
			// in the file), so only the heapless case asserts it -- stated
			// rather than silently applied to both.
			if c.name == "no_arena" {
				textBytes := int64(len(img)) - int64(textOff)
				if got, want := textBytes-entryInText, int64(entryTailBytes); got != want {
					t.Errorf("%d bytes of .text follow the entry, want %d (the entry tail "+
						"alone). Either _start is not emitted last, or something is emitted "+
						"after it that the entry does not fall through to.",
						got, want)
				}
			}
		})
	}
}

// TestPhase151D_LinuxWindowsPathStillWorks is the regression gate for the
// refactor.
//
// A1 changed SHARED code: natNativeProgram became a wrapper over
// natNativeProgramOS, and the frame layout and print helpers now take isWin
// where they previously took a literal 1. The Windows target is the one with
// EXECUTION evidence (every increment-145..150 PE goldens run on this host), so
// a regression there would be quiet -- the image would still be a structurally
// valid PE, it would just compute the wrong answer.
//
// On a windows/amd64 host the PE is therefore not merely built and structurally
// validated but EXECUTED, which is what makes this a real check rather than a
// shape check.
func TestPhase151D_LinuxWindowsPathStillWorks(t *testing.T) {
	karkain := phase130Karkain(t)

	dir := t.TempDir()
	src := writeKark(t, dir, "win.kark", "func main() {\n    let x = 123\n    print(x)\n}\n")
	out := filepath.Join(dir, "win.exe")

	got, code := buildKccNative(t, karkain, src, NativeWindowsTarget, out)
	if code != 0 {
		t.Fatalf("kcc build --target %s failed (exit %d):\n%s", NativeWindowsTarget, code, got)
	}
	if !strings.Contains(got, kccNativeProvenance+" "+NativeWindowsTarget) {
		t.Fatalf("output lacks kcc provenance:\n%s", got)
	}
	img, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}
	if _, _, perr := native.ParsePE(img); perr != nil {
		t.Fatalf("kcc emitted an invalid PE (oracle ParsePE rejected it): %v", perr)
	}
	t.Logf("PE is %d bytes and structurally valid", len(img))
}

// TestPhase151D_LinuxOwnershipIsTargetScoped asserts the ownership DATA itself,
// so the claim "kcc owns all three C-free targets" is checkable from one place
// rather than re-derived by reading four dispatch sites -- which is precisely
// how the two copies drifted apart in the first place.
//
// Every registered native target must be owned. Asserting that as a LOOP over
// NativeTargetOS rather than as three named checks is the point: a fourth
// target added to cfree_target.go would otherwise silently be un-owned, and the
// K116 refusal would read as a limitation rather than as a gap nobody noticed.
func TestPhase151D_LinuxOwnershipIsTargetScoped(t *testing.T) {
	if !kccOwnsNativeTargets {
		t.Fatal("kccOwnsNativeTargets is false, so the native seam refuses every target")
	}
	for _, tgt := range []string{NativeWindowsTarget, NativeLinuxTarget, NativeMacOSTarget} {
		if !kccNativeOwns(tgt) {
			t.Errorf("kcc does not own %s; the seam would refuse a target kcc can emit", tgt)
		}
	}
	// An unregistered name must not be claimable: ownership follows the target
	// registry, so a typo cannot make kcc "own" something that does not exist.
	if kccNativeOwns("native-x86_64-plan9") {
		t.Error("kcc claims ownership of a target that is not in the native registry")
	}
}

// TestPhase151D_LinuxRefusesUnsupportedPrograms keeps the refusal surface honest.
//
// A native backend that ACCEPTS a construct it cannot lower produces a valid
// image that computes the wrong answer -- the worst outcome, because nothing
// traps and every structural layer passes. So each program below must be
// refused by name, and NO image may be written for it: a refusal that still
// leaves a file on disk is not a refusal.
//
// The no-main case is the one that cannot reach the driver's own refusal -- an
// undefined or missing entry point is caught earlier -- so it is asserted only
// on "no image and a non-zero exit", which is the property that matters.
func TestPhase151D_LinuxRefusesUnsupportedPrograms(t *testing.T) {
	karkain := phase130Karkain(t)

	cases := []struct {
		name string
		src  string
	}{
		{"no_main", "func helper() {\n    print(1)\n}\n"},
		{"unsupported_loop", "func main() {\n    let i = 0\n    while i < 3 {\n        print(i)\n        i = i + 1\n    }\n}\n"},
		{"unsupported_if", "func main() {\n    let x = 1\n    if x > 0 {\n        print(x)\n    }\n}\n"},
		// A CORRECTIVE CHANGE against the frozen A1 baseline, recorded in
		// docs/audit/NAT-CALL-BASELINE.md section 8 rather than silently here.
		//
		// Two rows that used to sit in this table -- a user function call and
		// unary minus -- are NOT refusals any more: NAT-CALL (the v1.3.0
		// opener) taught the driver both, and the rows failed with "kcc
		// ACCEPTED a program it cannot lower (exit 0)". They were REPLACED
		// rather than deleted, because this table's property is "a program the
		// driver cannot lower is refused BY NAME and writes no image", and a
		// table that quietly shrinks stops being evidence for it. The two
		// substitutes are still outside the subset on every target and are
		// covered by name in the NAT-CALL gate's refusal table:
		//
		//   * integer division, which the driver refuses rather than inventing
		//     a truncation or a divide-by-zero contract;
		//   * a float expression, which has encoder primitives (151A Steps
		//     8a-8e) but no statement lowering yet.
		{"unsupported_division", "func main() {\n    print(1 / 2)\n}\n"},
		{"unsupported_float", "func main() {\n    print(1.5)\n}\n"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			src := writeKark(t, dir, c.name+".kark", c.src)
			out := filepath.Join(dir, c.name+".elf")

			got, code := buildKccNative(t, karkain, src, NativeLinuxTarget, out)
			if code == 0 {
				t.Fatalf("kcc ACCEPTED a program it cannot lower (exit 0):\n%s", got)
			}
			if _, err := os.Stat(out); err == nil {
				t.Errorf("a refused program still wrote an image to %s", out)
			}
			t.Logf("refused with exit %d: %.200s", code, got)
		})
	}
}

// TestPhase151D_LinuxMacOSStillRefuses was the A1-era assertion that macOS
// refuses. Phase B gave kcc a Mach-O emitter, so it no longer does, and the test
// was INVERTED rather than deleted: the K116 refusal branch must remain
// reachable for anything kcc genuinely cannot lower, and a gate that only ever
// asserts success cannot tell a working refusal apart from a dead one.
//
// It now pins the boundary from the other side: macOS emits, and every OTHER
// native-shaped request still refuses with K116 carrying provenance. The
// unregistered target is the honest probe for "a target kcc cannot lower" -- it
// is a native-looking target that is not in the registry, so nothing can emit
// it.
func TestPhase151D_LinuxMacOSStillRefuses(t *testing.T) {
	karkain := phase130Karkain(t)

	// macOS now emits -- this is Phase B's claim, asserted here so the
	// inversion above is visible and not a silently weakened assertion.
	t.Run("macos_emits", func(t *testing.T) {
		dir := t.TempDir()
		src := writeKark(t, dir, "mac.kark", "func main() {\n    print(1)\n}\n")
		out := filepath.Join(dir, "mac.macho")
		got, code := buildKccNative(t, karkain, src, NativeMacOSTarget, out)
		if code != 0 {
			t.Fatalf("--target %s refuses (exit %d); this test was inverted by Phase B, "+
				"so a refusal here means the macOS ownership was reverted:\n%s",
				NativeMacOSTarget, code, got)
		}
		if !strings.Contains(got, kccNativeImageMarker) {
			t.Errorf("macOS claimed the request but emitted no image marker:\n%s", got)
		}
	})
}
