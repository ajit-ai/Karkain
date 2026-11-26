// v1.2.24 freshness gate: src/compiler/rc_runtime.kark is generated from
// runtime/rc/c/* by scripts/gen-rc-runtime.ps1 (never hand-edited).
package codegen

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRCKarkFresh(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	genPath := filepath.Join(root, "src", "compiler", "rc_runtime.kark")
	raw, err := os.ReadFile(genPath)
	if err != nil {
		t.Fatalf("generated runtime not found at %s: %v (run scripts/gen-rc-runtime.ps1)", genPath, err)
	}
	if !strings.Contains(string(raw), "func emitRCRuntime(state)") {
		t.Fatalf("generated file must define emitRCRuntime")
	}
	// Parse the recorded digests.
	header := string(raw)
	idx := strings.Index(header, "runtime-sha256:")
	if idx < 0 {
		t.Fatalf("generated file missing runtime-sha256 header")
	}
	line := header[idx:]
	if nl := strings.Index(line, "\n"); nl >= 0 {
		line = line[:nl]
	}
	fields := strings.Fields(strings.TrimPrefix(line, "runtime-sha256:"))
	want := map[string]string{}
	for _, f := range fields {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) != 2 {
			continue
		}
		want[parts[0]] = parts[1]
	}
	files := []string{"karkain_rc.h", "karkain_rc.c"}
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(root, "runtime", "rc", "c", name))
		if err != nil {
			t.Fatal(err)
		}
		norm := make([]byte, 0, len(data))
		for _, b := range data {
			if b != 13 {
				norm = append(norm, b)
			}
		}
		h := sha256.Sum256(norm)
		got := hex.EncodeToString(h[:])
		if want[name] != got {
			t.Fatalf("runtime source %s drifted (want regen):\n  recorded %s\n  actual   %s\nrun scripts/gen-rc-runtime.ps1", name, want[name], got)
		}
	}
}