# Karkain 1.0.0 — GitHub Release & Announcement Draft

> DRAFT for owner review. Do not publish verbatim without checking the
> **Owner actions** at the bottom. `FINAL_RELEASE_SOURCE_COMMIT` =
> `f23c0245c3ccfecefec8d895755f741ba89ed261` (see the certification
> report §17).

## GitHub Release `v1.0.0` body

### Karkain 1.0.0 (Stable)

Karkain is a statically typed systems programming language with a
self-hosted compiler, a byte-identical dual-engine pipeline (Go front end +
self-hosted `kcc`), a real standard library, native executables,
cross-compilation, and an experimental concurrency/WASM surface. It compiles
`.kark` source to C23 and delegates to a host C compiler (GCC/Clang/MSVC)
for final machine code.

This is the first **Stable** release. Status vocabulary and guarantees:
`docs/source/status/index.rst`, `docs/source/status/scope.rst`,
`docs/source/status/compatibility.rst`.

**In this release (honest scope)**

- Stable language core: variables, functions, recursion, control flow,
  `while (cond)` parity, structs (records), enums, `match`, arrays, strings,
  maps, modules with `public` exports.
- Standard library: `std.string`, `std.collections`, `std.io`,
  `std.encoding`, `std.crypto`, `std.testing` — importable and regression-
  gated on both engines.
- CLI: `check`, `build`, `run`, `test --filter`, `transpile`, `fmt`,
  `lint`, `debug`, `prof`, `target`, `pkg`, `workspace`, `clean`,
  `explain`, `bench`, `lsp`.
- Self-hosted `kcc` is the default engine for `check/build/run/test`;
  stage-2 == stage-3 bootstrap identity is proven bitwise identical.
- Cross-compilation with explicit triples and deterministic
  "no cross-linker" diagnostics (never a silent host fallback).
- Runtime error model with source locations and stack traces; numeric error
  codes documented by `karkain explain`.
- Developer tooling: formatter, linter, LSP, VS Code extension, DWARF debug
  sections, incremental compilation, profiling.

**Experimental surfaces (may change between releases, Go engine only):**
concurrency runtime (`spawn`/channels/actors), `wasm32-wasi` target
(wasmtime-gated), SIMD/vector types, profiling/trace.

**Not yet implemented:** networking, databases, web framework, GPU/NPU and
quantum kernel language surfaces, advanced package registry. Nothing on the
project site presents these as available.

**Binaries:** 13 archives (Windows amd64/arm64, Linux amd64/arm64/armv7/
i386/ppc64le/s390x, macOS amd64/arm64, FreeBSD amd64). Each archive contains
the `karkain` binary, the standard library, `README.md`, `LICENSE` and a
`VERSION` file. SHA-256 checksums: `docs/audit/KARKAIN-1.0.0-FINAL-RELEASE-CERTIFICATION.md` §16.

**Source:** tag `v1.0.0` = `f23c024` (release-certified commit). Build from
source: `go build -o karkain ./cmd/karkain` (requires Go 1.21+ and a C
compiler).

**Docs:** https://github.com/ajit-ai/Karkain (Sphinx site auto-published on
push to `main` via `.github/workflows/docs.yml`).

**Feedback:** open an issue (https://github.com/ajit-ai/Karkain/issues);
report security issues through private advisories (SECURITY.md).

---

## Short public announcement (social / blog intro)

> **Karkain 1.0.0 (Stable)** is out. A statically typed systems language
> with a self-hosted compiler, a byte-identical dual-engine pipeline, an
> importable standard library, native + WASM + cross-compilation targets,
> an honest status system (nothing is advertised as working unless a gate
> tests it via the real CLI), and 50 runnable examples. 13 pre-built
> binaries for Windows/Linux/macOS/FreeBSD. MIT licensed.

---

## Owner actions before publishing

1. **Create the GitHub Release**: the tag `v1.0.0` exists and is pushed;
   the Release page has not been created (a release-page `v1.0.0` returns
   404 today). Attach the 13 archives from `releases/` (SHA-256 in the
   certification report §16).
2. **Verify the docs-Pages deploy** after publishing (docs auto-publish on
   `main` push).
3. **Watch CI**: the `Run unit tests` step fails intermittently on
   `pkg/codegen` `TestPhase107_CodegenSpawnJoin` (reproduced locally in the
   combined `go test ./pkg/lexer/... ./pkg/parser/... ./pkg/codegen/...
   ./pkg/pm/...` suite; isolated runs pass). Decide to fix, skip, or
   quarantine before relying on CI green.
4. Update this draft's wording after the release exists.