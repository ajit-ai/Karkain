package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// artifactExtensions lists generated outputs that live next to their source
// .kark file. Clean only ever removes one of these when a sibling .kark file
// exists, so hand-written C sources are never touched.
var artifactExtensions = []string{".wgsl", ".cl", ".spv"}

// CleanCommand removes generated build artifacts from a directory tree:
// transpiled C, shaders, and compiled executables that correspond to a .kark
// source present in the same directory. With all=true it also empties the
// project bin/ directory. Source files, manifests and lockfiles are never
// removed. Removals are ordered (sorted paths) so the report is deterministic.
func CleanCommand(rootDir string, all bool) CommandResult {
	info, err := os.Stat(rootDir)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error accessing path: %v", err)}
	}
	if !info.IsDir() {
		return CommandResult{ExitCode: ExitUsage, Message: "clean expects a directory path"}
	}

	// Collect .kark source bases first so removals are source-anchored.
	bases := []string{}
	testBases := []string{}
	err = filepath.Walk(rootDir, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(fi.Name())) != ".kark" {
			return nil
		}
		base := strings.TrimSuffix(path, filepath.Ext(path))
		if strings.HasSuffix(base, "_test") || strings.HasSuffix(base, ".test") {
			testBases = append(testBases, base)
		} else {
			bases = append(bases, base)
		}
		return nil
	})
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Error scanning directory: %v", err)}
	}

	unique := func(s []string) []string {
		m := map[string]bool{}
		out := []string{}
		for _, v := range s {
			if !m[v] {
				m[v] = true
				out = append(out, v)
			}
		}
		sort.Strings(out)
		return out
	}

	removed := 0
	var failures []string
	var removedPaths []string
	mark := func(path string) {
		fi, statErr := os.Stat(path)
		if statErr != nil {
			return
		}
		if fi.IsDir() {
			return
		}
		if rmErr := os.Remove(path); rmErr != nil {
			failures = append(failures, path)
		} else {
			removed++
			removedPaths = append(removedPaths, path)
		}
	}

	for _, base := range unique(bases) {
		mark(base + ".c")
		mark(base + ".exe")
		mark(base)
		for _, ext := range artifactExtensions {
			// Kernel shaders are named <base>_<kernel>.<ext>.
			if matches, mErr := filepath.Glob(base + "_*" + ext); mErr == nil {
				for _, m := range matches {
					mark(m)
				}
			}
		}
	}
	for _, base := range unique(testBases) {
		mark(base + ".test.c")
		mark(base + ".test.exe")
	}

	if all {
		binDir := filepath.Join(rootDir, "bin")
		if bInfo, bErr := os.Stat(binDir); bErr == nil && bInfo.IsDir() {
			if entries, rErr := os.ReadDir(binDir); rErr == nil {
				names := []string{}
				for _, e := range entries {
					names = append(names, filepath.Join(binDir, e.Name()))
				}
				sort.Strings(names)
				for _, n := range names {
					mark(n)
				}
			}
		}
	}

	msg := fmt.Sprintf("Clean removed %d artifact(s)", removed)
	if len(failures) > 0 {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("%s; %d failure(s) (first: %s)", msg, len(failures), failures[0])}
	}
	for _, p := range unique(removedPaths) {
		fmt.Println("  removed", p)
	}
	return CommandResult{ExitCode: ExitSuccess, Message: msg}
}
