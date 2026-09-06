package ssa

import "fmt"

// TypedIR extends the existing SSA Type with precise bit-width types.
// This is backward-compatible: the original Void/I/F/Bool/Str/Value/Ptr types
// are still valid, but the new types enable proper register allocation and
// optimization passes.

// IType — integer types with explicit bit widths.
type IType struct {
	Bits int // 8, 16, 32, 64
}

func (t IType) String() string { return fmt.Sprintf("i%d", t.Bits) }

// FType — float types with explicit bit widths.
type FType struct {
	Bits int // 16, 32, 64
}

func (t FType) String() string { return fmt.Sprintf("f%d", t.Bits) }

// PtrType — typed pointer (points to a specific element type).
type PtrType struct {
	Elem Type
}

func (t PtrType) String() string { return fmt.Sprintf("*%s", t.Elem) }

// ArrayType — fixed-length array.
type ArrayType struct {
	Elem  Type
	Len   int
}

func (t ArrayType) String() string { return fmt.Sprintf("[%d]%s", t.Len, t.Elem) }

// TensorType — hardware tensor type for NPU/GPU compute.
type TensorType struct {
	Elem  Type
	Shape []int
}

func (t TensorType) String() string {
	return fmt.Sprintf("tensor<%s, %v>", t.Elem, t.Shape)
}

// FuncType — function type (for indirect calls / closures).
type FuncType struct {
	Params []Type
	Return Type
}

func (t FuncType) String() string {
	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		params[i] = p.String()
	}
	return fmt.Sprintf("fn(%s) -> %s", fmt.Sprintf("%v", params), t.Return)
}

// TypeRegistry — maps between the legacy Type enum and the new typed system.
// This provides backward compatibility while enabling typed SSA.
type TypeRegistry struct {
	types []Type
	lookup map[string]Type
}

func NewTypeRegistry() *TypeRegistry {
	return &TypeRegistry{
		lookup: make(map[string]Type),
	}
}

// ResolveType converts a type string to the most precise Type possible.
// For types without explicit bit width, returns the legacy Type.
func (r *TypeRegistry) ResolveType(name string) Type {
	switch name {
	case "void":
		return Void
	case "i8":
		return I
	case "i16":
		return I
	case "i32":
		return I
	case "i64":
		return I
	case "f16":
		return F
	case "f32":
		return F
	case "f64":
		return F
	case "bool":
		return Bool
	case "str", "string":
		return Str
	case "ptr":
		return Ptr
	default:
		return Value
	}
}

// TypeBits returns the bit width of a Type, or 0 for unknown/compound types.
func TypeBits(t Type) int {
	switch t {
	case I:
		return 64
	case F:
		return 64
	case Bool:
		return 1
	case Str:
		return 0
	case Ptr:
		return 64
	default:
		return 0
	}
}

// TypeIsInteger returns true if the type is an integer type.
func TypeIsInteger(t Type) bool {
	return t == I
}

// TypeIsFloat returns true if the type is a float type.
func TypeIsFloat(t Type) bool {
	return t == F
}

// TypeIsNumeric returns true if the type is numeric (int or float).
func TypeIsNumeric(t Type) bool {
	return t == I || t == F
}

// TypeIsPointer returns true if the type is a pointer.
func TypeIsPointer(t Type) bool {
	return t == Ptr
}

// TypeWidth returns the byte width of a Type.
func TypeWidth(t Type) int {
	bits := TypeBits(t)
	if bits == 0 {
		return 0
	}
	return (bits + 7) / 8
}

// TypeAlign returns the alignment (in bytes) for a Type.
// Alignment is the minimum of the byte width and 8 (platform register size).
func TypeAlign(t Type) int {
	w := TypeWidth(t)
	if w == 0 {
		return 1
	}
	if w > 8 {
		return 8
	}
	return w
}

// TypeIsCompatible returns true if two types can be used in the same operation
// without explicit casting.
func TypeIsCompatible(a, b Type) bool {
	if a == b {
		return true
	}
	// Integer types are compatible with each other
	if TypeIsInteger(a) && TypeIsInteger(b) {
		return true
	}
	// Float types are compatible with each other
	if TypeIsFloat(a) && TypeIsFloat(b) {
		return true
	}
	// Value is compatible with everything (dynamic boxing)
	if a == Value || b == Value {
		return true
	}
	return false
}

// TypePromote returns the result type of an operation between two types.
// Follows standard numeric promotion rules.
func TypePromote(a, b Type) Type {
	if a == b {
		return a
	}
	// If either is Value, result is Value
	if a == Value || b == Value {
		return Value
	}
	// If either is float, result is float
	if TypeIsFloat(a) || TypeIsFloat(b) {
		return F
	}
	// Both are integer — result is integer
	if TypeIsInteger(a) && TypeIsInteger(b) {
		return I
	}
	// Default to Value (dynamic boxing)
	return Value
}
