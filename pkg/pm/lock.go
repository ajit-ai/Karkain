package pm

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const LockFileName = "karkain.lock"

// LockEntry represents a single locked package.
type LockEntry struct {
	Name      string
	Version   string
	Source    string // "registry", "git", "local"
	URL       string
	Rev       string // git revision (commit hash)
	Checksum  string // e.g. "sha256:abc123..."
	Integrity bool
}

// LockMetadata stores lock file metadata.
type LockMetadata struct {
	KarkainVersion string
	ResolvedAt     string
}

// LockFile represents the full karkain.lock structure.
type LockFile struct {
	Packages []LockEntry
	Metadata LockMetadata
}

// ReadLockFile parses a karkain.lock file at the given path.
func ReadLockFile(path string) (*LockFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open lock file: %w", err)
	}
	defer f.Close()

	lf := &LockFile{}
	section := ""
	var current LockEntry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[[package]]") {
			if current.Name != "" {
				lf.Packages = append(lf.Packages, current)
				current = LockEntry{}
			}
			section = "package"
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inner := line[1 : len(line)-1]
			section = inner
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = stripQuotes(val)

		switch section {
		case "package":
			switch key {
			case "name":
				current.Name = val
			case "version":
				current.Version = val
			case "source":
				current.Source = val
			case "url":
				current.URL = val
			case "rev":
				current.Rev = val
			case "checksum":
				current.Checksum = val
			case "integrity":
				current.Integrity = val == "true"
			}

		case "metadata":
			switch key {
			case "karkain_version":
				lf.Metadata.KarkainVersion = val
			case "resolved_at":
				lf.Metadata.ResolvedAt = val
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading lock file: %w", err)
	}

	if current.Name != "" {
		lf.Packages = append(lf.Packages, current)
	}

	return lf, nil
}

// WriteLockFile writes a lock file to the given path.
func WriteLockFile(path string, lf *LockFile) error {
	var sb strings.Builder

	for _, pkg := range lf.Packages {
		sb.WriteString("[[package]]\n")
		sb.WriteString(fmt.Sprintf("name = %q\n", pkg.Name))
		sb.WriteString(fmt.Sprintf("version = %q\n", pkg.Version))
		if pkg.Source != "" {
			sb.WriteString(fmt.Sprintf("source = %q\n", pkg.Source))
		}
		if pkg.URL != "" {
			sb.WriteString(fmt.Sprintf("url = %q\n", pkg.URL))
		}
		if pkg.Rev != "" {
			sb.WriteString(fmt.Sprintf("rev = %q\n", pkg.Rev))
		}
		if pkg.Checksum != "" {
			sb.WriteString(fmt.Sprintf("checksum = %q\n", pkg.Checksum))
		}
		sb.WriteString(fmt.Sprintf("integrity = %s\n", strconv.FormatBool(pkg.Integrity)))
		sb.WriteString("\n")
	}

	if lf.Metadata.KarkainVersion != "" || lf.Metadata.ResolvedAt != "" {
		sb.WriteString("[metadata]\n")
		if lf.Metadata.KarkainVersion != "" {
			sb.WriteString(fmt.Sprintf("karkain_version = %q\n", lf.Metadata.KarkainVersion))
		}
		if lf.Metadata.ResolvedAt != "" {
			sb.WriteString(fmt.Sprintf("resolved_at = %q\n", lf.Metadata.ResolvedAt))
		}
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// UpdateLockEntry adds or updates an entry in the lock file.
func UpdateLockEntry(lf *LockFile, name string, entry LockEntry) {
	for i, pkg := range lf.Packages {
		if pkg.Name == name {
			lf.Packages[i] = entry
			return
		}
	}
	lf.Packages = append(lf.Packages, entry)
}

// RemoveLockEntry removes an entry from the lock file by name.
func RemoveLockEntry(lf *LockFile, name string) {
	for i, pkg := range lf.Packages {
		if pkg.Name == name {
			lf.Packages = append(lf.Packages[:i], lf.Packages[i+1:]...)
			return
		}
	}
}

// IsLocked checks if a package with the exact name and version is locked.
func IsLocked(lf *LockFile, name, version string) bool {
	for _, pkg := range lf.Packages {
		if pkg.Name == name && pkg.Version == version {
			return true
		}
	}
	return false
}

// SetResolvedNow sets the metadata resolved_at timestamp to the current time.
func SetResolvedNow(lf *LockFile) {
	lf.Metadata.ResolvedAt = time.Now().UTC().Format(time.RFC3339)
}

// SortedPackageNames returns the names of all locked packages in sorted order.
func SortedPackageNames(lf *LockFile) []string {
	names := make([]string, 0, len(lf.Packages))
	for _, pkg := range lf.Packages {
		names = append(names, pkg.Name)
	}
	sort.Strings(names)
	return names
}
