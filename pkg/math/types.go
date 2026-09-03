package math

import "fmt"

// MathType represents the type system for Math IR nodes.
type MathType int

const (
	Unknown MathType = iota
	Bool
	Int32
	Int64
	Float32
	Float64
	Complex64
	Complex128
	StringType
)

// String returns the name of the math type.
func (t MathType) String() string {
	switch t {
	case Unknown:
		return "unknown"
	case Bool:
		return "bool"
	case Int32:
		return "i32"
	case Int64:
		return "i64"
	case Float32:
		return "f32"
	case Float64:
		return "f64"
	case Complex64:
		return "complex64"
	case Complex128:
		return "complex128"
	case StringType:
		return "string"
	default:
		return fmt.Sprintf("MathType(%d)", int(t))
	}
}

// IsNumeric returns true if the type is numeric (int, float, complex).
func (t MathType) IsNumeric() bool {
	switch t {
	case Int32, Int64, Float32, Float64, Complex64, Complex128:
		return true
	}
	return false
}

// IsInteger returns true if the type is an integer type.
func (t MathType) IsInteger() bool {
	return t == Int32 || t == Int64
}

// IsFloat returns true if the type is a floating-point type.
func (t MathType) IsFloat() bool {
	return t == Float32 || t == Float64
}

// IsComplex returns true if the type is a complex type.
func (t MathType) IsComplex() bool {
	return t == Complex64 || t == Complex128
}

// BitWidth returns the bit width of the type.
func (t MathType) BitWidth() int {
	switch t {
	case Int32, Float32, Complex64:
		return 32
	case Int64, Float64, Complex128:
		return 64
	default:
		return 0
	}
}

// ============================================================
// Type Promotion
// ============================================================

// PromoteType returns the result type when combining two types.
// Rules:
//   - Same type → same type
//   - int + float → float (wider)
//   - float + complex → complex (wider)
//   - int + complex → complex (wider)
//   - i32 + i64 → i64
//   - f32 + f64 → f64
func PromoteType(a, b MathType) MathType {
	if a == b {
		return a
	}

	// Bool promotes to the other type
	if a == Bool {
		return b
	}
	if b == Bool {
		return a
	}

	// Complex dominates everything
	if a.IsComplex() || b.IsComplex() {
		w := max(a.BitWidth(), b.BitWidth())
		if w >= 64 {
			return Complex128
		}
		return Complex64
	}

	// Float dominates integer
	if (a.IsFloat() && b.IsInteger()) || (a.IsInteger() && b.IsFloat()) {
		w := max(a.BitWidth(), b.BitWidth())
		if w >= 64 {
			return Float64
		}
		return Float32
	}

	// Both floats
	if a.IsFloat() && b.IsFloat() {
		if max(a.BitWidth(), b.BitWidth()) >= 64 {
			return Float64
		}
		return Float32
	}

	// Both integers
	if a.IsInteger() && b.IsInteger() {
		if max(a.BitWidth(), b.BitWidth()) >= 64 {
			return Int64
		}
		return Int32
	}

	return Unknown
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ============================================================
// Function Signatures
// ============================================================

// FuncSig defines the expected signature of a math function.
type FuncSig struct {
	Name     string
	ArgTypes []MathType // nil means any numeric type accepted
	RetType  MathType   // Unknown means inferred from args
	MinArgs  int
	MaxArgs  int // -1 means unlimited
}

// BuiltInFuncs returns the standard math function signatures.
func BuiltInFuncs() []FuncSig {
	anyFloat := []MathType{Float32, Float64}
	anyNumeric := []MathType{Int32, Int64, Float32, Float64}

	return []FuncSig{
		// Single-argument functions (return same type)
		{Name: "sqrt", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "exp", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "log", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "log2", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "log10", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "sin", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "cos", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "tan", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "asin", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "acos", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "atan", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "abs", ArgTypes: anyNumeric, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "floor", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "ceil", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "round", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 1, MaxArgs: 1},
		{Name: "sign", ArgTypes: anyNumeric, RetType: Unknown, MinArgs: 1, MaxArgs: 1},

		// Two-argument functions
		{Name: "pow", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 2, MaxArgs: 2},
		{Name: "atan2", ArgTypes: anyFloat, RetType: Unknown, MinArgs: 2, MaxArgs: 2},
		{Name: "min", ArgTypes: anyNumeric, RetType: Unknown, MinArgs: 2, MaxArgs: 2},
		{Name: "max", ArgTypes: anyNumeric, RetType: Unknown, MinArgs: 2, MaxArgs: 2},
		{Name: "mod", ArgTypes: anyNumeric, RetType: Unknown, MinArgs: 2, MaxArgs: 2},

		// Variadic
		{Name: "sum", ArgTypes: anyNumeric, RetType: Unknown, MinArgs: 1, MaxArgs: -1},
		{Name: "product", ArgTypes: anyNumeric, RetType: Unknown, MinArgs: 1, MaxArgs: -1},

		// Utility (C intrinsic bridge)
		{Name: "len", ArgTypes: nil, RetType: Int64, MinArgs: 1, MaxArgs: 1},
		{Name: "type", ArgTypes: nil, RetType: StringType, MinArgs: 1, MaxArgs: 1},
	}
}

// LookupFunc finds a built-in function by name.
func LookupFunc(name string) (FuncSig, bool) {
	for _, sig := range BuiltInFuncs() {
		if sig.Name == name {
			return sig, true
		}
	}
	return FuncSig{}, false
}
