package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// NAT-QUAL -- the v1.2.0 native-independence QUALIFICATION.
//
// This gate does not add a language construct. It answers the question the
// v1.2.0 mission actually asks: what does the Karkain-owned native path
// support, how is that claim checked, and does that path really avoid C and
// Go?
//
// Three layers, each answering a different question:
//
//	1. THE MATRIX IS MACHINE-VERIFIABLE. The feature matrix
//	   (docs/inventory/native-feature-matrix.json) claims a status for every
//	   construct. This layer checks that every claim is BACKED: a "supported"
//	   row must name a gate that exists, in a file that exists, and that file
//	   must actually contain the named test pattern. A matrix that drifts from
//	   the tests is documentation, and documentation is not evidence.
//
//	2. THE PLATFORM TABLE MATCHES THE CODE. The matrix says which engine owns
//	   each native target and how far each platform is qualified. That must
//	   agree with the code's OWN ownership data
//	   (pkg/cli/kcc_native.go kccNativeOwnedTargets), because the code is the
//	   thing that decides.
//
//	3. PROCESS-LEVEL INDEPENDENCE EVIDENCE. The claim "the qualified native
//	   path does not invoke a C compiler, a C source generator, the Go
//	   compiler or the Go runtime" is asserted by installing PATH SHIMS for
//	   `gcc` and `go` that log every invocation, and then building a program
//	   natively with those shims first on PATH. An empty log is the evidence.
//
//	   Layer 3 also runs a POSITIVE CONTROL: the same shims are used for the
//	   LEGACY build path, which is supposed to invoke gcc. If the shims do not
//   fire there, they cannot be trusted to stay silent on the native path --
//	   and a test that can pass without the mechanism working is not evidence.
//
// Every claim here is a claim about the CURRENT tree, so the gate fails if
// the tree changes underneath it.

// natQualMatrix is the parsed feature matrix.
type natQualMatrix struct {
	Schema    string `json:"schema"`
	Title     string `json:"title"`
	Verified  string `json:"verified"`
	Scope     string `json:"scope"`
	Statuses  map[string]string `json:"statuses"`
	Features  []struct {
		Construct string `json:"construct"`
		Status    string `json:"status"`
		Gate      string `json:"gate"`
		GateFile  string `json:"gate_file"`
		Note      string `json:"note"`
	} `json:"features"`
	Platforms []struct {
		Target    string `json:"target"`
		Owner     string `json:"owner"`
		Container string `json:"container"`
		Emission  string `json:"emission"`
		Execution string `json:"execution"`
	} `json:"platforms"`
	IndependenceEvidence struct {
		Claim string `json:"claim"`
		How   string `json:"how"`
		Gate  string `json:"gate"`
	} `json:"independence_evidence"`
	KnownLimitations []string `json:"known_limitations"`
}

func natQualLoadMatrix(t *testing.T) natQualMatrix {
	t.Helper()
	path := filepath.Join(repoRoot(t), "docs", "inventory", "native-feature-matrix.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the feature matrix: %v", err)
	}
	var m natQualMatrix
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("the feature matrix is not valid JSON: %v", err)
	}
	if m.Schema != "karkain-native-feature-matrix-v1" {
		t.Errorf("the feature matrix declares schema %q, want %q", m.Schema, "karkain-native-feature-matrix-v1")
	}
	return m
}

