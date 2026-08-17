package runtime

import (
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================
// Coroutine Tests
// ============================================================

func TestCoroutine_BasicLifecycle(t *testing.T) {
	co := NewCoroutine("basic", func(co *Coroutine) (CoroutineState, error) {
		switch co.GetCurrentState() {
		case 0:
			co.Value = "step1"
			return 1, nil
		case 1:
			co.Value = "step2"
			return 2, nil
		default:
			return 0, nil // completed
		}
	})

	if co.State != CoCreated {
		t.Errorf("expected created, got %s", co.State)
	}
	if !co.IsAlive() {
		t.Error("coroutine should be alive")
	}

	alive, _ := co.Resume()
	if !alive {
		t.Error("should be alive after first resume")
	}
	if co.GetValue() != "step1" {
		t.Errorf("expected 'step1', got %v", co.GetValue())
	}

	alive, _ = co.Resume()
	if !alive {
		t.Error("should be alive after second resume")
	}
	if co.GetValue() != "step2" {
		t.Errorf("expected 'step2', got %v", co.GetValue())
	}

	alive, _ = co.Resume()
	if alive {
		t.Error("should be done after third resume")
	}
	if co.State != CoCompleted {
		t.Errorf("expected completed, got %s", co.State)
	}
}

func TestCoroutine_YieldValue(t *testing.T) {
	co := NewCoroutine("yielder", func(co *Coroutine) (CoroutineState, error) {
		switch co.GetCurrentState() {
		case 0:
			co.Yield(42, 1)
			return 1, nil
		case 1:
			if co.GetValue() != 42 {
				t.Errorf("expected 42, got %v", co.GetValue())
			}
			return 0, nil
		default:
			return 0, nil
		}
	})

	co.Resume() // step 0 -> yields 42
	if co.State != CoSuspended {
		t.Errorf("expected suspended, got %s", co.State)
	}
	if co.GetValue() != 42 {
		t.Errorf("expected value 42, got %v", co.GetValue())
	}

	co.Resume() // step 1 -> completes
	if co.State != CoCompleted {
		t.Errorf("expected completed, got %s", co.State)
	}
}

func TestCoroutine_Error(t *testing.T) {
	co := NewCoroutine("failer", func(co *Coroutine) (CoroutineState, error) {
		return 0, &testError{"boom"}
	})

	alive, err := co.Resume()
	if alive {
		t.Error("should not be alive after error")
	}
	if err == nil {
		t.Error("expected error")
	}
	if co.State != CoFailed {
		t.Errorf("expected failed, got %s", co.State)
	}
}

func TestCoroutine_Priority(t *testing.T) {
	co := NewCoroutineWithPriority("pri", 5, func(co *Coroutine) (CoroutineState, error) {
		return 0, nil
	})
	if co.Priority != 5 {
		t.Errorf("expected priority 5, got %d", co.Priority)
	}
}

func TestCoroutine_Duration(t *testing.T) {
	co := NewCoroutine("dur", func(co *Coroutine) (CoroutineState, error) {
		return 0, nil
	})
	time.Sleep(10 * time.Millisecond)
	d := co.Duration()
	if d < 10*time.Millisecond {
		t.Errorf("expected >= 10ms, got %v", d)
	}
}

func TestCoroutineState_String(t *testing.T) {
	tests := []struct {
		state CoroutineState
		want  string
	}{
		{CoCreated, "created"},
		{CoRunning, "running"},
		{CoSuspended, "suspended"},
		{CoCompleted, "completed"},
		{CoFailed, "failed"},
		{CoroutineState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("CoroutineState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

// ============================================================
// Channel Tests
// ============================================================

func TestChannel_BufferedSendReceive(t *testing.T) {
	ch := NewChannel("int", 10)

	if ch.ElementType() != "int" {
		t.Errorf("expected 'int', got '%s'", ch.ElementType())
	}
	if ch.BufferSize() != 10 {
		t.Errorf("expected buffer 10, got %d", ch.BufferSize())
	}

	ok := ch.Send("hello", 0)
	if !ok {
		t.Fatal("send should succeed")
	}
	if ch.Len() != 1 {
		t.Errorf("expected len 1, got %d", ch.Len())
	}

	val, ok := ch.Receive(0)
	if !ok {
		t.Fatal("receive should succeed")
	}
	if val != "hello" {
		t.Errorf("expected 'hello', got %v", val)
	}
}

func TestChannel_Unbuffered(t *testing.T) {
	ch := NewChannel("int", 0)
	go func() {
		time.Sleep(10 * time.Millisecond)
		ch.Send(42, 0)
	}()

	val, ok := ch.Receive(100 * time.Millisecond)
	if !ok {
		t.Fatal("receive should succeed")
	}
	if val != 42 {
		t.Errorf("expected 42, got %v", val)
	}
}

func TestChannel_NonBlocking(t *testing.T) {
	ch := NewChannel("int", 1)
	ch.Send("full", 0)

	// Non-blocking send on full channel should fail
	ok := ch.SendNonBlocking("overflow")
	if ok {
		t.Error("non-blocking send on full channel should fail")
	}

	// Non-blocking receive should work
	val, ok := ch.ReceiveNonBlocking()
	if !ok {
		t.Fatal("non-blocking receive should succeed")
	}
	if val != "full" {
		t.Errorf("expected 'full', got %v", val)
	}

	// Non-blocking receive on empty channel
	_, ok = ch.ReceiveNonBlocking()
	if ok {
		t.Error("non-blocking receive on empty channel should fail")
	}
}

func TestChannel_Close(t *testing.T) {
	ch := NewChannel("int", 5)
	ch.Send(1, 0)
	ch.Send(2, 0)
	ch.Close()

	if !ch.IsClosed() {
		t.Error("channel should be closed")
	}

	// Should still be able to receive buffered values
	val, ok := ch.Receive(0)
	if !ok || val != 1 {
		t.Errorf("expected (1, true), got (%v, %v)", val, ok)
	}

	// Send after close should fail
	ok = ch.Send(3, 0)
	if ok {
		t.Error("send after close should fail")
	}
}

func TestChannel_Timeout(t *testing.T) {
	ch := NewChannel("int", 0)
	_, ok := ch.Receive(50 * time.Millisecond)
	if ok {
		t.Error("receive on empty unbuffered channel should timeout")
	}
}

func TestChannel_Direction(t *testing.T) {
	ch := NewChannelWithDirection("int", 5, ChSendOnly)
	if ch.Direction() != ChSendOnly {
		t.Errorf("expected send-only, got %d", ch.Direction())
	}

	ch2 := NewChannelWithDirection("int", 5, ChRecvOnly)
	if ch2.Direction() != ChRecvOnly {
		t.Errorf("expected recv-only, got %d", ch2.Direction())
	}
}

func TestChannel_ID(t *testing.T) {
	ch1 := NewChannel("int", 0)
	ch2 := NewChannel("int", 0)
	if ch1.ID() == ch2.ID() {
		t.Error("channels should have unique IDs")
	}
}

func TestChannel_LenCap(t *testing.T) {
	ch := NewChannel("int", 10)
	ch.Send(1, 0)
	ch.Send(2, 0)
	if ch.Len() != 2 {
		t.Errorf("expected len 2, got %d", ch.Len())
	}
	if ch.Cap() != 10 {
		t.Errorf("expected cap 10, got %d", ch.Cap())
	}
}

// ============================================================
// Scheduler Tests
// ============================================================

func TestScheduler_SpawnAndRun(t *testing.T) {
	sched := NewScheduler(DefaultSchedulerConfig())

	var count int32
	gt := sched.SpawnFunc("counter", func() (interface{}, error) {
		atomic.AddInt32(&count, 1)
		return "done", nil
	})

	if gt == nil {
		t.Fatal("spawn should succeed")
	}

	go sched.Run()
	sched.Wait(2 * time.Second)
	sched.Stop()

	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("expected count 1, got %d", atomic.LoadInt32(&count))
	}
}

func TestScheduler_MultipleThreads(t *testing.T) {
	sched := NewScheduler(SchedulerConfig{
		MaxGreenThreads: 10,
		Quantum:         5 * time.Millisecond,
	})

	var total int32
	for i := 0; i < 5; i++ {
		sched.SpawnFunc("t", func() (interface{}, error) {
			atomic.AddInt32(&total, 1)
			return nil, nil
		})
	}

	go sched.Run()
	sched.Wait(2 * time.Second)
	sched.Stop()

	if atomic.LoadInt32(&total) != 5 {
		t.Errorf("expected 5, got %d", atomic.LoadInt32(&total))
	}
	if sched.TotalCompleted() != 5 {
		t.Errorf("expected 5 completed, got %d", sched.TotalCompleted())
	}
}

func TestScheduler_CapacityLimit(t *testing.T) {
	sched := NewScheduler(SchedulerConfig{
		MaxGreenThreads: 2,
		Quantum:         5 * time.Millisecond,
	})

	gt1 := sched.SpawnFunc("a", func() (interface{}, error) { time.Sleep(100 * time.Millisecond); return nil, nil })
	gt2 := sched.SpawnFunc("b", func() (interface{}, error) { time.Sleep(100 * time.Millisecond); return nil, nil })
	gt3 := sched.SpawnFunc("c", func() (interface{}, error) { return nil, nil })

	if gt1 == nil || gt2 == nil {
		t.Fatal("first two spawns should succeed")
	}
	if gt3 != nil {
		t.Error("third spawn should fail (capacity limit)")
	}
}

func TestScheduler_SpawnFromCoroutine(t *testing.T) {
	sched := NewScheduler(DefaultSchedulerConfig())

	co := NewCoroutine("co1", func(co *Coroutine) (CoroutineState, error) {
		return 0, nil
	})
	gt := sched.Spawn("co1", co)
	if gt == nil {
		t.Fatal("spawn should succeed")
	}
	if sched.ThreadCount() != 1 {
		t.Errorf("expected 1 thread, got %d", sched.ThreadCount())
	}
}

func TestScheduler_Registry(t *testing.T) {
	sched := NewScheduler(DefaultSchedulerConfig())

	co := NewCoroutine("reg", func(co *Coroutine) (CoroutineState, error) {
		return 0, nil
	})
	id := sched.RegisterCoroutine(co)

	fetched := sched.GetCoroutine(id)
	if fetched == nil {
		t.Fatal("expected to find coroutine")
	}
	if fetched.Name != "reg" {
		t.Errorf("expected name 'reg', got '%s'", fetched.Name)
	}

	if sched.GetCoroutine(999) != nil {
		t.Error("expected nil for non-existent ID")
	}
}

func TestScheduler_Stats(t *testing.T) {
	sched := NewScheduler(DefaultSchedulerConfig())

	sched.SpawnFunc("ok", func() (interface{}, error) { return nil, nil })
	sched.SpawnFunc("fail", func() (interface{}, error) { return nil, &testError{"boom"} })

	go sched.Run()
	sched.Wait(2 * time.Second)
	sched.Stop()

	if sched.TotalSpawned() != 2 {
		t.Errorf("expected 2 spawned, got %d", sched.TotalSpawned())
	}
	if sched.TotalCompleted() != 1 {
		t.Errorf("expected 1 completed, got %d", sched.TotalCompleted())
	}
	if sched.TotalFailed() != 1 {
		t.Errorf("expected 1 failed, got %d", sched.TotalFailed())
	}
}

func TestScheduler_GetAllThreads(t *testing.T) {
	sched := NewScheduler(DefaultSchedulerConfig())
	sched.SpawnFunc("a", func() (interface{}, error) { time.Sleep(200 * time.Millisecond); return nil, nil })
	sched.SpawnFunc("b", func() (interface{}, error) { time.Sleep(200 * time.Millisecond); return nil, nil })

	threads := sched.GetAllThreads()
	if len(threads) != 2 {
		t.Errorf("expected 2 threads, got %d", len(threads))
	}
}

func TestScheduler_Stop(t *testing.T) {
	sched := NewScheduler(DefaultSchedulerConfig())

	// Use a yielding coroutine that checks stopCh via the scheduler loop
	co := NewCoroutine("yielder", func(co *Coroutine) (CoroutineState, error) {
		// Yield at state 0, will be rescheduled
		return 1, nil
	})
	sched.Spawn("yielder", co)

	go sched.Run()
	time.Sleep(10 * time.Millisecond)
	sched.Stop()
	time.Sleep(200 * time.Millisecond)

	if sched.IsRunning() {
		t.Error("scheduler should not be running after stop")
	}
}

func TestScheduler_DoubleRun(t *testing.T) {
	sched := NewScheduler(DefaultSchedulerConfig())
	sched.SpawnFunc("a", func() (interface{}, error) { return nil, nil })

	go sched.Run()
	sched.Wait(time.Second)
	// Second Run should be no-op
	sched.Run()
	sched.Stop()
}

// ============================================================
// Future Tests
// ============================================================

func TestFuture_Resolve(t *testing.T) {
	f := NewFuture()
	if f.IsResolved() {
		t.Error("should not be resolved initially")
	}

	f.Resolve(42)
	if !f.IsResolved() {
		t.Error("should be resolved")
	}
	if f.Result() != 42 {
		t.Errorf("expected 42, got %v", f.Result())
	}
}

func TestFuture_Reject(t *testing.T) {
	f := NewFuture()
	f.Reject(&testError{"fail"})
	if !f.IsResolved() {
		t.Error("should be resolved after reject")
	}
	if f.Error() == nil {
		t.Error("expected error")
	}
}

func TestFuture_Wait(t *testing.T) {
	f := NewFuture()
	go func() {
		time.Sleep(20 * time.Millisecond)
		f.Resolve("done")
	}()

	val, err := f.Wait(2 * time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "done" {
		t.Errorf("expected 'done', got %v", val)
	}
}

func TestFuture_WaitTimeout(t *testing.T) {
	f := NewFuture()
	_, err := f.Wait(50 * time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

// ============================================================
// GreenThread Tests
// ============================================================

func TestGreenThread_Stack(t *testing.T) {
	gt := NewGreenThread("stack_test", NewCoroutine("inner", func(co *Coroutine) (CoroutineState, error) {
		return 0, nil
	}))

	gt.Push(10)
	gt.Push(20)
	gt.Push(30)

	val, ok := gt.Pop()
	if !ok || val != 30 {
		t.Errorf("expected (30, true), got (%v, %v)", val, ok)
	}

	val, ok = gt.Pop()
	if !ok || val != 20 {
		t.Errorf("expected (20, true), got (%v, %v)", val, ok)
	}

	gt.Push(40)
	val, ok = gt.Pop()
	if !ok || val != 40 {
		t.Errorf("expected (40, true), got (%v, %v)", val, ok)
	}

	val, ok = gt.Pop()
	if !ok || val != 10 {
		t.Errorf("expected (10, true), got (%v, %v)", val, ok)
	}

	val, ok = gt.Pop()
	if ok {
		t.Error("pop on empty stack should return false")
	}
}

func TestGreenThread_ResultAndError(t *testing.T) {
	gt := NewGreenThread("res", NewCoroutine("inner", func(co *Coroutine) (CoroutineState, error) {
		return 0, nil
	}))

	gt.SetResult("result")
	if gt.Result() != "result" {
		t.Errorf("expected 'result', got %v", gt.Result())
	}
	if gt.State != CoCompleted {
		t.Error("should be completed")
	}

	gt2 := NewGreenThread("err", NewCoroutine("inner2", func(co *Coroutine) (CoroutineState, error) {
		return 0, nil
	}))
	gt2.SetError(&testError{"oops"})
	if gt2.Error() == nil {
		t.Error("expected error")
	}
	if gt2.State != CoFailed {
		t.Error("should be failed")
	}
}

func TestGreenThread_BindChannel(t *testing.T) {
	gt := NewGreenThread("ch_bound", NewCoroutine("inner", func(co *Coroutine) (CoroutineState, error) {
		return 0, nil
	}))
	ch := NewChannel("int", 10)
	gt.BindChannel(ch)
	if gt.channel != ch {
		t.Error("channel should be bound")
	}
}

// ============================================================
// Select Tests
// ============================================================

func TestSelect_RecvFires(t *testing.T) {
	ch1 := NewChannel("int", 1)
	ch2 := NewChannel("int", 1)
	ch1.Send("from_ch1", 0)

	result := ""
	idx := Select([]SelectCaseRt{
		{Type: SelectRecv, Channel: ch1, Action: func(v interface{}) { result = v.(string) }},
		{Type: SelectRecv, Channel: ch2},
	}, nil, 0)

	if idx != 0 {
		t.Errorf("expected case 0, got %d", idx)
	}
	if result != "from_ch1" {
		t.Errorf("expected 'from_ch1', got '%s'", result)
	}
}

func TestSelect_SendFires(t *testing.T) {
	ch := NewChannel("int", 1)

	result := ""
	idx := Select([]SelectCaseRt{
		{Type: SelectSend, Channel: ch, Value: "hello", Action: func(v interface{}) { result = v.(string) }},
	}, nil, 0)

	if idx != 0 {
		t.Errorf("expected case 0, got %d", idx)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got '%s'", result)
	}

	val, ok := ch.ReceiveNonBlocking()
	if !ok || val != "hello" {
		t.Errorf("expected 'hello', got %v", val)
	}
}

func TestSelect_Default(t *testing.T) {
	ch := NewChannel("int", 0) // unbuffered, nothing to send

	defaultFired := false
	idx := Select([]SelectCaseRt{
		{Type: SelectSend, Channel: ch, Value: "nope"},
	}, func() {
		defaultFired = true
	}, 0)

	if idx != -1 {
		t.Errorf("expected default (-1), got %d", idx)
	}
	if !defaultFired {
		t.Error("default should have fired")
	}
}

func TestSelect_Timeout(t *testing.T) {
	idx := Select(nil, nil, 50*time.Millisecond)
	if idx != -1 {
		t.Errorf("expected -1 on timeout, got %d", idx)
	}
}

// ============================================================
// AwaitAll Tests
// ============================================================

func TestAwaitAll_AllComplete(t *testing.T) {
	c1 := NewCoroutine("c1", func(co *Coroutine) (CoroutineState, error) {
		co.Value = "one"
		return 0, nil
	})
	c2 := NewCoroutine("c2", func(co *Coroutine) (CoroutineState, error) {
		co.Value = "two"
		return 0, nil
	})

	results, errs := AwaitAll([]*Coroutine{c1, c2}, 2*time.Second)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, e := range errs {
		if e != nil {
			t.Errorf("unexpected error: %v", e)
		}
	}
}

func TestAwaitAll_Timeout(t *testing.T) {
	slow := NewCoroutine("slow", func(co *Coroutine) (CoroutineState, error) {
		// Simulate a long-running coroutine by yielding repeatedly
		switch co.GetCurrentState() {
		case 0:
			return 1, nil // yield
		default:
			time.Sleep(100 * time.Millisecond)
			return 0, nil
		}
	})

	_, _ = AwaitAll([]*Coroutine{slow}, 50*time.Millisecond)
}

// ============================================================
// Integration: Scheduler + Coroutine + Channel
// ============================================================

func TestIntegration_CoroutineThroughScheduler(t *testing.T) {
	sched := NewScheduler(DefaultSchedulerConfig())

	var count int32
	for i := 0; i < 10; i++ {
		co := NewCoroutine("worker", func(co *Coroutine) (CoroutineState, error) {
			atomic.AddInt32(&count, 1)
			return 0, nil
		})
		sched.Spawn("worker", co)
	}

	go sched.Run()
	sched.Wait(2 * time.Second)
	sched.Stop()

	if atomic.LoadInt32(&count) != 10 {
		t.Errorf("expected 10, got %d", atomic.LoadInt32(&count))
	}
}

func TestIntegration_ChannelBetweenCoroutines(t *testing.T) {
	ch := NewChannel("int", 5)

	// Producer
	co1 := NewCoroutine("producer", func(co *Coroutine) (CoroutineState, error) {
		ch.Send("data", time.Second)
		return 0, nil
	})

	// Consumer
	co2 := NewCoroutine("consumer", func(co *Coroutine) (CoroutineState, error) {
		val, ok := ch.Receive(time.Second)
		if ok {
			co.Value = val
		}
		return 0, nil
	})

	sched := NewScheduler(DefaultSchedulerConfig())
	sched.Spawn("producer", co1)
	sched.Spawn("consumer", co2)

	go sched.Run()
	sched.Wait(2 * time.Second)
	sched.Stop()

	if co2.GetValue() != "data" {
		t.Errorf("consumer should have 'data', got %v", co2.GetValue())
	}
}

func TestIntegration_ChannelCloseDetection(t *testing.T) {
	ch := NewChannel("int", 1)
	ch.Send("only", 0)
	ch.Close()

	val, ok := ch.Receive(time.Second)
	if !ok || val != "only" {
		t.Errorf("expected ('only', true), got (%v, %v)", val, ok)
	}

	_, ok = ch.Receive(50 * time.Millisecond)
	if ok {
		t.Error("receive from closed channel should return false")
	}
}

// ============================================================
// Helper types
// ============================================================

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
