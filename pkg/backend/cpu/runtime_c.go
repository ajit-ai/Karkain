package cpu

// TensorCRuntime is the embedded C23 reference implementation of the
// Karkain tensor runtime. This is the correctness oracle — every accelerator
// must produce the same results.
const TensorCRuntime = `
#include <stdlib.h>
#include <string.h>
#include <math.h>

/* Karkain Tensor IR Reference Runtime (CPU backend - correctness oracle) */

typedef struct Tensor {
    int ndim;
    int* shape;
    int* strides;
    int dtype;
    double* data;
    int refcount;
} Tensor;

static size_t tensor_numel(const Tensor* t) {
    size_t n = 1;
    for (int i = 0; i < t->ndim; i++) n *= (size_t)t->shape[i];
    return n;
}

Tensor* tensor_create(int ndim, const int* shape, int dtype) {
    Tensor* t = (Tensor*)calloc(1, sizeof(Tensor));
    t->ndim = ndim;
    t->shape = (int*)calloc(ndim ? ndim : 1, sizeof(int));
    t->strides = (int*)calloc(ndim ? ndim : 1, sizeof(int));
    if (ndim > 0) memcpy(t->shape, shape, ndim * sizeof(int));
    t->dtype = dtype;
    size_t numel = tensor_numel(t);
    t->data = (double*)calloc(numel ? numel : 1, sizeof(double));
    t->refcount = 1;
    /* row-major strides */
    size_t stride = 1;
    for (int i = ndim - 1; i >= 0; i--) {
        t->strides[i] = (int)stride;
        stride *= (size_t)t->shape[i];
    }
    return t;
}

void tensor_destroy(Tensor* t) {
    if (!t) return;
    if (--t->refcount > 0) return;
    free(t->shape);
    free(t->strides);
    free(t->data);
    free(t);
}

Tensor* tensor_copy(Tensor* t) {
    Tensor* r = tensor_create(t->ndim, t->shape, t->dtype);
    memcpy(r->data, t->data, tensor_numel(t) * sizeof(double));
    return r;
}

static int tensor_flat_index(const Tensor* t, const int* idx) {
    int flat = 0;
    for (int i = 0; i < t->ndim; i++) flat += idx[i] * t->strides[i];
    return flat;
}

double tensor_get(const Tensor* t, const int* idx) {
    return t->data[tensor_flat_index(t, idx)];
}

void tensor_set(Tensor* t, const int* idx, double value) {
    t->data[tensor_flat_index(t, idx)] = value;
}

static void broadcast_strides(const Tensor* a, int ndim, int* out) {
    int ra = a->ndim;
    int shift = ndim - ra;
    for (int i = 0; i < ndim; i++) {
        int dimA = i - shift;
        if (dimA < 0) { out[i] = 0; continue; }
        int sizeA = a->shape[dimA];
        out[i] = (sizeA == 1) ? 0 : a->strides[dimA];
    }
}

static int max2a(int a, int b) { return a > b ? a : b; }

static Tensor* elemwise2(const Tensor* a, const Tensor* b, double (*fn)(double, double)) {
    int ndim = max2a(a->ndim, b->ndim);
    int shape[8] = {0};
    int ra = a->ndim, rb = b->ndim;
    for (int i = 0; i < ndim; i++) {
        int sa = i < ndim - ra ? 1 : a->shape[i - (ndim - ra)];
        int sb = i < ndim - rb ? 1 : b->shape[i - (ndim - rb)];
        shape[i] = sa > sb ? sa : sb;
    }
    Tensor* r = tensor_create(ndim, shape, 0);
    int prod = 1;
    for (int i = 0; i < ndim; i++) prod *= shape[i];
    int sta[8] = {0}, stb[8] = {0};
    broadcast_strides(a, ndim, sta);
    broadcast_strides(b, ndim, stb);
    for (int flat = 0; flat < prod; flat++) {
        int idx[8];
        int rem = flat;
        for (int i = ndim - 1; i >= 0; i--) {
            idx[i] = rem % shape[i];
            rem /= shape[i];
        }
        int ia = 0, ib = 0;
        for (int i = 0; i < ndim; i++) {
            ia += idx[i] * sta[i];
            ib += idx[i] * stb[i];
        }
        r->data[flat] = fn(a->data[ia], b->data[ib]);
    }
    return r;
}

static double fn_add(double x, double y) { return x + y; }
static double fn_sub(double x, double y) { return x - y; }
static double fn_mul(double x, double y) { return x * y; }
static double fn_div(double x, double y) { return x / y; }

Tensor* tensor_add(Tensor* a, Tensor* b) { return elemwise2(a, b, fn_add); }
Tensor* tensor_sub(Tensor* a, Tensor* b) { return elemwise2(a, b, fn_sub); }
Tensor* tensor_mul(Tensor* a, Tensor* b) { return elemwise2(a, b, fn_mul); }
Tensor* tensor_div(Tensor* a, Tensor* b) { return elemwise2(a, b, fn_div); }

Tensor* tensor_matmul(Tensor* a, Tensor* b) {
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
                for (int k = 0; k < K; k++) {
                    int ia = b0 * M * K + i * K + k;
                    int ib = b0 * K * N + k * N + j;
                    sum += a->data[ia] * b->data[ib];
                }
                r->data[b0 * M * N + i * N + j] = sum;
            }
        }
    }
    return r;
}

Tensor* tensor_relu(Tensor* x) {
    Tensor* r = tensor_copy(x);
    size_t numel = tensor_numel(r);
    for (size_t i = 0; i < numel; i++) if (r->data[i] < 0.0) r->data[i] = 0.0;
    return r;
}

Tensor* tensor_sigmoid(Tensor* x) {
    Tensor* r = tensor_copy(x);
    size_t numel = tensor_numel(r);
    for (size_t i = 0; i < numel; i++) r->data[i] = 1.0 / (1.0 + exp(-r->data[i]));
    return r;
}

Tensor* tensor_tanh(Tensor* x) {
    Tensor* r = tensor_copy(x);
    size_t numel = tensor_numel(r);
    for (size_t i = 0; i < numel; i++) r->data[i] = tanh(r->data[i]);
    return r;
}

Tensor* tensor_softmax(Tensor* x) {
    Tensor* r = tensor_copy(x);
    size_t numel = tensor_numel(r);
    double maxv = r->data[0];
    for (size_t i = 1; i < numel; i++) if (r->data[i] > maxv) maxv = r->data[i];
    double sum = 0.0;
    for (size_t i = 0; i < numel; i++) {
        r->data[i] = exp(r->data[i] - maxv);
        sum += r->data[i];
    }
    for (size_t i = 0; i < numel; i++) r->data[i] /= sum;
    return r;
}

Tensor* tensor_transpose(Tensor* t) {
    int ndim = t->ndim;
    int shape[8] = {0};
    for (int i = 0; i < ndim; i++) shape[i] = t->shape[ndim - 1 - i];
    Tensor* r = tensor_create(ndim, shape, t->dtype);
    size_t numel = tensor_numel(t);
    for (size_t flat = 0; flat < numel; flat++) {
        int idx[8];
        int rem = (int)flat;
        for (int i = ndim - 1; i >= 0; i--) {
            idx[i] = rem % t->shape[i];
            rem /= t->shape[i];
        }
        int flatNew = 0;
        for (int i = 0; i < ndim; i++) flatNew += idx[ndim - 1 - i] * r->strides[i];
        r->data[flatNew] = t->data[flat];
    }
    return r;
}

Tensor* tensor_reshape(Tensor* t, const int* new_shape, int new_ndim) {
    Tensor* r = tensor_create(new_ndim, new_shape, t->dtype);
    memcpy(r->data, t->data, tensor_numel(t) * sizeof(double));
    return r;
}

Tensor* tensor_reduce_sum(Tensor* t, int axis) {
    int ndim = t->ndim;
    if (ndim == 0) return tensor_copy(t);
    Tensor* r = tensor_create(ndim - 1, t->shape, t->dtype);
    size_t inner = 1;
    int redSize = t->shape[axis];
    for (int i = axis + 1; i < ndim; i++) inner *= (size_t)t->shape[i];
    size_t numel = tensor_numel(t);
    for (size_t flat = 0; flat < numel; flat++) {
        size_t reduced = (flat / (inner * (size_t)redSize)) * inner + (flat % inner);
        r->data[reduced] += t->data[flat];
    }
    return r;
}
`
