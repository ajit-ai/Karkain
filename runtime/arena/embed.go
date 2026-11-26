// Package arena owns the v1.2.25 C23 arena region runtime sources so both code
// generators embed the same bytes. The Go engine uses go:embed; the self-hosted
// engine consumes src/compiler/arena_runtime.kark, generated from these files by
// scripts/gen-arena-runtime.ps1 and checked by pkg/codegen TestArenaKarkFresh.
package arena

import _ "embed"

//go:embed c/karkain_arena.h
var Header string

//go:embed c/karkain_arena.c
var Source string

// Files lists the embedded C sources in canonical embed order.
var Files = []struct{ Name, Src string }{
	{"karkain_arena.h", Header},
	{"karkain_arena.c", Source},
}