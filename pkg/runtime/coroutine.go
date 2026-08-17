package runtime

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// Phase 38: Coroutine/Async Runtime & Green Thread Scheduler
// Stackless coroutines via state machines, M:N green thread
// scheduler, typed channels with select multiplexer
// ============================================================

// ============================================================
// Coroutine State Machine
// ============================================================

// CoroutineState represents the lifecycle state of a coroutine
type CoroutineState int32

const (
	CoCreated  CoroutineState = iota // initial state
	CoRunning                        // actively executing
	CoSuspended                      // yielded / awaiting
	CoCompleted                      // finished normally
	CoFailed                         // finished with error
)

func (cs CoroutineState) String() string {
	switch cs {
	case CoCreated:
		return "created"
	case CoRunning:
		return "running"
	case CoSuspended:
		return "suspended"
	case CoCompleted:
		return "completed"
	case CoFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// CoroutineID is a unique coroutine identifier
type CoroutineID uint64

// Coroutine is a stackless coroutine implemented as a state machine.
// Each suspend point maps to a numbered state; the body function is
// re-entered from the saved state after resumption.
type Coroutine struct {
	ID       CoroutineID
	Name     string
	State    CoroutineState
	Priority int // scheduling priority (higher = earlier)

	// State machine
	currentState int // which suspend point we resume from
	totalStates  int // how many suspend points exist

	// User-provided body: called with current state; must return
	// the next state to transition to (same state = yield again).
	body func(co *Coroutine) (CoroutineState, error)

	// Values passed through await/yield
	Value interface{} // value received from channel or passed by send
	Error error       // error if failed

	// Lifecycle
	createdAt time.Time
	suspendedAt time.Time
	completedAt time.Time
	result     interface{}

	// For scheduler integration
	scheduler *Scheduler
	greenThread *GreenThread
}

// NewCoroutine creates a new coroutine with the given name and body.
// The body function is called on each resumption; it should:
//   - Check co.currentState to determine which section to execute
//   - Call co.Yield(value) to suspend with a value
//   - Return CoCompleted when done
func NewCoroutine(name string, body func(co *Coroutine) (CoroutineState, error)) *Coroutine {
	return &Coroutine{
		Name:      name,
		State:     CoCreated,
		Priority:  0,
		body:      body,
		createdAt: time.Now(),
	}
}

// NewCoroutineWithPriority creates a coroutine with a given priority
func NewCoroutineWithPriority(name string, priority int, body func(co *Coroutine) (CoroutineState, error)) *Coroutine {
	co := NewCoroutine(name, body)
	co.Priority = priority
	return co
}

// Resume advances the coroutine. Returns true if the coroutine
// is still alive (running or suspended), false if completed/failed.
func (co *Coroutine) Resume() (alive bool, err error) {
	if co.State == CoCompleted || co.State == CoFailed {
		return false, co.Error
	}

	co.State = CoRunning
	nextState, err := co.body(co)
	if err != nil {
		co.State = CoFailed
		co.Error = err
		co.completedAt = time.Now()
		return false, err
	}

	// body returns the next state to execute from on next resume
	co.currentState = int(nextState)

	if nextState == 0 {
		// Reached the end
		co.State = CoCompleted
		co.completedAt = time.Now()
		return false, nil
	}

	// Suspended (yielded)
	co.State = CoSuspended
	co.suspendedAt = time.Now()
	return true, nil
}

// Yield suspends the coroutine and passes a value to the caller.
// targetState is the state to resume from next time.
func (co *Coroutine) Yield(value interface{}, targetState int) (CoroutineState, error) {
	co.Value = value
	return CoSuspended, nil // signals suspend to Resume()
}

// SendValue provides a value to the coroutine from an external source
func (co *Coroutine) SendValue(value interface{}) {
	co.Value = value
}

// GetValue returns the last value yielded or received
func (co *Coroutine) GetValue() interface{} {
	return co.Value
}

// IsAlive returns true if the coroutine can be resumed
func (co *Coroutine) IsAlive() bool {
	return co.State == CoCreated || co.State == CoSuspended || co.State == CoRunning
}

// GetCurrentState returns the current suspend point index
func (co *Coroutine) GetCurrentState() int {
	return co.currentState
}

// Duration returns how long the coroutine has existed
func (co *Coroutine) Duration() time.Duration {
	if !co.completedAt.IsZero() {
		return co.completedAt.Sub(co.createdAt)
	}
	return time.Since(co.createdAt)
}

// ============================================================
// Green Thread — coroutine + scheduler integration
// ============================================================

// GreenThread is a lightweight thread managed by the Scheduler.
// It wraps a coroutine and provides stack-based execution for
// simpler async patterns.
type GreenThread struct {
	ID         uint64
	Name       string
	State      CoroutineState
	Coroutine  *Coroutine
	Stack      []interface{} // value stack for simple computation
	channel    *Channel       // bound channel, if any
	result     interface{}
	err        error
	yielded    int32 // atomic: 1 if yielded
	CreatedAt  time.Time
	ParentID   uint64 // parent thread ID (for spawn trees)
}

// NewGreenThread creates a new green thread wrapping a coroutine
func NewGreenThread(name string, co *Coroutine) *GreenThread {
	gt := &GreenThread{
		ID:        atomic.AddUint64(&greenThreadCounter, 1),
		Name:      name,
		State:     CoCreated,
		Coroutine: co,
		Stack:     make([]interface{}, 0, 16),
		CreatedAt: time.Now(),
	}
	co.greenThread = gt
	return gt
}

var greenThreadCounter uint64

// Push pushes a value onto the thread's stack
func (gt *GreenThread) Push(value interface{}) {
	gt.Stack = append(gt.Stack, value)
}

// Pop pops a value from the thread's stack
func (gt *GreenThread) Pop() (interface{}, bool) {
	if len(gt.Stack) == 0 {
		return nil, false
	}
	top := gt.Stack[len(gt.Stack)-1]
	gt.Stack = gt.Stack[:len(gt.Stack)-1]
	return top, true
}

// SetResult sets the final result of the thread
func (gt *GreenThread) SetResult(result interface{}) {
	gt.result = result
	gt.State = CoCompleted
}

// Result returns the thread's result
func (gt *GreenThread) Result() interface{} {
	return gt.result
}

// SetError sets an error on the thread
func (gt *GreenThread) SetError(err error) {
	gt.err = err
	gt.State = CoFailed
}

// Error returns the thread's error
func (gt *GreenThread) Error() error {
	return gt.err
}

// BindChannel binds a channel to this thread
func (gt *GreenThread) BindChannel(ch *Channel) {
	gt.channel = ch
}

// ============================================================
// Channel — typed communication between coroutines
// ============================================================

// ChannelDirection represents send/recv capability
type ChannelDirection int

const (
	ChBidirectional ChannelDirection = iota
	ChSendOnly
	ChRecvOnly
)

// Channel is a typed communication primitive for coroutines.
// Supports buffered and unbuffered (synchronous) modes.
type Channel struct {
	id       uint64
	elementType string
	bufferSize int
	dir      ChannelDirection
	buffer   chan interface{}
	closed   int32
	mu       sync.RWMutex
	sendWaiters  []*Coroutine
	recvWaiters  []*Coroutine
}

var channelCounter uint64

// NewChannel creates a new typed channel with optional buffer size.
// bufferSize=0 creates a synchronous (unbuffered) channel.
func NewChannel(elementType string, bufferSize int) *Channel {
	size := bufferSize
	if size < 0 {
		size = 0
	}
	return &Channel{
		id:          atomic.AddUint64(&channelCounter, 1),
		elementType: elementType,
		bufferSize:  size,
		dir:         ChBidirectional,
		buffer:      make(chan interface{}, size),
	}
}

// NewChannelWithDirection creates a channel with a specific direction
func NewChannelWithDirection(elementType string, bufferSize int, dir ChannelDirection) *Channel {
	ch := NewChannel(elementType, bufferSize)
	ch.dir = dir
	return ch
}

// ID returns the channel's unique identifier
func (ch *Channel) ID() uint64 {
	return ch.id
}

// ElementType returns the channel's element type
func (ch *Channel) ElementType() string {
	return ch.elementType
}

// BufferSize returns the channel's buffer capacity
func (ch *Channel) BufferSize() int {
	return ch.bufferSize
}

// Direction returns the channel's direction
func (ch *Channel) Direction() ChannelDirection {
	return ch.dir
}

// Send sends a value to the channel. Blocks if unbuffered and no receiver.
// Returns false if the channel is closed.
func (ch *Channel) Send(value interface{}, timeout time.Duration) bool {
	ch.mu.RLock()
	if atomic.LoadInt32(&ch.closed) == 1 {
		ch.mu.RUnlock()
		return false
	}
	ch.mu.RUnlock()

	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case ch.buffer <- value:
			return true
		case <-timer.C:
			return false
		}
	}

	ch.buffer <- value
	return true
}

