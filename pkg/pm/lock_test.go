package pm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLock_WriteAndReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LockFileName)

	lf := &LockFile{
		Packages: []LockEntry{
			{
				Name:      "math",
				Version:   "1.2.3",
				Source:    "registry",
				Checksum:  "sha256:abc123",
				Integrity: true,
			},
			{
				Name:      "io",
				Version:   "0.5.0",
				Source:    "git",
				URL:       "https://github.com/example/io",
				Rev:       "deadbeef",
				Checksum:  "sha256:def456",
				Integrity: false,
			},
		},
		Metadata: LockMetadata{
			KarkainVersion: "0.14.0",
			ResolvedAt:     "2026-01-15T10:30:00Z",
		},
	}

	if err := WriteLockFile(path, lf); err != nil {
		t.Fatalf("WriteLockFile failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("lock file was not created")
	}

	parsed, err := ReadLockFile(path)
	if err != nil {
		t.Fatalf("ReadLockFile failed: %v", err)
	}

	if len(parsed.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(parsed.Packages))
	}

	math := parsed.Packages[0]
	if math.Name != "math" {
		t.Errorf("package 0 name: expected %q, got %q", "math", math.Name)
	}
	if math.Version != "1.2.3" {
		t.Errorf("package 0 version: expected %q, got %q", "1.2.3", math.Version)
	}
	if math.Source != "registry" {
		t.Errorf("package 0 source: expected %q, got %q", "registry", math.Source)
	}
	if math.Checksum != "sha256:abc123" {
		t.Errorf("package 0 checksum: expected %q, got %q", "sha256:abc123", math.Checksum)
	}
	if !math.Integrity {
		t.Error("package 0 integrity: expected true")
	}

	io := parsed.Packages[1]
	if io.Name != "io" {
		t.Errorf("package 1 name: expected %q, got %q", "io", io.Name)
	}
	if io.URL != "https://github.com/example/io" {
		t.Errorf("package 1 url: expected %q, got %q", "https://github.com/example/io", io.URL)
	}
	if io.Rev != "deadbeef" {
		t.Errorf("package 1 rev: expected %q, got %q", "deadbeef", io.Rev)
	}
	if io.Integrity {
		t.Error("package 1 integrity: expected false")
	}

	if parsed.Metadata.KarkainVersion != "0.14.0" {
		t.Errorf("metadata karkain_version: expected %q, got %q", "0.14.0", parsed.Metadata.KarkainVersion)
	}
	if parsed.Metadata.ResolvedAt != "2026-01-15T10:30:00Z" {
		t.Errorf("metadata resolved_at: expected %q, got %q", "2026-01-15T10:30:00Z", parsed.Metadata.ResolvedAt)
	}
}

func TestLock_UpdateEntry(t *testing.T) {
	lf := &LockFile{
		Packages: []LockEntry{
			{Name: "math", Version: "1.0.0", Source: "registry"},
		},
	}

	newEntry := LockEntry{
		Name:      "math",
		Version:   "1.2.3",
		Source:    "registry",
		Checksum:  "sha256:updated",
		Integrity: true,
	}
	UpdateLockEntry(lf, "math", newEntry)

	if len(lf.Packages) != 1 {
		t.Fatalf("expected 1 package after update, got %d", len(lf.Packages))
	}
	if lf.Packages[0].Version != "1.2.3" {
		t.Errorf("expected version %q, got %q", "1.2.3", lf.Packages[0].Version)
	}
	if lf.Packages[0].Checksum != "sha256:updated" {
		t.Errorf("expected checksum %q, got %q", "sha256:updated", lf.Packages[0].Checksum)
	}

	addEntry := LockEntry{
		Name:    "io",
		Version: "0.5.0",
		Source:  "git",
	}
	UpdateLockEntry(lf, "io", addEntry)

	if len(lf.Packages) != 2 {
		t.Fatalf("expected 2 packages after add, got %d", len(lf.Packages))
	}

	dir := t.TempDir()
	path := filepath.Join(dir, LockFileName)
	if err := WriteLockFile(path, lf); err != nil {
		t.Fatalf("WriteLockFile failed: %v", err)
	}
	parsed, err := ReadLockFile(path)
	if err != nil {
		t.Fatalf("ReadLockFile failed: %v", err)
	}
	if len(parsed.Packages) != 2 {
		t.Fatalf("round-trip: expected 2 packages, got %d", len(parsed.Packages))
	}
}

func TestLock_RemoveEntry(t *testing.T) {
	lf := &LockFile{
		Packages: []LockEntry{
			{Name: "math", Version: "1.0.0"},
			{Name: "io", Version: "0.5.0"},
			{Name: "fmt", Version: "2.0.0"},
		},
	}

	RemoveLockEntry(lf, "io")

	if len(lf.Packages) != 2 {
		t.Fatalf("expected 2 packages after remove, got %d", len(lf.Packages))
	}
	for _, pkg := range lf.Packages {
		if pkg.Name == "io" {
			t.Error("package 'io' should have been removed")
		}
	}

	RemoveLockEntry(lf, "nonexistent")
	if len(lf.Packages) != 2 {
		t.Errorf("removing nonexistent should not change count: got %d", len(lf.Packages))
	}

	dir := t.TempDir()
	path := filepath.Join(dir, LockFileName)
	if err := WriteLockFile(path, lf); err != nil {
		t.Fatalf("WriteLockFile failed: %v", err)
	}
	parsed, err := ReadLockFile(path)
	if err != nil {
		t.Fatalf("ReadLockFile failed: %v", err)
	}
	if len(parsed.Packages) != 2 {
		t.Errorf("round-trip: expected 2 packages, got %d", len(parsed.Packages))
	}
}

func TestLock_IsLocked(t *testing.T) {
	lf := &LockFile{
		Packages: []LockEntry{
			{Name: "math", Version: "1.2.3"},
			{Name: "io", Version: "0.5.0"},
		},
	}

	if !IsLocked(lf, "math", "1.2.3") {
		t.Error("expected math 1.2.3 to be locked")
	}
	if !IsLocked(lf, "io", "0.5.0") {
		t.Error("expected io 0.5.0 to be locked")
	}
	if IsLocked(lf, "math", "2.0.0") {
		t.Error("math 2.0.0 should not be locked")
	}
	if IsLocked(lf, "fmt", "1.0.0") {
		t.Error("fmt 1.0.0 should not be locked")
	}
	if IsLocked(lf, "math", "") {
		t.Error("empty version should not match")
	}
}

func TestLock_SortedPackageNames(t *testing.T) {
	lf := &LockFile{
		Packages: []LockEntry{
			{Name: "zebra"},
			{Name: "alpha"},
			{Name: "mid"},
		},
	}

	names := SortedPackageNames(lf)
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	expected := []string{"alpha", "mid", "zebra"}
	for i, n := range expected {
		if names[i] != n {
			t.Errorf("names[%d]: expected %q, got %q", i, n, names[i])
		}
	}
}
