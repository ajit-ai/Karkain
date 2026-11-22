==================================================
Karkain AI and Scientific Computing Readiness
==================================================

This assessment maps Karkain's actual, evidenced capabilities against the AI
workloads named in the foundation assignment. It is deliberately conservative:
a unit-tested internal Go package is **not** the same as a capability reachable
from a ``.kark`` program, and neither is the same as a verified end-to-end
workload.

Evidence classes
================

``EXECUTABLE``
   A gate exists (and is CI-wired) that exercises the capability.
``UNIT ONLY``
   Real implementation with unit tests, but unreachable from ``.kark`` source.
``SOURCE ONLY``
   Source exists without an executing gate.
``ABSENT``
   No implementation found after an exhaustive repository search.

1. What actually exists
========================

.. list-table::
   :widths: 34 16 50
   :header-rows: 1

   * - Component
     - Class
     - Finding
   * - ``pkg/math`` (Math IR, Phase 71)
     - UNIT ONLY
     - Real graph model, interpreter, optimiser, printer, validator; ~60 unit
       tests in ``ir_test.go``
   * - ``pkg/tensor`` (Tensor IR, Phases 72/74)
     - UNIT ONLY
     - Shape/type system, graph builder (matmul/relu/softmax/reshape/transpose),
       broadcasting, ``LowerToC``, gradient tests
   * - ``pkg/tensor/gradient.go``
     - UNIT ONLY
     - Reverse-mode gradient emission (matmul/relu/sigmoid/elementwise) with
       ``grad_test.go``
   * - ``pkg/sema/autodiff.go``
     - UNIT ONLY
     - Real reverse-mode engine + shape checker with ``autodiff_test.go``; the
       parser cannot produce the required ``TensorStmt`` AST from ``.kark``
       text, so no program reaches it
   * - ``pkg/npu`` + 5 vendor adapters
     - UNIT ONLY
     - Fusion, memory planning, INT8/INT4/mixed quantization, MLIR dialect,
       CPU-oracle fallback; adapters are env-gated and fall back to CPU
   * - ``pkg/backend/cpu``
     - UNIT ONLY
     - Embedded C23 reference runtime (``runtime.go``, ``runtime_c.go``,
       ``simd_c.go``) with ``cpu_test.go`` / ``parity_test.go``
   * - ``std.numerics`` (40 functions)
     - EXECUTABLE
     - Vector/matrix/tensor helpers, activations, losses, MLP forward;
       byte-identical Go↔kcc goldens (Phase 114/131/132)
   * - SIMD (8 lane types, 6 ``@simd_*`` builtins)
     - EXECUTABLE
     - ``TestPhase106_*`` checks generated C, scalar differential and emitted
       AVX instructions (Go engine; kcc lowering deferred)
   * - WGSL kernels (``@target(gpu)``)
     - EXECUTABLE (compile-only)
     - ``TestPhase138_*`` verifies emitted WGSL and deterministic target
       rejection; **no hardware, SDK or driver required or used**
   * - ``stdlib/gpu/gpu.kark`` (31 functions)
     - SOURCE ONLY
     - No dedicated gate located
   * - Bytecode/JIT tensor opcodes (``pkg/ir/bytecode.go``, ``pkg/jit``)
     - SOURCE ONLY (stubs)
     - Documented as stubs ("matmul returns 0, grad returns empty") in
       ``docs/KARKAIN-MATH-REPOSITORY-ASSESSMENT.md:86-87``
   * - Tokenizer / transformer / attention / embeddings / inference / LLM
     - ABSENT
     - Exhaustive search found only the compiler's own lexer ``tokenize()`` and
       the word "inference" in capability descriptions. ``examples/09-ai`` and
       ``examples/10-machine-learning`` READMEs state there is no ``std.ai``,
       ``tensor()`` or ``infer()`` surface

2. Workload-by-workload assessment
===================================

.. list-table::
   :widths: 32 18 50
   :header-rows: 1

   * - Workload
     - Status
     - Prerequisites / limitation
   * - Data processing and transformation
     - PARTIALLY EXECUTABLE
     - ``std.string``/``collections``/``io``/``encoding``/``json`` are gated;
       CSV/XML/compression are absent
   * - Numerical and scientific computing
     - PARTIALLY EXECUTABLE
     - 40 ``std.numerics`` functions with both-engine goldens; no native float
       statements in the kcc driver; no linear-algebra library beyond
       small-matrix helpers
   * - Machine learning
     - PARTIALLY EXECUTABLE
     - Six AI/ML examples with goldens (nearest neighbour, linear classifier,
       linear regression, gradient descent, MLP forward, numerics forward)
   * - Deep learning / neural networks
     - NOT EXECUTABLE
     - Only a hand-written forward MLP example; no layer/autograd/optimizer
       surface reachable from ``.kark``
   * - Automatic differentiation
     - UNIT ONLY
     - ``pkg/sema/autodiff.go`` and ``pkg/tensor/gradient.go`` are real but
       unreachable from source (parser gap)
   * - Tensor computation
     - UNIT ONLY
     - ``pkg/tensor`` IR is real; no language-level tensor type or runtime
   * - Transformer / attention
     - NOT FEASIBLE TODAY
     - No attention primitive, tokenizer or model format
   * - LLM inference
     - NOT FEASIBLE TODAY
     - No model loading, tokenizer, transformer or inference engine
   * - Model training / fine-tuning
     - NOT FEASIBLE TODAY
     - No optimiser, no reachable autodiff, no weight storage format
   * - Embeddings / retrieval
     - NOT FEASIBLE TODAY
     - No vector index or embedding model surface
   * - Multimodal AI
     - NOT FEASIBLE TODAY
     - No image/audio input primitives
   * - AI agents / workflows
     - NOT FEASIBLE TODAY
     - No tool-calling or model interface; ``std.async`` is not importable
   * - GPU-accelerated computation
     - COMPILE-ONLY / EXPERIMENTAL
     - WGSL emission is gated and hardware-independent; no device execution
       evidence; ``compute-targets.rst`` states the Phase 124 model drives no
       real hardware
   * - Distributed training / inference
     - NOT FEASIBLE TODAY
     - No distributed runtime; ``std.net`` is C-backend only and native-refused

3. Dependency-aware prerequisites (do not skip ahead)
=======================================================

Before any LLM/training capability is plausible, the following must exist in
order:

1. **Language/runtime**: native float statements and a reachable tensor value
   type; currently refused by name in the kcc native driver.
2. **Autodiff reachability**: bridge ``TensorStmt`` into the parser/AST so the
   existing ``pkg/sema/autodiff.go`` engine can be invoked from ``.kark``.
3. **Tensor runtime**: a native tensor representation and BLAS-class kernels
   (the current ``std.numerics`` uses small fixed loops).
4. **Model I/O**: a serialization format and loader (no format exists today).
5. **Tokenization**: a text/subword tokenizer module (absent).
6. **Acceleration**: real GPU backend execution, not just WGSL text emission.

Only then do transformer/attention and inference become meaningful targets.

4. Honest conclusion
=====================

Karkain has a **real numerical and compiler-internal AI foundation** (math IR,
tensor IR, autodiff engine, NPU quantization, SIMD, WGSL emission, 40-function
numerics stdlib and six executable AI/ML examples). It does **not** have a
language-reachable deep-learning or LLM stack. No claim of model training, GPU
execution or LLM inference can be made from the evidence in this repository.
