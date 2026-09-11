============
Dependencies
============

Karkain manages dependencies through a project manifest and a deterministic
lockfile.

Commands
========

.. code-block:: console

    $ karkain init                       # create a project and karkain.toml
    $ karkain pkg add mylib "1.0.0"      # add a dependency
    $ karkain pkg add utils --source git --url https://github.com/bob/utils.git
    $ karkain pkg add utils --source local --url ../utils
    $ karkain pkg update                 # re-resolve, write karkain.lock
    $ karkain pkg list                   # direct + transitive dependencies
    $ karkain pkg tree                   # recursive dependency graph
    $ karkain pkg fetch                  # fetch the locked versions

``karkain init`` and ``karkain new`` create a fresh project with
``karkain.toml``, ``src/main.kark`` and ``tests/main_test.kark``.

Manifest
========

``karkain.toml`` holds the project identity and its dependency requests:

.. code-block:: toml

    name = "my_project"
    version = "0.1.0"
    description = "A Karkain project"
    license = "MIT"
    targets = ["native"]

    [dependencies]
    mylib = "1.0.0"                          # shorthand: registry source
    utils = { version = "0.2.0", source = "git", url = "https://github.com/bob/utils" }
    helpers = { version = "0.1.0", source = "local", url = "../helpers" }

A dependency entry is either a bare version string (``source = "registry"``
implied) or an inline table with ``version``, ``source`` (``registry``,
``git`` or ``local``) and ``url``.

Resolution
==========

Dependencies resolve deterministically:

- **Local** — ``source = "local"`` deps are read from sibling directories
  resolving to a project with its own ``karkain.toml``.
- **Git** — ``source = "git"`` deps are fetched from a repository URL at a
  pinned revision.
- **Registry** — registry lookup is **not yet available**; a registry source
  fails with an explicit error rather than being faked.

The deterministic resolver writes **``karkain.lock``** — the resolved,
versioned graph with checksums. ``karkain pkg fetch`` retrieves exactly the
locked versions; ``update`` re-resolves within the manifest constraints and
rewrites the lockfile.

Status
======

Local resolution and git fetching are implemented and tested. The package
registry (search / publish / login) is an explicit "not available" boundary —
no silent simulation.

.. seealso::

   :doc:`project-layout` — the manifest in its project context.
   :doc:`modules` — importing the modules you depend on.