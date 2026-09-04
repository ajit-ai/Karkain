// Package pm provides the Karkain package manager.
// It handles project initialization, dependency management, and module resolution
// using karkain.toml manifest files.
package pm

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	ManifestFile = "karkain.toml"
	CacheDir     = ".karkain"
	CacheModules = ".karkain/cache"
)

// Manifest represents a parsed karkain.toml project manifest.
type Manifest struct {
	Name            string
	Version         string
	Author          string
	Description     string
	License         string
	Repository      string
	Targets         []string
	Dependencies    map[string]Dependency
	DevDependencies map[string]Dependency
}

// Dependency represents a single dependency entry.
type Dependency struct {
	Name    string
	Version string
	Source  string // "registry", "git", or "local"
	URL     string // git URL or local path (for non-registry deps)
}

// InitResult is returned by InitProject.
type InitResult struct {
	ProjectDir string
	Manifest   string
	Created    []string // list of created paths
}

// Manager provides package management operations.
type Manager struct {
	RootDir string // project root directory
}

// NewManager creates a new Manager rooted at the given directory.
func NewManager(rootDir string) *Manager {
	return &Manager{RootDir: rootDir}
}

// ============================================================
// karkain.toml Parser
// ============================================================

// ParseManifest reads and parses a karkain.toml file at the given path.
func ParseManifest(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open manifest: %w", err)
	}
	defer f.Close()

	m := &Manifest{
		Dependencies:    make(map[string]Dependency),
		DevDependencies: make(map[string]Dependency),
	}

	section := "" // current section header, e.g. "[dependencies]"
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section headers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}

		// Key-value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = stripQuotes(val)

		switch {
		case section == "":
			// Top-level fields
			switch key {
			case "name":
				m.Name = val
			case "version":
				m.Version = val
			case "author":
				m.Author = val
			case "description":
				m.Description = val
			case "license":
				m.License = val
			case "repository":
				m.Repository = val
			case "targets":
				m.Targets = parseStringArray(val)
			}

		case section == "dependencies":
			dep := parseDependency(key, val)
			m.Dependencies[key] = dep
		case section == "dev-dependencies":
			dep := parseDependency(key, val)
			m.DevDependencies[key] = dep
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading manifest: %w", err)
	}

	return m, nil
}

// WriteManifest writes a manifest to a file.
func WriteManifest(path string, m *Manifest) error {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("name = %q\n", m.Name))
	sb.WriteString(fmt.Sprintf("version = %q\n", m.Version))
	if m.Author != "" {
		sb.WriteString(fmt.Sprintf("author = %q\n", m.Author))
	}
	if m.Description != "" {
		sb.WriteString(fmt.Sprintf("description = %q\n", m.Description))
	}
	if m.License != "" {
		sb.WriteString(fmt.Sprintf("license = %q\n", m.License))
	}
	if m.Repository != "" {
		sb.WriteString(fmt.Sprintf("repository = %q\n", m.Repository))
	}
	if len(m.Targets) > 0 {
		sb.WriteString(fmt.Sprintf("targets = [%s]\n", formatStringArray(m.Targets)))
	}

	if len(m.Dependencies) > 0 {
		sb.WriteString("\n[dependencies]\n")
		// Sort keys for deterministic output
		keys := make([]string, 0, len(m.Dependencies))
		for k := range m.Dependencies {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			dep := m.Dependencies[k]
			if dep.URL != "" {
				sb.WriteString(fmt.Sprintf("%s = { version = %q, source = %q, url = %q }\n",
					k, dep.Version, dep.Source, dep.URL))
			} else {
				sb.WriteString(fmt.Sprintf("%s = { version = %q, source = %q }\n",
					k, dep.Version, dep.Source))
			}
		}
	}

	if len(m.DevDependencies) > 0 {
		sb.WriteString("\n[dev-dependencies]\n")
		keys := make([]string, 0, len(m.DevDependencies))
		for k := range m.DevDependencies {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			dep := m.DevDependencies[k]
			if dep.URL != "" {
				sb.WriteString(fmt.Sprintf("%s = { version = %q, source = %q, url = %q }\n",
					k, dep.Version, dep.Source, dep.URL))
			} else {
				sb.WriteString(fmt.Sprintf("%s = { version = %q, source = %q }\n",
					k, dep.Version, dep.Source))
			}
		}
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// ============================================================
// Project Initialization
// ============================================================

// InitProject creates a new Karkain project with standard directory structure.
func InitProject(projectDir string, name string) (*InitResult, error) {
	result := &InitResult{ProjectDir: projectDir}

	// Create directory structure
	dirs := []string{
		filepath.Join(projectDir, "src"),
		filepath.Join(projectDir, "tests"),
		filepath.Join(projectDir, CacheDir),
		filepath.Join(projectDir, CacheModules),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("cannot create directory %s: %w", d, err)
		}
		result.Created = append(result.Created, d)
	}

	// Create karkain.toml
	manifest := &Manifest{
		Name:         name,
		Version:      "0.1.0",
		Description:  "A Karkain project",
		License:      "MIT",
		Targets:      []string{"native"},
		Dependencies: make(map[string]Dependency),
	}

	manifestPath := filepath.Join(projectDir, ManifestFile)
	if err := WriteManifest(manifestPath, manifest); err != nil {
		return nil, fmt.Errorf("cannot write manifest: %w", err)
	}
	result.Manifest = manifestPath
	result.Created = append(result.Created, manifestPath)

	// Create main.kark entry point
	mainKar := filepath.Join(projectDir, "src", "main.kark")
	mainContent := fmt.Sprintf(`// %s - Main entry point

func main() {
    print("Hello from %s!")
}
`, name, name)
	if err := os.WriteFile(mainKar, []byte(mainContent), 0644); err != nil {
		return nil, fmt.Errorf("cannot write main.kark: %w", err)
	}
	result.Created = append(result.Created, mainKar)

	// Create example test file
	testKar := filepath.Join(projectDir, "tests", "main_test.kark")
	testContent := `// Test file for the project

func test_basic() {
    let result = 1 + 1
    print(result)
}
`
	if err := os.WriteFile(testKar, []byte(testContent), 0644); err != nil {
		return nil, fmt.Errorf("cannot write test file: %w", err)
	}
	result.Created = append(result.Created, testKar)

	// Create .gitignore
	gitignore := filepath.Join(projectDir, ".gitignore")
	gitignoreContent := `bin/
dist/
*.exe
*.o
*.c
` + CacheDir + `/
`
	if err := os.WriteFile(gitignore, []byte(gitignoreContent), 0644); err != nil {
		return nil, fmt.Errorf("cannot write .gitignore: %w", err)
	}
	result.Created = append(result.Created, gitignore)

	return result, nil
}

