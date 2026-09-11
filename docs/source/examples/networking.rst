:orphan:

Networking
==========

:not-implemented:`Not Yet Implemented`

There is **no networking API in Karkain today**. No socket or HTTP
functions exist in the standard library, the runtime, or either compiler
engine, and nothing on this page describes a hypothetical interface as real.
(The ``http.get`` name that appears in some historical notes is a
recognized builtin name only — it has no network implementation and must
not be built on.)

Planned
-------

:planned:`Planned` — the following are design intentions, not APIs. No
signatures, functions or modules exist yet.

* TCP/UDP sockets (``connect``/``listen``/``accept``-style primitives and an
  address model).
* An HTTP client and server (requests, responses, status codes, routing).
* Examples in this category will be added when the capability is
  implemented — the category placeholder exists to document that
  honestly, per the Phase 113 framework. Phase 114 will populate it with
  validated programs once a networking runtime exists.

.. seealso::

   :doc:`/status/planned` — the planned-feature register.
   :doc:`/status/index` — the status vocabulary used on this site.