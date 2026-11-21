package cli

import (
	"bytes"
	"context"
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

// Phase 2.2 (increment 151, slice "NAT-FORIN") -- `for x in arr` in the
// self-hosted native driver.
//
// This is the first iterator construct the kcc native driver lowers. It is a
// real control-flow shape: it carries a hidden index slot, its loop variable is
// rebound on every iteration, and `break`/`continue` must act on the NEAREST
// loop. The oracle's lowering is in pkg/native/program.go (emitForIn). The
// driver reproduces it rather than inventing a second semantics.
//
// EVIDENCE DISCIPLINE. There are four independent layers, because a single one
// can be satisfied by the wrong implementation:
//
//  1. EXECUTION: kcc-produced images for the host container are written,
//     validated by the ORACLE's parser, executed, and compared to EXACT bytes.
//     Nothing is trimmed -- the Step 9b defect was a missing trailing newline,
//     and a TrimSpace assertion would have called it a pass. The Go oracle's
//     image is built and executed from the SAME source as a second opinion.
//  2. BYTE IDENTITY: the LINUX ELF image is compared byte for byte to the one
//     the Go oracle emits from the same source. A golden would be wrong here:
//     it would freeze whatever bytes happen to be current. The comparison is a
//     differential, and the kcc build must also carry its provenance line so a
//     silent Go fallback cannot pass it.
//  3. REFUSALS: the shapes this slice still declines must be refused BY NAME,
//     exit non-zero, and leave no image. The out-of-scope PE `push()` failure is
//     pinned here rather than fixed: it is a pre-existing defect, and this gate
//     exists to keep it visible, not to hide it behind a silent success.
//  4. NON-VACUITY: distinct sources must produce distinct images, a rebuild
//     must reproduce the same bytes, the corpus must be non-empty, and the CI
//     step that runs this gate must exist. A gate that never varies cannot
//     detect a driver that ignores its input.
//
// WHAT IS NOT CLAIMED. macOS execution is unproven everywhere (no Intel-Mac
// host and no macOS runner in CI). Linux ELF execution is not available on this
// Windows development host, so on Windows the Linux leg is byte-identity only;
// the Linux CI runner supplies execution evidence for it. PE images are NOT
// byte-identical to Go's -- the two entry bootstraps are separate
// implementations -- so for PE the comparison is stdout, not bytes.

type forinCase struct {
	name    string
	src     string
	wantOut []byte
}

// forinCorpus is the executed corpus. Every entry is a DIFFERENT .kark source;
// the pair (plain, single) is the load-bearing one, because a driver that
// hard-coded the first array's output would fail the second.
var forinCorpus = []forinCase{
	{
		name: "plain",
		src: "func main() {\n" +
			"    let a = [10, 20, 30]\n" +
			"    for x in a {\n" +
			"        print(x)\n" +
			"    }\n" +
			"}\n",
		wantOut: []byte("10\n20\n30\n"),
	},
	{
		name: "single",
		src: "func main() {\n" +
			"    let a = [42]\n" +
			"    for x in a {\n" +
			"        print(x)\n" +
			"    }\n" +
			"}\n",
		wantOut: []byte("42\n"),
	},
	{
		name: "sum",
		src: "func main() {\n" +
			"    let a = [1, 2, 3, 4]\n" +
			"    let s = 0\n" +
			"    for x in a {\n" +
			"        s = s + x\n" +
			"    }\n" +
			"    print(s)\n" +
			"}\n",
		wantOut: []byte("10\n"),
	},
	{
		name: "nested",
		src: "func main() {\n" +
			"    let a = [1, 2]\n" +
			"    let b = [10, 20]\n" +
			"    for x in a {\n" +
			"        for y in b {\n" +
			"            print(x * 100 + y)\n" +
			"        }\n" +
			"    }\n" +
			"}\n",
		wantOut: []byte("110\n120\n210\n220\n"),
	},
	{
		name: "break_continue",
		src: "func main() {\n" +
			"    let a = [1, 2, 3, 4]\n" +
			"    for x in a {\n" +
			"        if x == 2 {\n" +
			"            continue\n" +
			"        }\n" +
			"        if x == 4 {\n" +
			"            break\n" +
			"        }\n" +
			"        print(x)\n" +
			"    }\n" +
			"}\n",
		wantOut: []byte("1\n3\n"),
	},
	{
		name: "index_inside",
		src: "func main() {\n" +
			"    let a = [5, 6, 7]\n" +
			"    let i = 0\n" +
			"    for x in a {\n" +
			"        print(a[i])\n" +
			"        i = i + 1\n" +
			"    }\n" +
			"}\n",
		wantOut: []byte("5\n6\n7\n"),
	},
	{
		name: "cond_body",
		src: "func main() {\n" +
			"    let a = [0, 4, 6, 0]\n" +
			"    for x in a {\n" +
			"        print(x * 2)\n" +
			"    }\n" +
			"}\n",
		wantOut: []byte("0\n8\n12\n0\n"),
	},
}

var (
	forinSetupOnce sync.Once
	forinKCC       string
	forinSetupErr  error
)

// forinEnsurePATH adds the local MinGW bin directory on Windows when gcc is not
// already resolvable. CI has gcc on PATH; this is only for the development host,
// where the toolchain lives under C:\msys64. It is deliberately a PATH change in
// the TEST process so every child (including the kcc bootstrap) sees it.
func forinEnsurePATH(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}
	if _, err := exec.LookPath("gcc"); err == nil {
		return
	}
	for _, dir := range []string{`C:\msys64\ucrt64\bin`, `C:\msys64\mingw64\bin`} {
		if _, err := os.Stat(filepath.Join(dir, "gcc.exe")); err == nil {
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			return
		}
	}
}

