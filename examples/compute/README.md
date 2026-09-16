# Compute Target Workloads

Phase 124 example corpus: host-executable workloads whose KIR profiles map
onto the experimental GPU/NPU/Quantum compute targets modelled in
`pkg/target`. Every file is a real, runnable Karkain program (`Status:
Runnable`, `Engine: both` — byte-identical output on the Go front end and the
self-hosted kcc). They exist to illustrate the KIR lowering boundary, not to
claim any accelerator execution.

Catalog (`karkain target`):

| Target                | Maturity     | Surface                                        |
| --------------------- | ------------ | ---------------------------------------------- |
| `cpu`                 | implemented  | Reference scalar CPU executor                  |
| `simd`                | implemented  | CPU + lane-vector arithmetic (`@simd_*`)       |
| `wasm32-wasi`         | experimental | WebAssembly/WASI executor (Phase 108)          |
| `gpu-experimental`    | experimental | Kernel launch, tensor/matrix ops, async        |
| `npu-experimental`    | experimental | Synchronous tensor inference, device memory    |
| `quantum-experimental`| research     | Gate primitives and measurement (spec model)   |

Use `karkain target <name>` for the full capability view of any compute
target (capabilities, accepted KIR v1 classes, native tensor operations).

| File         | KIR profile of interest                     |
| ------------ | ------------------------------------------- |
| `vector_add` | element-wise add (GPU/NPU tensor ops)       |
| `matmul`     | 2x2 matrix multiply (GPU/NPU matmul)        |
| `reduce`     | parallel reduction (GPU/NPU reduce_sum)     |

These workloads are the boundary demonstration for the Phase 124 lowering
layer (`pkg/target/lowering.go`): the GPU and NPU targets accept their tensor
profiles, while host I/O and function-call boundaries are rejected
deterministically (`error[K124]`).