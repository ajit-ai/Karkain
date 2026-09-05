package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// probesDir locates the examples/probes corpus relative to the package dir.
func probesDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := filepath.Join(wd, "..", "..", "examples", "probes")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("probes corpus not found at %s", dir)
	}
	return dir
}

// probeExpected pins the golden stdout for each probe directory. A probe's
// exact output is a contract: any change here or in the probe source must be a
// deliberate, reviewed change of semantics.
var probeExpected = map[string]string{
	"hello":          "hello world",
	"strings":        "11\nh\nw",
	"strings_concat": "foobar\nfoobarfoo",
	"control_flow":   "12\n-1\n0\n1\n2",
	"functions":      "55\n12",
	"structs":        "Ana\n25",
	"arrays":         "3\n30\n50\n5\n99",
	"maps":           "30\nAna",
	"math":           "3\n2\n-1\n3.5\n10\n1",
	"algorithms":     "11\n25\n90\n1\n0",
	"phase81":        "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n1\n-1\n1",
}

// TestProbesCorpus_RunsEveryProbe compiles+runs each examples/probes probe via
// the real pipeline and asserts its exact golden output.
func TestProbesCorpus_RunsEveryProbe(t *testing.T) {
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
		got := normalizeOutput(t, runKarkFile(t, main))
		if got != want {
			t.Errorf("probe %s: got %q want %q", name, got, want)
		}
		ran++
	}
	if ran != len(probeExpected) {
		t.Errorf("probes corpus mismatch: ran %d probes but %d goldens pinned", ran, len(probeExpected))
	}
}

// normalizeOutput trims trailing whitespace and normalizes CRLF, matching the
// algorithm corpus comparison style.
func normalizeOutput(t *testing.T, out string) string {
	t.Helper()
	return strings.ReplaceAll(strings.TrimSpace(out), "\r\n", "\n")
}