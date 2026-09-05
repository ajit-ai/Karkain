# Phase 82 — Toolchain Contract

Stable, machine-readable contracts for tooling that drives the `karkain`
compiler. Anything marked **contract** is covered by an automated test.

## Exit codes (contract)

| Code | Meaning | Set by |
|------|---------|--------|
| 0 | success | all commands |
| 1 | failure | general failures |
| 2 | usage / bad flag | CLI argument handling |
| 3 | compile error | `build`, `run`, `check` (diagnostics printed) |
| 4 | test failure | `karkain test` (≥1 failing test) |
| 5 | package error | `karkain pkg ...` |
| 6 | environment error | missing compiler/runtime prerequisites |

`karkain ide info` exposes these as `exitCodes`.

## `karkain check --format=json` (contract)

JSON mode emits **nothing on stdout except the diagnostic array**; human text
goes home on the default format only. Parse errors and name-resolution errors
are collected from the real pipeline and reported as structured objects:

```json
[
  {
    "file": "examples/probes/phase81/main.kark",
    "line": 4,
    "column": 2,
    "severity": "error",
    "code": "E-K-SYN",
    "message": "unexpected token"
  }
]
```

Schema name `karkain-diagnostics-v1` (see `karkain ide info`). A valid file
produces `[]` — no message text, exit 0. Diagnostics with errors exit 3.

### Columns (contract, v1)

- **Parse errors:** the true 1-based token-start column (`Parser.ErrorCols`).
- **Name-resolution errors:** line-anchored, column always `1` (a known v1
  limitation; consumers must treat column as informational for `E-K-RES`).

### Diagnostic codes

| Code | Class |
|------|-------|
| `E-K-SYN` | syntax / parse |
| `E-K-RES` | name resolution |
| `E-K-BRW` | borrow checker |
| `E-K-SEM` | semantic |
| `E-K-TYP` | type |
| `E-K-CG` | codegen |
| `E-K-PKG` | package manager |
| `E-K-ENV` | environment |

## `karkain fmt` (contract)

Conservative token-level canonicalization, **not** an AST pretty-printer:

- Fixes inter-token spacing, trailing whitespace, and line endings (CRLF → LF).
- Preserves line structure, leading indentation, blank lines, comments, and
  token text/order — guaranteed idempotent by construction.
- `//` inside string literals is never treated as a comment.
- `--check` reports non-canonical files via exit code without rewriting.

Semantics proof: `go test ./pkg/cli -run FormatCommand` formats corpus sources,
re-runs them, and asserts identical output.

## `karkain ide info` (contract)

The IDE-probing contract — one JSON document describing the language, project
detection, and every toolchain invocation an IDE must make. Full schema served
by `karkain ide info`; the toolchain commands are ready-for-shell argv arrays:

`check`, `checkJSON`, `build`, `run`, `test`, `compileCorpus`, `format`,
`formatCheck`, `lint`, `lsp`.

## `karkain test` (KTF-001)

- Discovers `*_test.kark` recursively; `test_`-prefixed functions are test
  cases; `--filter` selects by name; per-test stdout/stderr captured.
- Each test is compiled ahead with its file's own non-test declarations
  (self-contained corpus model) plus the module scope when inside a project.
- Exit 4 when any test fails; failure diagnostics include the failing
  assertion and source location.

## `karkain test --compile <dir>` (KTF-002)

Manifest-driven compile/pass corpus under `pkg/cli/testdata/compile/`
(`manifest.json`, `pass/`, `fail/`). Real lint-pipeline diagnostics; exit 4 on
any unexpected compile result. GCC-gated for pass cases.

## Language server

`karkain lsp` and `karkain language-server` both serve the real LSP server
(`pkg/lsp`, Content-Length framing over stdio). See
`docs/audit/PHASE-82-IDE-INTEGRATION.md`.

## Known limitations (honest)

- `E-K-RES` columns are line-anchored (column 1).
- User-declared functions that shadow C library symbols (`abs`, …) fail at the
  C compiler stage — symbol namespacing is a planned compiler change.
- `fmt` is contract-level canonicalization, not a formatting *style* engine.