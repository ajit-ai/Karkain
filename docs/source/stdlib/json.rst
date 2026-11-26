.. _stdlib-json:

std.json — JSON parsing and serialization
=========================================

:implemented:`Implemented` — importable on both engines.

``std.json`` provides basic JSON field extraction and serialization using
string operations. No external dependencies.

The v1.2.1 gate ``TestPhaseV121_`` executes every documented function on both
engines. ``json_array_length`` counts top-level elements of a flat array; a
corrective change in that slice fixed its depth accounting, which previously
reported 1 for a three-element array.

Importing
---------

.. code-block:: karkain

   import std.json

Module reference
----------------

``json_get_field(json, field)``
   Extracts a string value for a given key from a flat JSON object.
   Returns: ``string``

``json_get_int(json, field)``
   Extracts an integer field from a JSON object.
   Returns: ``int``

``json_get_bool(json, field)``
   Extracts a boolean field from a JSON object.
   Returns: ``bool``

``json_has_field(json, field)``
   Returns ``true`` if the JSON object has the given field.
   Returns: ``bool``

``json_stringify(s)``
   Wraps a string in JSON quotes.
   Returns: ``string``

``json_array_length(json)``
   Returns the number of elements in a JSON array.
   Returns: ``int``

Example
-------

.. code-block:: karkain

   import std.json

   func main() {
       string j = "{\"name\":\"karkain\",\"count\":42}"
       print(json_get_field(j, "name"))    // karkain
       print(json_get_int(j, "count"))     // 42
       print(json_has_field(j, "name"))    // true
   }

JSON DOM (v1.2.20)
------------------------

The six flat helpers remain for compatibility. v1.2.20 adds a recursive
DOM on the C23 path, available on both the Go front end and kcc.

``json_dom_parse(text)``
    Parse JSON text into a Value tree. Objects become maps, arrays become
    arrays, scalars become Karkain values and JSON null becomes the
    ``__json_null__`` sentinel. Returns an empty string on failure.

``json_dom_type(node)``
    Return ``null``, ``bool``, ``number``, ``string``, ``array`` or ``object``.

``json_dom_get(node, path)``
    Query a dotted path with array indices, for example ``a.b[0].c``.

``json_dom_serialize(node)``
    Serialize the DOM node back to JSON with correct escaping.

``json_dom_error()``
    Return the last deterministic JSON failure string, or empty on success.

Native and WASM have no JSON builtins; those targets refuse the surface by
the existing platform boundaries.