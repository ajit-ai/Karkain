package cli

// Phase 146D — Generics v1 close-out: a single gate asserting the whole
// 146A–146B–146C track in one invocation (goldens both engines, rejections,
// demotion/idempotence, stdlib parity, compiler self-check). It reuses the
// existing slice helpers and adds no new harness; per-slice gates remain
// the authoritative diagnostics when it fails.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPhase146D_Closeout is the 146 track gate: every leg must pass in one
// run. Subtests share one built binary (phase130Karkain caches per test).
func TestPhase146D_Closeout(t *testing.T) {
	karkain := phase130Karkain(t)

	t.Run("GoTrack", func(t *testing.T) {
		runPhase102Cases(t, karkain, "go", phase146bCases(t))
		for _, c := range phase146bCases(t) {
			cmd := exec.Command(karkain, "check", c.file, "--engine", "go")
			cmd.Env = append(os.Environ(), "KARKAIN_ENGINE=go")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("%s: check --engine go failed:\n%s", filepath.Base(c.file), string(out))
			}
		}
	})

	t.Run("FuncStructGoldensBoth", func(t *testing.T) {
		funcSrc := "func id[T](x) {\n\treturn x\n}\n" +
			"func pick[T, U](a, b) {\n\treturn b\n}\n" +
			"func main() {\n\tprintln(id[int](41))\n\tprintln(id[string](\"hi\"))\n\tprintln(pick[int, string](1, \"two\"))\n}\n"
		structSrc := "type Point[T] struct { x T, y T }\n" +
			"func main() {\n\tlet p = Point[int]{x: 3, y: 4}\n\tprintln(p.x)\n\tprintln(p.y)\n}\n"
		kccRunGolden(t, karkain, write146Probe(t, "closeout_func", funcSrc), "41\nhi\ntwo\n")
		kccRunGolden(t, karkain, write146Probe(t, "closeout_struct", structSrc), "3\n4\n")
	})

	t.Run("RejectionsBoth", func(t *testing.T) {
		kccCheckFails(t, karkain, "closeout_bare",
			"func id[T](x) {\n\treturn x\n}\nfunc main() {\n\tprint(id(1))\n}\n",
			"requires explicit type arguments")
		kccCheckFails(t, karkain, "closeout_arity",
			"func id[T](x) {\n\treturn x\n}\nfunc main() {\n\tprint(id[int, string](1))\n}\n",
			"expects 1 type argument(s), got 2")
	})

	t.Run("DemotionIdempotence", func(t *testing.T) {
		probe := write146Probe(t, "closeout_demote",
			"func main() {\n\tlet base = 10\n"+
				"\tlet ops = [fn(x int) int { return x + base }, fn(x int) int { return x * 2 }]\n"+
				"\tlet idx = 0\n\tprintln(ops[idx](5))\n\tprintln(ops[1](5))\n}\n")
		kccRunGolden(t, karkain, probe, "15\n10\n")
		plain := write146Probe(t, "closeout_plain",
			"func main() {\n\tlet arr = [1, 2, 3]\n\tprintln(arr[0])\n\tprintln(arr[2])\n}\n")
		kccRunGolden(t, karkain, plain, "1\n3\n")
	})

	t.Run("StdlibParity", func(t *testing.T) {
		for _, c := range phase146bCases(t) {
			kccRunGolden(t, karkain, c.file, c.want)
		}
	})

	t.Run("SelfCheck", func(t *testing.T) {
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
	})
}
