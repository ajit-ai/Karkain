package cli

// Increment 152-A — the no-C closure gate.
//
// 152-A is deliberately narrow (see docs/audit/PHASE-152-BASELINE.md): it
// closes one latent soundness defect (string concatenation inside a loop) and
// draws one capability line (the socket builtins). It implements NO new stdlib
// function, and Increment 152 is NOT complete — `sha256_hex`, `trim`, `split`
// and the base64/hex codecs remain open for 152-B.
//
// What this gate establishes, in order:
//
//  1. Toolchain proof. gcc, clang and cl are each PROVEN unusable, not assumed
//     absent. Resolution must land on a failing stub and running that stub must
//     fail. If any of the three cannot be proven, the gate FAILS — it never
//     skips, because a silent skip is indistinguishable from a pass.
//  2. A native program builds and runs with no C compiler reachable.
//  3. The socket capability boundary rejects by name, asserted on the
//     diagnostic TEXT (a non-zero exit alone cannot distinguish "this target
//     has no sockets" from "this program is invalid").
//  4. `--target c23` still builds and links — the C path is a regression guard
//     here and is never modified by 152-A.
//  5. An unknown target still fails deterministically, never a silent fallback.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// noCTools are the compilers whose absence increment 152 is about. Each must be
// proven unusable before any no-C claim is made.
var noCTools = []string{"gcc", "clang", "cl"}

