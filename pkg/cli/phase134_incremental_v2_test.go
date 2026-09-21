package cli

// Phase 134 — Incremental Compilation v2 gate.
//
// Phase 105 cached one monolithic C unit + executable. Phase 134 splits
// emission into per-module translation units (shared runtime TU + one TU per
// source module, linked together) so only changed modules recompile.
//
// This gate pins the v2 contract:
//  1. Object naming: deterministic, filesystem-safe, content-keyed.
//  2. Runtime identity: sensitive to header text, flags and compiler key.
//  3. Artifact store: atomic multi-file persist + stale-version rejection.
//  4. Split emission structure: header guard + extern decls, single runtime
//     definitions, root TU owns main, module TUs own their functions.
//  5. End-to-end: split-flow clean build prints the phase105 golden,
//     no-op rebuild reuses everything, a body-only leaf edit keeps the
//     golden while recompiling only the leaf, and the cache holds
//     runtime.o + per-module .o files under manifest v2.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karkain/pkg/codegen"
	"karkain/pkg/compiler"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

func TestPhase134_ObjectNaming(t *testing.T) {
	h1 := compiler.ContentHash([]byte("public func twice(n) { return n * 2 }"))
	h2 := compiler.ContentHash([]byte("public func twice(n) { return n * 3 }"))

	a := compiler.ObjectNameFor("math.kark", h1)
	b := compiler.ObjectNameFor("math.kark", h1)
	if a != b {
		t.Fatalf("ObjectNameFor not deterministic: %q vs %q", a, b)
	}
	if !strings.HasSuffix(a, ".o") || !strings.HasPrefix(a, "math_") {
		t.Fatalf("ObjectNameFor(%q) = %q, want math_<hash>.o", "math.kark", a)
	}
	if c := compiler.ObjectNameFor("math.kark", h2); c == a {
		t.Fatalf("ObjectNameFor ignores content hash: %q == %q", c, a)
	}
	// Sanitization: separators and odd characters must not leak into the name.
	weird := compiler.ObjectNameFor(filepath.Join("sub dir", "my-mod.kark"), h1)
	for _, r := range strings.TrimSuffix(weird, ".o") {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
			t.Fatalf("ObjectNameFor(%q) = %q contains unsafe rune %q", "my-mod.kark", weird, r)
		}
	}
	if got := compiler.ObjectNameFor(".kark", h1); !strings.HasPrefix(got, "module_") {
		t.Fatalf("ObjectNameFor empty base = %q, want module_ prefix", got)
	}
}

func TestPhase134_RuntimeKey(t *testing.T) {
	key := "compiler-identity"
	hdr := "int _karkain_gargc = 0;\nValue make_int(int x);\n"
	base := compiler.RuntimeKey(key, hdr, false, false, false)

	if again := compiler.RuntimeKey(key, hdr, false, false, false); again != base {
		t.Fatalf("RuntimeKey not deterministic: %q vs %q", again, base)
	}
	if same := compiler.RuntimeKey(key, hdr+"// new helper\n", false, false, false); same == base {
		t.Fatalf("RuntimeKey ignores header text change")
	}
	for _, flags := range [][3]bool{{true, false, false}, {false, true, false}, {false, false, true}} {
		if got := compiler.RuntimeKey(key, hdr, flags[0], flags[1], flags[2]); got == base {
			t.Fatalf("RuntimeKey ignores flags %v", flags)
		}
	}
	if got := compiler.RuntimeKey("other-compiler", hdr, false, false, false); got == base {
		t.Fatalf("RuntimeKey ignores compiler key change")
	}
}

func TestPhase134_StoreArtifactsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := compiler.NewCache(dir)

	plan := &compiler.Plan{
		Rebuild:     true,
		ProjectHash: "proj",
		CompilerKey: "key",
		Modules: []*compiler.ModuleRecord{
			{Path: "math.kark", ContentHash: "aa", InterfaceHash: "bb", Status: compiler.StatusCompiled},
		},
	}
	m := compiler.ManifestFor(plan, "proj.c", "proj.exe")
	m.Version = compiler.ManifestVersion
	m.RuntimeKey = "rt"
	m.RuntimeObj = codegen.RuntimeObjectName
	m.Objects = map[string]string{"math.kark": "math_aa.o"}

	files := map[string][]byte{
		"proj.c":                  []byte("c-src"),
		"proj.exe":                []byte("exe"),
		codegen.RuntimeObjectName: []byte("obj"),
		"math_aa.o":               []byte("obj"),
	}
	if err := c.StoreArtifacts(m, files); err != nil {
		t.Fatalf("StoreArtifacts: %v", err)
	}
	for name := range files {
		if !c.HasArtifact(name) {
			t.Fatalf("missing cached artifact %s", name)
		}
	}
	loaded := c.Load()
	if loaded == nil {
		t.Fatalf("Load returned nil after StoreArtifacts")
	}
	if loaded.Version != compiler.ManifestVersion || loaded.RuntimeKey != "rt" || len(loaded.Objects) != 1 {
		t.Fatalf("Load round-trip mismatch: %+v", loaded)
	}

	// Stale schema versions are rejected (never served as fresh).
	raw, _ := json.Marshal(map[string]any{"version": 1, "modules": []any{}})
	if err := os.WriteFile(filepath.Join(dir, compiler.ManifestName), raw, 0o644); err != nil {
		t.Fatalf("write stale manifest: %v", err)
	}
	if got := c.Load(); got != nil {
		t.Fatalf("Load served stale manifest version: %+v", got)
	}
}

