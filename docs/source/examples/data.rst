:orphan:

Data
====

:not-implemented:`Not Yet Implemented`

There is **no databases/filesystem-data API** in Karkain today: no database
drivers, no query language, no structured-data format handling beyond the
flat file I/O in ``std.io`` (``readFile`` / ``writeFile`` / ``readLine``)
and the codecs in ``std.encoding`` (hex, Base64, UTF-8). No data-framework
surface exists in the standard library, the runtime, or either compiler
engine.

Planned
-------

:planned:`Planned` — what follows is an intention, not a contract. No APIs
exist.

* Data processing on top of the implemented array/string/float primitives.
* Examples will be added when the capability is implemented; the category
  placeholder exists so the framework documents the gap explicitly.

.. seealso::

   :doc:`/stdlib/index` — the implemented standard-library modules
   (``std.io``, ``std.encoding``) currently available for data work.
   :doc:`/status/index` — the status vocabulary used on this site.