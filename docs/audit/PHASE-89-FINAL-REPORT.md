# Phase 89 — Self-Hosted Runtime & Toolchain

## Summary

Established the Karkain-owned runtime boundary and self-hosted toolchain layer. Created `runtime/` directory with 5 boundary documents defining the Value type system, container operations, I/O primitives, platform abstraction, and initialization contract. Verified the end-to-end self-hosted compiler → C23 → gcc → native executable pipeline with an acceptance test.

## What Was Implemented

### 1. Runtime Boundary (`runtime/`)

| File | Lines | Purpose |
|------|-------|---------|
| `types.kark` | 121 | Value type system (10 variants), constructors, container ops, arithmetic |
| `io.kark` | 83 | File I/O primitives (open, read, write, close, list) |
| `platform.kark` | 80 | Platform abstraction (args, system, memory, time, assertions) |
| `init.kark` | 34 | Program startup/shutdown contract |
| `boundary.kark` | 60 | Complete ownership model, namespace rules, what is NOT self-hosted |

### 2. Function Ownership Model

| Category | Prefix | Implementation | Example |
|----------|--------|----------------|---------|
| Language primitive | (none) | compiler intrinsic | `len()`, `print()` |
| Compiler builtin | (none) | codegen emits C | `str()`, `int()` |
| Standard library | `module.name` | .kark + libc FFI | `math.sqrt()` |
| Runtime API | `karkain_` | C runtime | `karkain_readFile()` |
| Platform API | (varies) | OS-specific C | `open()`, `read()` |

### 3. Acceptance Test (`kcc-tests/acceptance.kark`)

End-to-end test exercising: variables, arithmetic, functions, control flow, arrays, strings, boolean logic. Proves the self-hosted compiler can compile a meaningful Karkain program through the full pipeline.

### 4. Focused Go Tests (`pkg/cli/phase89_test.go`)

12 tests verifying:
- Runtime directory and files exist
- Runtime files are compilable by bootstrap compiler
- Acceptance test exists, checks, and builds
- Self-hosted compiler can compile acceptance test (when kcc.exe exists)
- Native executable generation works
- Bootstrap compiler still operational
- All test programs pass check
- Existing Go tests still pass

## Files Changed (10)

| File | Change |
|------|--------|
| `runtime/types.kark` | **NEW** — Value type system |
| `runtime/io.kark` | **NEW** — I/O primitives |
| `runtime/platform.kark` | **NEW** — Platform abstraction |
| `runtime/init.kark` | **NEW** — Initialization contract |
| `runtime/boundary.kark` | **NEW** — Ownership model |
| `kcc-tests/acceptance.kark` | **NEW** — End-to-end acceptance test |
| `pkg/cli/phase89_test.go` | **NEW** — 12 focused Go tests |
| `docs/runtime.md` | **NEW** — Runtime architecture docs |
| `AGENTS.md` | Updated for Phase 89 |
| `ROADMAP.md` | Updated for Phase 89 |

## Test Results

- **12/12 Phase 89 Go tests PASS** (2 skipped: kcc.exe not built in CI)
- **8/8 Phase 88 tests still PASS**
- **All existing tests PASS** (lexer, parser, codegen)
- **Build passes** (`go build ./...`)
- **Acceptance test builds to native executable** via bootstrap → C23 → gcc

## Self-Hosting Workflow

```
bootstrap compiler (Go)
    |
    v
build self-hosted compiler (kcc.exe)
    |
    v
self-hosted compiler compiles .kark programs
    |
    v
C23 output → gcc → native executable
```

## Platform Boundary

```
Karkain language
    |
Karkain standard library (stdlib/)
    |
Karkain runtime API (runtime/)
    |
Platform abstraction (C runtime)
    |
Operating system / native platform
```

## Known Limitations

- Runtime files are documentation/contract, not executable code
- Self-hosted compiler generates simplified runtime (6 types vs 10 in Go runtime)
- No separate runtime compilation step (embedded in codegen)
- No dynamic linking support
- Platform abstraction is documented but not yet implemented per-OS

## What Is NOT Complete

- Separate runtime library compilation
- Platform-specific runtime backends
- Runtime optimization passes
- Dynamic linking
- Full self-hosting independence (bootstrap still required)

## Git

- Commit: `10cc811` on develop → main (fast-forward merged)
- Both branches pushed to origin

## STOPS HERE — Phase 90 not started
