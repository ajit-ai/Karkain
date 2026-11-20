# Karkain v1.3.0 — Engineering Requirements: Production-Readiness Foundation

**Status:** Proposed engineering scope (owner-authorized 2026-11-20). Not yet
implemented; not yet gated. Recorded here so the v1.3.0 release has a written
home for objectives that go beyond the increment queue in
`docs/release/v1.3.0-CHECKLIST.md`.

**Companion documents:**
- `docs/audit/KARKAIN-IMPLEMENTATION-CONTROL.md` (authoritative queue)
- `docs/audit/KARKAIN-VERSION-PLAN.md` (version scope)
- `docs/release/v1.3.0-CHECKLIST.md` (increment queue + closure conditions)
- `docs/audit/KARKAIN_MATURITY_SCORECARD.md` (30-dimension quality assessment)

**Governing rule:** A capability is not complete because it is implemented. It is
complete only when its gate passes and its evidence is recorded. "Green except X"
is not complete. Unsupported constructs must be rejected explicitly, not
silently routed through another backend.

---

## 0. Recommended implementation order

1. Memory safety — establish ownership, borrowing, lifetime, and concurrency rules.
2. Native feature parity — make the native backend implement the defined language semantics consistently.
3. Toolchain independence — close compiler bootstrap, linker, runtime, standard-library, and build-system dependencies.
4. Independent performance qualification — benchmark the stable native compiler and publish reproducible results.
5. GPU and quantum compilation — develop as separate target platforms on top of stable compiler infrastructure.

GPU and quantum are longer-term compiler targets, not prerequisites for a
credible native CPU compiler. They should not be started until the core
compiler interfaces are stable.

## 1. Architectural prerequisite — the Language and Compiler Contract

Before implementing the five objectives below, Karkain needs a written Language
and Compiler Contract covering:

1. Language semantics and supported constructs.
2. Memory and concurrency safety guarantees.
3. KIR specification and backend responsibilities.
4. Runtime and ABI specifications.
5. Supported target platforms and their limitations.
6. Conformance, security, and performance acceptance tests.

Without this contract, each backend can evolve differently, and a feature can
appear implemented without being consistently supported.

## 2. Objective 1 — Complete production-grade memory safety

**Design objective:** Prevent programs written in the supported safe subset from
causing memory corruption or invalid memory access, without relying on garbage
collection.

### Design requirements

| Requirement | What Karkain needs |
|---|---|
| Ownership model | Define who owns each allocated value and when ownership transfers. |
| Borrow checking | Enforce valid shared and exclusive borrows, including across function calls. |
| Lifetime analysis | Prevent references from outliving the data they reference. |
| Bounds safety | Check array, slice, and pointer-derived access where static proof is unavailable. |
| Initialization safety | Prevent reads of uninitialized or partially initialized values. |
| Type safety | Prevent invalid casts, layout violations, and type confusion. |
| Concurrency safety | Prevent data races and unsafe shared mutation in safe code. |
| Unsafe boundary | Define explicit unsafe operations and their obligations. |

### Required enhancements

- Formal ownership and borrowing rules in the language specification.
- Compile-time lifetime and alias analysis.
- Safe allocation, deallocation, and container APIs.
- Sanitizer or runtime-checking support for development builds.
- Negative compiler tests for use-after-free, dangling references, double-free,
  invalid aliasing, and out-of-bounds access.

### Acceptance criterion (Gate 1 — Safety)

A documented safe subset with tests demonstrating that invalid memory operations
are rejected or safely trapped. Do not claim complete memory safety for
unsupported or explicitly unsafe operations. No known unresolved critical
memory-safety defects in the claimed safe language subset.

## 3. Objective 2 — Full native feature parity

**Design objective:** Programs accepted by Karkain's language front end should
have equivalent supported behavior on the native backend and the C23 backend.

### Design requirements

| Area | Required enhancement |
|---|---|
| Types | All supported primitive, composite, and user-defined types |
| Functions | Calls, recursion, closures if supported, and calling conventions |
| Collections | Arrays, slices, strings, maps, and records |
| Control flow | Loops, iteration, pattern matching, and error propagation |
| Memory | Allocation, ownership, references, and destruction |
| Standard library | Native implementations of all supported library modules |
| Runtime | Consistent panic, error, I/O, and concurrency behavior |
| ABI | Defined data layout, alignment, and external calling conventions |

### Architectural requirement

Build a single backend-independent intermediate representation (KIR) with
explicit semantics. Both native and C23 backends implement the same language
contract.

### Acceptance criterion (Gate 2 — Parity)

A shared conformance suite runs the same programs on both backends and compares
exit status, output, errors, and observable behavior. Unsupported constructs
must be rejected explicitly, not silently routed through another backend. Shared
conformance tests pass for all officially supported native and C23 features.

## 4. Objective 3 — Complete toolchain independence

**Design objective:** Build, compile, link, test, and package Karkain programs
without requiring a C compiler, Go compiler, or external system toolchain for
the supported native targets.

This is broader than generating ELF, PE, or Mach-O files. It requires control
over the complete compilation and delivery pipeline.

### Design requirements

| Area | Required enhancement |
|---|---|
| Compiler bootstrap | Build the Karkain compiler from Karkain source using a previously qualified Karkain compiler. |
| Native linker | Provide an internal linker or a clearly specified alternative that does not require an external C/Go toolchain. |
| Runtime | Implement required startup, memory, panic, threading, and system-call support. |
| Standard library | Remove dependencies on C/Go implementations for supported modules. |
| Object formats | Support required object files, symbols, relocations, libraries, and executable formats. |
| Cross-compilation | Support target-specific layouts, ABIs, and system interfaces. |
| Build system | Compile, link, test, and package projects using Karkain-native tools. |
| Reproducibility | Produce repeatable builds with recorded compiler and dependency versions. |

