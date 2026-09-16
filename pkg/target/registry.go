package target

import (
	"fmt"
	"sort"
	"sync"
)

// Registry is the mutable catalog of compute targets available to the
// compiler target selection/dispatch layer. Registration is additive; every
// register call keeps deterministic ordering (sorted by name).
type Registry struct {
	mu     sync.RWMutex
	byName map[string]*ComputeTarget
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]*ComputeTarget)}
}

// Register adds ct and returns an error when the name is already taken.
func (r *Registry) Register(ct *ComputeTarget) error {
	if ct == nil || ct.Name == "" {
		return fmt.Errorf("compute target requires a non-empty name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[ct.Name]; exists {
		return fmt.Errorf("compute target %q already registered", ct.Name)
	}
	r.byName[ct.Name] = ct
	return nil
}

// Lookup returns the compute target registered under name, or nil.
func (r *Registry) Lookup(name string) *ComputeTarget {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byName[name]
}

// All returns every registered compute target sorted by name.
func (r *Registry) All() []*ComputeTarget {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*ComputeTarget, 0, len(r.byName))
	for _, ct := range r.byName {
		out = append(out, ct)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ByFamily returns the registered targets of one family sorted by name.
func (r *Registry) ByFamily(family string) []*ComputeTarget {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*ComputeTarget
	for _, ct := range r.byName {
		if ct.Family == family {
			out = append(out, ct)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// defaultRegistry is the process-wide catalog of Phase-124 compute targets.
var (
	defaultRegistryOnce sync.Once
	defaultRegistry     *Registry
)

// ensureDefaultRegistry registers the built-in compute targets exactly once:
// the implemented scalar executors (cpu, simd, wasm32-wasi) and the three
// Phase-124 experimental implementations (gpu, npu, quantum).
func ensureDefaultRegistry() {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewRegistry()
		for _, ct := range []*ComputeTarget{
			cpuTarget(),
			simdTarget(),
			wasmTarget(),
			gpuTarget(),
			npuTarget(),
			quantumTarget(),
		} {
			// Built-in names are fixed at startup; registration cannot fail.
			_ = defaultRegistry.Register(ct)
		}
	})
}

// RegisterComputeTarget adds a compute target to the process-wide catalog.
func RegisterComputeTarget(ct *ComputeTarget) error {
	ensureDefaultRegistry()
	return defaultRegistry.Register(ct)
}

// ComputeTargets returns the process-wide catalog sorted by name.
func ComputeTargets() []*ComputeTarget {
	ensureDefaultRegistry()
	return defaultRegistry.All()
}

// LookupComputeTarget returns the process-wide compute target named name.
func LookupComputeTarget(name string) *ComputeTarget {
	ensureDefaultRegistry()
	return defaultRegistry.Lookup(name)
}

// ComputeTargetByFamily returns the process-wide catalog for one family.
func ComputeTargetByFamily(family string) []*ComputeTarget {
	ensureDefaultRegistry()
	return defaultRegistry.ByFamily(family)
}

// IsComputeTargetName reports whether name is a registered compute target.
func IsComputeTargetName(name string) bool {
	return LookupComputeTarget(name) != nil
}