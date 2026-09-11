:orphan:

Security
========

:not-implemented:`Not Yet Implemented` — with one real exception.

What exists today
-----------------

* :implemented:`Implemented` — **``std.crypto``** (Phase 109): ``sha256()``
  and ``sha512()`` digests, validated byte-for-byte against NIST FIPS 180
  vectors on **both** engines. See :doc:`/stdlib/crypto`.
* :implemented:`Implemented` — **``std.encoding``**: hex and Base64 codecs
  (RFC 4648), useful for digest formatting.

Everything broader — symmetric/asymmetric ciphers, MACs/HMACs, secure random
number generation, key management, TLS — is **not implemented**. Karkain
ships no cryptographic primitives beyond the two SHA-2 digests.

Planned
-------

:planned:`Planned` — design intentions, not APIs. No cipher, MAC, KDF, RNG
or TLS surface exists.

* Broader security/crypto APIs (the roadmap's security track).
* Examples will be added as capabilities are implemented; Phase 114 will
  populate this category with validated programs once the surface exists.

.. seealso::

   :doc:`/stdlib/crypto` — the implemented digest surface.
   :doc:`/stdlib/encoding` — the implemented hex/Base64 codecs.
   :doc:`/status/planned` — the planned-feature register.