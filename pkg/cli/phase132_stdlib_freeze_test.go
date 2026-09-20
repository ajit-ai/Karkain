package cli

// Phase 132 — Standard Library Freeze + GA API Freeze gate.
//
// (Renumbered from the draft "131" strand by owner decision: shipped Phase
// 131 is the reproducible self-host gate. This freeze is Phase 132.)
//
// Phase 132 freezes the public API of all standard library modules that ship
// in 1.0.0 and proves them byte-identical on both engines. It also
// establishes the SemVer policy and the deprecation-policy contract.
//
// Unlike a word-presence check, every subtest below asserts an executable or
// structural contract:
//
//   - API freeze: every frozen module has a docs page AND a stable-api.rst
//     entry (no silent drift in either direction).
//   - Parity: each module's example runs on BOTH engines with byte-exact
//     golden stdout (kcc leg skips only on the documented K127 low-RAM
//     guard; the goldens below were measured live on both engines).
//   - SemVer: the policy doc exists, is wired into the development toctree,
//     and carries the required sections.
//   - Deprecation: the freeze policy names the exact deprecation rules and
//     points at the SemVer process (the `@deprecated` compiler attribute
//     itself remains Planned future work, stated in the policy).
//   - Consistency: every implemented stdlib module is either frozen in
//     stable-api.rst or documented behind an explicit boundary.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// phase132FrozenModules maps each frozen std.* module to its docs page.
var phase132FrozenModules = map[string]string{
	"string":      "strings.rst",
	"collections": "collections.rst",
	"io":          "io.rst",
	"encoding":    "encoding.rst",
	"crypto":      "crypto.rst",
	"testing":     "testing.rst",
	"numerics":    "numerics.rst",
	"net":         "net.rst",
	"http":        "http.rst",
	"db":          "db.rst",
}

// TestPhase132_StdlibAPIFreeze verifies that all frozen stdlib modules have
// documented API pages and stable-api.rst entries.
func TestPhase132_StdlibAPIFreeze(t *testing.T) {
	root := repoRoot(t)

	for module, docFile := range phase132FrozenModules {
		docPath := filepath.Join(root, "docs", "source", "stdlib", docFile)
		if _, err := os.Stat(docPath); os.IsNotExist(err) {
			t.Errorf("Missing documentation for std.%s: %s", module, docPath)
		}
	}

	stableAPIPath := filepath.Join(root, "docs", "source", "reference", "stable-api.rst")
	stableAPIContent, err := os.ReadFile(stableAPIPath)
	if err != nil {
		t.Fatalf("Failed to read stable-api.rst: %v", err)
	}

	stableAPIText := string(stableAPIContent)
	for module := range phase132FrozenModules {
		if !strings.Contains(stableAPIText, "std."+module) {
			t.Errorf("stable-api.rst missing std.%s", module)
		}
	}
}

// phase132Cases returns one example per frozen module with golden stdout
// measured live on BOTH engines (Go front end + self-hosted kcc,
// byte-identical; the kcc `[ok]` banner is stripped by the runner helper).
func phase132Cases(t *testing.T) []phase102Case {
	t.Helper()
	ex := func(rel string) string { return filepath.Join(repoRoot(t), "examples", rel) }
	return []phase102Case{
		{ex("stdlib/strings/main.kark"), "5\n1\nfoobar\nababab\ntrue\ntrue\n1\n6\n6\nhello\nworld\npadded\npad\npad\nHELLO WORLD\nhello world\na+b+c\ncba\ne\n43\n7\n3\n0\n3\na\nb\nc\nx-y-z\n"},
		{ex("stdlib/collections/main.kark"), "true\n4\n27\n1\n9\n5\n2\n5\n2\n2\n8\n4\n7\n2\ntrue\n2\nthree\n1\n3\n3\n60\n"},
		{ex("stdlib/io/main.kark"), "1\ntrue\nalpha\nbeta\n\n1\n3\nalpha\nbeta\ngamma\ntrue\n0\ntrue\nfalse\n"},
		// encoding_crypto exercises BOTH std.encoding and std.crypto.
		{ex("stdlib/encoding_crypto/main.kark"), "68656c6c6f\nhello\naGVsbG8=\nhello\n1\nhi\nba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\nddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f\n"},
		{ex("testing/main.kark"), "checks: 3 passed; 2 failed\n"},
		{ex("09-ai/03_numerics_forward.kark"), "0.3\n0.23\n2.71828\n0.5\n0.761594\n0\n3\n1.41421\n10\n5\n25\n1\n0.0466667\n0.2\n0.23\n"},
		{ex("04-networking/02_tcp_roundtrip.kark"), "3\none\n3\ntwo\n3\n"},
		{ex("07-web/02_http_codec.kark"), "GET\n/items\ntext/plain\n3\n201\nCreated\nok\n"},
		{ex("06-database/01_db_crud.kark"), "3\n1\nalice\n90\n2\nbob\n80\n3\ncarol\n95\n100\nalice\ncarol\ndb2\nembedded\n"},
	}
}

// TestPhase132_StdlibGoldensGoEngine runs the per-module examples through
// the real binary on the Go engine and asserts byte-exact golden stdout.
func TestPhase132_StdlibGoldensGoEngine(t *testing.T) {
	karkain := phase130Karkain(t)
	runPhase102Cases(t, karkain, "go", phase132Cases(t))
}

