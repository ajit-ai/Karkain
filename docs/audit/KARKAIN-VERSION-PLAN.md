# Karkain Version Plan — 1.1.0 → 2.0.0

**Status**: authoritative forward view (planning + release ledger).
**Baseline**: increment **149** complete, `VERSION` = `1.1.0`, tree clean on
`main` @ `624323e`, CI gates Phase 114–148 + `pkg/native` + docs green.
**Companion documents** (this file does not replace them):

| Document | Role |
|---|---|
| `AGENTS.md` | completion record + branch/cadence rules |
| `docs/audit/GENERAL-AVAILABILITY-ROADMAP.md` | GA-1/2/3 milestones (phases 128–142) |
| `docs/audit/KARKAIN-INDEPENDENCE-ROADMAP.md` | I / S / L / P / D track definitions + per-module stdlib backlog |
| `docs/source/development/semver-policy.rst` | what a patch / minor / major may contain |
| `docs/source/status/scope.rst`, `docs/source/reference/stable-api.rst` | the public status + API ledger |
| `ROADMAP.md` | historical phase log (Phases 1–49) |

---

## 1. Cadence: version-planned, increment-executed

Work is **planned by version** and **executed by increment**. An increment *is*
what previous phases were: one deliverable, one gate, one record, one merge.

**Per increment (every time)**

1. Implement the slice (A/B/C/D as needed) + its gate file.
2. `go build ./...`; `go vet` on touched packages; run the new gate in
   isolation plus the regressions it can break (the ~4 GB host rule: heavy
   gates are never combined in one command).
3. Update the audit note / `AGENTS.md` record with the version tag
   (`Increment NNN (vX.Y.Z)`).
4. `git commit` on `develop` → merge `main` → push. The push runs the full CI
   matrix (`.github/workflows/ci.yml`, `on: push`) and republishes GitHub
   Pages (`.github/workflows/docs.yml`, push to `main`).

**Per version close (one go)**

1. `docs/release/vX.Y.Z-CHECKLIST.md` written **when the version opens**
   (freezes scope).
2. Full QA battery: units, Phase-114 corpus (both engines), conformance,
   probes, every gate file, Sphinx `-W` + linkcheck, `verify-examples`,
   `install`/`verify-install`, `rc-journey`.
3. Version identity: `VERSION`, Go banners (×2), kcc banners (×3), generated
   headers, LSP, and every version-pinning test.
4. Docs flip: `roadmap.rst`, `compatibility.rst`, `scope.rst`,
   `stable-api.rst`, `installation.rst`, release notes.
5. Final `develop` → `main` merge + push.
6. **Tag `vX.Y.Z`** (owner-only) → CI `release` job publishes the 13 archives
   + `checksums.txt` → GitHub Release → Pages → post-publish verification
   against the *shipped* binary. Previous line gets/keeps its LTS branch.

**No long-lived version branch, ever.** `main` stays shippable at all times;
that property is why the 1.0/1.1 lines shipped at all.

## 2. Rules

1. One increment = one gate file = one audit note = one merge/push.
2. A version closes only when its exit-criteria table is 100 % green **and**
   the ceremony ran. "Green except X" is not closed; X moves to the next
   version or to an explicit carry-over list.
3. **Scope freeze**: after the version opens, new ideas go to the next
   version. No silent additions.
4. SemVer class is decided at open: additive-only ⇒ MINOR; anything that
   removes/renames stable syntax, changes a stable signature or the shape of
   stable diagnostics ⇒ the next MAJOR (`2.0.0` is already earmarked for the
   C-layout struct change).
5. `scope.rst` / `stable-api.rst` / NFR ledger rows update **in the same
   commit** as the increment that moves them (the L-6 standing rule).

## 3. Version table