// forinSetup builds (or reuses) the self-hosted engine, points the CLI at it,
// and returns the freshly built karkain binary. The engine is cached for the
// whole test binary: building it once per top-level test would spend minutes of
// the ~4 GB host's memory budget on repetition rather than on the gate.
func forinSetup(t *testing.T) string {
	t.Helper()
	forinEnsurePATH(t)
	hasGCC(t)
	forinSetupOnce.Do(func() {
		// phase95KCC builds into a t.TempDir, which is removed when the FIRST
		// top-level test returns. Cache a copy in an OS temp directory that
		// outlives every test in this binary, or later tests would point at a
		// deleted compiler -- an environmental failure that reads like a
		// compiler regression.
		built := phase95KCC(t)
		data, err := os.ReadFile(built)
		if err != nil {
			forinSetupErr = err
			return
		}
		dir, err := os.MkdirTemp("", "phase151a9i-kcc")
		if err != nil {
			forinSetupErr = err
			return
		}
		name := "kcc"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		forinKCC = filepath.Join(dir, name)
		if err := os.WriteFile(forinKCC, data, 0o755); err != nil {
			forinSetupErr = err
			return
		}
		forinSetupErr = os.Setenv("KARKAIN_KCC", forinKCC)
	})
	if forinSetupErr != nil {
		t.Fatalf("selecting the self-hosted compiler: %v", forinSetupErr)
	}
	return phase130Karkain(t)
}

// forinExt is the on-disk extension for a native target string. target is a
// triple-like NAME, not a path, so filepath.Ext would return "".
func forinExt(target string) string {
	switch target {
	case NativeWindowsTarget:
		return ".exe"
	case NativeLinuxTarget:
		return ".elf"
	case NativeMacOSTarget:
		return ".macho"
	default:
		return ".img"
	}
}

// forinHostTarget is the native container this host can execute, if any.
func forinHostTarget() string {
	switch runtime.GOOS {
	case "windows":
		return NativeWindowsTarget
	case "linux":
		return NativeLinuxTarget
	default:
		return ""
	}
}

