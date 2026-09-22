package cli

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"karkain/pkg/target"
)

// --- helpers ---------------------------------------------------------------

// exitCodeOf extracts the process exit code from a runBin error, or -1 for a
// non-exit failure (only possible for build/run harness problems).
func exitCodeOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	if ok := asExitError(err, &ee); ok {
		return ee.ExitCode()
	}
	return -1
}

func asExitError(err error, out **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*out = ee
		return true
	}
	return false
}

// norm canonicalizes platform line endings so Windows artifacts (which inherit
// the MSVC runtime's CRLF console translation) compare against \n goldens.
func norm(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// peMachine parses a PE image and returns the IMAGE_FILE_HEADER.Machine value,
// failing the test if the file is not a well-formed PE.
func peMachine(t *testing.T, path string) uint16 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if len(data) < 0x40 || string(data[0:2]) != "MZ" {
		t.Fatalf("artifact %s is not an MZ image (len=%d)", path, len(data))
	}
	eLfanew := binary.LittleEndian.Uint32(data[0x3C:0x40])
	if int(eLfanew)+24 > len(data) {
		t.Fatalf("artifact %s: truncated PE header (e_lfanew=%d len=%d)", path, eLfanew, len(data))
	}
	if string(data[eLfanew:eLfanew+4]) != "PE\x00\x00" {
		t.Fatalf("artifact %s: missing PE signature at %d", path, eLfanew)
	}
	return binary.LittleEndian.Uint16(data[eLfanew+4 : eLfanew+6])
}

// copyExampleToTemp copies an example source into an isolated temp dir so builds
// never drop generated .c artifacts into the repository tree.
func copyExampleToTemp(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join(repoRoot(t), "examples", "cross_compile", name+".kark")
	dir := t.TempDir()
	dst := filepath.Join(dir, name+".kark")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read example %s: %v", name, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("copy example %s: %v", name, err)
	}
	return dst
}

// --- validation: normalization and target model -----------------------------

func TestPhase111_NormalizeTarget_Canonical(t *testing.T) {
	cases := map[string]string{
		"native":                    "native",
		"c23":                       "c23",
		"native-link":               "native-link",
		"wasm32-wasi":               "wasm32-wasi",
		"x86_64-windows":            "x86_64-windows",
		"x86_64-pc-windows-msvc":    "x86_64-windows",
		"x86_64-unknown-linux":      "x86_64-linux",
		"x86_64-linux-gnu":          "x86_64-linux",
		"x86_64-unknown-linux-gnu":  "x86_64-linux",
		"aarch64-linux":             "aarch64-linux",
		"aarch64-unknown-linux-gnu": "aarch64-linux",
	}
	inputs := make([]string, 0, len(cases))
	for in := range cases {
		inputs = append(inputs, in)
	}
	sort.Strings(inputs)
	for _, in := range inputs {
		got, err := NormalizeTarget(in)
		if err != nil {
			t.Errorf("NormalizeTarget(%q): unexpected error: %v", in, err)
			continue
		}
		if got != cases[in] {
			t.Errorf("NormalizeTarget(%q) = %q, want %q", in, got, cases[in])
		}
	}
}

func TestPhase111_SelectedTarget_Machine(t *testing.T) {
	win, ok := SelectedTarget("x86_64-windows")
	if !ok {
		t.Fatal("x86_64-windows should be a concrete triple")
	}
	if got := win.SameMachine(target.Host()); !got {
		t.Errorf("x86_64-windows on host %s should share the machine", target.Host())
	}
	lnx, ok := SelectedTarget("x86_64-linux")
	if !ok {
		t.Fatal("x86_64-linux should be a concrete triple")
	}
	if got := lnx.SameMachine(target.Host()); got && target.Host().OS != target.OSLinux {
		t.Errorf("x86_64-linux incorrectly shares machine with host %s", target.Host())
	}
	arm, ok := SelectedTarget("aarch64-linux")
	if !ok {
		t.Fatal("aarch64-linux should be a concrete triple")
	}
	if got := arm.SameMachine(target.Host()); got {
		t.Errorf("aarch64-linux incorrectly shares machine with host %s", target.Host())
	}
	if _, ok := SelectedTarget("native"); ok {
		t.Error("native must not resolve to a concrete triple")
	}
	// wasm32-wasi IS a concrete triple (arch=wasm32, os=wasi) in the target
	// model; the CLI still routes it to the dedicated WASM backend before the
	// cross-run check, so the parse result here just needs to be well-formed.
	w, ok := SelectedTarget("wasm32-wasi")
	if !ok || w.Arch != target.ArchWasm32 || w.OS != target.OSWasi {
		t.Errorf("wasm32-wasi should parse as a wasm32/wasi triple, got %+v", w)
	}
}