// SendNonBlocking attempts a non-blocking send.
// Returns true if sent, false if would block.
func (ch *Channel) SendNonBlocking(value interface{}) bool {
	if atomic.LoadInt32(&ch.closed) == 1 {
		return false
	}
	select {
	case ch.buffer <- value:
		return true
	default:
		return false
	}
}

// Receive receives a value from the channel. Blocks if empty.
// Returns (value, true) on success, (nil, false) if closed/empty.
func (ch *Channel) Receive(timeout time.Duration) (interface{}, bool) {
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case val, ok := <-ch.buffer:
			return val, ok
		case <-timer.C:
			return nil, false
		}
	}

	val, ok := <-ch.buffer
	return val, ok
}

// ReceiveNonBlocking attempts a non-blocking receive.
func (ch *Channel) ReceiveNonBlocking() (interface{}, bool) {
	select {
	case val, ok := <-ch.buffer:
		return val, ok
	default:
		return nil, false
	}
}

// Close closes the channel. Subsequent sends return false.
func (ch *Channel) Close() {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	if atomic.CompareAndSwapInt32(&ch.closed, 0, 1) {
		close(ch.buffer)
	}
}

// IsClosed returns true if the channel has been closed
func (ch *Channel) IsClosed() bool {
	return atomic.LoadInt32(&ch.closed) == 1
}

