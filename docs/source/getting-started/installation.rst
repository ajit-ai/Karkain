============
Installation
============

Karkain is distributed as pre-compiled binaries and built from source.

.. note::

   Karkain is **Beta 1** software. The Beta stable core is regression-gated;
   experimental surfaces may change between releases.

Pre-compiled binaries
=====================

:planned:`Planned` — pre-built binaries are not published yet. The
planned distribution channels are GitHub Releases and a container image,
with archives/image names following the conventions listed below once a
first release exists. Today, Karkain must be :ref:`built from source
<installation-source-build>`.

The planned archive naming convention (to be confirmed at first release):

.. list-table::
   :widths: 30 30 20 20
   :header-rows: 1

   * - Platform
     - Architecture
     - Archive (planned)
     - Status
   * - Windows
     - amd64
     - ``karkain-v<ver>-windows-amd64.zip``
     - Planned
   * - Windows
     - arm64
     - ``karkain-v<ver>-windows-arm64.zip``
     - Planned
   * - Linux
     - amd64
     - ``karkain-v<ver>-linux-amd64.tar.gz``
     - Planned
   * - Linux
     - arm64
     - ``karkain-v<ver>-linux-arm64.tar.gz``
     - Planned
   * - Linux
     - armv7
     - ``karkain-v<ver>-linux-armv7.tar.gz``
     - Planned
   * - Linux
     - i386
     - ``karkain-v<ver>-linux-i386.tar.gz``
     - Planned
   * - Linux
     - ppc64le
     - ``karkain-v<ver>-linux-ppc64le.tar.gz``
     - Planned
   * - Linux
     - s390x
     - ``karkain-v<ver>-linux-s390x.tar.gz``
     - Planned
   * - macOS
     - amd64
     - ``karkain-v<ver>-darwin-amd64.tar.gz``
     - Planned
   * - macOS
     - arm64
     - ``karkain-v<ver>-darwin-arm64.tar.gz``
     - Planned
   * - FreeBSD
     - amd64
     - ``karkain-v<ver>-freebsd-amd64.tar.gz``
     - Planned
   * - Docker
     - (multi-arch)
     - ``ghcr.io/ajit-ai/karkain:latest``
     - Planned

Windows
-------

.. note::

   Not yet available — see :ref:`installation-source-build`. The commands
   below show the intended flow once binaries ship.

.. code-block:: powershell

   # Download and extract (replace <ver>)
   Expand-Archive -Path .\karkain-v<ver>-windows-amd64.zip -DestinationPath $env:LOCALAPPDATA\Karkain

   # Add to PATH (PowerShell)
   $env:PATH += ";$env:LOCALAPPDATA\Karkain"

   # Verify
   karkain --version

Linux
-----

.. note::

   Not yet available — see :ref:`installation-source-build`.

.. code-block:: bash

   tar xzf karkain-v<ver>-linux-amd64.tar.gz
   sudo mv karkain /usr/local/bin/
   karkain --version

macOS
-----

.. note::

   Not yet available — see :ref:`installation-source-build`.

.. code-block:: bash

   tar xzf karkain-v<ver>-darwin-amd64.tar.gz
   sudo mv karkain /usr/local/bin/
   karkain --version

Docker
------

.. note::

   Not yet available — see :ref:`installation-source-build`.

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