func TestPhase111_ValidateTarget_Rejections(t *testing.T) {
	bad := []string{
		"bogus",               // unknown architecture family nor vendor
		"x86_64",              // 1 component — malformed
		"native-X",            // unknown alias, parses as arch=native → unknown arch
		"s390x-linux",         // unsupported architecture
		"riscv64-windows",     // Phase 139: riscv64 parses but is Linux-only
		"x86_64-openbsd",      // unsupported OS
		"x86_64-linux-musl",   // musl ABI recognized but unsupported
		"aarch64-linux-musl",  // musl ABI recognized but unsupported
		"x86_64-linux-msvc",   // msvc env is windows-only
		"x86_64-macos-gnu",    // Phase 139: macOS builds use clang, never gnu
		"x86_64-linux-macabi", // unknown env
		"wasm32-linux",        // wasm32 is only supported with wasi
		"wasm32-macos",        // Phase 139: macos is x86_64/aarch64-only
		"x86_64-wasi",         // wasi only pairs with wasm32
	}
	for _, value := range bad {
		if err := ValidateTarget(value); err == nil {
			t.Errorf("ValidateTarget(%q) unexpectedly accepted", value)
		}
	}
}

func TestPhase111_TargetCommand_HostAndSupported(t *testing.T) {
	bin := buildKarkain(t)
	out, err := runBin(t, bin, t.TempDir(), "target")
	if err != nil {
		t.Fatalf("karkain target: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Host:") {
		t.Errorf("karkain target should report the host triple, got:\n%s", out)
	}
	for _, tt := range target.SupportedTargets() {
		if !strings.Contains(out, tt.String()) {
			t.Errorf("karkain target output lacks supported triple %q:\n%s", tt.String(), out)
		}
	}
	for _, frag := range []string{"x86_64", "aarch64", "wasm32", "64-bit little-endian", "glibc", "Default: native"} {
		if !strings.Contains(out, frag) {
			t.Errorf("karkain target output lacks %q:\n%s", frag, out)
		}
	}
	// Legacy aliases must keep appearing (the pre-Phase-111 gate).
	for _, frag := range []string{"native", "c23", "wasm32-wasi"} {
		if !strings.Contains(out, frag) {
			t.Errorf("karkain target output lacks legacy alias %q:\n%s", frag, out)
		}
	}
}

// --- host-executable cross build (x86_64-windows) ----------------------------

func TestPhase111_CrossBuildWindows_HostExecutable(t *testing.T) {
	bin := buildKarkain(t)
	hello := copyExampleToTemp(t, "hello")
	outDir := t.TempDir()
	out := filepath.Join(outDir, "hello.exe")

	runOut, err := runBin(t, bin, t.TempDir(), "build", hello,
		"--engine", "go", "--target", "x86_64-windows", "-o", out)
	if err != nil {
		t.Fatalf("cross build x86_64-windows failed: %v\n%s", err, runOut)
	}
	if !strings.Contains(runOut, "Build successful") {
		t.Fatalf("build did not succeed:\n%s", runOut)
	}
	if machine := peMachine(t, out); machine != 0x8664 {
		t.Fatalf("built artifact machine = 0x%04x, want 0x8664 (x86-64)", machine)
	}
	execOut, err := exec.Command(out).CombinedOutput()
	if err != nil {
		t.Fatalf("run built artifact: %v\n%s", err, execOut)
	}
	if got, want := norm(string(execOut)), "Hello from Karkain cross compilation\n"; got != want {
		t.Fatalf("artifact stdout = %q, want %q", got, want)
	}
}

// TestPhase111_CrossBuildDefaultEngine proves the kcc-default engine routes
// triple targets to the Go front end (no kcc involvement, no silent host
// fallback) by running the CLI without any --engine or KARKAIN_ENGINE pinning.
func TestPhase111_CrossBuildDefaultEngine(t *testing.T) {
	bin := buildKarkain(t)
	hello := copyExampleToTemp(t, "hello")
	out := filepath.Join(t.TempDir(), "hello.exe")

	cmd := exec.Command(bin, "build", hello, "--target", "x86_64-windows", "-o", out)
	cmd.Dir = t.TempDir()
	env := []string{}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "KARKAIN_ENGINE=") {
			env = append(env, e)
		}
	}
	cmd.Env = env
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("default-engine cross build failed: %v\n%s", err, raw)
	}
	if machine := peMachine(t, out); machine != 0x8664 {
		t.Fatalf("default-engine artifact machine = 0x%04x, want 0x8664", machine)
	}
	execOut, err := exec.Command(out).CombinedOutput()
	if err != nil {
		t.Fatalf("run default-engine artifact: %v\n%s", err, execOut)
	}
	if got, want := norm(string(execOut)), "Hello from Karkain cross compilation\n"; got != want {
		t.Fatalf("default-engine stdout = %q, want %q", got, want)
	}
}

