std.http
========

HTTP/1.1 client and server building blocks, plus a C23 TLS client surface.

Phase 125A added minimal HTTP/1.1 support building on the ``net_*`` runtime
builtins. The server half provides building blocks for deterministic single-
process loopback choreography; ``http_get`` / ``http_post`` are one-shot plain
TCP clients. v1.2.17 adds ``https_get`` / ``https_post`` on the C23 path.

.. note::

   Body transfer is Content-Length based. Every plain HTTP message terminates
   with ``Connection: close``. Failure in the plain client is signaled via the
   ``error`` field; TLS failure yields an empty string from ``https_get`` and a
   reason from ``https_last_error``.

Types
-----

``Request``
    ``{ method, path, headers, body, error }``
``Response``
    ``{ status, reason, headers, body, error }``
``headers``
    Map of header name (string) to header value (string)

Client functions
----------------

``http_make_request(method, path, headers, body)``
    Build a Request value
``http_new_headers()``
    Return empty header map
``http_get(host, port, path)``
    One-shot plain HTTP GET (returns Response)
``http_post(host, port, path, body)``
    One-shot plain HTTP POST (returns Response)
``http_request_send(method, host, port, path, headers, body)``
    Send a complete plain HTTP request
``http_response_recv(conn)``
    Receive Response from connection

Server functions
----------------

``http_make_response(status, reason, headers, body)``
    Build a Response value
``http_listen(host, port)``
    Bind and listen for HTTP connections
``http_accept(listener)``
    Accept next HTTP connection
``http_read_request(conn, max)``
    Read HTTP request from connection
``http_write_response(conn, status, reason, body)``
    Write HTTP response to connection
``http_close(conn)``
    Close HTTP connection

Parsing utilities
-----------------

``http_parse_request(data)``
    Parse HTTP request data into Request
``http_parse_response(data)``
    Parse HTTP response data into Response
``http_parse_headers(data)``
    Parse headers into map
``http_split_message(data)``
    Split message into headers and body
``http_build_request(method, host, port, path, headers, body)``
    Serialize Request to HTTP string
``http_build_response(status, reason, headers, body)``
    Serialize Response to HTTP string
``http_build_headers(headers)``
    Serialize headers to HTTP string

HTTPS (C23 path, v1.2.17)
-------------------------

``https_get(host, port, path, verify)``
    Perform a TLS GET and return the decrypted entity body (empty on failure).

``https_post(host, port, path, body, verify)``
    Perform a TLS POST and return the decrypted entity body.

``https_last_error()``
    Return the last TLS failure string (empty on success).

``verify=1`` validates the server certificate chain and hostname with the
platform trust store. ``verify=0`` disables certificate checks for loopback
self-signed test servers only; the connection is still encrypted. Windows uses
WinHTTP (Schannel); POSIX uses OpenSSL. The native and WASM targets have no
sockets and refuse these calls by name (v1.2.10 platform decision).

Example
-------

.. code-block:: kark

   import std.http

   func main() {
       let resp = http_get("127.0.0.1", 8080, "/")
       if (resp["error"] == "") {
           print resp["body"]
       }
       let body = https_get("127.0.0.1", 8443, "/", 0)
       print(body)
   }

See ``examples/07-web/`` for HTTP loopback and codec examples.

Concurrent HTTP server (v1.2.18)
---------------------------------

``http_serve_echo(host, port)`` binds a listener and serves each connection
in a spawned ``http_echo_worker`` task. The worker reads one request, writes
``200 OK`` with body ``"hello " + req["path"]`` and closes the socket.
Because ``spawn`` requires a named top-level function, the worker is public
in this module. Tasks are detached; the accept loop continues until a
listener error.

The server is C23-only. Native targets have no socket syscalls (v1.2.10)
and WASM rejects concurrency builtins with K108; plain HTTP only — the
v1.2.17 HTTPS surface is a client.

.. code-block:: kark

   import std.http

   func main() {
       http_serve_echo("127.0.0.1", 8080)
   }