package pm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ListCache prints all cached packages.
func ListCache() {
	cwd, _ := os.Getwd()
	projectDir, err := FindProjectRoot(cwd)
	if err != nil {
		projectDir = cwd
	}

	cacheDir := filepath.Join(projectDir, CacheModules)
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		fmt.Println("No cache directory found")
		return
	}

	if len(entries) == 0 {
		fmt.Println("Cache is empty")
		return
	}

	fmt.Printf("Cached packages (%s):\n\n", cacheDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, _ := entry.Info()
		modTime := ""
		if info != nil {
			modTime = info.ModTime().Format("2006-01-02 15:04")
		}
		fmt.Printf("  %-40s %s\n", entry.Name(), modTime)
	}
}

// CleanCache removes cached packages.
func CleanCache(staleOnly bool) {
	cwd, _ := os.Getwd()
	projectDir, err := FindProjectRoot(cwd)
	if err != nil {
		projectDir = cwd
	}

	cacheDir := filepath.Join(projectDir, CacheModules)
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		fmt.Println("No cache directory found")
		return
	}

	cutoff := time.Now().AddDate(0, 0, -30)
	removed := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if staleOnly {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(cutoff) {
				continue
			}
		}

		path := filepath.Join(cacheDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not remove %s: %v\n", entry.Name(), err)
			continue
		}
		fmt.Printf("Removed %s\n", entry.Name())
		removed++
	}

	if removed == 0 {
		if staleOnly {
			fmt.Println("No stale packages found (>30 days)")
		} else {
			fmt.Println("Cache is already empty")
		}
	} else {
		fmt.Printf("\nRemoved %d packages\n", removed)
	}
}