// Len returns the current number of buffered elements
func (ch *Channel) Len() int {
	return len(ch.buffer)
}

// Cap returns the buffer capacity
func (ch *Channel) Cap() int {
	return cap(ch.buffer)
}

// ============================================================
// Select — multiplexed channel operations
// ============================================================

// SelectCase represents one case in a select statement
type SelectCaseType int

const (
	SelectSend SelectCaseType = iota
	SelectRecv
)

// SelectCaseRt is a runtime select case
type SelectCaseRt struct {
	Type    SelectCaseType
	Channel *Channel
	Value   interface{} // for send: the value to send; for recv: received value
	Action  func(value interface{})
}

// Select multiplexes over multiple channel operations.
// Returns the index of the case that fired, or -1 for default.
func Select(cases []SelectCaseRt, defaultCase func(), timeout time.Duration) int {
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		for {
			for i, sc := range cases {
				switch sc.Type {
				case SelectSend:
					if sc.Channel.SendNonBlocking(sc.Value) {
						if sc.Action != nil {
							sc.Action(sc.Value)
						}
						return i
					}
				case SelectRecv:
					if val, ok := sc.Channel.ReceiveNonBlocking(); ok {
						if sc.Action != nil {
							sc.Action(val)
						}
						return i
					}
				}
			}
			if defaultCase != nil {
				defaultCase()
				return -1
			}
			select {
			case <-timer.C:
				return -1
			default:
				time.Sleep(time.Microsecond)
			}
		}
	}

	for {
		for i, sc := range cases {
			switch sc.Type {
			case SelectSend:
				if sc.Channel.SendNonBlocking(sc.Value) {
					if sc.Action != nil {
						sc.Action(sc.Value)
					}
					return i
				}
			case SelectRecv:
				if val, ok := sc.Channel.ReceiveNonBlocking(); ok {
					if sc.Action != nil {
						sc.Action(val)
					}
					return i
				}
			}
		}
		if defaultCase != nil {
			defaultCase()
			return -1
		}
		time.Sleep(time.Microsecond)
	}
}

