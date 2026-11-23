package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// P0.2 - deterministic task cancellation and bounded joins.
//
// Harness notes (all measured on the ~4GB Windows host):
//
//   - The CLI is built ONCE per test binary and the self-hosted compiler ONCE
//     per test binary. Rebuilding either per case pushed one engine's run past
//     Go's 10-minute limit; the kcc stage-1 bootstrap alone takes minutes.
//   - Case directories use gccTempDir, whose cleanup retries. t.TempDir's
//     RemoveAll fails a test when the OS/AV indexer still holds the freshly
//     compiled main.exe ("being used by another process").
//   - "Still running" is expressed with a channel rendezvous, NEVER a busy
//     loop. A task blocked in receive() is deterministically incomplete, and
//     the case finishes in milliseconds instead of spinning.
//   - Queued cancellation is forced with KARKAIN_WORKERS=1 so the victim
//     cannot be picked up by another worker before cancel() runs. The run
//     queues are LIFO, so relying on spawn order alone is racy.
//   - Every child gets a bounded context so a fixture deadlock reports as a
//     clear failure instead of a go-test timeout panic.

var (
	p02BinOnce sync.Once
	p02BinPath string
	p02BinErr  error

	p02KCCOnce sync.Once
	p02KCCPath string
)

// p02Karkain returns the CLI built once from the working tree.
func p02Karkain(t *testing.T) string {
	t.Helper()
	p02BinOnce.Do(func() { p02BinErr = buildKarkainErr(t, &p02BinPath) })
	if p02BinErr != nil {
		t.Fatalf("building karkain: %v", p02BinErr)
	}
	return p02BinPath
}

// p02KCC returns the self-hosted compiler built once from the working tree.
// An inherited KARKAIN_KCC is deliberately ignored while building: a cached
// binary from an earlier revision reports K101 for cancel/join_timeout, which
// would make this gate fail for the wrong reason.
func p02KCC(t *testing.T) string {
	t.Helper()
	hasGCC(t)
	p02KCCOnce.Do(func() {
		old, had := os.LookupEnv("KARKAIN_KCC")
		_ = os.Unsetenv("KARKAIN_KCC")
		built := phase95KCC(t)
		if had {
			_ = os.Setenv("KARKAIN_KCC", old)
		}
		// phase95KCC builds into a gccTempDir owned by the CALLING test, so the
		// binary is removed as soon as this case finishes. Copy it somewhere
		// that lives for the whole test binary; otherwise the second case sees
		// "KARKAIN_KCC points to missing binary".
		dir, err := os.MkdirTemp("", "karkain-p02-kcc")
		if err != nil {
			t.Fatalf("kcc cache dir: %v", err)
		}
		dst := filepath.Join(dir, "kcc.exe")
		data, err := os.ReadFile(built)
		if err != nil {
			t.Fatalf("read kcc: %v", err)
		}
		if err := os.WriteFile(dst, data, 0o755); err != nil {
			t.Fatalf("cache kcc: %v", err)
		}
		p02KCCPath = dst
	})
	if p02KCCPath == "" {
		t.Fatal("self-hosted compiler unavailable (its build failed in an earlier case)")
	}
	return p02KCCPath
}

// p02Engines selects the engine(s) under test. KARKAIN_P02_ENGINE exists
// because a combined single-process run of both engines is unreliable on this
// host (memory class); CI and the audit run each engine separately.
func p02Engines() []string {
	if e := os.Getenv("KARKAIN_P02_ENGINE"); e != "" {
		return []string{e}
	}
	return []string{"go", "kcc"}
}

// p02Invoke drives the real CLI over one isolated source directory. Inherited
// engine overrides are removed: a stale KARKAIN_ENGINE in the test process
// otherwise silently selects the wrong engine.
func p02Invoke(t *testing.T, engine string, args []string, src string, extraEnv ...string) (string, int) {
	t.Helper()
	v123EnsurePATH(t)
	dir := gccTempDir(t)
	srcPath := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := p02Karkain(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, append(append([]string{}, args...), srcPath)...)
	cmd.Dir = dir
	env := make([]string, 0, len(os.Environ())+4)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "KARKAIN_ENGINE=") || strings.HasPrefix(e, "KARKAIN_KCC=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env, "KARKAIN_ENGINE="+engine)
	if engine == "kcc" {
		env = append(env, "KARKAIN_KCC="+p02KCC(t))
	}
	env = append(env, extraEnv...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("%s %v timed out after 3m (program deadlock?):\n%s", engine, args, out)
	}
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run: %v", err)
		}
		code = ee.ExitCode()
	}
	return strings.ReplaceAll(string(out), "\r\n", "\n"), code
}

