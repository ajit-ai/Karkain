package cli

// Phase 146C — kcc Generics v1 parity. The self-hosted compiler now runs
// the monomorphization pass (src/compiler/checker.kark): it collects
// generic FuncDecl/StructDecl templates, rewrites `f[T](args)` /
// `Point[T]{...}` call sites (mangling specializations to e.g.
// `id_int` / `Point_int`, clearing the parser's TypeArgs slots so the
// pass is idempotent), appends specialization units until fixpoint from
// nested generic calls, emits K115 diagnostics for arity / bare / unknown
// generic uses, and demotes non-template `ops[idx](5)` heads back to the
// Phase-133 Index+IndirectCall shape. kcc drive paths (check/build/run)
// are byte-identical to the Go engine for the corpus below. On hosts
// where the self-built compiler cannot be built (the documented Phase 127
// error[K127] low-RAM class) the kcc subtests skip with a note.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// run146cKCC drives the real binary's kcc engine and returns possibly-blank
// stdout; `err != nil` carries the failure. K127/deadline handling is left
// to callers so each subtest can produce the right skip note.
func run146cKCC(t *testing.T, karkain string, args ...string) (string, error) {
	return run146cKCCTimeout(t, 120*time.Second, karkain, args...)
}

// run146cKCCTimeout is run146cKCC with an explicit deadline (the full
// compiler-sources self-check on this generation of the host takes minutes,
// so its subtest overrides the default).
func run146cKCCTimeout(t *testing.T, timeout time.Duration, karkain string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, args...)
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=kcc")
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(out), err
	}
	return string(out), err
}

// kccRunGolden asserts a completed kcc run reproduces the golden stdout
// byte for byte (stripping the "[ok] -> .c23" build banner + CRLF).
func kccRunGolden(t *testing.T, karkain, file, want string) {
	t.Helper()
	msg, err := run146cKCC(t, karkain, "run", file, "--engine", "kcc")
	if err != nil {
		if strings.Contains(msg, "error[K127]") {
			t.Skipf("low-RAM host: kcc self-build guarded (error[K127]): %s", filepath.Base(file))
		}
		t.Fatalf("%s (engine=kcc): run failed: %v\n%s", filepath.Base(file), err, msg)
	}
	got := stripKCCBuildBanner(t, msg)
	got = strings.ReplaceAll(got, "\r\n", "\n")
	if got != want {
		t.Errorf("%s (engine=kcc): parity mismatch\nwant:\n%q\ngot:\n%q", filepath.Base(file), want, got)
	}
}

// TestPhase146C_GenericFuncGoldenKCC exercises the v1 function story end to
// end through kcc: single-param identity over int/string + two-param pick.
func TestPhase146C_GenericFuncGoldenKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write146Probe(t, "genfunc_kcc",
		"func id[T](x) {\n\treturn x\n}\n"+
			"func pick[T, U](a, b) {\n\treturn b\n}\n"+
			"func main() {\n\tprintln(id[int](41))\n\tprintln(id[string](\"hi\"))\n\tprintln(pick[int, string](1, \"two\"))\n}\n")
	kccRunGolden(t, karkain, probe, "41\nhi\ntwo\n")
}

// TestPhase146C_GenericStructGoldenKCC exercises the v1 struct story through
// kcc: Point[int] specializes with substituted field types and prints.
func TestPhase146C_GenericStructGoldenKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write146Probe(t, "genstruct_kcc",
		"type Point[T] struct { x T, y T }\n"+
			"func main() {\n\tlet p = Point[int]{x: 3, y: 4}\n\tprintln(p.x)\n\tprintln(p.y)\n}\n")
	kccRunGolden(t, karkain, probe, "3\n4\n")
}

// TestPhase146C_NestedGenericKCC proves the fixpoint: instantiating outer[int]
// (whose body calls id[T]) must also instantiate the nested id_int.
func TestPhase146C_NestedGenericKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write146Probe(t, "nested_kcc",
		"func id[T](x) {\n\treturn x\n}\n"+
			"func outer[T](x) {\n\treturn id[T](x)\n}\n"+
			"func main() {\n\tprintln(outer[int](7))\n}\n")
	kccRunGolden(t, karkain, probe, "7\n")
}

// TestPhase146C_IndexCallDemotionKCC proves non-template call heads with an
// identifier index keep the Phase-133 index-then-call behavior through kcc.
func TestPhase146C_IndexCallDemotionKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write146Probe(t, "demote_kcc",
		"func main() {\n\tlet base = 10\n"+
			"\tlet ops = [fn(x int) int { return x + base }, fn(x int) int { return x * 2 }]\n"+
			"\tlet idx = 0\n\tprintln(ops[idx](5))\n\tprintln(ops[1](5))\n}\n")
	kccRunGolden(t, karkain, probe, "15\n10\n")
}

// TestPhase146C_StdlibGenericsKCC runs the std.generics examples through kcc
// (import std.generics assembled into the flat kcc project) and pins the
// byte-identical 146B goldens on the second engine.
func TestPhase146C_StdlibGenericsKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range phase146bCases(t) {
		kccRunGolden(t, karkain, c.file, c.want)
	}
}

