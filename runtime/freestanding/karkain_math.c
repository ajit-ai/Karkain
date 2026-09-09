#include "karkain_runtime.h"
#include <stdint.h>

/* No-libm math. Double-precision functions implemented from first principles:
 * bit-level floor/ceil/trunc/round, Newton sqrt, range-reduced Taylor for
 * exp/log, and fmod via trunc. NaN/inf are produced from bit patterns so no
 * runtime support is required. Accuracy: near or at 1 ulp for algebraic
 * functions; ~1e-12 relative for exp/log/pow. */

typedef union {
    double d;
    uint64_t u;
} karkain_u64;

static double karkain_nan(void) {
    karkain_u64 v;
    v.u = 0x7FF8000000000000ull;
    return v.d;
}

static double karkain_infpos(void) {
    karkain_u64 v;
    v.u = 0x7FF0000000000000ull;
    return v.d;
}

static int karkain_exponent(double x) {
    karkain_u64 v;
    v.d = x;
    return (int)((v.u >> 52) & 0x7FF) - 1023;
}

double karkain_fabs(double x) {
    karkain_u64 v;
    v.d = x;
    v.u &= 0x7FFFFFFFFFFFFFFFull;
    return v.d;
}

double karkain_copysign(double x, double y) {
    karkain_u64 vx, vy;
    vx.d = x;
    vy.d = y;
    vx.u = (vx.u & 0x7FFFFFFFFFFFFFFFull) | (vy.u & 0x8000000000000000ull);
    return vx.d;
}

double karkain_fmin(double a, double b) {
    if (a != a) return b;
    if (b != b) return a;
    return a < b ? a : b;
}

double karkain_fmax(double a, double b) {
    if (a != a) return b;
    if (b != b) return a;
    return a > b ? a : b;
}

double karkain_trunc(double x) {
    karkain_u64 v;
    v.d = x;
    int e = karkain_exponent(x);
    if (e < 0) {
        v.u &= 0x8000000000000000ull; /* |x| < 1 -> +-0.0 */
        return v.d;
    }
    if (e >= 52) return x;
    v.u &= ~(0xFFFFFFFFFFFFFull >> e);
    return v.d;
}

double karkain_floor(double x) {
    double t = karkain_trunc(x);
    if (x < 0.0 && t != x) return t - 1.0;
    return t;
}

double karkain_ceil(double x) {
    double t = karkain_trunc(x);
    if (x > 0.0 && t != x) return t + 1.0;
    return t;
}

double karkain_round(double x) {
    if (x < 0.0) return -karkain_floor(-x + 0.5);
    return karkain_floor(x + 0.5);
}

double karkain_fmod(double x, double y) {
    if (y == 0.0) return karkain_nan();
    return x - karkain_trunc(x / y) * y;
}

double karkain_sqrt(double x) {
    if (x == 0.0) return 0.0;
    if (x < 0.0) return karkain_nan();

    /* Seed: normalize the mantissa to [1,2), halve the exponent, then Newton. */
    karkain_u64 v;
    v.d = x;
    long e = (long)((v.u >> 52) & 0x7FF) - 1023;
    v.u = (v.u & 0x800FFFFFFFFFFFFFull) | 0x3FF0000000000000ull;
    double m = v.d;

    double y;
    karkain_u64 t;
    if (e >= 0) {
        long k = e >> 1;
        double s = (e & 1) ? 1.4142135623730951 : 1.0;
        t.d = m * s;
        t.u += (uint64_t)k << 52;
        y = t.d;
    } else {
        long ab = -e;
        long k = ab >> 1;
        if (ab & 1) m *= 0.7071067811865476;
        t.d = m;
        t.u -= (uint64_t)k << 52;
        y = t.d;
    }

    for (int i = 0; i < 8; i++) {
        double ny = 0.5 * (y + x / y);
        if (ny == y) break;
        y = ny;
    }
    return y;
}

double karkain_exp(double x) {
    if (x > 709.0) return karkain_infpos();
    if (x < -745.0) return 0.0;

    /* Range-reduce through log2(e), Taylor on the remainder. */
    double y = x * 1.4426950408889634; /* log2(e) */
    double k = karkain_round(y);
    double f = y - k;
    double t = f * 0.6931471805599453; /* remainder in [-0.35, 0.35] */

    double sum = 1.0;
    double term = 1.0;
    for (int i = 1; i <= 11; i++) {
        term *= t / i;
        sum += term;
    }

    karkain_u64 v;
    v.d = sum;
    v.u += (uint64_t)((long)k) << 52;
    return v.d;
}

double karkain_log(double x) {
    if (x < 0.0) return karkain_nan();
    if (x == 0.0) return -karkain_infpos();

    karkain_u64 v;
    v.d = x;
    long e = (long)((v.u >> 52) & 0x7FF) - 1023;
    v.u = (v.u & 0x800FFFFFFFFFFFFFull) | 0x3FF0000000000000ull;
    double m = v.d; /* m in [1, 2) */

    /* ln(m) = 2*atanh((m-1)/(m+1)); |r| <= 1/3, series converges fast. */
    double r = (m - 1.0) / (m + 1.0);
    double r2 = r * r;
    double term = r;
    double sum = r;
    for (int i = 3; i < 21; i += 2) {
        term *= r2;
        sum += term / i;
    }

    return (double)e * 0.6931471805599453 + 2.0 * sum;
}

double karkain_pow(double x, double y) {
    if (y == 0.0) return 1.0;
    if (x == 1.0) return 1.0;
    if (x < 0.0) {
        double t = karkain_round(y);
        int negative = (t == y) && ((long)t % 2 != 0);
        double r = karkain_exp(y * karkain_log(-x));
        return negative ? -r : r;
    }
    return karkain_exp(y * karkain_log(x));
}