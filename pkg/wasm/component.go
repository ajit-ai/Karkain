package wasm

import (
	"fmt"
)

// Component-model envelope, Phase 139.
//
// A component nests core modules as sections inside a component-typed binary:
// the preamble carries layer 1 (component) instead of layer 0 (core module),
// and section id 2 denotes an embedded core module. This file implements that
// nesting layer only: WrapComponent embeds one core module so downstream
// tooling (registries, runtimes, wit-bindgen) can treat Karkain output as a
// component from day one.
//
// Deliberate boundary (NOT a defect): canonical ABI lifting/lowering
// (lift/lower of WIT-typed imports/exports) is future work. The envelope is
// structural and spec-shaped; typed boundaries arrive with the import
// direction of wit-bindgen.

// Component section ids (Component Model binary format, v1).
const (
	componentSectionCoreModule = 2
)

// WrapComponent embeds core (a complete core-module binary, as produced by
// CompileProgram) in a minimal component: preamble + one core-module section.
// The output is deterministic in the input (no names, no timestamps).
func WrapComponent(core []byte) ([]byte, error) {
	if len(core) < 8 || string(core[0:4]) != "\x00asm" {
		return nil, fmt.Errorf("WrapComponent: input is not a core module (bad magic)")
	}
	out := []byte{0x00, 'a', 's', 'm', 0x01, 0x01}
	out = appendUleb(out, componentSectionCoreModule)
	out = appendUleb(out, uint64(len(core)))
	out = append(out, core...)
	return out, nil
}

// UnwrapComponent extracts the embedded core module, verifying the preamble
// and section framing. It is the structural inverse of WrapComponent and the
// assertion helper for tests (round-trip pins the envelope, not an SDK).
func UnwrapComponent(comp []byte) ([]byte, error) {
	if len(comp) < 8 {
		return nil, fmt.Errorf("UnwrapComponent: truncated component")
	}
	if string(comp[0:4]) != "\x00asm" || comp[4] != 0x01 || comp[5] != 0x01 {
		return nil, fmt.Errorf("UnwrapComponent: bad component preamble")
	}
	rest := comp[6:]
	id, n := readUleb(rest)
	if n <= 0 || id != componentSectionCoreModule {
		return nil, fmt.Errorf("UnwrapComponent: want leading core-module section (id 2)")
	}
	rest = rest[n:]
	size, m := readUleb(rest)
	if m <= 0 || uint64(len(rest[m:])) < size {
		return nil, fmt.Errorf("UnwrapComponent: truncated core-module section")
	}
	core := rest[m : m+int(size)]
	if len(core) < 4 || string(core[0:4]) != "\x00asm" {
		return nil, fmt.Errorf("UnwrapComponent: embedded payload lacks module magic")
	}
	return core, nil
}

// readUleb decodes an unsigned LEB128 at the head of b, returning the value
// and the bytes consumed (n <= 0 on truncation/overflow).
func readUleb(b []byte) (uint64, int) {
	var v uint64
	var shift uint
	for i, c := range b {
		if i >= 10 {
			return 0, -1
		}
		v |= uint64(c&0x7f) << shift
		if c&0x80 == 0 {
			return v, i + 1
		}
		shift += 7
	}
	return 0, -1
}