// ============================================================
// Scheduler — M:N Green Thread Scheduler
// ============================================================

// SchedulerConfig holds configuration for the green thread scheduler
type SchedulerConfig struct {
	MaxGreenThreads int           // maximum concurrent green threads
	Quantum         time.Duration // time slice per thread
	Preemptive      bool          // enable preemptive scheduling
}

// DefaultSchedulerConfig returns sensible defaults
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		MaxGreenThreads: 256,
		Quantum:         10 * time.Millisecond,
		Preemptive:      false,
	}
}

// Scheduler manages green threads and coroutines using M:N scheduling.
// N green threads are multiplexed over M OS threads (goroutines).
type Scheduler struct {
	config    SchedulerConfig
	threads   []*GreenThread
	threadMu  sync.RWMutex
	runQueue  []*GreenThread
	runQueueMu sync.Mutex
	blocked   []*GreenThread
	blockedMu sync.RWMutex

	// Coroutine registry
	coroutines   map[CoroutineID]*Coroutine
	coMu         sync.RWMutex
	nextCoID     uint64

	// Stats
	totalSpawned   uint64
	totalCompleted uint64
	totalFailed    uint64
	activeThreads  int32 // atomic: threads still alive

	running int32 // atomic
	stopCh  chan struct{}
}

// NewScheduler creates a new green thread scheduler
func NewScheduler(config SchedulerConfig) *Scheduler {
	if config.MaxGreenThreads <= 0 {
		config.MaxGreenThreads = 256
	}
	if config.Quantum <= 0 {
		config.Quantum = 10 * time.Millisecond
	}
	return &Scheduler{
		config:     config,
		threads:    make([]*GreenThread, 0, config.MaxGreenThreads),
		runQueue:   make([]*GreenThread, 0, 64),
		blocked:    make([]*GreenThread, 0, 64),
		coroutines: make(map[CoroutineID]*Coroutine),
		stopCh:     make(chan struct{}),
	}
}

// Spawn creates a new green thread from a coroutine and adds it to the run queue
func (s *Scheduler) Spawn(name string, co *Coroutine) *GreenThread {
	s.threadMu.Lock()
	defer s.threadMu.Unlock()

	if len(s.threads) >= s.config.MaxGreenThreads {
		return nil // at capacity
	}

	gt := NewGreenThread(name, co)
	co.scheduler = s
	s.threads = append(s.threads, gt)

	atomic.AddUint64(&s.totalSpawned, 1)
	atomic.AddInt32(&s.activeThreads, 1)

	s.runQueueMu.Lock()
	s.runQueue = append(s.runQueue, gt)
	s.runQueueMu.Unlock()

	return gt
}

// SpawnFunc creates a green thread from a simple function
func (s *Scheduler) SpawnFunc(name string, fn func() (interface{}, error)) *GreenThread {
	co := NewCoroutine(name, func(co *Coroutine) (CoroutineState, error) {
		result, err := fn()
		if err != nil {
			return CoFailed, err
		}
		co.Value = result
		return 0, nil // 0 signals completion
	})
	return s.Spawn(name, co)
}

// RegisterCoroutine adds a coroutine to the registry
func (s *Scheduler) RegisterCoroutine(co *Coroutine) CoroutineID {
	s.coMu.Lock()
	defer s.coMu.Unlock()
	id := CoroutineID(atomic.AddUint64(&s.nextCoID, 1))
	co.ID = id
	s.coroutines[id] = co
	return id
}

