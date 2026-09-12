# Karkain Developer Preview — Release Readiness Checklist

This checklist is the definition of `Phase 115 — Developer Preview
Readiness`. It is executed by `pkg/cli/phase115_developer_preview_test.go`
(the automated gate) and by this document (the human-readable record).
Every box reflects a verifiable property — nothing here is aspirational.

Public status: **🚀 Karkain Developer Preview** (v0.115.0).

## 1. Obtainable

- [x] Repository is public: https://github.com/ajit-ai/Karkain
- [x] `README.md` is the landing page: clone URL, build steps, quick start,
      docs links.
- [x] Feedback channel declared: GitHub Issues linked from README and docs.
- [x] `LICENSE` (MIT) present at repo root.

## 2. Buildable from a fresh checkout

- [x] `git clone <repo> && go build ./cmd/karkain` succeeds with Go 1.21+.
- [x] Default self-hosted engine (`kcc`) runs `check`/`build`/`run` on a
      hello program.
- [x] Go engine (`--engine go`) runs the same on the same program.
- [x] Gate: `TestPhase115_FreshCheckout` clones the repo into a temp dir and
      repeats the build+run flow (Go engine for speed; kcc leg when gcc is
      present).

## 3. Usable

- [x] `karkain --version` prints `Karkain Compiler v0.115.0 (<os>/<arch>,
      Developer Preview Build)` and exits 0.
- [x] `karkain --help` lists every implemented subcommand.
- [x] `karkain check` / `build` / `run` / `test` / `debug` / `prof` /
      `target` all respond to `--help`.
- [x] The example corpus runs end-to-end: `verify-examples.ps1` →
      45 passed / 0 failed / 5 skipped.

## 4. Honest capability labeling

- [x] README and docs use the six-level status vocabulary
      (Stable / Implemented / Experimental / Developer Preview / Planned /
      Not Yet Implemented) from `docs/source/status/index.rst`.
- [x] Every advanced domain states its real status: networking, databases,
      web, quantum, GPU/NPU kernel surface — `Planned` / `Not Yet
      Implemented`.
- [x] Experimental features state WHY they are experimental (Go-engine only,
      kcc parity deferred, wasmtime-gated).
- [x] No `v1.0.0` / "release ready" claims anywhere; Phase 91's superseded
      1.0 decision is explicitly corrected in AGENTS.md and the release notes.

## 5. Baseline validation (this phase)

- [x] `go build ./...` clean.
- [x] `go vet ./...` clean.
- [x] CLI help snapshots taken and asserted by the gate.

## 6. Fresh-checkout simulation

- [x] Temp-dir clone → build → write `hello.kark` → `karkain run` →
      `karkain check` → `karkain build` → `karkain test` flow, with
      evidence captured (gate log + this record).

## 7. Error experience audit

- [x] Unknown/typoed commands produce a clear usage message and non-zero exit.
- [x] Missing files produce a usage diagnostic (exit 2), not a crash.
- [x] Syntax errors report exit 3 with `error[K...]` diagnostics.
- [x] No panics observed while probing the CLI with bad input (see audit log
      in the Phase 115 readiness report).

## 8. Documentation updated

- [x] README (status, quick start, CLI, cross-compilation, structure).
- [x] Installation page — source build is THE way (binaries `Planned`).
- [x] `release-notes.rst` — honest version history, no 1.0 claims.
- [x] `status/index.rst`, `status/implemented.rst` — unchanged vocabulary,
      consistent counts.
- [x] `examples/index.rst` + 15 category pages (Phase 114).
- [x] `reference/example-matrix.rst` — capability × status × engine.
- [x] `development/developer-preview.rst` — current milestone text.
- [x] Docs build strict (`-W --keep-going`): 0 warnings; linkcheck clean.

## 9. Non-applicable features blocked / labeled

- [x] GPU, NPU, quantum kernel language surface labeled `Planned`.
- [x] Networking / database / web labeled `Not Yet Implemented` with
      README-only placeholders.
- [x] Package-registry subcommands labeled "registry not available" at
      runtime (no faked registry).

## 10. Release notes

- [x] `docs/source/release-notes.rst` — Developer Preview Build, version
      identity, honest gaps.
- [x] CLI version identity updated to `v0.115.0 ... Developer Preview Build`.

## 11. Contribution guide

- [x] `CONTRIBUTING.md` — fork, branch off `develop`, test commands,
      conventions, honesty rules.
- [x] README links it from "Contributing".

## 12. Code of conduct

- [x] `CODE_OF_CONDUCT.md` — Contributor Covenant 2.1.

## 13. LICENSE

- [x] `LICENSE` — MIT.

## 14. CI review

- [x] `.github/workflows/ci.yml` builds docs, runs unit tests, runs the
      Developer Preview gate, packages archives with README+LICENSE.
- [x] `.github/workflows/docs.yml` publishes strict-built docs to Pages.
- [x] CI errors on doc warnings and missing pages.

## 15. Gate test (automation of this checklist)

- [x] `pkg/cli/phase115_developer_preview_test.go`:
      version/help contract, repo-integrity files, docs pages, example-corpus
      availability, fresh-checkout build+run + kcc sanity.

## 16. Regression

- [x] Full unit set (`pkg/lexer`, `pkg/parser`, `pkg/codegen`, `pkg/pm`,
      `pkg/sema`, `pkg/cli` phase gates 109/110/114) green.
- [x] Conformance 59/59 on both engines.
- [x] Strict Sphinx build 0 warnings; linkcheck clean.

---
Last checked: v0.115.0 (Phase 115).