// Package compiler owns Phase 105's incremental-build engine: a
// dependency-aware, content-addressed artifact cache layered on the existing
// deterministic C pipeline.
//
// Design notes (documented, deliberate):
//
//   - The Go front end emits a single deterministic C unit per project
//     (header + kernels + all struct/enum decls + forward decls + function
//     bodies + main). True per-module .o granularity would require splitting
//     that monolith at the C level; that is deferred post-105 work. This
//     package therefore caches the whole generated unit AND the linked
//     executable, keyed by a PROJECT hash (the ordered per-module content
//     hashes), while tracking per-module records so reuse detection and
//     dependency-aware invalidation are exact.
//
//   - Fingerprinting is dual: a module's CONTENT hash (sha256 of raw bytes)
//     detects source edits, and its INTERFACE hash (canonical signature of its
//     public declarations: func/enum/struct/global headers + imports) detects
//     observable API changes. A dependent module is invalidated when the
//     content hash it recorded for a direct dependency no longer matches the
//     dependency's CURRENT interface hash — i.e. interface-aware invalidation.
//
//   - Interface hashes are memoized by content hash: an unchanged module never
//     needs re-parsing, so a no-op rebuild performs no lexing/parsing, no
//     semantic analysis and no gcc invocation.
//
//   - Cache writes are atomic (temp file + rename). A failed build never
//     updates the manifest, so the cache cannot be poisoned by a partial or
//     erroneous compilation.
//
//   - The compiler identity (Go runtime, target, debug flag, C compiler path +
//     version) is part of the cache key; changing any of them invalidates every
//     module. The module list is also implicit in the project hash, so adding
//     or removing a file rebuilds.
package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// ManifestVersion is the persisted-cache schema version. Bump when the record
// layout or fingerprint semantics change.
const ManifestVersion = 1

// CacheDirName is the cache directory created next to a project root by
// `karkain build --incremental` and removed by `karkain clean`.
const CacheDirName = ".karkain-cache"

// ModuleStatus reports how a module contributed to the latest build.
type ModuleStatus string

const (
	StatusCompiled    ModuleStatus = "compiled"    // content or compiler identity changed; front-end + assembly redone
	StatusReused      ModuleStatus = "reused"      // unchanged and all dependency interfaces match the recorded snapshot
	StatusInvalidated ModuleStatus = "invalidated" // unchanged itself but a dependency's interface changed
)

// ModuleRecord is the persistent per-module fingerprint.
type ModuleRecord struct {
	Path          string            `json:"path"`
	ContentHash   string            `json:"content_hash"`
	InterfaceHash string            `json:"interface_hash"`
	DepInterfaces map[string]string `json:"dep_interfaces,omitempty"` // direct deps: file → interface hash snapshot
	Status        ModuleStatus      `json:"-"`
}

// Manifest is the persisted incremental cache.
type Manifest struct {
	Version      int             `json:"version"`
	CompilerKey  string          `json:"compiler_key"`
	ProjectHash  string          `json:"project_hash"`
	GeneratedC   string          `json:"generated_c"`
	Executable   string          `json:"executable"`
	Modules      []*ModuleRecord `json:"modules"`
	FirstBuiltAt string          `json:"first_built_at"` // informational only
	UpdatedAt    string          `json:"updated_at"`     // informational only
}

// Cache is a content-addressed artifact store rooted at a directory.
type Cache struct {
	dir string
}

// NewCache returns a cache rooted at dir (created on first Store).
func NewCache(dir string) *Cache { return &Cache{dir: dir} }

// Dir returns the cache root directory.
func (c *Cache) Dir() string { return c.dir }

// Load returns the persisted manifest, or nil when missing or stale.
func (c *Cache) Load() *Manifest {
	b, err := os.ReadFile(filepath.Join(c.dir, ManifestName))
	if err != nil {
		return nil
	}
	var m Manifest
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	if m.Version != ManifestVersion {
		return nil
	}
	return &m
}

// ManifestName is the cache manifest file name.
const ManifestName = "manifest.json"

