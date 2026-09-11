# Cross-Compilation Examples (Phase 111)

Each program is written in canonical Karkain and compiles unchanged for every
supported `--target` triple. Builds are host-exact: same-machine targets reuse
the host C toolchain, cross targets use a triple-prefixed GNU cross-gcc (or
clang `--target`), and a missing cross-linker fails deterministically instead
of silently producing a host binary.

| Program | What it verifies |
|---------|------------------|
| `hello.kark` | Minimal target: single deterministic stdout line. Golden for the Phase 111 CLI tests. |
| `functions.kark` | Value model across targets: i64 arithmetic, calls, recursion. `fib(14) = 377`. |
| `collections.kark` | Reference/pointer-sized data (arrays + strings) so per-target data layout stays honest. |
| `platform.kark` | Target-agnostic behavior: deterministic 64-bit arithmetic (no `target.os`/`arch` builtins yet — the Phase 111 target contract lives in generated C via `KARKAIN_TARGET_*` defines). |

## Usage

```bash
# Build and run a real executable for this host (PE32+ x86-64 on Windows x86_64)
karkain build examples/cross_compile/hello.kark --target x86_64-windows
./hello.exe

# Cross-target with a cross-linker installed (mechanism identical; artifacts
# are N/A on hosts without the cross toolchain — the build fails with a
# ToolchainError listing exactly what was searched)
karkain build examples/cross_compile/hello.kark --target x86_64-linux
karkain build examples/cross_compile/hello.kark --target aarch64-linux

# Inspect the host triple, supported matrix and per-target features
karkain target
```

Generated C is self-describing: every build emits a
`/* karkain-target: <triple> */` comment plus `KARKAIN_TARGET_ARCH_*` and
`KARKAIN_TARGET_OS_*` preprocessor defines (`KARKAIN_TARGET_WINDOWS`,
`KARKAIN_TARGET_LINUX`, ...), the Phase-111 target contract for `import "C"`
blocks.

## Expected output

- `hello` → `Hello from Karkain cross compilation`
- `functions` → `377`, `50`, `1`, `0` (one per line)
- `collections` → `31`, `cross-collections-8`, `80`
- `platform` → `platform-ok 78112`