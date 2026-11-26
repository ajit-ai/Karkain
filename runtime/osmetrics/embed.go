// Package osmetrics owns the v1.2.19 C23 OS metric/process runtime sources
// so both code generators embed the same bytes. The Go engine uses go:embed;
// the self-hosted engine consumes src/compiler/osmetrics_runtime.kark,
// generated from these files by scripts/gen-osmetrics-runtime.ps1 and checked
// by pkg/codegen TestOSMetricsKarkFresh.
package osmetrics

import _ "embed"

//go:embed c/karkain_osmetrics.h
var Header string

//go:embed c/karkain_osmetrics.c
var Source string

// Files lists the embedded C sources in canonical embed order.
var Files = []struct{ Name, Src string }{
	{"karkain_osmetrics.h", Header},
	{"karkain_osmetrics.c", Source},
}
