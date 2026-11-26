// Package sqlite owns the v1.2.22 C23 SQLite binding and the vendored
// amalgamation source. The Go CLI writes Source to a temp file next to the
// generated C and compiles it with the user program; kcc's generated runtime
// contains only the small binding, never the amalgamation.

package sqlite

import _ "embed"

//go:embed c/vendor/sqlite3.c
var Source string

//go:embed c/vendor/sqlite3.h
var Header string

//go:embed c/karkain_sqlite.h
var BindingHeader string

//go:embed c/karkain_sqlite.c
var BindingSource string

// Files lists the generated-binding sources in canonical order.
var Files = []struct{ Name, Src string }{
	{"karkain_sqlite.h", BindingHeader},
	{"karkain_sqlite.c", BindingSource},
}
