//go:build windows

package bootstrap

import "testing"

// trapExternalKill is a no-op on Windows: the OS has no SIGTERM, so an
// exit-143-style external kill cannot occur there. The gate's K127 guard
// and skip paths already cover the Windows failure modes.
func trapExternalKill(t *testing.T, stage *string) {
	t.Helper()
}
