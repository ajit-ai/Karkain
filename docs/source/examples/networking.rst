Networking
==========

:implemented:`Implemented` — ``std.net`` provides TCP client/server networking
callable from ``.kark`` on both engines (the Go front end and the self-hosted
``kcc`` engine), backed by socket primitives in the generated C preamble.

The ``examples/04-networking/`` category contains two runnable examples
demonstrating ``std.net`` TCP primitives end-to-end.

.. list-table:: Examples
   :widths: 40 60

   * - :doc:`01_tcp_echo <networking>`
     - TCP echo server + client loopback: bind, connect, send/recv, close.
   * - :doc:`02_tcp_roundtrip <networking>`
     - Two-message request/response round-trip over a single TCP connection.

Implemented ``std.net`` surface
-------------------------------

.. code-block:: kark

   std.net:
     net_address(host, port) -> address     # { host, port }
     net_endpoint(host, port) -> endpoint   # address + scheme ("tcp")
     net_dial(point) -> connection fd       # TCP connect (or -1)
     net_serve(point) -> listener fd        # bind + listen (or -1)
     net_accept_next(listener) -> connection fd
     net_recv(conn, max) -> received string
     net_send(conn, data) -> bytes sent
     net_shut(fd)
     net_error() -> last-error string
     net_fd_open(fd) -> bool                # fd >= 0

The module wraps the runtime builtins ``net_connect`` / ``net_listen`` /
``net_accept`` / ``net_read`` / ``net_write`` / ``net_close`` /
``net_last_error``. Blocking reads time out after 5 seconds; failures return
-1 (or ``""``) and set a deterministic last-error string readable via
``net_error``.

Networking is pinned byte-identical on both engines in the phase114 gate —
the runtime socket primitives live in the generated C preamble.

.. seealso::

   :doc:`/examples/web` — HTTP layer built on top of ``std.net``.