# Phase 111: Cross-Compilation — Final Report

```
Phase 111: COMPLETE

Deliverable:
`--target <triple>` is a real, explicit cross-compilation switch backed by a
Karkain-owned target model. A `.kark` program builds for a requested
architecture/OS with a target-aware C-driver selection, deterministic
cross-linker diagnostics when the required toolchain is missing (never a
silent host fallback), a cross-run restriction for foreign machines, binary
self-description in the generated C, and honest per-target status reporting.
```

## Implemented

- `pkg/target/triple.go` (new)
  - `Arch` (unknown, x86_64, aarch64, wasm32), `OS` (unknown, windows, linux,
    wasi), `Env` (unknown, gnu, msvc, musl)
  - `Target`: `Parse` (2–4 components, vendor disambiguation), canonical
    `String()` (short form), `Equal`, `SameMachine` (arch + OS), `IsHost`,
    `Host()` (from `runtime.GOOS/GOARCH`, driving the Unix→
    `unknown-linux-gnu` default), `SupportedArchs/OSes`
  - `ParseError` kinds: malformed, unknown-arch, unknown-os, unknown-env,
    unsupported-abi — all reach `error[K1xx]`-style usage diagnostics
  - Env defaulting: windows/linux default to `gnu`; `msvc` only pairs with
    windows; `musl` parses but is rejected as unsupported-ABI; `wasm32` only
    with `wasi` and vice-versa
  - Canonical form smokes: `x86_64-pc-windows-msvc` → `x86_64-windows`,
    `x86_64-unknown-linux-gnu` → `x86_64-linux`, `aarch64-unknown-linux-gnu`
    → `aarch64-linux`
- `pkg/target/features.go` (new)
  - `Features` (pointer width, endianness, object/executable format, ABI,
    calling convention, runtime variant, linker requirement), `FeaturesOf`
  - `SupportedTargets()` = {x86_64-windows, x86_64-linux, aarch64-linux,
    wasm32-wasi}; `IsSupported`; `MingwTriple` (mingw cross-gcc prefix);
    `ToolchainError` (Target, Host, Searched list, complete message);
    `Describe`
- `pkg/cli/exitcodes.go`
  - `ValidateTarget`: legacy alias map (`native`/`c23`/`native-link`/
    `wasm32-wasi`) OR a parsed triple — nothing else is accepted
  - `NormalizeTarget`: aliases pass through, triples canonicalize (used by
    `karkain` main dispatch)
  - `SelectedTarget`: aliases → `Host()`, triples → parsed target
    (`wasm32-wasi` remains a well-formed parse but its dedicated backend runs
    first)
  - `TargetError{Target, Reason}` renders full parse reasons
  - `classifyCompileError` maps `ToolchainError` → `ExitEnv` (6)
- `pkg/cli/config_target.go` — `karkain target` now prints the host triple,
  the alias set (unchanged legacy lines) and the full supported matrix with
  per-triple `Features.Describe` rows plus `Default: native`
- `pkg/codegen/cross_target.go` (new) — `selectedTarget`, `isHostMachine`,
  `detectCompilerForTarget`:
  - Same-machine targets: historical probing kept byte-for-byte
    (`detectHostCompiler` + `hostNativeFlags`; CC env → gcc → clang → MSVC
    cl)
  - Windows cross (foreign host): `x86_64|aarch64-w64-mingw32-gcc` →
    `clang --target=<mingw triple>`
  - Linux cross: `x86_64|aarch64-linux-gnu-gcc`,
    `...-pc-linux-gnu-gcc`, `...-unknown-linux-gnu-gcc` →
    `clang --target=<arch>-unknown-linux-gnu`
  - Missing toolchain → `crossToolchainError` (a `ToolchainError` listing
    exactly what was searched); CC override still honored first
  - WASI direct-codegen path preserved (`clang --target=wasm32-wasi`); the
    CLI routes wasm builds to `pkg/wasm` before codegen as before
- `pkg/codegen/codegen.go`
  - `GenerateAndCompile` resolves the target compiler via
    `detectCompilerForTarget`; `ToolchainError` propagates unwrapped;
    other failures wrap with `orNative`/`targetHint`
  - `targetHeaderPrefix()` + `generateCHeader`: every emitted C starts with
    `/* karkain-target: <triple> */` plus
    `KARKAIN_TARGET_ARCH_<ARCH> 1` / `KARKAIN_TARGET_OS_<OS> 1` /
    `KARKAIN_TARGET_X86_64|AARCH64|WASM32 1` /
    `KARKAIN_TARGET_WINDOWS|LINUX|WASI 1` — the Phase-111 target contract
  - `detectCompiler` retained as a host-only thin wrapper (callers unchanged)
- `cmd/karkain/main.go`
  - Both `--target=value` and `--target value` canonicalize via
    `cli.NormalizeTarget` (error → exit 2 with the full reason)
  - kcc `run` branch routes non-(native|c23|empty) targets to the Go
    `cli.RunCommand` so explicit triples never run through kcc
- `pkg/cli/commands.go`
  - `RunCommand`: after native-link and wasm32-wasi dispatch, a triple whose
    machine differs from the host is refused:
    `cannot run a <triple> binary on the host (<host>): cross-run requires an
    emulator or a remote target; use \`karkain build --target <triple> -o
    <path>\` to build only` (exit 6)
  - `BuildCommand`: for a concrete triple, `CompileOnly` is disabled so a
    plain `karkain build --target <triple>` emits a REAL executable (not just
    the historical C transpile); legacy aliases keep the transpile-only path
  - `BuildCommandIncremental` already linked real executables; the
    content-addressed cache is target-keyed (`CompilerKey` includes
    `cfg.Target`), so cross-target caches can never collide
