============
Installation
============

Karkain is distributed as pre-compiled binaries and built from source.

.. note::

   Karkain is **1.1.0 (Stable)** software. The stable core is regression-gated;
   experimental surfaces may change between releases.

Pre-compiled binaries
=====================

:implemented:`Available` — pre-built binaries for **v1.1.0** have been
built and validated as a **13-archive set**. The v1.1.0 GitHub Release
(tag-triggered CI pipeline) publishes them to `GitHub Releases
<https://github.com/ajit-ai/Karkain/releases>`_ with a ``checksums.txt``
(SHA-256) listing; :ref:`release-status` below records whether the release is
live. Each archive contains the ``karkain`` binary, the standard library,
``README.md``, ``LICENSE`` and a ``VERSION`` file. The repository
install/verify scripts also build from source; see
:ref:`installation-source-build`.

.. _release-status:

Release status
--------------

.. note::

   :planned:`Release cut pending` — once the owner publishes the v1.1.0
   release (owner-only action on the existing CI pipeline), the archive
   links below become live and this note should be removed. Until then the
   archives are not yet downloadable and the
   :ref:`build-from-source <installation-source-build>` path below is the
   way to get the toolchain.

.. note::

   The binaries ship the **self-contained Go engine**. The self-hosted
   ``kcc`` engine (the default engine when present) is compiled from the
   repository's ``src/compiler`` sources, so to use it: run from a clone of
   the repository, or point the ``KARKAIN_KCC`` environment variable at an
   ``src/compiler`` directory. To use the self-contained Go engine instead,
   pass ``--engine go`` to ``karkain`` (or set ``KARKAIN_ENGINE=go``).

Archive naming convention:

.. list-table::
   :widths: 30 30 20 20
   :header-rows: 1

   * - Platform
     - Architecture
     - Archive (v1.0.0)
     - Status
   * - Windows
     - amd64
     - ``karkain-v1.0.0-windows-amd64.zip``
     - :implemented:`Available`
   * - Windows
     - arm64
     - ``karkain-v1.0.0-windows-arm64.zip``
     - :implemented:`Available`
   * - Linux
     - amd64
     - ``karkain-v1.0.0-linux-amd64.tar.gz``
     - :implemented:`Available`
   * - Linux
     - arm64
     - ``karkain-v1.0.0-linux-arm64.tar.gz``
     - :implemented:`Available`
   * - Linux
     - arm (ARMv7)
     - ``karkain-v1.0.0-linux-arm.tar.gz``
     - :implemented:`Available`
   * - Linux
     - 386
     - ``karkain-v1.0.0-linux-386.tar.gz``
     - :implemented:`Available`
   * - Linux
     - ppc64le
     - ``karkain-v1.0.0-linux-ppc64le.tar.gz``
     - :implemented:`Available`
   * - Linux
     - s390x
     - ``karkain-v1.0.0-linux-s390x.tar.gz``
     - :implemented:`Available`
   * - macOS
     - amd64
     - ``karkain-v1.0.0-darwin-amd64.tar.gz``
     - :implemented:`Available`
   * - macOS
     - arm64
     - ``karkain-v1.0.0-darwin-arm64.tar.gz``
     - :implemented:`Available`
   * - FreeBSD
     - amd64
     - ``karkain-v1.0.0-freebsd-amd64.tar.gz``
     - :implemented:`Available`
   * - NetBSD
     - amd64
     - ``karkain-v1.0.0-netbsd-amd64.tar.gz``
     - :implemented:`Available`
   * - OpenBSD
     - amd64
     - ``karkain-v1.0.0-openbsd-amd64.tar.gz``
     - :implemented:`Available`
   * - Docker
     - (multi-arch)
     - ``ghcr.io/ajit-ai/karkain:latest``
     - :planned:`Planned`

Windows
-------

.. code-block:: powershell

   # Once the v1.0.0 GitHub Release is published, download the archive and:
   Expand-Archive -Path .\karkain-v1.0.0-windows-amd64.zip -DestinationPath $env:LOCALAPPDATA\Karkain

   # Add to PATH (PowerShell)
   $env:PATH += ";$env:LOCALAPPDATA\Karkain\karkain-v1.0.0-windows-amd64"

   # Verify
   karkain --version

