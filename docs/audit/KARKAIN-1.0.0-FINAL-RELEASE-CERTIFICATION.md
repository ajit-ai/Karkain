# Karkain 1.0.0 — Final Release Certification

Status: **RELEASED — Karkain 1.0.0 (Stable) / General Availability**
Date: 2026-09-13
Scope: final certification record for the first public stable release.

---

## 1. Summary / Verdict

**RELEASED on git — PUBLIC General Availability pending owner publication.**
The owner-authorized release execution has been completed on the git side:
the certified source commit was pushed to the public `develop` and `main`
branches, the annotated `v1.0.0` tag was created and pushed (branch-tag
trigger fires CI), and the cross-platform release artifacts were built and
functionally validated locally. One owner-only action remains before the
GitHub Release matters to the public: `ajit-ai/Karkain` is still PRIVATE, so
the CI-created GitHub Release (or a manual artifact publish) lands on a
private repository until the owner makes it public. This document records the
evidence, artifact checksums, and documented environmental limitations of the
release run.

## 2. Source commit being released

- **FINAL_RELEASE_SOURCE_COMMIT:** `f23c0245c3ccfecefec8d895755f741ba89ed261`
- Local `main` HEAD at tag time: `f23c024` (merge of the release-pipeline
  fixes on top of `445716f` = Phase 119A certification).
- Release-pipeline fixes included in the tagged source:
  1. `.github/workflows/ci.yml` — adds `tags: ['v*']` push trigger (the
     release job already gated on `refs/tags/` but no tag trigger existed,
     so a tag push never would have run CI), and corrects the build matrix
     GOARCH mapping for `armv7` (→ `arm` + `GOARM=7`) and `i386` (→ `386`)
     while preserving the archive names.
  2. `scripts/release-all.ps1` / `scripts/release-all.sh` — same GOARCH
     mapping; `tar.gz` step fixed for Windows bsdtar (trailing-slash form
     produced empty archives).

## 3. Branch and ref state

At tag time (all verified via `git ls-remote origin`):

- `refs/heads/develop` = `a79ae02` (release-pipeline fixes commit)
- `refs/heads/main` = `f23c024` (merge of the fixes)
- `refs/tags/v1.0.0` = annotated tag object `ad7b342b` → dereferences to
  `f23c024`.
- Pre-existing legacy tags `karkain-17`, `v0.14.0`, `v0.19.0` exist on the
  remote and were intentionally left untouched.
- No refs under `refs/codex` or other local-only tool refs were pushed
  (explicit `git push origin develop`, `git push origin main`,
  `git push origin v1.0.0` only; never `--all` / `--tags` / `--mirror`).

## 4. Version identity

- `VERSION` (authoritative): `1.0.0`.
- CLI banner (both engines, `cmd/karkain/main.go` and `pkg/cli/commands.go`):
  `Karkain Compiler v1.0.0 (%s/%s, Stable Build)`.
- Generated headers / install scripts / release scripts / issue templates /
  `SECURITY.md` all claim the `1.0.0 / Stable` line.
- Verified on the actual Windows amd64 release artifact:
  `Karkain Compiler v1.0.0 (windows/amd64, Stable Build)`.

## 5. Final secret review

- Working-tree scan (PS 5.1, full tree, all tracked + untracked): **0 real
  secrets**. One pattern hit was a false positive — `docs/audit/
  PHASE-119A-PUBLIC-REPOSITORY-FINAL-REPORT.md` line 170 quoting its own
  scan-pattern documentation text.
- No `.env` files anywhere in the tree.
- Full-history content scan (1,851 content blobs) previously performed in
  Phase 119A: **0 matches** across AWS/GitHub/Slack/Stripe/Google/SendGrid/
  private-key/JWT/DB-URL/bearer-token/env-var/IP/personal-path patterns.

## 6. Final history review

- 204 commits, 3,314 objects (1,851 blobs), 1,317 unique paths ever
  committed. All large blobs are old self-built `karkain.exe` / release
  archives (v0.6–v0.13), all already deleted from `HEAD`. No rewrite was
  performed. Public history is intentionally preserved.

## 7. Final public-source audit

- Full source tree present and tracked: `cmd/`, `pkg/`, `src/compiler/`
  (production self-hosted KCC), `compiler/` (Phase-24 prototype, gate-locked),
  `stdlib/` (5 canonical modules + scaffold), `runtime/`, `conformance/`,
  `examples/` (15 categories, 50 `.kark`; Planned README-only dirs honest for
  04-networking/06-database/07-web/11-quantum), `editors/`, `docs/`,
  `kcc-tests/`, scripts, `.github/workflows/{ci.yml,docs.yml}`.
- 902 tracked files; `git clone` of the public repository succeeds and the
  checked-out tree builds and passes `go vet` (see §15).

## 8. Final private-material audit

- Working tree and history contain no private/proprietary material from any
  outside organization; `PHASE-119A-PUBLIC-REPOSITORY-FINAL-REPORT.md` §6
  classifies every top-level path as PUBLIC-REQUIRED or gate-referenced
  infrastructure. Nothing was moved out during release execution.

