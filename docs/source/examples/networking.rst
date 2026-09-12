Networking
==========

:not-implemented:`Not Yet Implemented` — no networking APIs are callable from
``.kark`` today.

The category directory ``examples/04-networking/`` exists in the corpus only
to preserve the 15-category framework and carry an honest README: there is no
socket or HTTP language surface, and the placeholder functions in
``pkg/stdlib/http.go`` are not wired into the language.

An intended future surface (roadmap only, **not** implemented):

.. code-block:: kark

   std.net:
     tcp_connect(host, port) -> connection
     tcp_listen(port) -> listener
     http_get(url) -> response
     http_server(port, handler) -> server

None of the above is runnable. The prerequisite is a socket runtime in the
generated C; until then this page stays honest and no example is provided.

.. seealso::

   :doc:`/status/planned` — the capability roadmap.
   :doc:`/examples/web` — the web category that depends on networking.