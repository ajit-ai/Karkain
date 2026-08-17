package gpu

import (
	"fmt"
	"sync"
)

type UnifiedBuffer struct {
	Name     string
	Size     int
	HostData []byte
	dirty    bool
	mu       sync.RWMutex
}

func NewUnifiedBuffer(name string, size int) *UnifiedBuffer {
	return &UnifiedBuffer{
		Name:     name,
		Size:     size,
		HostData: make([]byte, size),
		dirty:    false,
	}
}

func (ub *UnifiedBuffer) Write(offset int, data []byte) error {
	ub.mu.Lock()
	defer ub.mu.Unlock()

	if offset < 0 || offset+len(data) > ub.Size {
		return fmt.Errorf("write out of bounds: offset=%d, len=%d, size=%d", offset, len(data), ub.Size)
	}

	copy(ub.HostData[offset:offset+len(data)], data)
	ub.dirty = true
	return nil
}

func (ub *UnifiedBuffer) Read(offset, length int) ([]byte, error) {
	ub.mu.RLock()
	defer ub.mu.RUnlock()

	if offset < 0 || offset+length > ub.Size {
		return nil, fmt.Errorf("read out of bounds: offset=%d, len=%d, size=%d", offset, length, ub.Size)
	}

	result := make([]byte, length)
	copy(result, ub.HostData[offset:offset+length])
	return result, nil
}

func (ub *UnifiedBuffer) SyncToDevice() error {
	ub.mu.Lock()
	defer ub.mu.Unlock()

	if !ub.dirty {
		return nil
	}

	ub.dirty = false
	return nil
}

func (ub *UnifiedBuffer) IsDirty() bool {
	ub.mu.RLock()
	defer ub.mu.RUnlock()
	return ub.dirty
}