// GetCoroutine retrieves a coroutine by ID
func (s *Scheduler) GetCoroutine(id CoroutineID) *Coroutine {
	s.coMu.RLock()
	defer s.coMu.RUnlock()
	return s.coroutines[id]
}

// Run executes the scheduler until all threads complete or Stop is called.
func (s *Scheduler) Run() {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&s.running, 0)

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		s.runQueueMu.Lock()
		if len(s.runQueue) == 0 {
		s.runQueueMu.Unlock()
		// Check if there are blocked threads or all threads done
		active := atomic.LoadInt32(&s.activeThreads)
		if active == 0 {
			return
		}
			// Check blocked threads
			s.blockedMu.RLock()
			if len(s.blocked) == 0 {
				s.blockedMu.RUnlock()
				time.Sleep(time.Microsecond)
				continue
			}
			s.blockedMu.RUnlock()
			time.Sleep(time.Microsecond)
			continue
		}

		// Pick next thread (priority-sorted)
		best := 0
		for i := 1; i < len(s.runQueue); i++ {
			if s.runQueue[i].Coroutine != nil &&
				s.runQueue[best].Coroutine != nil &&
				s.runQueue[i].Coroutine.Priority > s.runQueue[best].Coroutine.Priority {
				best = i
			}
		}
		gt := s.runQueue[best]
		s.runQueue = append(s.runQueue[:best], s.runQueue[best+1:]...)
		s.runQueueMu.Unlock()

		// Execute one quantum
		s.executeThread(gt)
	}
}

// executeThread runs a green thread for one quantum
func (s *Scheduler) executeThread(gt *GreenThread) {
	if gt.Coroutine == nil {
		gt.State = CoCompleted
		return
	}

	gt.State = CoRunning
	alive, err := gt.Coroutine.Resume()

	if err != nil {
		gt.State = CoFailed
		gt.err = err
		atomic.AddUint64(&s.totalFailed, 1)
		atomic.AddInt32(&s.activeThreads, -1)
		return
	}

	if !alive {
		gt.State = CoCompleted
		gt.result = gt.Coroutine.Value
		atomic.AddUint64(&s.totalCompleted, 1)
		atomic.AddInt32(&s.activeThreads, -1)
		return
	}

	// Still alive — put back in run queue or move to blocked
	if gt.Coroutine.State == CoSuspended {
		// Check if waiting on a channel
		if gt.channel != nil && gt.channel.Len() == 0 {
			s.blockedMu.Lock()
			s.blocked = append(s.blocked, gt)
			s.blockedMu.Unlock()
			return
		}
	}
	s.runQueueMu.Lock()
	s.runQueue = append(s.runQueue, gt)
	s.runQueueMu.Unlock()
}

// UnblockThread moves a blocked thread back to the run queue
func (s *Scheduler) UnblockThread(gt *GreenThread) {
	s.blockedMu.Lock()
	for i, b := range s.blocked {
		if b.ID == gt.ID {
			s.blocked = append(s.blocked[:i], s.blocked[i+1:]...)
			break
		}
	}
	s.blockedMu.Unlock()

	s.runQueueMu.Lock()
	s.runQueue = append(s.runQueue, gt)
	s.runQueueMu.Unlock()
}

// Stop signals the scheduler to stop
func (s *Scheduler) Stop() {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

// IsRunning returns true if the scheduler is active
func (s *Scheduler) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1
}

// ThreadCount returns the number of active green threads
func (s *Scheduler) ThreadCount() int {
	s.threadMu.RLock()
	defer s.threadMu.RUnlock()
	return len(s.threads)
}

// RunQueueLen returns the number of threads in the run queue
func (s *Scheduler) RunQueueLen() int {
	s.runQueueMu.Lock()
	defer s.runQueueMu.Unlock()
	return len(s.runQueue)
}

// BlockedCount returns the number of blocked threads
func (s *Scheduler) BlockedCount() int {
	s.blockedMu.RLock()
	defer s.blockedMu.RUnlock()
	return len(s.blocked)
}

