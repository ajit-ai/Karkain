# Karkain

> **Karkain 1.0.0 (Stable)** — Karkain is a working compiler and language, not an
> empty roadmap. Everything labeled Implemented below is verified by automated
> gates through the real CLI. Everything else is honestly labeled
> `Planned` / `Not Yet Implemented`.

Karkain is a statically typed systems programming language with a
self-hosted compiler, a byte-identical dual-engine pipeline, a real standard
library, native executables, cross-compilation, and an experimental
concurrency surface (WASM is a Go-engine production candidate). It compiles
`.kark` source to C23 and delegates to a
host C compiler (GCC/Clang/MSVC) for final machine code.

**Honesty policy:** the status vocabulary in
[`docs/source/status/index.rst`](docs/source/status/index.rst) governs every
claim in this project — a feature is `Implemented` only when an automated
gate test exercises it through the real pipeline. See the
[capability summary](docs/source/status/compatibility.rst).

## Quick Start

Karkain builds from source with Go 1.21+ and any C compiler.

```bash
git clone https://github.com/ajit-ai/Karkain.git
cd Karkain
go build -o karkain ./cmd/karkain        # Windows: go build -o karkain.exe ./cmd/karkain
./karkain --version                      # Karkain Compiler v1.0.0 (..., Stable Build)
```

Write your first program:

```kark
// hello.kark
func main() {
    println("Hello, Karkain!")
}
```

Run it:

```bash
karkain run hello.kark
karkain check hello.kark      # syntax + semantics, exit 3 on error
karkain build hello.kark      # native executable
```

**Documentation:**

- [Getting Started](docs/source/getting-started/index.rst) — installation, first program, project layout
- [Karkain by Example](docs/source/examples/index.rst) — 52 real, runnable programs
- [Language Guide](docs/source/language/index.rst) — the language as it actually works
- [Standard Library](docs/source/stdlib/index.rst) — `std.string`, `std.collections`, `std.io`, `std.encoding`, `std.crypto`, `std.testing`
- [CLI Reference](docs/source/tools/index.rst) — every command, flag and exit code
- [Compiler Architecture](docs/source/compiler/index.rst) — the Go front end and the self-hosted `kcc` engine
- [Targets & Cross-Compilation](docs/source/targets/index.rst) — host matrix, triples, WASM
- [Status & Roadmap](docs/source/status/index.rst) — what works, what is planned
- [Contributing](CONTRIBUTING.md) — how to help
- [Security](SECURITY.md) — private vulnerability reporting

## What works today

Implemented (gated, byte-identical on both the Go front end and the
self-hosted `kcc` engine):

- **Language core** — `let`/`var`/`const`, `func`, `while`, `if/else`,
  `for-in` and C-style `for`, structs (records), enums, `match` on literals
  and `Option`, arrays, strings, maps (with `std.collections` helpers)
- **Standard library** — 6 importable modules with NIST/RFC-verified digest
  and codec behavior
- **CLI** — `check`, `build`, `run`, `test` (+ `--filter`), `transpile`,
  `fmt`, `lint`, `debug`, `prof`, `target`, `pkg`, `workspace`, `clean`,
  `explain`, `bench`, `lsp`, `new`
- **Self-hosting** — `kcc` is the default engine for `check/build/run/test`;
  stage-2 == stage-3 bootstrap identity is proven bitwise identical
- **Cross-compilation** — `karkain build --target <triple>` with honest
  "no cross-linker" diagnostics, never a silent host fallback
- **Runtime error model** — checked div/mod/indexing report
  `runtime error: <kind> at <file>:<line>` with stack traces
- **Developer tooling** — formatter, linter, LSP server, VS Code extension,
  DWARF debug info, incremental compilation, profiling

WASM bounds (Go engine; the self-hosted `kcc` parity is the documented
boundary):

- **WASM target** (Phases 108/123) — `karkain build --target wasm32-wasi`
  emits deterministic WASM binaries; run with `wasmtime`. WASI exit codes,
  `stderr` diagnostics and `getArgs()` are implemented. Classified
  **production candidate**.

Neuro/heterogeneous work:

