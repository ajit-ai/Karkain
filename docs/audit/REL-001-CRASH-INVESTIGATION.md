# REL-001 crash investigation: kcc `0xC0000005` on whole-tree `kir --verify`

**Date:** 2026-10-15
**Authorization:** REL-001 diagnostic build and crash discovery only.
**Outcome: CRASH NOT REPRODUCED. REL-001 remains UNCLASSIFIED.**

No root cause is claimed. Five consecutive clean runs are evidence that the
trigger is not currently firing, not evidence of any cause.

## 1. Baseline

| Property | Value |
|----------|-------|
| Branch | `develop` |
| HEAD | `80a945535ab69fba7950b9b3faa339febf5e6769` |
| Working tree | CLEAN |
| `main` / `origin/main` / `origin/develop` | `f66863d` / `f66863d` / `989a04c` |
| Stash | `stash@{0}` intact, never applied or deleted |
| Tags | 5, unchanged |
| Last change to `src/compiler/` | `cc44c95` (D5) |
| Host | windows/amd64, ~4 GB, `GOMEMLIMIT=3GiB` |

## 2. Toolchain and diagnostic method

| Item | Finding |
|------|---------|
| Compiler | `gcc.exe (Rev2, Built by MSYS2 project) 14.2.0` |
| Target | `x86_64-w64-mingw32` |
| Linker | `GNU ld (GNU Binutils) 2.43.1` |
| **AddressSanitizer** | **NOT AVAILABLE.** Probe: `gcc -fsanitize=address` fails with `cannot find -lasan`. MSYS2's UCRT64 gcc ships no `libasan`, so ASan was rejected on evidence rather than assumed |
| Stack diagnostic | `-Wl,--stack,N` accepted by this `ld` (probe: exit 0, valid PE produced). Confirmed for the active toolchain rather than assumed from GNU documentation |
| Isolation method | `KARKAIN_KCC` environment variable overrides the kcc binary path in `kccBinaryPath` (`pkg/cli/kcc_engine.go`). Both diagnostic binaries were built **outside the repository**, in `%TEMP%\kcc-rel001`, so the repo's `kcc.exe` and its build path were never touched |

Build recipe reproduced from `buildKCC` (`pkg/cli/kcc_engine.go`), which links
with `-std=c99 -x c -D_POSIX_C_SOURCE=200809L main.c -lgmp -lm -lws2_32`:

```text
# stage 1 (Go front end, gitignored output)
cd src/compiler && KARKAIN_ENGINE=go karkain build main.kark
#        -> src/compiler/main.c, 2,219,798 bytes, exit 0, 12 s

# control arm (repo flags exactly, no diagnostic flag)
gcc -std=c99 -x c -D_POSIX_C_SOURCE=200809L main.c -o kcc-control.exe -lgmp -lm -lws2_32
#        -> 3,503,487 bytes, exit 0, 13 s

# diagnostic arm (identical, plus a 64 MiB stack reserve)
gcc ... -o kcc-stack64m.exe -lgmp -lm -lws2_32 -Wl,--stack,67108864
#        -> 3,503,487 bytes, exit 0, 9 s
```

## 3. Instrumentation verified

Both artifacts were checked by reading the PE optional header directly, because
identical file sizes prove nothing: `--stack` alters header fields, not code.
`SizeOfStackReserve` sits at optional-header offset 72 in PE32+.

| Artifact | PE magic | `SizeOfStackReserve` |
|----------|----------|----------------------|
| `kcc-control.exe` | `0x020B` (PE32+) | 2,097,152 (2 MiB - MinGW default) |
| `kcc-stack64m.exe` | `0x020B` (PE32+) | **67,108,864 (64 MiB)** |

**VERDICT: CONFIRMED.** The stack reserve differs by the requested amount, so
the diagnostic option took effect. Identical file size between the two binaries
is expected and consistent with a header-only change.

## 4. Discovery runs

Command in every run: `go run ./cmd/karkain kir --verify src/compiler/kir.kark`,
run sequentially with no concurrent long-running jobs.

| # | Binary | Stack reserve | Exit | Elapsed | Free RAM before | Output |
|---|--------|---------------|------|---------|-----------------|--------|
| DIAG-1 | `kcc-stack64m.exe` | 64 MiB | 0 | 466 s | 814 MB | `[ok] kir verify: 14050 lines ok` |
| DIAG-2 | `kcc-stack64m.exe` | 64 MiB | 0 | 608 s | 808 MB | same |
| DIAG-3 | `kcc-stack64m.exe` | 64 MiB | 0 | 473 s | 597 MB | same |
| CONTROL | `kcc-control.exe` | 2 MiB | 0 | 487 s | 596 MB | same |

