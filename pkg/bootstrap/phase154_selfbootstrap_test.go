// Package bootstrap TestPhase154 ---- Increment 154 (v1.2.0 "Sovereignty I"):
// retires the KARKAIN_ENGINE=go stage-1 pin so compiler children run the
// self-hosted kcc by default ("kcc builds kcc").
//
// QUARANTINE, measured rather than assumed (2026-10-29). The three
// bootstrap-running tests in this file each drive a full three-stage build.
// On a GitHub-hosted runner the step is killed with **exit 143** (SIGTERM)
// 110 s in, ~35-60 s after the stage-2 transpile starts -- the exact signature
// REL-002 / B2 already recorded for `TestBootstrap_SeedClosure`
// (docs/audit/PHASE-143 report): no witness line even unbuffered, no timer, no
// OOM or crash fingerprint. This was invisible until 2026-10-29 because every
// earlier CI run failed at step 16 and skipped this gate entirely, so the step
// had never executed.
//
// The split therefore mirrors the Phase-143 arrangement exactly:
//
//   - TestPhase154_PinMechanism runs in the required `Test` job. It proves the
//     contract increment 154 actually changes -- whether KARKAIN_ENGINE=go
//     reaches a compiler child's environment -- without building a compiler,
//     so the required job reports a real conclusion instead of a skip.
//   - TestPhase154_ControlPinActive / _PinRetired / _NoGoEnginePin keep their
//     full assertions but are opt-in behind KARKAIN_SEED_CLOSURE=1. The
//     `phase154-closure` CI job sets it and treats a SKIP as a failure, so the
//     end-to-end claim stays measured somewhere rather than being withdrawn.
//
// A skip here is NOT evidence for v1.2.0 conditions 5-6; those stay recorded
// as open in docs/release/v1.2.0-CHECKLIST.md until a capable host runs them.
package bootstrap

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// findProjectRoot exists in this package (see bootstrap.go); declared here
// for the white-box gate.

// skipIfInsufficientMemory skips the test honestly when the bootstrap memory
// guard (K127) fires; a skipped gate is never a failure.
func skipIfInsufficientMemory(t *testing.T, stage int) {
	if err := CheckBootstrapMemory(stage); err != nil {
		if errors.Is(err, errInsufficientMemory) {
			t.Skipf("insufficient free RAM for bootstrap stage %d: %v", stage, err)
		}
		t.Fatalf("memory probe failed: %v", err)
	}
}

// requireSeedClosure gates the full-bootstrap tests behind an explicit opt-in.
// See the file header: the minutes-long stage-2 transpile is killed with exit
// 143 on GitHub-hosted runners (REL-002 / B2), so an ungated run there reports
// a harness limit as a compiler failure and masks every step after it.
//
// The same variable and the same wording as Phase 143's seed closure, because
// it is the same measured limitation -- not a second, weaker rule.
func requireSeedClosure(t *testing.T) {
	t.Helper()
	if os.Getenv("KARKAIN_SEED_CLOSURE") != "1" {
		t.Skip("full self-bootstrap quarantined: set KARKAIN_SEED_CLOSURE=1 on a capable host (see the file header and PHASE-154-BASELINE.md)")
	}
}

// envHas reports whether a child environment carries exactly want.
func envHas(env []string, want string) bool {
	for _, kv := range env {
		if kv == want {
			return true
		}
	}
	return false
}