// forinRunImage validates an image with the ORACLE's parser, writes it to a
// temporary file, executes it under a timeout, and returns EXACT stdout bytes.
// A hang is a failure, never an absent result.
func forinRunImage(t *testing.T, img []byte, target string) []byte {
	t.Helper()
	var ext string
	switch target {
	case NativeWindowsTarget:
		if _, _, err := native.ParsePE(img); err != nil {
			t.Fatalf("kcc image is not a structurally valid PE: %v", err)
		}
		ext = ".exe"
	case NativeLinuxTarget:
		if _, _, err := native.Parse(img); err != nil {
			t.Fatalf("kcc image is not a structurally valid ELF: %v", err)
		}
		ext = ".elf"
	default:
		t.Fatalf("no execution rule for target %q", target)
	}

	path := filepath.Join(t.TempDir(), "prog"+ext)
	if err := os.WriteFile(path, img, 0o755); err != nil {
		t.Fatalf("writing the image: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("the image did not terminate within 20s; a for-in loop that " +
			"never advances is a corpus bug, not a slow test")
	}
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("executing the image: %v", err)
		}
	}
	if code != 0 {
		t.Fatalf("image exited %d; stdout = % x", code, out)
	}
	return out
}

// TestPhase151A9I_ExecutedImagesMatchOracle is layer 1.
//
// Each kcc-produced host image is executed and compared to its exact golden,
// AND to the Go oracle's image built from the same source. The PE is not
// byte-identical between engines (separate entry bootstraps), so stdout is the
// comparison; the byte-identity claim is layer 2's Linux differential.
func TestPhase151A9I_ExecutedImagesMatchOracle(t *testing.T) {
	target := forinHostTarget()
	if target == "" {
		t.Skip("no native execution host for this OS; byte-identity layer still runs")
	}
	karkain := forinSetup(t)

	for _, c := range forinCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			kccRes := nativeBuildWithEngine(t, karkain, c.src, target, "kcc", c.name+"-kcc"+forinExt(target))
			if kccRes.ExitCode != 0 || !kccRes.WroteImage {
				t.Fatalf("kcc produced no image (exit %d):\n%s", kccRes.ExitCode, kccRes.Output)
			}
			if !kccProducedImage(kccRes.Output) {
				t.Fatalf("the kcc build did not carry provenance (%q), so a Go fallback "+
					"cannot be excluded:\n%s", kccNativeProvenance, kccRes.Output)
			}
			got := forinRunImage(t, kccRes.Image, target)
			if !bytes.Equal(got, c.wantOut) {
				t.Fatalf("kcc stdout = %q (% x), want %q (% x)\n  source:\n%s\n"+
					"  EXACT bytes: a missing newline is a real defect",
					got, got, c.wantOut, c.wantOut, c.src)
			}

			goRes := nativeBuildWithEngine(t, karkain, c.src, target, "go", c.name+"-go"+forinExt(target))
			if goRes.ExitCode != 0 || !goRes.WroteImage {
				t.Fatalf("the Go oracle produced no image (exit %d):\n%s", goRes.ExitCode, goRes.Output)
			}
			want := forinRunImage(t, goRes.Image, target)
			if !bytes.Equal(got, want) {
				t.Fatalf("kcc and the Go oracle print different bytes:\n"+
					"  kcc: %q (% x)\n  go : %q (% x)\n  source:\n%s",
					got, got, want, want, c.src)
			}
		})
	}
}

