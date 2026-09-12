=====================
Workspaces
=====================

A workspace is a set of member packages in one repository that are built,
tested and resolved together. An application member can consume a sibling
library member through the workspace dependency mechanism — the Phase 117
workspace dependency fix made cross-member calls resolve deterministically.

.. contents:: Sections
   :local:
   :depth: 1

When to use a workspace
=======================

Use a workspace when one repository contains several ``.kark`` packages and
an application must call functions that live in a sibling package. For a
single package, the :doc:`first-project` layout is enough.

The minimal workspace
=====================

Create a workspace root with two members, ``app`` and ``library``:

.. code-block:: text

    workspace/
      karkain.toml       # workspace manifest (karkain init / workspace)
      app/
        karkain.toml     # app manifest
        src/
          main.kark
      library/
        karkain.toml     # library manifest
        src/
          api.kark

Setup
=====

.. code-block:: console

    $ karkain workspace init              # in the workspace root
    $ karkain workspace add app
    $ karkain workspace add library

Each member directory gets a manifest. The ``library`` member exposes its
API from a **non-main** module (the assembly keeps only the root file's
``func main``), e.g. ``library/src/api.kark``:

.. code-block:: karkain

    func greet() {
        print("hi from library api")
    }

The ``app`` member declares the workspace dependency in its manifest and
calls the library function from ``app/src/main.kark``:

.. code-block:: karkain

    func main() {
        greet()
    }

Build, test and run the whole workspace:

.. code-block:: console

    $ karkain workspace build
    $ karkain workspace test
    $ karkain workspace run

Members are processed in dependency order — a member that depends on
another builds after it. A dependency cycle is rejected before any build.
The ``workspace graph`` command prints the member dependency graph.

Behavior on the Phase 117 fix
=============================

Cross-member calls resolve through the workspace dependency sources:
an ``app`` member may call a function exported by a sibling ``library``
member. The reverse is also true only if a dependency is declared. A
cross-member call without a declared dependency fails at build/run with an
undefined-identifier error on both engines — it is never silently dropped.

The runnable example lives at ``examples/workspace/`` in the repository and
is exercised by the release-candidate gate
(``pkg/cli/phase118_rc_test.go``).

Real surfaces and boundaries
============================

* ``workspace list | build | test | run | graph | init | add | remove |
  clean | lint | check`` are implemented.
* Workspace dependencies are resolved from the local repository only —
  there is no public package registry (see
  :doc:`/reference/stable-api`).
* The package manager also supports ``--source local`` (path) dependencies
  for a single project; see :doc:`dependencies`.

.. seealso::

   :doc:`dependencies` — single-project dependency resolution.
   :doc:`first-project` — the minimal single-package project.
   :doc:`project-layout` — the manifest and directory conventions.
   :doc:`/tools/cli` — the ``workspace`` command reference.