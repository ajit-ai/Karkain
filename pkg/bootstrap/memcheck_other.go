//go:build !windows && !linux

package bootstrap

import "errors"

// platformAvailableRAM is unsupported on this platform. The guard is disabled
// rather than guessing about memory the process cannot reliably observe; the
// documented OOM class only manifests on the measured windows/linux hosts.
func platformAvailableRAM() (uint64, error) {
	return 0, errors.New("memory probe not implemented on this platform")
}