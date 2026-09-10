package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

// project writes a balanced three-module incremental project into a temp dir:
// main imports math; strings is independent. files returns the deterministic
// assembly order (deps first, root last).
type project struct {
	dir      string
	main     string
	math     string
	strings  string
	all      []string
}

func newProject(t *testing.T) *project {
	t.Helper()
	dir := t.TempDir()
	p := &project{
		dir:     dir,
		main:    filepath.Join(dir, "main.kark"),
		math:    filepath.Join(dir, "math.kark"),
		strings: filepath.Join(dir, "strings.kark"),
	}
	p.write(t, p.main, `import math

func main() {
    print(math.twice(3))
}
`)
	p.write(t, p.math, `public func twice(n) {
    return n * 2
}
`)
	p.write(t, p.strings, `public func greeting(n) {
    return "hi " + n
}
`)
	p.all = []string{p.math, p.strings, p.main}
	return p
}

func (p *project) write(t *testing.T, path, src string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func (p *project) mathRewriteBody(t *testing.T) {
	t.Helper()
	p.write(t, p.math, `public func twice(n) {
    let r = n * 2
    return r
}
`)
}

func (p *project) mathRewriteInterface(t *testing.T) {
	t.Helper()
	p.write(t, p.math, `public func twice(n, m) {
    return n * m
}
`)
}

func (p *project) modifyStrings(t *testing.T) {
	t.Helper()
	p.write(t, p.strings, `public func greeting(n) {
    return "hello " + n
}
`)
}

func statuses(plan *Plan) map[string]ModuleStatus {
	out := map[string]ModuleStatus{}
	for _, m := range plan.Modules {
		out[m.Path] = m.Status
	}
	return out
}

func TestIncremental_CleanBuildCompilesUnit(t *testing.T) {
	p := newProject(t)
	plan := PlanBuild(nil, p.all, "key")
	if len(plan.Modules) != 3 {
		t.Fatalf("expected 3 module records, got %d", len(plan.Modules))
	}
	if !plan.Rebuild {
		t.Error("fresh cache must trigger a rebuild")
	}
	if plan.ProjectHash == "" {
		t.Error("project hash must be non-empty")
	}
	for _, m := range plan.Modules {
		if m.Status != StatusCompiled {
			t.Errorf("%s status = %s, want compiled", m.Path, m.Status)
		}
		if m.ContentHash == "" || m.InterfaceHash == "" {
			t.Errorf("%s missing fingerprints", m.Path)
		}
	}
	// Deterministic: planning again against the same files yields the same hash.
	again := PlanBuild(nil, p.all, "key")
	if again.ProjectHash != plan.ProjectHash {
		t.Error("project hash must be deterministic")
	}
}

func TestIncremental_NoOpRebuildReusesEverything(t *testing.T) {
	p := newProject(t)
	plan := PlanBuild(nil, p.all, "key")
	manifest := ManifestFor(plan, plan.ProjectHash+".c", plan.ProjectHash+".exe")

	noop := PlanBuild(manifest, p.all, "key")
	if noop.Rebuild {
		t.Error("no-op rebuild must not require regeneration")
	}
	if noop.ProjectHash != manifest.ProjectHash {
		t.Error("project hash must be stable across no-op builds")
	}
	for _, m := range noop.Modules {
		if m.Status != StatusReused {
			t.Errorf("%s status = %s, want reused", m.Path, m.Status)
		}
	}
}

func TestIncremental_LeafChangeRebuildsOnlyLeaf(t *testing.T) {
	p := newProject(t)
	plan := PlanBuild(nil, p.all, "key")
	manifest := ManifestFor(plan, "c", "e")

	p.mathRewriteBody(t) // interface unchanged, content changed

	next := PlanBuild(manifest, p.all, "key")
	if !next.Rebuild {
		t.Error("leaf change must trigger regeneration")
	}
	st := statuses(next)
	if st[p.math] != StatusCompiled {
		t.Errorf("math status = %s, want compiled", st[p.math])
	}
	if st[p.main] != StatusReused {
		t.Errorf("main status = %s, want reused (interface unchanged)", st[p.main])
	}
	if st[p.strings] != StatusReused {
		t.Errorf("strings status = %s, want reused", st[p.strings])
	}
}

func TestIncremental_InterfaceChangeInvalidatesDependents(t *testing.T) {
	p := newProject(t)
	plan := PlanBuild(nil, p.all, "key")
	manifest := ManifestFor(plan, "c", "e")

	p.mathRewriteInterface(t) // signature change → observable API change

	next := PlanBuild(manifest, p.all, "key")
	if !next.Rebuild {
		t.Error("interface change must trigger regeneration")
	}
	st := statuses(next)
	if st[p.math] != StatusCompiled {
		t.Errorf("math status = %s, want compiled", st[p.math])
	}
	if st[p.main] != StatusInvalidated {
		t.Errorf("main status = %s, want invalidated (deps interface changed)", st[p.main])
	}
}

func TestIncremental_UnrelatedModuleChange(t *testing.T) {
	p := newProject(t)
	plan := PlanBuild(nil, p.all, "key")
	manifest := ManifestFor(plan, "c", "e")

	p.modifyStrings(t) // strings has no dependents and main does not import it

	next := PlanBuild(manifest, p.all, "key")
	if !next.Rebuild {
		t.Error("project hash changed, regeneration required")
	}
	st := statuses(next)
	if st[p.strings] != StatusCompiled {
		t.Errorf("strings status = %s, want compiled", st[p.strings])
	}
	if st[p.math] != StatusReused {
		t.Errorf("math status = %s, want reused", st[p.math])
	}
	if st[p.main] != StatusReused {
		t.Errorf("main status = %s, want reused", st[p.main])
	}
}

func TestIncremental_InterfaceHashIgnoresBodies(t *testing.T) {
	before := ModuleInterface(`public func twice(n) {
    return n * 2
}
`)
	after := ModuleInterface(`public func twice(n) {
    let r = n * 2
    return r
}
`)
	if before != after {
		t.Error("interface hash must ignore function bodies")
	}
	changed := ModuleInterface(`public func twice(a, b) {
    return a * b
}
`)
	if before == changed {
		t.Error("interface hash must change when signatures change")
	}
}

func TestIncremental_ConfigChangeInvalidatesAll(t *testing.T) {
	p := newProject(t)
	plan := PlanBuild(nil, p.all, "key-v1")
	manifest := ManifestFor(plan, "c", "e")

	next := PlanBuild(manifest, p.all, "key-v2")
	if !next.Rebuild {
		t.Error("compiler identity change must trigger regeneration")
	}
	for _, m := range next.Modules {
		if m.Status != StatusInvalidated {
			t.Errorf("%s status = %s, want invalidated", m.Path, m.Status)
		}
	}
}

func TestIncremental_FailedBuildDoesNotPoisonCache(t *testing.T) {
	p := newProject(t)
	plan := PlanBuild(nil, p.all, "key")
	manifest := ManifestFor(plan, "c", "e")

	// A build that fails mid-way (e.g. gcc error) never reaches Cache.Store, so
	// the manifest stays the last GOOD one. Simulate the failure by planning
	// against a broken source but discarding the result (as the caller does
	// when GenerateAndCompile errors).
	p.write(t, p.math, `public func twice(n) {
    return n * 2
`)
	_ = PlanBuild(manifest, p.all, "key") // caller would return here, no Store

	// Restore the last-good source; the cache must still be perfectly reusable.
	p.mathRewriteBody(t)
	p.write(t, p.math, `public func twice(n) {
    return n * 2
}
`)
	next := PlanBuild(manifest, p.all, "key")
	if next.Rebuild {
		t.Error("restored last-good source should replay cleanly from the cache")
	}
	for _, m := range next.Modules {
		if m.Status != StatusReused {
			t.Errorf("%s status = %s, want reused", m.Path, m.Status)
		}
	}
}

func TestIncremental_CacheStoreLoadRoundTrip(t *testing.T) {
	p := newProject(t)
	dir := filepath.Join(p.dir, ".karkain-cache")
	c := NewCache(dir)

	plan := PlanBuild(nil, p.all, "key")
	m := ManifestFor(plan, "abc123.c", "abc123.exe")
	if err := c.Store(m, []byte("int main(void){return 0;}"), []byte("MZ-bin")); err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	got := c.Load()
	if got == nil {
		t.Fatal("Load returned nil after Store")
	}
	if got.ProjectHash != m.ProjectHash || got.CompilerKey != "key" {
		t.Errorf("manifest round-trip mismatch: %+v", got)
	}
	if !c.HasArtifact("abc123.c") || !c.HasArtifact("abc123.exe") {
		t.Error("artifacts missing after Store")
	}
	data, err := c.ReadArtifact("abc123.c")
	if err != nil || string(data) != "int main(void){return 0;}" {
		t.Errorf("artifact bytes did not round-trip: %v %q", err, data)
	}
}

func TestIncremental_ProjectHashChangesWhenFileAdded(t *testing.T) {
	p := newProject(t)
	plan := PlanBuild(nil, p.all, "key")

	extra := filepath.Join(p.dir, "extra.kark")
	p.write(t, extra, `public func extra() {
    return 7
}
`)
	all := append(append([]string{}, p.all...), extra)
	next := PlanBuild(nil, all, "key")
	if next.ProjectHash == plan.ProjectHash {
		t.Error("adding a file must change the project hash")
	}
	if len(next.Modules) != 4 {
		t.Errorf("expected 4 modules, got %d", len(next.Modules))
	}
}