// TestPhase154_PinMechanism is the fast half of the 154 gate and the only part
// that runs in the required `Test` job.
//
// It proves the contract increment 154 actually changes -- whether
// KARKAIN_ENGINE=go reaches a compiler child's environment -- by launching a
// trivial probe command in both pin states. Turning the pin into a no-op makes
// the first half fail; forcing the Go engine unconditionally makes the second
// half fail. Neither needs a compiler to be built, so the required job reports
// a real conclusion instead of a skip.
//
// It is deliberately NOT a substitute for the end-to-end proof: this test says
// the pin reaches the child, not that kcc can rebuild kcc.
func TestPhase154_PinMechanism(t *testing.T) {
	t.Cleanup(func() { SetGoEnginePin(true) })
	dir := t.TempDir()

	// 1. Pin active (the pre-154 default): the child carries KARKAIN_ENGINE=go.
	SetGoEnginePin(true)
	if GoEnginePinRetired() {
		t.Fatal("GoEnginePinRetired() = true while the pin is active")
	}
	if _, err := runCmdOutput(dir, "go", "env", "GOOS"); err != nil {
		t.Fatalf("probe child failed: %v", err)
	}
	if !envHas(CompilerChildEnv(), "KARKAIN_ENGINE=go") {
		t.Fatal("pin active: compiler child env does not carry KARKAIN_ENGINE=go")
	}

	// 2. Pin retired (154's state): the same child carries no Go pin, which is
	//    what lets the default self-hosted kcc engine drive the pipeline.
	RetireGoEnginePin()
	if !GoEnginePinRetired() {
		t.Fatal("GoEnginePinRetired() = false after RetireGoEnginePin()")
	}
	if _, err := runCmdOutput(dir, "go", "env", "GOOS"); err != nil {
		t.Fatalf("probe child failed: %v", err)
	}
	if envHas(CompilerChildEnv(), "KARKAIN_ENGINE=go") {
		t.Fatal("pin retired: compiler child env still carries KARKAIN_ENGINE=go")
	}
}

// TestPhase154_ControlPinActive is the control for the retirement: with the
// go pin STILL active, the pipeline must still build all three stages and
// produce stage2 == stage3. It proves the retirement is a real opt-in
// behavior change (the engine of compiler children), not a no-op.
func TestPhase154_ControlPinActive(t *testing.T) {
	requireSeedClosure(t)
	projectRoot := findProjectRoot(t)
	cleanupBinaries(t, projectRoot)
	defer cleanupBinaries(t, projectRoot)

	s1, err := RunStage1(projectRoot)
	if err != nil {
		t.Fatalf("control stage 1 seed build failed: %v", err)
	}
	skipIfInsufficientMemory(t, 2)
	s2, err := RunStage2(projectRoot, s1.Binary)
	if err != nil {
		t.Fatalf("control stage 2 failed: %v", err)
	}
	skipIfInsufficientMemory(t, 3)
	s3, err := RunStage3(projectRoot, s2.Binary)
	if err != nil {
		t.Fatalf("control stage 3 failed: %v", err)
	}
	identical, err := VerifyIdentity(s2, s3)
	if err != nil {
		t.Fatalf("control VerifyIdentity error: %v", err)
	}
	if !identical {
		t.Fatalf("control stage2==stage3 FAILED: %x != %x", s2.SHA256, s3.SHA256)
	}
	// The control child env must still record the Go pin, demonstrating the
	// retirement is observable and the pipeline was actually wired through
	// the old path before 154.
	found := false
	for _, kv := range CompilerChildEnv() {
		if kv == "KARKAIN_ENGINE=go" {
			found = true
		}
	}
	if !found {
		t.Fatal("control: compiler child env must carry KARKAIN_ENGINE=go while the pin is active")
	}
	t.Logf("phase154 control: pin ACTIVE, stage2 == stage3 bitwise identical (%d bytes), child env carried KARKAIN_ENGINE=go", s2.Size)
}

