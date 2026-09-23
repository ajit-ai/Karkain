package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// Phase 141A: the SSA optimizer pipeline drives emission via emitFunctionViaIR
// (proven fold/DCE pair; Mem2Reg/CSE/LICM/SROA evaluated below). This gate
// pins the contract from the executable side for a small deterministic
// corpus:
//
//  1. Parity goldens: optimized AND legacy binaries print identical goldens
//     (runtime proof — especially the loop-carried accumulator, which is
//     the shape that miscompiles under unsound passes).
//  2. Fold pin: `2+3*4` compiles to `make_int(14)` with no surviving
//     `make_int(2/3/4)` in the function body.
//  3. DCE pin: the `if (1==2)` arm leaves no `99` in the function body.
//  4. Determinism: the same source builds byte-identical C twice.
//  5. Anti-bloat: optimized total stays within 2% of legacy (the IR path
//     is structurally more verbose — one C line per instr plus decls —
//     so strict shrink is the wrong metric; the bound fails loudly if a
//     future pass explodes output).
//
// Pass verdicts measured for this phase (see PHASE-141 report):
//   fold/DCE (module methods): sound, wired. Mem2Reg: UNSOUND on
//   loop-carried cells (cross-block rewrite gap) — stays unwired pending
//   the loop-aware slice. CSE: sound but inert on this lowerer (fresh raw
//   temps per occurrence defeat syntactic matching) — unwired, no benefit.
//   LICM/SROA: sound on the corpus but the lowerer falls back to legacy
//   for C-style for loops, so no loop IR ever reaches them — unwired until
//   lowerer coverage grows.

var phase141Corpus = []struct {
	name   string
	src    string
	golden string
}{
	{"fold", `func f() int {
    return 2 + 3 * 4
}
func main() {
    print(f())
}
`, "14\n"},
	{"cse", `func g(a, b) {
    let x = a * b
    let y = a * b
    return x + y
}
func main() {
    print(g(3, 4))
}
`, "24\n"},
	{"loop", `func main() {
    let s = 0
    for (let i = 0; i < 10; i = i + 1) {
        s = s + i
    }
    print(s)
}
`, "45\n"},
	{"dce", `func h() int {
    if (1 == 2) {
        return 99
    }
    return 7
}
func main() {
    print(h())
}
`, "7\n"},
	{"licm", `func m(a, b) {
    let s = 0
    for (let i = 0; i < 5; i = i + 1) {
        s = s + a * b
    }
    return s
}
func main() {
    print(m(3, 4))
}
`, "60\n"},
	{"sroa", `type Pair struct { x int; y int }
func main() {
    let p = Pair { x: 6, y: 7 }
    print(p.x + p.y)
}
`, "13\n"},
}

// requireGCC skips the test when no C compiler is on PATH (the gate links
// real binaries to prove runtime parity, not just IR shape).
func requireGCC(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not found")
	}
}

// normOut canonicalizes program stdout (Windows consoles translate to CRLF;
// goldens are LF — the same normalization the Phase 111 harness applies).
func normOut(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func phase141Build(t *testing.T, src, file string, disableSSA bool) (cText, stdout string) {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse: %v", p.Errors)
	}
	dir := t.TempDir()
	srcPath := filepath.Join(dir, file)
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "prog")
	cfg := NewConfig()
	cfg.DisableSSA = disableSSA
	cfg.OutputPath = exe
	g := New(cfg)
	// One retry on gcc failure: Windows file locks (AV scanner) flake
	// single compiles transiently (same class the Phase 114 parity
	// harness retries). A persistent breakage fails twice and still
	// surfaces; parse/run errors never retry.
	var genErr error
	for attempt := 0; attempt < 2; attempt++ {
		if genErr = g.GenerateAndCompile(prog, srcPath); genErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if genErr != nil {
		t.Fatalf("GenerateAndCompile(disableSSA=%v): %v", disableSSA, genErr)
	}
	cRaw, err := os.ReadFile(srcPath[:len(srcPath)-len(filepath.Ext(srcPath))] + ".c")
	if err != nil {
		t.Fatalf("read generated C: %v", err)
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatalf("run(disableSSA=%v): %v\n%s", disableSSA, err, out)
	}
	return string(cRaw), normOut(string(out))
}

// funcBody extracts the C lines of one generated function for body-scoped
// pins (preamble text must never satisfy them).
func funcBody(c, fn string) string {
	lines := strings.Split(c, "\n")
	var out []string
	in := false
	depth := 0
	for _, line := range lines {
		if !in {
			if strings.Contains(line, fn+"(") {
				in = true
			}
			continue
		}
		out = append(out, strings.TrimSpace(line))
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if strings.HasPrefix(strings.TrimSpace(line), "}") && depth <= 0 {
			break
		}
		if len(out) > 60 {
			break
		}
	}
	return strings.Join(out, "\n")
}

// countCodeLines counts non-empty, non-comment, non-directive C lines — the
// stable proxy for emitted code size (preamble is identical across variants,
// so only body differences move the total).
func countCodeLines(c string) int {
	n := 0
	for _, line := range strings.Split(c, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") {
			continue
		}
		n++
	}
	return n
}

func TestPhase141_PipelineParity(t *testing.T) {
	requireGCC(t)
	optTotal, legTotal := 0, 0
	optBodies := map[string]string{}
	for _, c := range phase141Corpus {
		optC, optOut := phase141Build(t, c.src, "main.kark", false)
		legC, legOut := phase141Build(t, c.src, "main.kark", true)
		if optOut != c.golden {
			t.Errorf("%s: optimized stdout = %q, want %q", c.name, optOut, c.golden)
		}
		if legOut != c.golden {
			t.Errorf("%s: legacy stdout = %q, want %q", c.name, legOut, c.golden)
		}
		optTotal += countCodeLines(optC)
		legTotal += countCodeLines(legC)
		t.Logf("%s: opt=%d leg=%d", c.name, countCodeLines(optC), countCodeLines(legC))
		optBodies[c.name] = optC
	}
	// Fold pin: the whole `2+3*4` is one constant in the f body.
	foldBody := funcBody(optBodies["fold"], "karkain_user_f")
	if !strings.Contains(foldBody, "make_int(14)") {
		t.Errorf("fold body lacks make_int(14):\n%s", foldBody)
	}
	for _, frag := range []string{"make_int(2)", "make_int(3)", "make_int(4)"} {
		if strings.Contains(foldBody, frag) {
			t.Errorf("fold body still carries unfolded %q:\n%s", frag, foldBody)
		}
	}
	// DCE pin: the dead arm leaves no trace in the h body.
	if body := funcBody(optBodies["dce"], "karkain_user_h"); strings.Contains(body, "99") {
		t.Errorf("dce body still carries dead 99:\n%s", body)
	}
	// Anti-bloat: within 2% of legacy (integer math, no floats).
	if optTotal*50 > legTotal*51 {
		t.Errorf("pipeline C total %d exceeds legacy %d by >2%%", optTotal, legTotal)
	} else {
		t.Logf("pipeline C total %d vs legacy %d", optTotal, legTotal)
	}
}

func TestPhase141_DeterministicBuild(t *testing.T) {
	requireGCC(t)
	a, _ := phase141Build(t, phase141Corpus[2].src, "main.kark", false)
	b, _ := phase141Build(t, phase141Corpus[2].src, "main.kark", false)
	if a != b {
		t.Fatal("optimized C differs across identical builds")
	}
}
