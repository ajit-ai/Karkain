package main

import (
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

const versionString = "Karkain Compiler v0.5.0 (%s/%s, C99 Backend)\n"

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

Options:
  -o <path>           Specify custom output binary file path (used with build)
  -c, --compile-only  Keep generated C source code file (temp_runner.c) on disk
  --verbose           Emit detailed pipeline logs (Tokens, AST, C Code, Compiler Invocation)
  -v, --version       Show version information
  -h, --help          Show this help message

Examples:
  karkain run examples/array_test.kar
  karkain build examples/compiler_test.kar -o bin/app.exe
  karkain examples/phase1_test.kar --verbose`)
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printHelp()
		os.Exit(0)
	}

	command := ""
	targetFile := ""
	cfg := codegen.Config{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
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
		case "-o":
			if i+1 < len(args) {
				cfg.OutputPath = args[i+1]
				i++
			} else {
				fmt.Println("Error: -o flag requires an output file path")
				os.Exit(1)
			}
		case "build", "run":
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