// TestPhase151A9I_LinuxELFIsByteIdenticalToOracle is layer 2.
//
// The expected bytes are produced by calling the oracle at test time. A golden
// would pin a snapshot; this asserts agreement of two implementations instead.
func TestPhase151A9I_LinuxELFIsByteIdenticalToOracle(t *testing.T) {
	karkain := forinSetup(t)

	for _, c := range forinCorpus {
		c := c
		t.Run(c.name, func(t *testing.T) {
			kccRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "kcc", c.name+"-kcc.elf")
			if kccRes.ExitCode != 0 || !kccRes.WroteImage {
				t.Fatalf("kcc produced no ELF (exit %d):\n%s", kccRes.ExitCode, kccRes.Output)
			}
			if !kccProducedImage(kccRes.Output) {
				t.Fatalf("kcc did not carry provenance (%q):\n%s", kccNativeProvenance, kccRes.Output)
			}
			if _, _, err := native.Parse(kccRes.Image); err != nil {
				t.Fatalf("kcc emitted an ELF the ORACLE's Parse rejects: %v", err)
			}

			goRes := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "go", c.name+"-go.elf")
			if goRes.ExitCode != 0 || !goRes.WroteImage {
				t.Fatalf("the Go oracle produced no ELF (exit %d):\n%s", goRes.ExitCode, goRes.Output)
			}
			if !bytes.Equal(kccRes.Image, goRes.Image) {
				t.Fatalf("kcc and Go emitted different ELF images for the same source:\n"+
					"  kcc: %d bytes sha %s\n  go : %d bytes sha %s\n  source:\n%s",
					len(kccRes.Image), kccRes.SHA, len(goRes.Image), goRes.SHA, c.src)
			}
			if runtime.GOOS == "linux" {
				got := forinRunImage(t, kccRes.Image, NativeLinuxTarget)
				if !bytes.Equal(got, c.wantOut) {
					t.Fatalf("Linux execution stdout = %q (% x), want %q (% x)",
						got, got, c.wantOut, c.wantOut)
				}
			}
		})
	}
}

// TestPhase151A9I_ImagesAreDistinctAndDeterministic is layer 4's anti-vacuity
// check. A driver that ignored the AST and emitted one fixed image would pass
// every per-case golden whose value it happened to pick, but cannot produce
// distinct images for distinct inputs.
func TestPhase151A9I_ImagesAreDistinctAndDeterministic(t *testing.T) {
	karkain := forinSetup(t)

	seen := make(map[string]string)
	total := 0
	for _, c := range forinCorpus {
		res := nativeBuildWithEngine(t, karkain, c.src, NativeLinuxTarget, "kcc", c.name+"-distinct.elf")
		if res.ExitCode != 0 || !res.WroteImage {
			t.Fatalf("kcc produced no image for %s: %s", c.name, res.Output)
		}
		if prev, ok := seen[res.SHA]; ok {
			t.Errorf("%s and %s produced the SAME image (sha %s); the driver must "+
				"read the source, not a hard-coded program", prev, c.name, res.SHA)
		}
		seen[res.SHA] = c.name
		total += len(res.Image)
	}
	if len(seen) < 5 {
		t.Fatalf("only %d distinct images from %d distinct sources; the corpus is vacuous",
			len(seen), len(forinCorpus))
	}
	if total < 2000 {
		t.Fatalf("the %d images total only %d bytes; a corpus that stopped carrying "+
			"real code must not report success", len(forinCorpus), total)
	}

	first := nativeBuildWithEngine(t, karkain, forinCorpus[0].src, NativeLinuxTarget, "kcc", "repeat-a.elf")
	second := nativeBuildWithEngine(t, karkain, forinCorpus[0].src, NativeLinuxTarget, "kcc", "repeat-b.elf")
	if first.SHA != second.SHA {
		t.Errorf("the same source built twice gave different images: %s vs %s", first.SHA, second.SHA)
	}
}

// forinRefusal is one boundary this slice deliberately keeps.
type forinRefusal struct {
	name   string
	src    string
	target string
	want   string
}

