// v1.2.25 - Memory safety Phase 3: Arena Regions runtime foundation.
package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhaseV1225_ArenaRegions(t *testing.T) {
	dir, err := os.MkdirTemp("", "v1225arena")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	src := filepath.Join(dir, "main.c")
	prog := `#include "karkain_arena.h"
#include <stdio.h>
#include <string.h>

int main(void) {
    karkain_region_t *r = karkain_region_new(128);
    printf("cap=%zu\n", karkain_region_capacity(r));
    int *a = (int *)karkain_region_alloc(r, sizeof(int));
    *a = 7;
    int *b = (int *)karkain_region_alloc(r, sizeof(int));
    *b = 9;
    printf("used_ge=%d\n", (int)(karkain_region_used(r) >= 32));
    printf("a=%d b=%d\n", *a, *b);
    // bulk reset invalidates prior pointers logically; re-alloc reuses space
    karkain_region_reset(r);
    printf("used_after_reset=%zu\n", karkain_region_used(r));
    void *p = karkain_region_alloc(r, 10000); // exceeds capacity
    printf("exhausted=%d\n", p == 0 ? 1 : 0);
    karkain_region_free(r);
    printf("freed_ok=1\n");
    return 0;
}
`
	if err := os.WriteFile(src, []byte(prog), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "arena.exe")
	cmd := exec.Command("gcc", "-I", "F:/Codes/Git/Karkain/runtime/arena/c", src,
		"F:/Codes/Git/Karkain/runtime/arena/c/karkain_arena.c", "-o", out, "-std=c11", "-O1")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile failed: %v\n%s", err, string(b))
	}
	run := exec.Command(out)
	got, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, string(got))
	}
	want := []string{"cap=128", "used_ge=1", "a=7 b=9", "used_after_reset=0", "exhausted=1", "freed_ok=1"}
	raw := string(got)
	for _, w := range want {
		if !strings.Contains(raw, w) {
			t.Fatalf("missing %q in output:\n%s", w, raw)
		}
	}
}