package main

import (
	"encoding/json"
	"fmt"
	"karkain/pkg/cli"
	"karkain/pkg/codegen"
	"karkain/pkg/lsp"
	kpkg "karkain/pkg/pm"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const versionString = "Karkain Compiler v1.0.0 (%s/%s, LSP Engine & IDE Tooling)\n"

func printVersion() {
	fmt.Printf(versionString, runtime.GOOS, runtime.GOARCH)
}

func printHelp() {
	fmt.Println(`Karkain Programming Language Toolchain

Usage:
  karkain [command] [options] <file.kark>

COMPILER COMMANDS:
  run <file.kark>          Compile and run (default)
  build <file.kark>        Compile to native executable
  transpile <file.kark>    Generate C or other backend output
  check <file.kark>        Validate syntax and semantics
  test <path>              Discover and run *_test.kark files
  test --compile [dir]     Run compile-pass/compile-fail corpus (diagnostics)
  bench <path>             Time bench_-prefixed functions (single run each)
  lint <file.kark>         Full front-end analysis (incl. borrow checker)
  explain <code>           Explain a toolchain error code
  fmt <file.kark>          Canonicalize formatting (--check to verify only)
  clean [path] [--all]     Remove generated artifacts (sources never touched)
  target                   List supported --target values
  config                   Print effective toolchain configuration
  lsp                      Start Language Server Protocol server (stdio)
  language-server          Alias for the LSP server command
  ide info                 Print machine-readable IDE/toolchain contract (JSON)

WORKSPACE COMMANDS:
  workspace list           List members in dependency order
  workspace build          Build all members (dependency order)
  workspace test           Test all members
  workspace check          Validate all members (no binaries produced)
  workspace run            Build and run all member entrypoints
  workspace clean [--all]  Clean generated artifacts from all members
  workspace lint           Full front-end analysis of all members
  workspace graph          Show member dependency graph
  workspace init           Initialize workspace root
  workspace add <path>     Add member package
  workspace remove <path>  Remove member package

PACKAGE MANAGEMENT (top-level):
  karkain init [name]                Create new project (in current dir)
  karkain new <name>                 Create a new project directory
  karkain add <pkg> [version]        Add dependency
  karkain remove <pkg>               Remove dependency
  karkain fetch                      Fetch resolved dependencies (lockfile)
  karkain update [pkg]               Re-resolve versions, write karkain.lock
  karkain list                       List direct + transitive dependencies
  karkain tree                       Show recursive dependency graph

PACKAGE MANAGEMENT (detailed):
  karkain pkg init [name]                Create new project
  karkain pkg add <pkg> [version]        Add dependency
  karkain pkg add <pkg> --source git --url <url>
  karkain pkg add <pkg> --source local --url <path>
  karkain pkg remove <pkg>               Remove dependency
  karkain pkg update [pkg]               Re-resolve versions
  karkain pkg upgrade                    Update all to latest compatible
  karkain pkg fetch                      Fetch resolved dependencies (lockfile)
  karkain pkg deps                       List dependencies
  karkain pkg deps --tree                Show dependency tree
  karkain pkg deps --outdated            Check for newer versions
  karkain pkg list                       List direct + transitive dependencies
  karkain pkg tree                       Show recursive dependency graph
  karkain pkg search <query>             Search package registry
  karkain pkg info <pkg>                 Show package details
  karkain pkg publish                    Publish to registry
  karkain pkg login                      Authenticate with registry
  karkain pkg logout                     Clear auth token
  karkain pkg whoami                     Show current user
  karkain pkg audit                      Check for vulnerabilities
  karkain pkg audit --licenses           License compatibility check
  karkain pkg verify                     Verify checksums of all deps
  karkain pkg cache list                 Show cached packages
  karkain pkg cache clean                Remove all cached packages
  karkain pkg cache clean --stale        Remove unused packages (>30 days)
  karkain pkg cache path                 Show cache directory
  karkain pkg workspace init             Initialize workspace root
  karkain pkg workspace add <path>       Add member package
  karkain pkg workspace remove <path>    Remove member package
  karkain pkg workspace build            Build all packages
  karkain pkg workspace test             Test all packages
  karkain pkg workspace lint             Full front-end analysis of all packages
  karkain pkg workspace graph            Show member dependency graph

OPTIONS:
  -o <path>               Output binary path (build)
  -c, --compile-only      Keep generated C source
  -g, --debug             Generate debug symbols + #line directives
  --target <target>       Target architecture (native, c23, wasm32-wasi)
  --filter <pattern>      Run only matching tests (substring of test name)
  --verbose               Emit detailed pipeline logs
  -v, --version           Show version
  -h, --help              Show this help

EXIT CODES:
  0  success              1  program failure        2  CLI usage error
  3  compile/type/sema    4  test failure           5  package/dependency
  6  infrastructure (e.g. no usable C compiler)

Examples:
  karkain run examples/array_test.kark
  karkain build examples/compiler_test.kark -o bin/app.exe
  karkain clean --all
  karkain pkg init my_project
  karkain pkg add stdlib ^0.14.0
  karkain pkg add utils --source git --url https://github.com/bob/utils.git
  karkain pkg fetch
  karkain pkg deps --tree
  karkain pkg search quantum
  karkain new my_app && cd my_app
  karkain add ./deps/libs
  karkain update
  karkain list
  karkain tree`)
}

// handlePackageCommand dispatches all 'karkain pkg' subcommands and returns the
// process exit code to use. Operational failures classify as ExitPackage(5),
// CLI usage errors within the package namespace as ExitUsage(2).
func handlePackageCommand(args []string) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		return cli.ExitEnv
	}

	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: karkain pkg <command> [options]")
		fmt.Println("Run 'karkain --help' for full command list")
		return cli.ExitSuccess
	}

	subCmd := args[0]
	rest := args[1:]

	switch subCmd {

	// --- PROJECT INIT ---
	case "init":
		name := ""
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			name = rest[0]
		}
		if name == "" {
			name = filepath.Base(cwd)
		}
		projectDir := filepath.Join(cwd, name)
		result, err := kpkg.InitProject(projectDir, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
		fmt.Printf("Project '%s' created in %s\n", name, result.ProjectDir)
		fmt.Println("Created:")
		for _, f := range result.Created {
			fmt.Printf("  %s\n", f)
		}
		fmt.Printf("\nNext steps:\n  cd %s\n  karkain pkg add <dependency>\n  karkain run\n", name)

	// --- NEW PROJECT (distinct from init: requires a name, creates a dir) ---
	case "new":
		if len(rest) < 1 || strings.HasPrefix(rest[0], "-") {
			fmt.Fprintln(os.Stderr, "Error: project name required\n  Usage: karkain new <name>")
			return cli.ExitUsage
		}
		name := rest[0]
		result, err := kpkg.NewProject(cwd, name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
		fmt.Printf("Created new project '%s' in %s\n", name, result.ProjectDir)
		fmt.Println("Created:")
		for _, f := range result.Created {
			fmt.Printf("  %s\n", f)
		}
		fmt.Printf("\nNext steps:\n  cd %s\n  karkain add <dependency>\n  karkain run\n", name)

	// --- ADD DEPENDENCY ---
	case "add":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "Error: package name required\n  Usage: karkain pkg add <pkg> [version]")
			return cli.ExitUsage
		}
		pkgName := rest[0]
		version := "*"
		source := "registry"
		url := ""
		for i := 1; i < len(rest); i++ {
			switch rest[i] {
			case "--source":
				if i+1 < len(rest) {
					i++
					source = rest[i]
				}
			case "--url":
				if i+1 < len(rest) {
					i++
					url = rest[i]
				}
			default:
				if !strings.HasPrefix(rest[i], "-") {
					version = rest[i]
				}
			}
		}
		projectDir, err := kpkg.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: not in a Karkain project (no karkain.toml)")
			return cli.ExitPackage
		}
		err = kpkg.AddDependency(projectDir, pkgName, version, source, url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
		fmt.Printf("Added %s@%s (%s)\n", pkgName, version, source)
		dep := kpkg.Dependency{Name: pkgName, Version: version, Source: source, URL: url}
		if fetchErr := kpkg.FetchModule(projectDir, dep); fetchErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: fetch failed: %v\n  Run 'karkain pkg fetch' to retry\n", fetchErr)
		} else {
			fmt.Printf("Cached to .karkain/cache/\n")
		}

	// --- REMOVE DEPENDENCY ---
	case "remove", "rm":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "Error: package name required\n  Usage: karkain pkg remove <pkg>")
			return cli.ExitUsage
		}
		projectDir, err := kpkg.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: not in a Karkain project")
			return cli.ExitPackage
		}
		err = kpkg.RemoveDependency(projectDir, rest[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
		fmt.Printf("Removed %s\n", rest[0])

	// --- FETCH ---
	case "fetch":
		projectDir, _ := kpkg.FindProjectRoot(cwd)
		if projectDir == "" {
			fmt.Fprintln(os.Stderr, "Error: not in a Karkain project")
			return cli.ExitPackage
		}
		fmt.Println("Fetching dependencies...")
		// Resolve (writes karkain.lock) then fetch exactly the locked versions.
		_, rerr := kpkg.ResolveAndLock(projectDir)
		if rerr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", rerr)
			return cli.ExitPackage
		}
		if _, ferr := kpkg.FetchLocked(projectDir); ferr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", ferr)
			return cli.ExitPackage
		}
		deps, _ := kpkg.ListDependencies(projectDir)
		if len(deps) == 0 {
			fmt.Println("No dependencies to fetch")
		} else {
			fmt.Printf("Fetched %d dependencies\n", len(deps))
			for _, d := range deps {
				fmt.Printf("  %s\n", d)
			}
		}

	// --- UPDATE ---
	case "update":
		projectDir, err := kpkg.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: not in a Karkain project")
			return cli.ExitPackage
		}
		if len(rest) > 0 {
			manifest, pErr := kpkg.ParseManifest(filepath.Join(projectDir, kpkg.ManifestFile))
			if pErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", pErr)
				return cli.ExitPackage
			}
			dep, exists := manifest.Dependencies[rest[0]]
			if !exists {
				fmt.Fprintf(os.Stderr, "Error: %s is not a dependency\n", rest[0])
				return cli.ExitPackage
			}
			fmt.Printf("Re-fetching %s@%s...\n", rest[0], dep.Version)
			if fErr := kpkg.FetchModule(projectDir, dep); fErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", fErr)
				return cli.ExitPackage
			}
			fmt.Printf("Updated %s\n", rest[0])
		} else {
			// update = re-resolve versions within manifest constraints, then
			// rewrite karkain.lock. This is distinct from fetch (which only
			// retrieves already-resolved versions).
			fmt.Println("Resolving dependencies...")
			if _, err := kpkg.ResolveAndLock(projectDir); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return cli.ExitPackage
			}
			deps, _ := kpkg.ListDependencies(projectDir)
			fmt.Printf("Updated %d dependencies (karkain.lock written)\n", len(deps))
		}

	// --- UPGRADE ---
	case "upgrade":
		projectDir, err := kpkg.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: not in a Karkain project")
			return cli.ExitPackage
		}
		manifest, pErr := kpkg.ParseManifest(filepath.Join(projectDir, kpkg.ManifestFile))
		if pErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", pErr)
			return cli.ExitPackage
		}
		fmt.Println("Checking for updates...")
		updated := 0
		for name, dep := range manifest.Dependencies {
			if dep.Source != "registry" {
				continue
			}
			if fErr := kpkg.FetchModule(projectDir, dep); fErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update %s: %v\n", name, fErr)
				continue
			}
			updated++
		}
		if updated == 0 {
			fmt.Println("All dependencies are up to date")
		} else {
			fmt.Printf("Updated %d packages\n", updated)
		}

	// --- DEPS ---
	case "deps":
		projectDir, err := kpkg.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: not in a Karkain project")
			return cli.ExitPackage
		}
		manifest, pErr := kpkg.ParseManifest(filepath.Join(projectDir, kpkg.ManifestFile))
		if pErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", pErr)
			return cli.ExitPackage
		}
		if len(manifest.Dependencies) == 0 {
			fmt.Println("No dependencies")
			return cli.ExitSuccess
		}
		treeMode := false
		outdatedMode := false
		for _, a := range rest {
			if a == "--tree" {
				treeMode = true
			}
			if a == "--outdated" {
				outdatedMode = true
			}
		}
		if treeMode {
			fmt.Printf("%s@%s\n", manifest.Name, manifest.Version)
			for name, dep := range manifest.Dependencies {
				connector := "Ã¢â€Å“Ã¢â€â‚¬Ã¢â€â‚¬ "
				if name == lastDepKey(manifest.Dependencies) {
					connector = "Ã¢â€â€Ã¢â€â‚¬Ã¢â€â‚¬ "
				}
				fmt.Printf("%s%s %s@%s (%s)\n", connector, name, name, dep.Version, dep.Source)
			}
			return cli.ExitSuccess
		}
		if outdatedMode {
			fmt.Println("Checking for newer versions...")
			for name, dep := range manifest.Dependencies {
				if dep.Source != "registry" {
					continue
				}
				latest, lErr := kpkg.LatestVersion(name)
				if lErr != nil {
					fmt.Printf("  %s %s -> (unable to check)\n", name, dep.Version)
					continue
				}
				if latest != dep.Version {
					fmt.Printf("  %s %s -> %s\n", name, dep.Version, latest)
				}
			}
			return cli.ExitSuccess
		}
		fmt.Printf("%-20s %-8s %-10s %s\n", "PACKAGE", "VERSION", "SOURCE", "URL")
		for name, dep := range manifest.Dependencies {
			fmt.Printf("%-20s %-8s %-10s %s\n", name, dep.Version, dep.Source, dep.URL)
		}

	// --- LIST (direct + transitive, resolved/versioned) ---
	case "list":
		projectDir, err := kpkg.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: not in a Karkain project")
			return cli.ExitPackage
		}
		details, derr := kpkg.ResolvedDetails(projectDir)
		if derr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", derr)
			return cli.ExitPackage
		}
		if len(details) == 0 {
			fmt.Println("Karkain Dependencies\n\n  (none)")
			return cli.ExitSuccess
		}
		fmt.Println("Karkain Dependencies")
		direct, transitive := 0, 0
		fmt.Println("\nDirect:")
		for _, d := range details {
			if d.Direct {
				direct++
				label := d.Resolved
				if label == "" {
					label = d.Version
				}
				fmt.Printf("  %-20s %s\n", d.Name, label)
			}
		}
		fmt.Println("\nTransitive:")
		for _, d := range details {
			if !d.Direct {
				transitive++
				label := d.Resolved
				if label == "" {
					label = d.Version
				}
				fmt.Printf("  %-20s %s\n", d.Name, label)
			}
		}
		fmt.Printf("\n%d direct, %d transitive\n", direct, transitive)

	// --- TREE (recursive dependency graph) ---
	case "tree":
		projectDir, err := kpkg.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: not in a Karkain project")
			return cli.ExitPackage
		}
		lines, terr := kpkg.TreeLines(projectDir)
		if terr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", terr)
			return cli.ExitPackage
		}
		for _, l := range lines {
			fmt.Println(l)
		}

	// --- SEARCH ---
	case "search":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "Error: search query required\n  Usage: karkain pkg search <query>")
			return cli.ExitUsage
		}
		query := strings.Join(rest, " ")
		results, err := kpkg.SearchRegistry(query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
		if len(results) == 0 {
			fmt.Println("No packages found")
			return cli.ExitSuccess
		}
		fmt.Printf("Found %d packages:\n\n", len(results))
		for _, r := range results {
			fmt.Printf("  %-25s %-8s %s\n", r.Name, r.Version, r.Description)
		}

	// --- INFO ---
	case "info":
		if len(rest) < 1 {
			fmt.Fprintln(os.Stderr, "Error: package name required\n  Usage: karkain pkg info <pkg>")
			return cli.ExitUsage
		}
		info, err := kpkg.PackageInfo(rest[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
		fmt.Printf("Name:        %s\n", info.Name)
		fmt.Printf("Latest:      %s\n", info.Latest)
		fmt.Printf("Description: %s\n", info.Description)
		fmt.Printf("Author:      %s\n", info.Author)
		fmt.Printf("License:     %s\n", info.License)
		fmt.Printf("Repository:  %s\n", info.Repository)
		fmt.Printf("Keywords:    %s\n", strings.Join(info.Keywords, ", "))

	// --- PUBLISH ---
	case "publish":
		projectDir, err := kpkg.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: not in a Karkain project")
			return cli.ExitPackage
		}
		token, tErr := kpkg.LoadToken()
		if tErr != nil {
			fmt.Fprintln(os.Stderr, "Error: not logged in. Run 'karkain pkg login' first")
			return cli.ExitPackage
		}
		manifest, pErr := kpkg.ParseManifest(filepath.Join(projectDir, kpkg.ManifestFile))
		if pErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", pErr)
			return cli.ExitPackage
		}
		fmt.Printf("Publishing %s@%s...\n", manifest.Name, manifest.Version)
		err = kpkg.PublishPackage(projectDir, manifest, token.Token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
		fmt.Printf("Published %s@%s\n", manifest.Name, manifest.Version)

	// --- LOGIN ---
	case "login":
		fmt.Print("Username: ")
		var user string
		fmt.Scanln(&user)
		fmt.Print("Password: ")
		var pass string
		fmt.Scanln(&pass)
		token, err := kpkg.Authenticate(user, pass)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
		err = kpkg.SaveToken(token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: could not save token: %v\n", err)
			return cli.ExitPackage
		}
		fmt.Printf("Logged in as %s\n", user)

	// --- LOGOUT ---
	case "logout":
		err := kpkg.ClearToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
		fmt.Println("Logged out")

	// --- WHOAMI ---
	case "whoami":
		token, err := kpkg.LoadToken()
		if err != nil {
			fmt.Println("Not logged in")
			return cli.ExitSuccess
		}
		fmt.Printf("Logged in as %s\n", token.Username)

	// --- AUDIT ---
	case "audit":
		projectDir, err := kpkg.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: not in a Karkain project")
			return cli.ExitPackage
		}
		if containsFlag(rest, "--json") {
			if containsFlag(rest, "--licenses") {
				warnings := kpkg.CheckLicenses(projectDir)
				data, jerr := json.MarshalIndent(struct {
					Licenses []string `json:"license_warnings"`
				}{Licenses: warnings}, "", "  ")
				if jerr != nil {
					fmt.Fprintln(os.Stderr, "Error: json encoding failed")
					return cli.ExitPackage
				}
				fmt.Println(string(data))
				return cli.ExitSuccess
			}
			vulns := kpkg.AuditDependencies(projectDir)
			if vulns == nil {
				vulns = []kpkg.VulnReport{}
			}
			data, jerr := json.MarshalIndent(struct {
				Vulnerabilities []kpkg.VulnReport `json:"vulnerabilities"`
			}{Vulnerabilities: vulns}, "", "  ")
			if jerr != nil {
				fmt.Fprintln(os.Stderr, "Error: json encoding failed")
				return cli.ExitPackage
			}
			fmt.Println(string(data))
			return cli.ExitSuccess
		}
		if containsFlag(rest, "--licenses") {
			fmt.Println("Checking license compatibility...")
			warnings := kpkg.CheckLicenses(projectDir)
			if len(warnings) == 0 {
				fmt.Println("All licenses compatible")
			} else {
				for _, w := range warnings {
					fmt.Printf("  WARN  %s\n", w)
				}
			}
			return cli.ExitSuccess
		}
		fmt.Println("Scanning for vulnerabilities...")
		vulns := kpkg.AuditDependencies(projectDir)
		if len(vulns) == 0 {
			fmt.Println("No known vulnerabilities found")
		} else {
			for _, v := range vulns {
				fmt.Printf("  WARN  %s@%s: %s\n", v.Name, v.Version, v.Advisory)
				fmt.Printf("        -> %s\n", v.Fix)
			}
			fmt.Printf("\n%d issues found\n", len(vulns))
		}

	// --- VERIFY ---
	case "verify":
		projectDir, err := kpkg.FindProjectRoot(cwd)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: not in a Karkain project")
			return cli.ExitPackage
		}
		if containsFlag(rest, "--json") {
			results := kpkg.VerifyIntegrity(projectDir)
			if results == nil {
				results = []kpkg.VerifyResult{}
			}
			allOk := true
			for _, r := range results {
				if !r.Valid {
					allOk = false
				}
			}
			data, jerr := json.MarshalIndent(struct {
				Ok       bool                `json:"ok"`
				Packages []kpkg.VerifyResult `json:"packages"`
			}{Ok: allOk, Packages: results}, "", "  ")
			if jerr != nil {
				fmt.Fprintln(os.Stderr, "Error: json encoding failed")
				return cli.ExitPackage
			}
			fmt.Println(string(data))
			if allOk {
				return cli.ExitSuccess
			}
			return cli.ExitPackage
		}
		if containsFlag(rest, "--signatures") {
			fmt.Println("Verifying package signatures... (signature verification not yet implemented)")
			return cli.ExitSuccess
		}
		fmt.Println("Verifying package integrity...")
		results := kpkg.VerifyIntegrity(projectDir)
		allOk := true
		for _, r := range results {
			if r.Valid {
				fmt.Printf("  OK   %s@%s\n", r.Name, r.Version)
			} else {
				fmt.Printf("  FAIL %s@%s: %s\n", r.Name, r.Version, r.Error)
				allOk = false
			}
		}
		if allOk {
			fmt.Println("\nAll packages verified")
		} else {
			fmt.Println("\nIntegrity check failed")
			return cli.ExitPackage
		}

	// --- CACHE ---
	case "cache":
		if len(rest) == 0 {
			fmt.Println("Usage: karkain pkg cache <list|clean|path>")
			return cli.ExitUsage
		}
		switch rest[0] {
		case "list":
			kpkg.ListCache()
		case "clean":
			staleOnly := containsFlag(rest[1:], "--stale")
			kpkg.CleanCache(staleOnly)
		case "path":
			pDir, _ := kpkg.FindProjectRoot(cwd)
			if pDir == "" {
				pDir = cwd
			}
			fmt.Println(filepath.Join(pDir, kpkg.CacheModules))
		default:
			fmt.Println("Usage: karkain pkg cache <list|clean|path>")
		}

	// --- WORKSPACE ---
	case "workspace", "ws":
		wsArgs := rest
		if len(wsArgs) == 0 {
			fmt.Println("Usage: karkain pkg workspace <init|add|remove|list|build|test|check|run|clean|lint|graph>")
			return cli.ExitUsage
		}
		wscfg := codegen.NewConfig()
		switch wsArgs[0] {
		case "init":
			if err := kpkg.InitWorkspace(cwd); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return cli.ExitPackage
			}
			fmt.Println("Initialized workspace")
		case "add":
			if len(wsArgs) < 2 {
				fmt.Fprintln(os.Stderr, "Error: path required")
				return cli.ExitUsage
			}
			if err := kpkg.AddWorkspaceMember(cwd, wsArgs[1]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return cli.ExitPackage
			}
			fmt.Printf("Added %s to workspace\n", wsArgs[1])
		case "remove":
			if len(wsArgs) < 2 {
				fmt.Fprintln(os.Stderr, "Error: path required")
				return cli.ExitUsage
			}
			if err := cli.WorkspaceRemove(cwd, wsArgs[1]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return cli.ExitPackage
			}
			fmt.Printf("Removed %s from workspace\n", wsArgs[1])
		case "graph":
			if err := cli.WorkspaceGraph(cwd); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return cli.ExitPackage
			}
		case "lint":
			if err := cli.WorkspaceLint(cwd, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return cli.ExitCompile
			}
		case "list", "ls":
			if err := cli.WorkspaceList(cwd); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return cli.ExitPackage
			}
		case "build":
			if err := cli.WorkspaceBuild(cwd, wscfg, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return cli.ExitPackage
			}
		case "test":
			if err := cli.WorkspaceTest(cwd, wscfg, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return cli.ExitPackage
			}
		case "check":
			if err := cli.WorkspaceCheck(cwd, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return cli.ExitPackage
			}
		case "run":
			if err := cli.WorkspaceRun(cwd, wscfg, false); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return cli.ExitPackage
			}
		case "clean":
			wsAll := false
			for _, a := range wsArgs[1:] {
				if a == "--all" {
					wsAll = true
				}
			}
			if err := cli.WorkspaceClean(cwd, wsAll); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return cli.ExitPackage
			}
		default:
			fmt.Fprintf(os.Stderr, "Unknown workspace command: %s\n", wsArgs[0])
			return cli.ExitUsage
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown pkg command: %s\n", subCmd)
		fmt.Println("Run 'karkain --help' for available commands")
		return cli.ExitUsage
	}

	return cli.ExitSuccess
}