- **AI / ML** — hand-rolled arithmetic demonstrations only (nearest
  neighbor, linear classifier, regression, gradient descent). No tensor
  language surface.
- **Scientific computing / finance / security** — Newton sqrt, numerical
  integration, statistics, matrix multiply, compound interest, loan
  amortization, net-present value, SHA-256/SHA-512 digests, hex/Base64, UTF-8
  round-trips. All real.

## What is planned or not yet implemented

| Capability | Status |
|---|---|
| Networking (TCP/UDP/HTTP) | Not Yet Implemented |
| Databases | Not Yet Implemented |
| Web framework | Not Yet Implemented |
| Quantum language surface | Not Yet Implemented (infrastructure only) |
| GPU/NPU kernel language surface | Not Yet Implemented from `.kark` |
| Advanced package registry | Not Yet Implemented (local resolution + lockfiles only) |
| Pre-built release binaries | Available — 13 `v1.0.0` archives (see the [Installation guide](docs/source/getting-started/installation.rst)) |

## Experimental surface

Experimental means *real and testable, but the surface may change*:

- **Concurrency runtime** (Phase 107) — `spawn`/`join`, channels, actors.
  Go engine only; kcc parity deferred.
- **SIMD/vector types** (Phase 106) — `[N]f32`/`[N]f64`/`[N]i32`/`[N]i64`
  with `@simd_*` builtins.
- **Profiling & debug trace** — `karkain prof` and `karkain debug`, both
  Go-engine instrumentation.

## Example corpus

`examples/` holds **52 real `.kark` programs across 15 categories** — 50
Runnable (incl. test-mode), 2 Experimental, 4 Planned (README only).

```bash
karkain run examples/01-fundamentals/01_hello_world.kark
karkain test examples/15-developer-tools/02_assertions_test.kark
powershell -ExecutionPolicy Bypass -File scripts\verify-examples.ps1
```

Every Runnable example is pinned to a golden output on **both engines** by
`pkg/cli/phase114_examples_test.go`. The authoritative inventory and status
per file lives in [`examples/EXAMPLES.md`](examples/EXAMPLES.md).

## Language at a glance

```kark
// Functions
func add(a: int, b: int) -> int {
    return a + b
}

// Control flow
func classify(n: int) -> string {
    if n < 10 {
        return "small"
    } else {
        return "big"
    }
}

// Records
struct Counter {
    value: int
    owner: string
}

// Options and match
func maybe(x: int) {
    let o: Option<int> = Some(x)
    match o {
        Some(v) => println(v)
        None => println("none")
    }
}

func main() {
    let c = Counter { value: 1, owner: "karkain" }
    println(add(c.value, 41))
    println(classify(42))
    maybe(7)
}
```

`std.encoding` / `std.crypto` modules:

```kark
import std.crypto
import std.encoding

func main() {
    println(hex_encode(bytes("karkain")))   // 6b61726b61696e
    println(sha256("abc"))                   // ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad
}
```

## CLI at a glance

```text
karkain run hello.kark          # compile and run (default)
karkain build hello.kark        # native executable
karkain transpile hello.kark    # keep the generated C source
karkain check hello.kark        # validate syntax + semantics
karkain test path/              # discover *_test.kark, run, --filter
karkain fmt hello.kark          # canonicalize formatting (--check to verify)
karkain lint hello.kark         # full front-end analysis incl. borrow checking
karkain debug hello.kark        # opt-in function enter/leave trace (Go engine)
karkain prof hello.kark         # per-function counts/timings/flame-graph data (Go engine)
karkain target                  # host triple + supported target matrix
karkain build hello.kark --target wasm32-wasi   # WASM (needs wasmtime to run)
karkain lsp                     # Language Server Protocol over stdio
karkain pkg init myapp          # new project skeleton
```

Use `-h/--help` on any command for its full options. Exit codes: 0 success,
1 program/failure, 2 usage, 3 compile/check error, 4 test failure, 6 missing
toolchain, 7 lint findings.

## Building from source

**Prerequisites:** Go 1.21+, a C compiler (GCC, Clang, or MSVC), Git.