func TestPhase111_CrossBuildWindows_NormalizedTriple(t *testing.T) {
	bin := buildKarkain(t)
	hello := copyExampleToTemp(t, "hello")
	out := filepath.Join(t.TempDir(), "hello.exe")

	runOut, err := runBin(t, bin, t.TempDir(), "build", hello,
		"--engine", "go", "--target", "x86_64-pc-windows-msvc", "-o", out)
	if err != nil {
		t.Fatalf("cross build x86_64-pc-windows-msvc failed: %v\n%s", err, runOut)
	}
	if machine := peMachine(t, out); machine != 0x8664 {
		t.Fatalf("normalized artifact machine = 0x%04x, want 0x8664", machine)
	}
	execOut, err := exec.Command(out).CombinedOutput()
	if err != nil {
		t.Fatalf("run normalized artifact: %v\n%s", err, execOut)
	}
	if got, want := norm(string(execOut)), "Hello from Karkain cross compilation\n"; got != want {
		t.Fatalf("normalized stdout = %q, want %q", got, want)
	}
}

func TestPhase111_CrossRun_HostAndForeign(t *testing.T) {
	bin := buildKarkain(t)
	hello := copyExampleToTemp(t, "hello")

	// Host machine may run: x86_64-windows on an x86_64 windows host.
	hostWin, _ := target.Parse("x86_64-windows")
	runOut, err := runBin(t, bin, t.TempDir(), "run", hello, "--engine", "go",
		"--target", "x86_64-windows")
	if err != nil {
		// Foreign machine only on exotic hosts — x86_64-windows is either host
		// or not; if it is not the host this assertion is replaced below.
		if !target.Host().SameMachine(hostWin) {
			t.Logf("host %s cannot run x86_64-windows locally (expected): %v", target.Host(), err)
		} else {
			t.Fatalf("host run of x86_64-windows failed: %v\n%s", err, runOut)
		}
	} else if !strings.Contains(runOut, "Hello from Karkain cross compilation") {
		t.Fatalf("host run stdout wrong:\n%s", runOut)
	}

	// A foreign-architecture/OS binary can never run on this host.
	foreign, err := runBin(t, bin, t.TempDir(), "run", hello, "--engine", "go",
		"--target", "x86_64-linux")
	if err == nil {
		t.Fatalf("run --target x86_64-linux unexpectedly succeeded:\n%s", foreign)
	}
	if code := exitCodeOf(t, err); code != ExitEnv {
		t.Errorf("foreign run exit = %d, want %d\n%s", code, ExitEnv, foreign)
	}
	if !strings.Contains(foreign, "cannot run a x86_64-linux binary on the host") ||
		!strings.Contains(foreign, "cross-run") {
		t.Errorf("foreign run diagnostic missing expected text:\n%s", foreign)
	}
}

// --- missing cross-linker (never a silent host fallback) ----------------------

func TestPhase111_CrossBuildLinux_MissingToolchain(t *testing.T) {
	bin := buildKarkain(t)
	hello := copyExampleToTemp(t, "hello")

	for _, triple := range []string{"x86_64-linux", "aarch64-linux"} {
		outPath := filepath.Join(t.TempDir(), "out")
		out, err := runBin(t, bin, t.TempDir(), "build", hello,
			"--engine", "go", "--target", triple, "-o", outPath)
		if err == nil {
			t.Errorf("build --target %s unexpectedly succeeded:\n%s", triple, out)
			continue
		}
		if code := exitCodeOf(t, err); code != ExitEnv {
			t.Errorf("build --target %s exit = %d, want %d", triple, code, ExitEnv)
		}
		for _, frag := range []string{
			"no cross-linker available for target '" + triple + "'",
			"on host '" + target.Host().String() + "'",
			"host toolchain can neither link nor execute",
			"cross-toolchain for " + triple,
		} {
			if !strings.Contains(out, frag) {
				t.Errorf("build --target %s output lacks %q:\n%s", triple, frag, out)
			}
		}
		if _, serr := os.Stat(outPath); !os.IsNotExist(serr) {
			t.Errorf("build --target %s must not create an artifact on failure (%s present)", triple, outPath)
		}
	}
}

// --- determinism and generated-C self-description -----------------------------