var forinRefusals = []forinRefusal{
	{
		// The oracle refuses a literal iterator too: an iterator must be an
		// array VARIABLE whose header lives in a frame slot.
		name:   "literal_iterator",
		src:    "func main() {\n    for x in [1, 2] { print(x) }\n}\n",
		target: NativeLinuxTarget,
		want:   "for-in iterates a variable on the native target",
	},
	{
		// len() is lowered in a direct print/return position, but not inside a
		// loop body in this slice. The refusal must name it, not emit a wrong
		// header load.
		name:   "len_inside_forin",
		src:    "func main() {\n    let a = [1, 2]\n    for x in a { print(len(a)) }\n}\n",
		target: NativeLinuxTarget,
		want:   "`len()` inside a called function is not lowered by the native driver",
	},
	{
		// PRE-EXISTING, OUT OF SCOPE, PINNED. The PE linker's patch guard rejects
		// a push() image whose rodata reference does not fit the section body.
		// Phase 2.2 does not fix it; it makes sure the failure stays loud rather
		// than being mistaken for a for-in regression or silently succeeding.
		name:   "push_pe_patch_guard",
		src:    "func main() { let a = [1, 2] let b = push(a, 3) print(len(b)) }\n",
		target: NativeWindowsTarget,
		want:   "native pe: rodata patch",
	},
}

// TestPhase151A9I_RefusalsAreByName is layer 3.
func TestPhase151A9I_RefusalsAreByName(t *testing.T) {
	karkain := forinSetup(t)

	for _, c := range forinRefusals {
		c := c
		t.Run(c.name, func(t *testing.T) {
			res := nativeBuildWithEngine(t, karkain, c.src, c.target, "kcc", c.name+".img")
			if res.ExitCode == 0 {
				t.Fatalf("kcc ACCEPTED a program it cannot lower (exit 0):\n%s", res.Output)
			}
			if !strings.Contains(res.Output, c.want) {
				t.Errorf("the refusal does not name %q:\n%s", c.want, res.Output)
			}
			if res.WroteImage {
				t.Errorf("a refused program still wrote an image (%d bytes)", len(res.Image))
			}
		})
	}
}

// TestPhase151A9I_RegAllocPathAgrees checks the opt-in Phase-150C register path
// against the qualified stack-based path. The register path is not the default;
// agreement here means enabling it does not change what a for-in program prints.
func TestPhase151A9I_RegAllocPathAgrees(t *testing.T) {
	target := forinHostTarget()
	if target == "" {
		t.Skip("no native execution host for this OS")
	}
	karkain := forinSetup(t)

	for _, c := range forinCorpus[:3] {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("KARKAIN_NATIVE_REGALLOC", "0")
			stack := nativeBuildWithEngine(t, karkain, c.src, target, "kcc", c.name+"-stack.img")
			if stack.ExitCode != 0 || !stack.WroteImage {
				t.Fatalf("stack path produced no image: %s", stack.Output)
			}
			gotStack := forinRunImage(t, stack.Image, target)

			t.Setenv("KARKAIN_NATIVE_REGALLOC", "1")
			regs := nativeBuildWithEngine(t, karkain, c.src, target, "kcc", c.name+"-regs.img")
			if regs.ExitCode != 0 || !regs.WroteImage {
				t.Fatalf("register path produced no image: %s", regs.Output)
			}
			gotRegs := forinRunImage(t, regs.Image, target)

			if !bytes.Equal(gotStack, gotRegs) || !bytes.Equal(gotStack, c.wantOut) {
				t.Fatalf("the two allocation paths disagree, or one moved off the golden:\n"+
					"  stack: % x\n  regs : % x\n  want : % x", gotStack, gotRegs, c.wantOut)
			}
		})
	}
}

// TestPhase151A9I_CIWiring is the companion rule from AGENTS.md: a gate is a
// test file PLUS a ci.yml step. A gate that only runs locally is not a gate.
func TestPhase151A9I_CIWiring(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	ci := string(data)
	for _, want := range []string{
		"Run 151A Step 9I for-in gate",
		"go test ./pkg/cli/ -run 'TestPhase151A9I_' -count=1 -timeout 30m",
	} {
		if !strings.Contains(ci, want) {
			t.Errorf("ci.yml does not contain %q; this gate must have its own step", want)
		}
	}
}
