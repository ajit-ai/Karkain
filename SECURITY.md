# Security Policy

Karkain is developed in the open by the QuantsMind community. Reports from
external contributors make the toolchain safer for everyone.

## Supported versions

Only the current Beta release line receives security fixes.

| Version               | Category                     | Supported |
|-----------------------|------------------------------|-----------|
| 0.117.0-beta1 (Beta 1) | Current release candidate line | Supported |
| 0.115.x Developer Preview | Superseded                  | Not supported |

Use the version reported by `karkain --version` when reporting.

## Reporting a vulnerability

Please do **not** open a public issue for a security vulnerability.

The preferred path is GitHub private reporting:

1. Go to https://github.com/ajit-ai/Karkain/security/advisories
2. Select **New draft security advisory**.
3. Fill in the affected version, the vulnerability description, and the
   minimal steps to reproduce.

Private advisories are visible only to the repository maintainers until they
are published, so you can report without exposing details publicly.

If you cannot use GitHub private reporting, contact a maintainer privately
through GitHub (for example, a direct message to an active maintainer listed
on the repository). Do not post exploit details in public issues.

## What to include

To help us reproduce and fix quickly, include:

- `karkain --version` output (exact build identity).
- Operating system and architecture (`karkain target` output).
- The `KARKAIN_ENGINE` value if you set it.
- The command that triggered the issue.
- A minimal, self-contained `.kark` source file (prefer two lines to two
  hundred).
- Expected behavior and actual behavior.
- Any diagnostics emitted by the toolchain (`error[K...]` / runtime errors).

Never include credentials, tokens, or private data in a report.

## Response expectations

We aim to acknowledge receipt within 7 days and publish a fix in the next
Beta/RC release. Security-relevant fixes are documented in the
`docs/source/release-notes.rst` file.