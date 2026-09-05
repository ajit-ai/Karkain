package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

// toolchainInfoVersion is the schema version of the machine-readable
// LanguageProvider contract emitted by `karkain ide info`.
const toolchainInfoVersion = 1

// ToolchainInfo is the machine-readable contract that an IDE host (LiteIDE, VS
// Code, etc.) consumes to integrate the Karkain toolchain. Field names and
// command layouts are part of the toolchain contract.
type ToolchainInfo struct {
	SchemaVersion int `json:"schemaVersion"`
	Language      struct {
		Name            string   `json:"name"`
		ID              string   `json:"id"`
		Extensions      []string `json:"extensions"`
		DefaultFilename string   `json:"defaultFilename"`
	} `json:"language"`
	ProjectDetection struct {
		Manifest string `json:"manifest"`
		Marker   string `json:"marker"`
	} `json:"projectDetection"`
	Toolchain struct {
		Binary        string   `json:"binary"`
		Check         []string `json:"check"`
		CheckJSON     []string `json:"checkJSON"`
		Build         []string `json:"build"`
		Run           []string `json:"run"`
		Test          []string `json:"test"`
		CompileCorpus []string `json:"compileCorpus"`
		Format        []string `json:"format"`
		FormatCheck   []string `json:"formatCheck"`
		Lint          []string `json:"lint"`
		LSP           []string `json:"lsp"`
	} `json:"toolchain"`
	Diagnostics struct {
		Format string `json:"format"`
		Schema string `json:"schema"`
		Code   string `json:"code"`
	} `json:"diagnostics"`
	ExitCodes struct {
		Success int `json:"success"`
		Failure int `json:"failure"`
		Usage   int `json:"usage"`
		Compile int `json:"compile"`
		Test    int `json:"test"`
		Package int `json:"package"`
		Env     int `json:"env"`
	} `json:"exitCodes"`
}

// ToolchainInfoCommand emits the LanguageProvider contract as JSON to stdout.
func ToolchainInfoCommand() CommandResult {
	var info ToolchainInfo
	info.SchemaVersion = toolchainInfoVersion
	info.Language.Name = "Karkain"
	info.Language.ID = "karkain"
	info.Language.Extensions = []string{".kark"}
	info.Language.DefaultFilename = "main.kark"
	info.ProjectDetection.Manifest = "karkain.toml"
	info.ProjectDetection.Marker = "main.kark"
	info.Toolchain.Binary = "karkain"
	info.Toolchain.Check = []string{"karkain", "check", "<file>"}
	info.Toolchain.CheckJSON = []string{"karkain", "check", "--format=json", "<file>"}
	info.Toolchain.Build = []string{"karkain", "build", "<file>"}
	info.Toolchain.Run = []string{"karkain", "run", "<file>"}
	info.Toolchain.Test = []string{"karkain", "test", "<path>"}
	info.Toolchain.CompileCorpus = []string{"karkain", "test", "--compile", "<dir>"}
	info.Toolchain.Format = []string{"karkain", "fmt", "<file>"}
	info.Toolchain.FormatCheck = []string{"karkain", "fmt", "--check", "<file>"}
	info.Toolchain.Lint = []string{"karkain", "lint", "<file>"}
	info.Toolchain.LSP = []string{"karkain", "lsp"}
	info.Diagnostics.Format = "json"
	info.Diagnostics.Schema = "karkain-diagnostics-v1"
	info.Diagnostics.Code = "E-K-*"
	info.ExitCodes.Success = ExitSuccess
	info.ExitCodes.Failure = ExitFailure
	info.ExitCodes.Usage = ExitUsage
	info.ExitCodes.Compile = ExitCompile
	info.ExitCodes.Test = ExitTest
	info.ExitCodes.Package = ExitPackage
	info.ExitCodes.Env = ExitEnv

	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(info); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("error serializing toolchain info: %v", err)}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: strings.TrimSuffix(sb.String(), "\n")}
}