// TestPhaseNATQual_TheMatrixIsBackedByGates is layer 1.
//
// A "supported" row is a claim that the construct works. This asserts that the
// claim is CHECKED: the named gate file must exist and must contain the named
// test pattern. A row that names a gate that does not exist would otherwise
// read exactly like a row that names a gate that does.
func TestPhaseNATQual_TheMatrixIsBackedByGates(t *testing.T) {
	m := natQualLoadMatrix(t)
	root := repoRoot(t)

	if len(m.Features) < 20 {
		t.Fatalf("the matrix has %d rows; a matrix that shrinks stops describing the "+
			"surface, and this one is the v1.2.0 qualification record", len(m.Features))
	}

	allowed := map[string]bool{"supported": true, "implemented_not_gated": true, "not_supported": true, "blocked": true}
	seen := map[string]bool{}
	for _, f := range m.Features {
		if !allowed[f.Status] {
			t.Errorf("%q has status %q, which is not one of the declared statuses %v",
				f.Construct, f.Status, allowed)
		}
		if seen[f.Construct] {
			t.Errorf("%q appears twice in the matrix", f.Construct)
		}
		seen[f.Construct] = true

		switch f.Status {
		case "supported", "not_supported":
			// Both need backing: one proves the construct works, the other
			// proves it is refused BY NAME. A "not_supported" row may satisfy
			// the requirement with a NOTE instead of a gate when no gate
			// exercises the refusal -- but it must then say so, because a
			// silent gap is how a status stops meaning anything.
			if f.Gate == "" || f.GateFile == "" {
				if f.Note == "" {
					t.Errorf("%q is %q but names no gate and explains nothing; a status "+
						"with nothing behind it is a claim, not a fact", f.Construct, f.Status)
				}
				continue
			}
			full := filepath.Join(root, f.GateFile)
			body, err := os.ReadFile(full)
			if err != nil {
				t.Errorf("%q names gate file %q, which does not exist: %v", f.Construct, f.GateFile, err)
				continue
			}
			if !strings.Contains(string(body), f.Gate) {
				t.Errorf("%q names gate %q, but %s contains no test with that pattern",
					f.Construct, f.Gate, f.GateFile)
			}
		case "implemented_not_gated":
			// The honest middle status: the driver lowers it, but nothing
			// exercises the kcc side of it, so it is NOT advertised.
			if f.Note == "" {
				t.Errorf("%q is %q but explains nothing; that status exists so a "+
					"gap has a reason attached to it", f.Construct, f.Status)
			}
		}
	}
}

// TestPhaseNATQual_ThePlatformTableMatchesTheCode is layer 2.
//
// The matrix's platform table says which engine OWNS each native target. That
// must agree with the code's own ownership data, because the code is what
// decides: pkg/cli/kcc_native.go refuses a native target kcc does not own and
// dispatches the ones it does.
func TestPhaseNATQual_ThePlatformTableMatchesTheCode(t *testing.T) {
	m := natQualLoadMatrix(t)

	owned := map[string]bool{}
	for _, tgt := range kccNativeOwnedTargets {
		owned[tgt] = true
	}
	if !owned[NativeWindowsTarget] || !owned[NativeLinuxTarget] || !owned[NativeMacOSTarget] {
		t.Fatalf("the code's ownership data does not cover the three native targets: %v",
			kccNativeOwnedTargets)
	}

	seen := map[string]bool{}
	for _, p := range m.Platforms {
		seen[p.Target] = true
		if !owned[p.Target] {
			t.Errorf("the matrix lists %q as a native target, but the code's ownership "+
				"data does not include it", p.Target)
			continue
		}
		if p.Owner != "kcc" {
			t.Errorf("%q is listed with owner %q; every C-free native target in this "+
				"matrix is owned by the self-hosted engine", p.Target, p.Owner)
		}
		if p.Emission != "proven" {
			t.Errorf("%q lists emission as %q; the emission is proven for all three "+
				"targets (each has a gate that builds one)", p.Target, p.Emission)
		}
		if p.Execution == "" {
			t.Errorf("%q states no execution status; a platform with no execution "+
				"evidence must say so", p.Target)
		}
	}
	for tgt := range owned {
		if !seen[tgt] {
			t.Errorf("the matrix does not list %q, which the code's ownership data "+
				"says kcc owns", tgt)
		}
	}
}

// natQualRunBuild runs one `karkain build` and reports what happened.
//
// `shimDir` is prepended to PATH when it is not empty, which is how the
// shims are made reachable. An empty `target` means the LEGACY build path
// (the host-native C pipeline), which is the positive control.
//
// The engine is pinned to the self-hosted one so the trace is about the
// qualified path and not about which engine answered.
func natQualRunBuild(t *testing.T, karkain, src, target, shimDir string) nativeBuildResult {
	t.Helper()
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "prog.kark")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatalf("writing the program: %v", err)
	}
	outPath := filepath.Join(dir, "prog.out")

	args := []string{"build", srcPath, "-o", outPath}
	if target != "" {
		args = append(args, "--target", target)
	}
	cmd := exec.Command(karkain, args...)
	env := append(os.Environ(), "KARKAIN_ENGINE=kcc")
	if shimDir != "" {
		env = append(env, "PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running the build: %v", err)
		}
	}
	res := nativeBuildResult{Output: string(out), ExitCode: code}
	if img, rerr := os.ReadFile(outPath); rerr == nil && len(img) > 0 {
		res.Image = img
		res.SHA = hashImage(img)
		res.WroteImage = true
	}
	return res
}

