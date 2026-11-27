package codegen

// covRuntimeC is the opt-in function-coverage runtime for `karkain coverage`.
// It is emitted ONLY when Config.Coverage is true, so ordinary builds keep
// their exact historical bytes. Counts are keyed by Karkain function name
// (the same string karkain_frame_enter already receives) and dumped at exit
// to $KARKAIN_COV_OUT as deterministic "name\tcount" lines.
//
// This slice reports FUNCTION coverage, not line coverage: the generated C
// only carries a set_line at function entry, so claiming line coverage here
// would overstate what is measured.
func covRuntimeC() string {
	return `
/* v1.2.27 function-coverage runtime (opt-in) */
#define KARKAIN_COV_MAX 4096
typedef struct { const char* name; long long count; } karkain_cov_entry;
static karkain_cov_entry karkain_cov_table[KARKAIN_COV_MAX];
static int karkain_cov_count = 0;
static void karkain_cov_dump(void) {
    const char* out = getenv("KARKAIN_COV_OUT");
    int i;
    FILE* f;
    if (out == 0) return;
    f = fopen(out, "w");
    if (f == 0) return;
    for (i = 0; i < karkain_cov_count; i++) {
        fprintf(f, "%s\t%lld\n", karkain_cov_table[i].name, karkain_cov_table[i].count);
    }
    fclose(f);
}
static void karkain_cov_hit(const char* name) {
    static int registered = 0;
    int i;
    if (!registered) { atexit(karkain_cov_dump); registered = 1; }
    for (i = 0; i < karkain_cov_count; i++) {
        if (strcmp(karkain_cov_table[i].name, name) == 0) {
            karkain_cov_table[i].count++;
            return;
        }
    }
    if (karkain_cov_count < KARKAIN_COV_MAX) {
        karkain_cov_table[karkain_cov_count].name = name;
        karkain_cov_table[karkain_cov_count].count = 1;
        karkain_cov_count++;
    }
}
`
}
