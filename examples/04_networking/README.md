# 04 — Networking

**Status: Planned**

No networking APIs are callable from `.kark` today.

| Example | Status  | Notes |
|---------|---------|-------|
| (none)  | Planned | See below |

## What exists behind the scenes

- `pkg/stdlib/http.go` ships placeholder Go functions (`HttpGet`, `HttpPost`,
  `HttpServer`) — they are **stubs**, not wired into any `.kark`-callable
  surface, and must not be treated as a working HTTP module.
- Phase 15-era codegen comments mention a cross-platform socket abstraction,
  but no language-level TCP/UDP/HTTP surface has been implemented.

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