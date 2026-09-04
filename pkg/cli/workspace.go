package cli

import (
	"fmt"
	"karkain/pkg/codegen"
	"karkain/pkg/pm"
	"os"
	"path/filepath"
)

// WorkspaceBuild builds every workspace member exactly once in dependency
// (topological) order. Compilation reuses the standard BuildCommand pipeline.
func WorkspaceBuild(rootDir string, cfg codegen.Config, verbose bool) error {
	members, err := pm.WorkspaceOrder(rootDir)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		fmt.Println("No workspace members")
		return nil
	}

	fmt.Printf("Workspace build (%d members, dependency order):\n", len(members))
	for _, m := range members {
		mainKar, skip := memberEntrypoint(m.Dir)
		if skip {
			fmt.Printf("  SKIP  [%d] %s (no entrypoint found)\n", m.Order, m.Name)
			continue
		}
		fmt.Printf("  BUILD [%d] %s\n", m.Order, m.Name)
		res := BuildCommand(mainKar, "", cfg, verbose)
		if res.ExitCode != 0 {
			return fmt.Errorf("workspace build failed for %s: %s", m.Name, res.Message)
		}
	}
	return nil
}

// WorkspaceTest runs each member's test files in dependency order (P2.11).
// Dev-dependencies are included in test scope but never in normal builds.
func WorkspaceTest(rootDir string, cfg codegen.Config, verbose bool) error {
	members, err := pm.WorkspaceOrder(rootDir)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		fmt.Println("No workspace members")
		return nil
	}

	fmt.Printf("Workspace test (%d members, dependency order):\n", len(members))
	failed := 0
	for _, m := range members {
		fmt.Printf("  TEST  [%d] %s\n", m.Order, m.Name)
		res := TestCommand(m.Dir, cfg, verbose)
		if res.ExitCode != 0 {
			failed++
			fmt.Printf("    FAIL: %s\n", res.Message)
		}
	}
	if failed > 0 {
		return fmt.Errorf("workspace test: %d member(s) failed", failed)
	}
	return nil
}

// memberEntrypoint locates a member's compilable entry file. It returns
// (path, false) when found, or ("", true) to skip the member. The project
// layout places the entry at <member>/src/main.kark; a flat main.kark is also
// accepted.
func memberEntrypoint(memberDir string) (string, bool) {
	candidates := []string{
		filepath.Join(memberDir, "src", "main.kark"),
		filepath.Join(memberDir, "main.kark"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, false
		}
	}
	// A library member without an executable entrypoint is not an error:
	// it only contributes sources to members that depend on it.
	return "", true
}
