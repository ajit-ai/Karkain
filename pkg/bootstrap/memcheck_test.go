package bootstrap

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestParseMemBytes(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"0", 0},
		{"1048576", 1048576},
		{"1536MiB", 1536 * 1024 * 1024},
		{"1536mib", 1536 * 1024 * 1024},
		{"2G", 2 << 30},
		{"512K", 512 << 10},
		{"2 GiB", 2 << 30},
		{"128 KiB", 128 << 10},
	}
	for _, c := range cases {
		got, err := parseMemBytes(c.in)
		if err != nil {
			t.Errorf("parseMemBytes(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseMemBytes(%q) = %d, want %d", c.in, got, c.want)
		}
	}
	for _, bad := range []string{"", "12GBX", "abc", "1.5GiB", "-1"} {
		if _, err := parseMemBytes(bad); err == nil {
			t.Errorf("parseMemBytes(%q) expected error, got none", bad)
		}
	}
}

// TestCheckBootstrapMemoryGuard verifies the guard deterministically on any
// host by injecting a fake 512 MiB available-memory probe. Host RAM is never
// depended upon (max/0 thresholds are decision-stable on every platform).
func TestCheckBootstrapMemoryGuard(t *testing.T) {
	old := availableRAM
	availableRAM = func() (uint64, error) { return 512 * 1024 * 1024, nil } // 512 MiB
	defer func() { availableRAM = old }()

	orig := os.Getenv("KARKAIN_BOOTSTRAP_MIN_MEM")
	defer os.Setenv("KARKAIN_BOOTSTRAP_MIN_MEM", orig)

	setEnv := func(v string) { os.Setenv("KARKAIN_BOOTSTRAP_MIN_MEM", v) }

	// Disabled: threshold 0 must always pass.
	setEnv("0")
	if err := CheckBootstrapMemory(2); err != nil {
		t.Errorf("threshold 0 should disable the guard, got %v", err)
	}

	// Below threshold: max uint64 can never be satisfied by 512 MiB.
	setEnv("18446744073709551615")
	err := CheckBootstrapMemory(3)
	if err == nil {
		t.Fatal("max threshold with 512 MiB available must fail")
	}
	if !strings.Contains(err.Error(), "error[K127]") {
		t.Errorf("error must carry error[K127], got: %v", err)
	}
	if !strings.Contains(err.Error(), "stage 3") {
		t.Errorf("error must name the stage, got: %v", err)
	}
	if !errors.Is(err, errInsufficientMemory) {
		t.Errorf("error must wrap errInsufficientMemory: %v", err)
	}

	// Above threshold: 256 MiB < available 512 MiB -> pass.
	setEnv("256M")
	if err := CheckBootstrapMemory(2); err != nil {
		t.Errorf("256 MiB against 512 MiB available should pass, got %v", err)
	}

	// Invalid threshold: reported, never swallowed.
	setEnv("notabytes")
	if err := CheckBootstrapMemory(2); err == nil || !strings.Contains(err.Error(), "invalid KARKAIN_BOOTSTRAP_MIN_MEM") {
		t.Errorf("invalid env must be reported, got %v", err)
	}
}

// TestCheckBootstrapMemoryNoProbe covers platforms where the probe is
// unavailable: the guard must pass through (disabled), not fail the build.
func TestCheckBootstrapMemoryNoProbe(t *testing.T) {
	old := availableRAM
	availableRAM = func() (uint64, error) { return 0, errors.New("no probe") }
	defer func() { availableRAM = old }()

	orig := os.Getenv("KARKAIN_BOOTSTRAP_MIN_MEM")
	defer os.Setenv("KARKAIN_BOOTSTRAP_MIN_MEM", orig)
	os.Setenv("KARKAIN_BOOTSTRAP_MIN_MEM", "18446744073709551615")

	if err := CheckBootstrapMemory(2); err != nil {
		t.Errorf("platform without a probe must pass through, got %v", err)
	}
}