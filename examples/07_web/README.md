# 07 — Web

**Status: Planned**

No web framework or HTTP server is runnable from `.kark` today.

| Example | Status  | Notes |
|---------|---------|-------|
| (none)  | Planned | See below |

## What exists behind the scenes

- `pkg/stdlib/http.go` contains placeholder `HttpGet` / `HttpPost` /
  `HttpServer` functions that return immediately without doing work and are
  not wired into the language. They are a marker of intent, not a module.

## Roadmap for this category

1. socket runtime in the generated C (foundation)
2. `std.net` TCP client/server (foundation)
3. `std.http` request/response (serialization surface)
4. routing + middleware conventions
5. static file serving

When step (1) lands, the first real `05_*/07_web` examples can be a plain
TCP echo client/server. Until then this page stays honest.

## Run

Nothing to run yet. See `docs/source/examples/web.rst`.