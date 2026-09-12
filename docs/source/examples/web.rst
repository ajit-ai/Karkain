Web
===

:not-implemented:`Not Yet Implemented` — there is no web framework or HTTP
server runnable from ``.kark`` today.

The category directory ``examples/07-web/`` exists only to preserve the
15-category framework and record the honest status: no web or HTTP surface is
implemented, and a sockets prerequisite (see :doc:`/examples/networking`) has
not shipped yet.

The roadmap path is:

1. socket runtime in the generated C,
2. ``std.net`` TCP client/server,
3. ``std.http`` request/response,
4. routing and middleware conventions,
5. static file serving.

A plain TCP echo client/server will be the first real example once step (1)
lands.

.. seealso::

   :doc:`/examples/networking` — the prerequisite networking surface.
   :doc:`/status/planned` — the roadmap vocabulary.