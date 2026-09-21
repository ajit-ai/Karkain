.. _operators:

==========
Operators
==========

Karkain provides a small, explicit set of operators. There are no bitwise
operators and no ternary operator.

Arithmetic operators
====================

.. list-table::
   :widths: 10 30 40
   :header-rows: 1

   * - Operator
     - Name
     - Works on
   * - ``+``
     - Addition
     - ``int``, ``float64``
   * - ``-``
     - Subtraction
     - ``int``, ``float64``
   * - ``*``
     - Multiplication
     - ``int``, ``float64``
   * - ``/``
     - Division
     - ``int``, ``float64``
   * - ``%``
     - Modulo (remainder)
     - ``int``

Comparison operators
====================

.. list-table::
   :widths: 10 30
   :header-rows: 1

   * - Operator
     - Name
   * - ``==``
     - Equal
   * - ``!=``
     - Not equal
   * - ``<``
     - Less than
   * - ``>``
     - Greater than
   * - ``<=``
     - Less than or equal
   * - ``>=``
     - Greater than or equal

Logical operators
=================

.. list-table::
   :widths: 10 30
   :header-rows: 1

   * - Operator
     - Name
   * - ``&&``
     - Logical AND
   * - ``||``
     - Logical OR
   * - ``!``
     - Logical NOT

.. note::

   ``&&`` and ``||`` do **not** short-circuit in the traditional sense.
   Both sides of a binary logical expression are fully evaluated before the
   result is computed.

String operators
================

``+`` concatenates two strings:

.. code-block:: karkain

   "hello" + " " + "world"

Assignment operator
===================

``=`` assigns a value to a variable or mutable binding:

.. code-block:: karkain

   x = 10

Index operator
==============

``[]`` accesses an element of an array or map by index:

.. code-block:: karkain

   arr[0]
   myMap["key"]

Member access operator
======================

``.`` accesses a field of a struct or record, or qualifies a call
against an imported module:

.. code-block:: karkain

    point.x
    math.twice(21)

Precedence table
================

Highest to lowest.

.. list-table::
   :widths: 10 40
   :header-rows: 1

   * - Precedence
     - Operators
   * - 1
     - ``*``  ``/``  ``%``
   * - 2
     - ``+``  ``-``
   * - 3
     - ``<``  ``>``  ``<=``  ``>=``
   * - 4
     - ``==``  ``!=``
   * - 5
     - ``&&``  ``||``
   * - 6 (lowest)
     - ``=``

.. note::

   There are **no bitwise operators** in the current language. Bitwise
   operations are planned for a future phase.

   There is **no ternary operator**. Use ``if`` as an expression instead:

   .. code-block:: karkain

       let result = if x > 0 { x } else { 0 }
