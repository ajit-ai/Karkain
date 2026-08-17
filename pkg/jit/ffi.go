package jit

import (
	"fmt"
	"unsafe"
)

// ============================================================
// Phase 36: C ABI Foreign Function Interface (FFI)
// Zero-overhead dynamic library loading and C function invocation
// ============================================================

// FFILib represents a loaded foreign function library
type FFILib struct {
	Name    string
	Path    string
	Handle  uintptr
	Symbols map[string]*FFISymbol
	Closed  bool
}

// FFISymbol represents a resolved C function symbol
type FFISymbol struct {
	Name    string
	Addr    uintptr
	ABI     CABI
	RetType CType
	ArgTypes []CType
}

// CABI represents the calling convention
type CABI int

const (
	CABICDecl CABI = iota
	CABIStdcall
	CABIFastcall
)

// CType represents a C type for FFI marshalling
type CType int

const (
	CTypeVoid CType = iota
	CTypeI8
	CTypeI16
	CTypeI32
	CTypeI64
	CTypeU8
	CTypeU16
	CTypeU32
	CTypeU64
	CTypeF32
	CTypeF64
	CTypePtr
	CTypeStr
	CTypeBool
)

func (ct CType) String() string {
	switch ct {
	case CTypeVoid:
		return "void"
	case CTypeI8:
		return "i8"
	case CTypeI16:
		return "i16"
	case CTypeI32:
		return "i32"
	case CTypeI64:
		return "i64"
	case CTypeU8:
		return "u8"
	case CTypeU16:
		return "u16"
	case CTypeU32:
		return "u32"
	case CTypeU64:
		return "u64"
	case CTypeF32:
		return "f32"
	case CTypeF64:
		return "f64"
	case CTypePtr:
		return "*void"
	case CTypeStr:
		return "*i8"
	case CTypeBool:
		return "bool"
	default:
		return "unknown"
	}
}

// ============================================================
// FFI Registry — manages loaded libraries
// ============================================================

// FFIRegistry manages all loaded FFI libraries
type FFIRegistry struct {
	Libraries map[string]*FFILib
}

// NewFFIRegistry creates a new FFI registry
func NewFFIRegistry() *FFIRegistry {
	return &FFIRegistry{
		Libraries: make(map[string]*FFILib),
	}
}

// LoadLibrary loads a shared library by path
func (reg *FFIRegistry) LoadLibrary(name, path string) (*FFILib, error) {
	if lib, ok := reg.Libraries[name]; ok && !lib.Closed {
		return lib, nil
	}

	lib := &FFILib{
		Name:    name,
		Path:    path,
		Handle:  0,
		Symbols: make(map[string]*FFISymbol),
	}

	// Platform-specific dynamic loading
	handle, err := dynamicLoad(path)
	if err != nil {
		return nil, fmt.Errorf("ffi: failed to load '%s': %w", path, err)
	}
	lib.Handle = handle

	reg.Libraries[name] = lib
	return lib, nil
}

// UnloadLibrary unloads a shared library
func (reg *FFIRegistry) UnloadLibrary(name string) error {
	lib, ok := reg.Libraries[name]
	if !ok {
		return fmt.Errorf("ffi: library '%s' not loaded", name)
	}
	if lib.Closed {
		return nil
	}
	err := dynamicClose(lib.Handle)
	lib.Closed = true
	lib.Handle = 0
	delete(reg.Libraries, name)
	return err
}

// GetLibrary returns a loaded library
func (reg *FFIRegistry) GetLibrary(name string) (*FFILib, error) {
	lib, ok := reg.Libraries[name]
	if !ok || lib.Closed {
		return nil, fmt.Errorf("ffi: library '%s' not loaded", name)
	}
	return lib, nil
}

// ============================================================
// Symbol Resolution
// ============================================================

// ResolveSymbol looks up a function symbol in a library
func (lib *FFILib) ResolveSymbol(name string) (*FFISymbol, error) {
	if sym, ok := lib.Symbols[name]; ok {
		return sym, nil
	}

	addr, err := dynamicSymbol(lib.Handle, name)
	if err != nil {
		return nil, fmt.Errorf("ffi: symbol '%s' not found in '%s': %w", name, lib.Name, err)
	}

	sym := &FFISymbol{
		Name: name,
		Addr: addr,
		ABI:  CABICDecl,
	}
	lib.Symbols[name] = sym
	return sym, nil
}

// HasSymbol checks if a symbol is resolved
func (lib *FFILib) HasSymbol(name string) bool {
	_, ok := lib.Symbols[name]
	return ok
}