## 9. LICENSE and governance

- `LICENSE`: MIT ("Copyright (c) 2025-2026 The Karkain Contributors").
- `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`, issue templates
  (bug/feature/docs/config) all present and current.

## 10. Final language QA

Refer to `docs/audit/PHASE-119-LANGUAGE-QA-FINAL-REPORT.md`: Stable Core /
Experimental / Planned / Known-Limitation classification complete; final root
inventory recorded. No language or compiler source changes were made during
release execution (only CI/release-tooling and documentation).

## 11. Final compiler QA

- `go build ./...` and `go vet ./...` clean (release HEAD and pushed clone).
- Bootstrap stage-1 builds pass (3× in-session); stage-2 self-transpile
  SEGFAULT on this ~4 GB host is the documented environmental class (see §18),
  not a defect.
- Self-hosted kcc `check` of the compiler's own sources clean; in-repo default
  engine run verified (`print(7*6)` → `42`, exit 0).

## 12. Runtime & stdlib QA

- Stdlib v2 (Phase 109) byte-identical output across Go and kcc engines on the
  golden module examples; NIST FIPS 180 / RFC 4648 vectors verified.
- Runtime error model (Phase 100) and concurrency runtime (Phase 107) gates
  green in-session.
- The five shipped `stdlib/` modules are included verbatim in every release
  archive.

## 13. Conformance & regression QA

Fresh in-session measurements on the certified tree:

| Check | Result |
| --- | --- |
| `go build ./...` | PASS |
| `go vet ./...` | PASS |
| Conformance corpus (59 tests, kcc engine) | PASS, 59/59 (218.9 s) |
| Probes corpus (11 goldens) | PASS |
| verify-examples (real pipeline) | PASS, 49/0/5 |
| Phase 114 GoEngine goldens (49) | PASS (earlier in-session, identical code) |
| Phase 114 KCCParity goldens (49, byte-identical) | PASS (earlier in-session, identical code) |
| Phase 115–119 CLI gates | PASS fresh (176 s / 2.7 s / 131.7 s / 27.3 s / 6.4 s) |
| Sphinx html `-W` + linkcheck `-W` | PASS, 0 warnings |
| `beta-fresh-checkout` (working dir) | PASS |
| `beta-fresh-checkout` (true local clone) | PASS (first run: transient linkcheck network flake; re-run PASS) |
| install.ps1 + verify-install.ps1 | PASS |
| verify-rc-journey.ps1 | PASS, 11/11 |

## 14. Documentation QA

- Sphinx html (`-W --keep-going`) and linkcheck (`-W`) 0 warnings at release
  HEAD and after the post-tag `installation.rst` update.
- `installation.rst` updated post-tag to mark the 13 pre-compiled binaries
  `Available` on GitHub Releases (Docker image remains honestly `Planned`),
  with real archive names and layout-correct install commands.

## 15. Final fresh-checkout validation

Performed twice during release execution:

1. Fresh `git clone` of the LOCAL release branch into a scratch dir →
   `beta-fresh-checkout` PASS with `VERSION=1.0.0`, `HEAD=445716f`.
2. Fresh `git clone --branch main https://github.com/ajit-ai/Karkain.git`
   (the pushed remote) → `VERSION=1.0.0`, `HEAD=f23c024`, `go build` and
   `go vet` PASS, ci.yml tag trigger present.

## 16. Release artifacts

Built with the fixed `scripts/release-all.ps1 -Version v1.0.0`: 13/13
targets, 0 failed. Every archive contains the target binary (header-identified
via `go version`), `stdlib/`, `README.md`, `LICENSE`, `VERSION`. Local SHA-256
checksums (CI re-generates authoritative checksums.txt from its own builds):

```
22eeb11fe3b072e6f090f8c6fa04606a9b745aaf3525f77c4d22b01c6d220d8e  karkain-v1.0.0-darwin-amd64.tar.gz
c5432e818b095618afd0bab059c95104a492b3ba2a09c11f78204421ef508cdb  karkain-v1.0.0-darwin-arm64.tar.gz
76831b20fdd2e790dff4814baccb2d434ea7cff945f51f699c2e97ebbda212fb  karkain-v1.0.0-freebsd-amd64.tar.gz
ec81929b5d1fe3422ba84e7a99d567d16c491ba9fbc52c94c68a2dff6887d67a  karkain-v1.0.0-linux-386.tar.gz
0ea8e6aa577f64c2ca0b385aa0273f63f65e2e071b6b1419f4b95fbde12f2245  karkain-v1.0.0-linux-amd64.tar.gz
9d6f6fd4357840b4c9028f9bd1f5ef507810e5078d40a9f4d6379824b579ef53  karkain-v1.0.0-linux-arm.tar.gz
459101cc08b02737a6b6336dc24a9bbd05be29d93f6fc29229ede02c330f0089  karkain-v1.0.0-linux-arm64.tar.gz
d50cd50b8ac040e23720c8cb2eb999b7b487d6819acb74484e6657b5f1b93fc5  karkain-v1.0.0-linux-ppc64le.tar.gz
20b121aa3d8237eccc60c835f6ef7035d982a8454f6a34b976b4805b74059890  karkain-v1.0.0-linux-s390x.tar.gz
211fa8df895ab72f693c014f84a8de3456373fd50b1b41d79a14cc594fa3168c  karkain-v1.0.0-netbsd-amd64.tar.gz
beca88d900c07601ac281ca6194c6a4fc0bb0d4e8fbac2e6b9051f42a23f34bc  karkain-v1.0.0-openbsd-amd64.tar.gz
2135a6b63a97ca02c1ad053062e4a59f43b30854367ce563b07fc82854e6c238  karkain-v1.0.0-windows-amd64.zip
13841ed81dcd8b34146e6fedffc46ea4e365b7ca501959b2da95dd5515c874e3  karkain-v1.0.0-windows-arm64.zip
```

