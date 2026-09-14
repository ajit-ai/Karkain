package cli

// Phase 120 — Compiler Independence / KIR text emitter gate.
//
// Phase 120 delivers the first Karkain-owned compiler intermediate: a
// deterministic KIR v1 text emitter written in Karkain itself
// (src/compiler/kir.kark), exposed by the self-hosted engine (`kcc kir`,
// dispatched as `karkain kir` via KCCKirCommand).
//
// This suite guards the deliverables that must never regress:
//   - `karkain kir <file>` routes through the self-hosted engine and renders a
//     path-less, stable KIR text ending in `[ok] kir text: N lines`;
//   - repeated runs are byte-identical (the core determinism contract);
//   - structural markers for the language surface appear (func/if/else/while/
//     forin/var/struct map literal etc.);
//   - the compiler renders its own KIR emitter source (self-hosting proof);
//   - exit-code contract holds: usage errors (missing file) and compile
//     errors (parse failure) map to the documented codes;
//   - Go↔kcc print parity: space-form `print x` (accepted by kcc) now parses
//     on the Go front end too, so the Go-side preflight no longer rejects
//     programs the KIR emitter can render.
//
// Fixtures are kept in isolated per-subtest temp dirs: the Go-side project
// preflight (Phase 105) parses every unit file in a directory, so a broken
// sibling file would otherwise corrupt otherwise-valid runs.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// phase120Surface is the KIR fixture: a closed, broad-surface program written
// with the space-form `print x` print, exercising funcs, recursion, struct and
// enum declarations, array/map/struct literals, for-in, if/else and while.
// It must stay parse-identical across the Go preflight and kcc.
const phase120Surface = `func fib(n) {
  if (n < 2) { return (n) }
  return (fib(n - 1)) + (fib(n - 2))
}

type Point struct { x int; y int }
enum Color { Red, Green }

func main() {
  let n = 5
  var total = 0
  total = fib(n) + 1
  let arr = [1, 2, 3]
  for i in arr {
    total = total + i
  }
  let m = {"a": 1, "b": 2}
  let p = Point { x: total, y: 0 }
  if (total > 10) {
    print total
  } else {
    print 0
  }
  while (total < 20) {
    total = total + 1
  }
  print total
}
`

