// Package rc owns the v1.2.24 C23 reference-counting runtime sources so both
// code generators embed the same bytes. The Go engine uses go:embed; the
// self-hosted engine consumes src/compiler/rc_runtime.kark, generated from
// these files by scripts/gen-rc-runtime.ps1 and checked by
// pkg/codegen TestRCKarkFresh.
package rc

import _ "embed"

//go:embed c/karkain_rc.h
var Header string

//go:embed c/karkain_rc.c
var Source string

// Files lists the embedded C sources in canonical embed order.
var Files = []struct{ Name, Src string }{
	{"karkain_rc.h", Header},
	{"karkain_rc.c", Source},
}