// TotalSpawned returns the total number of green threads spawned
func (s *Scheduler) TotalSpawned() uint64 {
	return atomic.LoadUint64(&s.totalSpawned)
}

// TotalCompleted returns the total number of completed green threads
func (s *Scheduler) TotalCompleted() uint64 {
	return atomic.LoadUint64(&s.totalCompleted)
}

// TotalFailed returns the total number of failed green threads
func (s *Scheduler) TotalFailed() uint64 {
	return atomic.LoadUint64(&s.totalFailed)
}

// GetAllThreads returns a snapshot of all threads
func (s *Scheduler) GetAllThreads() []*GreenThread {
	s.threadMu.RLock()
	defer s.threadMu.RUnlock()
	out := make([]*GreenThread, len(s.threads))
	copy(out, s.threads)
	return out
}

// Wait blocks until all threads complete or timeout
func (s *Scheduler) Wait(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return false
		default:
		}
		s.runQueueMu.Lock()
		rq := len(s.runQueue)
		s.runQueueMu.Unlock()
		s.blockedMu.RLock()
		bk := len(s.blocked)
		s.blockedMu.RUnlock()
		if rq == 0 && bk == 0 {
			return true
		}
		time.Sleep(time.Millisecond)
	}
}

// ============================================================
// AwaitAll — wait for multiple coroutines
// ============================================================

// AwaitAll waits for multiple coroutines to complete and returns their results
func AwaitAll(coroutines []*Coroutine, timeout time.Duration) ([]interface{}, []error) {
	results := make([]interface{}, len(coroutines))
	errs := make([]error, len(coroutines))
	var wg sync.WaitGroup

	for i, co := range coroutines {
		wg.Add(1)
		go func(idx int, c *Coroutine) {
			defer wg.Done()
			deadline := time.Now().Add(timeout)
			for c.IsAlive() {
				if time.Now().After(deadline) {
					errs[idx] = fmt.Errorf("timeout waiting for coroutine %s", c.Name)
					return
				}
				alive, err := c.Resume()
				if err != nil {
					errs[idx] = err
					return
				}
				if !alive {
					results[idx] = c.Value
					return
				}
				time.Sleep(time.Microsecond)
			}
			results[idx] = c.Value
		}(i, co)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return results, errs
	case <-time.After(timeout):
		return results, errs
	}
}

// ============================================================
// Async/Await helpers
// ============================================================

// Future represents a pending asynchronous result
type Future struct {
	id       uint64
	result   interface{}
	err      error
	done     int32 // atomic: 1 when resolved
	awaiters []*Coroutine
	mu       sync.Mutex
}

var futureCounter uint64

// NewFuture creates a new future
func NewFuture() *Future {
	return &Future{
		id: atomic.AddUint64(&futureCounter, 1),
	}
}

// Resolve resolves the future with a value
func (f *Future) Resolve(value interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.result = value
	atomic.StoreInt32(&f.done, 1)
	// Wake up awaiters
	for _, co := range f.awaiters {
		co.Value = value
	}
}

// Reject rejects the future with an error
func (f *Future) Reject(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
	atomic.StoreInt32(&f.done, 1)
	for _, co := range f.awaiters {
		co.Error = err
	}
}

// IsResolved returns true if the future has been resolved
func (f *Future) IsResolved() bool {
	return atomic.LoadInt32(&f.done) == 1
}

// Wait blocks until the future is resolved
func (f *Future) Wait(timeout time.Duration) (interface{}, error) {
	if f.IsResolved() {
		return f.result, f.err
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	ticker := time.NewTicker(time.Microsecond)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			return nil, fmt.Errorf("future wait timeout")
		case <-ticker.C:
			if f.IsResolved() {
				return f.result, f.err
			}
		}
	}
}

// Result returns the result if resolved, nil otherwise
func (f *Future) Result() interface{} {
	return f.result
}

// Error returns the error if resolved, nil otherwise
func (f *Future) Error() error {
	return f.err
}
