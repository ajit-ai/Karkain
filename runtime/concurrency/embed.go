// Package concurrency owns the Phase 107 C concurrency runtime sources so the
// code generator can embed them (byte-for-byte, no drift) into the generated
// translation unit. The pkg/runtime gate additionally compiles the same files
// from disk with the standalone driver concurrency_main.c, proving the two
// consumption paths share one source of truth.
//
// The C files contain `#include "karkain_conc.h"` / `#include
// "karkain_sched_impl.h"` lines that are stripped by Embedded prep before the
// bodies are appended to the generated C (the header text is emitted first).
package concurrency

import _ "embed"

//go:embed c/karkain_conc.h
var Header string

//go:embed c/karkain_sched_impl.h
var ImplHeader string

//go:embed c/karkain_channel.c
var ChannelSrc string

//go:embed c/karkain_actor.c
var ActorSrc string

//go:embed c/karkain_scheduler.c
var SchedulerSrc string

// Files lists the embedded C sources in canonical embed order.
var Files = []struct{ Name, Src string }{
	{"karkain_conc.h", Header},
	{"karkain_sched_impl.h", ImplHeader},
	{"karkain_channel.c", ChannelSrc},
	{"karkain_actor.c", ActorSrc},
	{"karkain_scheduler.c", SchedulerSrc},
}

// Driver returns the standalone scenario-driver source (concurrency_main.c),
// used by the pkg/runtime gate to compile+run the runtime in isolation.
func Driver() string { return _Driver }

//go:embed c/concurrency_main.c
var _Driver string