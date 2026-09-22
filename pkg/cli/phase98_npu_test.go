package cli

import (
	"strings"
	"testing"

	"karkain/pkg/sema"
)

// Phase 98: @target(...) integration through the CLI pipeline. These tests
// prove the attribute is accepted for check/run (both engines), unknown
// targets are rejected with a semantic diagnostic on every engine (including
// the default self-hosted kcc engine, which preserves the attribute as an
// inert C comment), and plain programs are entirely unaffected.

const phase98TargetedSrc = `@target(npu)
func helper(x int) {
    print(x)
}

func main() {
    helper(41)
}
`

const phase98BogusSrc = `@target(tpu_turbo)
func helper(x int) {
    print(x)
}

func main() {
    helper(1)
}
`

func TestPhase98_CLI_AnalyzeSourceValidatesTargets(t *testing.T) {
	diags, _, _ := AnalyzeSource("valid.kark", phase98TargetedSrc, nil)
	if len(diags) != 0 {
		t.Fatalf("valid @target(npu) produced diagnostics: %v", diags)
	}

	badDiags, _, _ := AnalyzeSource("bad.kark", phase98BogusSrc, nil)
	if len(badDiags) != 1 {
		t.Fatalf("expected 1 diagnostic for unknown target, got %d", len(badDiags))
	}
	if !strings.Contains(badDiags[0].Message, "unknown execution target") {
		t.Errorf("diagnostic = %q, want unknown-execution-target wording", badDiags[0].Message)
	}
}

func TestPhase98_CLI_GoEngine_CheckAcceptsAndRejects(t *testing.T) {
	dir := t.TempDir()
	valid := writeTestFile(t, dir, "valid.kark", phase98TargetedSrc)
	res := CheckCommand(valid, false)
	if res.ExitCode != ExitSuccess {
		t.Errorf("valid @target(npu) check = exit %d, want 0: %s", res.ExitCode, res.Message)
	}

	bogus := writeTestFile(t, dir, "bogus.kark", phase98BogusSrc)
	res = CheckCommand(bogus, false)
	if res.ExitCode != ExitCompile {
		t.Errorf("bogus target check = exit %d, want ExitCompile(%d): %s", res.ExitCode, ExitCompile, res.Message)
	}
	if !strings.Contains(res.Message, "semantic") {
		t.Errorf("check message = %q, want semantic-stage wording", res.Message)
	}
}

func TestPhase98_CLI_KCCEngine_CheckAcceptsAndRejects(t *testing.T) {
	hasGCC(t)
	dir := t.TempDir()
	valid := writeTestFile(t, dir, "valid.kark", phase98TargetedSrc)
	res := KCCCheckCommand(nil, valid, false)
	if res.ExitCode != ExitSuccess {
		t.Errorf("kcc check of valid @target(npu) = exit %d, want 0: %s", res.ExitCode, res.Message)
	}

	bogus := writeTestFile(t, dir, "bogus.kark", phase98BogusSrc)
	res = KCCCheckCommand(nil, bogus, false)
	if res.ExitCode != ExitCompile {
		t.Errorf("kcc check of bogus target = exit %d, want ExitCompile(%d): %s", res.ExitCode, ExitCompile, res.Message)
	}
	if !strings.Contains(res.Message, "semantic") {
		t.Errorf("kcc check message = %q, want semantic-stage wording", res.Message)
	}
}

func TestPhase98_CLI_GoEngine_RunTargetedProgram(t *testing.T) {
	hasGCC(t)
	dir := t.TempDir()
	file := writeTestFile(t, dir, "run.kark", phase98TargetedSrc)
	cfg := mustConfig(t)
	out := captureStdout(t, func() {
		res := RunCommand(file, cfg, false)
		if res.ExitCode != ExitSuccess {
			t.Errorf("go run of @target(npu) = exit %d, want 0: %s", res.ExitCode, res.Message)
		}
	})
	if !strings.Contains(out, "41") {
		t.Errorf("go run output does not contain 41, got: %s", out)
	}
}

func TestPhase98_CLI_KCCEngine_RunTargetedProgram(t *testing.T) {
	hasGCC(t)
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	dir := t.TempDir()
	file := writeTestFile(t, dir, "run.kark", phase98TargetedSrc)
	cfg := mustConfig(t)
	res := KCCRunCommand(nil, file, cfg, false)
	if res.ExitCode != ExitSuccess {
		t.Errorf("kcc run of @target(npu) = exit %d, want 0: %s", res.ExitCode, res.Message)
	}
	if !strings.Contains(res.Message, "41") {
		t.Errorf("kcc run output does not contain 41, got: %s", res.Message)
	}
}

func TestPhase98_CLI_KCCEngine_BuildTargetedProgram(t *testing.T) {
	hasGCC(t)
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	dir := t.TempDir()
	file := writeTestFile(t, dir, "build.kark", phase98TargetedSrc)
	cfg := mustConfig(t)
	res := KCCBuildCommand(nil, file, "", cfg, false)
	if res.ExitCode != ExitSuccess {
		t.Errorf("kcc build of @target(npu) = exit %d, want 0: %s", res.ExitCode, res.Message)
	}
}

func TestPhase98_CLI_PlainProgramUnaffectedOnKCC(t *testing.T) {
	hasGCC(t)
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	dir := t.TempDir()
	file := writeTestFile(t, dir, "plain.kark", `
func main() {
    print(7)
}
`)
	res := KCCRunCommand(nil, file, mustConfig(t), false)
	if res.ExitCode != ExitSuccess {
		t.Errorf("kcc run of plain program = exit %d, want 0: %s", res.ExitCode, res.Message)
	}
	if !strings.Contains(res.Message, "7") {
		t.Errorf("kcc run output does not contain 7, got: %s", res.Message)
	}
}

func TestPhase98_CLI_TargetNamesMatchAnalyzer(t *testing.T) {
	// The CLI wiring and the analyzer must agree on supported targets
	// (Phase 138 added gpu to the Phase 98 cpu/npu set).
	for _, name := range sema.TargetNames {
		if name != sema.TargetCPU && name != sema.TargetNPU && name != sema.TargetGPU {
			t.Errorf("unexpected target in registry: %q", name)
		}
	}
}