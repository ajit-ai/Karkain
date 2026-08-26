package pm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WorkspaceConfig is the workspace root configuration.
type WorkspaceConfig struct {
	Members []string `json:"members"`
}

const WorkspaceFile = "karkain.workspace.json"

// InitWorkspace creates a workspace root configuration.
func InitWorkspace(dir string) error {
	path := filepath.Join(dir, WorkspaceFile)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("workspace already initialized at %s", dir)
	}

	ws := &WorkspaceConfig{Members: []string{}}
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// AddWorkspaceMember adds a package directory to the workspace.
func AddWorkspaceMember(rootDir, memberPath string) error {
	path := filepath.Join(rootDir, WorkspaceFile)
	ws, err := readWorkspace(path)
	if err != nil {
		return fmt.Errorf("not a workspace root (no %s found)", WorkspaceFile)
	}

	absPath, _ := filepath.Abs(filepath.Join(rootDir, memberPath))
	for _, m := range ws.Members {
		if m == memberPath || m == absPath {
			return fmt.Errorf("%s is already a workspace member", memberPath)
		}
	}

	ws.Members = append(ws.Members, memberPath)
	data, err := json.MarshalIndent(ws, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// WorkspaceBuild builds all packages in the workspace.
func WorkspaceBuild(rootDir string) error {
	path := filepath.Join(rootDir, WorkspaceFile)
	ws, err := readWorkspace(path)
	if err != nil {
		return fmt.Errorf("not a workspace root: %w", err)
	}

	if len(ws.Members) == 0 {
		fmt.Println("No workspace members")
		return nil
	}

	for _, member := range ws.Members {
		memberDir := filepath.Join(rootDir, member)
		mainKar := filepath.Join(memberDir, "src", "main.kar")
		if _, err := os.Stat(mainKar); os.IsNotExist(err) {
			// Try memberDir/main.kar
			mainKar = filepath.Join(memberDir, "main.kar")
			if _, err := os.Stat(mainKar); os.IsNotExist(err) {
				fmt.Printf("  SKIP  %s (no main.kar found)\n", member)
				continue
			}
		}
		fmt.Printf("  BUILD %s\n", member)
		// TODO: invoke compiler for each member
		_ = mainKar
	}

	return nil
}

// WorkspaceTest runs tests for all packages in the workspace.
func WorkspaceTest(rootDir string) error {
	path := filepath.Join(rootDir, WorkspaceFile)
	ws, err := readWorkspace(path)
	if err != nil {
		return fmt.Errorf("not a workspace root: %w", err)
	}

	if len(ws.Members) == 0 {
		fmt.Println("No workspace members")
		return nil
	}

	for _, member := range ws.Members {
		memberDir := filepath.Join(rootDir, member)
		fmt.Printf("  TEST  %s\n", member)
		_ = memberDir
		// TODO: discover and run test files in each member
	}

	return nil
}

func readWorkspace(path string) (*WorkspaceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ws WorkspaceConfig
	if err := json.Unmarshal(data, &ws); err != nil {
		return nil, err
	}
	return &ws, nil
}
