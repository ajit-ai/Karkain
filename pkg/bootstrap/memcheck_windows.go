//go:build windows

package bootstrap

import (
	"syscall"
	"unsafe"
)

// memoryStatusEx mirrors the WIN32_MEMORY_STATUS_EX structure consumed by
// GlobalMemoryStatusEx (kernel32.dll). Field order and sizes must match the
// native layout exactly; the caller sets Length before the call.
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// platformAvailableRAM reports current available physical memory in bytes on
// Windows via GlobalMemoryStatusEx. It uses only the stdlib syscall package
// (no cgo, no external dependency).
func platformAvailableRAM() (uint64, error) {
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	r1, _, err := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&ms)))
	if r1 == 0 {
		return 0, err
	}
	return ms.AvailPhys, nil
}