// splitTestProject stages a two-module program and returns the assembled
// *parser.Program plus the deterministic file order (dep first, root last).
func splitTestProject(t *testing.T) (string, []string, *parser.Program) {
	t.Helper()
	dir := t.TempDir()
	math := filepath.Join(dir, "math.kark")
	main := filepath.Join(dir, "main.kark")
	mathSrc := "public func twice(n) {\n    return n * 2\n}\n"
	mainSrc := "import math\n\nfunc main() {\n    print(math.twice(21))\n}\n"
	if err := os.WriteFile(math, []byte(mathSrc), 0o644); err != nil {
		t.Fatalf("write math: %v", err)
	}
	if err := os.WriteFile(main, []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}
	files := []string{math, main}
	var stmts []parser.Node
	var imports []*parser.ModuleImport
	for _, f := range files {
		data, _ := os.ReadFile(f)
		p := parser.New(lexer.New(string(data)))
		prog := p.ParseProgram()
		if len(p.Errors) > 0 {
			t.Fatalf("parse %s: %s", f, p.Errors[0])
		}
		stmts = append(stmts, prog.Statements...)
		imports = append(imports, prog.Imports...)
	}
	return main, files, &parser.Program{Statements: stmts, Imports: imports}
}

func TestPhase134_SplitEmissionStructure(t *testing.T) {
	root, files, prog := splitTestProject(t)
	split, err := codegen.GenerateSplit(codegen.Config{}, prog, files, root)
	if err != nil {
		t.Fatalf("GenerateSplit: %v", err)
	}
	if split.UsesConcurrency || split.Profiling {
		t.Fatalf("plain project flagged concurrency=%v profiling=%v", split.UsesConcurrency, split.Profiling)
	}
	if !strings.Contains(split.Header, "#ifndef KARKAIN_RUNTIME_H") {
		t.Fatalf("header missing include guard")
	}
	if !strings.Contains(split.Header, "extern int _karkain_gargc;") {
		t.Fatalf("header missing extern shared-global decl")
	}
	if strings.Contains(split.Header, "static int _karkain_gargc") {
		t.Fatalf("header still defines shared mutable global")
	}
	if !strings.Contains(split.RuntimeSrc, `#include "`+codegen.RuntimeHeaderName+`"`) {
		t.Fatalf("runtime source missing header include")
	}
	if !strings.Contains(split.RuntimeSrc, "int _karkain_gargc = 0;") {
		t.Fatalf("runtime source missing single global definition")
	}
	// NOTE: `main` keeps its canonical C name (userFuncC: main/getArgs are
	// unmangled); every other user function is karkain_user_<name>. Shared
	// forward prototypes appear in every TU, so definition ownership is
	// pinned by occurrence counts: prototype (1) vs prototype+definition (2+).
	if colZeroDefs(split.RootTU, "main") != 1 {
		t.Fatalf("root TU main definitions = %d, want 1", colZeroDefs(split.RootTU, "main"))
	}
	if !strings.Contains(split.RootTU, `#include "`+codegen.RuntimeHeaderName+`"`) {
		t.Fatalf("root TU missing runtime header include")
	}
	mathTU, ok := split.ModuleTUs[files[0]]
	if !ok {
		t.Fatalf("no module TU for math.kark (keys: %v)", keysOf(split.ModuleTUs))
	}
	if colZeroDefs(mathTU, "karkain_user_twice") != 1 {
		t.Fatalf("math TU twice definitions = %d, want 1", colZeroDefs(mathTU, "karkain_user_twice"))
	}
	if colZeroDefs(mathTU, "main") != 0 {
		t.Fatalf("math TU leaks root main definition")
	}
	if colZeroDefs(split.RootTU, "karkain_user_twice") != 0 {
		t.Fatalf("root TU duplicates math definition")
	}
}

// colZeroDefs counts column-zero C function definition lines for sym:
// `<...> sym(...) {` with no leading whitespace (prototypes end in `;`,
// call sites are indented, so only true definitions match).
func colZeroDefs(tu, sym string) int {
	n := 0
	for _, line := range strings.Split(tu, "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
			continue
		}
		if !strings.Contains(line, sym+"(") || !strings.HasSuffix(strings.TrimSpace(line), "{") {
			continue
		}
		n++
	}
	return n
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func readManifest(t *testing.T, cacheDir string) *compiler.Manifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(cacheDir, compiler.ManifestName))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m compiler.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return &m
}

