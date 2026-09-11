package codegen

import (
	"fmt"
	"strings"
)

// profNameTableC emits the function-id name table for the Phase 110 profiler.
// FIDs are assigned in source order by initProfiling and emitted in the same
// order, so the C side and the Go CLI interpret the identifiers identically.
func profNameTableC(names []string) string {
	var sb strings.Builder
	sb.WriteString("/* Phase 110: profile function name table (fid = array index). */\n")
	sb.WriteString("static const char* k_pf_names[] = {\n")
	for _, n := range names {
		fmt.Fprintf(&sb, "\t%q,\n", n)
	}
	sb.WriteString("\t0\n};\n")
	sb.WriteString("#define K_PF_NUM_FUNCS ((int)(sizeof(k_pf_names) / sizeof(k_pf_names[0])) - 1)\n")
	return sb.String()
}

// profRuntimeC returns the Phase 110 profiling runtime embedded in generated C
// when Config.Profiling is set. It is aggregation-based (every call is measured
// exactly, not sampled) and uses only bounded static memory so allocation
// metrics reflect the program rather than the profiler. On Windows the wall
// clock is QueryPerformanceCounter (declared directly to avoid pulling in
// windows.h); elsewhere CLOCK_MONOTONIC. The malloc/free wrappers intercept
// only allocations emitted in the generated user code below, never library
// headers or the preamble helpers above.
//
// The runtime emits a JSON document to the path in KARKAIN_PROF_OUT at process
// exit (via atexit, registered by karkain_prof_init in the generated main).
func profRuntimeC() string {
	return `/* =====================================================================
 * Phase 110: profiling runtime (emitted only when profiling is enabled).
 * Single-threaded, deterministic, bounded static memory, no malloc traffic.
 * ===================================================================== */
#if defined(_WIN32)
/* windows.h (already included by the C preamble) provides
 * LARGE_INTEGER and the QueryPerformanceCounter/Frequency prototypes. */
static long long karkain_prof_now_ns(void) {
    static LARGE_INTEGER k_pf_freq;
    static int k_pf_freq_ready = 0;
    LARGE_INTEGER k_pf_c;
    if (!k_pf_freq_ready) { QueryPerformanceFrequency(&k_pf_freq); k_pf_freq_ready = 1; }
    QueryPerformanceCounter(&k_pf_c);
    return (long long)((k_pf_c.QuadPart * 1000000000LL) / k_pf_freq.QuadPart);
}
#else
#include <time.h>
static long long karkain_prof_now_ns(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (long long)ts.tv_sec * 1000000000LL + (long long)ts.tv_nsec;
}
#endif

#define K_PF_MAX_FUNCS 512
#define K_PF_MAX_EDGES 4096
#define K_PF_MAX_PATHS 4096
#define K_PF_MAX_DEPTH 256
#define K_PF_MAX_PATH 32
#define K_PF_MAX_LIVE 8192

/* per-function counters (fid indexes k_pf_names) */
static int k_pf_count[K_PF_MAX_FUNCS];
static long long k_pf_total[K_PF_MAX_FUNCS];
static long long k_pf_min[K_PF_MAX_FUNCS];
static long long k_pf_max[K_PF_MAX_FUNCS];
static long long k_pf_child[K_PF_MAX_FUNCS]; /* inclusive time of direct calls */
static long long k_pf_prog_start;

/* call stack */
static int k_pf_stack_fid[K_PF_MAX_DEPTH];
static long long k_pf_stack_start[K_PF_MAX_DEPTH];
static int k_pf_stack_n;

/* call edges: caller -> callee */
static int k_pf_edge_caller[K_PF_MAX_EDGES];
static int k_pf_edge_callee[K_PF_MAX_EDGES];
static long long k_pf_edge_count[K_PF_MAX_EDGES];
static long long k_pf_edge_total[K_PF_MAX_EDGES];
static int k_pf_edge_n;

/* folded stack paths recorded at each completed call */
static int k_pf_path_fids[K_PF_MAX_PATHS][K_PF_MAX_PATH];
static int k_pf_path_len[K_PF_MAX_PATHS];
static long long k_pf_path_ns[K_PF_MAX_PATHS];
static int k_pf_path_n;

/* allocation metrics (malloc/free emitted in user code only) */
static long long k_pf_alloc_count;
static long long k_pf_alloc_bytes;
static long long k_pf_live_bytes;
static long long k_pf_peak_bytes;
static void* k_pf_live_ptr[K_PF_MAX_LIVE];
static long long k_pf_live_sz[K_PF_MAX_LIVE];
static int k_pf_live_n;

static volatile int k_pf_overflow;

static void karkain_prof_flush(void);

static void karkain_prof_init(void) {
    k_pf_prog_start = karkain_prof_now_ns();
    atexit(karkain_prof_flush);
}

static void karkain_prof_enter(int k_pf_fid) {
    long long k_pf_t;
    int k_pf_caller;
    int i;
    if (k_pf_overflow) return;
    if (k_pf_fid < 0 || k_pf_fid >= K_PF_NUM_FUNCS) return;
    k_pf_t = karkain_prof_now_ns();
    k_pf_caller = (k_pf_stack_n > 0) ? k_pf_stack_fid[k_pf_stack_n - 1] : -1;
    if (k_pf_stack_n >= K_PF_MAX_DEPTH) { k_pf_overflow = 1; return; }
    k_pf_stack_fid[k_pf_stack_n] = k_pf_fid;
    k_pf_stack_start[k_pf_stack_n] = k_pf_t;
    k_pf_stack_n++;
    k_pf_count[k_pf_fid]++;
    if (k_pf_caller < 0) return;
    i = 0;
    while (i < k_pf_edge_n && !(k_pf_edge_caller[i] == k_pf_caller && k_pf_edge_callee[i] == k_pf_fid)) i++;
    if (i == k_pf_edge_n) {
        if (k_pf_edge_n >= K_PF_MAX_EDGES) { k_pf_overflow = 1; return; }
        k_pf_edge_caller[i] = k_pf_caller;
        k_pf_edge_callee[i] = k_pf_fid;
        k_pf_edge_count[i] = 0;
        k_pf_edge_total[i] = 0;
        k_pf_edge_n++;
    }
    k_pf_edge_count[i]++;
}

static void karkain_prof_leave(int k_pf_fid) {
    long long k_pf_t, k_pf_dt, k_pf_t0;
    int k_pf_caller;
    int i, m;
    if (k_pf_overflow) return;
    if (k_pf_fid < 0 || k_pf_fid >= K_PF_NUM_FUNCS) return;
    if (k_pf_stack_n <= 0) return;
    if (k_pf_stack_fid[k_pf_stack_n - 1] != k_pf_fid) return; /* stack mismatch guard */
    k_pf_t = karkain_prof_now_ns();
    k_pf_t0 = k_pf_stack_start[k_pf_stack_n - 1];
    k_pf_caller = (k_pf_stack_n >= 2) ? k_pf_stack_fid[k_pf_stack_n - 2] : -1;
    k_pf_stack_n--;
    k_pf_dt = k_pf_t - k_pf_t0;
    if (k_pf_dt < 0) k_pf_dt = 0;
    k_pf_total[k_pf_fid] += k_pf_dt;
    if (k_pf_count[k_pf_fid] == 1) {
        k_pf_min[k_pf_fid] = k_pf_dt;
        k_pf_max[k_pf_fid] = k_pf_dt;
    } else {
        if (k_pf_dt < k_pf_min[k_pf_fid]) k_pf_min[k_pf_fid] = k_pf_dt;
        if (k_pf_dt > k_pf_max[k_pf_fid]) k_pf_max[k_pf_fid] = k_pf_dt;
    }
    if (k_pf_caller >= 0) {
        k_pf_child[k_pf_caller] += k_pf_dt;
        i = 0;
        while (i < k_pf_edge_n && !(k_pf_edge_caller[i] == k_pf_caller && k_pf_edge_callee[i] == k_pf_fid)) i++;
        if (i < k_pf_edge_n) k_pf_edge_total[i] += k_pf_dt;
    }
    /* folded path: current stack plus the leaving frame */
    {
        int k_pf_cap = k_pf_stack_n + 1;
        int k_pf_pidx = 0;
        if (k_pf_cap > K_PF_MAX_PATH) k_pf_cap = K_PF_MAX_PATH;
        for (i = 0; i < k_pf_path_n; i++) {
            if (k_pf_path_len[i] == k_pf_cap) {
                int k_pf_ok = 1;
                for (m = 0; m < k_pf_cap; m++) {
                    int k_pf_sid = (m == k_pf_stack_n) ? k_pf_fid : k_pf_stack_fid[m];
                    if (k_pf_path_fids[i][m] != k_pf_sid) { k_pf_ok = 0; break; }
                }
                if (k_pf_ok) { k_pf_path_ns[i] += k_pf_dt; k_pf_pidx = 1; break; }
            }
        }
        if (!k_pf_pidx) {
            if (k_pf_path_n >= K_PF_MAX_PATHS) { k_pf_overflow = 1; return; }
            for (m = 0; m < k_pf_cap; m++) {
                k_pf_path_fids[k_pf_path_n][m] = (m == k_pf_stack_n) ? k_pf_fid : k_pf_stack_fid[m];
            }
            k_pf_path_len[k_pf_path_n] = k_pf_cap;
            k_pf_path_ns[k_pf_path_n] = k_pf_dt;
            k_pf_path_n++;
        }
    }
}

/* allocation interception for allocations emitted in generated user code */
#undef malloc
#undef free
static void* karkain_prof_malloc(size_t k_pf_n) {
    long long k_pf_sz = (long long)k_pf_n;
    void* k_pf_p;
    k_pf_alloc_count++;
    k_pf_alloc_bytes += k_pf_sz;
    k_pf_p = malloc(k_pf_n);
    if (k_pf_p != 0) {
        if (k_pf_live_n < K_PF_MAX_LIVE) {
            k_pf_live_ptr[k_pf_live_n] = k_pf_p;
            k_pf_live_sz[k_pf_live_n] = k_pf_sz;
            k_pf_live_n++;
            k_pf_live_bytes += k_pf_sz;
            if (k_pf_live_bytes > k_pf_peak_bytes) k_pf_peak_bytes = k_pf_live_bytes;
        } else {
            k_pf_overflow = 1;
        }
    }
    return k_pf_p;
}
static void karkain_prof_free(void* k_pf_p) {
    int i;
    if (k_pf_p != 0) {
        for (i = 0; i < k_pf_live_n; i++) {
            if (k_pf_live_ptr[i] == k_pf_p) {
                k_pf_live_bytes -= k_pf_live_sz[i];
                k_pf_live_ptr[i] = k_pf_live_ptr[k_pf_live_n - 1];
                k_pf_live_sz[i] = k_pf_live_sz[k_pf_live_n - 1];
                k_pf_live_n--;
                break;
            }
        }
    }
    free(k_pf_p);
}
#define malloc karkain_prof_malloc
#define free karkain_prof_free

static void karkain_prof_flush(void) {
    const char* k_pf_out;
    FILE* k_pf_f;
    int i, m;
    long long k_pf_dur;
    int k_pf_first;
    long long k_pf_self;
    long long k_pf_avg;
    if (!k_pf_prog_start) return;
    k_pf_out = getenv("KARKAIN_PROF_OUT");
    if (k_pf_out == 0 || *k_pf_out == 0) return;
    k_pf_f = fopen(k_pf_out, "w");
    if (!k_pf_f) return;
    k_pf_dur = karkain_prof_now_ns() - k_pf_prog_start;
    if (k_pf_dur < 0) k_pf_dur = 0;
    fprintf(k_pf_f, "{\n");
    fprintf(k_pf_f, "  \"duration_ns\": %lld,\n", k_pf_dur);
    fprintf(k_pf_f, "  \"overflow\": %d,\n", (int)k_pf_overflow);
    fprintf(k_pf_f, "  \"allocation\": {\n");
    fprintf(k_pf_f, "    \"count\": %lld,\n", k_pf_alloc_count);
    fprintf(k_pf_f, "    \"bytes\": %lld,\n", k_pf_alloc_bytes);
    fprintf(k_pf_f, "    \"peak_bytes\": %lld\n", k_pf_peak_bytes);
    fprintf(k_pf_f, "  },\n");
    fprintf(k_pf_f, "  \"functions\": [\n");
    k_pf_first = 1;
    for (i = 0; i < K_PF_NUM_FUNCS; i++) {
        if (k_pf_count[i] == 0 && k_pf_total[i] == 0) continue;
        k_pf_self = k_pf_total[i] - k_pf_child[i];
        if (k_pf_self < 0) k_pf_self = 0;
        k_pf_avg = (k_pf_count[i] > 0) ? (k_pf_total[i] / k_pf_count[i]) : 0;
        if (!k_pf_first) fprintf(k_pf_f, ",\n");
        k_pf_first = 0;
        fprintf(k_pf_f,
            "    {\"name\": \"%s\", \"calls\": %d, \"total_ns\": %lld, \"self_ns\": %lld, \"min_ns\": %lld, \"max_ns\": %lld, \"avg_ns\": %lld}",
            k_pf_names[i], k_pf_count[i], k_pf_total[i], k_pf_self, k_pf_min[i], k_pf_max[i], k_pf_avg);
    }
    fprintf(k_pf_f, "\n  ],\n");
    fprintf(k_pf_f, "  \"calls\": [\n");
    k_pf_first = 1;
    for (i = 0; i < k_pf_edge_n; i++) {
        if (!k_pf_first) fprintf(k_pf_f, ",\n");
        k_pf_first = 0;
        fprintf(k_pf_f,
            "    {\"caller\": \"%s\", \"callee\": \"%s\", \"count\": %lld, \"total_ns\": %lld}",
            k_pf_names[k_pf_edge_caller[i]], k_pf_names[k_pf_edge_callee[i]],
            k_pf_edge_count[i], k_pf_edge_total[i]);
    }
    fprintf(k_pf_f, "\n  ],\n");
    fprintf(k_pf_f, "  \"folded\": [\n");
    k_pf_first = 1;
    for (i = 0; i < k_pf_path_n; i++) {
        if (!k_pf_first) fprintf(k_pf_f, ",\n");
        k_pf_first = 0;
        fprintf(k_pf_f, "    {\"path\": \"");
        for (m = 0; m < k_pf_path_len[i]; m++) {
            if (m > 0) fprintf(k_pf_f, ";");
            fprintf(k_pf_f, "%s", k_pf_names[k_pf_path_fids[i][m]]);
        }
        fprintf(k_pf_f, "\", \"ns\": %lld}", k_pf_path_ns[i]);
    }
    fprintf(k_pf_f, "\n  ]\n}\n");
    fclose(k_pf_f);
}
`
}