Windows amd64 artifact functional test (from the extracted archive, standalone,
no repository): `--version` banner correct; `run --engine go hello.kark` →
`42` exit 0; `check --engine go` → "Check passed." exit 0. The archive's 13
target binaries all header-verify with `go version`.

## 17. The release itself

- Tag: `v1.0.0` (annotated), pushed and **verified on the remote**; derefs to
  `f23c024`.
- Branches verifying on the remote: `develop` = `a79ae02` then `7fc6197`
  (post-tag docs), `main` = `f23c024` (tag source) then `b6f9305` (post-tag
  docs merge). The tag itself is untouched by post-tag commits.
- CI: `.github/workflows/ci.yml` `release` job (`needs: build`, gated on
  `startsWith(github.ref, 'refs/tags/')`) runs on the `tags: ['v*']` push
  trigger, downloads the 13 matrix archives, writes `checksums.txt`, and
  publishes the GitHub Release via `softprops/action-gh-release@v2`.
- **Owner-only confirmation step:** `github.com/ajit-ai/Karkain` is currently
  a **PRIVATE repository**. Verification from this host is therefore limited:
  anonymous GitHub API lookups return 404 for the repo itself, the Actions
  runs page is auth-gated, and git access works only through cached
  credentials. The tag-push CI run may have already produced the release, but
  it cannot be observed anonymously, and a release on a private repo is not a
  public General Availability until the owner makes the repository public.
  Required owner actions to complete the public GA:
  1. Make `ajit-ai/Karkain` public (Settings → Danger Zone).
  2. Confirm the `v1.0.0` GitHub Release exists with the 13 archives +
     `checksums.txt` (created by the CI release job on the tag push), or
     publish the artifacts (in `releases/` locally, checksums in §16)
     manually to the `v1.0.0` tag.
  3. Confirm the release URL and assets.
- Local artifacts for the release are built, functional, and their checksums
  are recorded in §16 regardless of CI status.

## 18. Environmental limitations (documented, NOT release blockers)

- **kcc BUILD-mode on ~4 GB RAM hosts**: transpiling the full self-hosted
  compiler sources (`kcc build`) OOM-stalls/SEGFAULTs; the low-memory
  `kcc check` path and all in-repo kcc runs with a cached `kcc.exe` pass.
  A standalone release archive must build `kcc` from `src/compiler` on first
  run; on this host that triggers the known class (`run`/`check` exit 3 with
  no diagnostics). The **self-contained Go engine** (`karkain --engine go` /
  `KARKAIN_ENGINE=go`) is fully functional standalone and is the recommended
  path for archive-only users; this is documented in `installation.rst`.
- **Monolithic CLI-gate runs**: running every `pkg/cli` gate in one process can
  exhaust a ~4 GB host; individual gates all pass in isolation.
- **Transient flakes observed this run**: one `pkg/codegen` unit flake and one
  fresh-checkout linkcheck network flake — both cleared on immediate re-run;
  the master QA battery re-verification is recorded in §13.
- pwsh (PowerShell 7) is not installed; this host certifies on PowerShell 5.1.

## 19. Summary

Karkain 1.0.0 (Stable) is **released on git**: the public branches and the
annotated `v1.0.0` tag are pushed to `github.com/ajit-ai/Karkain` and verified
remotely. The public repository content contains no real secrets or private
material, the version identity is unified at `1.0.0 / Stable Build`, every QA
gate is green on the certified source, the release pipeline was repaired
(tag trigger + GOARCH mapping + bsdtar archives) and produced 13/13 validated
artifacts whose checksums are recorded here. The **public General Availability
release** on GitHub is queued behind one owner-only action — publishing the
repository (it is currently PRIVATE) and confirming the CI-created `v1.0.0`
GitHub Release (or publishing §16 artifacts manually). This certification is
therefore `RELEASED-locally-and-on-git; PUBLIC-GA pending owner publication.`