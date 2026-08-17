package gpu

import (
	"sync"
	"testing"
)

func TestUnifiedMemory_AllocationAndSync(t *testing.T) {
	buf := NewUnifiedBuffer("test_buf", 1024)

	if buf.Size != 1024 {
		t.Fatalf("expected size 1024, got %d", buf.Size)
	}
	if len(buf.HostData) != 1024 {
		t.Fatalf("expected HostData length 1024, got %d", len(buf.HostData))
	}
	if buf.IsDirty() {
		t.Fatal("buffer should not be dirty after creation")
	}

	data := []byte{1, 2, 3, 4, 5}
	err := buf.Write(0, data)
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if !buf.IsDirty() {
		t.Fatal("buffer should be dirty after write")
	}

	err = buf.SyncToDevice()
	if err != nil {
		t.Fatalf("unexpected sync error: %v", err)
	}
	if buf.IsDirty() {
		t.Fatal("buffer should not be dirty after sync")
	}

	readBack, err := buf.Read(0, 5)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	for i, b := range readBack {
		if b != data[i] {
			t.Errorf("readBack[%d] = %d, want %d", i, b, data[i])
		}
	}
}

func TestDeviceManager_Discovery(t *testing.T) {
	dm := NewDeviceManager()

	descriptors := dm.GetDescriptors()
	if len(descriptors) == 0 {
		t.Fatal("expected at least one device descriptor")
	}

	foundCPU := false
	for _, d := range descriptors {
		if d.Backend == BackendCPU {
			foundCPU = true
			break
		}
	}
	if !foundCPU {
		t.Error("expected BackendCPU in default descriptors")
	}

	cpuDevices := dm.GetByBackend(BackendCPU)
	if len(cpuDevices) == 0 {
		t.Error("expected at least one CPU device")
	}
}

func TestUnifiedBuffer_Concurrency(t *testing.T) {
	buf := NewUnifiedBuffer("concurrent_buf", 4096)

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			data := byte(offset % 256)
			payload := []byte{data, data, data, data}
			_ = buf.Write(offset*4, payload)
		}(i)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			_, _ = buf.Read(offset*4, 4)
		}(i)
	}

	wg.Wait()

	err := buf.SyncToDevice()
	if err != nil {
		t.Fatalf("unexpected sync error: %v", err)
	}
}
