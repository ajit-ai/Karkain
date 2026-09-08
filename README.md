# Karkain Programming Language

**Fast. Safe. Heterogeneous.**

Karkain is a statically typed, high-performance systems programming language designed for heterogeneous CPU/GPU/quantum computing, native actor concurrency, and compile-time memory safety. It compiles to C23 and delegates to GCC/Clang/MSVC for final machine code — giving you portability without sacrificing speed.

> **Language specification:** see [`SPEC.md`](SPEC.md) for the authoritative,
> versioned spec of the Karkain language (keywords, grammar, types, memory model,
> and conformance status).

```
.kark source → Lexer → Parser → SSA IR → Optimizer → Verifier → C23 → GCC/Clang → Binary
                                    ↓
                              GPU Shaders (WGSL, SPIR-V, OpenCL)
                                    ↓
                              Quantum Circuits (OpenQASM 3.0, QIR)
```

---

## Table of Contents

- [Design Philosophy](#design-philosophy)
- [Language Architecture](#language-architecture)
- [Compiler Pipeline](#compiler-pipeline)
- [Key Features](#key-features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Developer Showcase](#developer-showcase)
- [CLI Reference](#cli-reference)
- [Package Manager (KPM)](#package-manager-kpm)
- [Project Structure](#project-structure)
- [Building from Source](#building-from-source)
- [Cross-Compilation](#cross-compilation)
- [Platform Support](#platform-support)
- [Roadmap](#roadmap)
- [License](#license)

---

## Design Philosophy

Karkain is built on four non-negotiable principles:

1. **Speed** — Zero-cost abstractions, value semantics, no garbage collector. Compiles to C23, optimized by mature C compilers (GCC, Clang, MSVC).

2. **Safety** — Compile-time ownership and borrow checking. No null pointers, no use-after-free, no data races. Option/Result types replace exceptions.

3. **Heterogeneous** — Write CPU code and GPU compute kernels in the same `.kark` file. Quantum circuits compile to OpenQASM/QIR. One language, all hardware.

4. **Independence** — The ultimate goal: `karkain` compiles itself. No dependency on Go, Rust, or any other toolchain for day-to-day development.

### What Karkain is NOT

- NOT a managed language (no GC, no VM)
- NOT an interpreted language
- NOT LLVM-based (uses GCC/Clang as backend, not LLVM frontend)
- NOT a scripting language (despite having a REPL planned)

---

## Language Architecture

### Type System

```karkain
// Primitives
let x: int = 42
let pi: float64 = 3.14159
let flag: bool = true
let msg: string = "hello"

// Algebraic Data Types
enum Option<T> { Some(T), None }
enum Result<T, E> { Ok(T), Err(E) }

// Structs with ownership
struct Point { x: float64, y: float64 }
struct Buffer { data: *u8, len: int }

// Generics with trait bounds
fn max<T: Ord>(a: T, b: T) -> T { ... }

// Traits
trait Drawable { fn draw(self); }
impl Drawable for Point { fn draw(self) { print(self.x, self.y) } }
```

### Memory Model

Karkain uses a hybrid ownership model:

```karkain
// Owned values (stack-allocated, freed on scope exit)
let a = Point { x: 1.0, y: 2.0 }

// Borrows (immutable references, checked at compile time)
fn distance(p: &Point) -> float64 { ... }

// Mutable borrows (exclusive access)
fn translate(p: &mut Point, dx: float64) { p.x += dx }

// Raw pointers (escape hatch, unsafe boundary)
let raw: *mut int = @addr(x)

// Manual allocation (for performance-critical paths)
let buf = alloc<u8>(1024)
free(buf)
```

### Concurrency

```karkain
// Actor model
actor Counter {
    state { count: int }
    on Increment { self.count += 1 }
    on GetCount -> int { return self.count }
}

// Channels and coroutines
co producer(ch: Send<int>) {
    for i in 0..10 { ch <- i }
}

co consumer(ch: Recv<int>) {
    loop { let val = <-ch; print(val) }
}

// Select multiplexing
select {
    case msg := <-ch1 { handle(msg) }
    case msg := <-ch2 { handle(msg) }
    default { idle() }
}
```

### GPU Kernels

```karkain
kernel matmul(A: *float64, B: *float64, C: *float64, M: int, N: int, K: int) {
    let row = global_id(0)
    let col = global_id(1)
    var sum: float64 = 0.0
    for k in 0..K {
        sum += A[row * K + k] * B[k * N + col]
    }
    C[row * N + col] = sum
}
```

Compiles to WGSL (WebGPU), OpenCL C, and SPIR-V automatically.

### Quantum Computing

```karkain
qreg q[4]

// Quantum gates
H q[0]
CNOT q[0], q[1]
Rx(0.5) q[2]
Rz(1.2) q[3]

// Measurement
let result = measure q[0]
print(result)
```

Compiles to OpenQASM 3.0, QIR (LLVM IR), and OpenPulse pulse schedules.

---

## Compiler Pipeline

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              COMPILER PIPELINE                               │
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐              │
│  │   Lexer      │───▶│   Parser     │───▶│   Macro      │              │
│  │  (tokens)    │    │   (AST)      │    │   Expand     │              │
│  └──────────────┘    └──────────────┘    └──────────────┘              │
│                                       │                     │
│                                       ▼                     │
│                              ┌──────────────┐             │
│                              │ Semantic     │             │
│                              │ Analysis     │             │
│                              │ • borrow check│             │
│                              │ • type check  │             │
│                              │ • quantum safe│             │
│                              └──────────────┘             │
│                                      │                      │
│                                      ▼                      │
│  ┌──────────────────────────────────────────────────────┐      │
│  │                SSA IR Pipeline                    │      │
│  │                                                   │      │
│  │  AST → SSA → Constant Fold → DCE → Verify → C23 │      │
│  └──────────────────────────────────────────────────────┘      │
│                         │                                   │
│              ┌──────────┼──────────┐                       │
│              ▼          ▼          ▼                        │
│         ┌──────────┐ ┌──────────┐ ┌──────────┐                │
│         │  C23     │ │  WGSL    │ │OpenQASM 3│                │
│         │ (CPU)    │ │ (GPU)    │ │ (Quantum)│                │
│         └──────────┘ └──────────┘ └──────────┘                │
│             │          │           │                        │
│             ▼          ▼           ▼                        │
│         ┌──────────┐ ┌──────────┐ ┌──────────┐                │
│         │ GCC/     │ │ WebGPU   │ │ IBM/     │                │
│         │ Clang    │ │ Runtime  │ │ Azure Q  │                │
│         └──────────┘ └──────────┘ └──────────┘                │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Compiler Stages

| Stage | Package | What it does |
|-------|---------|-------------|
| **Lexer** | `pkg/lexer/` | Tokenizes `.kark` source into token stream |
| **Parser** | `pkg/parser/` | Builds AST with arena allocation, line tracking |
| **Macro Expander** | `pkg/sema/macro.go` | Hygienic macro expansion, `@derive`, `@unroll`, `@target_guard` |
| **Borrow Checker** | `pkg/sema/borrow_checker.go` | Ownership, borrowing, move semantics, linear types |
| **Type Checker** | `pkg/sema/` | Generics, traits, monomorphization |
| **Quantum Safety** | `pkg/sema/quantum.go` | No-cloning theorem, measurement collapse, gate-after-measurement |
| **Autodiff** | `pkg/sema/autodiff.go` | Tensor shape checking, computation graph, reverse-mode differentiation |
| **SSA Lowering** | `pkg/codegen/lower.go` | AST → SSA IR with variable cells, CFG |
| **SSA Optimizer** | `pkg/ir/ssa/opt.go` | Constant folding, dead code elimination |
| **SSA Verifier** | `pkg/ir/ssa/verify.go` | Type checking, control flow validation |
| **C Emitter** | `pkg/codegen/emit_ir.go` | SSA IR → C23 source code |
| **Legacy Emitter** | `pkg/codegen/codegen.go` | Direct AST → C99 (fallback when SSA fails) |
| **GPU Emitter** | `pkg/codegen/wgsl.go`, `gpu.go`, `spirv.go` | Kernel → WGSL, OpenCL, SPIR-V |
| **Quantum Emitter** | `pkg/codegen/qasm.go`, `qir.go` | Circuit → OpenQASM, QIR |

---

## Key Features

### Compile-Time Safety

- **Ownership & Borrow Checking** — Values have single owners. Borrows are tracked at compile time. No use-after-free, no data races.
- **Option/Result Types** — No null pointers. `Option<T>` forces explicit handling of missing values. `Result<T, E>` replaces exceptions.
- **Exhaustive Match** — `match` must cover all variants. The compiler rejects incomplete pattern matches.
- **Linear Types** — Types marked `linear` must be used exactly once. Prevents resource leaks.
- **Quantum Safety** — The no-cloning theorem is enforced at compile time. Measurement collapse is tracked.

### Performance

- **Value Semantics** — Structs are stack-allocated by default. No hidden boxing, no pointer chasing.
- **No Garbage Collector** — Manual allocation when needed, automatic scope-based deallocation otherwise.
- **Checked Arithmetic** — `add_checked`, `sub_checked`, `mul_checked` return `Option<int>` on overflow.
- **SIMD Support** — `@simd_add`, `@simd_mul` intrinsics with AVX2 auto-detection at runtime.
- **Matrix Operations** — 64-byte aligned contiguous arrays with AVX2-optimized multiplication.
- **SSA Optimization** — Constant folding, dead code elimination on the IR before C emission.

### Heterogeneous Computing

- **CPU** — Compiles to C23, linked with GCC/Clang/MSVC. Full platform support.
- **GPU** — `kernel` keyword emits WGSL, OpenCL C, and SPIR-V. Host launcher auto-generated.
- **Quantum** — Gate syntax compiles to OpenQASM 3.0, QIR, OpenPulse. Noise simulation built in.
- **WASM** — Target `wasm32-wasi` for web deployment.

### Developer Experience

- **Package Manager** — `karkain pkg add`, semver resolution, lock files, registry.
- **LSP Server** — Hover, go-to-definition for IDE integration.
- **Debug Mode** — `--debug` emits `#line` directives for GDB/LLDB source mapping.
- **Verbose Mode** — `--verbose` shows token stream, AST, generated C, compiler invocation.
- **Test Framework** — `*_test.kark` files with `test_*` functions, auto-discovered and isolated.

---

## Installation

### Pre-compiled Binaries

Download the latest release for your platform:

| Platform | Architecture | File | 
|----------|-------------|------|
| **Windows** | x86_64 | `karkain-v1.0.0-windows-amd64.zip` |
| **Windows** | ARM64 | `karkain-v1.0.0-windows-arm64.zip` |
| **Linux** | x86_64 | `karkain-v1.0.0-linux-amd64.tar.gz` |
| **Linux** | ARM64 | `karkain-v1.0.0-linux-arm64.tar.gz` |
| **Linux** | ARMv7 | `karkain-v1.0.0-linux-armv7.tar.gz` |
| **Linux** | i386 | `karkain-v1.0.0-linux-i386.tar.gz` |
| **Linux** | ppc64le | `karkain-v1.0.0-linux-ppc64le.tar.gz` |
| **Linux** | s390x | `karkain-v1.0.0-linux-s390x.tar.gz` |
| **macOS** | Apple Silicon (M1/M2/M3/M4) | `karkain-v1.0.0-darwin-arm64.tar.gz` |
| **macOS** | Intel | `karkain-v1.0.0-darwin-amd64.tar.gz` |
| **FreeBSD** | x86_64 | `karkain-v1.0.0-freebsd-amd64.tar.gz` |
| **NetBSD** | x86_64 | `karkain-v1.0.0-netbsd-amd64.tar.gz` |
| **OpenBSD** | x86_64 | `karkain-v1.0.0-openbsd-amd64.tar.gz` |

### Platform-Specific Install Instructions

#### Windows

```powershell
# Option 1: Download and extract
Invoke-WebRequest -Uri "https://github.com/ajit-ai/Karkain/releases/latest/download/karkain-v1.0.0-windows-amd64.zip" -OutFile "karkain.zip"
Expand-Archive -Path "karkain.zip" -DestinationPath "C:\karkain"
$env:PATH += ";C:\karkain"

# Option 2: Build from source (requires Go 1.21+ and GCC/Clang)
git clone https://github.com/ajit-ai/Karkain.git
cd Karkain
go build -o karkain.exe ./cmd/karkain

# Verify
karkain --version
```

#### Linux (Debian/Ubuntu)

```bash
# Download and install
curl -L https://github.com/ajit-ai/Karkain/releases/latest/download/karkain-v1.0.0-linux-amd64.tar.gz | tar xz
sudo mv karkain /usr/local/bin/

# Or build from source (requires Go 1.21+ and GCC)
git clone https://github.com/ajit-ai/Karkain.git
cd Karkain
go build -o karkain ./cmd/karkain
sudo mv karkain /usr/local/bin/

# Install C compiler if not present
sudo apt install gcc build-essential

# Verify
karkain --version
```

#### Linux (Fedora/RHEL/CentOS)

```bash
# Install prerequisites
sudo dnf install gcc golang

# Build from source
git clone https://github.com/ajit-ai/Karkain.git
cd Karkain
go build -o karkain ./cmd/karkain
sudo mv karkain /usr/local/bin/
```

#### Linux (Arch/Manjaro)

```bash
# Install prerequisites
sudo pacman -S gcc go

# Build from source
git clone https://github.com/ajit-ai/Karkain.git
cd Karkain
go build -o karkain ./cmd/karkain
sudo mv karkain /usr/local/bin/
```

#### macOS (Apple Silicon / Intel)

```bash
# Option 1: Download
curl -L https://github.com/ajit-ai/Karkain/releases/latest/download/karkain-v1.0.0-darwin-arm64.tar.gz | tar xz
sudo mv karkain /usr/local/bin/

# Option 2: Build from source (requires Xcode CLI tools + Go)
xcode-select --install
brew install go
git clone https://github.com/ajit-ai/Karkain.git
cd Karkain
go build -o karkain ./cmd/karkain
sudo mv karkain /usr/local/bin/

# Verify
karkain --version
```

#### FreeBSD

```bash
# Install prerequisites
pkg install go gcc

# Build from source
git clone https://github.com/ajit-ai/Karkain.git
cd Karkain
go build -o karkain ./cmd/karkain
mv karkain /usr/local/bin/
```

#### NetBSD / OpenBSD

```bash
# Install prerequisites
# NetBSD: pkgin install go gcc
# OpenBSD: pkg_add go gcc

git clone https://github.com/ajit-ai/Karkain.git
cd Karkain
go build -o karkain ./cmd/karkain
doas mv karkain /usr/local/bin/
```

### Docker

```bash
# Build image
docker build -t karkain .

# Run
docker run --rm -v $(pwd):/src karkain run /src/main.kark
```

### Verify Installation

```bash
karkain --version
karkain --help
```

---

## Quick Start

### Hello World

```bash
# Create project
karkain pkg init hello
cd hello

# Edit src/main.kark
cat > src/main.kark << 'EOF'
fn main() {
    print("Hello, World!")
}
EOF

# Run it
karkain run src/main.kark
```

### Variables and Functions

```karkain
// src/main.kark
fn add(a: int, b: int) -> int {
    return a + b
}

fn main() {
    let x: int = 10
    let y: int = 20
    print(add(x, y))  // 30
}
```

### Structs and Pattern Matching

```karkain
struct Point { x: float64, y: float64 }

fn distance(p: &Point) -> float64 {
    return (p.x * p.x + p.y * p.y) ^ 0.5
}

fn main() {
    let p = Point { x: 3.0, y: 4.0 }
    print(distance(p))  // 5.0
}
```

### GPU Kernels

```karkain
kernel vector_add(A: *float64, B: *float64, C: *float64, N: int) {
    let i = global_id(0)
    if i < N {
        C[i] = A[i] + B[i]
    }
}

fn main() {
    // Host code runs on CPU
    // Kernel runs on GPU (auto-detected)
}
```

---

## Developer Showcase

Real, validated `.kark` programs live in [`examples/showcase/`](examples/showcase/).
Every working example below was checked and run with the real CLI on the
default self-hosted (`kcc`) engine, and each one pins the verified output.

```
karkain check examples/showcase/<category>/<name>/main.kark
karkain run   examples/showcase/<category>/<name>/main.kark
```

| # | Category | Status |
|---|----------|--------|
| 01 | Fundamentals | WORKING TODAY |
| 02 | Algorithms | WORKING TODAY |
| 03 | Systems | WORKING TODAY |
| 04 | Networking | NOT CURRENTLY SUPPORTED |
| 05 | Data | WORKING TODAY |
| 06 | Database | NOT CURRENTLY SUPPORTED |
| 07 | Web | NOT CURRENTLY SUPPORTED |
| 08 | Concurrency | NOT CURRENTLY SUPPORTED |
| 09 | AI | WORKING TODAY |
| 10 | ML | WORKING TODAY |
| 11 | Quantum | NOT CURRENTLY SUPPORTED |
| 12 | Scientific Computing | WORKING TODAY |
| 13 | Finance | WORKING TODAY |
| 14 | Security | WORKING TODAY (non-cryptographic) |
| 15 | Developer Tools | WORKING TODAY |

Highlights: a FIFO queue, a file-processing pipeline, text statistics, vector
math + nearest-neighbor (AI), least-squares linear regression (ML), Newton
sqrt + trapezoid integration + deterministic Monte Carlo pi (Scientific),
finance calculations, a non-cryptographic checksum, and a multi-file banking
app with `karkain test` (4/4) and a `karkain fmt` demo.

The showcase honestly documents real current-capability findings, including
engines gaps: `int(string)` works only on the Go engine (not the default kcc
engine), arrays are passed by value into functions, there is no map iteration
API, and Networking / Database / Web / Concurrency / Quantum are not
available today. See [`examples/showcase/README.md`](examples/showcase/README.md)
for the full matrix and findings.

---

## CLI Reference

### Compiler Commands

| Command | Description |
|---------|-------------|
| `karkain run <file.kark>` | Compile and run (default) |
| `karkain build <file.kark>` | Compile to native executable |
| `karkain transpile <file.kark>` | Generate C source (keeps .c file) |
| `karkain check <file.kark>` | Validate syntax and semantics |
| `karkain test <path>` | Discover and run `*_test.kark` files |
| `karkain lsp` | Start Language Server Protocol server |

### Compiler Options

| Flag | Description |
|------|-------------|
| `-o <path>` | Output binary path |
| `-c, --compile-only` | Keep generated C source |
| `-g, --debug` | Debug symbols + `#line` directives |
| `--target <arch>` | Cross-compile (native, wasm32-wasi, aarch64-linux-gnu) |
| `--verbose` | Show token stream, AST, C code, compiler invocation |
| `-v, --version` | Show version |
| `-h, --help` | Show help |

### Package Manager Commands

| Command | Description |
|---------|-------------|
| `karkain pkg init [name]` | Create new project |
| `karkain pkg add <pkg> [version]` | Add dependency |
| `karkain pkg add <pkg> --source git --url <url>` | Git dependency |
| `karkain pkg add <pkg> --source local --url <path>` | Local dependency |
| `karkain pkg remove <pkg>` | Remove dependency |
| `karkain pkg update [pkg]` | Re-resolve versions |
| `karkain pkg upgrade` | Update all to latest compatible |
| `karkain pkg fetch` | Download all dependencies |
| `karkain pkg deps` | List dependencies |
| `karkain pkg deps --tree` | Show dependency tree |
| `karkain pkg deps --outdated` | Check for newer versions |
| `karkain pkg search <query>` | Search package registry |
| `karkain pkg info <pkg>` | Show package details |
| `karkain pkg publish` | Publish to registry |
| `karkain pkg login` | Authenticate with registry |
| `karkain pkg logout` | Clear auth token |
| `karkain pkg whoami` | Show current user |
| `karkain pkg audit` | Check for vulnerabilities |
| `karkain pkg audit --licenses` | License compatibility check |
| `karkain pkg verify` | Verify checksums |
| `karkain pkg cache list` | Show cached packages |
| `karkain pkg cache clean` | Remove all cached packages |
| `karkain pkg cache clean --stale` | Remove packages unused >30 days |
| `karkain pkg cache path` | Show cache directory |
| `karkain pkg workspace init` | Initialize workspace root |
| `karkain pkg workspace add <path>` | Add member package |
| `karkain pkg workspace build` | Build all packages |
| `karkain pkg workspace test` | Test all packages |

---

## Package Manager (KPM)

Karkain includes a built-in package manager accessed via `karkain pkg`.

### Project Structure

```
my-project/
├── karkain.toml              # Manifest (name, version, dependencies)
├── karkain.lock              # Lock file (pinned versions, checksums)
├── src/
│   ├── main.kark              # Entry point
│   └── lib.kark               # Library modules
├── tests/
│   └── main_test.kark         # Test files
└── .karkain/
    └── cache/                # Downloaded packages
```

### Manifest Format (karkain.toml)

```toml
[package]
name = "my-project"
version = "1.0.0"
author = "Your Name"
description = "A Karkain project"
license = "MIT"

[dependencies]
math = { version = "^1.0.0", source = "registry" }
utils = { source = "git", url = "https://github.com/user/utils.git", tag = "v2.0" }
helper = { source = "local", url = "../helper" }

[features]
default = ["std"]
gpu = ["math/gpu"]
```

### Version Syntax

| Syntax | Meaning |
|--------|---------|
| `"1.2.3"` | Exact version |
| `"^1.2.3"` | Compatible (>=1.2.3, <2.0.0) |
| `"~1.2.3"` | Patch-level (>=1.2.3, <1.3.0) |
| `">=1.0 <2.0"` | Range |
| `"1.2.x"` | Wildcard |
| `"*"` | Any version |

---

## Project Structure

```
Karkain/
├── cmd/karkain/              # CLI entry point (main.go)
├── pkg/
│   ├── lexer/                # Tokenizer
│   ├── parser/               # AST + arena allocator
│   ├── codegen/              # C code generation + GPU/quantum emitters
│   ├── ir/ssa/               # SSA intermediate representation
│   ├── sema/                 # Semantic analysis (types, borrow, quantum safety)
│   ├── pm/                   # Package manager (KPM)
│   ├── cli/                  # CLI command handlers
│   ├── jit/                  # JIT compilation engine + FFI
│   ├── lsp/                  # Language Server Protocol
│   ├── bootstrap/            # Self-hosting bootstrap
│   ├── diagnostics/          # Error reporting
│   ├── runtime/              # Runtime support (actors, coroutines, GPU)
│   └── stdlib/               # Standard library (Go)
├── src/compiler/             # Self-hosted compiler (written in .kark)
├── stdlib/                   # Karkain standard library source
├── std/                      # Alternative stdlib location
├── examples/                 # Example programs (25 E2E tests)
├── scripts/                  # Build, release, bootstrap scripts
├── editors/                  # Editor integrations
├── .github/workflows/        # CI/CD (GitHub Actions)
├── go.mod                    # Go module definition
└── README.md                 # This file
```

---

## Building from Source

### Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| **Go** | 1.21+ | Required for compiler build |
| **GCC** or **Clang** | Any recent | Required for compiling generated C code |
| **MSVC** | 2019+ | Windows alternative to GCC/Clang |

### Build Commands

```bash
# Clone
git clone https://github.com/ajit-ai/Karkain.git
cd Karkain

# Build (Linux/macOS/FreeBSD)
go build -o karkain ./cmd/karkain

# Build (Windows)
go build -o karkain.exe ./cmd/karkain

# Build with optimizations
go build -ldflags="-s -w" -o karkain ./cmd/karkain

# Run tests
go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1

# Run E2E tests
for f in examples/*.kark; do karkain run "$f"; done
```

### Using Build Scripts

```bash
# Linux/macOS/FreeBSD
./scripts/build.sh

# Windows PowerShell
.\scripts\build.ps1
```

---

## Cross-Compilation

Karkain supports cross-compilation via the `--target` flag and Go's cross-compilation.

### Supported Targets

| Target | GOOS | GOARCH | Notes |
|--------|------|--------|-------|
| `windows/amd64` | windows | amd64 | Primary development target |
| `windows/arm64` | windows | arm64 | Windows on ARM (Surface Pro X, etc.) |
| `linux/amd64` | linux | amd64 | Primary server target |
| `linux/arm64` | linux | arm64 | Raspberry Pi 4+, AWS Graviton |
| `linux/armv7` | linux | arm | Raspberry Pi 3, embedded |
| `linux/i386` | linux | 386 | Legacy 32-bit |
| `linux/ppc64le` | linux | ppc64le | IBM POWER |
| `linux/s390x` | linux | s390x | IBM Z |
| `darwin/amd64` | darwin | amd64 | Intel Mac |
| `darwin/arm64` | darwin | arm64 | Apple Silicon (M1/M2/M3/M4) |
| `freebsd/amd64` | freebsd | amd64 | FreeBSD server |
| `netbsd/amd64` | netbsd | amd64 | NetBSD |
| `openbsd/amd64` | openbsd | amd64 | OpenBSD |
| `js/wasm` | js | wasm | WebAssembly (via WASI) |

### Cross-Compile for a Specific Target

```bash
# Build for Linux ARM64 from any host
GOOS=linux GOARCH=arm64 go build -o karkain-linux-arm64 ./cmd/karkain

# Build for macOS Intel from Linux
GOOS=darwin GOARCH=amd64 go build -o karkain-darwin-amd64 ./cmd/karkain

# Build for FreeBSD from Linux
GOOS=freebsd GOARCH=amd64 go build -o karkain-freebsd-amd64 ./cmd/karkain
```

### Release Packaging

Use the release script to build all targets and create archives:

```bash
# Linux/macOS/FreeBSD
./scripts/release.sh

# Windows PowerShell
.\scripts\release.ps1
```

This produces archives in `releases/` for all platforms.

---

## Platform Support

Karkain runs wherever Go and a C compiler are available.

| Platform | Status | Notes |
|----------|--------|-------|
| **Windows 10/11 (amd64)** | Full | Primary development platform. MSYS2 GCC or MSVC. |
| **Windows (arm64)** | Full | Surface Pro X, Snapdragon laptops. |
| **Ubuntu 20.04+ (amd64)** | Full | `apt install gcc build-essential` |
| **Ubuntu (arm64)** | Full | Raspberry Pi 4, AWS Graviton. |
| **Debian 11+ (amd64)** | Full | Stable server deployment. |
| **Fedora 38+ (amd64)** | Full | `dnf install gcc` |
| **RHEL/CentOS 9+ (amd64)** | Full | Enterprise server. |
| **Arch Linux (amd64)** | Full | `pacman -S gcc` |
| **macOS 12+ (Apple Silicon)** | Full | M1/M2/M3/M4. Xcode CLI tools. |
| **macOS 12+ (Intel)** | Full | Xcode CLI tools. |
| **FreeBSD 13+ (amd64)** | Full | `pkg install gcc` |
| **NetBSD 10+ (amd64)** | Full | `pkgin install gcc` |
| **OpenBSD 7+ (amd64)** | Full | `pkg_add gcc` |
| **Alpine Linux (amd64)** | Full | `apk add gcc musl-dev` |
| **Docker (any)** | Full | Multi-stage build supported. |
| **WASM (wasm32-wasi)** | Partial | Compile-only, no runtime execution yet. |

### Runtime Requirements

| Component | Required? | Purpose |
|-----------|-----------|---------|
| **Go 1.21+** | Build only | Compiles the Karkain compiler itself |
| **GCC/Clang/MSVC** | Runtime | Compiles generated C code to machine code |
| **GMP library** | Optional | Arbitrary precision arithmetic (`-lgmp` when compiling manually) |
| **Git** | Optional | Package manager git dependencies |
| **curl/wget** | Optional | Package manager registry access |

---

## Roadmap

### Current Progress

| Phase | Status |
|-------|--------|
| 50: Bug fixes & correctness | Done |
| 51: Borrow checker lexical scoping | Done |
| 52: Value-by-value runtime API | Done |
| 53: SSA IR infrastructure | Done |
| 54: Closure & capture semantics | Done |
| 55: String & slice types | Done |
| 55b: Deep equality, checked arithmetic | Done |
| 55c: HTTP ifdef, AST line tracking, `#line` | Done |
| 55d: KPM package manager | Done |
| 56: Self-Hosting Compiler | In Progress |
| 57: Actor & Concurrency Runtime | Done |
| 58: GPU Kernel Integration | Done |
| 59: Quantum Pipeline | Done |
| 60: Standard Library | Done |
| 62: Diagnostics | Done |
| 63: Full Borrow Checker + Ownership | Done |
| 64-67: IR Optimizer + Toolchain | Done |
| 68: Self-Hosting Completion (INDEPENDENCE) | In Progress |
| 69: Ecosystem Hardening + v1.0 | Done |
| 70: SIMD Vector Types & Atomics | Done |
| 71: Math IR Foundation | Done |
| 72: Tensor IR Foundation | Done |
| 73: CPU Reference Backend | Done |
| 74: Autodiff Integration | Done |
| 75: Backend Abstraction | Done |
| 76: GPU/WGSL Backend | Done |
| 77: NPU Abstraction + Vendor Adapters | Done |
| 78: NPU Optimization | Done |
| 79: Compiler Integrity / IR Architecture / Self-Hosting Readiness Audit | Done |
| 83: Enhanced Diagnostic & Warning Foundation | Done |

### INDEPENDENCE Milestone

The ultimate goal: `karkain` compiles `.kark` source including its own compiler source, with zero dependency on any other language's toolchain.

```
karkain build src/compiler/main.kark  # Karkain compiles itself
```

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes
4. Run tests: `go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/... ./pkg/pm/... -count=1`
5. Ensure E2E passes: `for f in examples/*.kark; do karkain run "$f"; done`
6. Submit a pull request

---

## License

MIT License

---

## Links

- **Repository**: https://github.com/ajit-ai/Karkain
- **Releases**: https://github.com/ajit-ai/Karkain/releases
- **Issues**: https://github.com/ajit-ai/Karkain/issues
