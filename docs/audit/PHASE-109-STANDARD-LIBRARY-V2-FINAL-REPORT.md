# Phase 109: Standard Library v2 — Final Report

```
Phase 109: COMPLETE

Implemented:
- Collections (stdlib/collections/collections.kark)
- Strings    (stdlib/string/string.kark)
- I/O        (stdlib/io/io.kark)
- Encoding   (stdlib/encoding/encoding.kark)
- Crypto     (stdlib/crypto/crypto.kark)

Real .kark examples:
- examples/stdlib/strings/main.kark        (string API: length/repeat/index/
                                             sub/slice/trim/case/join/replace)
- examples/stdlib/strings/utf8.kark        (UTF-8 byte model: byte length,
                                             hex/base64 byte round-trips,
                                             multibyte text untouched by ASCII
                                             case mapping)
- examples/stdlib/collections/main.kark    (contains/index/reverse/unique/
                                             flatten/sum/min/max/sort, nested
                                             arrays, map keys)
- examples/stdlib/io/main.kark             (create → write → read → verify →
                                             cleanup; append/exists/list;
                                             temp-dir usage; no residue)
- examples/stdlib/encoding_crypto/main.kark(NIST/RFC vectors: hex, base64,
                                             UTF-8, sha256 abc/"" sha512 abc)
- examples/stdlib_v2/main.kark             (multi-library E2E: strings +
                                             collections + encoding + crypto →
                                             sha256("karkain") and UTF-8 byte
                                             round-trip, deterministic)
- NEGATIVE: examples/stdlib_errors/bad_hex/main.kark (malformed hex)
- NEGATIVE: examples/stdlib_errors/bad_base64/main.kark (malformed base64)
- NEGATIVE: examples/stdlib_errors/missing_module/main.kark (import std.nope)

Tests:
- pkg/cli/phase109_stdlib_test.go
  * stdlib modules: every single-module example builds AND runs on both the
    Go and self-hosted kcc engines with byte-identical stdout, pinned against
    golden output (hex/base64 RFC 4648, sha256/sha512 NIST vectors)
  * strings/utf8: UTF-8 byte round-trips byte-identical on both engines
  * stdlib_v2 multi-module E2E: byte-identical across engines, pinned golden,
    determinism (re-run same exe + fresh rebuild reproduce identical bytes)
  * module-assembly guard: resolveSourcesRun pulls the stdlib module sources
    into the compiled unit
  * negative parity: malformed hex/base64 → ExitFailure on both engines with
    the SAME runtime-error kind and the SAME file:line
  * missing module: import std.does_not_exist rejected on both engines
- pkg/sema/phase109_builtins_test.go
  * the 8 byte-level builtins resolve clean as builtins (literal and variable
    operands), never flagged as undefined identifiers
- Coverage built into the examples/tests above: empty input (sha256(""),
  utf8_valid(""), base64/hex of ""), ASCII + multibyte text, byte slicing
  boundaries, search failure (-1), split/join, round-trips, repeated
  deterministic hashing, malformed encodings, file not found / missing
  module, append/exists/delete sequences. UTF-8 is never treated as ASCII.

Native:
PASS  (all examples, both engines, through the normal pipeline)

WASM/WASI:
N/A — Phase 109's new builtins are K108-gated (native-only). The WASM backend
is unchanged and its existing gate suite is green; the boundary is documented
in pkg/wasm/backend.go (scanUnsupported K108 gate) and docs/stdlib.md §9.
Nothing was "faked" for WASM.

Phase 106 regression:
PASS  (pkg/cli TestPhase106 + full pkg/cli phase gates green)

Phase 107 regression:
PASS  (pkg/cli phase107 concurrency gates + pkg/codegen + pkg/runtime green)

Phase 108 regression:
PASS  (full pkg/wasm suite green; phase108 CLI E2E green)

go build:
PASS  (go build ./...)

go vet:
PASS  (go vet ./...)

Documentation:
- docs/audit/PHASE-109-STANDARD-LIBRARY-V2-FINAL-REPORT.md (this report)
- docs/stdlib.md (v2 modules, builtin table, module assembly, boundary)
- ROADMAP.md (Phase 109 COMPLETE entry + roadmap table row)
- AGENTS.md (post-109 summary incl. implementation notes)
- SPEC.md (§6.2b importable std.* surface; §15.1 stdlib exemption note)

Commit:
965d7da — "Phase 109: Standard Library v2 (importable std.string/collections/io/encoding/crypto, both engines, byte-identical)"

develop:
PUSHED (develop == working tree)

main:
PUSHED (merged from develop; develop and main synchronized)

Working tree:
CLEAN

Remaining blockers:
None.
```

