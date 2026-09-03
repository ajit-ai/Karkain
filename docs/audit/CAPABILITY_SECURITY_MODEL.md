# KARKAIN CAPABILITY-BASED SECURITY MODEL — FEASIBILITY STUDY

Concept (FUTURE): capability-gated functions, e.g.
```
fn read_data() requires FileRead { }
```
Potential capabilities: FileRead, FileWrite, Network, Process, Environment, Database, GPU,
ForeignCall. This is a FEASIBILITY STUDY ONLY — do NOT implement until a complete model exists.

## Where it belongs
Not in the language core. The capability set is domain/ecosystem-specific (FileRead, Database,
GPU, ForeignCall) and should live as:
- **Primitive core capabilities** (a small fixed set: Memory, CPU, IO, Network, GPU, ForeignCall,
  Process) defined in the language runtime/spec.
- **Higher capabilities** (Database, specific device kinds) as library/ecosystem annotations.

Recommended placement to keep the core SMALL:
| Layer | Capability examples | Owner |
|-------|---------------------|-------|
| Language core | Memory, CPU (host compute) | spec |
| Runtime | IO, Network, GPU, ForeignCall (atomic-ish) | runtime |
| Stdlib/packages | Process, Environment, Database | packages |

## Whether it belongs as annotations vs in-core
Annotations (attributes/effects metadata) are the least invasive: a compiler pass that
checks a function's called capabilities against a declared `requires` list at the unsafe/FFI
boundary. This fits without new control-flow.

## Constraints / interaction
- Must be **opt-in and capability-permissive by default** for the safe core (memory safety via
  borrow is already the primary safety; capabilities are a secondary, opt-in discipline).
- Interacts with `unsafe` and FFI: every `raw`/`extern` boundary is where capabilities matter most.
- Should NOT block normal memory-safe code (Pillar: SIMPLE — don't force annotations everywhere).

## Decision
Do NOT implement now. If pursued, define a minimal primitive capability set in the runtime, add
annotation-based `requires` gating at the unsafe/FFI boundary only, and defer richer capabilities
to the ecosystem. This preserves the small-core principle (Pillar 1).