// shadowCompilersWithStubs prepends a directory holding a FAILING stub for each
// compiler in noCTools, so any PATH-based invocation resolves to the stub.
//
// Shadowing, not directory removal: Phase 143 recorded that `go` can share a
// directory with `gcc` on some CI images, so removing the directory would
// amputate unrelated tools. Shadowing is layout-agnostic and leaves every other
// tool resolvable.
//
// Windows resolves commands through PATHEXT, so a bare shell script would be
// skipped and the REAL compiler found — silently invalidating the whole proof.
// That is why both a script form and a .bat/.cmd form are written for each
// tool, exactly as phase143_seed_test.go does for `go`.
func shadowCompilersWithStubs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sh := "#!/bin/sh\necho 'C compiler disabled for the increment-152 no-C proof' >&2\nexit 1\n"
	for _, tool := range noCTools {
		if err := os.WriteFile(filepath.Join(dir, tool), []byte(sh), 0o755); err != nil {
			t.Fatalf("write %s stub: %v", tool, err)
		}
		bat := "@echo off\r\necho C compiler disabled for the increment-152 no-C proof 1>&2\r\nexit /b 1\r\n"
		for _, ext := range []string{".bat", ".cmd"} {
			if err := os.WriteFile(filepath.Join(dir, tool+ext), []byte(bat), 0o644); err != nil {
				t.Fatalf("write %s%s stub: %v", tool, ext, err)
			}
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

// TestPhase152A_NoCToolchainProvenAbsent is the load-bearing assertion of the
// whole gate. For each compiler it asserts that resolution finds the stub (and
// not the real toolchain) and that the stub FAILS when run.
//
// Any failure here fails the test. There is deliberately no t.Skip: a gate that
// skips when it cannot prove the precondition is not evidence, and reporting
// "skipped" as if it were "green" is precisely the dishonesty this gate exists
// to prevent.
func TestPhase152A_NoCToolchainProvenAbsent(t *testing.T) {
	stubDir := shadowCompilersWithStubs(t)

	for _, tool := range noCTools {
		t.Run(tool, func(t *testing.T) {
			resolved, err := exec.LookPath(tool)
			if err != nil {
				t.Fatalf("%s does not resolve at all, so absence cannot be proven: %v", tool, err)
			}
			// Resolution MUST land inside the stub directory. If it lands
			// elsewhere the real compiler is still reachable and every
			// "no-C" claim below would be false.
			if filepath.Dir(resolved) != stubDir {
				t.Fatalf("%s resolved to %s, outside the stub dir %s: the real toolchain is still reachable, so the no-C proof is void",
					tool, resolved, stubDir)
			}
			out, runErr := exec.Command(resolved, "--version").CombinedOutput()
			if runErr == nil {
				t.Fatalf("%s (%s) ran successfully; it was expected to fail: %q", tool, resolved, out)
			}
			if !strings.Contains(string(out), "disabled for the increment-152 no-C proof") {
				t.Errorf("%s failed but not with the stub's diagnostic, so the stub may not be what ran: %q", tool, out)
			}
		})
	}
}

// hostNativeTarget is the native target this host can both build AND execute,
// so the gate can prove a real image ran rather than merely being emitted.
func hostNativeTarget() string {
	if runtime.GOARCH != "amd64" {
		return ""
	}
	switch runtime.GOOS {
	case "windows":
		return "native-x86_64-windows"
	case "darwin":
		return "native-x86_64-macos"
	case "linux":
		return "native-x86_64-linux"
	}
	return ""
}

// TestPhase152A_NativeRunsWithoutCCompiler proves the increment-152 premise on
// this host: a native program builds and RUNS while no C compiler is reachable.
//
// 152-A ships no new stdlib support, so the program exercises the surface that
// already worked (increments 148/150) — that is precisely the point: the
// toolchain chain is already C-free and had to be PROVEN, not assumed.
func TestPhase152A_NativeRunsWithoutCCompiler(t *testing.T) {
	target := hostNativeTarget()
	if target == "" {
		t.Fatalf("no executable native target on %s/%s; the no-C proof cannot run here", runtime.GOOS, runtime.GOARCH)
	}
	shadowCompilersWithStubs(t)
	karkain := phase130Karkain(t)

	dir := t.TempDir()
	src := filepath.Join(dir, "hello.kark")
	// print of a string and of an int, plus arithmetic and a loop: the
	// increment-148/150 surface, with no stdlib dependency at all.
	prog := "func main() {\n\tprint(\"karkain\")\n\tprint(40 + 2)\n\tlet i = 0\n\tlet s = 0\n\twhile (i < 5) {\n\t\ti = i + 1\n\t\ts = s + i\n\t}\n\tprint(s)\n}\n"
	if err := os.WriteFile(src, []byte(prog), 0o644); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(dir, "hello.out")
	build := exec.Command(karkain, "build", src, "--engine", "go", "--target", target, "-o", img)
	build.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("native build must succeed with no C compiler on PATH:\n%s", out)
	}
	st, err := os.Stat(img)
	if err != nil || st.Size() == 0 {
		t.Fatalf("native image missing or empty: %v", err)
	}
	if !nativeHostCanRun(NativeTargetOS(target)) {
		t.Skipf("built %s but this host cannot execute it; emission is proven, execution is not", target)
	}
	run := exec.Command(karkain, "run", src, "--engine", "go", "--target", target)
	run.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("native run must succeed with no C compiler on PATH:\n%s", out)
	}
	got := string(out)
	for _, want := range []string{"karkain", "42", "15"} {
		if !strings.Contains(got, want) {
			t.Errorf("native output missing %q; got:\n%s", want, got)
		}
	}
}