---

## Summary

Phase 109 delivers a real, importable standard library for Karkain. Five
canonical-syntax modules (`stdlib/string`, `stdlib/collections`, `stdlib/io`,
`stdlib/encoding`, `stdlib/crypto`) can be `import std.x`-ed from actual
`.kark` programs and run through the normal `karkain check/build/run` pipeline
on **both** engines — the Go front end and the self-hosted kcc engine —
byte-identical, verified by automated gates.

## Implementation

### Eight byte-level runtime builtins (both engines)

The encoding/crypto/collections surfaces are backed by eight builtins, each a
single C helper present in both engines' generated output:

| Builtin | Purpose |
|---|---|
| `hex_encode_bytes` / `hex_decode_bytes` | Lowercase hex + strict decode |
| `base64_encode_bytes` / `base64_decode_bytes` | RFC 4648 §4 with padding + strict decode |
| `utf8_valid_bytes` | Valid-UTF-8 check |
| `sha256_hex` / `sha512_hex` | NIST SHA-256/SHA-512, hex digest |
| `map_keys_of` | Map keys → string array |

Go engine: builtin names in `pkg/sema/resolve.go`, C preamble/helpers +
`genExpr` dispatch in `pkg/codegen/codegen.go`. kcc engine: checker/sema
tables in `src/compiler/checker.kark` / `src/compiler/sema.kark` and C-helper
emission in `src/compiler/codegen.kark`. Both engines raise the same Phase 100
runtime-error diagnostics (`runtime error: invalid hex string at
<file>:<line>`, same line numbers) and reject a missing module identically.

### Module/import integration

The existing module loader now serves stdlib imports for single- and
multi-module programs (`resolveSourcesRun`), and `kccAssembleSource`
(`pkg/cli/kcc_engine.go`) performs the same module-aware assembly then strips
dotted `import std.x` lines before its parser (which only understands bare
`import <name>`); `import "C" { ... }` blocks are preserved. `example
stdlib_v2` proves strings + collections + encoding + crypto imported together.

### Notable fixes made during the phase

- Stdlib modules moved to Go-parser-canonical syntax (`while(...)` parens —
  unlike `if`, Karkain's Go parser requires parentheses on `while`).
- UTF-8-no-BOM writes for stdlib/.kark sources (PowerShell 5.1
  `Set-Content -Encoding UTF8` BOMs break mid-assembly module parsing).
- `readLineEOF` / `listFiles` arity-1 entries added to kcc sema tables.
- IO examples `io_close` before `io_delete_file` — Windows shares the runtime
  file handle and would otherwise keep the file locked.
- String-array direct prints differ cross-engine (`["x"]` vs `[x]`, pre-existing)
  — examples/tests iterate instead.

## Boundaries

- **WASM:** the 8 builtins are K108-gated (native-only); WASM backend and its
  gates are untouched. Documented in `docs/stdlib.md` and `pkg/wasm`.
- **`stdlib/core`, `stdlib/math`, `stdlib/system`, `stdlib/gpu`,
  `stdlib/async`:** remain behind this phase. Their sources predate Phase 109
  (non-canonical syntax, no builtin backing) and are not importable; the
  boundary is documented rather than half-repaired.
- **Crypto:** SHA-256/SHA-512 only, deterministic and vector-verified. No
  randomness or encryption surface, and nothing non-cryptographic is labeled
  secure.