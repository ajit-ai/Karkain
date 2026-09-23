# Self-Hosted Compiler Architecture

## Overview

Phase 88 establishes the foundation for Karkain's self-hosted compiler. The goal is to transition from the Go bootstrap compiler to a Karkain-owned compiler implementation.

## Architecture

```
Bootstrap compiler (Go)          Self-hosted compiler (Karkain → C23 → native)
┌─────────────────────┐          ┌──────────────────────────────────────────┐
│ pkg/lexer/           │          │ src/compiler/ast.kark    — AST types     │
│ pkg/parser/          │   ──►   │ src/compiler/lexer.kark  — Lexer        │
│ pkg/codegen/         │  build  │ src/compiler/parser.kark — Parser       │
│ pkg/sema/            │          │ src/compiler/sema.kark   — Sema         │
│ pkg/ir/ssa/          │          │ src/compiler/codegen.kark — C23 codegen │
└─────────────────────┘          │ src/compiler/main.kark  — Entry point   │
                                 └──────────────────────────────────────────┘
```

## Bootstrap Workflow

```
1. Build bootstrap compiler:
   go build -o karkain.exe ./cmd/karkain/

2. Use bootstrap to compile self-hosted compiler to C23:
   ./karkain.exe build src/compiler/main.kark --target c23

3. Compile C23 output with gcc:
   gcc -std=c99 -o kcc.exe src/compiler/main.c -lm -lgmp

4. Run self-hosted compiler:
   ./kcc.exe check <file.kark>
   ./kcc.exe build <file.kark>
```

Or use the build script:
```bash
bash scripts/build-kcc.sh
```

## Self-Hosted Compiler Components

### AST (`src/compiler/ast.kark`)
- Token kind constants (TK_INT_LIT, TK_FUNC, etc.)
- Location tracking (line, col, offset)
- AST node constructors (Program, FuncDecl, VarDecl, etc.)

### Lexer (`src/compiler/lexer.kark`)
- Character-by-character scanning
- Keyword lookup (20+ keywords)
- Token production with location tracking

### Parser (`src/compiler/parser.kark`)
- Recursive descent parser
- Pratt expression parsing
- Source text tracking for zero-copy tokens
- C import block parsing

### Semantic Analysis (`src/compiler/sema.kark`)
- Symbol table
- Type checking
- Undefined-identifier detection

### Code Generation (`src/compiler/codegen.kark`)
- Multi-target: C23, WGSL, OpenCL, OpenQASM, QIR, KBC
- C11 runtime generation (Value type system, file I/O, etc.)

### Entry Point (`src/compiler/main.kark`)
- CLI argument handling
- check/build/run/test/lsp commands
- Multi-file source loading

## Current Status (Phase 88)

### What Works
- Bootstrap compiler can compile the self-hosted compiler
- Self-hosted compiler can check/build .kark programs
- C23 output compiles to native executable with gcc
- 7 representative test programs exercise the pipeline

### Supported Subset
- Identifiers and literals (int, float, string, bool)
- Variable declarations (var, let)
- Basic expressions and arithmetic
- Function declarations and calls
- Return statements
- Basic control flow (if/else)
- Print statements

### Limitations
- No semantic analysis in self-hosted compiler (undefined functions not reported)
- No while/for loops in self-hosted parser
- No struct/enum support in self-hosted parser
- No module import resolution in self-hosted compiler
- Windows `run` command has path issues (`./` vs `.\`)

## Test Programs

Located in `kcc-tests/`:
- `01_hello.kark` — Basic print
- `02_arithmetic.kark` — Expressions and precedence
- `03_functions.kark` — Function declarations and calls
- `04_variables.kark` — Variable declarations and assignment
- `05_return.kark` — Return values
- `06_control_flow.kark` — If/else
- `07_error.kark` — Error reporting

## Key Files

| File | Purpose |
|------|---------|
| `src/compiler/main.kark` | Self-hosted compiler entry point |
| `src/compiler/ast.kark` | AST node definitions |
| `src/compiler/lexer.kark` | Tokenizer |
| `src/compiler/parser.kark` | Recursive descent parser |
| `src/compiler/sema.kark` | Semantic analysis |
| `src/compiler/codegen.kark` | Multi-target code generation |
| `src/compiler/main.c` | Generated C23 output (from bootstrap) |
| `scripts/build-kcc.sh` | Build script |
| `kcc-tests/*.kark` | Test programs |
| `pkg/cli/phase88_test.go` | Go tests for Phase 88 |

## Differences from Bootstrap Compiler
| Feature | Bootstrap (Go) | Self-hosted (Karkain) |
|---------|----------------|----------------------|
| Lexer | 69 token types | 44 token types |
| Parser | Pratt + full syntax | Recursive descent subset |
| AST | Go structs | Array-based nodes |
| Sema | Full resolution | Basic symbol table |
| Codegen | SSA → C23 | Direct C23 emission |
| Diagnostics | Phase-83 model | Basic error messages |
| Modules | Full import resolution | Not implemented |

## Seed Closure (Phase 143) — Go-free self-reproduction

Go is the **historical seed authorship**, not a runtime requirement. The
bootstrap pipeline's stages 2 and 3 invoke only the seed compiler binary
(absolute path) plus gcc — the `go` tool never runs there
(`KARKAIN_ENGINE=go` is Go-CLI-side configuration, inert on the native
seed). Consequently, FROM any working seed, the closure completes with no
Go on PATH:

1. Seed (downloaded per the 142 ceremony, or stage-1 built) transpiles
   `src/compiler/main.kark` → C, gcc links `karkain-compiler2`.
2. `karkain-compiler2` repeats the step → `karkain-compiler3`.
3. Stage 2 and stage 3 are bitwise identical (`VerifyIdentity`).

Gate: `TestBootstrap_SeedClosure` (`pkg/bootstrap/phase143_seed_test.go`)
builds the seed with Go present, scrubs the Go tool's directory from PATH
(proving unresolvability), runs stages 2–3 go-less, and asserts identity.
It skips fast when `go`/`gcc` is absent or free RAM is below the K127
threshold — the full closure runs on capable hosts and CI.

What still needs Go or C (honest boundaries): the Go reference toolchain
itself (always available as an option, never a requirement after the seed
ceremony), and gcc as the linker (native backend is Phase 145).
