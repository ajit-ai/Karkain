# KARKAIN BACKWARD COMPATIBILITY AUDIT

Before recommending any change, preserve existing work. Classify impact of the recommended
direction (memory model, generics, error handling) as SAFE / COMPATIBLE / MIGRATION_REQUIRED /
BREAKING.

## Existing surface today (all `.kark`)
- Syntax: func/print/println/let/var/return/if/else/while/for/in/break/continue/struct/enum/
  match/Some/None/Ok/Err/macro/quote/unquote/comptime/actor/spawn/receive/channel/send/kernel/
  device/global_id/barrier/matrix/qreg/gate/measure/alloc/free/addr/mut/raw/move/linear/packed/
  fn/type/import/&/&mut/?/@/=>/...
- CLI: `karkain run|build|transpile|check|test|lsp|pkg ...` with `-v -h -c -g -o --target --verbose`.
- Test programs: 25 examples/*.kark + self-host sources in src/compiler.
- Compiler assumptions: lexer/parser/sema/codegen in Go; SSA default; C23 backend.

## Classification of recommended changes
| Change | Class | Rationale |
|--------|-------|-----------|
| Fix parser OOM (add no-progress guard) | SAFE | Only ADDs a break condition to a loop that currently misbehaves; no valid program changes behavior (valid programs never hit the runaway). |
| Complete type checker + symbol table | COMPATIBLE | New passes; existing valid programs still compile. May newly REJECT programs that were silently accepting undefined names → treat as COMPATIBLE with a warning, then stricter error later. |
| Complete borrow lifetimes (fix BUG-5) | MIGRATION_REQUIRED | Programs relying on moves leaking across scopes (currently silently allowed) will begin failing borrow check. Must be staged with clear diagnostics. |
| Real generics parse + monomorphization | COMPATIBLE | Adds syntax not currently accepted; existing non-generic code unaffected. Generic code written against current string-substitution may change behavior → MIGRATION_REQUIRED for existing generic kernels. |
| Fix match/Result codegen (BUG-2/BUG-7) | MIGRATION_REQUIRED | Match behavior changes from always-true to correct. Existing code relying on the bug must be updated. |
| Make `?` propagate errors (BUG-4) | COMPATIBLE | Enables previously-no-op behavior; code that used `?` expecting no-op is likely already broken. |
| Add defer/RAII (G8) | COMPATIBLE | New keyword; no existing code breaks. |
| No GC / no new memory model | SAFE | Confirms existing ownership direction; no API change. |
| Remove HTTP stub from every binary (D1) | BREAKING-if-exposed | If any public stdlib function is removed. Do only at a planned major boundary or when unused (it is unused today). |

## Backward-compat stance
The `.kark` extension, core syntax, CLI, and C23 backend must be preserved. The correctness
fixes (BUG-1..8) are MIGRATION_REQUIRED and should be grouped and announced, ideally under the
V1 freeze (single controlled migration), not piecemeal. Prefer adding diagnostics that name the
old-vs-new behavior rather than silent breaking.