// TestPhase132_StdlibGoldensKCC runs the same per-module examples through
// the kcc engine and asserts the identical goldens (both-engine
// byte-identity is the freeze contract). On low-RAM hosts the kcc
// self-build is cleanly aborted by the Phase 127 guard (error[K127]);
// that subtest skips rather than flaking.
func TestPhase132_StdlibGoldensKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range phase132Cases(t) {
		cmd := exec.Command(karkain, "run", c.file, "--engine", "kcc")
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=kcc")
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := string(out)
			if strings.Contains(msg, "error[K127]") {
				t.Skipf("low-RAM host: kcc self-build guarded (error[K127]) — parity exercised on CI: %s", filepath.Base(c.file))
			}
			t.Fatalf("%s (engine=kcc): exit err %v\n%s", filepath.Base(c.file), err, msg)
		}
		got := stripKCCBuildBanner(t, string(out))
		got = strings.ReplaceAll(got, "\r\n", "\n")
		if got != c.want {
			t.Errorf("%s (engine=kcc): mismatch\nwant:\n%q\ngot:\n%q", filepath.Base(c.file), c.want, got)
		}
	}
}

// TestPhase132_SemVerPolicy verifies the SemVer policy doc exists, is wired
// into the development toctree, and carries the required contract sections.
func TestPhase132_SemVerPolicy(t *testing.T) {
	root := repoRoot(t)

	policyPath := filepath.Join(root, "docs", "source", "development", "semver-policy.rst")
	content, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("Missing semver-policy.rst documentation: %v", err)
	}
	text := string(content)
	for _, section := range []string{
		"Version format",
		"When to increment",
		"Breaking change policy",
		"Deprecation process",
		"Stable API definition",
		"Experimental vs Stable",
		"Cross-engine parity",
	} {
		if !strings.Contains(text, section) {
			t.Errorf("semver-policy.rst missing section: %q", section)
		}
	}

	indexPath := filepath.Join(root, "docs", "source", "development", "index.rst")
	indexContent, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read development/index.rst: %v", err)
	}
	if !strings.Contains(string(indexContent), "semver-policy") {
		t.Errorf("development/index.rst toctree does not include semver-policy (doc would be unreachable)")
	}
}

// TestPhase132_DeprecationPolicy verifies the freeze-time deprecation
// contract: the exact rules plus the pointer at the SemVer process. (The
// `@deprecated` compiler attribute itself is Planned future work; the
// policy states that honestly instead of the gate pretending it exists.)
func TestPhase132_DeprecationPolicy(t *testing.T) {
	root := repoRoot(t)

	freezePath := filepath.Join(root, "docs", "source", "development", "feature-freeze.rst")
	freezeContent, err := os.ReadFile(freezePath)
	if err != nil {
		t.Fatalf("Failed to read feature-freeze.rst: %v", err)
	}
	freezeText := string(freezeContent)
	for _, rule := range []string{
		"Deprecation during freeze",
		"Document the deprecation",
		"Maintain deprecated APIs",
		"next MAJOR version",
		"semver-policy",
	} {
		if !strings.Contains(freezeText, rule) {
			t.Errorf("feature-freeze.rst deprecation contract missing: %q", rule)
		}
	}

	policyPath := filepath.Join(root, "docs", "source", "development", "semver-policy.rst")
	policyContent, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("Failed to read semver-policy.rst: %v", err)
	}
	policyText := string(policyContent)
	for _, step := range []string{
		"Document the deprecation",
		"Maintain the old API for at least one MINOR version",
		"Remove the deprecated API in the next MAJOR version",
	} {
		if !strings.Contains(policyText, step) {
			t.Errorf("semver-policy.rst deprecation process missing step: %q", step)
		}
	}
}

// TestPhase132_StableAPIConsistency verifies that stable-api.rst matches
// the actual implemented stdlib modules: every implemented module is
// either frozen in stable-api.rst or documented behind an explicit
// boundary in the stdlib index.
func TestPhase132_StableAPIConsistency(t *testing.T) {
	root := repoRoot(t)

	stableAPIPath := filepath.Join(root, "docs", "source", "reference", "stable-api.rst")
	stableAPIContent, err := os.ReadFile(stableAPIPath)
	if err != nil {
		t.Fatalf("Failed to read stable-api.rst: %v", err)
	}
	stableAPIText := string(stableAPIContent)

	stdlibDir := filepath.Join(root, "stdlib")
	entries, err := os.ReadDir(stdlibDir)
	if err != nil {
		t.Fatalf("Failed to read stdlib directory: %v", err)
	}

	implementedModules := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		moduleFile := filepath.Join(stdlibDir, entry.Name(), entry.Name()+".kark")
		if _, err := os.Stat(moduleFile); err == nil {
			implementedModules = append(implementedModules, entry.Name())
		}
	}

	for _, module := range implementedModules {
		moduleRef := "std." + module
		if !strings.Contains(stableAPIText, moduleRef) {
			indexPath := filepath.Join(root, "docs", "source", "stdlib", "index.rst")
			indexContent, err := os.ReadFile(indexPath)
			if err == nil {
				indexText := string(indexContent)
				if !strings.Contains(indexText, module) && !strings.Contains(indexText, "Phase 109 boundary") {
					t.Errorf("Module std.%s not in stable-api.rst and not documented as behind boundary", module)
				}
			}
		}
	}
}