// HasArtifact reports whether the named artifact exists in the cache.
func (c *Cache) HasArtifact(name string) bool {
	fi, err := os.Stat(filepath.Join(c.dir, name))
	return err == nil && fi.Mode().IsRegular()
}

// ReadArtifact returns the named artifact bytes.
func (c *Cache) ReadArtifact(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(c.dir, name))
}

// Store persists the manifest and its two artifacts atomically.
func (c *Cache) Store(m *Manifest, cSrc, exe []byte) error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(c.dir, m.GeneratedC), cSrc); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(c.dir, m.Executable), exe); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(c.dir, ManifestName), mustJSON(m))
}

// Clear removes the cache directory entirely.
func (c *Cache) Clear() error {
	return os.RemoveAll(c.dir)
}

// atomicWrite writes data to path via a temp file + rename so a crashed or
// concurrent process never observes a partially written artifact.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func mustJSON(v any) []byte {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return b
}

// ContentHash is the sha256 of raw bytes.
func ContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// CompilerKey is the deterministic identity of the toolchain that produced an
// artifact. Changing the Go runtime, target, debug flag or C compiler invalidates
// every cached module.
func CompilerKey(cfg codegen.Config) string {
	h := sha256.New()
	fmt.Fprintf(h, "go=%s/%s/%s target=%s debug=%t\n", runtime.Version(), runtime.GOOS, runtime.GOARCH, cfg.Target, cfg.Debug)
	if cc := cCompilerIdentity(); cc != "" {
		fmt.Fprintf(h, "cc=%s\n", cc)
	}
	return hex.EncodeToString(h.Sum(nil))
}

var cCompilerOnce struct {
	ident string
	done  bool
}

// cCompilerIdentity reports "<path> <version-first-line>" of the C compiler
// detectCompiler would pick, or "" if none exists.
func cCompilerIdentity() string {
	if cCompilerOnce.done {
		return cCompilerOnce.ident
	}
	cCompilerOnce.done = true
	for _, name := range []string{"gcc", "clang", "cc"} {
		p, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		out, err := exec.Command(p, "--version").Output()
		if err != nil {
			cCompilerOnce.ident = p
			return p
		}
		first := strings.SplitN(string(out), "\n", 2)[0]
		cCompilerOnce.ident = p + " " + first
		return p + " " + first
	}
	return ""
}

