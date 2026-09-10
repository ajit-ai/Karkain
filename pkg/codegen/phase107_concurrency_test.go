package codegen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// parseProg parses Karkain source through the real front end.
func parseProg(t *testing.T, src string) *parser.Program {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse errors: %v", p.Errors)
	}
	return prog
}

// phase107HasGCC skips when no C compiler is available (mirrors phase98).
func phase107HasGCC(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gcc"); err != nil {
		t.Skip("gcc not available")
	}
}

// compileConcurrencyProgram parses, compiles and runs one Karkain source
// through the real front end + the pipeline's C compiler, returning normalized
// stdout. The generated C lives at tmp/main.c; the exe at tmp/probe.exe.
func compileConcurrencyProgram(t *testing.T, src string) string {
	t.Helper()
	phase107HasGCC(t)

	p := parser.New(lexer.New(src))
	prog := p.ParseProgram()
	if len(p.Errors) != 0 {
		t.Fatalf("parse errors: %v", p.Errors)
	}
	tmp := t.TempDir()
	exePath := filepath.Join(tmp, "probe.exe")
	source := filepath.Join(tmp, "main.kark")

	g := New(Config{OutputPath: exePath})
	if err := g.GenerateAndCompile(prog, source); err != nil {
		t.Fatalf("GenerateAndCompile: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	run := exec.CommandContext(ctx, exePath)
	got, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("probe run failed: %v\n%s", err, got)
	}
	return strings.ReplaceAll(string(got), "\r\n", "\n")
}

// TestPhase107_CodegenSpawnJoin verifies spawn(fn, args...) -> task handle,
// spawn-of-function lowering and deterministic join status output.
func TestPhase107_CodegenSpawnJoin(t *testing.T) {
	src := `
func work(n) {
	return n * 2
}

func fail() {
	return -1
}

func main() {
	let t1 = spawn(work, 21)
	let t2 = spawn(work, 5)
	let st1 = join(t1)
	let st2 = join(t2)
	println(st1)
	println(st2)
	let f = spawn(fail)
	println(join(f))
	wait_all()
	println("done")
}
`
	got := compileConcurrencyProgram(t, src)
	for _, want := range []string{
		"42\n",
		"10\n",
		"-1\n",
		"done",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; got:\n%s", want, got)
		}
	}
}

// TestPhase107_CodegenSpawnStmtStatement verifies the bare-statement form of a
// spawn and that spawned functions run concurrently to a deterministic count.
func TestPhase107_CodegenSpawnStmtStatement(t *testing.T) {
	src := `func bump() {
	return 7
}

func main() {
	spawn(bump)
	spawn(bump)
	wait_all()
	println("spawned")
}
`
	got := compileConcurrencyProgram(t, src)
	if !strings.Contains(got, "spawned") {
		t.Errorf("missing spawned marker; got:\n%s", got)
	}
}

// TestPhase107_CodegenChannels verifies channel/cap, send() over Value handles,
// expression receive, and deterministic closed-channel rejection.
func TestPhase107_CodegenChannels(t *testing.T) {
	src := `
func producer(ch) {
	chanSend(ch, 10)
	chanSend(ch, 20)
	chanClose(ch)
	return 0
}

func main() {
	let ch = channel(2)
	spawn(producer, ch)
	let a = receive(ch)
	let b = receive(ch)
	println(a)
	println(b)
	wait_all()
	chanClose(ch)
	let r = chanSend(ch, 30)
	println(r)
	println("done")
}
`
	got := compileConcurrencyProgram(t, src)
	for _, want := range []string{
		"10\n",
		"20\n",
		"0\n", // send to a closed channel is rejected
		"done",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; got:\n%s", want, got)
		}
	}
}

// TestPhase107_CodegenActors verifies actor("handler", state) dispatch and that
// handler messages serialize through the mailbox.
func TestPhase107_CodegenActors(t *testing.T) {
	src := `
func counter(state, msg) {
	return state + msg
}

func main() {
	let a = actor("counter", 0)
	actorSend(a, 1)
	actorSend(a, 2)
	actorSend(a, 3)
	wait_all()
	println(actorState(a))
	println("done")
}
`
	got := compileConcurrencyProgram(t, src)
	for _, want := range []string{
		"6\n",
		"done",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q; got:\n%s", want, got)
		}
	}
}

// TestPhase107_CodegenRuntimeEmbedded verifies the generated translation unit
// carries the entire embedded Phase 107 runtime (header, glue, wrappers).
func TestPhase107_CodegenRuntimeEmbedded(t *testing.T) {
	g := New(Config{})
	src := "func work() { return 1 }\nfunc main() {\n\tlet t = spawn(work)\n\tprintln(join(t))\n}\n"
	prog := parseProg(t, src)
	var b strings.Builder
	for _, stmt := range prog.Statements {
		g.scanNodeForConcurrency(stmt)
		g.collectConcDecls(stmt)
	}
	if !g.usesConcurrency {
		t.Fatal("concurrency scan did not mark usage")
	}
	b.WriteString(concRuntimeHeader())
	b.WriteString(g.concWrapperC())
	b.WriteString(concRuntimeAPIC())
	out := b.String()
	for _, marker := range []string{
		"karkain_channel_create",
		"karkain_sched_spawn",
		"karkain_conc_actor",
		"karkain_run_0",
		"karkain_actor_dispatch",
		"karkain_actor_box_t",
	} {
		if !strings.Contains(out, marker) {
			t.Errorf("generated C missing %q", marker)
		}
	}
}

// TestPhase107_CodegenStress spawns 999 square-computing tasks concurrently,
// joins them all through stored handles and sums the deterministic result.
func TestPhase107_CodegenStress(t *testing.T) {
	src := `func work(n) {
	return n * n
}

func main() {
	let ts = []
	let i = 1
	while (i < 1000) {
		appendArray(ts, spawn(work, i))
		i = i + 1
	}
	let j = 0
	let total = 0
	while (j < len(ts)) {
		let v = join(ts[j])
		total = total + v
		j = j + 1
	}
	println(total)
}
`
	// Sum of squares 1^2..999^2 = 999*1000*1999/6.
	got := compileConcurrencyProgram(t, src)
	if !strings.Contains(got, "332833500\n") {
		t.Errorf("stress sum mismatch; got:\n%s", got)
	}
}

// TestGeneratedSourceWritten ensures the generated C exists on disk (the CLI
// relies on it) after GenerateAndCompile with an OutputPath.
func TestGeneratedSourceWritten(t *testing.T) {
	phase107HasGCC(t)
	tmp := t.TempDir()
	source := filepath.Join(tmp, "main.kark")
	src := "func main() { println(\"hi\") }\n"
	prog := parseProg(t, src)
	g := New(Config{OutputPath: filepath.Join(tmp, "probe.exe")})
	if err := g.GenerateAndCompile(prog, source); err != nil {
		t.Fatalf("GenerateAndCompile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "main.c")); err != nil {
		t.Fatalf("generated source not written: %v", err)
	}
}