// handleWorkspaceCommand is the top-level `karkain workspace` dispatcher. It
// shares the pkg/workspace implementation so both spellings behave identically.
func handleWorkspaceCommand(args []string, verbose bool) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		return cli.ExitEnv
	}

	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println("Usage: karkain workspace <list|build|test|check|run|clean|lint|graph|init|add|remove>")
		return cli.ExitSuccess
	}

	cfg := codegen.NewConfig()
	wscfg := cfg

	sub := args[0]
	tail := args[1:]
	all := false
	for i := 0; i < len(tail); i++ {
		switch tail[i] {
		case "--all":
			all = true
		case "--verbose":
			verbose = true
		default:
			if strings.HasPrefix(tail[i], "-") {
				fmt.Printf("Error: Unknown flag '%s'\n", tail[i])
				return cli.ExitUsage
			}
		}
	}

	switch sub {
	case "list", "ls":
		if err := cli.WorkspaceList(cwd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
	case "build":
		if err := cli.WorkspaceBuild(cwd, wscfg, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
	case "test":
		if err := cli.WorkspaceTest(cwd, wscfg, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
	case "check":
		if err := cli.WorkspaceCheck(cwd, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
	case "run":
		if err := cli.WorkspaceRun(cwd, wscfg, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
	case "clean":
		if err := cli.WorkspaceClean(cwd, all); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
	case "init":
		if err := kpkg.InitWorkspace(cwd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
		fmt.Println("Initialized workspace")
	case "graph":
		if err := cli.WorkspaceGraph(cwd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
	case "lint":
		if err := cli.WorkspaceLint(cwd, verbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitCompile
		}
	case "add":
		if len(tail) == 0 {
			fmt.Fprintln(os.Stderr, "Error: path required")
			return cli.ExitUsage
		}
		path := ""
		for i := 0; i < len(tail); i++ {
			if !strings.HasPrefix(tail[i], "-") && path == "" {
				path = tail[i]
			}
		}
		if path == "" {
			fmt.Fprintln(os.Stderr, "Error: path required")
			return cli.ExitUsage
		}
		if err := kpkg.AddWorkspaceMember(cwd, path); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
		fmt.Printf("Added %s to workspace\n", path)
	case "remove":
		if len(tail) == 0 {
			fmt.Fprintln(os.Stderr, "Error: path required")
			return cli.ExitUsage
		}
		path := ""
		for i := 0; i < len(tail); i++ {
			if !strings.HasPrefix(tail[i], "-") && path == "" {
				path = tail[i]
			}
		}
		if path == "" {
			fmt.Fprintln(os.Stderr, "Error: path required")
			return cli.ExitUsage
		}
		if err := cli.WorkspaceRemove(cwd, path); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return cli.ExitPackage
		}
		fmt.Printf("Removed %s from workspace\n", path)
	default:
		fmt.Fprintf(os.Stderr, "Unknown workspace command: %s\n", sub)
		return cli.ExitUsage
	}
	return cli.ExitSuccess
}

func handleLSP() {
	// Phase 82: the `lsp` / `language-server` command now serves the real,
	// tested LSP engine in pkg/lsp (JSON-RPC 2.0 over stdio with correct
	// Content-Length framing) instead of the previous canned mock.
	srv := lsp.NewServer(os.Stdin, os.Stdout, os.Stdout)
	if err := srv.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Language server error: %v\n", err)
		os.Exit(cli.ExitFailure)
	}
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printHelp()
		os.Exit(0)
	}

	// Package-management commands take their own flags (e.g. add --source
	// --url) that do not belong to the global/compiler flag parser. Dispatch
	// them to the package command handler unconditionally, before parsing.
	switch args[0] {
	case "init", "new", "add", "remove", "rm", "update", "list", "tree", "fetch":
		os.Exit(handlePackageCommand(args))
	}

	command := ""
	targetFile := ""
	outputPath := ""
	cfg := codegen.NewConfig()
	verbose := false
	extraArgs := []string{}
	testFilter := ""       // KTF-001: deterministic substring filter for `karkain test`
	compileCorpus := false // KTF-002: run the compile-pass/compile-fail corpus
	formatJSON := false    // Phase 82: machine-readable structured output (e.g. check --format=json)
	fmtCheck := false     // Phase 82: `karkain fmt --check` verifies canonical formatting
	engine := cli.EngineFromEnv() // Phase 95: KARKAIN_ENGINE / --engine selects self-hosted kcc

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Handle flags with equals sign
		if strings.Contains(arg, "=") && strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg, "=", 2)
			flagName := parts[0]
			flagValue := parts[1]

			switch flagName {
			case "--target":
				cfg.Target = flagValue
				if err := cli.ValidateTarget(cfg.Target); err != nil {
					fmt.Println(err)
					os.Exit(cli.ExitUsage)
				}
				continue
			case "--filter":
				testFilter = flagValue
				continue
			case "--format":
				if flagValue == "json" {
					formatJSON = true
					continue
				}
				fmt.Printf("Error: Unknown --format value '%s' (supported: json)\n", flagValue)
				os.Exit(cli.ExitUsage)
			case "--engine":
				e, err := cli.EngineFlag(flagValue)
				if err != nil {
					fmt.Println(err)
					os.Exit(cli.ExitUsage)
				}
				engine = e
				continue
			}
		}

		switch arg {
		case "-v", "--version":
			printVersion()
			os.Exit(0)
		case "-h", "--help":
			printHelp()
			os.Exit(0)
		case "--verbose":
			verbose = true
		case "-c", "--compile-only":
			cfg.CompileOnly = true
		case "-g", "--debug":
			cfg.Debug = true
		case "--target":
			if i+1 < len(args) {
				cfg.Target = args[i+1]
				i++
			} else {
				fmt.Println("Error: --target flag requires a target architecture")
				os.Exit(cli.ExitUsage)
			}
			if err := cli.ValidateTarget(cfg.Target); err != nil {
				fmt.Println(err)
				os.Exit(cli.ExitUsage)
			}
		case "-o":
			if i+1 < len(args) {
				outputPath = args[i+1]
				i++
			} else {
				fmt.Println("Error: -o flag requires an output file path")
				os.Exit(cli.ExitUsage)
			}
		case "--compile":
			compileCorpus = true
		case "--json":
			formatJSON = true
		case "--check":
			// `karkain fmt --check <file>` verifies without rewriting.
			fmtCheck = true
		case "--format":
			if i+1 < len(args) {
				if args[i+1] == "json" {
					formatJSON = true
					i++
				} else {
					fmt.Printf("Error: Unknown --format value '%s' (supported: json)\n", args[i+1])
					os.Exit(cli.ExitUsage)
				}
			} else {
				fmt.Println("Error: --format flag requires a value")
				os.Exit(cli.ExitUsage)
			}
		case "--filter":
			if i+1 < len(args) {
				testFilter = args[i+1]
				i++
			} else {
				fmt.Println("Error: --filter flag requires a pattern")
				os.Exit(cli.ExitUsage)
			}
		case "--engine":
			if i+1 < len(args) {
				e, err := cli.EngineFlag(args[i+1])
				if err != nil {
					fmt.Println(err)
					os.Exit(cli.ExitUsage)
				}
				engine = e
				i++
			} else {
				fmt.Println("Error: --engine flag requires a value (go|kcc)")
				os.Exit(cli.ExitUsage)
			}
		case "build", "run", "check", "transpile", "test", "bench", "lint", "lsp", "language-server", "fmt":
			command = arg
		case "ide":
			// ide info: machine-readable LanguageProvider contract for IDEs
			// (LiteIDE et al). No subcommand/none emits a usage note.
			if i+1 < len(args) && args[i+1] == "info" {
				result := cli.ToolchainInfoCommand()
				fmt.Println(result.Message)
				os.Exit(result.ExitCode)
			}
			fmt.Println("ide: known subcommands: info")
			os.Exit(cli.ExitUsage)
		case "explain":
			// explain <code> or explain --list
			if i+1 < len(args) && args[i+1] == "--list" {
				result := cli.ExplainListCommand()
				os.Exit(result.ExitCode)
			}
			code := ""
			for j := i + 1; j < len(args); j++ {
				if !strings.HasPrefix(args[j], "-") {
					code = args[j]
					break
				}
			}
			result := cli.ExplainCommand(code)
			os.Exit(result.ExitCode)
		case "workspace", "ws":
			// top-level workspace family: list|build|test|check|run|clean
			os.Exit(handleWorkspaceCommand(args[i+1:], verbose))
		case "clean":
			// clean [path] [--all]: remove generated artifacts.
			cleanPath := "."
			cleanAll := false
			for j := i + 1; j < len(args); j++ {
				if args[j] == "--all" {
					cleanAll = true
					continue
				}
				if strings.HasPrefix(args[j], "-") {
					fmt.Printf("Error: Unknown flag '%s'\n", args[j])
					os.Exit(cli.ExitUsage)
				}
				if cleanPath == "." {
					cleanPath = args[j]
				}
			}
			i = len(args) - 1
			result := cli.CleanCommand(cleanPath, cleanAll)
			if result.Message != "" {
				fmt.Println(result.Message)
			}
			os.Exit(result.ExitCode)
		case "target":
			result := cli.TargetCommand()
			os.Exit(result.ExitCode)
		case "config":
			result := cli.ConfigCommand()
			os.Exit(result.ExitCode)
		case "pkg":
			// Collect all remaining args and hand off to package manager
			os.Exit(handlePackageCommand(args[i+1:]))
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Printf("Error: Unknown flag '%s'\n", arg)
				printHelp()
				os.Exit(cli.ExitUsage)
			}
			if targetFile == "" {
				targetFile = arg
			} else {
				extraArgs = append(extraArgs, arg)
			}
		}
	}

	if command == "lsp" || command == "language-server" {
		handleLSP()
		return
	}

	if command == "fmt" {
		if targetFile == "" {
			fmt.Println("Error: No input .kark file specified")
			printHelp()
			os.Exit(cli.ExitUsage)
		}
		result := cli.FormatCommand(targetFile, fmtCheck)
		if result.Message != "" {
			fmt.Println(result.Message)
		}
		os.Exit(result.ExitCode)
	}

	if command == "test" {
		if compileCorpus {
			dir := ""
			if targetFile != "" {
				dir = targetFile
			}
			if dir == "" {
				dir = cli.DefaultCompileCorpus
			}
			s := cli.RunCompileCorpus(dir, verbose, cfg)
			if s.Failed > 0 {
				os.Exit(cli.ExitTest)
			}
			os.Exit(cli.ExitSuccess)
		}
		testPath := targetFile
		if testPath == "" {
			testPath = "."
		}
		result := cli.TestCommandFiltered(testPath, cfg, verbose, testFilter)
		fmt.Print(result.Message)
		os.Exit(result.ExitCode)
	}

	if command == "bench" {
		benchPath := targetFile
		if benchPath == "" {
			benchPath = "."
		}
		result := cli.BenchCommand(benchPath, cfg, verbose)
		if result.Message != "" {
			fmt.Println(result.Message)
		}
		os.Exit(result.ExitCode)
	}

	if command == "lint" {
		if targetFile == "" {
			fmt.Println("Error: No input .kark file specified")
			printHelp()
			os.Exit(cli.ExitUsage)
		}
		result := cli.LintCommand(targetFile, verbose)
		if result.Message != "" {
			fmt.Println(result.Message)
		}
		os.Exit(result.ExitCode)
	}

	if targetFile == "" {
		fmt.Println("Error: No input .kark file specified")
		printHelp()
		os.Exit(cli.ExitUsage)
	}

	if err := cli.ValidateKarFile(targetFile); err != nil {
		fmt.Println(err)
		os.Exit(cli.ExitUsage)
	}

	if command == "" {
		command = "run"
	}

	var result cli.CommandResult
	if engine == cli.EngineKCC {
		// Phase 95: the self-hosted compiler is the primary engine for the
		// core pipeline (lex + parse + C codegen). `run`/`build`/`check`
		// dispatch to kcc unless the Go engine was explicitly requested.
		switch command {
		case "run":
			result = cli.KCCRunCommand(nil, targetFile, cfg, verbose)
		case "build", "transpile":
			// kcc emits C23 and links native executables through gcc, so it
			// owns both native and c23 targets. Exotic targets (wasm, etc.)
			// still route through the Go backend.
			if cfg.Target == "native" || cfg.Target == "c23" {
				result = cli.KCCBuildCommand(nil, targetFile, outputPath, cfg, verbose)
			} else {
				result = cli.BuildCommand(targetFile, outputPath, cfg, verbose)
			}
		case "check":
			result = cli.KCCCheckCommand(nil, targetFile, verbose)
		default:
			fmt.Printf("Error: Unknown command '%s'\n", command)
			printHelp()
			os.Exit(cli.ExitUsage)
		}
	} else {
		switch command {
		case "run":
			result = cli.RunCommand(targetFile, cfg, verbose)
		case "build":
			result = cli.BuildCommand(targetFile, outputPath, cfg, verbose)
		case "transpile":
			result = cli.BuildCommand(targetFile, outputPath, cfg, verbose)
		case "check":
			format := cli.CheckFormatHuman
			if formatJSON {
				format = cli.CheckFormatJSON
			}
			result = cli.CheckCommandFormatted(targetFile, verbose, format)
		default:
			fmt.Printf("Error: Unknown command '%s'\n", command)
			printHelp()
			os.Exit(cli.ExitUsage)
		}
	}

	if result.Message != "" {
		fmt.Println(result.Message)
	}
	os.Exit(result.ExitCode)
}

func lastDepKey(m map[string]kpkg.Dependency) string {
	last := ""
	for k := range m {
		last = k
	}
	return last
}

func containsFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
