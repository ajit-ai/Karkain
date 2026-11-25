// Package tls owns the v1.2.17 C23 transport-TLS runtime sources so both
// code generators embed the same bytes (no hand-copied drift). The Go engine
// uses go:embed; the self-hosted engine consumes src/compiler/tls_runtime.kark,
// generated from these files by scripts/gen-tls-runtime.ps1 and checked by
// pkg/codegen TestTLSTKarkFresh.
package tls

import _ "embed"

//go:embed c/karkain_tls.h
var Header string

//go:embed c/karkain_tls.c
var Source string

// Files lists the embedded C sources in canonical embed order.
var Files = []struct{ Name, Src string }{
	{"karkain_tls.h", Header},
	{"karkain_tls.c", Source},
}
