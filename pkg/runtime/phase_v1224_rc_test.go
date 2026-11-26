// v1.2.24 - Memory safety Phase 2: Reference Counting (RC) runtime foundation.
//
// Karkain-owned reference-counted box runtime under runtime/rc/c. This slice
// delivers the runtime primitives (retain/release/strong_count/payload) with a
// thread-safe atomic refcount, compiled and exercised directly against the C
// toolchain. Compiler integration (automatic retain/release insertion, an
// Rc[T] surface, borrow-checker integration) is the follow-up slice of Phase 2.
package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhaseV1224_RCPrimitives(t *testing.T) {
	dir, err := os.MkdirTemp("", "v1224rc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	src := filepath.Join(dir, "main.c")
	prog := `#include "karkain_rc.h"
#include <stdio.h>
#include <stdlib.h>

static void free_int(void *p) { free(p); }

int main(void) {
    int *v = (int *)malloc(sizeof(int));
    *v = 42;
    karkain_rc_box_t *b = karkain_rc_new(v, free_int);
    printf("new_count=%ld\n", karkain_rc_strong_count(b));
    karkain_rc_box_t *b2 = b;
    printf("retain_count=%ld\n", karkain_rc_retain(b2));
    printf("same_payload=%d\n", (int)(karkain_rc_payload(b) == karkain_rc_payload(b2)));
    printf("release1=%ld\n", karkain_rc_release(b));
    printf("release2=%ld\n", karkain_rc_release(b2));
    printf("null_count=%ld\n", karkain_rc_strong_count(0));
    printf("null_retain=%ld\n", karkain_rc_retain(0));
    printf("null_release=%ld\n", karkain_rc_release(0));
    return 0;
}
`
	if err := os.WriteFile(src, []byte(prog), 0644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "rc.exe")
	cmd := exec.Command("gcc", "-I", "F:/Codes/Git/Karkain/runtime/rc/c", src,
		"F:/Codes/Git/Karkain/runtime/rc/c/karkain_rc.c", "-o", out, "-std=c11", "-O1")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile failed: %v\n%s", err, string(b))
	}
	run := exec.Command(out)
	got, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, string(got))
	}
	want := "new_count=1\nretain_count=2\nsame_payload=1\nrelease1=1\nrelease2=0\nnull_count=0\nnull_retain=0\nnull_release=0"
	if strings.Join(strings.Fields(string(got)), "\n") != strings.Join(strings.Fields(want), "\n") {
		t.Fatalf("output mismatch:\ngot:\n%s\nwant:\n%s", string(got), want)
	}
	_ = strings.TrimSpace
}