Linux
-----

.. code-block:: bash

   tar xzf karkain-v1.0.0-linux-amd64.tar.gz
   sudo mv karkain-v1.0.0-linux-amd64/karkain /usr/local/bin/
   karkain --version

macOS
-----

.. code-block:: bash

   tar xzf karkain-v1.0.0-darwin-amd64.tar.gz
   sudo mv karkain-v1.0.0-darwin-amd64/karkain /usr/local/bin/
   karkain --version

Verifying the download
----------------------

Every archive is listed in the release's ``checksums.txt`` file (SHA-256).
Always verify a downloaded archive against it before running:

.. code-block:: bash

   # Linux / macOS: compare the published checksum with the local file
   sha256sum -c checksums.txt --ignore-missing   # (checks every listed file)

   # Or check one archive directly:
   grep "karkain-v1.0.0-linux-amd64.tar.gz" checksums.txt
   echo "<published-sha256>  karkain-v1.0.0-linux-amd64.tar.gz" | sha256sum -c -

Windows (PowerShell):

.. code-block:: powershell

   Get-FileHash .\karkain-v1.0.0-windows-amd64.zip -Algorithm SHA256

   # Compare the output hash with the published value in checksums.txt; a
   # matching SHA-256 means the archive is intact and authentic.

Docker
------

.. note::

   :planned:`Planned` — the container image is not published yet; use the
   pre-compiled binary or :ref:`build from source <installation-source-build>`.

.. code-block:: bash

   docker run -it ghcr.io/ajit-ai/karkain:latest karkain --version

.. _installation-source-build:

Building from source
====================

Requirements:

- Go 1.21+
- GCC or Clang (for the C transpilation backend)
- Git

.. code-block:: bash

   git clone https://github.com/ajit-ai/Karkain.git
   cd Karkain
   go build ./cmd/karkain
   ./karkain --version

On Windows with PowerShell:

.. code-block:: powershell

   go build ./cmd/karkain
   .\karkain.exe --version

Verified install script
=======================

The repository ships deterministic install scripts that build, install and
smoke-test the toolchain with explicit exit codes:

.. code-block:: powershell

   powershell -ExecutionPolicy Bypass -File scripts\install.ps1
   powershell -ExecutionPolicy Bypass -File scripts\verify-install.ps1

   # Linux / macOS (bash)
   ./scripts/install.sh

The scripts verify prerequisites (Go 1.21+ and a C compiler), build
``karkain`` into the prefix (``$LOCALAPPDATA\Karkain`` on Windows,
``$HOME/.local/karkain`` elsewhere), check ``karkain --help`` and compile +
run ``hello.kark``. Exit codes are deterministic: ``0`` success, ``1``
missing prerequisite, ``2`` build failure, ``3`` installation smoke-test
failure.

.. _installation-minimum-environment:

Minimum practical development environment
=========================================

The compiler itself is lightweight, but two parts of the ecosystem have real
resource needs:

* **Building the full test suite** — running every ``pkg/cli`` gate
  concurrently can exhaust machines with very limited RAM. On memory-constrained
  hosts (about 4 GB or less of usable RAM), run the CLI gates
  **sequentially or batched by phase** (e.g. ``go test ./pkg/cli/ -run
  'TestPhase11[45678]'``), not ``go test ./pkg/cli/ -count=1`` all at once.
  This is a documented environmental behavior, not a toolchain defect.
* **Full ``kcc`` *build* mode** — transpiling the compiler's own sources with
  ``kcc build`` can stall on very limited hosts; the low-memory ``kcc check``
  path works everywhere. See :doc:`/status/beta`.

The recommended practical setup is a machine with at least 4 GB of RAM and a
recent Go toolchain; single-program ``check``/``build``/``run`` work fine
well below that.

Runtime requirements
====================

The Karkain toolchain requires:

- A C compiler (GCC, Clang, or MSVC) on the host platform
- Git (for the package manager's dependency fetching)
- Go (for building Karkain from source or running the Go engine)

Optional:

- `wasmtime <https://wasmtime.dev/>`_ — required only for
  :doc:`wasm32-wasi </targets/target-triples>` targets
- GMP — optional big-integer backend (not required for standard usage)