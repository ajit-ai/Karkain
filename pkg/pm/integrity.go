package pm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const ChecksumFile = ".checksum"

// Checksum represents a stored checksum record.
type Checksum struct {
	Algorithm  string
	Hash       string
	VerifiedAt string
}

// ComputeFileChecksum computes the sha256 hex digest of a file.
func ComputeFileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cannot open file for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("cannot read file for checksum: %w", err)
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeDirChecksum computes a sha256 digest over all files in a directory,
// sorted by their relative paths.
func ComputeDirChecksum(dirPath string) (string, error) {
	type fileInfo struct {
		relPath string
		absPath string
	}

	var files []fileInfo

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		// Normalize to forward slashes for cross-platform determinism.
		rel = filepath.ToSlash(rel)
		// Exclude the checksum file itself so the recorded hash is stable and
		// self-consistent: the checksum is computed over package content only.
		if rel == ChecksumFile {
			return nil
		}
		files = append(files, fileInfo{relPath: rel, absPath: path})
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("cannot walk directory for checksum: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].relPath < files[j].relPath
	})

	h := sha256.New()
	for _, fi := range files {
		h.Write([]byte(fi.relPath + "\n"))

		f, err := os.Open(fi.absPath)
		if err != nil {
			return "", fmt.Errorf("cannot read %s for checksum: %w", fi.relPath, err)
		}
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return "", fmt.Errorf("cannot hash %s: %w", fi.relPath, err)
		}
		f.Close()

		h.Write([]byte("\n"))
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyChecksum checks whether a file matches the expected checksum.
func VerifyChecksum(path string, expectedChecksum string) (bool, error) {
	computed, err := ComputeFileChecksum(path)
	if err != nil {
		return false, err
	}
	return computed == expectedChecksum, nil
}

// WriteChecksumFile writes a .checksum file in a cached package directory.
func WriteChecksumFile(dirPath string) error {
	checksum, err := ComputeDirChecksum(dirPath)
	if err != nil {
		return fmt.Errorf("cannot compute directory checksum: %w", err)
	}

	cs := Checksum{
		Algorithm:  "sha256",
		Hash:       checksum,
		VerifiedAt: time.Now().UTC().Format(time.RFC3339),
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("algorithm = %q\n", cs.Algorithm))
	sb.WriteString(fmt.Sprintf("hash = %q\n", cs.Hash))
	sb.WriteString(fmt.Sprintf("verified_at = %q\n", cs.VerifiedAt))

	checksumPath := filepath.Join(dirPath, ChecksumFile)
	return os.WriteFile(checksumPath, []byte(sb.String()), 0644)
}

// VerifyChecksumFile verifies the contents of a directory against its stored .checksum file.
func VerifyChecksumFile(dirPath string) (bool, error) {
	checksumPath := filepath.Join(dirPath, ChecksumFile)
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return false, fmt.Errorf("cannot read checksum file: %w", err)
	}

	var stored Checksum
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = stripQuotes(val)

		switch key {
		case "algorithm":
			stored.Algorithm = val
		case "hash":
			stored.Hash = val
		case "verified_at":
			stored.VerifiedAt = val
		}
	}

	if stored.Hash == "" {
		return false, fmt.Errorf("checksum file has no hash")
	}

	current, err := ComputeDirChecksum(dirPath)
	if err != nil {
		return false, err
	}

	return current == stored.Hash, nil
}
