========
Packages
========

Karkain ships a real package manager with a deterministic resolver, a
lockfile, git and local source fetching, and a full ``pkg`` command
namespace.

Usage
=====

Top-level shortcuts, all of which have an equivalent ``karkain pkg
<verb>`` spelling:

.. code-block:: console

    $ karkain init [name]          # create a new project (in current dir)
    $ karkain new <name>           # create a new project directory
    $ karkain add <pkg> [version]  # add a dependency
    $ karkain remove|rm <pkg>      # remove a dependency
    $ karkain update [pkg]         # re-resolve versions, write karkain.lock
    $ karkain fetch                # fetch resolved dependencies (lockfile)
    $ karkain list                 # list direct + transitive dependencies
    $ karkain tree                 # show recursive dependency graph

Full command set
================

.. list-table::
   :widths: 42 58
   :header-rows: 1

   * - Command
     - Description
   * - ``pkg init [name]``
     - Create a new project
   * - ``pkg new <name>``
     - Create a new project directory
   * - ``pkg add <pkg> [version]``
     - Add a dependency (``--source registry|git|local``,
       ``--url <url>``)
   * - ``pkg remove|rm <pkg>``
     - Remove a dependency
   * - ``pkg update [pkg]``
     - Re-resolve versions; write ``karkain.lock``
   * - ``pkg upgrade``
     - Update all packages to the latest compatible version
   * - ``pkg fetch``
     - Resolve and fetch exactly the locked versions
   * - ``pkg deps``
     - List dependencies (``--tree``, ``--outdated``)
   * - ``pkg list``
     - List direct + transitive dependencies
   * - ``pkg tree``
     - Show recursive dependency graph
   * - ``pkg search <query>``
     - Search the package registry
   * - ``pkg info <pkg>``
     - Show package details
   * - ``pkg publish``
     - Publish the current project (``--registry <dir>`` for local)
   * - ``pkg registry init <dir>``
     - Create a local package registry (local-only)
   * - ``pkg login``
     - Authenticate with the registry
   * - ``pkg logout``
     - Clear the auth token
   * - ``pkg whoami``
     - Show the current user
   * - ``pkg audit``
     - Check for vulnerabilities (``--licenses`` license check,
       ``--json`` output)
   * - ``pkg verify``
     - Verify checksums / integrity of all dependencies
   * - ``pkg cache list``
     - Show cached packages
   * - ``pkg cache clean [--stale]``
     - Remove all (or stale) cached packages
   * - ``pkg cache path``
     - Show the cache directory
   * - ``pkg workspace <init|add|remove|list|build|test|check|run|clean|lint|graph>``
     - Workspace management (alias ``pkg ws``)

Sources
=======

Two dependency sources are fully implemented:

- **local** — ``--source local --url <path>`` points at a local project
  directory.
- **git** — ``--source git --url <url>`` fetches a git repository.

Fetching is made safe by atomic ``temp → rename`` writes plus checksum
verification, all stored under the project's ``.karkain/cache/``.

Registry
========

Phase 135 is local-only. No network registry exists yet: the registry is a
directory on the local filesystem, selected by ``--registry <dir>`` (flag
wins) or the ``KARKAIN_REGISTRY`` environment variable.

.. code-block:: console

    $ karkain pkg registry init ./my-registry   # create packages/ + index/
    $ karkain pkg publish --registry ./my-registry
    $ karkain add hello 1.0.0 --registry ./my-registry
    $ karkain fetch                             # resolve, verify, cache
    $ karkain run src/main.kark                 # build against the dep, run

Layout: ``packages/<name>/<version>/`` holds the manifest copy, the
published source tree and a digest; ``index/<name>`` lists the published
versions and the latest. Versions must be semver; resolution accepts an
exact version, ``*``/empty (latest) or an existing semver constraint.
Published versions are immutable — republishing ``name@version`` fails and
leaves the stored contents untouched. Names are validated (letters,
digits, ``-``, ``_``, ``.``); traversal and absolute paths are rejected.
Fetching verifies the stored digest and aborts on mismatch.

Limitations: registry dependencies resolve at their declared version and
are not transitively expanded; ``pkg search``/``pkg info`` against a
remote backend remain unserved (no silent results).

Lockfile
========

Resolved versions are written to ``karkain.lock`` at the project root.

- ``fetch`` uses the lockfile: it retrieves exactly the locked versions
  and never re-resolves.
- ``update`` re-resolves within the manifest constraints and rewrites
  the lockfile.
- ``add`` fetches the new dependency immediately and caches it.

Manifest
========

The project manifest is ``karkain.toml``. Dependencies map a package
name to a version, source flavor and optional URL.

.. code-block:: toml

    [dependencies]
    stdlib = "^0.14.0"

Workspaces
==========

A workspace root is marked with ``karkain pkg workspace init`` and gains
members through ``workspace add <path>``. Member operations run in
dependency order.

Exit codes
==========

- ``0`` — success
- ``5`` — package or dependency failure
- ``2`` — CLI usage error inside the ``pkg`` namespace

.. seealso::

   :doc:`/getting-started/dependencies` — dependency workflow.
   :doc:`/getting-started/project-layout` — directory layout including
   ``karkain.toml`` and the cache directory.