### Required enhancements

- A documented bootstrap chain, including how the first trusted compiler is produced.
- Native linker and runtime strategy for each supported operating system.
- A dependency inventory identifying every remaining external tool and library.
- Reproducible build tests on clean machines or isolated build environments.

### Acceptance criterion (Gate 3 — Independence)

A clean environment with no C or Go compiler installed can build the compiler
and supported applications for each claimed target. The bootstrap process must
also be documented and repeatable.

## 5. Objective 4 — Mature GPU or quantum compilation

**Design objective:** Extend Karkain's compiler architecture to target
accelerator and quantum hardware through explicit intermediate representations
and well-defined execution models. These are two substantial areas and must not
be treated as one feature.

### 5.1 GPU compilation

| Design requirement | What Karkain needs |
|---|---|
| GPU intermediate representation | A representation for kernels, memory spaces, and parallel execution. |
| Execution model | Explicit thread, block, grid, and synchronization semantics. |
| Memory model | Host/device memory, transfers, shared memory, and synchronization rules. |
| GPU backends | A chosen target such as NVIDIA CUDA/PTX or a portable GPU abstraction. |
| Runtime | Device discovery, allocation, kernel dispatch, and error handling. |
| Optimization | Coalescing, occupancy, memory reuse, and kernel optimization. |
| Testing | Hardware-backed correctness and performance tests. |

### 5.2 Quantum compilation

| Design requirement | What Karkain needs |
|---|---|
| Quantum IR | A representation of circuits, gates, measurements, and control flow. |
| Type model | Explicit quantum-state and classical-data boundaries. |
| Gate validation | Valid operations, gate parameters, and target constraints. |
| Lowering | Translation to a defined target such as OpenQASM or QIR. |
| Hardware mapping | Gate-set conversion, connectivity mapping, and scheduling. |
| Runtime | Simulator or hardware execution integration. |
| Verification | Circuit equivalence, simulator tests, and hardware-backed validation where available. |

### Acceptance criterion (Gate 4 — Accelerator research)

Each target needs a supported language subset, a defined compiler backend, a
runtime or execution integration, and independently reproducible correctness
tests. Merely generating PTX, OpenQASM, or QIR is not proof of mature GPU or
quantum execution. For v1.3.0: GPU and quantum architecture is documented, with
experimental prototypes clearly labelled. Full production maturity is reserved
for later releases.

## 6. Objective 5 — Independently benchmarked production performance

**Design objective:** Demonstrate that Karkain performs competitively and
reliably on real workloads, using fair and repeatable comparisons.

### Design requirements

| Design requirement | What Karkain needs |
|---|---|
| Benchmark suite | Representative systems, compute, memory, I/O, and concurrency workloads. |
| Baselines | Equivalent implementations in established languages and compilers. |
| Fair comparison | Same algorithms, input sizes, hardware, and optimization settings. |
| Measurement | Runtime, throughput, latency, memory usage, and binary size. |
| Repeatability | Multiple runs, controlled environments, and published methodology. |
| Correctness | Verify output correctness before accepting performance results. |
| Independent review | External engineers reproduce results and inspect the methodology. |
| Regression tracking | Detect performance degradation between releases. |

### Suggested benchmark categories

- CPU arithmetic and numerical computation.
- Memory allocation and data structures.
- String and collection processing.
- File and network I/O.
- Multithreaded workloads and contention.
- Startup time, compilation time, and executable size.
- Long-running workloads and memory stability.

### Acceptance criterion (Gate 5 — Performance)

Publish source code, compiler versions, hardware details, build flags, raw
results, and benchmark methodology. An independent reviewer should be able to
reproduce the results. Benchmark results and methodology are published, with
independent reproduction or review documented.

## 7. v1.3.0 release acceptance gates (summary)

| Gate | Objective | Criterion |
|---|---|---|
| Gate 1 — Safety | 1 | No known unresolved critical memory-safety defects in the claimed safe language subset. |
| Gate 2 — Parity | 2 | Shared conformance tests pass for all officially supported native and C23 features. |
| Gate 3 — Independence | 3 | Clean-environment builds succeed without C/Go toolchains for explicitly supported targets. |
| Gate 4 — Accelerator research | 4 | GPU and quantum architecture documented, with experimental prototypes clearly labelled. |
| Gate 5 — Performance | 5 | Benchmark results and methodology published, with independent reproduction or review documented. |

These gates are aspirational targets for v1.3.0. Given Karkain's v1.2.0
baseline, v1.3.0 is best framed as the **production-readiness foundation**:
complete the safety specification and negative qualification tests, close the
documented native backend gaps, establish shared conformance tests, complete
supported-target bootstrap/linker/runtime/stdlib dependency closure, document
GPU/quantum architecture with experimental prototypes, and publish reproducible
benchmarks. Full GPU/quantum maturity and any remaining advanced safety or
platform work are reserved for later releases, based on evidence rather than a
predetermined date.

## 8. Status discipline

- These objectives are `proposed` until an owner authorizes them as increments.
- No increment number is assigned here (per `KARKAIN-VERSION-PLAN.md` section 0,
  numbers are assigned when an increment is scheduled).
- A capability is not complete because it is implemented. It is complete only
  when its gate passes and its evidence is recorded.
- "Green except X" is not complete.
- Unsupported constructs must be rejected explicitly, not silently routed
  through another backend.
