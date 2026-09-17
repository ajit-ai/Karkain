# 04 - Networking

**Status: Runnable**

| Example | Status  | Notes |
|---------|---------|-------|
| `01_tcp_echo.kark` | Runnable | TCP echo server + client loopback |
| `02_tcp_roundtrip.kark` | Runnable | Two-message request/response round-trip |

## What is implemented

- `std.net` provides `net_address`, `net_endpoint`, `net_dial`, `net_serve`,
  `net_accept_next`, `net_recv`, `net_send`, `net_shut`, `net_error` and
  `net_fd_open` - TCP socket networking usable end-to-end from `.kark` on both
  engines, byte-identical (pinned in the Phase 114 gate). The module wraps the
  runtime builtins `net_connect`/`net_listen`/`net_accept`/`net_read`/
  `net_write`/`net_close`/`net_last_error`.

## Run

```powershell
karkain run examples/04-networking/01_tcp_echo.kark
```