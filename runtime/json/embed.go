// Package json owns the v1.2.20 C23 JSON DOM runtime sources so both code
// generators embed the same bytes. The Go engine uses go:embed; the
// self-hosted engine consumes src/compiler/json_runtime.kark, generated from
// these files by scripts/gen-json-runtime.ps1 and checked by
// pkg/codegen TestJSONKarkFresh.
package json

import _ "embed"

//go:embed c/karkain_json.h
var Header string

//go:embed c/karkain_json.c
var Source string

// Files lists the embedded C sources in canonical embed order.
var Files = []struct{ Name, Src string }{
	{"karkain_json.h", Header},
	{"karkain_json.c", Source},
}
