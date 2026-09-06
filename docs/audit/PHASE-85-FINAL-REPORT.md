# PHASE 85 — Native Build & Linking Integration

## Status: COMPLETE

**Commit:** `7d71e36` (develop) → `a5ac6be` (main)  
**Date:** 2026-09-06  
**Scope:** CLI integration of Phase-84 native object/linker infrastructure

---

## Objective

Integrate the Phase-84 native object/linker infrastructure into the normal
Karkain CLI compilation workflow. A user should be able to compile a `.kark`
program through the normal CLI path and reach the native build/link stage.

---

## Implementation

### 1. NativeBuilder (`pkg/codegen/native_builder.go`)

New orchestrator for the integrated pipeline:

```go
builder := NewNativeBuilder()
result, err := builder.Build(prog, "program.kark")
```

`Build` performs:
1. `NativeGenerator.GenerateObject` — AST → Object (sections, symbols, debug info)
2. `Linker.AddObject` + `Link` — Object → Executable (symbol resolution, section layout)
3. `relocateDebugAddresses` — offsets DebugInfo addresses from 0-based counters
   to actual virtual addresses (0x1000+ .text base)
4. `SourceAddressMap.BuildFromDebugInfo` — bidirectional source↔address mapping

Also provides `BuildMultiObject` for multi-object linking.

### 2. CLI Integration (`pkg/cli/commands.go`)

- Added `"native-link"` to `validTargets` in `exitcodes.go`
- `BuildCommand` dispatches to `nativeBuildCommand` when `cfg.Target == "native-link"`
- `RunCommand` dispatches to `nativeRunCommand` when `cfg.Target == "native-link"`
- `nativeBuildCommand`: parses → NativeBuilder.Build → formats diagnostics → writes KOBJ artifact
- `nativeRunCommand`: parses → NativeBuilder.Build → reports result
- `writeObjectFile`: writes KOBJ binary format (magic, sections, symbols)
- `formatDiagnostics`: formats Phase-83 diagnostics for human output

### 3. Debug Address Relocation (`pkg/codegen/native_builder.go`)

`relocateDebugAddresses` fixes the pre-relocation address problem:
- DebugInfoBuilder generates addresses starting at 0 (counter-based)
- Linker relocates .text section to 0x1000+
- After linking, all DebugInfo addresses (FunctionInfo, LineInfo, Variables)
  are offset by the .text section base address
- SourceMap is then built from the relocated DebugInfo

### 4. SourceAddressMap Enhancement (`pkg/codegen/debug.go`)

`BuildFromDebugInfo` now also maps FunctionInfo start lines (not just LineInfo),
ensuring function entry points appear in the source↔address mapping.

---

## Files Changed

| File | Change |
|------|--------|
| `pkg/codegen/native_builder.go` | **NEW** — NativeBuilder, NativeBuildResult, relocateDebugAddresses |
| `pkg/codegen/native_builder_test.go` | **NEW** — 8 focused tests |
| `pkg/cli/native_build_test.go` | **NEW** — 6 CLI integration tests |
| `pkg/cli/commands.go` | Modified — nativeBuildCommand, nativeRunCommand, writeObjectFile, formatDiagnostics |
| `pkg/cli/exitcodes.go` | Modified — "native-link" added to validTargets |
| `pkg/codegen/debug.go` | Modified — BuildFromDebugInfo includes FunctionInfo mappings |
| `docs/native-codegen.md` | Modified — Phase 85 section added |

**Total:** 7 files changed, 863 insertions, 3 deletions

---

## Test Results

### NativeBuilder Tests (8/8 PASS)
- `TestNativeBuilder_SimpleFunction` — end-to-end build, sections, entry point, debug info, source map
- `TestNativeBuilder_WithSymbols` — symbol collection with namespacing
- `TestNativeBuilder_Diagnostics` — error diagnostic propagation
- `TestNativeBuilder_DebugInfoPreserved` — function/variable debug info through build
- `TestNativeBuilder_SourceAddressMap` — forward (source→addr) and reverse (addr→source) mapping
- `TestNativeBuilder_MultiFunction` — multiple functions in debug info and executable
- `TestNativeBuilder_HasErrors` — initial state check
- `TestNativeBuilder_GetDiagnostics` — empty diagnostics check

### CLI Integration Tests (6/6 PASS)
- `TestBuildCommand_NativeLink` — compile with `--target=native-link`
- `TestBuildCommand_NativeLinkVerbose` — verbose mode output
- `TestRunCommand_NativeLink` — run with `--target=native-link`
- `TestValidateTarget_NativeLink` — target validation
- `TestValidateTarget_Invalid` — invalid target rejection

### Full Suite
```
ok  karkain/pkg/lexer    1.634s
ok  karkain/pkg/parser   2.430s
ok  karkain/pkg/codegen  1.715s
ok  karkain/pkg/pm       16.237s
ok  karkain/pkg/cli      354.850s
ok  karkain/pkg/lsp      1.440s
```

`go vet ./...` — clean

---

## Bug Fixed

**DebugInfo address mismatch:** DebugInfoBuilder generates addresses starting at
0 (counter-based), but the linker relocates .text to 0x1000+. The SourceMap was
built from pre-relocation DebugInfo, causing `GetAddressForSource` to return
address 0 (failing the `addr == 0` test assertion). Fixed by adding
`relocateDebugAddresses` that offsets all DebugInfo addresses by the .text
section base after linking.

---

## Architecture

```
program.kark
    |
CLI (--target=native-link)
    |
Parser → Semantic Analysis
    |
NativeGenerator.GenerateObject
    |  sections: .text, .rodata, .data
    |  symbols: karkain_user_* namespaced
    |  debug: function info, line info, variables
    v
Object
    |
Linker
    |  symbol resolution
    |  section layout (base 0x1000)
    |  relocation application
    v
Executable (in-memory)
    |
relocateDebugAddresses
    |  offset DebugInfo by .text base
    v
SourceAddressMap (bidirectional)
    |
KOBJ artifact (binary)
```

---

## What Was NOT Done

- No debugger implementation (only the foundation)
- No repository-wide audit
- No redesign of Phase-83/84 subsystems
- No Phase-86 work

---

## Next Steps

Phase 86+ candidates (from ROADMAP.md):
- LSP depth (hover/go-to-definition, UTF-16 spans, VS Code Problems panel)
- Style-grade formatter
- Unparenthesized `while`/`for` parity
- Kernel-sema span accuracy
- Continued conformance-corpus growth
