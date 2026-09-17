Web
===

:implemented:`Implemented` — ``std.http`` provides minimal HTTP/1.1 client
and server building blocks callable from ``.kark`` on both engines, built on
top of the ``std.net`` TCP socket primitives.

The ``examples/07-web/`` category contains two runnable examples
demonstrating HTTP request/response objects and server-side handling.

.. list-table:: Examples
   :widths: 40 60

   * - ``01_http_loopback.kark``
     - Request/receive/serve loopback: server receives a request, creates a
       200 response, client prints both.
   * - ``02_http_codec.kark``
     - HTTP object constructors and codecs: build and parse requests and
       responses, read back fields.

Implemented ``std.http`` surface
--------------------------------

.. code-block:: kark

   std.http (minimal HTTP/1.1):
     http_make_request(method, path, headers, body) -> request
     http_make_response(status, reason, headers, body) -> response
     http_new_headers() -> headers map
     http_parse_request(text) -> request
     http_parse_response(text) -> response
     http_parse_headers(header_body) -> headers map
     http_split_message(text) -> parts
     http_build_request(method, host, port, path, headers, body) -> wire text
     http_build_response(status, reason, headers, body) -> wire text
     http_request_send(method, host, port, path, headers, body) -> response
     http_get(host, port, path) -> response
     http_post(host, port, path, body) -> response
     http_listen(host, port) -> listener    # raw socket level
     http_accept(listener) -> connection
     http_read_request(conn, max) -> request
     http_write_response(conn, status, reason, body)
     http_read_all(fd, limit) / http_read_until(fd, marker, limit)
     http_close(fd)

Values use the shapes ``Request { method, path, headers, body, error }`` and
``Response { status, reason, headers, body, error }`` (headers: a name →
value string map). Body transfer is Content-Length based and every message is
terminated with ``Connection: close``; failure is signalled via the ``error``
field (empty = success).

HTTP runs on top of ``std.net`` TCP sockets, pinned byte-identical on both
engines in the phase114 gate.

.. seealso::

   :doc:`/examples/networking` — the prerequisite networking surface.