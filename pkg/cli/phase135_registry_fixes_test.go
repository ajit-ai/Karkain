package cli

// Phase 135 remediation regressions (items 1–3): project-relative registry
// resolution from subdirectories, bare `--registry` usage errors, and
// conflicting `add` options. Item 4 (internal-registry exclusion) is covered
// at pm level in pkg/pm/local_registry_fixes_test.go. All registries and
// projects live in temp dirs; the child binary gets its environment only
// from runBinEnv (never process-global).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exitCodeOf is shared with phase111_cross_compile_test.go (same package).

func mkdirSub(t *testing.T, dir string) string {
	t.Helper()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	return sub
}

func initReg(t *testing.T, bin, reg string) {
	t.Helper()
	if out, err := runBin(t, bin, filepath.Dir(reg), "pkg", "registry", "init", reg); err != nil {
		t.Fatalf("registry init %s failed: %v\n%s", reg, err, out)
	}
}

func TestPhase135_RelativeRegistryPaths(t *testing.T) {
	skipIfNoCompiler(t)
	bin := buildKarkain(t)
	root := t.TempDir()

	// Library source project (published into each consumer-local registry).
	if out, err := runBin(t, bin, root, "new", "superhello"); err != nil {
		t.Fatalf("new superhello failed: %v\n%s", err, out)
	}
	libDir := filepath.Join(root, "superhello")
	writeLibSource(t, libDir)

	// Consumer with a project-local registry: project/reg + project/sub.
	if out, err := runBin(t, bin, root, "new", "app"); err != nil {
		t.Fatalf("new app failed: %v\n%s", err, out)
	}
	appDir := filepath.Join(root, "app")
	appReg := filepath.Join(appDir, "reg")
	initReg(t, bin, appReg)
	out, err := runBin(t, bin, libDir, "pkg", "publish", "--registry", appReg)
	if err != nil {
		t.Fatalf("publish into app reg failed: %v\n%s", err, out)
	}
	if out, err := runBin(t, bin, appDir, "add", "superhello", "0.1.0", "--registry", "reg"); err != nil {
		t.Fatalf("add with relative registry failed: %v\n%s", err, out)
	}
	manifestRaw, _ := os.ReadFile(filepath.Join(appDir, "karkain.toml"))
	if !strings.Contains(string(manifestRaw), `url = "reg"`) {
		t.Fatalf("manifest should keep the relative url, got:\n%s", manifestRaw)
	}

	// From a project subdirectory the relative URL must anchor to the
	// project dir (pre-fix it resolved against the subdirectory and failed).
	sub := mkdirSub(t, appDir)
	out, err = runBin(t, bin, sub, "fetch")
	if err != nil {
		t.Fatalf("fetch from subdir with relative dep url failed: %v\n%s", err, out)
	}
	if _, lerr := os.Stat(filepath.Join(appDir, "karkain.lock")); lerr != nil {
		t.Fatalf("fetch from subdir should write the project lockfile: %v", lerr)
	}

	// A second consumer exercises the --registry flag form from a subdir.
	if out, err := runBin(t, bin, root, "new", "app2"); err != nil {
		t.Fatalf("new app2 failed: %v\n%s", err, out)
	}
	app2Dir := filepath.Join(root, "app2")
	app2Reg := filepath.Join(app2Dir, "reg")
	initReg(t, bin, app2Reg)
	out, err = runBin(t, bin, libDir, "pkg", "publish", "--registry", app2Reg)
	if err != nil {
		t.Fatalf("publish into app2 reg failed: %v\n%s", err, out)
	}
	if out, err := runBin(t, bin, app2Dir, "add", "superhello", "0.1.0"); err != nil {
		t.Fatalf("add without registry failed: %v\n%s", err, out)
	}
	sub2 := mkdirSub(t, app2Dir)
	out, err = runBin(t, bin, sub2, "fetch", "--registry", "./reg")
	if err != nil {
		t.Fatalf("fetch --registry ./reg from subdir failed: %v\n%s", err, out)
	}

	// update --registry ./reg from a subdir resolves the same way.
	out, err = runBin(t, bin, sub2, "update", "superhello", "--registry", "./reg")
	if err != nil {
		t.Fatalf("update --registry ./reg from subdir failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Updated superhello") {
		t.Fatalf("update should confirm, got:\n%s", out)
	}

	// Publish with a relative registry from a subdirectory lands in the project.
	if out, err := runBin(t, bin, root, "new", "lib2"); err != nil {
		t.Fatalf("new lib2 failed: %v\n%s", err, out)
	}
	lib2Dir := filepath.Join(root, "lib2")
	writeLibSource(t, lib2Dir)
	myReg := filepath.Join(lib2Dir, "my-reg")
	if out, err := runBin(t, bin, lib2Dir, "pkg", "registry", "init", myReg); err != nil {
		t.Fatalf("init my-reg failed: %v\n%s", err, out)
	}
	sub3 := mkdirSub(t, lib2Dir)
	out, err = runBin(t, bin, sub3, "pkg", "publish", "--registry", "./my-reg")
	if err != nil {
		t.Fatalf("publish --registry ./my-reg from subdir failed: %v\n%s", err, out)
	}
	if _, serr := os.Stat(filepath.Join(myReg, "index", "lib2")); serr != nil {
		t.Fatalf("relative publish should land in the project registry: %v", serr)
	}
	if _, serr := os.Stat(filepath.Join(sub3, "my-reg")); !os.IsNotExist(serr) {
		t.Fatalf("relative publish must not resolve against the subdirectory")
	}
}

func TestPhase135_RegistryFlagErrors(t *testing.T) {
	bin := buildKarkain(t)
	root := t.TempDir()

	if out, err := runBin(t, bin, root, "new", "app"); err != nil {
		t.Fatalf("new app failed: %v\n%s", err, out)
	}
	appDir := filepath.Join(root, "app")

	// Bare --registry is a usage error (exit 2), never a silent fallthrough.
	out, err := runBin(t, bin, appDir, "pkg", "publish", "--registry")
	if err == nil {
		t.Fatalf("bare --registry should fail")
	}
	if exitCodeOf(t, err) != 2 {
		t.Fatalf("bare --registry exit = %d, want 2 (usage)", exitCodeOf(t, err))
	}
	if !strings.Contains(out, "--registry") || !strings.Contains(out, "Usage") {
		t.Fatalf("bare --registry should explain usage, got:\n%s", out)
	}

	out, err = runBin(t, bin, appDir, "fetch", "--registry")
	if err == nil || exitCodeOf(t, err) != 2 {
		t.Fatalf("bare --registry on fetch should be a usage error, got exit %d:\n%s", exitCodeOf(t, err), out)
	}

	// Conflicting add options are usage errors, not last-wins.
	for _, args := range [][]string{
		{"add", "foo", "--registry", "./reg", "--source", "local"},
		{"add", "foo", "--registry", "./reg", "--url", "./x"},
	} {
		out, err := runBin(t, bin, appDir, args...)
		if err == nil {
			t.Fatalf("%v should fail", args)
		}
		if exitCodeOf(t, err) != 2 {
			t.Fatalf("%v exit = %d, want 2 (usage)", args, exitCodeOf(t, err))
		}
		if !strings.Contains(out, "cannot be combined") {
			t.Fatalf("%v should say cannot be combined, got:\n%s", args, out)
		}
	}

	// Each valid form on its own still records correctly.
	if out, err := runBin(t, bin, appDir, "add", "foo", "--registry", "./reg"); err != nil {
		t.Fatalf("valid --registry add failed: %v\n%s", err, out)
	}
	manifestRaw, _ := os.ReadFile(filepath.Join(appDir, "karkain.toml"))
	if !strings.Contains(string(manifestRaw), "foo") || !strings.Contains(string(manifestRaw), `url = "./reg"`) {
		t.Fatalf("valid --registry add should record the dep, got:\n%s", manifestRaw)
	}
}
