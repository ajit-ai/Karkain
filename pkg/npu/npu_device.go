package npu

import (
	"os"
	"runtime"
	"strings"
)

// DeviceDetector discovers NPU hardware on the host. Detection is
// environment-capability-based (no vendor SDK is required to run Karkain);
// a vendor adapter is only reported when the corresponding env/opt-in flag
// or platform match is present. This keeps detection deterministic and
// free of vendor-specific linking.
type DeviceDetector struct {
	// registered adapters (Vendor -> detect func)
	adapters map[Vendor]func() []NPUDevice
}

// NewDeviceDetector builds a detector with the standard vendor adapters.
func NewDeviceDetector() *DeviceDetector {
	d := &DeviceDetector{adapters: make(map[Vendor]func() []NPUDevice)}
	d.register(VendorIntel, detectIntel)
	d.register(VendorQualcomm, detectQualcomm)
	d.register(VendorApple, detectApple)
	d.register(VendorAMD, detectAMD)
	d.register(VendorArm, detectArm)
	return d
}

func (d *DeviceDetector) register(v Vendor, fn func() []NPUDevice) {
	d.adapters[v] = fn
}

// Register lets callers inject an adapter (used by tests and future vendors).
func (d *DeviceDetector) Register(v Vendor, fn func() []NPUDevice) {
	d.register(v, fn)
}

// Detect returns all devices across registered adapters.
func (d *DeviceDetector) Detect() []NPUDevice {
	var out []NPUDevice
	// Deterministic vendor order.
	for _, v := range []Vendor{VendorIntel, VendorQualcomm, VendorApple, VendorAMD, VendorArm} {
		if fn, ok := d.adapters[v]; ok {
			out = append(out, fn()...)
		}
	}
	return out
}

// DetectByVendor returns devices for a single vendor.
func (d *DeviceDetector) DetectByVendor(v Vendor) []NPUDevice {
	if fn, ok := d.adapters[v]; ok {
		return fn()
	}
	return nil
}

// envOn reports whether an opt-in environment variable is truthy.
func envOn(key string) bool {
	v := strings.TrimSpace(os.Getenv(key))
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on", "enabled":
		return true
	}
	return false
}

// detectIntel reports an Intel NPU (e.g., AI Boost / Neural Compute Stick)
// only when KARKAIN_NPU_INTEL is set, since there is no portable way to probe
// Intel NPU silicon without the OpenVINO runtime.
func detectIntel() []NPUDevice {
	if !envOn("KARKAIN_NPU_INTEL") {
		return nil
	}
	return []NPUDevice{{
		Name:       "Intel AI Boost (OpenVINO)",
		Vendor:     VendorIntel,
		VendorName: "intel",
		Capabilities: NewCPUCapabilities(),
		Memory:     1 << 28, // 256 MiB scratch
		Version:    "v1",
	}}
}

// detectQualcomm reports a Qualcomm Hexagon NPU (Snapdragon HTP/QNN) on
// Windows/Linux only when opt-in; on other platforms it is unavailable.
func detectQualcomm() []NPUDevice {
	if !envOn("KARKAIN_NPU_QUALCOMM") {
		return nil
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		return []NPUDevice{{
			Name:       "Qualcomm Hexagon HTP (QNN)",
			Vendor:     VendorQualcomm,
			VendorName: "qualcomm",
			Capabilities: NewCPUCapabilities(),
			Memory:     1 << 26, // 64 MiB
			Version:    "v2.24",
		}}
	}
	return nil
}

// detectApple reports the Apple Neural Engine via CoreML on darwin.
func detectApple() []NPUDevice {
	if !envOn("KARKAIN_NPU_APPLE") {
		return nil
	}
	if runtime.GOOS == "darwin" {
		return []NPUDevice{{
			Name:       "Apple Neural Engine (CoreML)",
			Vendor:     VendorApple,
			VendorName: "apple",
			Capabilities: NewCPUCapabilities(),
			Memory:     1 << 28,
			Version:    "ANE",
		}}
	}
	return nil
}

// detectAMD reports an AMD NPU (e.g., Ryzen AI / XDNA via VitisAI).
func detectAMD() []NPUDevice {
	if !envOn("KARKAIN_NPU_AMD") {
		return nil
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		return []NPUDevice{{
			Name:       "AMD XDNA (VitisAI)",
			Vendor:     VendorAMD,
			VendorName: "amd",
			Capabilities: NewCPUCapabilities(),
			Memory:     1 << 28,
			Version:    "xDNA1",
		}}
	}
	return nil
}

// detectArm reports an Arm Ethos-U NPU in embedded Linux environments.
func detectArm() []NPUDevice {
	if !envOn("KARKAIN_NPU_ARM") {
		return nil
	}
	if runtime.GOOS == "linux" {
		return []NPUDevice{{
			Name:       "Arm Ethos-U65",
			Vendor:     VendorArm,
			VendorName: "arm",
			Capabilities: NewCPUCapabilities(),
			Memory:     1 << 24, // 16 MiB
			Version:    "ethos-u65",
		}}
	}
	return nil
}