// ============================================================
// Dependency Management
// ============================================================

// AddDependency adds a dependency to the project manifest.
func AddDependency(projectDir, name, version, source, url string) error {
	manifestPath := filepath.Join(projectDir, ManifestFile)
	m, err := ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("cannot read manifest: %w", err)
	}

	m.Dependencies[name] = Dependency{
		Name:    name,
		Version: version,
		Source:  source,
		URL:     url,
	}

	return WriteManifest(manifestPath, m)
}

// RemoveDependency removes a dependency from the project manifest.
func RemoveDependency(projectDir, name string) error {
	manifestPath := filepath.Join(projectDir, ManifestFile)
	m, err := ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("cannot read manifest: %w", err)
	}

	if _, ok := m.Dependencies[name]; !ok {
		return fmt.Errorf("dependency %q not found", name)
	}

	delete(m.Dependencies, name)
	return WriteManifest(manifestPath, m)
}

// ============================================================
// Module Resolution and Fetching
// ============================================================

// ResolveModule finds the .kark file for a given import path.
func ResolveModule(projectDir, importPath string) (string, error) {
	// 1. Check local project src/ directory
	localPath := filepath.Join(projectDir, "src", importPath+".kark")
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	// 2. Check for subpath (e.g., "io/io" -> "io/io.kark")
	localSub := filepath.Join(projectDir, "src", importPath, filepath.Base(importPath)+".kark")
	if _, err := os.Stat(localSub); err == nil {
		return localSub, nil
	}

	// 3. Check project root for .kark file
	rootKar := filepath.Join(projectDir, importPath+".kark")
	if _, err := os.Stat(rootKar); err == nil {
		return rootKar, nil
	}

	// 4. Check stdlib directory relative to project
	stdlibPath := findStdlib(projectDir, importPath)
	if stdlibPath != "" {
		return stdlibPath, nil
	}

	// 5. Check .karkain/cache
	cachePath := filepath.Join(projectDir, CacheModules, importPath+".kark")
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	// 6. Check for nested stdlib patterns like "io/io"
	nestedStdlib := findStdlibNested(projectDir, importPath)
	if nestedStdlib != "" {
		return nestedStdlib, nil
	}

	return "", fmt.Errorf("module %q not found", importPath)
}

