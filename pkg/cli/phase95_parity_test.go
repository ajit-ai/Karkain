package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"karkain/pkg/codegen"
)

// mustConfig returns a fresh default codegen config for engine-driven tests.
func mustConfig(t *testing.T) codegen.Config {
	t.Helper()
	return codegen.NewConfig()
}

// Phase 95: self-hosted engine (kcc) parity gate. The acceptance contract for
// making kcc the primary engine is that it must reproduce the exact runtime
// behavior of the Go front end on both pinned corpora:
//
//   - examples/probes/*: 12 golden-output programs (same map as the Go path)
//   - conformance/*_test.kark: 11 files of test_* assertions (zero failures)
//
// The tests bootstrap kcc from src/compiler into an isolated temp dir (never
// writing into the repo) and drive it through the KCCRunCommand engine path.

// phase95KCC builds (or reuses) the self-hosted compiler binary in an isolated
// temp directory and returns its path. KARKAIN_KCC, when set and valid, wins.
func phase95KCC(t *testing.T) string {
	t.Helper()
	if env := os.Getenv("KARKAIN_KCC"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	hasGCC(t)
	root := repoRoot(t)
	srcDir := filepath.Join(root, "src", "compiler")
	if _, err := os.Stat(filepath.Join(srcDir, "main.kark")); err != nil {
		t.Skip("src/compiler not present; cannot build the self-hosted engine")
	}

	karkain := buildBootstrap(t, root)
	sandbox := gccTempDir(t)
	comp := filepath.Join(sandbox, "compiler")
	if err := os.MkdirAll(comp, 0o755); err != nil {
		t.Fatalf("mkdir compiler sandbox: %v", err)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatalf("read src/compiler: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".kark") {
			data, err := os.ReadFile(filepath.Join(srcDir, e.Name()))
			if err != nil {
				t.Fatalf("read %s: %v", e.Name(), err)
			}
			if err := os.WriteFile(filepath.Join(comp, e.Name()), data, 0o644); err != nil {
				t.Fatalf("copy %s: %v", e.Name(), err)
			}
		}
	}

	mainFile := filepath.Join(comp, "main.kark")
	build := exec.Command(karkain, "build", mainFile)
	build.Dir = comp
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("stage-1 build failed: %v\n%s", err, string(out))
	}
	if _, err := os.Stat(filepath.Join(comp, "main.c")); err != nil {
		t.Fatalf("stage-1 output main.c missing")
	}

	bin := filepath.Join(sandbox, "kcc.exe")
	link := exec.Command("gcc", "-std=c99", "-x", "c", filepath.Join(comp, "main.c"), "-o", bin, "-lgmp")
	link.Dir = comp
	if out, err := link.CombinedOutput(); err != nil {
		t.Fatalf("stage-1 link failed: %v\n%s", err, string(out))
	}
	return bin
}

// selectKCCEngine points the engine helpers at the isolated kcc binary for one
// test and restores the caller's environment afterwards.
func selectKCCEngine(t *testing.T, bin string) {
	t.Helper()
	old, had := os.LookupEnv("KARKAIN_KCC")
	t.Setenv("KARKAIN_KCC", bin)
	t.Cleanup(func() {
		if had {
			os.Setenv("KARKAIN_KCC", old)
		} else {
			os.Unsetenv("KARKAIN_KCC")
		}
	})
}

// TestPhase95_ProbesParity compiles and runs every examples/probes/* program via
// the self-hosted engine and asserts the pinned golden outputs. This is the same
// contract enforced on the Go path by TestProbesCorpus_RunsEveryProbe.
func TestPhase95_ProbesParity(t *testing.T) {
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	hasGCC(t)

	root := probesDir(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read probes dir: %v", err)
	}
	ran := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		main := filepath.Join(root, name, "main.kark")
		if _, err := os.Stat(main); err != nil {
			t.Fatalf("probe %s missing main.kark", name)
		}
		want, ok := probeExpected[name]
		if !ok {
			t.Errorf("probe %s has no pinned golden output", name)
			continue
		}
		res := KCCRunCommand(nil, main, mustConfig(t), false)
		if res.ExitCode != ExitSuccess {
			t.Errorf("probe %s (kcc): exit=%d msg=%q", name, res.ExitCode, res.Message)
			continue
		}
		got := lastGoldenLine(res.Message)
		if got != want {
			t.Errorf("probe %s (kcc): got %q want %q", name, got, want)
			continue
		}
		ran++
	}
	if ran < 10 {
		t.Fatalf("kcc parity exercised only %d probes", ran)
	}
}

