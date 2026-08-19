# Karkain Programming Language (v1.0.0)

Karkain is a statically typed, high-performance programming language designed for heterogeneous CPU/GPU computing, native actor concurrency, and Direct-to-Shader compilation.

## Key Features

- **Heterogeneous Runtime:** Write host code and GPU compute kernels (`kernel`) in unified `.kar` source files.
- **Multi-Backend Emission:** Transpiles to C11, WGSL, OpenCL C, and binary SPIR-V bytecode.
- **Actor Model Concurrency:** Built-in mailboxes, worker pools, dynamic channels, and `select` multiplexing.
- **Compile-Time GPU Safety:** Strict static analyzer prevents heap allocation, recursion, and forbidden syscalls inside GPU kernels.
- **Self-Hosting Compiler:** Written in Karkain with a verified 3-stage bootstrap pipeline and SHA-256 byte parity verification.

---

## Installation

Download the latest pre-compiled binary for your target platform from the `releases/` directory:

| OS / Target | Archive File |
| :--- | :--- |
| **Windows (amd64)** | `karkain-v1.0.0-windows-amd64.zip` |
| **Linux (amd64)** | `karkain-v1.0.0-linux-amd64.tar.gz` |
| **macOS (arm64)** | `karkain-v1.0.0-darwin-arm64.tar.gz` |

---

## Command Line Interface (`karkain`)

```powershell
# Create a new Karkain project
karkain init my_app

# Run a source file directly
karkain run src/main.kar

# Check syntax and GPU safety rules without building
karkain check src/main.kar

# Compile into a native executable
karkain build src/main.kar -o bin/my_app.exe

# Execute unit tests
karkain test ./...

# Add dependencies
karkain add stdlib 0.14.0
```
