=================================================
Semantic Versioning Policy (SemVer)
=================================================

Karkain follows Semantic Versioning 2.0.0 (SemVer) for versioning the
language, toolchain, and standard library. This policy defines how version
numbers are assigned and what constitutes a breaking change.

.. contents:: Sections
   :local:
   :depth: 1

Version format
==============

Karkain uses the standard SemVer format: ``MAJOR.MINOR.PATCH``

* **MAJOR**: Incompatible API changes
* **MINOR**: Backwards-compatible functionality additions
* **PATCH**: Backwards-compatible bug fixes

Additional labels:

* ``-beta.N``: Pre-release beta versions (e.g., ``1.0.0-beta.1``)
* ``-rc.N``: Release candidate versions (e.g., ``1.0.0-rc.1``)
* ``+build``: Build metadata (e.g., ``1.0.0+20260914``)

Current version
===============

**Current stable version**: ``1.0.0`` (since Phase 119)

The stable version is defined in the ``VERSION`` file at the repository root
and reflected across:
* Go CLI banner
* kcc binary banner
* Generated C header comments
* Documentation version markers

When to increment
=================

MAJOR (X.0.0 → Y.0.0)
---------------------

Increment the MAJOR version when:

* Removing or renaming stable language syntax
* Removing or changing the signature of stable stdlib functions
* Changing the behavior of a stable stdlib function in an incompatible way
* Removing stable CLI commands or changing their exit codes
* Changes to the stable diagnostic error codes (K001–K113)
* Changes to the target model that break existing ``--target`` values

MINOR (1.0.0 → 1.1.0)
-----------------------

Increment the MINOR version when:

* Adding new stable language syntax
* Adding new stable stdlib modules or functions
* Adding new stable CLI commands
* Adding new target architectures
* Adding new features that don't break existing code

PATCH (1.0.0 → 1.0.1)
-----------------------

Increment the PATCH version when:

* Bug fixes that don't change stable APIs
* Performance improvements
* Documentation updates
* Tooling improvements (build, install, CI)

Breaking change policy
======================

What constitutes a breaking change
-----------------------------------

A change is considered breaking if:

1. **Language surface**: Code that compiled successfully before now fails
   to compile or produces different runtime behavior
2. **Stdlib API**: Function signatures change or functions are removed
3. **CLI contract**: Command options change or exit codes change
4. **Target model**: Existing ``--target`` values become invalid
5. **Diagnostics**: Error codes change meaning or are removed

Deprecation process
-------------------

Before making a breaking change:

1. Document the deprecation in the relevant stdlib module documentation
2. Add a deprecation warning in the compiler or runtime (where feasible)
3. Maintain the old API for at least one MINOR version
4. Remove the deprecated API in the next MAJOR version

Stable API definition
=====================

The stable API is defined in ``docs/source/reference/stable-api.rst`` and
includes:

* **Stable syntax**: Language constructs that are guaranteed to work
* **Stable CLI**: Commands, options, and exit codes
* **Stable stdlib**: Importable ``std.*`` modules and their functions
* **Stable diagnostics**: Error code contracts and runtime error model
* **Stable target model**: ``--target`` values and host matrix

Changes to items in the stable API require:

* Documentation update in ``stable-api.rst``
* Increment of MAJOR version if breaking
* Increment of MINOR version if additive
* Update of ``VERSION`` file
* Release notes describing the change

Experimental vs Stable
======================

Items marked as **Experimental** in ``docs/source/status/scope.rst`` are not
part of the stable API and may change without MAJOR version increments.

Experimental items become stable when:

* They have been tested in beta releases
* They have documentation in ``stable-api.rst``
* They pass the relevant phase gates
* The maintainer explicitly promotes them to stable

Feature freeze during releases
==============================

During release candidate preparation (RC phase), the project operates under
a feature freeze:

* No new major language features
* No unnecessary syntax changes
* No broad stdlib redesign
* No compiler architecture rewrite

See :doc:`feature-freeze` for details on what is allowed during freeze.

Version verification
====================

The version is verified by:

* ``karkain --version`` command
* ``karkain version`` CLI command
* Generated C header comments
* Phase 118 RC gate test
* Release checklist verification

Cross-engine parity
===================

Karkain maintains byte-identical behavior between the Go engine and the
self-hosted kcc engine for all stable APIs. Changes that break this parity
require:

* Both engine implementation updates
* Parity gate tests passing
* Documentation of the parity restoration

Implementation notes
====================

* The ``VERSION`` file is the single source of truth for version numbers
* All other version references must be kept in sync
* CI builds should verify version consistency
* Release tags must match the ``VERSION`` file at the time of tagging

.. seealso::

   :doc:`feature-freeze` — Release-candidate feature freeze policy
   :doc:`/status/compatibility` — Compatibility guarantees
   :doc:`/reference/stable-api` — Current stable API snapshot