func p02Run(t *testing.T, engine, src string, extraEnv ...string) (string, int) {
	t.Helper()
	return p02Invoke(t, engine, []string{"run"}, src, extraEnv...)
}

func p02Check(t *testing.T, engine, src string) (string, int) {
	t.Helper()
	return p02Invoke(t, engine, []string{"check"}, src)
}

// p02Expect asserts an exact exit status and stdout.
func p02Expect(t *testing.T, engine, src, want string) {
	t.Helper()
	got, code := p02Run(t, engine, src)
	if code != 0 || got != want {
		t.Fatalf("exit %d, stdout %q, want %q", code, got, want)
	}
}

// p02Waiter announces readiness, then blocks until released and returns the
// released value. Nothing spins. Because channel() is unbounded, the readiness
// send cannot block, so receiving it proves the task is RUNNING (not queued) -
// which is what makes the running-cancellation cases deterministic.
const p02Waiter = `func waiter(ready, sig) {
  chanSend(ready, 1)
  let v = receive(sig)
  return v
}
`

// A - normal completion, bounded join, unbounded join agrees.
func TestPhaseP02_A_NormalCompletion(t *testing.T) {
	src := `func work() { return 42 }
func main() {
  let t = spawn(work)
  print(join_timeout(t, 5000))
  print(join(t))
  wait_all()
}
`
	want := "true\n42\n"
	for _, engine := range p02Engines() {
		t.Run(engine, func(t *testing.T) { p02Expect(t, engine, src, want) })
	}
}

// B - a timed join that expires must NOT cancel; the task stays joinable.
func TestPhaseP02_B_TimeoutDoesNotCancel(t *testing.T) {
	src := `func waiter(sig) {
  let v = receive(sig)
  return v
}
func main() {
  let sig = channel()
  let t = spawn(waiter, sig)
  print(join_timeout(t, 1))
  chanSend(sig, 5)
  print(join_timeout(t, 5000))
  print(join(t))
  wait_all()
}
`
	// The first timed join expires (false); the task is untouched and completes
	// once released, so the later joins both observe completion and its value.
	want := "false\ntrue\n5\n"
	for _, engine := range p02Engines() {
		t.Run(engine, func(t *testing.T) { p02Expect(t, engine, src, want) })
	}
}

// C - zero timeout on an already-completed task returns true.
func TestPhaseP02_C_ZeroTimeoutCompleted(t *testing.T) {
	src := `func work() { return 1 }
func main() {
  let t = spawn(work)
  print(join(t))
  print(join_timeout(t, 0))
  wait_all()
}
`
	want := "1\ntrue\n"
	for _, engine := range p02Engines() {
		t.Run(engine, func(t *testing.T) { p02Expect(t, engine, src, want) })
	}
}

// D - a cancelled QUEUED task never executes and joins as KARKAIN_TASK_CANCELLED.
func TestPhaseP02_D_QueuedCancellation(t *testing.T) {
	src := p02Waiter + `func victim() {
  print("VICTIM-RAN")
  return 0
}
func main() {
  let ready = channel()
  let sig = channel()
  let b = spawn(waiter, ready, sig)
  print(receive(ready))
  let v = spawn(victim)
  print(cancel(v))
  chanSend(sig, 0)
  print(join(b))
  print(join(v))
  wait_all()
}
`
	// readiness token, accepted cancel, blocker status, cancelled status.
	want := "1\ntrue\n0\n-4\n"
	for _, engine := range p02Engines() {
		t.Run(engine, func(t *testing.T) {
			// One worker: the readiness rendezvous plus a single worker makes the
			// victim provably still queued when cancel() runs.
			got, code := p02Run(t, engine, src, "KARKAIN_WORKERS=1")
			if code != 0 {
				t.Fatalf("exit %d, stdout %q", code, got)
			}
			if strings.Contains(got, "VICTIM-RAN") {
				t.Fatalf("a cancelled queued task executed: %q", got)
			}
			if got != want {
				t.Fatalf("stdout %q, want %q", got, want)
			}
		})
	}
}

