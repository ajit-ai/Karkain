package codegen

// v1.2.22 freshness gate: src/compiler/sqlite_runtime.kark is generated from
// the small binding sources, while the vendored amalgamation is compiled by
// the CLI. This test pins both mappings.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLiteKarkFresh(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Skipf("repo root not found: %v", err)
	}
	genPath := filepath.Join(root, "src", "compiler", "sqlite_runtime.kark")
	raw, err := os.ReadFile(genPath)
	if err != nil {
		t.Fatalf("generated runtime not found at %s: %v (run scripts/gen-sqlite-runtime.ps1)", genPath, err)
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
	files := []string{"karkain_sqlite.h", "karkain_sqlite.c"}
	if len(want) != len(files) {
		t.Fatalf("digest covers %d files, want %d", len(want), len(files))
	}
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(root, "runtime", "sqlite", "c", name))
		if err != nil {
			t.Fatalf("read binding source %s: %v", name, err)
		}
		data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
		data = bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want[name] {
			t.Fatalf("binding source %s drifted (want regen):\n  recorded %s\n  actual   %s", name, want[name], got)
		}
	}

	vendor := map[string]string{
		"sqlite3.c": "6c35bc5f7f85eac9c49928bacbb02bb694b547aabf69197e058cca245ad80e83",
		"sqlite3.h": "89b62c671c5964e137409ce034941b7b05a3af2c9875aba41f47f9483d0c2515",
	}
	for name, expected := range vendor {
		data, err := os.ReadFile(filepath.Join(root, "runtime", "sqlite", "c", "vendor", name))
		if err != nil {
			t.Fatalf("read vendored %s: %v", name, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != expected {
			t.Fatalf("vendored %s hash = %s, want %s", name, got, expected)
		}
	}
	if !strings.Contains(string(raw), "func emitSQLiteRuntime(state)") {
		t.Fatalf("generated file must define emitSQLiteRuntime")
	}
}
