# KARKAIN - Phase P2 (Package Ecosystem Maturity) - GATE 5 Engineering Report

Phase: P2 - Package Ecosystem Maturity (gated methodology)
Gate: GATE 5 - Workspace Build & Test (P2.10, P2.11)
Date: 2026-09-04
Repo: `F:\Codes\Git\Karkain`
Base: GATE 4 committed/merged/pushed (`b3b9e85`)

---

## 1. GATE 5 SCOPE

- P2.10 - Workspace Build: dependency graph -> cycle detection -> topological
  order -> build each package exactly once.
- P2.11 - Workspace Test: run each member's tests in dependency order;
  dev-dependencies must NOT leak into normal builds.
- Wire the CLI `pkg workspace build|test` (and `ws`) subcommands to the new
  pipeline.

---

## 2. ARCHITECTURE

`pkg/pm` cannot import `pkg/cli` (import cycle: cli imports pm). Conversely
`pkg/cli` already depends on `pkg/pm` and owns the full compiler pipeline
(`BuildCommand`/`TestCommand`). Therefore:

- **`pkg/pm/workspace.go`** owns the pure, testable graph logic:
  - `WorkspaceOrder(rootDir) ([]WorkspaceMember, error)` - reads
    `karkain.workspace.json`, resolves each member to its absolute directory
    (de-duplicated), builds the inter-member dependency graph from
    `source = "workspace"` manifest entries that resolve inside the workspace
    root, then uses **Kahn's algorithm** (deterministic, with sorted ready
    queue) for topological order.
  - **Cycle detection**: if the emitted count < member count, the remaining
    members form a cycle and a hard `E-PKG-WS-CYCLE` error is returned (never
    silently tolerated - build cannot deadlock).
  - `WorkspaceMember{Name, Dir, Order}` exposes the topological index so
    consumers can assert build-once ordering.
  - **Dev-dep isolation**: `workspaceDepDirs` collects edges only from
    `m.Dependencies`, never from `m.DevDependencies` (P2.11). A sibling
    referenced only as a dev-dep is not a build-order predecessor. Non-workspace
    deps (registry/git/external-local) are ignored - they are not members.
  - `WorkspaceTestOrder` documents the shared ordering seam.

- **`pkg/cli/workspace.go`** wires graph to compilation:
  - `WorkspaceBuild(rootDir, cfg, verbose)` - calls `pm.WorkspaceOrder`, then
    compiles each member once in order via the existing `BuildCommand`
    pipeline. Members without an executable entrypoint (library members) are
    skipped, not errors.
  - `WorkspaceTest(rootDir, cfg, verbose)` - calls `pm.WorkspaceOrder`, then
    runs each member's test files via the existing `TestCommand` pipeline.

- **`cmd/karkain/main.go`** dispatcher: `pkg workspace build|test` now calls the
  CLI versions (with a fresh `codegen.Config`).

No working subsystem was rewritten; the existing `InitWorkspace` /
`AddWorkspaceMember` / `WorkspaceConfig` / `WorkspaceFile` API is preserved.

---

## 3. VERIFICATION

| Check | Result |
|-------|--------|
| `go build ./...` | PASS (exit 0) |
| `go vet ./pkg/pm/... ./pkg/cli/...` | PASS |
| gofmt (new/modified, CR-normalized) | PASS |
| `go test ./pkg/pm/... -count=1` | PASS (incl. 6 new workspace tests) |
| `go test ./pkg/cli/... -count=1` | PASS (incl. 3 new workspace E2E) |
| module-system regression (lexer/parser/codegen/sema) | PASS |

New tests - `pkg/pm/workspace_test.go` (6):
- `TestWorkspaceOrder_TopologicalAndBuildOnce` - diamond (libA, libB->libA,
  app->libA+libB) yields [libA libB app]; each member exactly once.
- `TestWorkspaceOrder_CycleDetected` - a<->b cycle is a hard `E-PKG-WS-CYCLE`.
- `TestWorkspaceOrder_DevDepsExcluded` - sibling referenced only in
  `DevDependencies` does not create a build edge.
- `TestWorkspaceOrder_EmptyWorkspace` - empty workspace -> no members, no error.
- `TestWorkspaceOrder_NotAWorkspace` - missing workspace file -> `E-PKG-WS-NO-ROOT`.
- `TestWorkspaceOrder_ExternalDepIgnored` - local dep outside root is not a member.

New tests - `pkg/cli/workspace_test.go` (3, E2E against the real pipeline):
- `TestWorkspaceBuild_OrderE2E` - `WorkspaceBuild` compiles lib before app
  (captured stdout order).
- `TestWorkspaceBuild_CycleFails` - cycle rejected before any build runs.
- `TestWorkspaceTest_DiscoveryE2E` - `WorkspaceTest` runs each member's tests in
  dependency order.

Manual smoke test (built `karkain.exe`):
- `pkg ws init/add` -> correct `karkain.workspace.json`.
- `pkg ws build` with app->lib: `BUILD [0] lib`, `BUILD [1] app` (dependency
  order through the real dispatcher).
- `pkg ws build` with a lib<->app cycle: `error[E-PKG-WS-CYCLE]`, exit 1.
- `pkg ws test` runs each member's test summary, exit 0.

---

## 4. GATE 5 VERDICT

> ### PASS

GATE 5 (Workspace Build & Test) meets the master-prompt acceptance criteria:
- dependency graph -> cycle detection -> topological order -> build each
  package exactly once (P2.10),
- per-member test execution in dependency order with dev-dep isolation (P2.11),
- no import cycle between `pkg/pm` and `pkg/cli`,
- extended, not rewritten, architecture; full suite green.

Proceeding to **GATE 6 - final verification + full P2 report** next, per the
gated methodology.
