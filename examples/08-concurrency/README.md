# 08 — Concurrency

Runs on the Phase 107 work-stealing concurrency runtime: `spawn`/`join`,
bounded/unbounded channels, and actors. The runtime is embedded in generated
code through the Go front end on the `x86_64-windows` host.

| Example                     | Status      | Engine | What it shows              |
|-----------------------------|-------------|--------|----------------------------|
| `01_parallel_sum.kark`      | Experimental| go     | spawn/join grid            |
| `02_channel_ping.kark`      | Experimental| go     | producer channel, drain    |
| `pipeline/` (../concurrency) | Experimental| go     | channels + actors, E2E     |

## Status wording

These examples are truthful to the implementation:

- **Engine: go** — the self-hosted `kcc` engine lexes/parses the keywords but
  does not yet lower them (documented post-107 parity boundary).
- Deterministic by construction: joins serialize per-task results; channel
  reads use known message counts (the runtime's close-drain is deterministic,
  not a `-1` sentinel).

## Run

```
karkain run examples/08-concurrency/01_parallel_sum.kark --engine go
karkain run examples/08-concurrency/02_channel_ping.kark --engine go
```