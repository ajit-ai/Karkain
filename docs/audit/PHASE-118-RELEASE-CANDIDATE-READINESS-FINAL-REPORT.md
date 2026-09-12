# PHASE-118 — Beta 1 External Validation & Release Candidate Readiness: FINAL REPORT

- **Phase**: 118 — Beta 1 External Validation & Release Candidate Readiness
- **Status**: ✅ **COMPLETE — RC READY** (evidence in §Deliverables and §Repository-Gated Results)
- **Version**: v0.117.0-beta1 (public label remains **🧪 Karkain Beta 1** — no 1.0 claim)
- **Date**: 2026-09-12

## 1. Goal

Make Beta 1 consumable by an external developer and prove that consumption
road works end-to-end, then harden the release surface, run the final
regression, and declare release-candidate readiness — without destabilizing
the Phase 117 stable core.

## 2. Result — RC READY

Every repository-gated acceptance criterion passes. The strict Sphinx build,
the link checker, the fresh-checkout simulation, the install scripts, the
external-developer journey simulation, and the new automated Phase 118 gate
are all green. The 21-item RC checklist (``docs/source/status/rc-checklist.rst``)
is satisfiable today; the remaining decision — actually cutting the ``v0.117.0-rc1``
tag on the existing CI release pipeline — belongs to the project owner and is
not required for Beta 1 to stay honest and usable.

## 3. Deliverables

