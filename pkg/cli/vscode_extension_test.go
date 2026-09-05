package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// vscodeExtensionDir locates the VS Code extension relative to the package dir.
func vscodeExtensionDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := filepath.Join(wd, "..", "..", "editors", "vscode")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("vscode extension dir not found at %s", dir)
	}
	return dir
}

func readT(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

type vscodeManifest struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Main      string `json:"main"`
	Engines   map[string]string
	Activation []string `json:"activationEvents"`
	Contributes struct {
		Languages []struct {
			ID           string   `json:"id"`
			Name         string   `json:"name"`
			Extensions   []string `json:"extensions"`
			Configuration string   `json:"configuration"`
		} `json:"languages"`
		Grammars []struct {
			Language string `json:"language"`
			Scope    string `json:"scopeName"`
			Path     string `json:"path"`
		} `json:"grammars"`
		Commands []struct {
			Command string `json:"command"`
			Title   string `json:"title"`
		} `json:"commands"`
		Configuration struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"configuration"`
	} `json:"contributes"`
}

// TestVSCodeExtension_ManifestContract validates the extension manifest:
// grammar + language registration must target .kark with scope source.karkain,
// a main entry point must exist, and every advertised command must actually be
// registered by the extension code.
func TestVSCodeExtension_ManifestContract(t *testing.T) {
	dir := vscodeExtensionDir(t)

	var m vscodeManifest
	if err := json.Unmarshal([]byte(readT(t, filepath.Join(dir, "package.json"))), &m); err != nil {
		t.Fatalf("package.json invalid: %v", err)
	}
	if m.Name != "karkain" {
		t.Errorf("extension name = %q, want karkain", m.Name)
	}
	if v, ok := m.Engines["vscode"]; !ok || v == "" {
		t.Errorf("engines.vscode missing (engines must be an object, not an array)")
	}
	if m.Main != "./extension.js" {
		t.Errorf("main = %q, want ./extension.js", m.Main)
	}

	lang := m.Contributes.Languages
	if len(lang) != 1 || lang[0].ID != "karkain" {
		t.Fatalf("expected one karkain language contribution, got %+v", lang)
	}
	hasExt := false
	for _, e := range lang[0].Extensions {
		if e == ".kark" {
			hasExt = true
		}
	}
	if !hasExt {
		t.Errorf("language must register .kark extension")
	}

	if len(m.Contributes.Grammars) != 1 ||
		m.Contributes.Grammars[0].Language != "karkain" ||
		m.Contributes.Grammars[0].Scope != "source.karkain" {
		t.Errorf("grammar contribution malformed: %+v", m.Contributes.Grammars)
	}

	wantCommands := []string{
		"karkain.check", "karkain.compile", "karkain.run", "karkain.formatDocument",
	}
	haveCommands := map[string]bool{}
	for _, c := range m.Contributes.Commands {
		haveCommands[c.Command] = true
	}
	for _, c := range wantCommands {
		if !haveCommands[c] {
			t.Errorf("manifest missing command %s", c)
		}
	}

	if _, ok := m.Contributes.Configuration.Properties["karkain.compilerPath"]; !ok {
		t.Errorf("manifest missing karkain.compilerPath setting")
	}
	if _, ok := m.Contributes.Configuration.Properties["karkain.formatOnSave"]; !ok {
		t.Errorf("manifest missing karkain.formatOnSave setting")
	}
}

// TestVSCodeExtension_ExecutionUnitsExist validates the shipped files the
// manifest points at and that each advertised command is registered in the
// extension entry point.
func TestVSCodeExtension_ExecutionUnitsExist(t *testing.T) {
	dir := vscodeExtensionDir(t)

	ext := readT(t, filepath.Join(dir, "extension.js"))
	if len(ext) < 500 {
		t.Errorf("extension.js suspiciously small (%d bytes)", len(ext))
	}
	for _, c := range []string{
		"karkain.check", "karkain.compile", "karkain.run", "karkain.formatDocument",
	} {
		if !strings.Contains(ext, "registerCommand('"+c+"'") &&
			!strings.Contains(ext, `registerCommand("`+c+`"`) {
			t.Errorf("extension.js does not register %s", c)
		}
	}
	if !strings.Contains(ext, "registerDocumentFormattingEditProvider('karkain'") {
		t.Errorf("extension.js missing document formatting provider")
	}

	grammar := readT(t, filepath.Join(dir, "syntaxes", "karkain.tmLanguage.json"))
	var g struct {
		Scope string `json:"scopeName"`
	}
	if err := json.Unmarshal([]byte(grammar), &g); err != nil {
		t.Errorf("tmLanguage invalid JSON: %v", err)
	}
	if g.Scope != "source.karkain" {
		t.Errorf("grammar scope = %q, want source.karkain", g.Scope)
	}

	lc := readT(t, filepath.Join(dir, "language-configuration.json"))
	if !strings.Contains(lc, `"lineComment": "//"`) {
		t.Errorf("language-configuration.json missing line comment")
	}
	if !strings.Contains(lc, `"autoClosingPairs"`) || !strings.Contains(lc, `"folding"`) {
		t.Errorf("language-configuration.json missing auto-closing/folding config")
	}
}