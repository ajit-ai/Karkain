# 04 — Networking

**Status: Planned**

No networking APIs are callable from `.kark` today.

| Example | Status  | Notes |
|---------|---------|-------|
| (none)  | Planned | See below |

## What exists behind the scenes

- No `std.net` / `std.http` surface exists and no Go placeholder stubs ship in
  the repository — the networking modules are simply not implemented (they are
  roadmap items, not files).
- The 1.0.0 repository cleanup removed the Phase-15-era placeholder
  `pkg/stdlib/http.go` stubs (they were never wired into any `.kark`-callable
  surface); networking remains Planned.

## Intended design (roadmap)

A future networking module would provide a small, honest surface:

```
std.net:
  tcp_connect(host, port) -> connection
  tcp_listen(port) -> listener
  http_get(url) -> response
  http_server(port, handler) -> server
```

None of the above is runnable yet. Until a socket runtime exists in the
generated C, examples stay Planned and this README stays the honest record.

## Run

Nothing to run yet. See `docs/source/examples/networking.rst`.