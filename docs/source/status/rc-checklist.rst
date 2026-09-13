===========================
Release-Candidate Checklist
===========================

This is the authoritative acceptance checklist that was used for declaring
Karkain a **Release Candidate** (Phase 118). Every item below was satisfied;
the list is retained as historical evidence of the RC readiness process.

.. note::

   **Completed** — Phase 118 gate ``pkg/cli/phase118_rc_test.go`` passed
   and the RC READY verdict was issued. The 1.0.0 stable release followed
   from Phase 119 QA. See :doc:`/release-notes` and
   :doc:`/status/scope` for the current status.

Current state (Phase 118)
=========================

.. list-table:: Release-candidate checklist
   :widths: 6 64 20 10
   :header-rows: 1

   * - #
     - Criterion
     - Evidence
     - State
   * - 1
     - External developer journey works end to end
     - ``scripts/verify-rc-journey.ps1`` + ``pkg/cli/phase118_rc_test.go``
     - [ ]
   * - 2
     - Installation / build workflow is documented
     - :doc:`/getting-started/installation`, ``scripts/install.ps1``
     - [ ]
   * - 3
     - Version reporting works and identifies the Beta line
     - ``karkain --version`` → ``v1.0.0 (... Stable Build)``
     - [ ]
   * - 4
     - Release artifact strategy is documented
     - :doc:`/development/release`
     - [ ]
   * - 5
     - First-program tutorial works on a clean checkout
     - :doc:`/getting-started/first-program`
     - [ ]
   * - 6
     - Multi-file project example builds
     - :doc:`/getting-started/first-project`, Phase 118 gate
     - [ ]
   * - 7
     - Workspace example works (app consumes sibling library)
     - ``examples/workspace/``, Phase 118 gate
     - [ ]
   * - 8
     - CLI documentation is accurate
     - :doc:`/tools/cli`, ``karkain --help``
     - [ ]
   * - 9
     - Issue-reporting workflow exists
     - ``.github/ISSUE_TEMPLATE/*``, :doc:`/development/reporting-bugs`
     - [ ]
   * - 10
     - Security reporting path exists
     - ``SECURITY.md``
     - [ ]
   * - 11
     - Compatibility policy exists
     - :doc:`/status/compatibility`
     - [ ]
   * - 12
     - Migration notes exist
     - :doc:`/status/migration-beta1`
     - [ ]
   * - 13
     - Beta 1 release notes exist
     - :doc:`/release-notes`
     - [ ]
   * - 14
     - Documentation navigation is complete
     - :doc:`/status/index`, :doc:`/development/index`,
       :doc:`/reference/index`, :doc:`/getting-started/index`
     - [ ]
   * - 15
     - Stable API snapshot exists
     - :doc:`/reference/stable-api`
     - [ ]
   * - 16
     - Feature-freeze policy exists
     - :doc:`/development/feature-freeze`
     - [ ]
   * - 17
     - Automated Phase 118 gate exists and passes
     - ``pkg/cli/phase118_rc_test.go``
     - [ ]
   * - 18
     - Clean / fresh workflow passes
     - ``scripts/beta-fresh-checkout.ps1`` 16/16
     - [ ]
   * - 19
     - Phase 117 regression baseline remains green
     - Conformance 59/59, probes 11/11, example parity 49/49,
       verify-examples 49 pass / 0 fail
     - [ ]
   * - 20
     - No generated artifacts are committed
     - ``git status`` clean of ``*.exe`` / ``*.c`` / ``*.c23`` /
       ``docs/build/`` / ``releases/``
     - [ ]
   * - 21
     - Public status is synchronized everywhere
     - README, docs index, status pages, release notes
     - [ ]

Interpretation
==============

* Every item marked **[ ]** must transition to **[X]** before
  ``RC READY`` can be declared.
* A missing roadmap feature is **not** a checklist failure unless it was
  promised in the Beta stable scope.
* The checklist state is updated in the Phase 118 audit report
  (``docs/audit/PHASE-118-RELEASE-CANDIDATE-READINESS-FINAL-REPORT.md``)
  and mirrored in the :doc:`/status/scope` blockers section.

.. seealso::

   :doc:`/status/scope` — the status matrix that feeds this checklist.
   :doc:`/status/compatibility` — what each bucket guarantees.
   :doc:`/development/feature-freeze` — the freeze rules in force.