- Examples — `examples/cross_compile/` (`hello`, `functions`, `collections`,
  `platform` + README), all canonical-syntax, deterministic, target-agnostic

## Target matrix (honest, per the Definition of Complete)

| Target | This Windows x86_64 host | Evidence |
|--------|--------------------------|----------|
| `x86_64-windows` | **PASS** | `karkain build --target x86_64-windows` produces a runnable PE32+ x86-64 executable; `file` reports `PE32+ executable ... x86-64`; machine field = `0x8664` parsed from MZ/PE headers; golden stdout on run |
| `x86_64-pc-windows-msvc` | PASS (normalized) | identical artifact path; canonicalizes to `x86_64-windows` |
| `x86_64-linux` | **N/A** on this host | `ToolchainError` (exit 6) names the 3 searched Linux cross-gcc names + host; NO artifact created; mechanism implemented and deterministic — an ELF artifact is unverifiable without a cross-linker on this machine |
| `aarch64-linux` | **N/A** on this host | identical behavior with aarch64 names |
| `wasm32-wasi` | unchanged | Phase 108 backend (not rebuilt by this phase) |

Rule: the matrix follows what the host toolchain can REALISTICALLY support;
no target is advertised as verified that was not. The N/A rows carry the
mechanism (driver selection, diagnostics) and are unit-tested through their
deterministic negative behavior.

## Determinism

- Two identical builds in different temp dirs produce byte-identical generated
  C (sha256 equal) and byte-identical stdout at runtime.
- The mingw-linked executables carry a PE timestamp, so raw exe bytes differ
  between builds; the determinism gate therefore compares generated C and
  runtime behavior, not the raw exe image.

## Diagnostics samples

```
$ karkain build hello.kark --target x86_64-linux
Build Error: no cross-linker available for target 'x86_64-linux' on host
'x86_64-windows' (searched: 'x86_64-linux-gnu-gcc',
'x86_64-pc-linux-gnu-gcc', 'x86_64-unknown-linux-gnu-gcc'); install a C
cross-toolchain for x86_64-linux (see `karkain target`) and ensure it is on
PATH — the host toolchain can neither link nor execute linux output
[exit 6]

$ karkain run hello.kark --target x86_64-linux
cannot run a x86_64-linux binary on the host (x86_64-windows): cross-run
requires an emulator or a remote target; use `karkain build --target
x86_64-linux -o <path>` to build only
[exit 6]

$ karkain build hello.kark --target s390x-linux
unsupported target 's390x-linux': unsupported architecture 's390x' in target
's390x-linux' (supported architectures: x86_64, aarch64, wasm32)
[exit 2]
```

## Gates

- `pkg/target/triple_test.go` — parse matrix, canonical forms, env defaults,
  SameMachine/Host, ParseError reasons, musl/MSVC pairing rejections
- `pkg/cli/phase111_cross_compile_test.go`
  - `karkain target` shows host triple, all supported triples, features,
    legacy aliases, `Default: native`
  - canonical normalization table; `SelectedTarget` machine math;
    `ValidateTarget` rejection table
  - host-executable cross build + PE machine-field parse (0x8664) + golden
    stdout; default-engine (kcc) routing to the Go front end
  - normalized `x86_64-pc-windows-msvc` build + run; cross-run host (ok) and
    foreign (refused, exit 6); missing cross-linker negative (exit 6, exact
    names, no artifact); deterministic C; target markers in C; example corpus
    goldens; legacy aliases; incremental cross-target cache
- Full-file runs above green in isolation and in the combined Phase-111 run.

## Regressions

- `go test ./pkg/codegen/...` — 42.1s ok (includes Phase 110 profiling +
  Phase 106 SIMD + Phase 107 concurrency gates)
- `go test ./pkg/cli/ -run` Phase-105/108/110/111 + target/config/exitcode
  gates — 142.2s ok
- `go test ./pkg/target/...` — ok
- `go vet ./pkg/target/... ./pkg/cli/...` — clean
- `go build ./...` — clean

## Boundaries

- In-language `target.os`/`target.arch` builtins are intentionally NOT part of
  Phase 111 (they would require kcc-parity semantic changes; they are mapped
  to post-111 work). Target-specific behavior lives in generated C through
  the `KARKAIN_TARGET_*` contract usable by `import "C"` blocks.
- kcc engines: triple builds are routed to the Go front end; kcc codegen
  parity for triples is a documented post-111 boundary too.
- Linux/ARM artifacts are not verifiable on this Windows host (no cross
  linker); the cross-driver mechanism is deliberate and unit-tested via its
  failure diagnostics rather than claiming unverified support.
- `karkain run --target wasm32-wasi` and `--target native-link` behaviors are
  unchanged (their dispatch precedes the new cross-run check).

## Files

- `pkg/target/triple.go`, `pkg/target/features.go`, `pkg/target/triple_test.go`
- `pkg/codegen/cross_target.go`, `pkg/codegen/codegen.go`
- `pkg/cli/exitcodes.go`, `pkg/cli/config_target.go`, `pkg/cli/commands.go`,
  `pkg/cli/phase111_cross_compile_test.go`
- `cmd/karkain/main.go`
- `examples/cross_compile/{hello,functions,collections,platform}.kark`,
  `examples/cross_compile/README.md`
- `.gitignore` (cross_compile C artifacts), `README.md`, `ROADMAP.md`,
  `docs/ROADMAP-PRODUCTION.md`, `docs/audit/KARKAIN_FEATURE_PRIORITY_MATRIX.md`
```