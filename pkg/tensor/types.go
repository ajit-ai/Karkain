// Package tensor implements Karkain's Tensor IR — a Karkain-owned intermediate
// representation for tensor computation. Tensor IR sits above the SSA IR and
// below the execution backends (CPU, GPU, NPU).
//
// Tensor IR is NOT NumPy, NOT PyTorch, NOT a vendor format.
// It is Karkain's own representation of tensor computation.
package tensor

import (
	"fmt"
	"strings"
)

// ============================================================
// Element Types
// ============================================================

// ElemType represents the element type of a tensor.
type ElemType int

const (
	ElemUnknown ElemType = iota
	ElemI32
	ElemI64
	ElemF32
	ElemF64
	ElemBool
)

// String returns the name of the element type.
func (t ElemType) String() string {
	switch t {
	case ElemI32:
		return "i32"
	case ElemI64:
		return "i64"
	case ElemF32:
		return "f32"
	case ElemF64:
		return "f64"
	case ElemBool:
		return "bool"
	default:
		return "unknown"
	}
}

// ByteSize returns the size in bytes of one element.
func (t ElemType) ByteSize() int {
	switch t {
	case ElemI32, ElemF32, ElemBool:
		return 4
	case ElemI64, ElemF64:
		return 8
	default:
		return 0
	}
}

// IsNumeric returns true if the type is numeric.
func (t ElemType) IsNumeric() bool {
	return t == ElemI32 || t == ElemI64 || t == ElemF32 || t == ElemF64
}

// IsFloat returns true if the type is floating-point.
func (t ElemType) IsFloat() bool {
	return t == ElemF32 || t == ElemF64
}

// IsInteger returns true if the type is integer.
func (t ElemType) IsInteger() bool {
	return t == ElemI32 || t == ElemI64
}

// ============================================================
// Dimension
// ============================================================

// DimKind represents the kind of a dimension.
type DimKind int

const (
	DimStatic DimKind = iota // known at compile time
	DimDynamic              // runtime value (stored as negative marker)
	DimSymbolic             // symbolic variable (stored as name)
)

// Dimension represents a single dimension of a shape.
type Dimension struct {
	Kind DimKind
	Size int    // for DimStatic
	Name string // for DimSymbolic
}

// StaticDim creates a static dimension.
func StaticDim(size int) Dimension {
	return Dimension{Kind: DimStatic, Size: size}
}

// DynamicDim creates a dynamic dimension.
func DynamicDim() Dimension {
	return Dimension{Kind: DimDynamic, Size: -1}
}

// SymbolicDim creates a symbolic dimension.
func SymbolicDim(name string) Dimension {
	return Dimension{Kind: DimSymbolic, Name: name}
}

// IsStatic returns true if the dimension is known at compile time.
func (d Dimension) IsStatic() bool { return d.Kind == DimStatic }

// IsDynamic returns true if the dimension is dynamic.
func (d Dimension) IsDynamic() bool { return d.Kind == DimDynamic }

// IsSymbolic returns true if the dimension is symbolic.
func (d Dimension) IsSymbolic() bool { return d.Kind == DimSymbolic }

// String returns a human-readable representation.
func (d Dimension) String() string {
	switch d.Kind {
	case DimStatic:
		return fmt.Sprintf("%d", d.Size)
	case DimDynamic:
		return "?"
	case DimSymbolic:
		return d.Name
	default:
		return "??"
	}
}

// ============================================================
// Shape
// ============================================================

// Shape represents the shape of a tensor.
type Shape []Dimension

// NewShape creates a shape from static dimensions.
func NewShape(dims ...int) Shape {
	s := make(Shape, len(dims))
	for i, d := range dims {
		s[i] = StaticDim(d)
	}
	return s
}

// Rank returns the number of dimensions.
func (s Shape) Rank() int { return len(s) }

// IsScalar returns true if the shape is [] (rank 0).
func (s Shape) IsScalar() bool { return len(s) == 0 }

