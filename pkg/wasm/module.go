package wasm

import "fmt"

// ValueType is a WebAssembly value type.
type ValueType byte

const (
	I32 ValueType = 0x7f
	I64 ValueType = 0x7e
	F32 ValueType = 0x7d
	F64 ValueType = 0x7c
)

// FuncType is a function signature: params and results.
type FuncType struct {
	Params  []ValueType
	Results []ValueType
}

// ImportKind identifies the kind of an import (`0` = func, `2` = memory).
type ImportKind byte

const (
	ImportFunc ImportKind = 0
	ImportMem  ImportKind = 2
)

// Import describes a WASI/foreign import.
type Import struct {
	Module string
	Field  string
	Kind   ImportKind
	// Func: type index. Memory: min pages (max omitted).
	TypeIdx int
	Min     int
}

// ExportKind is `0` = func, `2` = memory, `3` = global.
type ExportKind byte

const (
	ExportFunc   ExportKind = 0
	ExportMemory ExportKind = 2
	ExportGlobal ExportKind = 3
)

// Export names an entity for the outside world.
type Export struct {
	Name string
	Kind ExportKind
	Idx  int
}

// Global is a mutable i64/i32 global with a constant init expression.
type Global struct {
	Type ValueType
	Mut  bool
	Init []byte
}

// DataSeg is a data segment written at a constant offset.
type DataSeg struct {
	Offset int
	Bytes  []byte
}

// ModuleBuilder assembles a WebAssembly binary module section by section.
type ModuleBuilder struct {
	Types     []FuncType
	Imports   []Import
	FuncTypes []int      // type index of each defined function
	Globals   []Global
	Exports   []Export
	HasStart  bool
	StartIdx  int
	Data      []DataSeg
	MemoryMin int
	MemoryMax int // -1 = unbounded
	Codes     []Code
}

type Code struct {
	Body   []byte
	Locals []ValueType // additional locals (after params), one entry per slot
}

// AddType interns a function type and returns its index.
func (m *ModuleBuilder) AddType(fn FuncType) int {
	for i, t := range m.Types {
		if len(t.Params) == len(fn.Params) && len(t.Results) == len(fn.Results) {
			same := true
			for j := range t.Params {
				if t.Params[j] != fn.Params[j] {
					same = false
					break
				}
			}
			if same {
				for j := range t.Results {
					if t.Results[j] != fn.Results[j] {
						same = false
						break
					}
				}
			}
			if same {
				return i
			}
		}
	}
	m.Types = append(m.Types, fn)
	return len(m.Types) - 1
}

// Encode serializes the module to binary form.
func (m *ModuleBuilder) Encode() []byte {
	var buf []byte
	buf = append(buf, 0x00, 0x61, 0x73, 0x6d) // \0asm
	buf = append(buf, 0x01, 0x00, 0x00, 0x00) // version 1

	// --- Type section (1) ---
	if len(m.Types) > 0 {
		buf = append(buf, 1)
		var sb secBuilder
		sb.uln(len(m.Types))
		for _, t := range m.Types {
			sb.byte(0x60)
			sb.typedVec(t.Params)
			sb.typedVec(t.Results)
		}
		buf = sb.appendTo(buf)
	}

	// --- Import section (2) ---
	if len(m.Imports) > 0 {
		buf = append(buf, 2)
		var sb secBuilder
		sb.uln(len(m.Imports))
		for _, im := range m.Imports {
			sb.str(im.Module)
			sb.str(im.Field)
			sb.byte(byte(im.Kind))
			switch im.Kind {
			case ImportFunc:
				sb.uint32(uint32(im.TypeIdx))
			case ImportMem:
				sb.byte(0x00) // limits: min present, max absent
				sb.uint32(uint32(im.Min))
			}
		}
		buf = sb.appendTo(buf)
	}

	// --- Function section (3) ---
	if len(m.FuncTypes) > 0 {
		buf = append(buf, 3)
		var sb secBuilder
		sb.uln(len(m.FuncTypes))
		for _, ti := range m.FuncTypes {
			sb.uint32(uint32(ti))
		}
		buf = sb.appendTo(buf)
	}

	// --- Memory section (5) ---
	if m.MemoryMin > 0 || m.MemoryMax >= 0 {
		buf = append(buf, 5)
		var sb secBuilder
		sb.byte(1) // one memory
		if m.MemoryMax >= 0 {
			sb.byte(0x01) // min + max present
			sb.uint32(uint32(m.MemoryMin))
			sb.uint32(uint32(m.MemoryMax))
		} else {
			sb.byte(0x00)
			sb.uint32(uint32(m.MemoryMin))
		}
		buf = sb.appendTo(buf)
	}

	// --- Global section (6) ---
	if len(m.Globals) > 0 {
		buf = append(buf, 6)
		var sb secBuilder
		sb.uln(len(m.Globals))
		for _, g := range m.Globals {
			sb.byte(byte(g.Type))
			if g.Mut {
				sb.byte(1)
			} else {
				sb.byte(0)
			}
			sb.raw(g.Init)
		}
		buf = sb.appendTo(buf)
	}

	// --- Export section (7) ---
	if len(m.Exports) > 0 {
		buf = append(buf, 7)
		var sb secBuilder
		sb.uln(len(m.Exports))
		for _, e := range m.Exports {
			sb.str(e.Name)
			sb.byte(byte(e.Kind))
			sb.uint32(uint32(e.Idx))
		}
		buf = sb.appendTo(buf)
	}

	// --- Start section (8) ---
	if m.HasStart {
		buf = append(buf, 8)
		var sb secBuilder
		sb.uint32(uint32(m.StartIdx))
		buf = sb.appendTo(buf)
	}

	// --- Code section (10) ---
	if len(m.Codes) > 0 {
		buf = append(buf, 10)
		var sb secBuilder
		sb.uln(len(m.Codes))
		for _, c := range m.Codes {
			var cb secBuilder
			// locals: run-length grouped by type
			groups := runLengthLocals(c.Locals)
			cb.uln(len(groups))
			for _, g := range groups {
				cb.uln(g.count)
				cb.byte(byte(g.typ))
			}
			cb.raw(c.Body)
			sb.sub(cb)
		}
		buf = sb.appendTo(buf)
	}

	// --- Data section (11) ---
	if len(m.Data) > 0 {
		buf = append(buf, 11)
		var sb secBuilder
		sb.uln(len(m.Data))
		for _, d := range m.Data {
			sb.byte(0x00) // active, memory index 0
			// offset = i32.const <offset> end
			sb.byte(0x41)
			sb.sl(int64(d.Offset))
			sb.byte(0x0b)
			sb.bytesv(d.Bytes)
		}
		buf = sb.appendTo(buf)
	}

	return buf
}

