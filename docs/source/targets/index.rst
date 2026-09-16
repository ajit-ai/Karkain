=======
Targets
=======

Karkain owns its cross-compilation model. ``karkain build --target
<triple>`` is a real, explicit cross-compilation switch backed by the
Karkain-owned ``pkg/target`` package (arch/OS/env, canonical short
triples, long-form normalization, ``Features``, host-vs-foreign
``SameMachine``); ``karkain target`` reports the host triple and the
full supported matrix.

Phase 111 implemented explicit cross-compilation: same-machine targets
use the historical host probe with byte-identical flags, foreign targets
use triple-prefixed GNU cross-gcc → clang ``--target``, and a missing
cross-linker fails deterministically with a ``ToolchainError`` (exit 6)
— never a silent host fallback.

.. toctree::
   :maxdepth: 2

   host-targets
   cross-compilation
   target-triples
   compute-targets

Overview
========

.. code-block:: console

    $ karkain target
    Host: x86_64-windows
    Supported targets:
      ...

    # Cross-compile for a foreign target (build only)
    $ karkain build hello.kark --target x86_64-linux -o hello_linux

    # Cross-RUN is refused with a build-only hint
    $ karkain run hello.kark --target x86_64-linux
    Error: cannot run a x86_64-linux binary on the host (x86_64-windows): ...

.. seealso::

   :doc:`/tools/target` — the ``karkain target`` command.
   :doc:`target-triples` — the triple format accepted and normalized.
   :doc:`host-targets` — how the host is detected and why it matters.
   :doc:`cross-compilation` — Phase 111 behavior and the honest matrix.
   :doc:`compute-targets` — Phase 124 GPU/NPU/quantum target model.