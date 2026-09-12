# Workspace Example

A minimal reproducible workspace: an `app` member that consumes a sibling
`library` member through the workspace dependency mechanism (the Phase 117
workspace-dependency fix).

```
workspace
  ├── karkain.workspace.json   # workspace root manifest
  ├── app                      # application member
  │   ├── karkain.toml         # declares the workspace dependency
  │   └── src/main.kark        # calls greet() from the library member
  └── library                  # library member
      ├── karkain.toml
      └── src/main.kark, src/api.kark
```

The library exposes its API from a **non-main** module (`src/api.kark`);
the workspace assembly keeps only the root file's `func main`, so the app
calls `greet()` which is defined there.

Run from inside `examples/workspace`:

```
karkain workspace build     # build all members in dependency order
karkain workspace run       # run all member entrypoints
karkain workspace graph     # show member dependency graph
```

Verified by `pkg/cli/phase118_rc_test.go` (release-candidate gate).