// TestPhase154_PinRetired is the 154 main assertion: with the pin retired,
// the seed stage is still built by the Go toolchain (the 143 ceremony), but
// every compiler child from stage 2 onward runs the self-hosted kcc engine
// with no KARKAIN_ENGINE=go in its environment.
func TestPhase154_PinRetired(t *testing.T) {
	requireSeedClosure(t)
	projectRoot := findProjectRoot(t)
	cleanupBinaries(t, projectRoot)
	defer cleanupBinaries(t, projectRoot)

	s1, err := RunStage1(projectRoot)
	if err != nil {
		t.Fatalf("stage 1 seed build failed: %v", err)
	}
	t.Cleanup(func() { cleanupBinaries(t, projectRoot) })

	skipIfInsufficientMemory(t, 2)
	SetGoEnginePin(true) // ensure the control state is clean
	RetireGoEnginePin()
	compilerChildEnv = nil // no compiler child may have run since retirement
	defer SetGoEnginePin(true)

	s2, err := RunStage2(projectRoot, s1.Binary)
	if err != nil {
		t.Fatalf("stage 2 with pin retired failed: %v", err)
	}
	s3, err := RunStage3(projectRoot, s2.Binary)
	if err != nil {
		t.Fatalf("stage 3 with pin retired failed: %v", err)
	}
	identical, err := VerifyIdentity(s2, s3)
	if err != nil {
		t.Fatalf("VerifyIdentity error: %v", err)
	}
	if !identical {
		t.Fatalf("stage2==stage3 FAILED under the retired pin: %x != %x", s2.SHA256, s3.SHA256)
	}
	t.Logf("phase154 witness: pin RETIRED, stage2 == stage3 bitwise identical (%d bytes)", s2.Size)
}

// TestPhase154_NoGoEnginePin asserts the direct contract of the increment:
// once retired, a compiler child's environment never carries
// KARKAIN_ENGINE=go, so the self-hosted kcc is the default engine of the
// pipeline.
func TestPhase154_NoGoEnginePin(t *testing.T) {
	requireSeedClosure(t)
	projectRoot := findProjectRoot(t)
	cleanupBinaries(t, projectRoot)
	defer cleanupBinaries(t, projectRoot)

	s1, err := RunStage1(projectRoot)
	if err != nil {
		t.Fatalf("stage 1 seed build failed: %v", err)
	}
	defer cleanupBinaries(t, projectRoot)

	skipIfInsufficientMemory(t, 2)
	RetireGoEnginePin()
	compilerChildEnv = nil // no compiler child may have run since retirement
	defer SetGoEnginePin(true)

	// A transplanted stage-2 build must already run without the pin.
	s2, err := RunStage2(projectRoot, s1.Binary)
	if err != nil {
		t.Fatalf("stage 2 without the go pin failed: %v", err)
	}
	s3, err := RunStage3(projectRoot, s2.Binary)
	if err != nil {
		t.Fatalf("stage 3 without the go pin failed: %v", err)
	}
	identical, err := VerifyIdentity(s2, s3)
	if err != nil {
		t.Fatalf("VerifyIdentity error: %v", err)
	}
	if !identical {
		t.Fatalf("stage2==stage3 FAILED: %x != %x", s2.SHA256, s3.SHA256)
	}
	for _, kv := range CompilerChildEnv() {
		if kv == "KARKAIN_ENGINE=go" {
			t.Fatalf("compiler child still carries the Go pin: %q", kv)
		}
	}
	t.Logf("phase154 no-go-pin: stage2 == stage3 bitwise identical (%d bytes), no KARKAIN_ENGINE=go in any compiler child env", s2.Size)
}

// TestPhase154_ReferenceArchive asserts the Go tree is archived, not
// deleted: reference/ holds the Go CLI + Go-engine C as a differential
// oracle, off the user path.
func TestPhase154_ReferenceArchive(t *testing.T) {
	projectRoot := findProjectRoot(t)

	if _, err := os.Stat(filepath.Join(projectRoot, "reference")); err != nil {
		t.Fatalf("reference/ missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "reference", "README.md")); err != nil {
		t.Fatalf("reference/README.md missing: %v", err)
	}
	gitignore, err := os.ReadFile(filepath.Join(projectRoot, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !bytes.Contains(gitignore, []byte("reference/")) {
		t.Error(".gitignore does not cover reference/")
	}
}