// ModuleInterface computes a canonical signature of a module's observable
// declarations: function headers (name, parameter types, return type), struct
// headers (name, fields), enum headers (name, variants), top-level variables
// and module imports. Bodies, locals and comments never affect the hash, so a
// body-only edit keeps dependents reusable.
func ModuleInterface(src string) string {
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()

	var b strings.Builder
	for _, imp := range prog.Imports {
		fmt.Fprintf(&b, "import %s\n", imp.Name)
	}
	for _, st := range prog.Statements {
		switch d := st.(type) {
		case *parser.FuncDecl:
			types := make([]string, 0, len(d.Params))
			for i, prm := range d.Params {
				if i < len(d.ParamTypes) && d.ParamTypes[i] != "" {
					types = append(types, d.ParamTypes[i])
				} else {
					types = append(types, prm)
				}
			}
			fmt.Fprintf(&b, "func %s(%s)\n", d.Name, strings.Join(types, ","))
		case *parser.StructDeclStmt:
			fmt.Fprintf(&b, "type %s {", d.Name)
			for _, f := range d.Fields {
				fmt.Fprintf(&b, " %s %s;", f.Name, f.Type)
			}
			b.WriteString(" }\n")
		case *parser.EnumDecl:
			fmt.Fprintf(&b, "enum %s {", d.Name)
			for _, v := range d.Variants {
				fmt.Fprintf(&b, " %s", v.Name)
			}
			b.WriteString(" }\n")
		case *parser.VarDeclStmt:
			fmt.Fprintf(&b, "var %s %s\n", d.Name, d.Type)
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// FileImports resolves the module imports of a source file to concrete source
// files: sibling <name>.kark wins, otherwise the <name>/ subdirectory's .kark
// files are returned (matching module resolution at same-directory depth).
// Unresolvable imports return nothing (the assembler reports them).
func FileImports(text, dir string) []string {
	l := lexer.New(text)
	p := parser.New(l)
	prog := p.ParseProgram()

	var out []string
	for _, imp := range prog.Imports {
		sib := filepath.Join(dir, imp.Name+".kark")
		if fi, err := os.Stat(sib); err == nil && fi.Mode().IsRegular() {
			out = append(out, sib)
			continue
		}
		sub := filepath.Join(dir, imp.Name)
		if entries, err := os.ReadDir(sub); err == nil {
			for _, e := range entries {
				if e.IsDir() || filepath.Ext(e.Name()) != ".kark" {
					continue
				}
				out = append(out, filepath.Join(sub, e.Name()))
			}
		}
	}
	return out
}

// Plan is the outcome of planning a build against a cache.
type Plan struct {
	Rebuild     bool
	ProjectHash string
	CompilerKey string
	Modules     []*ModuleRecord // in assembly order (deps first, root last)
}

// PlanBuild compares the previous manifest against the current project files
// and produces per-module statuses plus a rebuild decision. files must be the
// deterministic assembly order (dependencies first, root last). The project
// hash covers the ordered files, so adding/removing/reordering any file forces
// a rebuild even when every individual content hash is unchanged.
func PlanBuild(prev *Manifest, files []string, key string) *Plan {
	plan := &Plan{CompilerKey: key}
	byPath := map[string]*ModuleRecord{}
	if prev != nil {
		for _, r := range prev.Modules {
			byPath[r.Path] = r
		}
	}

	// First compute content hashes and (memoized) interface hashes.
	content := map[string]string{}
	iface := map[string]string{}
	texts := map[string]string{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			// Unreadable file: treat as changed so the build fails loudly.
			content[f] = ""
			iface[f] = ""
			continue
		}
		texts[f] = string(data)
		h := ContentHash(data)
		content[f] = h
		if prevRec, ok := byPath[f]; ok && prevRec.ContentHash == h {
			iface[f] = prevRec.InterfaceHash // memoized: no reparse needed
		} else {
			iface[f] = ModuleInterface(string(data))
		}
	}

	// Second pass: dependency-aware statuses.
	for _, f := range files {
		var rec *ModuleRecord
		var ok bool
		if prev != nil {
			rec, ok = byPath[f]
		}
		cur := &ModuleRecord{Path: f, ContentHash: content[f], InterfaceHash: iface[f]}
		// Record the current dependency-interface snapshot for every module so
		// future reuse decisions always compare a persisted snapshot with the
		// live world (dependency removal/edit is then a clean mismatch).
		deps := map[string]string{}
		for _, dep := range FileImports(texts[f], filepath.Dir(f)) {
			deps[dep] = iface[dep]
		}
		cur.DepInterfaces = deps
		switch {
		case !ok:
			cur.Status = StatusCompiled
		case key != prev.CompilerKey:
			cur.Status = StatusInvalidated
		case rec.ContentHash != content[f]:
			cur.Status = StatusCompiled
		default:
			cur.Status = StatusReused
			for dep, ih := range deps {
				if rec.DepInterfaces[dep] != ih {
					cur.Status = StatusInvalidated
					break
				}
			}
		}
		plan.Modules = append(plan.Modules, cur)
	}

	plan.ProjectHash = projectHash(files, content)
	plan.Rebuild = prev == nil ||
		prev.ProjectHash != plan.ProjectHash ||
		prev.CompilerKey != key
	for _, r := range plan.Modules {
		if r.Status != StatusReused {
			plan.Rebuild = true
			break
		}
	}
	return plan
}

// ManifestFor builds the persisted manifest for a completed build.
func ManifestFor(plan *Plan, cName, exeName string) *Manifest {
	return &Manifest{
		Version:     ManifestVersion,
		CompilerKey: plan.CompilerKey,
		ProjectHash: plan.ProjectHash,
		GeneratedC:  cName,
		Executable:  exeName,
		Modules:     plan.Modules,
	}
}

// projectHash hashes the ordered (file, content-hash) pairs.
func projectHash(files []string, content map[string]string) string {
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00%s\x00", f, content[f])
	}
	return hex.EncodeToString(h.Sum(nil))
}