func TestPhase134_SplitFlowE2E(t *testing.T) {
	skipIfNoCompiler(t)

	dir := t.TempDir()
	copyTree(t, filepath.Join("..", "..", "examples", phase105ExampleDir), dir)
	cacheDir := filepath.Join(dir, compiler.CacheDirName)
	exePath := filepath.Join(dir, "main134.exe")
	const golden = "41\n42\nHello karkain\n"

	// 1. Clean split-flow build: correct output, manifest v2 with runtime key.
	clean := runIncrementalBuild(t, dir, cacheDir, exePath, nil)
	if out := runExe(t, exePath); out != golden {
		t.Fatalf("clean split build output = %q, want %q", out, golden)
	}
	m := readManifest(t, cacheDir)
	if m.Version != compiler.ManifestVersion {
		t.Fatalf("manifest version = %d, want %d", m.Version, compiler.ManifestVersion)
	}
	if m.RuntimeKey == "" {
		t.Fatalf("manifest missing phase-134 runtime key")
	}
	if m.RuntimeObj != codegen.RuntimeObjectName {
		t.Fatalf("manifest runtime_obj = %q, want %q", m.RuntimeObj, codegen.RuntimeObjectName)
	}
	if len(m.Objects) != 3 {
		t.Fatalf("manifest objects = %d entries, want 3 (math+strings+main)", len(m.Objects))
	}
	for _, name := range m.Objects {
		if _, err := os.Stat(filepath.Join(cacheDir, name)); err != nil {
			t.Fatalf("missing cached object %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(cacheDir, codegen.RuntimeObjectName)); err != nil {
		t.Fatalf("missing cached runtime object: %v", err)
	}

	// 2. No-op rebuild: everything reused, output identical, no gcc storm.
	start := time.Now()
	res := BuildCommandIncremental(filepath.Join(dir, "main.kark"), exePath, codegen.Config{}, false, cacheDir)
	noopMs := time.Since(start).Milliseconds()
	if res.ExitCode != ExitSuccess {
		t.Fatalf("no-op rebuild failed: %q (exit %d)", res.Message, res.ExitCode)
	}
	if !strings.Contains(res.Message, "3 reused") {
		t.Errorf("no-op message %q should report 3 modules reused", res.Message)
	}
	if out := runExe(t, exePath); out != golden {
		t.Fatalf("no-op output changed: %q", out)
	}
	t.Logf("phase134 perf: clean=%dms noop=%dms", clean.ms, noopMs)
	if noopMs > 15000 {
		t.Errorf("no-op rebuild took %dms, want < 15000ms (should skip gcc entirely)", noopMs)
	}

	// 3. Body-only leaf edit (interface unchanged): golden preserved,
	// only the leaf recompiles, dependents stay reusable.
	stringsPath := filepath.Join(dir, "strings.kark")
	leaf, err := os.ReadFile(stringsPath)
	if err != nil {
		t.Fatalf("read strings.kark: %v", err)
	}
	orig := string(leaf)
	edited := strings.Replace(orig, `return "Hello " + name`, "let h = \"Hello \" + name\n    return h", 1)
	if edited == orig {
		t.Fatalf("leaf edit did not apply (fixture changed?)")
	}
	if err := os.WriteFile(stringsPath, []byte(edited), 0o644); err != nil {
		t.Fatalf("write strings.kark: %v", err)
	}
	res = BuildCommandIncremental(filepath.Join(dir, "main.kark"), exePath, codegen.Config{}, false, cacheDir)
	if res.ExitCode != ExitSuccess {
		t.Fatalf("leaf rebuild failed: %q (exit %d)", res.Message, res.ExitCode)
	}
	if !strings.Contains(res.Message, "1 compiled") {
		t.Errorf("leaf message %q should report exactly 1 module compiled", res.Message)
	}
	if out := runExe(t, exePath); out != golden {
		t.Fatalf("leaf rebuild output = %q, want %q", out, golden)
	}

	// 4. Cache purge (the `karkain clean` contract): everything rebuilds.
	if err := compiler.NewCache(cacheDir).Clear(); err != nil {
		t.Fatalf("cache clear: %v", err)
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Fatalf("cache dir still exists after Clear")
	}
	clean2 := runIncrementalBuild(t, dir, cacheDir, exePath, nil)
	if out := runExe(t, exePath); out != golden {
		t.Fatalf("post-clean output = %q, want %q", out, golden)
	}
	if !strings.Contains(clean2.message, "3 compiled") {
		t.Errorf("post-clean message %q should report 3 modules compiled", clean2.message)
	}
}
