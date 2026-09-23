//go:build !windows

package bootstrap

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

// trapExternalKill logs the circumstance of an external SIGTERM, then
// re-raises with default disposition so the exit code still reflects the
// external kill (143). The stage pointer tracks which closure step was
// active when the signal landed.
//
// The witness writes DIRECTLY to os.Stderr, never t.Logf: the test framework
// buffers t.Log output and a dying process never flushes it, which is
// exactly how the first two 143s arrived with no witness line.
func trapExternalKill(t *testing.T, stage *string) {
	t.Helper()
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGTERM)
	go func() {
		sig := <-ch
		fmt.Fprintf(os.Stderr, "SEED-CLOSURE: received %v during %s — external kill (manual cancel or runner preemption), not a test assertion; see the Actions run status (Cancelled vs Failed)\n", sig, *stage)
		signal.Reset(syscall.SIGTERM)
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	}()
	t.Cleanup(func() { signal.Stop(ch) })
}
