.. _stdlib-encoding:

std.encoding — Hex, Base64 and UTF-8 codecs
===========================================

:implemented:`Implemented` — importable on both engines, byte-identical.

``std.encoding`` provides hexadecimal, Base64 and UTF-8 byte encoding and
decoding. Karkain strings *are* UTF-8 byte strings, so ``utf8_encode`` and
``utf8_decode`` are identity operations and ``utf8_valid`` reports
well-formedness. Hex and Base64 operations delegate to runtime byte helpers
(the language has no bitwise operators or byte↔character primitives, so the
byte math lives in the Karkain runtime behind the documented builtin
boundary).

Hex and Base64 behavior is verified against the NIST FIPS 180 vectors and
RFC 4648, and is byte-identical on both engines.

Importing
=========

.. code-block:: karkain

   import std.encoding

Module reference
================

``hex_encode(data)``
   Returns the lowercase hexadecimal encoding of ``data`` (every byte becomes
   two hex characters).
   Returns: ``string``

``hex_decode(hex_string)``
   Decodes a hexadecimal string into its bytes, returned as a string. Raises
   a ``runtime error: invalid hex string`` if the input is malformed (odd
   length or non-hexadecimal characters).
   Returns: ``string``

``base64_encode(data)``
   Returns the standard RFC 4648 Base64 encoding of ``data``, including
   ``=`` padding.
   Returns: ``string``

``base64_decode(b64_string)``
   Decodes a standard Base64 string into its bytes, returned as a string.
   Raises a ``runtime error: invalid base64 string`` if the input is
   malformed.
   Returns: ``string``

``utf8_encode(s)``
   Returns the UTF-8 encoding of ``s``. Identity — Karkain strings are
   already UTF-8 byte strings.
   Returns: ``string``

``utf8_decode(s)``
   Returns the string decoded from UTF-8 bytes. Identity — Karkain strings
   are already UTF-8 byte strings.
   Returns: ``string``

``utf8_valid(s)``
   Returns ``true`` if the bytes in ``s`` form well-formed UTF-8.
   Returns: ``bool``

Malformed input
===============

``hex_decode`` and ``base64_decode`` fail with the same diagnostic on both
engines:

.. code-block:: text

   runtime error: invalid hex string at example.kark:12
   runtime error: invalid base64 string at example.kark:18

as a program failure (exit code ``1``) — same source-located runtime error
model used across the language.

Verified vectors
================

Hexadecimal (RFC 4648 §8 / FIPS 180):

.. code-block:: text

   hex_encode("")    -> ""
   hex_encode("abc") -> "616263"

Base64 (RFC 4648 §10 test vectors):

.. code-block:: text

   base64_encode("")      -> ""
   base64_encode("f")     -> "Zg=="
   base64_encode("fo")    -> "Zm8="
   base64_encode("foo")   -> "Zm9v"
   base64_encode("foob")  -> "Zm9vYg=="
   base64_encode("fooba") -> "Zm9vYmE="
   base64_encode("foobar")-> "Zm9vYmFy"

UTF-8:

.. code-block:: text

   utf8_valid("hello")          -> true
   utf8_valid("caf\xc3\xa9")    -> true   // "café" as UTF-8 bytes
   utf8_valid("caf\xa9")        -> false  // lone continuation byte

Example
=======

.. code-block:: karkain

   import std.encoding

   func main() {
       println(hex_encode("abc"))            // 616263
       println(hex_decode("616263"))         // abc
       println(base64_encode("foo"))         // Zm9v
       println(base64_decode("Zm9v"))        // foo
       println(utf8_valid("hello"))          // true
   }

   $ karkain run example.kark
   616263
   abc
   Zm9v
   foo
   true