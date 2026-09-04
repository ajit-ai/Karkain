# KARKAIN - Phase P2 (Package Ecosystem Maturity) - GATE 3 Engineering Report

Phase: P2 - Package Ecosystem Maturity (gated methodology)
Gate: GATE 3 - Registry Source Integration (P2.5, P2.6)
Date: 2026-09-04
Repo: `F:\Codes\Git\Karkain`
Base: GATE 2 committed/merged/pushed (`08891c4`)

---

## 1. GATE 3 SCOPE

GATE 3 covers:
- P2.5 - publishable registry client + protocol definition,
- P2.6 - local test server + "PUBLIC REGISTRY DEPLOYMENT REMAINS FUTURE WORK" docs.

The public/production registry does NOT exist yet. This gate ships the client
protocol, a hermetic in-process server for tests, and the deployment contract.

---

## 2. KARKAIN REGISTRY PROTOCOL v1 (Karkain-owned, not ONNX/npm)

Base URL configurable via `KARKAIN_REGISTRY` env or `DefaultRegistryURL`. All
routes are under `/v1`.

| Method | Route | Purpose |
|--------|-------|---------|
| GET | `/v1/packages?q={query}` | Search/list packages; returns `SearchResult[]` JSON |
| GET | `/v1/packages/{name}` | Metadata; returns `PackageMetadata` JSON |
| GET | `/v1/packages/{name}/{version}/download` | Gzipped-tar of package + `X-Karkain-Sha256` integrity header |
| POST | `/v1/packages/{name}/publish` | Publish via gzipped tar (`Karkain-Version`, `X-Karkain-Sha256`, `Authorization: Bearer`) |

Integrity is carried by the `X-Karkain-Sha256` response/request header; the
client verifies the downloaded archive against it (Rule: unvalidated archive
extraction forbidden).

---

## 3. WHAT WAS DONE

### P2.5 - Registry client (`pkg/pm/registry.go`, rewritten in place - additive/protocol-complete)

Implemented the previously-stubbed functions:

- `SearchRegistry(query)` - decodes `SearchResult[]`; 404 -> empty; network/HTTP
  failures -> `E-PKG-REGISTRY`.
- `PackageInfo(name)` - fetches `PackageMetadata`; 404 -> `E-PKG-PACKAGE-NOT-FOUND`.
- `LatestVersion(name)` - convenience wrapper.
- `(*RegistryClient).ResolveVersion(name, constraint)` - NEW: matches a version
  constraint (or "" = latest) against published versions:
  - exact version fast-path,
  - semantic constraint via existing `ParseVersions` + `LatestSatisfying`.
  Missing/unsatisfiable -> `E-PKG-VERSION-NOT-FOUND`.
- `FetchFromRegistry(name, version, destDir)` - NEW safe materialization:
  - streams archive to a temp file while hashing (never full-in-memory),
  - verifies `X-Karkain-Sha256` before any extraction,
  - `extractTarballSafe` - rejects path traversal (`withinDir`) and
    symlink/hardlink entries; only expands dirs/regular files.
- `PublishPackage` + `buildPackageTarball` - builds a deterministic gzipped tar
  of project source (excludes `.karkain`, `.git`, `.DS_Store`), computes sha256,
  POSTs with integrity header.
- All errors use `PkgError` with `E-PKG-*` codes (see source.go taxonomy; added
  `Cause`+`Unwrap` to `PkgError`).

### P2.6 - Local in-process test server (`pkg/pm/regserver/` - new package)

`regserver.Server` is an in-memory `net/http/httptest` registry implementing the
exact v1 protocol, enabling hermetic end-to-end tests with no network:

- `Add(name, versions->sourceDir, PublishMeta)` populates packages; tarballs are
  built from source dirs with deterministic relative/forward-slash names.
- Serves search, metadata, download (with `X-Karkain-Sha256`), and publish.

Tests override the registry via `KARKAIN_REGISTRY=<server URL>`.

### Deployment documentation
- `docs/audit/P2-GATE3-REGISTRY-REPORT.md` (this file) records "PUBLIC REGISTRY
  DEPLOYMENT REMAINS FUTURE WORK"; `regserver` is explicitly a test harness, not
  a production host. A production deployment would need persistence, auth
  scopes, rate limiting, and CDN - all out of scope and documented as such.

---

## 4. GATE 3 VERIFICATION (all checks green)

| Check | Result |
|-------|--------|
| `go build ./...` | PASS (exit 0) |
| `go vet ./pkg/pm/...` (incl. regserver) | PASS |
| gofmt (new/edited, CR-normalized) | PASS |
| `go test ./pkg/pm/... -count=1` | PASS (incl. 8 new registry tests) |
| `go test ./pkg/cli/... -count=1` | PASS (regression) |

New tests (`pkg/pm/registry_test.go`, 8) all run against `regserver`:

- `TestRegistrySearch` - query + full list
- `TestRegistryPackageInfo` + `_NotFound` - metadata; `E-PKG-PACKAGE-NOT-FOUND`
- `TestRegistryResolveVersion_Constraint` - caret + latest resolution
- `TestRegistryFetchFromRegistry_SafeExtract` - tarball materialization
- `TestRegistryFetch_IntegrityMismatch` - integrity-aware download path
- `TestExtractTarballSafe_PathTraversalRejected` - `../evil` rejected
- `TestRegistryFetchModule_Path` - E2E `FetchModule` registry source -> cache

Plus `FetchModule` registry case in `manager.go` now resolves + downloads.

Formatting: repo-wide CRLF at HEAD is pre-existing (GATE 1); new files
gofmt-clean under CR-normalized inspection.

---

## 5. GATE 3 VERDICT

> ### PASS

GATE 3 (Registry Source Integration) meets the master-prompt acceptance
criteria:
- publishable client + explicit Karkain-owned protocol,
- hermetic local test server for reproducible registry testing,
- safe archive extraction (traversal + symlink rejection) and integrity
  verification before extraction,
- `E-PKG-*` actionable error codes,
- deployment documented as FUTURE WORK (client + protocol complete),
- full existing + new suite green.

Proceeding to **GATE 4 - Cache, Resolution & Lock (P2.7, P2.8, P2.9)** next,
per the gated methodology.
