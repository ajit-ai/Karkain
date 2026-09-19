package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// minBootstrapAvailRAM is the default minimum available system memory required
// to run a self-hosted (kcc) bootstrap stage. Building the full src/compiler
// tree through the native compiler can approach ~2 GB RSS; requiring 1.5 GiB
// of headroom gives that process plus the gcc link step room to work while the
// guard stays effective on 4 GB hosts where the OOM/SEGFAULT class was
// documented (see docs/audit/PHASE-127-BOOTSTRAP-MEMORY-GUARD.md).
const minBootstrapAvailRAM = uint64(1536 * 1024 * 1024)

// errInsufficientMemory is wrapped by CheckBootstrapMemory so tests and
// callers can identify the OOM-guard failure with errors.Is.
var errInsufficientMemory = errors.New("insufficient memory for bootstrap stage")

// availableRAM reports available system memory in bytes. It is a var so tests
// can inject a deterministic probe on any platform. Each platform implementation
// must be dependency-free (stdlib only); unsupported platforms return an error
// from platformAvailableRAM, which disables the guard rather than guessing.
var availableRAM = platformAvailableRAM

// CheckBootstrapMemory returns an error when the currently available system
// memory is below the minimum required to run a native self-hosted compiler
// stage. It turns the documented low-RAM SEGFAULT/freeze class into a clean,
// actionable "error[K127]" rejection.
//
// The minimum defaults to 1.5 GiB. The KARKAIN_BOOTSTRAP_MIN_MEM environment
// variable overrides it and accepts a plain byte count or a K/M/G (or
// KiB/MiB/GiB) power-of-two suffix; setting it to 0 disables the guard.
//
// Stage 1 is intentionally not guarded: it drives the Go front end, which is
// comparatively light and is the definitionally-working bootstrap path.
func CheckBootstrapMemory(stage int) error {
	threshold := minBootstrapAvailRAM
	if raw := os.Getenv("KARKAIN_BOOTSTRAP_MIN_MEM"); raw != "" {
		v, err := parseMemBytes(raw)
		if err != nil {
			return fmt.Errorf("invalid KARKAIN_BOOTSTRAP_MIN_MEM %q: %w", raw, err)
		}
		threshold = v
	}
	if threshold == 0 {
		return nil
	}
	avail, err := availableRAM()
	if err != nil {
		// Without a memory probe we cannot guard; measured platforms are
		// windows (GlobalMemoryStatusEx) and linux (/proc/meminfo). Platforms
		// without a probe pass through.
		return nil
	}
	if avail < threshold {
		return fmt.Errorf("error[K127]: cannot run bootstrap stage %d: only %d MiB memory available, %d MiB required: %w (close memory-heavy apps, raise the page file, or set KARKAIN_BOOTSTRAP_MIN_MEM)",
			stage, avail/(1024*1024), threshold/(1024*1024), errInsufficientMemory)
	}
	return nil
}

// parseMemBytes parses a byte count from a string, accepting an optional
// power-of-two decimal suffix (K/M/G or KiB/MiB/GiB, case-insensitive).
func parseMemBytes(s string) (uint64, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, errors.New("empty value")
	}
	mult := uint64(1)
	u := strings.ToUpper(t)
	switch {
	case strings.HasSuffix(u, "GIB"):
		mult, u = 1<<30, strings.TrimSpace(u[:len(u)-3])
	case strings.HasSuffix(u, "MIB"):
		mult, u = 1<<20, strings.TrimSpace(u[:len(u)-3])
	case strings.HasSuffix(u, "KIB"):
		mult, u = 1<<10, strings.TrimSpace(u[:len(u)-3])
	case strings.HasSuffix(u, "G"):
		mult, u = 1<<30, strings.TrimSpace(u[:len(u)-1])
	case strings.HasSuffix(u, "M"):
		mult, u = 1<<20, strings.TrimSpace(u[:len(u)-1])
	case strings.HasSuffix(u, "K"):
		mult, u = 1<<10, strings.TrimSpace(u[:len(u)-1])
	}
	v, err := strconv.ParseUint(u, 10, 64)
	if err != nil {
		return 0, err
	}
	if v > ^uint64(0)/mult {
		return 0, errors.New("value overflows")
	}
	return v * mult, nil
}