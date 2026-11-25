package codegen

// v1.2.17 freshness gate: src/compiler/tls_runtime.kark is generated from
// runtime/tls/c/* by scripts/gen-tls-runtime.ps1 (never hand-edited). This
// test compares the per-file SHA-256 recorded in the generated header against
// the live sources and fails with the regen command when they drift.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTLSKarkFresh(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Skipf("repo root not found: %v", err)
	}
	genPath := filepath.Join(root, "src", "compiler", "tls_runtime.kark")
	raw, err := os.ReadFile(genPath)
	if err != nil {
		t.Fatalf("generated runtime not found at %s: %v (run scripts/gen-tls-runtime.ps1)", genPath, err)
	}
	const marker = "// runtime-sha256: "
	var recorded string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, marker) {
			recorded = strings.TrimSpace(strings.TrimPrefix(line, marker))
		}
	}
	if recorded == "" {
		t.Fatalf("generated header lacks the %q digest line", marker)
	}
	want := map[string]string{}
	for _, field := range strings.Fields(recorded) {
		name, sum, ok := strings.Cut(field, "=")
		if !ok || name == "" || sum == "" {
			t.Fatalf("malformed digest entry %q", field)
		}
		want[name] = sum
	}
	files := []string{"karkain_tls.h", "karkain_tls.c"}
	if len(want) != len(files) {
		t.Fatalf("digest covers %d files, want %d", len(want), len(files))
	}
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(root, "runtime", "tls", "c", name))
		if err != nil {
			t.Fatalf("read runtime source %s: %v", name, err)
		}
		data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
		data = bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want[name] {
			t.Fatalf("runtime source %s drifted (want regen):\n  recorded %s\n  actual   %s\nrun scripts/gen-tls-runtime.ps1", name, want[name], got)
		}
	}
	if !strings.Contains(string(raw), "func emitTlsRuntime(state)") {
		t.Fatalf("generated file must define emitTlsRuntime")
	}
}
