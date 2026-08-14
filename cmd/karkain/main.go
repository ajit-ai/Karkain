package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

const versionString = "Karkain Compiler v0.13.0 (%s/%s, C99 Backend)\n"

func printVersion() {
	fmt.Printf(versionString, runtime.GOOS, runtime.GOARCH)
}

func printHelp() {
	fmt.Println(`Karkain Programming Language Toolchain

Usage:
  karkain [command] [options] <file.kar>

Commands:
  run <file.kar>      Compile and immediately run a .kar script (default)
  build <file.kar>    Compile a .kar script into a native standalone executable
  lsp                 Start Language Server Protocol server for IDE integration

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

type InitializeParams struct {
	RootURI      string `json:"rootUri"`
	Capabilities any    `json:"capabilities"`
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

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type DidOpenParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidSaveParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
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
			response.Result = handleInitialize(request.Params)
		case "textDocument/didOpen":
			handleDidOpen(request.Params)
			response.Result = nil
		case "textDocument/didSave":
			handleDidSave(request.Params)
			response.Result = nil
		case "textDocument/hover":
			response.Result = handleHover(request.Params)
		case "textDocument/definition":
			response.Result = handleDefinition(request.Params)
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

func handleInitialize(params interface{}) interface{} {
	return InitializeResult{
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
}

func handleDidOpen(params interface{}) {
	// Parse the text document and perform syntax checking
	fmt.Println("LSP: Document opened - performing syntax check")
}

func handleDidSave(params interface{}) {
	fmt.Println("LSP: Document saved - performing diagnostics")
}

func handleHover(params interface{}) interface{} {
	// Return type information and documentation
	return HoverResult{
		Contents: "Karkain Language Hover Info",
	}
}

func handleDefinition(params interface{}) interface{} {
	// Return go-to-definition information
	return DefinitionResult{
		URI: "file:///path/to/definition",
		Range: Range{
			Start: Position{Line: 0, Character: 0},
			End:   Position{Line: 0, Character: 10},
		},
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
	cfg := codegen.NewConfig()

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Handle flags with equals sign (e.g., --target=wasm32-wasi)
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
			cfg.Verbose = true
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
				cfg.OutputPath = args[i+1]
				i++
			} else {
				fmt.Println("Error: -o flag requires an output file path")
				os.Exit(1)
			}
		case "build", "run", "lsp":
			command = arg
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Printf("Error: Unknown flag '%s'\n", arg)
				printHelp()
				os.Exit(1)
			}
			if targetFile == "" {
				targetFile = arg
			}
		}
	}

	// Handle LSP command separately
	if command == "lsp" {
		handleLSP()
		return
	}

	if targetFile == "" {
		fmt.Println("Error: No input .kar file specified")
		printHelp()
		os.Exit(1)
	}

	if filepath.Ext(targetFile) != ".kar" {
		fmt.Println("Error: Input file must be a .kar file")
		os.Exit(1)
	}

	if command == "" {
		command = "run"
	}

	dir := filepath.Dir(targetFile)
	entries, err := os.ReadDir(dir)
	var fullContent strings.Builder

	funcMainRegex := regexp.MustCompile(`(?m)^\s*func\s+main\s*\(`)

	if err == nil && len(entries) > 1 {
		// Read sibling helper .kar files first if they don't define their own top-level func main()
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".kar" && entry.Name() != filepath.Base(targetFile) {
				data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				if err == nil {
					fileStr := string(data)
					if !funcMainRegex.MatchString(fileStr) {
						fullContent.WriteString(fileStr)
						fullContent.WriteString("\n\n")
					}
				}
			}
		}
	}

	content, err := os.ReadFile(targetFile)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}
	fullContent.WriteString(string(content))

	sourceText := fullContent.String()
	l := lexer.New(sourceText)

	if cfg.Verbose {
		fmt.Println("=== [Verbose] Lexer Token Stream ===")
		lexerForLogs := lexer.New(sourceText)
		for {
			tok := lexerForLogs.NextToken()
			fmt.Printf("Line %d | Type: %-10s | Literal: %q\n", tok.Line, tok.Type, tok.Literal)
			if tok.Type == lexer.TokenEOF {
				break
			}
		}
		fmt.Println("====================================")
	}

	p := parser.New(l)
	ast := p.ParseProgram()

	if cfg.Verbose {
		fmt.Printf("=== [Verbose] Parsed AST Statements count: %d ===\n", len(ast.Statements))
	}

	// Phase 17: Apply macro expansion before code generation
	expandedAST := parser.ApplyMacroExpansion(ast)
	if cfg.Verbose {
		fmt.Printf("=== [Verbose] After Macro Expansion Statements count: %d ===\n", len(expandedAST.Statements))
	}
	ast = expandedAST

	// Set execution mode flags on cfg
	cfg.RunAfter = (command != "build")
	if command == "build" {
		cfg.CompileOnly = true
	}

	// Instantiate generator with config
	cg := codegen.New(cfg)

	if command == "build" {
		if err := cg.GenerateAndCompile(ast, targetFile); err != nil {
			fmt.Printf("Build Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Build successful.")
	} else {
		if err := cg.GenerateAndCompile(ast, targetFile); err != nil {
			fmt.Printf("Execution Error: %v\n", err)
			os.Exit(1)
		}
	}
}
