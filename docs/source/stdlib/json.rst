.. _stdlib-json:

std.json — JSON parsing and serialization
========================================

:implemented:`Implemented` — importable on both engines.

``std.json`` provides basic JSON field extraction and serialization using
string operations. No external dependencies.

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