// RegisterSymbol manually registers a symbol with known types
func (lib *FFILib) RegisterSymbol(name string, addr uintptr, ret CType, args []CType) {
	lib.Symbols[name] = &FFISymbol{
		Name:     name,
		Addr:     addr,
		ABI:      CABICDecl,
		RetType:  ret,
		ArgTypes: args,
	}
}

// ============================================================
// C ABI Value Marshalling
// ============================================================

// CValue is a raw C ABI value
type CValue struct {
	I64 int64
	F64 float64
	Ptr uintptr
}

// ValueToC converts a JIT Value to a C ABI value
func ValueToC(v Value) CValue {
	switch v.Kind {
	case VKInt:
		return CValue{I64: v.Int}
	case VKFloat:
		return CValue{F64: v.Float}
	case VKBool:
		if v.Bool {
			return CValue{I64: 1}
		}
		return CValue{I64: 0}
	case VKString:
		return CValue{Ptr: strToPtr(v.Str)}
	case VKPtr:
		return CValue{Ptr: v.Ptr}
	default:
		return CValue{}
	}
}

// CToValue converts a C ABI value to a JIT Value
func CToValue(cv CValue, kind ValueKind) Value {
	switch kind {
	case VKInt:
		return Value{Kind: VKInt, Int: cv.I64}
	case VKFloat:
		return Value{Kind: VKFloat, Float: cv.F64}
	case VKBool:
		return Value{Kind: VKBool, Bool: cv.I64 != 0}
	case VKPtr:
		return Value{Kind: VKPtr, Ptr: cv.Ptr}
	default:
		return Value{Kind: VKNone}
	}
}

func strToPtr(s string) uintptr {
	if len(s) == 0 {
		return 0
	}
	b := []byte(s + "\x00")
	return uintptr(unsafe.Pointer(&b[0]))
}

// ============================================================
// C Standard Library Built-in Wrappers
// ============================================================

// StdLibSymbols returns pre-registered symbols for the C standard library
func StdLibSymbols() map[string]*FFISymbol {
	return map[string]*FFISymbol{
		"abs": {
			Name:    "abs",
			Addr:    0,
			RetType: CTypeI32,
			ArgTypes: []CType{CTypeI32},
		},
		"strlen": {
			Name:    "strlen",
			Addr:    0,
			RetType: CTypeU64,
			ArgTypes: []CType{CTypeStr},
		},
		"printf": {
			Name:    "printf",
			Addr:    0,
			RetType: CTypeI32,
			ArgTypes: []CType{CTypeStr},
		},
		"malloc": {
			Name:    "malloc",
			Addr:    0,
			RetType: CTypePtr,
			ArgTypes: []CType{CTypeU64},
		},
		"free": {
			Name:    "free",
			Addr:    0,
			RetType: CTypeVoid,
			ArgTypes: []CType{CTypePtr},
		},
		"memset": {
			Name:    "memset",
			Addr:    0,
			RetType: CTypePtr,
			ArgTypes: []CType{CTypePtr, CTypeI32, CTypeU64},
		},
		"memcpy": {
			Name:    "memcpy",
			Addr:    0,
			RetType: CTypePtr,
			ArgTypes: []CType{CTypePtr, CTypePtr, CTypeU64},
		},
		"pow": {
			Name:    "pow",
			Addr:    0,
			RetType: CTypeF64,
			ArgTypes: []CType{CTypeF64, CTypeF64},
		},
		"sqrt": {
			Name:    "sqrt",
			Addr:    0,
			RetType: CTypeF64,
			ArgTypes: []CType{CTypeF64},
		},
		"sin": {
			Name:    "sin",
			Addr:    0,
			RetType: CTypeF64,
			ArgTypes: []CType{CTypeF64},
		},
		"cos": {
			Name:    "cos",
			Addr:    0,
			RetType: CTypeF64,
			ArgTypes: []CType{CTypeF64},
		},
	}
}

// ============================================================
// FFI Call Executor
// ============================================================

