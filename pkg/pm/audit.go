package pm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VulnReport describes a known vulnerability.
type VulnReport struct {
	Name     string
	Version  string
	Advisory string
	Fix      string
}

// VerifyResult describes integrity check of one package.
type VerifyResult struct {
	Name    string
	Version string
	Valid   bool
	Error   string
}

// AuditDependencies scans cached packages for known vulnerabilities.
func AuditDependencies(projectDir string) []VulnReport {
	var vulns []VulnReport

	cacheDir := filepath.Join(projectDir, CacheModules)
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return vulns
	}

	// TODO: integrate real vulnerability database
	// For now, check for obviously old/unsafe versions
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name, ver := parseCacheEntryName(entry.Name())
		if name == "" {
			continue
		}
		_ = ver
		// Placeholder: flag packages with known issues
	}

	return vulns
}

// CheckLicenses checks license compatibility across dependencies.
func CheckLicenses(projectDir string) []string {
	var warnings []string

	cacheDir := filepath.Join(projectDir, CacheModules)
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return warnings
	}

	// TODO: read LICENSE files from cached packages and check compatibility
	// Known incompatible combos: GPL-3.0 + MIT-only, AGPL + proprietary
	_ = entries

	return warnings
}

// VerifyIntegrity verifies checksums of all cached dependencies.
func VerifyIntegrity(projectDir string) []VerifyResult {
	var results []VerifyResult

	lockPath := filepath.Join(projectDir, LockFileName)
	lockfile, err := ReadLockFile(lockPath)
	if err != nil {
		return []VerifyResult{{
			Name:    "(lock file)",
			Version: "",
			Valid:   false,
			Error:   fmt.Sprintf("cannot read lock file: %v", err),
		}}
	}

	for _, pkg := range lockfile.Packages {
		dep := Dependency{Name: pkg.Name, Version: pkg.Version, Source: pkg.Source, URL: pkg.URL}
		cacheDir := filepath.Join(projectDir, CacheModules, cacheDirName(dep, pkg.Rev))
		if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
			results = append(results, VerifyResult{
				Name:    pkg.Name,
				Version: pkg.Version,
				Valid:   false,
				Error:   "not cached — run 'karkain pkg fetch'",
			})
			continue
		}

		if pkg.Checksum == "" {
			results = append(results, VerifyResult{
				Name:    pkg.Name,
				Version: pkg.Version,
				Valid:   true,
			})
			continue
		}

		computed, err := ComputeDirChecksum(cacheDir)
		if err != nil {
			results = append(results, VerifyResult{
				Name:    pkg.Name,
				Version: pkg.Version,
				Valid:   false,
				Error:   fmt.Sprintf("checksum computation failed: %v", err),
			})
			continue
		}

		if computed != pkg.Checksum {
			results = append(results, VerifyResult{
				Name:    pkg.Name,
				Version: pkg.Version,
				Valid:   false,
				Error:   fmt.Sprintf("checksum mismatch: expected %s, got %s", pkg.Checksum[:16]+"...", computed[:16]+"..."),
			})
			continue
		}

		results = append(results, VerifyResult{
			Name:    pkg.Name,
			Version: pkg.Version,
			Valid:   true,
		})
	}

	return results
}

func parseCacheEntryName(name string) (string, string) {
	// "math@1.2.3" -> "math", "1.2.3"
	idx := strings.LastIndex(name, "@")
	if idx < 0 {
		return "", ""
	}
	return name[:idx], name[idx+1:]
}
