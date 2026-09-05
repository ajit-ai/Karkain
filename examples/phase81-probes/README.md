# Phase 81 — Compiler Correctness executable probes

Run from the repo root via `bin\karkain.exe`:

| Probe | File | Command |
|-------|------|---------|
| A — source isolation | `probeA/main.kark` | `run examples/phase81-probes/probeA/main.kark` (expect `42`; stray.kark must be excluded) ; `check examples/phase81-probes/probeA/main.kark` (must pass despite ghost() in stray.kark) |
| B — unparenthesized `if` | `probeB/if.kark` | `run examples/phase81-probes/probeB/if.kark` (expect 1..10) |
| C — `%` single path | `probeC/mod.kark` | `run examples/phase81-probes/probeC/mod.kark` (expect 1, -1, 1, -1, 1, 0, 2) |
| D — diagnostic location | `probeD/diag.kark` | `check examples/phase81-probes/probeD/diag.kark` (expect `diag.kark:4:1`, NOT `:0:1`) |

Golden outputs:

- Probe A run: `42`
- Probe A check: `Check passed.` (ghost() in stray.kark is deliberately undefined — isolation proves exclusion)
- Probe B run:
  ```
  1
  2
  3
  4
  5
  6
  7
  8
  9
  10
  ```
- Probe C run:
  ```
  1
  -1
  1
  -1
  1
  0
  2
  ```
- Probe D check: name-resolution error at `probeD/diag.kark:4:1` (real line, not 0:1).