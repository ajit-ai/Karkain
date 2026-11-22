============================================
Upgrading from Karkain v1.1.0 to v1.2.0
============================================

This guide is for developers moving an existing Karkain project from v1.1.0
to v1.2.0 (release tag ``v1.2.0``). It states plainly where **no source change
is required**, where a change is *recommended*, where a change is *required*,
and where only the environment/build configuration changes. It does not invent
migration steps for behaviour that did not change.

Summary
=======

For the overwhelming majority of v1.1.0 projects, **no source change is
required**. v1.2.0 is an additive release: every v1.1.0 program builds and
runs identically. The one observable change is that the three C-free native
targets are now emitted by the self-hosted compiler rather than the Go
backend. See :doc:`V1.1-TO-V1.2-CHANGELOG` for the technical detail.

1. Version identification
=========================

No source change required.

``karkain --version`` now reports ``Karkain Compiler v1.2.0``. Update any CI
assertions, scripts or documentation that pin the old banner text::

   Karkain Compiler v1.2.0 (<arch>, Stable Build)

The public label moved from **1.1.0 (Stable)** to **1.2.0 (Stable) — Native
Independence**. The root ``VERSION`` file is ``1.2.0``.

2. Installation and compiler selection
=======================================

Environment/build change only.

The release still ships the same 13 archives (Windows amd64/arm64; Linux
amd64/arm64/arm/386/ppc64le/s390x; macOS amd64/arm64; FreeBSD/NetBSD/OpenBSD
amd64) plus ``checksums.txt``. Verify a downloaded archive against the
published checksums before use. See :doc:`/getting-started/installation`.

Go remains the reference toolchain and is still required to **build the
compiler from source**. For the C-free native targets it is no longer required
to **compile a user program** — that is the "Off Go" scope of this release.

3. CLI changes
==============

No source change required.

The CLI surface is unchanged. One corrective behaviour worth knowing: under
the default (kcc) engine, ``karkain run`` streams the program's stdout
verbatim — compiler build messages no longer appear on the program's stdout
and the program's own line endings are preserved; the build banner is printed
to stderr only under ``--verbose``. If a downstream script previously depended
on the compiler banner being mixed into stdout, adjust it.

4. Build command changes
========================

Recommended change (only for C-free native builds).

You can now build native machine-code images directly::

   karkain build --target native-x86_64-linux   main.kark
   karkain build --target native-x86_64-windows main.kark
   karkain build --target native-x86_64-macos   main.kark

These previously fell through to the Go backend or were refused; they are now
emitted by the self-hosted compiler with no C compiler and no Go toolchain in
the program-compilation path. ``--target c23`` is unchanged. ``--incremental``
is refused for native builds with an explanatory message (a whole-image cache
cannot repay a ~11–53 µs emission; a per-function split needs a real
relocation model).

5. Target selection
===================

No source change required.

All three C-free targets are owned by ``kcc``; none refuses with
``error[K116]``. Foreign C targets still fail deterministically when a
cross-linker is missing — this is unchanged. See :doc:`/targets/host-targets`
for the per-target ownership and verification status.

6. Native compilation changes
==============================

Environment change if you adopt native targets.

The native path supports a **qualified subset**. If your program uses any of
the following, the native driver refuses it by name with a diagnostic and
writes no image (it is never silently mis-compiled):

* float64 statements, parameters or returns;
* ``for ... in``;
* map literals;
* string equality or string slicing;
* a string literal inside a branch or loop, or a string in a callee.

Use the C23/C backend for programs that need these; it remains fully
supported. The native feature matrix is
``docs/inventory/native-feature-matrix.json``.

7. Source compatibility
=======================

No source change required.

The language core — variables, functions, recursion, control flow, arrays,
maps, structs, enums, ``match``, strings and modules with ``public`` exports —
is unchanged. Diagnostics and exit codes are unchanged.

8. Standard-library compatibility
==================================

No source change required.

The frozen v1.2.0 module surface (``std.string``, ``std.collections``,
``std.io``, ``std.encoding``, ``std.crypto``, ``std.testing``,
``std.numerics``, ``std.net``, ``std.http``, ``std.db`` and ``std.generics``)
is importable on both engines under the same names. See :doc:`/stdlib/index`.

Note the scope boundary: **running the standard library with no C compiler is
not a v1.2.0 closure condition**. The no-C ``stdlib_v2`` and no-C ``std.net``
proofs were carried to v1.3.0 as UNMET by owner-authorized amendment. Do not
assume an arbitrary stdlib program builds as a native image at this tag.

9. Build and deployment considerations
=======================================

No source change required; review your release provenance expectations.

v1.2.0 is a **locally built and published release, NOT a CI-green release**.
The tag-triggered workflow did not execute its Build matrix because the
account was subject to a billing/payment restriction. Archive contents are
identical to what the CI steps would produce, but the published checksums
identify the locally built archives. Record this in any downstream provenance
documentation.

10. Cross-platform considerations
==================================

No source change required; adjust verification expectations per target.

* **Linux ELF** — emitted, structurally validated, byte-identical to the Go
  oracle for the gated shapes, and executed on the Linux CI leg.
* **Windows PE** — emitted, structurally validated and executed on the dev
  host; **not** byte-identical to the Go oracle (the entry stub and PEB
  bootstrap are separate implementations).
* **macOS Mach-O** — emitted as a real PIE and structurally validated, but
  **never executed** (no macOS runner exists in this repository).

11. Migration checklist
=======================

- [ ] Confirm ``karkain --version`` reports ``v1.2.0`` (update CI pin assertions).
- [ ] Re-verify downloaded archives against the published ``checksums.txt``.
- [ ] If using native targets, confirm your program is inside the qualified
      subset (section 6); otherwise keep the C23 backend.
- [ ] If a script consumed ``karkain run`` stdout, confirm it still works with
      the verbatim-stdout behaviour (section 3).
- [ ] Record the release's local-build provenance in your own release notes.
- [ ] Do not treat Mach-O execution or full self-bootstrap as verified.

12. Troubleshooting
===================

``error[K116]`` on a native target
   This release owns all three C-free targets, so a genuine ``K116`` means the
   command predates the release or is using an older binary.

A native program is refused by name
   The construct is outside the qualified subset (section 6). Use the C23
   target, or restructure the program.

The native image builds but prints nothing on stderr for a bounds error
   The Phase-152-B0 ``GetStdHandle(STD_ERROR_HANDLE)`` limitation leaves the
   diagnostic stream empty on the dev host; the non-zero exit status is the
   observable. The diagnostic path is asserted structurally.

13. Known incompatibilities
===========================

The only behavioural change from v1.1.0 is the engine that emits native
targets. There are no source-level breaking changes. The documented native
byte-layout differences (reporter rodata interning in one fixture, the 25-byte
field-assignment difference, and the opt-in register allocator) do not change
program output.

14. Rollback considerations
===========================

v1.1.0 remains available as a historical release (tag ``v1.1.0``). Programs
and toolchains pinned to it continue to work unchanged; rolling back does not
require source changes. If a site depends on the old mixed-stdout behaviour of
``karkain run``, that behaviour is the only CLI-level difference to account
for.
