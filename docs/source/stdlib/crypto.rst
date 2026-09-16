.. _stdlib-crypto:

std.crypto — SHA-256 and SHA-512 digests
========================================

:implemented:`Implemented` — importable on both engines, byte-identical.

``std.crypto`` provides the SHA-256 and SHA-512 cryptographic hash functions.
Digests use the Karkain runtime's embedded reference implementation and are
returned as lowercase hexadecimal strings. The runtime implementation is the
well-known compact public-domain algorithm built on the FIPS 180-4 constants.

Importing
---------

.. code-block:: karkain

   import std.crypto

Module reference
----------------

``sha256(data)``
   Returns the SHA-256 digest of ``data`` as a 64-character lowercase
   hexadecimal string.
   Returns: ``string``

``sha512(data)``
   Returns the SHA-512 digest of ``data`` as a 128-character lowercase
   hexadecimal string.
   Returns: ``string``

Verified vectors (NIST FIPS 180)
--------------------------------

SHA-256:

.. code-block:: text

   sha256("abc")
     -> ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad
   sha256("")
     -> e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855

SHA-512:

.. code-block:: text

   sha512("abc")
     -> ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a
        2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f

Both engines produce byte-identical digests for the same input, so hashes of
``examples/stdlib_v2``-style programs match exactly whether compiled with the
Go front end or the self-hosted ``kcc`` engine.

Example
-------

.. code-block:: karkain

   import std.crypto

   func main() {
       println(sha256("karkain"))
       println(len(sha256("karkain")))   // 64
       println(len(sha512("karkain")))   // 128
   }

   $ karkain run example.kark
   00e0cba20c10cac449eb885a9926a4b646f0ac163ed7fbc5704d9d8a057ef44d
   64
   128