// E - cancelling a RUNNING task is cooperative: the request is accepted and the
// body's result still stands, matching the documented non-preemptive contract.
func TestPhaseP02_E_RunningCancellationCooperative(t *testing.T) {
	src := p02Waiter + `func main() {
  let ready = channel()
  let sig = channel()
  let t = spawn(waiter, ready, sig)
  print(receive(ready))
  print(cancel(t))
  chanSend(sig, 9)
  print(join_timeout(t, 5000))
  print(join(t))
  wait_all()
}
`
	// readiness, accepted cancel, completed before the deadline, body's value.
	want := "1\ntrue\ntrue\n9\n"
	for _, engine := range p02Engines() {
		t.Run(engine, func(t *testing.T) { p02Expect(t, engine, src, want) })
	}
}

// F - repeated cancellation is idempotent; a completed task is not cancellable.
func TestPhaseP02_F_RepeatedCancel(t *testing.T) {
	src := p02Waiter + `func main() {
  let ready = channel()
  let sig = channel()
  let t = spawn(waiter, ready, sig)
  print(receive(ready))
  print(cancel(t))
  print(cancel(t))
  chanSend(sig, 7)
  print(join(t))
  print(cancel(t))
  wait_all()
}
`
	want := "1\ntrue\nfalse\n7\nfalse\n"
	for _, engine := range p02Engines() {
		t.Run(engine, func(t *testing.T) { p02Expect(t, engine, src, want) })
	}
}

// G - invalid handles and negative timeouts are rejected safely, never crash.
func TestPhaseP02_G_InvalidHandlesAndArgs(t *testing.T) {
	src := `func work() { return 0 }
func main() {
  print(cancel(0))
  print(join_timeout(0, 10))
  let t = spawn(work)
  print(join_timeout(t, 0 - 1))
  print(join(t))
  wait_all()
}
`
	// No handle: both calls answer false. A negative timeout is refused (false)
	// and leaves the task untouched, so the join still succeeds.
	want := "false\nfalse\nfalse\n0\n"
	for _, engine := range p02Engines() {
		t.Run(engine, func(t *testing.T) { p02Expect(t, engine, src, want) })
	}
}

// H - the existing P0.1 concurrency goldens still hold.
func TestPhaseP02_H_ExistingConcurrencyGoldens(t *testing.T) {
	sum := `func square(i) { return i * i }
func main() {
  int[] tasks = []
  int n = 0
  while (n < 10) { push(tasks, spawn(square, n)); n = n + 1 }
  int total = 0
  int i = 0
  while (i < len(tasks)) { total = total + join(tasks[i]); i = i + 1 }
  print(total)
  wait_all()
}
`
	// Fixed-size result array: the checked-in fixture pushes into an empty
	// array, which trips a known static-bounds false positive on the Go engine.
	chanSrc := `func producer(ch) {
  int i = 0
  while (i < 5) { chanSend(ch, i * 2); i = i + 1 }
  chanClose(ch)
}
func main() {
  let ch = channel()
  spawn(producer, ch)
  int[] got = [0, 0, 0, 0, 0]
  int i = 0
  while (i < 5) { got[i] = receive(ch); i = i + 1 }
  print(len(got))
  print(got[0])
  print(got[4])
  wait_all()
}
`
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"spawn_join_work_grid", sum, "285\n"},
		{"channel_producer_consumer", chanSrc, "5\n0\n8\n"},
	}
	for _, engine := range p02Engines() {
		for _, c := range cases {
			c := c
			t.Run(engine+"/"+c.name, func(t *testing.T) {
				p02Expect(t, engine, c.src, c.want)
			})
		}
	}
}

// I - P0.1 actor regression: a valid quoted handler works, and an unquoted one
// is a compile-time diagnostic on BOTH engines (the P0.2 actor probe exposed
// that kcc accepted the crashing form).
func TestPhaseP02_I_ActorRegression(t *testing.T) {
	valid := `func counter(state, msg) {
  return state + msg
}
func main() {
  let a = actor("counter", 0)
  actorSend(a, 1)
  actorSend(a, 2)
  actorSend(a, 3)
  wait_all()
  print(actorState(a))
}
`
	invalid := `func handler(a) { return a }
func main() {
  let a = actor(handler, 0)
  print(1)
}
`
	for _, engine := range p02Engines() {
		t.Run(engine+"/valid", func(t *testing.T) { p02Expect(t, engine, valid, "6\n") })
		t.Run(engine+"/invalid_handler", func(t *testing.T) {
			out, code := p02Check(t, engine, invalid)
			if code != 3 {
				t.Fatalf("exit %d, output %q; want compile error 3", code, out)
			}
			if !strings.Contains(out, "quoted") {
				t.Fatalf("diagnostic does not name the quoted-handler rule: %q", out)
			}
		})
	}
}
