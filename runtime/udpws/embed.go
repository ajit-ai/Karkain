// Package udpws owns the v1.2.21 C23 UDP/WebSocket runtime sources so both
// code generators embed the same bytes. The Go engine uses go:embed; the
// self-hosted engine consumes src/compiler/udpws_runtime.kark, generated from
// these files by scripts/gen-udpws-runtime.ps1 and checked by
// pkg/codegen TestUDPWSKarkFresh.
package udpws

import _ "embed"

//go:embed c/karkain_udpws.h
var Header string

//go:embed c/karkain_udpws.c
var Source string

// Files lists the embedded C sources in canonical embed order.
var Files = []struct{ Name, Src string }{
	{"karkain_udpws.h", Header},
	{"karkain_udpws.c", Source},
}
