//go:build linux

package bootstrap

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

// platformAvailableRAM parses the MemAvailable line from /proc/meminfo.
// MemAvailable reflects reclaimable + free memory, which is a better estimate
// of what a child compiler can actually use than MemFree alone.
func platformAvailableRAM() (uint64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "MemAvailable:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, errors.New("malformed MemAvailable line")
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, err
		}
		return kb * 1024, nil
	}
	return 0, errors.New("MemAvailable not found in /proc/meminfo")
}