// FetchModule downloads/resolves a dependency into the local cache.
// It fetches into a temporary directory first and atomically renames it into
// place so a partial or failed download never leaves a corrupt package marked
// as valid in the cache. On success it writes a checksum record for the
// fetched package.
func FetchModule(projectDir string, dep Dependency) error {
	// Resolve a git revision up front so the cache identity is source+rev aware.
	rev := ""
	if dep.Source == "git" {
		if dep.URL == "" {
			return fmt.Errorf("git dependency %q requires a url", dep.Name)
		}
		res, _, err := ResolveGitRevision(dep.URL, dep.Version, "")
		if err != nil {
			return err
		}
		rev = res.SHA
	}

	cacheDir := filepath.Join(projectDir, CacheModules, cacheDirName(dep, rev))
	if err := os.MkdirAll(filepath.Join(projectDir, CacheModules), 0755); err != nil {
		return fmt.Errorf("cannot create cache dir: %w", err)
	}

	// Fetch into a temp sibling dir, then rename into place.
	tmpDir, err := os.MkdirTemp(filepath.Dir(cacheDir), ".tmp-"+dep.Name+"-*")
	if err != nil {
		return fmt.Errorf("cannot create temp fetch dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) // clean up on any failure

	switch dep.Source {
	case "local":
		if dep.URL == "" {
			return fmt.Errorf("local dependency %q requires a url (path)", dep.Name)
		}
		if err := fetchLocal(dep.URL, tmpDir); err != nil {
			return err
		}

	case "git":
		if err := cloneAtRev(dep.URL, rev, "", tmpDir); err != nil {
			return err
		}
		// Record the resolved immutable commit for the cached package.
		if err := WriteRevFile(tmpDir, rev); err != nil {
			return err
		}

	case "registry", "":
		if dep.Name == "" {
			return fmt.Errorf("registry dependency requires a name")
		}
		// Resolve the version constraint to an exact published version, then
		// download from the registry into the temp dir.
		c := NewRegistryClient()
		resolved, err := c.ResolveVersion(dep.Name, dep.Version)
		if err != nil {
			return err
		}
		if err := FetchFromRegistry(dep.Name, resolved, tmpDir); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unknown source type %q for dependency %q", dep.Source, dep.Name)
	}

	// Verify the temp result is non-empty before promoting, atomically moves it
	// into the cache, and records a checksum.
	if err := writeChecksumAndPromote(projectDir, tmpDir, cacheDir); err != nil {
		return err
	}
	return nil
}

// validateFetched ensures a fetched temp dir actually contains content,
// guarding against silently landing an empty or partial package.
func validateFetched(dir string) error {
	files := 0
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files++
		}
		return nil
	})
	if files == 0 {
		return fmt.Errorf("fetched package is empty")
	}
	return nil
}

// FetchAll fetches all dependencies listed in the manifest.
func FetchAll(projectDir string) error {
	manifestPath := filepath.Join(projectDir, ManifestFile)
	m, err := ParseManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("cannot read manifest: %w", err)
	}

	for name, dep := range m.Dependencies {
		if err := FetchModule(projectDir, dep); err != nil {
			return fmt.Errorf("failed to fetch %q: %w", name, err)
		}
	}
	return nil
}

// ListDependencies returns a sorted list of dependency names.
func ListDependencies(projectDir string) ([]string, error) {
	manifestPath := filepath.Join(projectDir, ManifestFile)
	m, err := ParseManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(m.Dependencies))
	for name := range m.Dependencies {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// ============================================================
// Internal helpers
// ============================================================

func stripQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseStringArray(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "[]")
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		val := strings.TrimSpace(p)
		val = stripQuotes(val)
		if val != "" {
			result = append(result, val)
		}
	}
	return result
}

func formatStringArray(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}

func parseDependency(key, val string) Dependency {
	dep := Dependency{
		Name:    key,
		Source:  "registry",
		Version: "*",
	}

	val = strings.TrimSpace(val)
	if strings.HasPrefix(val, "{") && strings.HasSuffix(val, "}") {
		inner := val[1 : len(val)-1]
		pairs := splitTopLevel(inner, ',')
		for _, pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])
			v = stripQuotes(v)

			switch k {
			case "version":
				dep.Version = v
			case "source":
				dep.Source = v
			case "url":
				dep.URL = v
			}
		}
	} else {
		dep.Version = stripQuotes(val)
	}

	return dep
}