```bash
# Clone
git clone https://github.com/ajit-ai/Karkain.git
cd Karkain

# Build (Linux/macOS/FreeBSD)
go build -o karkain ./cmd/karkain

# Build (Windows PowerShell)
go build -o karkain.exe ./cmd/karkain

# Test (unit suites)
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1

# Release-readiness gate
go test ./pkg/cli/ -run TestPhase115 -count=1

# Docs (Sphinx)
python -m pip install -r docs/requirements.txt
python -m sphinx -b html docs/source docs/build/html -W --keep-going
```

The default engine is the self-hosted `kcc` compiler. The Go front end is
selected with `KARKAIN_ENGINE=go` or `--engine go`.

## Cross-compilation

`karkain build --target <triple>` compiles for a requested
architecture/OS. Same-machine targets reuse the host C toolchain; foreign
targets use a triple-prefixed GNU cross-gcc (then clang `--target`). Without
a cross-linker the build fails deterministically with a diagnostic listing
exactly what was searched — never a wrong-architecture binary.

```bash
karkain build app.kark --target x86_64-windows   # native PE32+ on Windows
karkain build app.kark --target x86_64-linux     # needs a cross-linker here
karkain target                                   # host triple + matrix
```

`karkain run --target <foreign>` is refused (cross-run needs an emulator or
remote target). `wasm32-wasi` uses the Karkain-owned WASM backend (production
candidate, `wasmtime`-gated).

## Project structure

```
Karkain/
├── cmd/karkain/         # CLI entry point
├── pkg/
│   ├── lexer/ parser/ sema/ codegen/ ir/ssa/
│   ├── cli/             # command handlers + phase gates
│   ├── pm/              # package manager (local resolution)
│   ├── target/          # cross-compilation target model
│   ├── wasm/            # wasm32-wasi emitter
│   ├── compiler/        # incremental compilation cache
│   ├── bootstrap/       # self-hosting bootstrap
│   ├── lsp/ diagnostics/ source/ testing/ runtime/ backend/ npu/
├── src/compiler/        # self-hosted compiler, written in .kark
├── stdlib/              # Karkain standard library sources (.kark)
├── conformance/         # 59 native test_* functions, both engines
├── examples/            # 52 examples across 15 categories
├── scripts/             # build / release / verify scripts
├── docs/                # Sphinx documentation + phase audit reports
├── .github/workflows/   # CI and docs publishing
├── go.mod
├── LICENSE
├── CONTRIBUTING.md
└── README.md
```

## Roadmap

See [`ROADMAP.md`](ROADMAP.md) for the full development plan (phases 50–119
delivered, plus the current post-1.0.0 track). The project label is now
**Karkain 1.0.0 (Stable)**, released from the Phase 119 milestone.

High-level status today: the compiler is self-hosted, byte-identical on two
engines, and the standard library, toolchain and example corpus are real and
regression-gated — but networking, databases, web, quantum, GPU/NPU language
surfaces, and the package registry are `Planned`/`Not Yet Implemented`. The
1.0.0 release verdict and evidence are documented in the Phase 119 QA report
(`docs/audit/PHASE-119-LANGUAGE-QA-FINAL-REPORT.md`).

## Contributing

Feedback and contributions are welcome. Start with
[CONTRIBUTING.md](CONTRIBUTING.md), then:

1. Fork the repository.
2. Create a feature branch off `develop`.
3. Build and test:
   `go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1`
4. Submit a pull request (PRs are reviewed on the `develop` branch).

Report bugs and feature requests at
[https://github.com/ajit-ai/Karkain/issues](https://github.com/ajit-ai/Karkain/issues).
Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.yml); include
`karkain --version` output and a minimal source file. For security
vulnerabilities use the **private** path in [SECURITY.md](SECURITY.md) — do
not open a public issue.

## Credits

Karkain is developed and maintained by [Ajit Kumar](AUTHORS). See
[AUTHORS](AUTHORS) for the full attribution list and contributor guidelines.

## License

MIT — see [LICENSE](LICENSE). In short: use it, modify it, ship it; the
software is provided "as is" without warranty.