package cpu

// SimdExtensions is the SIMD/vector C implementation added on top of
// TensorCRuntime. It uses GNU vector extensions (portable across GCC and
// Clang) rather than ISA-specific intrinsics: the compiler lowers the fixed
// lane-width vectors to the target's native SIMD width, so the same runtime
// works on x86 (SSE/AVX), AArch64 NEON, etc. without -march flags.
//
// Determinism contract:
//   - Lane width is fixed (double4 = 4×double, float8 = 8×float).
//   - Results are identical to the scalar oracle for add/sub/mul/div/relu
//     (same element order, vector loads/stores are element-preserving).
//   - MatMul vectorizes the K accumulation in 4-lane blocks — floating-point
//     rounding order therefore differs from the scalar triple loop, so the
//     differential parity harness compares matmul with a tolerance.
const SimdExtensions = `
/* Karkain Tensor IR SIMD Extensions (GNU vector extensions, portable) */
typedef double double4 __attribute__((vector_size(32)));
typedef float  float8  __attribute__((vector_size(32)));

/* Same-shape contiguous elementwise op. op: 0=add 1=sub 2=mul 3=div.
 * Broadcast/rank-mismatched inputs are not vectorized here and fall back to
 * the scalar oracle path by the tensor_simd_* wrappers. */
static Tensor* tensor_simd_contig2(const Tensor* a, const Tensor* b, int op) {
    size_t n = tensor_numel(a);
    Tensor* r = tensor_create(a->ndim, a->shape, 0);
    size_t i = 0;
    for (; i + 4 <= n; i += 4) {
        double4 av = (double4){ a->data[i], a->data[i+1], a->data[i+2], a->data[i+3] };
        double4 bv = (double4){ b->data[i], b->data[i+1], b->data[i+2], b->data[i+3] };
        double4 rv;
        if (op == 0)      rv = av + bv;
        else if (op == 1) rv = av - bv;
        else if (op == 2) rv = av * bv;
        else              rv = av / bv;
        r->data[i]   = rv[0];
        r->data[i+1] = rv[1];
        r->data[i+2] = rv[2];
        r->data[i+3] = rv[3];
    }
    for (; i < n; i++) {
        double x = a->data[i], y = b->data[i];
        if (op == 0)     r->data[i] = x + y;
        else if (op == 1) r->data[i] = x - y;
        else if (op == 2) r->data[i] = x * y;
        else              r->data[i] = x / y;
    }
    return r;
}

Tensor* tensor_simd_add(Tensor* a, Tensor* b) {
    if (a->ndim == b->ndim && tensor_numel(a) == tensor_numel(b)) return tensor_simd_contig2(a, b, 0);
    return tensor_add(a, b);
}
Tensor* tensor_simd_sub(Tensor* a, Tensor* b) {
    if (a->ndim == b->ndim && tensor_numel(a) == tensor_numel(b)) return tensor_simd_contig2(a, b, 1);
    return tensor_sub(a, b);
}
Tensor* tensor_simd_mul(Tensor* a, Tensor* b) {
    if (a->ndim == b->ndim && tensor_numel(a) == tensor_numel(b)) return tensor_simd_contig2(a, b, 2);
    return tensor_mul(a, b);
}
Tensor* tensor_simd_div(Tensor* a, Tensor* b) {
    if (a->ndim == b->ndim && tensor_numel(a) == tensor_numel(b)) return tensor_simd_contig2(a, b, 3);
    return tensor_div(a, b);
}

/* MatMul with a 4-lane-vectorized K accumulation (batched form supported). */
Tensor* tensor_simd_matmul(Tensor* a, Tensor* b) {
    int M = a->shape[a->ndim - 2];
    int K = a->shape[a->ndim - 1];
    int N = b->shape[b->ndim - 1];
    int ndim = a->ndim + b->ndim - 2;
    int shape[8] = {0};
    for (int i = 0; i < a->ndim - 2; i++) shape[i] = a->shape[i];
    shape[ndim - 2] = M;
    shape[ndim - 1] = N;
    Tensor* r = tensor_create(ndim, shape, 0);
    int batchSize = 1;
    for (int i = 0; i < ndim - 2; i++) batchSize *= shape[i];
    for (int b0 = 0; b0 < batchSize; b0++) {
        for (int i = 0; i < M; i++) {
            for (int j = 0; j < N; j++) {
                double sum = 0.0;
                int base_a = b0 * M * K + i * K;
                int base_b = b0 * K * N + j;
                int k = 0;
                for (; k + 4 <= K; k += 4) {
                    double4 av = (double4){ a->data[base_a + k],     a->data[base_a + k + 1],
                                            a->data[base_a + k + 2], a->data[base_a + k + 3] };
                    double4 bv = (double4){ b->data[base_b + k * N],     b->data[base_b + (k + 1) * N],
                                            b->data[base_b + (k + 2) * N], b->data[base_b + (k + 3) * N] };
                    double4 pv = av * bv;
                    sum += pv[0] + pv[1] + pv[2] + pv[3];
                }
                for (; k < K; k++) sum += a->data[base_a + k] * b->data[base_b + k * N];
                r->data[b0 * M * N + i * N + j] = sum;
            }
        }
    }
    return r;
}

/* Relu with 4-lane vector loads and per-lane select (fully element-exact). */
Tensor* tensor_simd_relu(Tensor* x) {
    Tensor* r = tensor_copy(x);
    size_t n = tensor_numel(r);
    size_t i = 0;
    for (; i + 4 <= n; i += 4) {
        double4 v = (double4){ r->data[i], r->data[i+1], r->data[i+2], r->data[i+3] };
        r->data[i]   = (v[0]   < 0.0) ? 0.0 : v[0];
        r->data[i+1] = (v[1]   < 0.0) ? 0.0 : v[1];
        r->data[i+2] = (v[2]   < 0.0) ? 0.0 : v[2];
        r->data[i+3] = (v[3]   < 0.0) ? 0.0 : v[3];
    }
    for (; i < n; i++) if (r->data[i] < 0.0) r->data[i] = 0.0;
    return r;
}
`

// TensorSimdCRuntime is the full SIMD-enabled runtime: the scalar oracle
// runtime plus the SIMD extensions. Emitted by the SIMD CPU backend variant.
const TensorSimdCRuntime = TensorCRuntime + SimdExtensions