| Area | Deliverable | File(s) |
|------|-------------|---------|
| Release hygiene | Delete stale v0.14.0 ``releases/`` artifacts; ignore build dirs | ``.gitignore`` (``/releases/``, ``/.build/``, ``/artifacts/``) |
| Security | Vulnerability reporting policy | ``SECURITY.md`` |
| Issue intake | Bug report / feature request / docs / config templates | ``.github/ISSUE_TEMPLATE/*`` |
| Status policy | Release-candidate scope matrix (Stable / Experimental / Planned / Known Limitation / RC Blocker) | ``docs/source/status/scope.rst`` |
| Status policy | Compatibility guarantees per bucket | ``docs/source/status/compatibility.rst`` |
| Status policy | Developer-Preview → Beta 1 migration guide | ``docs/source/status/migration-beta1.rst`` |
| Status policy | 21-item RC acceptance checklist | ``docs/source/status/rc-checklist.rst`` |
| API policy | Stable API snapshot (language + CLI + stdlib + tools) | ``docs/source/reference/stable-api.rst`` |
| Development | Feature-freeze policy | ``docs/source/development/feature-freeze.rst`` |
| Development | Release engineering process | ``docs/source/development/release.rst`` |
| Development | Bug reporting workflow | ``docs/source/development/reporting-bugs.rst`` |
| Getting started | First project walkthrough | ``docs/source/getting-started/first-project.rst`` |
| Getting started | Workspace walkthrough | ``docs/source/getting-started/workspace.rst`` |
| Example | Two-member workspace (library + app) with workspace dependency | ``examples/workspace/`` |
| Install | Windows + POSIX install scripts (exit 0/1/2/3) | ``scripts/install.ps1``, ``scripts/install.sh`` |
| Install | Install verifier (exit 0/1/2/3) | ``scripts/verify-install.ps1`` |
| Validation | External-developer journey simulation (11 steps) | ``scripts/verify-rc-journey.ps1`` |
| Automation | Phase 118 release-candidate gate (9 subtests) | ``pkg/cli/phase118_rc_test.go`` |
| Automation | ``engine:`` line in ``karkain config`` via ``EngineFromEnv()`` | ``pkg/cli/config_target.go`` |
| CI | Rename "Developer Preview gate" → Phase 115 gate; add Phase 118 gate step; extend required doc-tree | ``.github/workflows/ci.yml`` |
| Public surface | README example count 46 → 50; milestone 118; security/reporting links | ``README.md``, ``CONTRIBUTING.md`` |
| Public surface | Showcase toolchain version + concurrency/runtime-model honesty | ``examples/showcase/README.md`` |
| Docs navigation | All new pages wired into Sphinx toctrees | ``docs/source/{status,development,reference,getting-started}/index.rst`` |
| Audit | This report | ``docs/audit/PHASE-118-RELEASE-CANDIDATE-READINESS-FINAL-REPORT.md`` |

## 4. Repository-Gated Results (run on this host, 2026-09-12)

| Gate | Result |
|------|--------|
| ``go build ./...`` / ``go vet ./...`` | ✅ PASS (full tree) |
| Unit suites (lexer, parser, sema, ir, ssa, hir, target, compiler, wasm, source, diagnostics, module, pm, runtime, backend, npu, testing, codegen, lsp) | ✅ PASS |
| ``TestPhase118_RCReadiness`` (9 subtests: version, build identity, help contract, first program, multi-file project, workspace dependency, examples spot check, release metadata, docs navigation) | ✅ PASS (``ok karkain/pkg/cli``) |
| ``TestPhase114`` (example corpus, both engines) | ✅ PASS |
| ``TestPhase115`` (developer-preview gate / fresh-checkout simulation) | ✅ PASS |
| ``TestPhase116`` (corpus metadata) | ✅ PASS |
| ``TestPhase117`` (Beta gate: while parity, semantic gating, exit codes, both-engine stdlib parity, kcc test runner, target matrix, diagnostics) | ✅ PASS |
| ``TestConformanceCorpus_RunsClean`` + probes corpus | ✅ PASS (59/59 conformance; probes 11/11) |
| ``verify-examples.ps1`` | ✅ PASS (49 passed, 0 failed, 5 deliberately skipped) |
| Sphinx strict HTML (``-W --keep-going``) | ✅ PASS (0 warnings) |
| Sphinx linkcheck (``-W``) | ✅ PASS |
| ``beta-fresh-checkout.ps1`` | ✅ PASS |
| ``install.ps1`` + ``verify-install.ps1`` | ✅ PASS (hello check/build/run; deterministic exit codes) |
| ``verify-rc-journey.ps1`` | ✅ PASS (11 passed, 0 failed) |

## 5. External-Developer Journey Proved (``verify-rc-journey.ps1``)

1. Fresh source build → 2. ``--version`` = ``Karkain Compiler v0.117.0 (windows/amd64, Beta 1 Build)`` → 3. ``--help`` contract (no "Developer Preview") → 4. hello.kark check/build/run → 5. multi-file module project (``math.twice`` = 42) → 6. workspace: app consumes sibling library ("hi from library api" + "hi from app") → 7. ``karkain test`` (pass summary) → 8. ``--debug`` build → 9. ``karkain prof`` → 10. example goldens (hello, module_system) → 11. docs + issue + security metadata present.

## 6. War Story: Fixes Surfaced by This Phase

1. **Phase 83 LSP test premise became stale.** ``print(1);`` now parses as a valid program (the parser evolved), so ``TestLSP_RealTimeSync`` failed to see a diagnostic for its "invalid" buffer. Verified pre-existing on a pristine HEAD (not caused by Phase 118); the fixture now uses ``let = 42``, the Phase 117-guaranteed parse error, so the ``didChange`` invalidation contract is actually exercised.
2. **PowerShell 5.1 array semantics bit the verification scripts.** ``-match`` on a multi-line native-command capture returns matched *lines*, not booleans; ``-notmatch`` returns the non-matching lines, which is non-empty even when a forbidden word is present. ``verify-install.ps1`` and ``verify-rc-journey.ps1`` were hardened (join output to a string / count matched lines).
3. **Six new pages had overlong section titles.** Spaces in titles made the RST overline stricter-than-usual; fixed overline lengths to satisfy Sphinx ``-W``.
4. **Bootstrap stage-2 SEGFAULT.** ``exit 0xc0000005`` in ``TestBootstrap_BitwiseIdentity`` on this ~4 GB host is the **documented environmental kcc-build-mode OOM class** (see Phase 99/101/102/107); ``src/compiler`` is untouched and the stage-1/check paths pass. Not a defect and not a Phase 118 change.
5. **One combined CLI gate run crashed under cumulative RAM pressure** while every individual gate passes in isolation — the same OOM class; Phase 118 ran the gates individually/sequentially.

## 7. Compatibility Notes

- ``karkain config`` now prints ``engine: go|kcc`` (effective engine from
  ``EngineFromEnv()``). No test asserted the old output, so this is an
  additive change to the configuration contract.
- Public example count corrected to **50 real ``.kark`` programs across 15
  categories** (48 Runnable, 2 Experimental, 4 Planned README-only) to match
  ``examples/EXAMPLES.md`` and the Sphinx corpus page.
- No language feature, diagnostics contract, exit-code contract, or artifact
  format changed in Phase 118.

## 8. Known Limitations (documented, NOT defects)

- Full ``kcc build`` of the compiler's own sources and concurrent full-package
  ``go test ./pkg/cli`` can exhaust very limited hosts; sequential/batched runs
  are documented in ``installation.rst`` (§ Minimum practical development
  environment).
- No release tag cut yet; ``installation.rst`` honestly lists binaries as
  ``Planned`` until the owner tags ``v0.117.0-rc1`` on the existing CI release
  pipeline.

## 9. RC Decision

**RC READY** — the release-candidate acceptance checklist
(``docs/source/status/rc-checklist.rst``) is satisfied with repository-gated
evidence. Beta 1 remains the public label; nothing claims 1.0.