| Version | Theme | Increments | Exit criteria (beyond "all gates green") |
|---|---|---|---|
| **1.1.0** | performance, parity, generics v1, LSP v2, local registry, native prep | 128–149 ✅ in-tree | only the owner ceremony: tag `v1.1.0` on current `main`, CI archives + checksums, `verify-install`/`rc-journey` against the shipped binary, docs marker flip |
| **1.2.0** | **Sovereignty I — off C, off Go** | 150, 151, 152, 154, 165 | hello + `stdlib_v2` + one `std.net` program build/run with `gcc`/`clang`/`cl` **absent from `PATH`**; `stage2 == stage3` with **no Go toolchain**; native images byte-identical Go↔kcc; Go described as *reference only* in `installation.rst` |
| **1.3.0** | language + library completeness | 155, 156, 158, 159, 160, 161 | OS stdlib modules ship (both engines); PM/scaffolding owned by kcc; generics v2 + `const` folding; linear types enforced; atomic surface with litmus goldens; **zero** Go-engine-only core rows |
| **1.4.0** | tooling, debugging, observability | 153, 157, 163, 164, 175, 176 | DWARF consumed (breakpoints/step); `prof`/SIMD/DWARF parity on kcc; KIR consumed by the backend; every known wrong-code class fixed or a loud `K1xx` |
| **1.5.0** | platforms & packaging | 166, 167, 168, 169, 170, 171, 172 | `--target freestanding` links with no libc; WASM GC/components + `wit`; MSI/DEB/RPM/DMG artifacts install in clean environments; riscv64 real; Android triple |
| **1.6.0** | performance, memory, registry | 162, 174, 179* | full self-build ≤ 2.5 GB; one measured SSA→emit pass with a published bench delta; CLI startup budget; public registry **only if operated** (*otherwise it stays Planned and is not claimed*) |
| **2.0.0** | layout & hardware (breaking line) | 177, 178, 180 | C-layout structs + `packed`/alignment with goldens re-pinned as the reviewed diff; `volatile`/MMIO/`asm()` with K108-style rejects; deprecation removals + migration guide |
| post-2.0 | Apple distribution, TLS/AEAD, async/await, JIT | 181, 182, 183, 184 | each needs a real runner, a costed dependency decision, or a measured win before it is claimed |

## 4. Increment queue — 1.2.0 "Sovereignty I" (open)

Order is dependency-forced: `150 → 151 → 152 → 154`, with `165` free-floating
(CI-only lane, no compiler risk).