func TestPhase111_DeterministicC(t *testing.T) {
	bin := buildKarkain(t)

	dirs := []string{t.TempDir(), t.TempDir()}
	for _, d := range dirs {
		data, err := os.ReadFile(filepath.Join(repoRoot(t), "examples", "cross_compile", "functions.kark"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "functions.kark"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := runBin(t, bin, t.TempDir(), "build", filepath.Join(d, "functions.kark"),
			"--engine", "go", "--target", "x86_64-windows",
			"-o", filepath.Join(d, "functions.exe"))
		if err != nil {
			t.Fatalf("build in %s: %v\n%s", d, err, out)
		}
	}

	a, err := os.ReadFile(filepath.Join(dirs[0], "functions.c"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dirs[1], "functions.c"))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Error("generated C is not byte-identical across identical builds")
	}

	// Both artifacts must behave identically regardless of PE metadata drift.
	want := "377\n50\n1\n0\n"
	for i, d := range dirs {
		execOut, err := exec.Command(filepath.Join(d, "functions.exe")).CombinedOutput()
		if err != nil {
			t.Fatalf("run artifact %d: %v\n%s", i, err, execOut)
		}
		if got := norm(string(execOut)); got != want {
			t.Errorf("artifact %d stdout = %q, want %q", i, got, want)
		}
	}
}

func TestPhase111_TargetMarkersInC(t *testing.T) {
	bin := buildKarkain(t)
	hello := copyExampleToTemp(t, "hello")
	out := filepath.Join(t.TempDir(), "hello.exe")

	if _, err := runBin(t, bin, t.TempDir(), "build", hello,
		"--engine", "go", "--target", "x86_64-windows", "-o", out); err != nil {
		t.Fatal(err)
	}
	c, err := os.ReadFile(filepath.Join(filepath.Dir(hello), "hello.c"))
	if err != nil {
		t.Fatal(err)
	}
	for _, frag := range []string{
		"/* karkain-target: x86_64-windows */",
		"#define KARKAIN_TARGET_ARCH_X86_64 1",
		"#define KARKAIN_TARGET_OS_WINDOWS 1",
		"#define KARKAIN_TARGET_X86_64 1",
		"#define KARKAIN_TARGET_WINDOWS 1",
	} {
		if !strings.Contains(string(c), frag) {
			t.Errorf("generated C lacks %q", frag)
		}
	}
}

// --- example corpus (host-machine cross builds) --------------------------------

func TestPhase111_ExampleCorpus_HostMachine(t *testing.T) {
	bin := buildKarkain(t)
	wants := map[string]string{
		"hello":       "Hello from Karkain cross compilation\n",
		"functions":   "377\n50\n1\n0\n",
		"collections": "31\ncross-collections-8\n80\n",
		"platform":    "platform-ok 78112\n",
	}
	for name, want := range wants {
		src := copyExampleToTemp(t, name)
		out := filepath.Join(t.TempDir(), name+".exe")
		bo, err := runBin(t, bin, t.TempDir(), "build", src,
			"--engine", "go", "--target", "x86_64-windows", "-o", out)
		if err != nil {
			t.Errorf("%s: build failed: %v\n%s", name, err, bo)
			continue
		}
		if machine := peMachine(t, out); machine != 0x8664 {
			t.Errorf("%s: machine = 0x%04x, want 0x8664", name, machine)
			continue
		}
		execOut, err := exec.Command(out).CombinedOutput()
		if err != nil {
			t.Errorf("%s: run failed: %v\n%s", name, err, execOut)
			continue
		}
		if got := norm(string(execOut)); got != want {
			t.Errorf("%s: stdout = %q, want %q", name, got, want)
		}
	}
}

// --- legacy targets and incremental coexistence ---------------------------------

func TestPhase111_LegacyAliases_StillAccepted(t *testing.T) {
	for _, value := range []string{"native", "c23", "native-link", "wasm32-wasi"} {
		if err := ValidateTarget(value); err != nil {
			t.Errorf("legacy alias %q rejected: %v", value, err)
		}
		if got, err := NormalizeTarget(value); err != nil || got != value {
			t.Errorf("NormalizeTarget(%q) = %q, err=%v", value, got, err)
		}
	}
}

func TestPhase111_Incremental_CrossTargetCache(t *testing.T) {
	bin := buildKarkain(t)
	src := copyExampleToTemp(t, "hello")
	exe := filepath.Join(t.TempDir(), "hello.exe")
	cache := filepath.Join(t.TempDir(), ".karkain-cache-incremental")

	first, err := runBin(t, bin, t.TempDir(), "build", "--incremental", "--incremental-cache", cache,
		src, "--engine", "go", "--target", "x86_64-windows",
		"-o", exe)
	if err != nil {
		t.Fatalf("incremental cross build: %v\n%s", err, first)
	}
	if machine := peMachine(t, exe); machine != 0x8664 {
		t.Fatalf("incremental artifact machine = 0x%04x, want 0x8664", machine)
	}
	execOut, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatalf("run incremental artifact: %v\n%s", err, execOut)
	}
	if want := "Hello from Karkain cross compilation\n"; norm(string(execOut)) != want {
		t.Fatalf("incremental stdout = %q, want %q", norm(string(execOut)), want)
	}
}
