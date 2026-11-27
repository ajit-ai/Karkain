// Package osctl owns the v1.2.34 C23 portable OS/process-control runtime sources
// so both code generators embed the same bytes. The Go engine uses go:embed;
// the self-hosted engine consumes src/compiler/osctl_runtime.kark,
// generated from these files by scripts/gen-osctl-runtime.ps1 and checked
// by pkg/codegen TestOSCtlKarkFresh.
package osctl

import _ "embed"

//go:embed c/karkain_osctl.h
var Header string

//go:embed c/karkain_osctl.c
var Source string

// Files lists the embedded C sources in canonical embed order.
var Files = []struct{ Name, Src string }{
	{"karkain_osctl.h", Header},
	{"karkain_osctl.c", Source},
}
