package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"karkain/pkg/cli"
	"karkain/pkg/codegen"
	kpkg "karkain/pkg/pm"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const versionString = "Karkain Compiler v0.16.0 (%s/%s, JIT Engine & C ABI FFI)\n"

func printVersion() {
	fmt.Printf(versionString, runtime.GOOS, runtime.GOARCH)
}

func printHelp() {
	fmt.Println(`Karkain Programming Language Toolchain

Usage:
  karkain [command] [options] <file.kar>

Commands:
  run <file.kar>       Compile and immediately run a .kar script (default)
  build <file.kar>     Compile a .kar script into a native standalone executable
  check <file.kar>     Validate syntax and semantics without producing output
  test <path>          Discover and run *_test.kar files
  lsp                  Start Language Server Protocol server for IDE integration

Package Management:
  init <name>          Initialize a new Karkain project with standard structure
  add <dep> [version]  Add a dependency to the project manifest (karkain.toml)
  fetch                Download and cache all project dependencies

Options:
  -o <path>           Specify custom output binary file path (used with build)
  -c, --compile-only  Keep generated C source code file (temp_runner.c) on disk
  -g, --debug         Generate debug symbols (DWARF/PDB) for GDB/LLDB/VS Code debugging
  --target <target>   Specify target architecture (native, wasm32-wasi)
  --verbose           Emit detailed pipeline logs (Tokens, AST, C Code, Compiler Invocation)
  -v, --version       Show version information
  -h, --help          Show this help message

Examples:
  karkain run examples/array_test.kar
  karkain build examples/compiler_test.kar -o bin/app.exe
  karkain check examples/phase1_test.kar
  karkain test examples/
  karkain init my_project
  karkain add stdlib 0.14.0
  karkain fetch
  karkain examples/phase1_test.kar --verbose
  karkain lsp`)
}

// LSP message types
type LSPRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	ID      int         `json:"id"`
	Params  interface{} `json:"params"`
}

type LSPResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *LSPError   `json:"error,omitempty"`
}

type LSPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo"`
}

type ServerCapabilities struct {
	TextDocumentSync   TextDocumentSync `json:"textDocumentSync"`
	HoverProvider      bool             `json:"hoverProvider"`
	DefinitionProvider bool             `json:"definitionProvider"`
}

