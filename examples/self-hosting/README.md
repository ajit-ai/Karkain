# Self-Hosting examples

Examples that demonstrate Karkain-owned compiler infrastructure — the
pieces of the compiler pipeline written in Karkain itself.

- `kir/` — the KIR v1 text emitter (`src/compiler/kir.kark`), the first
  fully Karkain-owned compiler component (Phase 120). `karkain kir`
  renders any `.kark` file as deterministic KIR text through the
  self-hosted engine.