// TestPhase95_ConformanceParity runs the conformance corpus (11 files of
// test_* assertions) through the self-hosted engine with synthesized drivers
// that call every test function. The corpus passes only when every assertion
// holds (runtime exit 0 and no failure output).
func TestPhase95_ConformanceParity(t *testing.T) {
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	hasGCC(t)

	dir := conformanceDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read conformance dir: %v", err)
	}
	ran := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_test.kark") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		testRe := regexp.MustCompile(`func\s+(test_\w+)\s*\(`)
		matches := testRe.FindAllStringSubmatch(string(src), -1)
		if len(matches) == 0 {
			t.Fatalf("conformance file %s declares no tests", e.Name())
		}
		var mainBody strings.Builder
		for _, m := range matches {
			mainBody.WriteString("    " + m[1] + "()\n")
		}
		driver := string(src) + "\nfunc main() {\n" + mainBody.String() + "}\n"

		sandbox := gccTempDir(t)
		driverFile := filepath.Join(sandbox, e.Name())
		if err := os.WriteFile(driverFile, []byte(driver), 0o644); err != nil {
			t.Fatalf("write driver %s: %v", e.Name(), err)
		}
		res := KCCRunCommand(nil, driverFile, mustConfig(t), false)
		if res.ExitCode != ExitSuccess {
			t.Errorf("conformance %s (kcc): exit=%d msg=%q", e.Name(), res.ExitCode, res.Message)
			continue
		}
		if strings.Contains(res.Message, "assertion failed") {
			t.Errorf("conformance %s (kcc): assertion failure in output %q", e.Name(), res.Message)
			continue
		}
		ran++
	}
	if ran < 5 {
		t.Fatalf("kcc conformance parity exercised only %d files", ran)
	}
}

// TestPhase95_EngineSelection verifies the --engine flag parsing contract.
func TestPhase95_EngineSelection(t *testing.T) {
	if k, err := EngineFlag("kcc"); err != nil || k != EngineKCC {
		t.Errorf("EngineFlag(kcc) = %v, %v", k, err)
	}
	if k, err := EngineFlag("go"); err != nil || k != EngineGo {
		t.Errorf("EngineFlag(go) = %v, %v", k, err)
	}
	if _, err := EngineFlag("bogus"); err == nil {
		t.Errorf("EngineFlag(bogus) expected error")
	}
	t.Setenv("KARKAIN_ENGINE", "kcc")
	if EngineFromEnv() != EngineKCC {
		t.Errorf("EngineFromEnv with KARKAIN_ENGINE=kcc expected EngineKCC")
	}
	t.Setenv("KARKAIN_ENGINE", "go")
	if EngineFromEnv() != EngineGo {
		t.Errorf("EngineFromEnv with KARKAIN_ENGINE=go expected EngineGo")
	}
	// Phase 97: kcc is now the default engine (mirrors the parity proof that
	// kcc reproduces the conformance corpus, probe goldens and test runner).
	t.Setenv("KARKAIN_ENGINE", "")
	if EngineFromEnv() != EngineKCC {
		t.Errorf("EngineFromEnv with empty KARKAIN_ENGINE expected EngineKCC (default)")
	}
}

// lastGoldenLine extracts the program output from a KCCRunCommand message that
// begins with the `[ok]` build banner followed by the program's stdout.
// lastGoldenLine extracts the program output from a KCCRunCommand message that
// begins with the `[ok]` build banner followed by the program's stdout.
func lastGoldenLine(msg string) string {
	if i := strings.Index(msg, "[ok]"); i >= 0 {
		body := strings.TrimSpace(msg[i:])
		if j := strings.IndexByte(body, '\n'); j >= 0 {
			return normalizeNewlines(strings.TrimSpace(body[j+1:]))
		}
		return ""
	}
	return normalizeNewlines(strings.TrimSpace(msg))
}

func normalizeNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}