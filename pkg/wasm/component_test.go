package wasm

import (
	"bytes"
	"testing"
)

// Phase 139: the component envelope is structural — preamble layer 1 plus
// one core-module section. These tests pin the framing bytes and the
// round-trip; typed canonical-ABI lifting is the documented future slice.

func TestComponentRoundTrip(t *testing.T) {
	core := compile(t, "func main() {\n    print(42)\n}\n", "main.kark")
	comp, err := WrapComponent(core)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	// Preamble: module magic, version 1, LAYER 1 (component, not module).
	if len(comp) < 8 || string(comp[0:4]) != "\x00asm" || comp[4] != 0x01 || comp[5] != 0x01 {
		t.Fatalf("bad component preamble: % x", comp[:8])
	}
	// First section must be the core-module section (id 2).
	id, n := readUleb(comp[6:])
	if n <= 0 || id != componentSectionCoreModule {
		t.Fatalf("want leading core-module section id 2, got id=%d n=%d", id, n)
	}
	back, err := UnwrapComponent(comp)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if !bytes.Equal(back, core) {
		t.Fatal("round-trip mismatch")
	}
	// Deterministic in the input.
	again, err := WrapComponent(core)
	if err != nil {
		t.Fatalf("re-wrap: %v", err)
	}
	if !bytes.Equal(again, comp) {
		t.Fatal("wrap not deterministic")
	}
}

func TestComponentNegatives(t *testing.T) {
	if _, err := WrapComponent([]byte("nope")); err == nil {
		t.Error("expected error wrapping non-module")
	}
	core := compile(t, "func main() {\n    print(1)\n}\n", "main.kark")
	comp, err := WrapComponent(core)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}
	// Corrupt the layer byte: core-module layer 0 must be rejected.
	badLayer := append([]byte{}, comp...)
	badLayer[5] = 0x00
	if _, err := UnwrapComponent(badLayer); err == nil {
		t.Error("expected error for layer-0 preamble")
	}
	// Truncate the section.
	if _, err := UnwrapComponent(comp[:8]); err == nil {
		t.Error("expected error for truncated component")
	}
	// Wrong leading section id.
	badID := append([]byte{}, comp...)
	badID[6] = 0x07
	if _, err := UnwrapComponent(badID); err == nil {
		t.Error("expected error for non-core-module leading section")
	}
}