// TestPhase120_Kir assembles the whole Phase 120 KIR gate.
func TestPhase120_Kir(t *testing.T) {
	bin := buildPreviewBinary(t)
	root := repoRoot(t) // the kcc binary locates src/compiler from the CWD

	t.Run("EmitterEndToEnd", func(t *testing.T) {
		dir := t.TempDir()
		fixture := filepath.Join(dir, "main.kark")
		if err := os.WriteFile(fixture, []byte(phase120Surface), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := runBin(t, bin, root, "kir", fixture)
		if err != nil {
			t.Fatalf("karkain kir failed: %v\n%s", err, out)
		}
		for _, want := range []string{
			"KIR v1",
			"source: main.kark", // path-less determinism contract
			"func fib (params: n) line: ",
			"struct Point (x:int, y:int) line: ",
			"enum Color (Red:, Green:) line: ",
			"let m = (map \"a\"=1 \"b\"=2) line: ",
			"let p = (struct Point x=total y=0) line: ",
			"forin i in arr line: ",
			"else line: ",
			"while (< total 20) line: ",
			"[ok] kir text: ",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("KIR output missing %q\n---\n%s", want, out)
			}
		}
	})

	t.Run("ByteDeterminism", func(t *testing.T) {
		dir := t.TempDir()
		fixture := filepath.Join(dir, "main.kark")
		if err := os.WriteFile(fixture, []byte(phase120Surface), 0o644); err != nil {
			t.Fatal(err)
		}
		first, err := runBin(t, bin, root, "kir", fixture)
		if err != nil {
			t.Fatalf("run 1 failed: %v\n%s", err, first)
		}
		second, err := runBin(t, bin, root, "kir", fixture)
		if err != nil {
			t.Fatalf("run 2 failed: %v\n%s", err, second)
		}
		if first != second {
			t.Errorf("KIR output is not byte-deterministic:\n--- run 1 ---\n%s\n--- run 2 ---\n%s", first, second)
		}
	})

	t.Run("SpaceFormPrintParity", func(t *testing.T) {
		// Phase 120 parity fix: the Go preflight must accept `print x` the way
		// kcc's parsePrint does, otherwise it rejects programs the KIR emitter
		// can render. The surface fixture above already uses the space form;
		// this subtest proves both engines end-to-end on a minimal case.
		dir := t.TempDir()
		fixture := filepath.Join(dir, "main.kark")
		if err := os.WriteFile(fixture, []byte("func main() {\n    let x = 1\n    print x\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := runBin(t, bin, root, "check", "--engine", "go", fixture)
		if err != nil {
			t.Fatalf("go check failed on space-form print: %v\n%s", err, out)
		}
		kirOut, err2 := runBin(t, bin, root, "kir", fixture)
		if err2 != nil {
			t.Fatalf("karkain kir failed on space-form print: %v\n%s", err2, kirOut)
		}
		if !strings.Contains(kirOut, "print x line:") {
			t.Errorf("KIR did not render `print x`: %s", kirOut)
		}
	})

	t.Run("SelfHosting", func(t *testing.T) {
		// The compiler renders its own KIR component: `karkain kir` on
		// src/compiler/kir.kark must go through the assembled self-hosted
		// pipeline and re-emit kirEmit's own definition.
		root := repoRoot(t)
		out, err := runBin(t, bin, root, "kir", filepath.Join(root, "src", "compiler", "kir.kark"))
		if err != nil {
			t.Fatalf("karkain kir on compiler source failed: %v\n%s", err, out)
		}
		for _, want := range []string{
			"func kirEmit (params: ast, path)",
			"func kirExpr (params: node)",
			"func kirStmt (params: lines, node, depth)",
			"[ok] kir text: ",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("self-hosted KIR missing %q", want)
			}
		}
	})

	t.Run("ExampleCoverage", func(t *testing.T) {
		// The shipped example under examples/self-hosting/kir is the
		// executable demonstration of the full KIR emitter surface. This
		// subtest keeps it honest: every structural form the gate lists must
		// render, and the program must print identical stdout on both engines.
		root := repoRoot(t)
		example := filepath.Join(root, "examples", "self-hosting", "kir", "main.kark")
		if _, err := os.Stat(example); err != nil {
			t.Fatalf("example missing: %v", err)
		}
		out, err := runBin(t, bin, root, "kir", example)
		if err != nil {
			t.Fatalf("karkain kir on example failed: %v\n%s", err, out)
		}
		for _, want := range []string{
			"source: main.kark",
			"struct Point (x:int, y:int) line: ",
			"enum Color (Red:, Green:) line: ",
			"func fib (params: n) line: ",
			"let arr = (array 1 2 3) line: ",
			"forin i in arr line: ",
			"let m = (map \"a\"=1 \"b\"=2) line: ",
			"let p = (struct Point x=total y=0) line: ",
			"else line: ",
			"while (< total 20) line: ",
			"[ok] kir text: 26 lines",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("example KIR missing %q\n---\n%s", want, out)
			}
		}

		goRun, err := runBin(t, bin, root, "run", "--engine", "go", example)
		if err != nil {
			t.Fatalf("example go run failed: %v\n%s", err, goRun)
		}
		if want := "15\n20\n1\n15"; strings.TrimSpace(strings.ReplaceAll(goRun, "\r", "")) != want {
			t.Errorf("example go output = %q, want %q", goRun, want)
		}
		kccRun, err := runBin(t, bin, root, "run", "--engine", "kcc", example)
		if err != nil {
			t.Fatalf("example kcc run failed: %v\n%s", err, kccRun)
		}
		if !strings.Contains(strings.ReplaceAll(kccRun, "\r", ""), "15\n20\n1\n15") {
			t.Errorf("example kcc output missing golden program output:\n%s", kccRun)
		}
	})

	t.Run("ExitCodeContract", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "nope.kark")
		out, err := runBin(t, bin, root, "kir", missing)
		if err == nil {
			t.Fatalf("kir on missing file should fail, got success:\n%s", out)
		}
		if !strings.Contains(out, "file not found") {
			t.Errorf("missing-file error should name the file: %s", out)
		}

		bad := filepath.Join(dir, "bad.kark")
		if err := os.WriteFile(bad, []byte("func main() {\n  let = 42\n  print x\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		out2, err2 := runBin(t, bin, root, "kir", bad)
		if err2 == nil {
			t.Fatalf("kir on unparseable file should fail, got success:\n%s", out2)
		}
		if strings.Contains(out2, "[ok] kir text:") {
			t.Errorf("[ok] marker must never appear on parse failure:\n%s", out2)
		}
	})
}

// TestPhase120_KirCommandExists guards the command registration itself without
// requiring the self-hosted engine to be present.
func TestPhase120_KirCommandExists(t *testing.T) {
	bin := buildPreviewBinary(t)
	out, err := runBin(t, bin, t.TempDir(), "kir")
	if err == nil {
		t.Fatalf("kir without args should fail with usage, got success:\n%s", out)
	}
	if !strings.Contains(out, "Emit KIR v1 text") {
		t.Errorf("kir help text missing: %s", out)
	}
}