type TextDocumentSync struct {
	OpenClose bool `json:"openClose"`
	Change    int  `json:"change"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type HoverResult struct {
	Contents string `json:"contents"`
}

type DefinitionResult struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// handlePackageCommand handles init, add, and fetch commands.
func handlePackageCommand(command, targetFile string, extraArgs []string) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	switch command {
	case "init":
		projectName := targetFile
		if projectName == "" {
			fmt.Println("Error: init command requires a project name")
			fmt.Println("Usage: karkain init <project_name>")
			os.Exit(1)
		}

		projectDir := filepath.Join(cwd, projectName)
		result, err := kpkg.InitProject(projectDir, projectName)
		if err != nil {
			fmt.Printf("Error creating project: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Project '%s' created successfully!\n", projectName)
		fmt.Printf("  Directory: %s\n", result.ProjectDir)
		fmt.Printf("  Manifest:  %s\n", result.Manifest)
		fmt.Println("\nNext steps:")
		fmt.Printf("  cd %s\n", projectName)
		fmt.Println("  karkain run src/main.kar")

	case "add":
		depName := targetFile
		if depName == "" {
			fmt.Println("Error: add command requires a dependency name")
			fmt.Println("Usage: karkain add <dependency> [version]")
			os.Exit(1)
		}

		version := "*"
		if len(extraArgs) > 0 {
			version = extraArgs[0]
		}

		// Try to find project root
		projectDir, err := kpkg.FindProjectRoot(cwd)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			fmt.Println("Make sure you are inside a Karkain project directory (with karkain.toml)")
			os.Exit(1)
		}

		err = kpkg.AddDependency(projectDir, depName, version, "registry", "")
		if err != nil {
			fmt.Printf("Error adding dependency: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Added dependency '%s' version %s\n", depName, version)

	case "fetch":
		projectDir, err := kpkg.FindProjectRoot(cwd)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			fmt.Println("Make sure you are inside a Karkain project directory (with karkain.toml)")
			os.Exit(1)
		}

		fmt.Println("Fetching dependencies...")
		err = kpkg.FetchAll(projectDir)
		if err != nil {
			fmt.Printf("Error fetching dependencies: %v\n", err)
			os.Exit(1)
		}

		deps, err := kpkg.ListDependencies(projectDir)
		if err != nil {
			fmt.Printf("Error listing dependencies: %v\n", err)
			os.Exit(1)
		}

		if len(deps) == 0 {
			fmt.Println("No dependencies to fetch")
		} else {
			fmt.Printf("Fetched %d dependency(ies):\n", len(deps))
			for _, d := range deps {
				fmt.Printf("  - %s\n", d)
			}
		}
	}
}

func handleLSP() {
	fmt.Println("Karkain LSP Server starting...")
	fmt.Println("Listening on stdin/stdout for JSON-RPC 2.0 messages")

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		var request LSPRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			fmt.Printf("Error parsing LSP request: %v\n", err)
			continue
		}

		var response LSPResponse
		response.Jsonrpc = "2.0"
		response.ID = request.ID

		switch request.Method {
		case "initialize":
			response.Result = InitializeResult{
				Capabilities: ServerCapabilities{
					TextDocumentSync: TextDocumentSync{
						OpenClose: true,
						Change:    1,
					},
					HoverProvider:      true,
					DefinitionProvider: true,
				},
				ServerInfo: ServerInfo{
					Name:    "karkain-lsp",
					Version: "0.1.0",
				},
			}
		case "textDocument/hover":
			response.Result = HoverResult{Contents: "Karkain Language Hover Info"}
		case "textDocument/definition":
			response.Result = DefinitionResult{
				URI: "file:///path/to/definition",
				Range: Range{
					Start: Position{Line: 0, Character: 0},
					End:   Position{Line: 0, Character: 10},
				},
			}
		default:
			response.Error = &LSPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not supported: %s", request.Method),
			}
		}

		responseJSON, _ := json.Marshal(response)
		fmt.Println(string(responseJSON))
	}
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printHelp()
		os.Exit(0)
	}

	command := ""
	targetFile := ""
	outputPath := ""
	cfg := codegen.NewConfig()
	verbose := false
	extraArgs := []string{} // extra positional args (e.g., dep name, version)

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
				os.Exit(1)
			}
		case "-o":
			if i+1 < len(args) {
				outputPath = args[i+1]
				i++
			} else {
				fmt.Println("Error: -o flag requires an output file path")
				os.Exit(1)
			}
		case "build", "run", "check", "test", "lsp", "init", "add", "fetch":
			command = arg
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Printf("Error: Unknown flag '%s'\n", arg)
				printHelp()
				os.Exit(1)
			}
			if targetFile == "" {
				targetFile = arg
			} else {
				extraArgs = append(extraArgs, arg)
			}
		}
	}

	// Handle LSP command separately
	if command == "lsp" {
		handleLSP()
		return
	}

	// Handle package management commands
	if command == "init" || command == "add" || command == "fetch" {
		handlePackageCommand(command, targetFile, extraArgs)
		return
	}

	// Handle test command - path may be a directory
	if command == "test" {
		testPath := targetFile
		if testPath == "" {
			testPath = "."
		}
		result := cli.TestCommand(testPath, cfg, verbose)
		fmt.Print(result.Message)
		os.Exit(result.ExitCode)
	}

	// All other commands require a .kar file
	if targetFile == "" {
		fmt.Println("Error: No input .kar file specified")
		printHelp()
		os.Exit(1)
	}

	if err := cli.ValidateKarFile(targetFile); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if command == "" {
		command = "run"
	}

	var result cli.CommandResult
	switch command {
	case "run":
		result = cli.RunCommand(targetFile, cfg, verbose)
	case "build":
		result = cli.BuildCommand(targetFile, outputPath, cfg, verbose)
	case "check":
		result = cli.CheckCommand(targetFile, verbose)
	default:
		fmt.Printf("Error: Unknown command '%s'\n", command)
		printHelp()
		os.Exit(1)
	}

	if result.Message != "" {
		fmt.Println(result.Message)
	}
	os.Exit(result.ExitCode)
}
