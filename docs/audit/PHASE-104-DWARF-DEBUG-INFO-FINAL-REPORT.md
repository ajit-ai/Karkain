# Phase 104 — Debug Information (DWARF) — Final Report

## Objective

Give native executables real, inspectable debug information: DWARF 4 debug
sections (`.debug_info`, `.debug_abbrev`, `.debug_str`, `.debug_line`) carrying
function names, declaration file/line, variable names, and a line-number
table, so a debugger-equivalent reader can map addresses back to source.

Per `docs/ROADMAP-PRODUCTION.md` Tier 3, this is the first Tier-3 phase
(104–109) and rests directly on the Phase 84/85 object + linker +
`DebugInfo` model.

## Delivered

- **Karkain-owned DWARF 4 emitter** — `pkg/codegen/dwarf.go`
  (`DwarfEmitter`): builds the four debug sections from the native
  `DebugInfo` model (FunctionInfo/VariableInfo/LineInfo/SourceFiles).
  - `.debug_info`: one `DW_TAG_compile_unit` whose children are a
    `DW_TAG_subprogram` per function (nested local-variable `DW_TAG_variable`
    DIEs), followed by `DW_TAG_base_type` DIEs referenced from `DW_AT_type`.
    `DW_AT_high_pc` is an offset-with-size form; addresses are the relocated
    `.text` addresses.
  - `.debug_abbrev`: four abbreviation codes (compile_unit/subprogram/
    base_type/variable) with `DW_FORM_strp/data1/data2/data4/addr/flag_present/
    ref4/sec_offset` attribute forms.
  - `.debug_str`: sorted, deduplicated null-terminated string pool.
  - `.debug_line` (v4): a correct line-program state machine using
    `DW_LNS_const_add_pc` (advance `(255-13)/14 = 17`), `DW_LNS_advance_pc`,
    `DW_LNS_advance_line`, `DW_LNS_set_file`, `DW_LNS_copy`, and
    `DW_LNE_end_sequence` row resets. The DWARF-4 file table uses
    null-terminated path strings (not the length-prefixed v5 form).
- **DWARF-4 reader** — `pkg/codegen/dwarf_parse.go`: decodes the abbrev table,
  the DIE tree, and the full line state machine back into
  `ParsedDWARF` (CUs, subprograms, variables, line rows).
- **`DwarfTextDump`** — a readelf-`--debug-dump`-style textual view of the
  debug sections for inspection without an external tool.
- **Pipeline integration** — `linker.go` (`SectionTypeDebug` added to the
  layout order), `native_builder.go` (`emitAndAttachDWARF` in both `Build`
  and `BuildMultiObject` after `relocateDebugAddresses`), and new
  `Executable` helpers `GetSection`, `GetDebugSections`, `HasDebugSections`.

### Bugs fixed during development

- The abbreviations' `has_children` byte was being emitted into each DIE
  (it belongs only in the abbrev table), corrupting every DIE offset/
  attribute stream; `buildInfo` now records real section offsets in a single
  append pass.
- The `DW_AT_decl_file` attribute emitted the internal 0-based `FileIndex`;
  DWARF file numbers are 1-based — now emitted as `FileIndex + 1` (the line
  program already used `file + 1`).

## Acceptance

Gate: `pkg/codegen/dwarf_test.go` (8 tests) + `pkg/cli/phase104_dwarf_test.go`
(3 E2E tests through the real `parseSource` → `NativeBuilder.Build` pipeline).

- Native executables carry exactly the four DWARF sections (Type
  `SectionTypeDebug`), all non-empty.
- Round-trip: `main`/`helper` subprograms with correct relocated `low_pc`,
  non-zero size, 1-based decl file/line; local variables bound to their owning
  function; a line-row table with function-start rows and per-function
  end-sequence reset markers.
- `DwarfTextDump` renders compile-unit, subprogram, var, producer, low_pc and
  `.debug_line` content.

Regression suite (low-memory pattern on the ~4GB host):
`go vet` clean; `pkg/codegen`, `pkg/ir/{,hir,ssa}`, `pkg/sema`, `pkg/lexer`,
`pkg/parser`, `pkg/pm`, `pkg/source`, `pkg/module`, `pkg/diagnostics`,
`pkg/backend/*`, `pkg/npu` all green; CLI gate
`TestConformanceCorpus_RunsClean` (59/59 conformance), `TestProbesCorpus*`,
`TestPhase102_FoundationGolden_Go/KCC`, `TestPhase102_CompilerSourcesSelfCheck`
(37.19s) all PASS. The documented environmental bootstrap OOM
(`TestBootstrap_Stage2SelfHosting` / `TestBootstrap_BitwiseIdentity` kcc
stage-2 segfault on the 4GB host, a known Phase 99+ class, unrelated to this
Go-side native-codegen change) persists in low-RAM concurrent runs.

## Known limitation (documented, NOT a defect)

The native pipeline produces an in-memory `Executable`; it does not yet write
an ELF/PE container. Real on-disk executables with DWARF come from the gcc
C-transpile path (already `-g` + `#line`). This phase proves the DWARF
sections end-to-end via the self-written parser + `DwarfTextDump` (a
readelf-equivalent view) rather than `readelf`/`llvm-dwarfdump`, which are not
present on the host. A container writer is a future phase; the DWARF sections
produced here are ready to be embedded the moment one exists.

## Verification

- 8 codegen unit tests (section presence, subprogram/var/line round-trips,
  text dump, nil handling, parser errors, builder attachment).
- 3 CLI E2E tests over `factorial`/`main` (DWARF carried, full round-trip,
  text-dump content).
- Compiler engines untouched: all changes are Go-side native codegen, so the
  self-hosted kcc pipeline and both-engine parity are unaffected.