func runLengthLocals(l []ValueType) []groupedLocal {
	var out []groupedLocal
	for _, t := range l {
		if len(out) == 0 || out[len(out)-1].typ != t {
			out = append(out, groupedLocal{typ: t, count: 1})
		} else {
			out[len(out)-1].count++
		}
	}
	return out
}

type groupedLocal struct {
	typ   ValueType
	count int
}

// secBuilder accumulates section payload bytes.
type secBuilder struct{ b []byte }

func (s *secBuilder) byte(v byte)      { s.b = append(s.b, v) }
func (s *secBuilder) raw(p []byte)     { s.b = append(s.b, p...) }
func (s *secBuilder) str(p string)     { s.ul(uint64(len(p))); s.b = append(s.b, p...) }
func (s *secBuilder) bytesv(p []byte)  { s.ul(uint64(len(p))); s.b = append(s.b, p...) }
func (s *secBuilder) uint32(v uint32)  { s.b = appendUleb(s.b, uint64(v)) }
func (s *secBuilder) ul(v uint64)      { s.b = appendUleb(s.b, v) }
func (s *secBuilder) uln(v int)        { s.b = appendUleb(s.b, uint64(v)) }
func (s *secBuilder) sl(v int64)       { s.b = appendSleb(s.b, v) }
func (s *secBuilder) typedVec(v []ValueType) {
	s.uln(len(v))
	for _, t := range v {
		s.byte(byte(t))
	}
}

// sub appends a nested section payload sized with its own length prefix.
func (s *secBuilder) sub(other secBuilder) {
	s.uln(len(other.b))
	s.b = append(s.b, other.b...)
}

// appendTo emits the length prefix then the payload into buf.
func (s *secBuilder) appendTo(dst []byte) []byte {
	dst = appendUleb(dst, uint64(len(s.b)))
	dst = append(dst, s.b...)
	return dst
}

func appendUleb(dst []byte, v uint64) []byte {
	for {
		c := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			c |= 0x80
		}
		dst = append(dst, c)
		if v == 0 {
			return dst
		}
	}
}

func appendSleb(dst []byte, v int64) []byte {
	for {
		c := byte(v & 0x7f)
		done := (v&0x40 == 0 && v>>7 == 0) || (v&0x40 != 0 && v>>7 == -1)
		if !done {
			c |= 0x80
		}
		dst = append(dst, c)
		if done {
			return dst
		}
		v >>= 7
	}
}

// MagicOK checks the wasm magic bytes and version.
func MagicOK(b []byte) bool {
	if len(b) < 8 {
		return false
	}
	return b[0] == 0x00 && b[1] == 0x61 && b[2] == 0x73 && b[3] == 0x6d &&
		b[4] == 0x01 && b[5] == 0x00 && b[6] == 0x00 && b[7] == 0x00
}

// DescribeModule returns a short human-readable summary of the module for
// the build report (function count, size, data size).
func DescribeModule(b []byte, name string) string {
	n := 0
	segs := 0
	// crude scan: count code section function entries by parsing is overkill;
	// report static facts.
	_ = n
	_ = segs
	return fmt.Sprintf("%s: %d bytes, magic ok=%v", name, len(b), MagicOK(b))
}