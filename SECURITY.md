# Security Policy

Karkain is developed in the open by its contributors. Reports from
external contributors make the toolchain safer for everyone.

## Supported versions

Only the current Stable release line receives security fixes.

| Version               | Category                     | Supported |
|-----------------------|------------------------------|-----------|
| 1.2.0 (Stable) | Current release line | Supported |
| 1.1.0 | Previous Stable line | Supported (security fixes) |
| 1.0.x (LTS) | Maintenance line (security fixes) | Supported |
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

### Response SLA (v1.2.33)

| Stage | Target |
|---|---|
| Acknowledgement | 7 calendar days |
| Severity assignment and initial triage | 14 calendar days |
| Fix for Critical / High | 30 calendar days |
| Fix for Medium / Low | next Stable release, 90-day target |
| Public disclosure | after a fix ships, coordinated with the reporter |

Targets are commitments for triage and fix work, not a claim that every
report will be exploitable. A report that does not reproduce is closed with
the evidence and the reporter is told why. Security-relevant fixes are
documented in `docs/source/release-notes.rst`.

## Supply-chain evidence

### Software bill of materials

Each release directory can be described by a deterministic SBOM generated
from the release artifacts and their SHA-256 digests:

```
karkain sbom --release-dir releases --version v1.2.0 --output releases/sbom.json
```

The document uses the `karkain-sbom-v1` schema. It contains no timestamp, so
regenerating it over an unchanged release directory produces identical bytes.
`go.mod` declares no external Go modules, so the `external_go_modules` list is
empty by construction rather than by assumption. Gate: `TestPhaseV1233_`.

### Artifact signing

**Not implemented.** Release archives are accompanied by SHA-256 digests, but
they are not signed. `cosign` signing is planned in increment 165 and is not
started. Do not describe a release as signed or verified by signature today;
verify by SHA-256 instead.