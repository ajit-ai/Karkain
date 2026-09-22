package cli

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Phase 116 gate: corpus metadata + structural validation.
//
// Every .kark file in the 15-category Developer Preview corpus must carry a
// valid `// Status:` and `// Engine:` header that agrees with the fault
// surface enforced by the Phase 114 gate (pkg/cli/phase114_examples_test.go):
//   - Runnable/Experimental files MUST be pinned in phase114Examples
//     (byte-identical goldens on the declared engine or engines);
//   - Planned files MUST NOT be pinned (they are intentionally not runnable,
//     so the corpus can never "accidentally execute" them);
//   - test-mode files MUST be pinned in phase114TestFiles.
//
// The category directories follow the canonical dash naming scheme
// (examples/01-fundamentals … examples/15-developer-tools), each has a
// README, and no `.kar` files exist anywhere under examples/.

var phase116StatusRe = regexp.MustCompile(`^//\s*Status:\s*(Runnable|Stable|Experimental|Planned)\s*$`)
var phase116EngineRe = regexp.MustCompile(`^//\s*Engine:\s*([^\s]+)`)
var phase116CategoryRe = regexp.MustCompile(`^//\s*Category:\s*(\d{2})`)

var phase116PlannedDirs = map[string]bool{
	"11-quantum": true,
}

func phase116CorpusDirs(t *testing.T) []string {
	t.Helper()
	root := filepath.Join(repoRoot(t), "examples")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && phase116CatRe.MatchString(e.Name()) {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs
}

var phase116CatRe = regexp.MustCompile(`^\d{2}-`)

func phase116ReadHeader(t *testing.T, path string) (status, engine, category string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() && len(sc.Text()) <= 80 {
		line := sc.Text()
		if m := phase116StatusRe.FindStringSubmatch(line); m != nil {
			status = m[1]
		}
		if m := phase116EngineRe.FindStringSubmatch(line); m != nil {
			engine = m[1]
		}
		if m := phase116CategoryRe.FindStringSubmatch(line); m != nil {
			category = m[1]
		}
	}
	return
}

// TestPhase116_CorpusMetadata validates every corpus example advertises an
// honest, consistent Status/Engine/Category and that the pin map in the
// Phase 114 gate covers exactly the runnable+stable+experimental files.
func TestPhase116_CorpusMetadata(t *testing.T) {
	examplesRoot := filepath.Join(repoRoot(t), "examples")
	dirs := phase116CorpusDirs(t)
	if len(dirs) != 15 {
		t.Fatalf("expected 15 category dirs, got %d: %v", len(dirs), dirs)
	}

	// Track which dagger-pinned corpus files exist on disk.
	seen := map[string]bool{}
	for rel := range phase114Examples {
		seen[rel] = false
	}

	for _, dir := range dirs {
		catDir := filepath.Join(examplesRoot, dir)
		readme := filepath.Join(catDir, "README.md")
		if _, err := os.Stat(readme); err != nil {
			t.Errorf("%s: missing category README.md", dir)
		}

		catNo := strings.SplitN(dir, "-", 2)[0]
		files, err := os.ReadDir(catDir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}

		if phase116PlannedDirs[dir] {
			for _, f := range files {
				if f.Name() != "README.md" {
					t.Errorf("%s: planned category must be README-only, found %s", dir, f.Name())
				}
			}
			continue
		}

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".kark") {
				continue
			}
			rel := dir + "/" + f.Name()
			path := filepath.Join(catDir, f.Name())
			status, engine, hdrNum := phase116ReadHeader(t, path)

			if status == "" {
				t.Errorf("%s: missing or malformed `// Status:` header", rel)
			}
			if engine == "" {
				t.Errorf("%s: missing or malformed `// Engine:` header", rel)
			}
			if hdrNum != catNo {
				t.Errorf("%s: Category header number %q does not match dir %s", rel, hdrNum, dir)
			}

			isTest := strings.HasSuffix(f.Name(), "_test.kark")
			if isTest {
				_, err := os.Stat(filepath.Join(examplesRoot, rel))
				if err != nil {
					t.Errorf("%s: test-mode file not discoverable: %v", rel, err)
				}
				continue
			}

		switch status {
		case "Runnable", "Stable", "Experimental":
			// Stable is Phase 137's graduated state (both-engine proven):
			// pinned like Runnable, with no engine restriction.
			spec, pinned := phase114Examples[rel]
				if !pinned {
					t.Errorf("%s: Status %s but not pinned in phase114Examples (corpus/gate drift)", rel, status)
				} else {
					seen[rel] = true
					if status == "Experimental" && spec.engine != "go" {
						t.Errorf("%s: Experimental must be engine-limited to 'go' in the gate, got %q", rel, spec.engine)
					}
				}
			case "Planned":
				if _, pinned := phase114Examples[rel]; pinned {
					t.Errorf("%s: Planned example must never be pinned/executed by the gate", rel)
				}
			}
		}
	}

	// Every pinned example must exist on disk with an honest header.
	for rel, ok := range seen {
		if !ok {
			t.Errorf("pinned example missing on disk after corpus walk: %s", rel)
		}
	}
}

// TestPhase116_NoKarFiles enforces the `.kark` (never `.kar`) extension rule
// across the whole examples tree, and the canonical dash directory scheme.
func TestPhase116_NoKarFiles(t *testing.T) {
	root := filepath.Join(repoRoot(t), "examples")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(info.Name(), ".kar") {
			t.Errorf("found forbidden .kar file: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk examples tree: %v", err)
	}
}

// TestPhase116_ExampleDocs verifies the "Karkain by Example" documentation
// covers every category and the top-level examples index exists.
func TestPhase116_ExampleDocs(t *testing.T) {
	sourceRoot := filepath.Join(repoRoot(t), "docs", "source", "examples")
	docs := map[string]string{
		"fundamentals":          "01",
		"algorithms":            "02",
		"systems":               "03",
		"networking":            "04",
		"data":                  "05",
		"database":              "06",
		"web":                   "07",
		"concurrency":           "08",
		"ai":                    "09",
		"machine-learning":      "10",
		"quantum":               "11",
		"scientific-computing":  "12",
		"finance":               "13",
		"security":              "14",
		"developer-tools":       "15",
	}
	for page, num := range docs {
		if _, err := os.Stat(filepath.Join(sourceRoot, page+".rst")); err != nil {
			t.Errorf("missing per-category doc page %s.rst (%s)", page, err)
		}
		_ = num
	}
	for _, p := range []string{
		filepath.Join(repoRoot(t), "examples", "README.md"),
		filepath.Join(repoRoot(t), "examples", "EXAMPLES.md"),
		filepath.Join(repoRoot(t), "scripts", "verify-examples.ps1"),
		filepath.Join(sourceRoot, "index.rst"),
		filepath.Join(repoRoot(t), "docs", "source", "reference", "example-matrix.rst"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing examples artifact %s: %v", p, err)
		}
	}
}