func splitTopLevel(s string, sep rune) []string {
	var parts []string
	depth := 0
	var current strings.Builder

	for _, ch := range s {
		switch ch {
		case '(', ')', '{', '}', '[', ']':
			if ch == '(' || ch == '{' || ch == '[' {
				depth++
			} else {
				depth--
			}
		}

		if ch == sep && depth == 0 {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

func findStdlib(projectDir, importPath string) string {
	candidates := []string{
		filepath.Join(projectDir, "stdlib", importPath+".kark"),
		filepath.Join(projectDir, "stdlib", importPath, filepath.Base(importPath)+".kark"),
		filepath.Join(projectDir, "std", importPath+".kark"),
		filepath.Join("stdlib", importPath+".kark"),
		filepath.Join("stdlib", importPath, filepath.Base(importPath)+".kark"),
		filepath.Join("std", importPath+".kark"),
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

func findStdlibNested(projectDir, importPath string) string {
	parts := strings.Split(importPath, "/")
	if len(parts) == 2 && parts[0] == parts[1] {
		candidates := []string{
			filepath.Join(projectDir, "stdlib", parts[0], parts[0]+".kark"),
			filepath.Join("stdlib", parts[0], parts[0]+".kark"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				abs, _ := filepath.Abs(candidate)
				return abs
			}
		}
	}
	return ""
}

func fetchLocal(srcPath, destDir string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("source path not found: %w", err)
	}

	if info.IsDir() {
		return copyDir(srcPath, destDir)
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("cannot read source: %w", err)
	}

	destFile := filepath.Join(destDir, filepath.Base(srcPath))
	return os.WriteFile(destFile, data, 0644)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// FindProjectRoot walks up from the given directory looking for karkain.toml.
func FindProjectRoot(startDir string) (string, error) {
	dir := startDir
	for {
		manifestPath := filepath.Join(dir, ManifestFile)
		if _, err := os.Stat(manifestPath); err == nil {
			return dir, nil
		}

		goMod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goMod); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("karkain.toml not found (searched from %s)", startDir)
		}
		dir = parent
	}
}

// ValidateManifest checks a manifest for common issues.
// validPackageName reports whether name is a valid Karkain package identifier:
// lowercase alphanumeric plus '-' or '_', non-empty, must not start or end with
// a separator, and must be a valid file basename. This is the package identity
// rule — separate from file/module naming.
func validPackageName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	if name[0] == '-' || name[0] == '_' || name[len(name)-1] == '-' || name[len(name)-1] == '_' {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

// ValidateManifest reports validation issues for a manifest. It is additive and
// backward compatible: manifests that previously validated still validate unless
// they actually violate a package-identity or dependency rule.
func ValidateManifest(m *Manifest) []string {
	var issues []string

	if m.Name == "" {
		issues = append(issues, "project name is required")
	} else if !validPackageName(m.Name) {
		issues = append(issues, fmt.Sprintf("project name %q is not a valid package name (lowercase alphanumeric plus '-' or '_')", m.Name))
	}
	if m.Version == "" {
		issues = append(issues, "project version is required (e.g., \"0.1.0\")")
	}
	if m.Version != "" {
		re := regexp.MustCompile(`^\d+\.\d+\.\d+`)
		if !re.MatchString(m.Version) {
			issues = append(issues, fmt.Sprintf("version %q does not follow semver (expected X.Y.Z)", m.Version))
		}
	}

	validateDeps := func(deps map[string]Dependency, class string) {
		seen := make(map[string]bool)
		for name, dep := range deps {
			if !validPackageName(name) {
				issues = append(issues, fmt.Sprintf("%s dependency %q is not a valid package name", class, name))
			}
			if seen[name] {
				issues = append(issues, fmt.Sprintf("duplicate %s dependency %q", class, name))
			}
			seen[name] = true
			if dep.Source != "registry" && dep.Source != "git" && dep.Source != "local" && dep.Source != "workspace" {
				issues = append(issues, fmt.Sprintf("%s dependency %q has unknown source %q (expected registry, git, local, or workspace)", class, name, dep.Source))
			}
			if dep.Source == "git" && dep.URL == "" {
				issues = append(issues, fmt.Sprintf("%s dependency %q with source \"git\" requires a url", class, name))
			}
			if dep.Source == "local" && dep.URL == "" {
				issues = append(issues, fmt.Sprintf("%s dependency %q with source \"local\" requires a url (path)", class, name))
			}
			if dep.Version == "" {
				issues = append(issues, fmt.Sprintf("%s dependency %q is missing a version", class, name))
			}
		}
	}

	validateDeps(m.Dependencies, "production")
	validateDeps(m.DevDependencies, "dev")

	return issues
}