// TestPhase152A_SocketCapabilityBoundaryRejected proves the 152-A capability
// line end to end through the CLI, asserting the diagnostic TEXT.
//
// net_connect is called DIRECTLY rather than through the std.net wrapper, and
// the reason was measured rather than assumed. The boundary is checked in
// retKindVisit, which runs during KIND INFERENCE -- before emission reaches the
// map literal that net_endpoint returns -- so the socket refusal is the first
// blocker a std.net program meets. Measured on the corpus example, the first
// diagnostic is `net_close`. Calling the builtin directly still keeps this
// subtest independent of which limitation happens to fire first, and it needs
// no `import std.net` because the socket primitives are registered builtins.
func TestPhase152A_SocketCapabilityBoundaryRejected(t *testing.T) {
	target := hostNativeTarget()
	if target == "" {
		t.Fatalf("no buildable native target on %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	karkain := phase130Karkain(t)

	t.Run("direct_builtin_reaches_boundary", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "netcap.kark")
		prog := "func main() {\n\tlet c = net_connect(\"127.0.0.1\", 9)\n\tprint(c)\n}\n"
		if err := os.WriteFile(src, []byte(prog), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(karkain, "build", src, "--engine", "go", "--target", target).CombinedOutput()
		if err == nil {
			t.Fatalf("a socket program built on the native target, but 152-A defines sockets as unsupported:\n%s", out)
		}
		msg := string(out)
		// Assert the reason, not just the exit status.
		if !strings.Contains(msg, "net_connect") {
			t.Errorf("rejection does not name the missing socket primitive:\n%s", msg)
		}
		if !strings.Contains(msg, "socket") {
			t.Errorf("rejection does not name the socket capability boundary:\n%s", msg)
		}
		if strings.Contains(msg, "undefined function") {
			t.Errorf("rejection degraded to 'undefined function', which a caller cannot distinguish from an invalid program:\n%s", msg)
		}
	})

	// The real std.net program must also not build on the native target. This
	// asserts the OUTCOME and records WHICH limitation stops it first, so the
	// map-return gap is visible rather than mistaken for the socket boundary.
	t.Run("corpus_net_program_does_not_build", func(t *testing.T) {
		src := filepath.Join(repoRoot(t), "examples", "04-networking", "02_tcp_roundtrip.kark")
		out, err := exec.Command(karkain, "build", src, "--engine", "go", "--target", target).CombinedOutput()
		if err == nil {
			t.Fatalf("the std.net corpus example built on the native target:\n%s", out)
		}
		t.Logf("first native blocker for the std.net example (expected, not the socket boundary): %s", strings.TrimSpace(string(out)))
	})

	// And the same program must STILL work on the C23 path, which 152-A does
	// not touch. That is what makes the boundary a native-target capability
	// line rather than a language restriction.
	t.Run("c23_path_unaffected", func(t *testing.T) {
		if _, err := exec.LookPath("gcc"); err != nil {
			t.Skipf("no C compiler to build the C23 path: %v", err)
		}
		src := filepath.Join(repoRoot(t), "examples", "04-networking", "02_tcp_roundtrip.kark")
		out := exec.Command(karkain, "build", src, "--engine", "go", "--target", "c23")
		out.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
		if msg, err := out.CombinedOutput(); err != nil {
			t.Fatalf("the C23 path must keep the full std.net surface; 152-A must not touch it:\n%s", msg)
		}
	})
}

// TestPhase152A_C23TargetStillWorks is the C-path regression guard. 152-A must
// not remove or redefine --target c23, so this builds and links through it. The
// C23 implementation itself is never modified by this increment.
func TestPhase152A_C23TargetStillWorks(t *testing.T) {
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Fatalf("this guard needs a real C compiler to link through; gcc is absent: %v", err)
	}
	karkain := phase130Karkain(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "c23.kark")
	if err := os.WriteFile(src, []byte("func main() {\n\tprint(\"c23\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := exec.Command(karkain, "build", src, "--engine", "go", "--target", "c23")
	out.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	if msg, err := out.CombinedOutput(); err != nil {
		t.Fatalf("--target c23 must still build and link; 152-A must not touch the C path:\n%s", msg)
	}
	// The emitted C is the C path's own output; assert it is really there so
	// "it built" cannot be satisfied by silently routing to another target.
	cPath := strings.TrimSuffix(src, ".kark") + ".c"
	b, err := os.ReadFile(cPath)
	if err != nil {
		t.Fatalf("expected generated C beside the source: %v", err)
	}
	if !strings.Contains(string(b), "karkain") {
		t.Errorf("generated C does not look like a C23 emission: %.200s", b)
	}
}

// TestPhase152A_UnknownTargetStillDeterministic pins that 152-A introduced no
// silent host fallback: an unknown --target must still fail with a toolchain
// diagnostic rather than quietly building for the host.
func TestPhase152A_UnknownTargetStillDeterministic(t *testing.T) {
	karkain := phase130Karkain(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "unk.kark")
	if err := os.WriteFile(src, []byte("func main() {\n\tprint(1)\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(karkain, "build", src, "--engine", "go", "--target", "generative-ai-9000").CombinedOutput()
	if err == nil {
		t.Fatalf("an unknown target built successfully (silent host fallback):\n%s", out)
	}
	if !strings.Contains(string(out), "generative-ai-9000") {
		t.Errorf("the failure does not name the offending target:\n%s", out)
	}
}
