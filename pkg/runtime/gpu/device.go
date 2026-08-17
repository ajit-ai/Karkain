package gpu

type Backend string

const (
	BackendCPU   Backend = "cpu"
	BackendCUDA  Backend = "cuda"
	BackendVulkan Backend = "vulkan"
	BackendWebGPU Backend = "webgpu"
)

type DeviceDescriptor struct {
	Name    string
	Backend Backend
	Memory  uint64
	Units   int
}

type DeviceManager struct {
	descriptors []DeviceDescriptor
}

func NewDeviceManager() *DeviceManager {
	return &DeviceManager{
		descriptors: []DeviceDescriptor{
			{Name: "CPU Fallback", Backend: BackendCPU, Memory: 0, Units: 1},
			{Name: "CUDA Default", Backend: BackendCUDA, Memory: 4 * 1024 * 1024 * 1024, Units: 128},
			{Name: "Vulkan Default", Backend: BackendVulkan, Memory: 2 * 1024 * 1024 * 1024, Units: 64},
			{Name: "WebGPU Default", Backend: BackendWebGPU, Memory: 1 * 1024 * 1024 * 1024, Units: 32},
		},
	}
}

func (dm *DeviceManager) GetDescriptors() []DeviceDescriptor {
	return dm.descriptors
}

func (dm *DeviceManager) GetByBackend(backend Backend) []DeviceDescriptor {
	var result []DeviceDescriptor
	for _, d := range dm.descriptors {
		if d.Backend == backend {
			result = append(result, d)
		}
	}
	return result
}