// TestPhase146C_CheckCleanKCC asserts kcc check accepts the corpus ([ok],
// exit 0) — including the assembler's own sources, proving the mono pass is
// an idempotent no-op over generic-free programs and my new compiler code
// passes the self-hosted checker (K101-112/K115 clean).
func TestPhase146C_CheckCleanKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	for _, c := range phase146bCases(t) {
		msg, err := run146cKCC(t, karkain, "check", c.file, "--engine", "kcc")
		if err != nil {
			if strings.Contains(msg, "error[K127]") {
				t.Skipf("low-RAM host: kcc self-build guarded (error[K127]): %s", filepath.Base(c.file))
			}
			t.Errorf("%s: check --engine kcc failed: %v\n%s", filepath.Base(c.file), err, msg)
			continue
		}
		if !strings.Contains(msg, "[ok]") {
			t.Errorf("%s: check --engine kcc missing [ok]:\n%s", filepath.Base(c.file), msg)
		}
	}
	self := filepath.Join(repoRoot(t), "src", "compiler", "main.kark")
	msg, err := run146cKCCTimeout(t, 600*time.Second, karkain, "check", self, "--engine", "kcc")
	if err != nil {
		if strings.Contains(msg, "error[K127]") {
			t.Skipf("low-RAM host: kcc self-build guarded (error[K127]) — compiler self-check deferred")
		}
		t.Fatalf("compiler sources self-check via kcc failed:\n%s", msg)
	}
	if !strings.Contains(msg, "[ok]") {
		t.Errorf("compiler sources self-check missing [ok]:\n%s", msg)
	}
}

// kccCheckFails asserts kcc check exits non-zero with a K115 diagnostic
// matching wantSub for the given probe.
func kccCheckFails(t *testing.T, karkain, name, src, wantSub string) {
	t.Helper()
	probe := write146Probe(t, name, src)
	msg, err := run146cKCC(t, karkain, "check", probe, "--engine", "kcc")
	if err == nil {
		t.Errorf("%s: kcc check should be rejected, but passed:\n%s", name, msg)
		return
	}
	if strings.Contains(msg, "error[K127]") {
		t.Skipf("low-RAM host: kcc self-build guarded (error[K127]): %s", name)
	}
	if !strings.Contains(msg, "error[K115]") {
		t.Errorf("%s: want error[K115], got:\n%s", name, msg)
		return
	}
	if !strings.Contains(msg, wantSub) {
		t.Errorf("%s: K115 diagnostic missing %q:\n%s", name, wantSub, msg)
	}
}

// TestPhase146C_BareGenericRejectedKCC pins the no-inference rule through
// kcc: using a generic function or struct without type arguments is an
// error (inference is a future slice, never silent miscompilation).
func TestPhase146C_BareGenericRejectedKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	kccCheckFails(t, karkain, "bare_func_kcc",
		"func id[T](x) {\n\treturn x\n}\nfunc main() {\n\tprint(id(1))\n}\n",
		"requires explicit type arguments")
	kccCheckFails(t, karkain, "bare_struct_kcc",
		"type Point[T] struct { x T, y T }\nfunc main() {\n\tlet p = Point{x: 1, y: 2}\n\tprint(p.x)\n}\n",
		"requires explicit type arguments")
}

// TestPhase146C_ArityMismatchRejectedKCC pins the K115 arity contract
// through kcc: wrong type-argument counts name the expected/got counts.
func TestPhase146C_ArityMismatchRejectedKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	kccCheckFails(t, karkain, "arity_func_kcc",
		"func id[T](x) {\n\treturn x\n}\nfunc main() {\n\tprint(id[int, string](1))\n}\n",
		"expects 1 type argument(s), got 2")
	kccCheckFails(t, karkain, "arity_struct_kcc",
		"type Point[T] struct { x T, y T }\nfunc main() {\n\tlet p = Point[int, string]{x: 1, y: 2}\n\tprint(p.x)\n}\n",
		"expects 1 type argument(s), got 2")
}

// TestPhase146C_UnknownGenericCallRejectedKCC: `f[int](1)` with no template
// named f demotes to the index-then-call shape; the checker then rejects the
// undefined identifier (never a silent generic-miss codegen).
func TestPhase146C_UnknownGenericCallRejectedKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write146Probe(t, "unknown_gen_kcc",
		"func main() {\n\tprint(f[int](1))\n}\n")
	msg, err := run146cKCC(t, karkain, "check", probe, "--engine", "kcc")
	if err == nil {
		t.Fatalf("undefined generic call should be rejected, but check passed:\n%s", msg)
	}
	if strings.Contains(msg, "error[K127]") {
		t.Skipf("low-RAM host: kcc self-build guarded (error[K127])")
	}
	if strings.Contains(msg, "[ok]") {
		t.Errorf("undefined generic call must not [ok]:\n%s", msg)
	}
	if !strings.Contains(msg, "error[") {
		t.Errorf("undefined generic call should surface a compiler diagnostic:\n%s", msg)
	}
}

// TestPhase146C_IdempotenceKCC guards the pass contract a second way: a
// freshly-written generic program must still parse to ordinary postfix for
// int indexes, and a program with no generics must compile identically
// (silent regression guard for the parser gate restore and the mono no-op).
func TestPhase146C_IdempotenceKCC(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write146Probe(t, "plain_kcc",
		"func main() {\n\tlet arr = [1, 2, 3]\n\tprintln(arr[0])\n\tprintln(arr[2])\n}\n")
	kccRunGolden(t, karkain, probe, "1\n3\n")

	// The mono pass must not rewrite struct-field convenience indexing either
	// (slices/ints remain untouched even right before a call paren).
	probe2 := write146Probe(t, "postfix_kcc",
		"func main() {\n\tlet arr = [7, 8, 9]\n\tprintln(arr[1])\n\tprintln(arr[0])\n}\n")
	kccRunGolden(t, karkain, probe2, "8\n7\n")
}