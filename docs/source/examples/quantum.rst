Quantum
=======

:not-implemented:`Not Yet Implemented` — quantum *machinery* exists inside the
compiler as infrastructure only, and **no quantum syntax is runnable end-to-end**
on either engine. The category is honest: ``Planned / Infrastructure-only``.

Karkain's quantum story is explicitly not MicroQuantum. The architecture
boundary is:

.. code-block:: text

   Karkain (.kark)
      |
      v  (future: quantum integration layer)
   Quantum integration layer
      |
      v
   MicroQuantum / future quantum backends

What exists today:

* Parser AST nodes (``CircuitDecl``, ``QubitAssignStmt``, ``QPUOpExpr``,
  ``MeasureExpr``)
* HIR quantum nodes and types
* Quantum safety analysis (no-cloning, lifecycle)
* OpenQASM 3 / OpenPulse / QEC surface-code generators
* A WGSL simulator bridge (validator)

**Verified fact:** the lexer/parser do not tokenize ``circuit`` / ``qubit`` /
``qpu`` / ``measure`` — a ``circuit`` declaration fails ``karkain check`` with
a parse error. The first real example (Bell state, GHZ, Deutsch–Jozsa, Grover)
can only ship after parser wiring and a simulator runtime land.

.. seealso::

   :doc:`/status/planned` — the roadmap vocabulary.
   :doc:`/compiler/architecture` — where the quantum machinery lives.