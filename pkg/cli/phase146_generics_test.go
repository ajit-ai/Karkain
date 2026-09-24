package cli

// Phase 146A — Generics v1, Go-engine slice: explicit type arguments
// (`f[int](...)`, `Point[int]{...}`) monomorphize to plain units with
// byte-identical behavior; arity/bare-use errors surface as
// resolve-class diagnostics (exit 3); index-then-call demotion keeps
// Phase-133 programs working; `explain` documents K115; WASM keeps its
// K108 generics rejection. kcc parity is slice 146C (separate subtests).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write146Probe(t *testing.T, name, src string) string {
	t.Helper()
	probe := filepath.Join(t.TempDir(), name+".kark")
	if err := os.WriteFile(probe, []byte(src), 0644); err != nil {
		t.Fatalf("writing probe: %v", err)
	}
	return probe
}

// TestPhase146_GenericFuncGoldenGo pins the v1 function story end to end:
// single-param identity over two concrete types plus a two-param pick.
func TestPhase146_GenericFuncGoldenGo(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write146Probe(t, "genfunc",
		"func id[T](x) {\n\treturn x\n}\n"+
			"func pick[T, U](a, b) {\n\treturn b\n}\n"+
			"func main() {\n\tprintln(id[int](41))\n\tprintln(id[string](\"hi\"))\n\tprintln(pick[int, string](1, \"two\"))\n}\n")
	cmd := exec.Command(karkain, "run", probe, "--engine", "go")
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generic func golden should run:\n%s", string(out))
	}
	got := strings.ReplaceAll(string(out), "\r\n", "\n")
	if got != "41\nhi\ntwo\n" {
		t.Errorf("golden mismatch: want 41/hi/two, got %q", got)
	}
}

// TestPhase146_GenericStructGoldenGo pins the v1 struct story: a generic
// Point[T] specialized at int carries substituted field types.
func TestPhase146_GenericStructGoldenGo(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write146Probe(t, "genstruct",
		"type Point[T] struct { x T, y T }\n"+
			"func main() {\n\tlet p = Point[int]{x: 3, y: 4}\n\tprintln(p.x)\n\tprintln(p.y)\n}\n")
	cmd := exec.Command(karkain, "run", probe, "--engine", "go")
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generic struct golden should run:\n%s", string(out))
	}
	got := strings.ReplaceAll(string(out), "\r\n", "\n")
	if got != "3\n4\n" {
		t.Errorf("golden mismatch: want 3/4, got %q", got)
	}
}

// TestPhase146_ArityMismatchRejected pins the K115 contract on the Go
// engine: wrong type-argument counts fail check with exit 3 and name the
// expected/got counts (kcc reports the same shape as error[K115]).
func TestPhase146_ArityMismatchRejected(t *testing.T) {
	karkain := phase130Karkain(t)
	cases := map[string]string{
		"func":   "func id[T](x) {\n\treturn x\n}\nfunc main() {\n\tprint(id[int, string](1))\n}\n",
		"struct": "type Point[T] struct { x T, y T }\nfunc main() {\n\tlet p = Point[int, string]{x: 1, y: 2}\n\tprint(p.x)\n}\n",
	}
	for name, src := range cases {
		probe := write146Probe(t, "arity_"+name, src)
		cmd := exec.Command(karkain, "check", probe, "--engine", "go")
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("%s: arity mismatch should be rejected, but check passed", name)
			continue
		}
		if !strings.Contains(string(out), "expects 1 type argument(s), got 2") {
			t.Errorf("%s: want arity diagnostic, got:\n%s", name, string(out))
		}
	}
}

// TestPhase146_BareGenericRejected pins the no-inference rule: using a
// generic function or struct without explicit type arguments is an
// error (inference is a future slice, never silent miscompilation).
func TestPhase146_BareGenericRejected(t *testing.T) {
	karkain := phase130Karkain(t)
	cases := map[string]string{
		"func":   "func id[T](x) {\n\treturn x\n}\nfunc main() {\n\tprint(id(1))\n}\n",
		"struct": "type Point[T] struct { x T, y T }\nfunc main() {\n\tlet p = Point{x: 1, y: 2}\n\tprint(p.x)\n}\n",
	}
	for name, src := range cases {
		probe := write146Probe(t, "bare_"+name, src)
		cmd := exec.Command(karkain, "check", probe, "--engine", "go")
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("%s: bare generic use should be rejected, but check passed", name)
			continue
		}
		if !strings.Contains(string(out), "requires explicit type arguments") {
			t.Errorf("%s: want explicit-args diagnostic, got:\n%s", name, string(out))
		}
	}
}

// TestPhase146_IndexCallDemotion proves the optimistic-parse contract:
// `ops[idx](5)` with an identifier index still lowers to the Phase-133
// index-then-call (15), and int-index calls are untouched (10).
func TestPhase146_IndexCallDemotion(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write146Probe(t, "demote",
		"func main() {\n\tlet base = 10\n"+
			"\tlet ops = [fn(x int) int { return x + base }, fn(x int) int { return x * 2 }]\n"+
			"\tlet idx = 0\n\tprintln(ops[idx](5))\n\tprintln(ops[1](5))\n}\n")
	cmd := exec.Command(karkain, "run", probe, "--engine", "go")
	cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("demotion probe should run:\n%s", string(out))
	}
	got := strings.ReplaceAll(string(out), "\r\n", "\n")
	if got != "15\n10\n" {
		t.Errorf("demotion golden mismatch: want 15/10, got %q", got)
	}
}

// TestPhase146_ExplainK115 guards the documentation contract: `explain`
// knows K115 and `--list` includes it.
func TestPhase146_ExplainK115(t *testing.T) {
	karkain := phase130Karkain(t)
	out, err := exec.Command(karkain, "explain", "K115").CombinedOutput()
	if err != nil {
		t.Fatalf("explain K115 failed:\n%s", string(out))
	}
	if !strings.Contains(string(out), "K115") {
		t.Errorf("explain K115 does not mention its code:\n%s", string(out))
	}
	list, err := exec.Command(karkain, "explain", "--list").CombinedOutput()
	if err != nil {
		t.Fatalf("explain --list failed:\n%s", string(list))
	}
	if !strings.Contains(string(list), "K115") {
		t.Errorf("explain --list missing K115:\n%s", string(list))
	}
}

// TestPhase146_WasmBoundaryStays pins the v1 non-goal: the WASM backend
// keeps rejecting generics with error[K108].
func TestPhase146_WasmBoundaryStays(t *testing.T) {
	karkain := phase130Karkain(t)
	probe := write146Probe(t, "wasmgen",
		"func id[T](x) {\n\treturn x\n}\nfunc main() {\n\tprint(id[int](1))\n}\n")
	build := exec.Command(karkain, "build", probe, "--target", "wasm32-wasi")
	out, err := build.CombinedOutput()
	if err == nil {
		t.Fatalf("wasm build of generics should be rejected, but succeeded:\n%s", string(out))
	}
	if !strings.Contains(string(out), "K108") {
		t.Fatalf("want error[K108], got:\n%s", string(out))
	}
}
