package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTargetCommand_ListsAllTargets(t *testing.T) {
	out := captureStdout(t, func() {
		if res := TargetCommand(); res.ExitCode != ExitSuccess {
			t.Errorf("target: got %d", res.ExitCode)
		}
	})
	for _, want := range []string{"native", "c23", "wasm32-wasi", "Default: native"} {
		if !strings.Contains(out, want) {
			t.Errorf("target output missing %q:\n%s", want, out)
		}
	}
}

func TestConfigCommand_PrintsEffectiveConfig(t *testing.T) {
	out := captureStdout(t, func() {
		if res := ConfigCommand(); res.ExitCode != ExitSuccess {
			t.Errorf("config: got %d", res.ExitCode)
		}
	})
	for _, want := range []string{"target:", "native", "ssa-pipeline:", "true", "output-path:"} {
		if !strings.Contains(out, want) {
			t.Errorf("config output missing %q:\n%s", want, out)
		}
	}
}

func TestCLI_TargetConfig_E2E(t *testing.T) {
	bin := buildKarkain(t)
	root := t.TempDir()

	out, err := runBin(t, bin, root, "target")
	if err != nil {
		t.Fatalf("karkain target failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "c23") || !strings.Contains(out, "wasm32-wasi") {
		t.Errorf("target output unexpected:\n%s", out)
	}

	out, err = runBin(t, bin, root, "config")
	if err != nil {
		t.Fatalf("karkain config failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "target:") || !strings.Contains(out, "native") {
		t.Errorf("config output unexpected:\n%s", out)
	}
}

func TestCLI_AuditVerifyJSON_E2E(t *testing.T) {
	bin := buildKarkain(t)
	root := t.TempDir()

	if _, err := runBin(t, bin, root, "new", "proj"); err != nil {
		t.Fatalf("karkain new failed: %v", err)
	}
	proj := strings.TrimSpace(root)
	appDir := root + "\\proj"

	// audit --json in a fresh project: valid JSON, empty vulnerability list.
	out, err := runBin(t, bin, appDir, "pkg", "audit", "--json")
	if err != nil {
		t.Fatalf("pkg audit --json failed: %v\n%s", err, out)
	}
	var auditDoc struct {
		Vulnerabilities []struct {
			Name string `json:"Name"`
		} `json:"vulnerabilities"`
	}
	if jerr := json.Unmarshal([]byte(out), &auditDoc); jerr != nil {
		t.Fatalf("audit --json not valid JSON: %v\n%s", jerr, out)
	}
	if len(auditDoc.Vulnerabilities) != 0 {
		t.Errorf("expected empty vulnerabilities, got %+v", auditDoc.Vulnerabilities)
	}

	// verify --json without a lockfile: ok=false, one failure entry, exit 5.
	out, err = runBin(t, bin, appDir, "pkg", "verify", "--json")
	if err == nil {
		t.Fatalf("pkg verify --json without lock should fail, out=%s", out)
	}
	var verifyDoc struct {
		Ok       bool `json:"ok"`
		Packages []struct {
			Valid bool `json:"Valid"`
		} `json:"packages"`
	}
	if jerr := json.Unmarshal([]byte(out), &verifyDoc); jerr != nil {
		t.Fatalf("verify --json not valid JSON: %v\n%s", jerr, out)
	}
	if verifyDoc.Ok {
		t.Error("verify --json said ok without a lockfile")
	}
	if len(verifyDoc.Packages) == 0 {
		t.Error("verify --json should report the lock-file failure entry")
	}
	_ = proj
}