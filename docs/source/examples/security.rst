Security
========

:implemented:`Implemented` — safe, educational security examples live under
``examples/14_security/`` and run byte-identically on both engines. All data
is synthetic; digests are verified against the published NIST FIPS 180
vectors.

The surface used is the real, importable ``std.crypto`` and ``std.encoding``
modules — no fabricated cryptographic APIs. See
:doc:`/stdlib/crypto` and :doc:`/stdlib/encoding`.

.. list-table:: examples/14_security/
   :widths: 30 70
   :header-rows: 1

   * - File
     - Demonstrates
   * - ``01_digests.kark``
     - ``sha256`` / ``sha512`` against NIST test vectors
   * - ``02_encoding_roundtrip.kark``
     - RFC 4648 hex / base64 round-trips
   * - ``03_password_hash.kark``
     - Educational salt + sha256 demonstration (deliberately **not** a KDF)
   * - ``04_utf8_text.kark``
     - UTF-8 encode / decode / validate with a non-ASCII string

Digest check (excerpt)
----------------------

.. code-block:: kark

   import std.crypto

   func main() {
       print(sha256("abc"))
       print(sha256(""))
       print(sha512("abc"))
   }

Expected output (verified against NIST):

.. code-block:: text

   ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad
   e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
   ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a…
   2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f

.. note::

   Malformed hex/base64 input raises the same source-located runtime error on
   both engines (``runtime error: invalid (hex|base64) string at file:line``).

.. seealso::

   :doc:`/stdlib/index` — the implemented standard-library modules.