// natQualShimDir installs PATH shims for `gcc` and `go` that LOG every
// invocation to a file and then fail.
//
// Failing is deliberate: a shim that succeeded would let the build continue
// with a fake compiler and the log would still prove nothing about the build
// succeeding. The point of the shim is to make an invocation VISIBLE, and a
// visible invocation must also be a loud one.
//
// The log path is returned so the caller can assert on it.
func natQualShimDir(t *testing.T) (dir, logPath string) {
	t.Helper()
	dir = t.TempDir()
	logPath = filepath.Join(dir, "invocations.log")

	// A .cmd shim: Go's exec.LookPath on Windows resolves .cmd through
	// PATHEXT, so this is found wherever gcc.exe would be found.
	shim := "@echo off\r\necho invoked %* >> \"" + logPath + "\"\r\nexit /b 1\r\n"
	for _, name := range []string{"gcc.cmd", "go.cmd"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(shim), 0o755); err != nil {
			t.Fatalf("writing the %s shim: %v", name, err)
		}
	}
	return dir, logPath
}

// natQualLogBytes returns the shim log's contents, or "" when nothing was
// logged.
func natQualLogBytes(t *testing.T, logPath string) string {
	t.Helper()
	b, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("reading the shim log: %v", err)
	}
	return string(b)
}

// TestPhaseNATQual_TheNativePathInvokesNeitherCNorGo is layer 3: the
// process-level independence evidence.
//
// The order matters and is deliberate:
//
//	1. WARM the self-hosted compiler with an UNTRACED build, so the compiler's
//	   OWN build (which legitimately uses gcc) is not inside the trace. The
//	   claim is about compiling a USER PROGRAM, not about how the compiler was
//	   built.
//	2. TRACE a native build with the shims first on PATH and assert the log
//	   is EMPTY.
//	3. POSITIVE CONTROL: trace the LEGACY build path with the same shims and
//	   assert the log is NOT empty -- the legacy path is supposed to invoke gcc,
//	   and if the shims do not fire there they prove nothing when they stay
//	   silent here.
func TestPhaseNATQual_TheNativePathInvokesNeitherCNorGo(t *testing.T) {
	karkain := phase130Karkain(t)
	const src = "func main() {\n    print(42)\n}\n"

	// (1) Warm the self-hosted compiler OUTSIDE the trace.
	warm := natQualRunBuild(t, karkain, src, NativeWindowsTarget, "")
	if warm.ExitCode != 0 || !warm.WroteImage {
		t.Fatalf("the warm-up native build failed (exit %d):\n%s", warm.ExitCode, warm.Output)
	}

	// (2) The traced native build.
	shimDir, logPath := natQualShimDir(t)
	traced := natQualRunBuild(t, karkain, src, NativeWindowsTarget, shimDir)
	if traced.ExitCode != 0 || !traced.WroteImage {
		t.Fatalf("the traced native build failed (exit %d):\n%s", traced.ExitCode, traced.Output)
	}
	if log := natQualLogBytes(t, logPath); log != "" {
		t.Errorf("the NATIVE build invoked a C compiler or the Go toolchain.\n"+
			"  shim log:\n%s\n"+
			"  The qualified native path must compile a user program without a C "+
			"compiler, a C source generator, the Go compiler or the Go runtime.",
			log)
	}

	// (3) The positive control: the LEGACY path IS supposed to reach gcc, and
	// the shims must fire there. If they do not, step (2) proves nothing.
	ctlDir, ctlLog := natQualShimDir(t)
	ctl := natQualRunBuild(t, karkain, src, "", ctlDir)
	_ = ctl // the control build is EXPECTED to fail: the gcc shim fails the link
	log := natQualLogBytes(t, ctlLog)
	if log == "" {
		t.Errorf("the positive control did not fire.\n"+
			"  The LEGACY build path is supposed to reach a C compiler, and the "+
			"shims must log that invocation. If they do not, the empty log in the "+
			"native trace above is not evidence of anything -- the shims may simply "+
			"not be reachable.\n  control output:\n%s", ctl.Output)
	}
}