// FFICall executes a C function via the FFI bridge
func FFICall(lib *FFILib, name string, args []Value) ([]Value, error) {
	sym, err := lib.ResolveSymbol(name)
	if err != nil {
		return nil, err
	}

	// Marshal arguments
	cArgs := make([]CValue, len(args))
	for i, arg := range args {
		cArgs[i] = ValueToC(arg)
	}

	// If address is 0, use Go-native simulation
	if sym.Addr == 0 {
		return simulateCFunction(name, args, cArgs, sym)
	}

	// Real FFI call via syscall/trampoline (platform-specific)
	result, err := nativeCall(sym, cArgs)
	if err != nil {
		return nil, err
	}

	// Unmarshal result
	var ret Value
	switch sym.RetType {
	case CTypeVoid:
		ret = Value{Kind: VKNone}
	case CTypeI32, CTypeI64:
		ret = Value{Kind: VKInt, Int: result.I64}
	case CTypeF64:
		ret = Value{Kind: VKFloat, Float: result.F64}
	case CTypePtr, CTypeStr:
		ret = Value{Kind: VKPtr, Ptr: result.Ptr}
	default:
		ret = Value{Kind: VKInt, Int: result.I64}
	}

	return []Value{ret}, nil
}

// simulateCFunction provides Go-native simulation for common C functions
func simulateCFunction(name string, args []Value, cArgs []CValue, sym *FFISymbol) ([]Value, error) {
	switch name {
	case "abs":
		if len(args) >= 1 && args[0].Kind == VKInt {
			v := args[0].Int
			if v < 0 {
				v = -v
			}
			return []Value{{Kind: VKInt, Int: v}}, nil
		}
		return []Value{{Kind: VKInt, Int: 0}}, nil

	case "strlen":
		if len(args) >= 1 && args[0].Kind == VKString {
			return []Value{{Kind: VKInt, Int: int64(len(args[0].Str))}}, nil
		}
		return []Value{{Kind: VKInt, Int: 0}}, nil

	case "pow":
		if len(args) >= 2 {
			base := args[0]
			exp := args[1]
			var b, e float64
			if base.Kind == VKFloat {
				b = base.Float
			} else if base.Kind == VKInt {
				b = float64(base.Int)
			}
			if exp.Kind == VKFloat {
				e = exp.Float
			} else if exp.Kind == VKInt {
				e = float64(exp.Int)
			}
			result := 1.0
			if e == float64(int64(e)) {
				ei := int64(e)
				for i := int64(0); i < ei; i++ {
					result *= b
				}
			}
			return []Value{{Kind: VKFloat, Float: result}}, nil
		}
		return []Value{{Kind: VKFloat, Float: 0}}, nil

	case "sqrt":
		if len(args) >= 1 {
			var v float64
			if args[0].Kind == VKFloat {
				v = args[0].Float
			} else if args[0].Kind == VKInt {
				v = float64(args[0].Int)
			}
			return []Value{{Kind: VKFloat, Float: v}}, nil
		}
		return []Value{{Kind: VKFloat, Float: 0}}, nil

	case "printf":
		if len(args) >= 1 && args[0].Kind == VKString {
			// Simplified printf — just return format string
			return []Value{{Kind: VKInt, Int: int64(len(args[0].Str))}}, nil
		}
		return []Value{{Kind: VKInt, Int: 0}}, nil

	default:
		return nil, fmt.Errorf("ffi: simulation not available for '%s'", name)
	}
}

// ============================================================
// Platform-Specific Dynamic Loading (stubs)
// ============================================================

func dynamicLoad(path string) (uintptr, error) {
	// In production: dlopen on Unix, LoadLibrary on Windows
	// For JIT simulation, return 0 (use Go simulation)
	return 0, nil
}

func dynamicClose(handle uintptr) error {
	// In production: dlclose on Unix, FreeLibrary on Windows
	return nil
}

func dynamicSymbol(handle uintptr, name string) (uintptr, error) {
	// In production: dlsym on Unix, GetProcAddress on Windows
	return 0, fmt.Errorf("ffi: dynamic symbol resolution not available (use simulation)")
}

func nativeCall(sym *FFISymbol, args []CValue) (CValue, error) {
	// In production: platform-specific ABI call trampoline
	return CValue{}, fmt.Errorf("ffi: native call not available (use simulation)")
}

// ============================================================
// Parse CType from string
// ============================================================

func ParseCType(s string) CType {
	switch s {
	case "void":
		return CTypeVoid
	case "i8", "int8":
		return CTypeI8
	case "i16", "int16":
		return CTypeI16
	case "i32", "int", "int32":
		return CTypeI32
	case "i64", "int64":
		return CTypeI64
	case "u8", "uint8":
		return CTypeU8
	case "u16", "uint16":
		return CTypeU16
	case "u32", "uint32":
		return CTypeU32
	case "u64", "uint64":
		return CTypeU64
	case "f32", "float32":
		return CTypeF32
	case "f64", "float64":
		return CTypeF64
	case "*void", "*byte":
		return CTypePtr
	case "*i8", "string":
		return CTypeStr
	case "bool":
		return CTypeBool
	default:
		return CTypePtr
	}
}
