# 07 - Web

**Status: Runnable**

| Example | Status  | Notes |
|---------|---------|-------|
| `01_http_loopback.kark` | Runnable | Request/receive/serve loopback |
| `02_http_codec.kark` | Runnable | Request/response constructors and codecs |

## What is implemented

- `std.http` provides minimal HTTP/1.1 client and server building blocks:
  `http_make_request`, `http_make_response`, `http_new_headers`,
  `http_parse_request`, `http_parse_response`, `http_parse_headers`,
  `http_split_message`, `http_build_request`, `http_build_response`,
  `http_request_send`, `http_get`, `http_post`, `http_listen`, `http_accept`,
  `http_read_request`, `http_write_response`, `http_read_all`,
  `http_read_until` and `http_close`.
- HTTP is built on top of `std.net` TCP sockets, runnable on both engines
  with byte-identical output (pinned in the Phase 114 gate).

## Run

```powershell
karkain run examples/07-web/01_http_loopback.kark
```