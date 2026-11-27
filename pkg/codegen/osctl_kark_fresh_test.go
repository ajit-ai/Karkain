package codegen

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// TestOSCtlKarkFresh verifies that src/compiler/osctl_runtime.kark was
// generated from the current runtime/osctl/c/* sources and has not drifted.
// Follows the same pattern as TestOSMetricsKarkFresh.
func TestOSCtlKarkFresh(t *testing.T) {
	// Locate repository root relative to this test file.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")

	karkPath := filepath.Join(root, "src", "compiler", "osctl_runtime.kark")
	cDir := filepath.Join(root, "runtime", "osctl", "c")

	// Read the generated file and extract the recorded SHA-256 digests.
	karkBytes, err := os.ReadFile(karkPath)
	if err != nil {
		t.Fatalf("cannot read %s: %v", karkPath, err)
	}
	karkText := string(karkBytes)

	re := regexp.MustCompile(`// runtime-sha256: (.+)`)
	m := re.FindStringSubmatch(karkText)
	if m == nil {
		t.Fatal("osctl_runtime.kark has no // runtime-sha256: header line")
	}
	recorded := map[string]string{}
	for _, part := range strings.Fields(m[1]) {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			recorded[kv[0]] = kv[1]
		}
	}

	files := []string{"karkain_osctl.h", "karkain_osctl.c"}
	for _, f := range files {
		path := filepath.Join(cDir, f)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("cannot read %s: %v", path, err)
		}
		// Normalize CRLF → LF before hashing (matches PowerShell script).
		normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
		h := sha256.New()
		io.WriteString(h, normalized)
		actual := fmt.Sprintf("%x", h.Sum(nil))

		want, ok := recorded[f]
		if !ok {
			t.Errorf("file %s not found in // runtime-sha256: header", f)
			continue
		}
		if actual != want {
			t.Errorf("file %s digest mismatch:\n  recorded: %s\n  actual:   %s\nRerun scripts/gen-osctl-runtime.ps1", f, want, actual)
		}
	}
}