// IsVector returns true if the shape is [n].
func (s Shape) IsVector() bool { return len(s) == 1 }

// IsMatrix returns true if the shape is [m, n].
func (s Shape) IsMatrix() bool { return len(s) == 2 }

// NumElements returns the total number of elements (for static shapes).
// Returns -1 if any dimension is dynamic/symbolic.
func (s Shape) NumElements() int {
	if len(s) == 0 {
		return 1
	}
	result := 1
	for _, d := range s {
		if !d.IsStatic() {
			return -1
		}
		result *= d.Size
	}
	return result
}

// IsStatic returns true if all dimensions are static.
func (s Shape) IsStatic() bool {
	for _, d := range s {
		if !d.IsStatic() {
			return false
		}
	}
	return true
}

// IsCompatible returns true if two shapes are compatible for element-wise ops.
func (s Shape) IsCompatible(other Shape) bool {
	if s.Rank() != other.Rank() {
		return false
	}
	for i := range s {
		if s[i].IsStatic() && other[i].IsStatic() && s[i].Size != other[i].Size {
			return false
		}
	}
	return true
}

// String returns a human-readable representation.
func (s Shape) String() string {
	if len(s) == 0 {
		return "()"
	}
	parts := make([]string, len(s))
	for i, d := range s {
		parts[i] = d.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// Equals returns true if two shapes are exactly equal.
func (s Shape) Equals(other Shape) bool {
	if len(s) != len(other) {
		return false
	}
	for i := range s {
		if s[i].Kind != other[i].Kind {
			return false
		}
		switch s[i].Kind {
		case DimStatic:
			if s[i].Size != other[i].Size {
				return false
			}
		case DimSymbolic:
			if s[i].Name != other[i].Name {
				return false
			}
		}
	}
	return true
}

// ============================================================
// TensorType
// ============================================================

// TensorType represents the full type of a tensor.
type TensorType struct {
	Elem ElemType
	Shape Shape
}

// NewTensorType creates a tensor type.
func NewTensorType(elem ElemType, shape Shape) TensorType {
	return TensorType{Elem: elem, Shape: shape}
}

// Rank returns the rank of the tensor.
func (t TensorType) Rank() int { return t.Shape.Rank() }

// NumElements returns the total number of elements.
func (t TensorType) NumElements() int { return t.Shape.NumElements() }

// IsScalar returns true if rank 0.
func (t TensorType) IsScalar() bool { return t.Shape.IsScalar() }

// String returns a human-readable representation.
func (t TensorType) String() string {
	if t.Shape.IsScalar() {
		return t.Elem.String()
	}
	return t.Elem.String() + t.Shape.String()
}

// ============================================================
// Layout
// ============================================================

// Layout represents the memory layout of a tensor.
type Layout int

const (
	LayoutRowMajor    Layout = iota // C-style (default)
	LayoutColumnMajor               // Fortran-style
	LayoutStrided                   // custom strides
)

// String returns the name of the layout.
func (l Layout) String() string {
	switch l {
	case LayoutRowMajor:
		return "row-major"
	case LayoutColumnMajor:
		return "column-major"
	case LayoutStrided:
		return "strided"
	default:
		return "unknown"
	}
}

// ============================================================
// Device
// ============================================================

// Device represents the target device for a tensor.
type Device int

const (
	DeviceCPU Device = iota
	DeviceGPU
	DeviceNPU
)

// String returns the name of the device.
func (d Device) String() string {
	switch d {
	case DeviceCPU:
		return "cpu"
	case DeviceGPU:
		return "gpu"
	case DeviceNPU:
		return "npu"
	default:
		return "unknown"
	}
}

// ============================================================
// Ownership
// ============================================================

// Ownership represents tensor memory ownership.
type Ownership int

const (
	OwnershipOwned Ownership = iota // we own the memory
	OwnershipBorrowed              // we borrow someone else's memory
	OwnershipAliased               // shared with another tensor
)