No crash, no non-zero exit, no truncated output, no dump. Output length was
61 bytes on every run, so nothing was silently truncated.

## 5. Comparison with previously reported clean runs

| Run | Binary | Exit | Elapsed |
|-----|--------|------|---------|
| Earlier: `TestPhase151A5_KIRPinHolds` | repo `kcc.exe` | 0 | 908.3 s |
| Earlier: direct CLI | repo `kcc.exe` | 0 | 996 s |
| DIAG-1/2/3 + CONTROL | isolated builds | 0 | 466 / 608 / 473 / 487 s |

Five consecutive clean runs total, four of them this session, across three
different binaries and two stack configurations.

The isolated builds ran roughly **half** the elapsed time of the repo-binary
runs. The cause was not investigated and is **not** attributed here; plausible
candidates include compiler version, filesystem caching or ambient load. It is
recorded because an unexplained 2x timing difference is itself an observation
someone should not have to rediscover.

## 6. Interpretation - stated conservatively

**What the evidence supports:** the crash did not occur in five attempts, at
this commit, on this host, under 596-996 MB free RAM, with either a 2 MiB or a
64 MiB stack reserve.

**What the evidence does NOT support:**

- **No root cause.** Four clean runs on a 64 MiB stack say nothing about
  whether stack size is involved, because a run that does not crash cannot
  discriminate between hypotheses.
- **Not evidence that stack size is irrelevant.** The earlier record already
  notes that a 64 MiB reserve failed to stop a crash when one occurred; that
  experiment is not contradicted, it simply did not recur today.
- **Not a fix.** No code changed.

The previously falsified hypotheses stand unchanged: the low-RAM class
(inconsistent with succeeding at 127 MiB and failing at 818 MiB - this session
adds three more successes at 597-814 MB, the same spread), stack exhaustion at a
64 MiB reserve, and a corrupt or partially-written binary. The corrupt-binary
hypothesis is now weaker still: three independently linked binaries in an
isolated directory all behaved identically.

## 7. Incidental observation, not a finding

The stage-1 compile emitted two `-W` diagnostics that are recorded because they
concern code this investigation touched, and deliberately **not** treated as a
crash cause:

- `main.c:49646` and `main.c:50547`: "integer constant is so large that it is
  unsigned" on `make_int(-9223372036854775808)`, inside
  `karkain_user_natPrintFloatHelper` and `karkain_user_natFloatNeg`. These are
  the `1<<63` sign-mask sites recorded in the 151A Step 8b history. In C the
  literal is parsed as unsigned and negated modularly, which yields the intended
  two's-complement bit pattern, so this is very likely benign - but "very likely"
  is not verified, and it is recorded as an open question rather than a
  conclusion.
- `-Wpsabi`: "AVX vector return without AVX enabled changes the ABI" in
  `karkain_simd_splat_f32x8`. Relevant to the SIMD dimension, unrelated to the
  crash.

## 8. Git changes and artifacts

| Item | State |
|------|-------|
| Commits created | **none by the investigation itself** |
| Repo files modified | **none** - working tree CLEAN, identical to baseline |
| Diagnostic binaries | built in `%TEMP%\kcc-rel001`, **deleted after use**; never added to the repository |
| `src/compiler/main.c` | regenerated stage-1 artifact, gitignored (`.gitignore:74`), never staged or committed |
| Repository `kcc.exe` | never rebuilt, never replaced |
| Push / merge / tag / release | **none** |

The only repository change from this task is this document plus the REL-001
row in the issue register, both committed together as an evidence record.

## 9. Updated REL-001 status and recommended next work

**Status: `blocked` - UNCLASSIFIED.** Unchanged in substance, strengthened in
evidence: five clean runs recorded, ASan proven unavailable on this toolchain,
and the stack diagnostic proven effective and exercised.

Recommended next work, in order:

1. **Capture a dump when it next occurs.** This is the single highest-value
   action and needs no code change: enable WER LocalDumps for `kcc.exe`
   (FullDump) and leave it in place. Every hypothesis below needs a dump and no
   hypothesis can advance without one.
2. **Re-run the three falsified hypotheses only against a captured dump**, not
   against a fresh guess.
3. **Do not build ASan on this toolchain.** It is unavailable; the realistic
   alternatives are a dump, a debugger attach, or moving the investigation to a
   toolchain that ships `libasan` (an owner decision, since it changes the
   build).
4. **Re-attempt reproduction later.** The trigger is not firing at this commit;
   if it fires again on a different commit, that diff is itself evidence.

No implementation follows from this investigation. REL-001 stays open.