| Increment | Track | Deliverable | Gate / Definition of Done | Boundaries (not this increment) |
|---|---|---|---|---|
| **150** | I (C-front) | Native **Value model** (boxed `Value`: arrays, `for-in`, floats, maps, structs, string ops) + register allocation + Mach-O PIE/rebase + native-split incremental cache + **`--target native-x86_64-windows` / `native-x86_64-macos`** CLI targets (listing, `--help`, per-OS build/run matrix, run only on matching hosts else exit 6) | `pkg/native` executed goldens on windows/amd64 (PE) and linux/amd64 (ELF), structural Mach-O everywhere; new `pkg/cli/phase150_native_targets_test.go` (magic per OS, run refusals, listing, incremental refusal); ELF byte-identity differential vs the 147/148/149 corpus | arm64 native; PE delay-load/TLS/SEH/resources/signing; Mach-O **execution** (no Intel-mac runner); kcc native parity (151) |
| **151** | I | **kcc native parity** — the self-hosted engine selects and emits the same native targets | One corpus file per native feature byte-identical Go↔kcc; 148/150 gates re-run with the kcc engine un-pinned | new native features (that is 150's contract) |
| **152** | I | **Native stdlib + no-C closure** (completes I-5): `hello`, `stdlib_v2` and one `std.net` program link and run with no C compiler on `PATH` | Link-without-compiler gate; `--target c23` still available and unchanged; deterministic rejects for net/db/http on a runtime without OS sockets | removing the C path (it stays, as an option) |
| **154** | I | **I-6 self-bootstrap** — `kcc` builds `kcc`; retire the `KARKAIN_ENGINE=go` stage-1 pin; Go tree archived as `reference/` differential oracle | `stage2 == stage3` (SHA) with the Go toolchain absent from the build host; full QA battery green in that state | deleting the Go tree (kept as oracle) |
| **165** | P | **P-0 Docker image**: slim base + gcc + wasmtime, multi-arch `ghcr.io` push on tag, cosign | `docker run ghcr.io/ajit-ai/karkain:<tag> karkain --version` + hello `run` inside the image on amd64 and arm64 | MSI/DEB/RPM/DMG (1.5.0) |

**Owner items for 1.2.0** (cannot be done from the repo): the `v1.1.0` tag cut
that closes the previous line, container-registry visibility for `ghcr.io`,
and (later, for P-1/P-3) code-signing identities.

## 5. Non-functional requirement catalogue

Every increment names the NFRs it moves. "Measured" means a value exists in an
audit report; "target" is what would close it.

| ID | Requirement | Measured now | Target | Owner increment |
|---|---|---|---|---|
| NFR-1 | Engine determinism (byte-identical Go↔kcc) | 61 corpus goldens + 64 conformance tests byte-identical | zero divergence on every Stable row, incl. native | 151 |
| NFR-2 | Reproducible builds | stage-1 byte-reproducible (SHA `0c01de43f796b153`) across CWDs; tamper divergence detected | same for native images and every packaged artifact | 152, 165, 166–168 |
| NFR-3 | Memory footprint (4 GB host) | kcc check 544.5 MB · `go build` 229.9 MB · gcc big-TU 484 MB · guard threshold 1536 MiB | full self-build ≤ 2.5 GB; `error[K127]` instead of any SEGFAULT | 162 |
| NFR-4 | Compile latency | incremental no-op **79 ms** vs clean 2790 ms | no-op ≤ 100 ms sustained; native-split parity | 150 |
| NFR-5 | Runtime performance | AVX2/SIMD kernels exist; the SSA optimiser drives emit for the gated corpus (141) but not the full path | one measured SSA→emit pass with a published bench delta | 174 |
| NFR-6 | CLI startup | not measured | measured budget recorded and enforced | 162 |
| NFR-7 | Static safety | borrow checker, escape rejection (K114), checked div/mod/index, runtime stack traces | linear types enforced; match-arm bodies checked; every wrong-code class fixed or loudly rejected | 160, 176 |
| NFR-8 | Diagnostics quality | `K0xx`/`K1xx`/`K145` codes, real spans + excerpts, `karkain-diagnostics-v1` JSON, typo hints | every new surface ships its codes in the same commit | every increment |
| NFR-9 | Error semantics | exit codes 0/1/2/3/4/6/7; `runtime error: <kind> at <file>:<line>` + stack | identical semantics on the native path | 150 |
| NFR-10 | Portability | 10 triples + `wasm32-wasi` + `native-x86_64-linux`; deterministic exit 6 without a cross-linker | native win/macOS; riscv64 real; freestanding; Android | 150, 170, 171, 172 |
| NFR-11 | Security | digest-verified registry fetch; no implicit network; SECURITY.md | constant-time compare + HMAC + streaming digests (TLS and AES-GCM/RSA stay Planned — they fight the no-C-dependency rule) | 158, 182 |
| NFR-12 | Concurrency correctness | work-stealing scheduler, bounded channels, actors, >1 M msg/s runtime test, kcc parity (137) | in-language `atomic_*` + memory ordering with litmus goldens | 161 |
| NFR-13 | ABI stability | native calling convention documented (148); C path is C23 | freeze; `2.0.0` deliberately resets struct layout (the one breaking change) | 177 |
| NFR-14 | Observability | DWARF-4 emitted + parsed; `prof` text/json/folded; `karkain dbg` gdb backtraces | DWARF **consumed** (breakpoints/step); `prof` allocation-site accuracy | 153, 175 |
| NFR-15 | Toolchain independence | stages 2–3 run Go-free (143 gate); a C compiler is still required | no Go anywhere; no C compiler on the default path | 152, 154 |
| NFR-16 | Unicode / i18n | string case is ASCII-only (documented debt) | unicode-aware, or explicitly documented **and** gated | 158 |
| NFR-17 | Freestanding footprint | `runtime/freestanding` + `runtime/core` compile with `-ffreestanding -nostdlib` | a real `--target freestanding` used by codegen (MCU path) | 170 |
| NFR-18 | Edge/binary size | hello WASM = 2488 bytes, deterministic | GC/components without a size regression | 171 |

## 6. Library backlog (method-level)

**Measured today**: 16 `.kark` stdlib files / **414 functions** — **11 modules /
207 functions importable** (`string` 28, `collections` 24, `io` 11, `encoding`
7, `crypto` 2, `testing` 9, `numerics` 40, `net` 10, `http` 21, `db` 43,
`generics` 12) and **5 modules / 207 functions source-present but not
importable** (`core` 23, `math` 100, `system` 16, `gpu` 31, `actor` 37 — the
Phase-109 boundary).

Convention: **EXTEND** = add methods to a frozen module; **PROMOTE** = unfreeze
with both-engine gates; **NEW** = a new module; **MERGE** = fold overlaps so
each operation has exactly one home.

| Module | Add | Class | Increment |
|---|---|---|---|
| `std.string` | `str_pad_left/right`, `str_split_lines`, `str_replace_all`, `str_to_float`, `str_is_numeric`, `str_format` (`{}`); unicode-aware case verdict (NFR-16) | EXTEND | 158 |
| `std.collections` | `array_sort/sorted`, `map_remove`, `map_get_default`, `array_zip/enumerate`, `set_*`; then `array_map/filter/reduce` (first-class `fn` already shipped in 133) | EXTEND | 158 |
| `std.io` ⊃ `std.system` | **MERGE verdict**: one home for `file_exists`/`list_dir`/`remove_file`; add `stat_size/mtime`, `mkdir_all`, stdin/stdout/stderr handles, buffered read/write; locks later | MERGE + EXTEND | 156/158 |
| `std.encoding` | `url_encode/decode`, `base32`; JSON **moves out** to the new `std.json` | EXTEND | 158 |
| `std.crypto` | `hmac_sha256`, constant-time `secure_eq`, streaming `sha256_update/final`, `random_bytes` (NFR-11) | EXTEND | 158 |
| `std.testing` | `assert_approx_eq`, `skip(reason)`, grouped `suite`, `bench_*` timing helper | EXTEND | 163 |
| `std.numerics` | broadcasting, `mat_det/inv/solve`, `fft`, tensor `save/load`, autodiff `grad` | EXTEND | 159+ |
| `std.net` | `udp_send/recv`, `set_timeout`, `dns_lookup`; non-blocking + `select` (with NFR-12) | EXTEND | 158/161 |
| `std.http` | `query_parse/build`, `header_get/set`, cookies, `follow_redirects`, timeouts; keep-alive + chunked later | EXTEND | 158 |
| `std.db` | `JOIN`, `ORDER BY/LIMIT/OFFSET`, aggregates (`count/sum/avg/min/max`), `db_bind`, `csv_import/export`, migration helper — one clause per increment, external drivers never | EXTEND | one per increment from 156 |
| `std.generics` | extend after generics v2 (`Deque[T]`, `Set[T]` on the same pattern) | EXTEND | 159 |
| **`std.path`** (new) | `path_join`, `path_base`, `path_dir`, `path_ext`, `path_exists`, `path_abs`, `path_clean` | NEW | 155 |
| **`std.env`** (new) | `env_get`, `env_set`, `env_has`, `env_all`, args accessors | NEW | 155 |
| **`std.time`** (new) | `time_now`, `time_sleep`, `time_monotonic`, duration arithmetic/format | NEW | 155 |
| **`std.random`** (new) | seeded `rand_int(lo,hi)`, `rand_bytes(n)`, `rand_seed(n)` — deterministic goldens | NEW | 155 |
| **`std.json`** (new) | flat `json_encode`/`json_decode` + typed getters | NEW | 155 |
| later **NEW** | `std.log`, `std.cli`, `std.fs` (`walk`/`glob`), `std.compress` (gzip), `std.regex`, `std.sync` (atomics surface) — one per increment after 1.4.0 | NEW | 1.5.0+ |
| promote/cut verdicts | `std.math` (needs NaN/Inf/div-zero edge gates), `std.actor` (needs `timeout_recv`; `select` with NFR-12), `std.core` (**merge** into math/collections), `std.system` (**merge** into io), `std.gpu` (promote only with the WGSL surface) | PROMOTE/MERGE | 156 |

**Honesty note**: the I-1 OS modules (`path`/`env`/`time`/`random`/`json`) were
planned for the "133 slot" but **never shipped** — there is no source file for
any of them. They gate native/PM work, so they open 1.3.0 as increment 155.

## 7. Verified state snapshot (increment 149)

| Fact | Value |
|---|---|
| `VERSION` / banner | `1.1.0` / `Karkain Compiler v1.1.0 (windows/amd64, Stable Build)` |
| Branch state | `main` @ `624323e`, tree clean; `develop` and `1.0.x` present and pushed |
| Tags on origin | `karkain-17`, `v0.14.0`, `v0.19.0`, `v1.0.0` — **no `v1.1.0`** |
| Gates | 24 CI gate steps (Phases 114–148 + `pkg/native` + unit suites), plus the `native-windows` job and the informational `native-evidence` job |
| Corpus | 61 pinned goldens across 15 categories (210 `.kark` files under `examples/`) |
| Conformance | 12 files / **64** `func test_*` tests |
| Tests in tree | 211 `*_test.go`, of which 81 are `phase*_test.go` (59 in `pkg/cli`) |
| Audit reports | 85 `PHASE-*.md` records |
| KIR pin | whole-tree KIR = **9574** lines (Phase 146c) |
| Toolchain | Go 1.27.0 windows/amd64; gcc on PATH; wasmtime available for the WASM leg |

## 8. Status ledger (updated per increment)

| Version | Increments | State | Blocker |
|---|---|---|---|
| 1.1.0 | 128–149 | **in-tree complete**, gates green | owner tag/archive ceremony |
| 1.2.0 | 150, 151, 152, 154, 165 | **open** — 0 of 5 started | none (starts with 150) |
| 1.3.0 | 155, 156, 158, 159, 160, 161 | scoped, not started | needs the 1.2.0 native/lib baseline |
| 1.4.0 | 153, 157, 163, 164, 175, 176 | scoped, not started | needs the 1.3.0 library rows |
| 1.5.0 | 166, 167, 168, 169, 170, 171, 172 | scoped, not started | macOS signing and BSD VM are owner/community items |
| 1.6.0 | 162, 174, 179* | scoped, not started | 179 needs an operated registry service |
| 2.0.0 | 177, 178, 180 | scoped, not started | 177 is the intentional breaking change |

## 9. Numbering and anti-scope

* Increments 128–149 are historical facts. **150–153 are already decided** by
  the audit trail (`PHASE-147-BASELINE.md` §4 plus the 148/149 reports);
  **154+ are proposals** — a number is assigned only when an increment is
  scheduled, and renumbering is allowed only before its baseline note is
  written.
* Ride-along, unnumbered housekeeping (the L-0/L-6 rule): doc and bucket drift
  fixes, `AGENTS.md` records, the `std.generics` documentation gap, the
  `std.db` documented-vs-real function-count mismatch, and the promote-or-cut
  verdicts for the source-present-but-not-importable modules. These attach to
  the increment that touches the same area.
* **Never claimed** (stays `Planned` in `docs/source/status/planned.rst` with
  "no code exists" until a gate can run on real hardware or an operated
  service): public package registry, TLS, AES-GCM/ChaCha/RSA, async/await, JIT
  tier, quantum authoring surface, GPU/NPU hardware validation, HPC/space
  profiles, Android/iOS distribution, and any device scale without a runner or
  an acknowledged emulator.



