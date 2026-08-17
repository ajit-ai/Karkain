package runtime

import (
	"encoding/gob"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================
// Phase 37: Distributed Actor System & Micro-Runtime Network Mesh
// Thread-safe MPSC mailboxes, location-transparent addressing,
// TCP network dispatch, node discovery & heartbeat mesh
// ============================================================

// ============================================================
// Message
// ============================================================

// Message is the unit of communication between actors
type Message struct {
	Type     MessageType
	Sender   *ActorRef
	Target   *ActorRef
	Payload  interface{}
	ID       uint64
	ReplyTo *ActorRef // for request-reply
}

type MessageType uint8

const (
	MsgSend MessageType = iota // async fire-and-forget
	MsgRequest                 // sync request-reply
	MsgReply                   // reply to a request
	MsgSpawn                   // spawn a new actor
	MsgShutdown                // graceful shutdown
	MsgHeartbeat               // node heartbeat
	MsgJoin                    // join cluster
	MsgLeave                   // leave cluster
	MsgDeadLetter              // undeliverable message
)

func (mt MessageType) String() string {
	switch mt {
	case MsgSend:
		return "send"
	case MsgRequest:
		return "request"
	case MsgReply:
		return "reply"
	case MsgSpawn:
		return "spawn"
	case MsgShutdown:
		return "shutdown"
	case MsgHeartbeat:
		return "heartbeat"
	case MsgJoin:
		return "join"
	case MsgLeave:
		return "leave"
	case MsgDeadLetter:
		return "dead_letter"
	default:
		return "unknown"
	}
}

// ============================================================
// ActorRef — Location-Transparent Address
// ============================================================

// ActorRef is a location-transparent actor address
type ActorRef struct {
	ID       uint64
	Name     string
	NodeID   string // local = "", remote = node address
	mailbox  chan *Message
	actor    Actor
	system   *ActorSystem
	closed   int32
}

// Send sends an async message (fire-and-forget)
func (ref *ActorRef) Send(msg interface{}) error {
	if atomic.LoadInt32(&ref.closed) == 1 {
		return fmt.Errorf("actor '%s' is shut down", ref.Name)
	}
	m := &Message{
		Type:    MsgSend,
		Sender:  nil,
		Target:  ref,
		Payload: msg,
		ID:      nextMsgID(),
	}
	return ref.deliver(m)
}

// Request sends a sync request and waits for a reply
func (ref *ActorRef) Request(msg interface{}, timeout time.Duration) (*Message, error) {
	if atomic.LoadInt32(&ref.closed) == 1 {
		return nil, fmt.Errorf("actor '%s' is shut down", ref.Name)
	}

	replyCh := make(chan *Message, 1)
	senderRef := &ActorRef{
		ID:      nextMsgID(),
		Name:    "__requester",
		mailbox: replyCh,
		system:  ref.system,
	}

	m := &Message{
		Type:    MsgRequest,
		Sender:  senderRef,
		Target:  ref,
		Payload: msg,
		ID:      nextMsgID(),
		ReplyTo: senderRef,
	}

	if err := ref.deliver(m); err != nil {
		return nil, err
	}

	select {
	case reply := <-replyCh:
		return reply, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("request to '%s' timed out after %v", ref.Name, timeout)
	}
}

// Shutdown gracefully shuts down the actor
func (ref *ActorRef) Shutdown() error {
	if !atomic.CompareAndSwapInt32(&ref.closed, 0, 1) {
		return nil
	}
	m := &Message{
		Type:   MsgShutdown,
		Target: ref,
	}
	return ref.deliver(m)
}

func (ref *ActorRef) deliver(m *Message) error {
	select {
	case ref.mailbox <- m:
		return nil
	default:
		return fmt.Errorf("actor '%s' mailbox full", ref.Name)
	}
}

func (ref *ActorRef) String() string {
	if ref.NodeID != "" {
		return fmt.Sprintf("actor://%s/%s", ref.NodeID, ref.Name)
	}
	return fmt.Sprintf("actor://local/%s", ref.Name)
}

// ============================================================
// Actor Interface
// ============================================================

// Actor is the interface that all actors must implement
type Actor interface {
	HandleMessage(msg *Message) error
	OnStart() error
	OnStop() error
}

// ============================================================
// ActorSystem — The Runtime
// ============================================================

type msgIDCounter uint64

var globalMsgID uint64

func nextMsgID() uint64 {
	return atomic.AddUint64(&globalMsgID, 1)
}

// ActorSystem manages all actors and the network mesh
type ActorSystem struct {
	Name     string
	actors   map[uint64]*ActorRef
	actorMu  sync.RWMutex
	nextID   uint64
	nodes    map[string]*RemoteNode
	nodeMu   sync.RWMutex
	config   SystemConfig
	dead     []*Message
	deadMu   sync.Mutex
	running  int32
	heartbeatTicker *time.Ticker
}

type SystemConfig struct {
	MaxMailboxSize   int
	HeartbeatInterval time.Duration
	NodeTimeout       time.Duration
	ListenAddr        string
	PeerAddrs         []string
}

func DefaultSystemConfig() SystemConfig {
	return SystemConfig{
		MaxMailboxSize:   1024,
		HeartbeatInterval: 5 * time.Second,
		NodeTimeout:       15 * time.Second,
		ListenAddr:        "127.0.0.1:7900",
	}
}

// NewActorSystem creates a new distributed actor system
func NewActorSystem(name string, cfg SystemConfig) *ActorSystem {
	as := &ActorSystem{
		Name:    name,
		actors:  make(map[uint64]*ActorRef),
		nodes:   make(map[string]*RemoteNode),
		config:  cfg,
		dead:    make([]*Message, 0),
		running: 1,
	}
	return as
}

// Spawn creates a new local actor
func (as *ActorSystem) Spawn(name string, actor Actor) *ActorRef {
	as.actorMu.Lock()
	defer as.actorMu.Unlock()

	as.nextID++
	id := as.nextID

	mailbox := make(chan *Message, as.config.MaxMailboxSize)
	ref := &ActorRef{
		ID:      id,
		Name:    name,
		mailbox: mailbox,
		actor:   actor,
		system:  as,
	}

	as.actors[id] = ref

	// Start actor goroutine
	go as.runActor(ref)

	return ref
}

func (as *ActorSystem) runActor(ref *ActorRef) {
	if ref.actor != nil {
		if err := ref.actor.OnStart(); err != nil {
			return
		}
	}

	for {
		select {
		case msg, ok := <-ref.mailbox:
			if !ok {
				break
			}
			if msg.Type == MsgShutdown {
				if ref.actor != nil {
					ref.actor.OnStop()
				}
				return
			}
			if ref.actor != nil {
				err := ref.actor.HandleMessage(msg)
				if err != nil {
					// Route to dead letter
					as.routeDeadLetter(msg)
				}
			}
		default:
			if atomic.LoadInt32(&ref.closed) == 1 {
				if ref.actor != nil {
					ref.actor.OnStop()
				}
				return
			}
			time.Sleep(time.Millisecond)
		}
	}
}

// ============================================================
// Actor Lookup & Routing
// ============================================================

// GetActor returns an actor ref by name
func (as *ActorSystem) GetActor(name string) *ActorRef {
	as.actorMu.RLock()
	defer as.actorMu.RUnlock()
	for _, ref := range as.actors {
		if ref.Name == name {
			return ref
		}
	}
	return nil
}

// GetActorByID returns an actor ref by ID
func (as *ActorSystem) GetActorByID(id uint64) *ActorRef {
	as.actorMu.RLock()
	defer as.actorMu.RUnlock()
	return as.actors[id]
}

// Send sends an async message to an actor by name
func (as *ActorSystem) Send(targetName string, msg interface{}) error {
	ref := as.GetActor(targetName)
	if ref == nil {
		return fmt.Errorf("actor '%s' not found", targetName)
	}
	return ref.Send(msg)
}

// Request sends a sync request to an actor by name
func (as *ActorSystem) Request(targetName string, msg interface{}, timeout time.Duration) (*Message, error) {
	ref := as.GetActor(targetName)
	if ref == nil {
		return nil, fmt.Errorf("actor '%s' not found", targetName)
	}
	return ref.Request(msg, timeout)
}

// ============================================================
// Dead Letters
// ============================================================

func (as *ActorSystem) routeDeadLetter(msg *Message) {
	as.deadMu.Lock()
	defer as.deadMu.Unlock()
	msg.Type = MsgDeadLetter
	as.dead = append(as.dead, msg)
}

// GetDeadLetters returns all dead-letter messages
func (as *ActorSystem) GetDeadLetters() []*Message {
	as.deadMu.Lock()
	defer as.deadMu.Unlock()
	result := make([]*Message, len(as.dead))
	copy(result, as.dead)
	return result
}

// ============================================================
// Shutdown
// ============================================================

// Shutdown stops all actors and the system
func (as *ActorSystem) Shutdown() {
	if !atomic.CompareAndSwapInt32(&as.running, 1, 0) {
		return
	}

	if as.heartbeatTicker != nil {
		as.heartbeatTicker.Stop()
	}

	as.actorMu.RLock()
	for _, ref := range as.actors {
		ref.Shutdown()
	}
	as.actorMu.RUnlock()

	// Close all network connections
	as.nodeMu.RLock()
	for _, node := range as.nodes {
		node.Close()
	}
	as.nodeMu.RUnlock()
}

// IsRunning returns whether the system is active
func (as *ActorSystem) IsRunning() bool {
	return atomic.LoadInt32(&as.running) == 1
}

// ActorCount returns the number of registered actors
func (as *ActorSystem) ActorCount() int {
	as.actorMu.RLock()
	defer as.actorMu.RUnlock()
	return len(as.actors)
}

// ============================================================
// Remote Node — TCP Network Mesh
// ============================================================

// RemoteNode represents a connected peer node
type RemoteNode struct {
	ID       string
	Addr     string
	Conn     net.Conn
	Encoder  *gob.Encoder
	Decoder  *gob.Decoder
	System   *ActorSystem
	Alive    int32
	LastSeen time.Time
	mu       sync.Mutex
	closed   int32
}

// ConnectToNode establishes a TCP connection to a remote node
func (as *ActorSystem) ConnectToNode(nodeID, addr string) (*RemoteNode, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s at %s: %w", nodeID, addr, err)
	}

	node := &RemoteNode{
		ID:       nodeID,
		Addr:     addr,
		Conn:     conn,
		Encoder:  gob.NewEncoder(conn),
		Decoder:  gob.NewDecoder(conn),
		System:   as,
		Alive:    1,
		LastSeen: time.Now(),
	}

	as.nodeMu.Lock()
	as.nodes[nodeID] = node
	as.nodeMu.Unlock()

	// Send join message
	joinMsg := &NetworkMessage{
		Type:   MsgJoin,
		NodeID: as.Name,
	}
	_ = node.Encoder.Encode(joinMsg)

	// Start receiver goroutine
	go node.receiveLoop()

	return node, nil
}

// StartListening starts a TCP server for incoming connections
func (as *ActorSystem) StartListening() error {
	ln, err := net.Listen("tcp", as.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", as.config.ListenAddr, err)
	}

	go func() {
		for atomic.LoadInt32(&as.running) == 1 {
			conn, err := ln.Accept()
			if err != nil {
				continue
			}
			go as.handleIncoming(conn)
		}
		ln.Close()
	}()

	return nil
}

func (as *ActorSystem) handleIncoming(conn net.Conn) {
	node := &RemoteNode{
		ID:       conn.RemoteAddr().String(),
		Addr:     conn.RemoteAddr().String(),
		Conn:     conn,
		Encoder:  gob.NewEncoder(conn),
		Decoder:  gob.NewDecoder(conn),
		System:   as,
		Alive:    1,
		LastSeen: time.Now(),
	}

	as.nodeMu.Lock()
	as.nodes[node.ID] = node
	as.nodeMu.Unlock()

	node.receiveLoop()
}

// Broadcast sends a message to all connected nodes
func (as *ActorSystem) Broadcast(msg *NetworkMessage) {
	as.nodeMu.RLock()
	defer as.nodeMu.RUnlock()
	for _, node := range as.nodes {
		node.SendNetworkMessage(msg)
	}
}

// GetNodes returns all connected nodes
func (as *ActorSystem) GetNodes() map[string]*RemoteNode {
	as.nodeMu.RLock()
	defer as.nodeMu.RUnlock()
	result := make(map[string]*RemoteNode, len(as.nodes))
	for k, v := range as.nodes {
		result[k] = v
	}
	return result
}

// ============================================================
// Heartbeat & Discovery
// ============================================================

// StartHeartbeat begins periodic heartbeat broadcasting
func (as *ActorSystem) StartHeartbeat() {
	as.heartbeatTicker = time.NewTicker(as.config.HeartbeatInterval)
	go func() {
		for range as.heartbeatTicker.C {
			if !as.IsRunning() {
				return
			}
			as.sendHeartbeats()
			as.checkNodeHealth()
		}
	}()
}

func (as *ActorSystem) sendHeartbeats() {
	msg := &NetworkMessage{
		Type:     MsgHeartbeat,
		NodeID:   as.Name,
		Timestamp: time.Now().UnixNano(),
	}
	as.Broadcast(msg)
}

func (as *ActorSystem) checkNodeHealth() {
	as.nodeMu.Lock()
	defer as.nodeMu.Unlock()
	now := time.Now()
	for id, node := range as.nodes {
		if atomic.LoadInt32(&node.Alive) == 1 &&
			now.Sub(node.LastSeen) > as.config.NodeTimeout {
			atomic.StoreInt32(&node.Alive, 0)
			node.Close()
			delete(as.nodes, id)
		}
	}
}

// ============================================================
// Network Message (for serialization)
// ============================================================

// NetworkMessage is the wire format for network communication
type NetworkMessage struct {
	Type      MessageType
	NodeID    string
	Target    string // target actor name
	Payload   []byte
	Timestamp int64
}

func init() {
	gob.Register(&NetworkMessage{})
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
}

// ============================================================
// RemoteNode Methods
// ============================================================

func (rn *RemoteNode) receiveLoop() {
	for atomic.LoadInt32(&rn.closed) == 0 {
		var nm NetworkMessage
		err := rn.Decoder.Decode(&nm)
		if err != nil {
			atomic.StoreInt32(&rn.Alive, 0)
			return
		}
		rn.LastSeen = time.Now()
		rn.System.handleNetworkMessage(rn, &nm)
	}
}

func (rn *RemoteNode) SendNetworkMessage(nm *NetworkMessage) error {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	if atomic.LoadInt32(&rn.closed) == 1 {
		return fmt.Errorf("node %s is disconnected", rn.ID)
	}
	if rn.Encoder == nil {
		return fmt.Errorf("node %s has no encoder (not connected)", rn.ID)
	}
	return rn.Encoder.Encode(nm)
}

func (rn *RemoteNode) Close() {
	if !atomic.CompareAndSwapInt32(&rn.closed, 0, 1) {
		return
	}
	if rn.Conn != nil {
		rn.Conn.Close()
	}
}

func (rn *RemoteNode) IsAlive() bool {
	return atomic.LoadInt32(&rn.Alive) == 1
}

func (as *ActorSystem) handleNetworkMessage(from *RemoteNode, nm *NetworkMessage) {
	switch nm.Type {
	case MsgHeartbeat:
		from.LastSeen = time.Now()
		atomic.StoreInt32(&from.Alive, 1)

	case MsgJoin:
		// New node joined — acknowledge
		ack := &NetworkMessage{
			Type:   MsgHeartbeat,
			NodeID: as.Name,
		}
		from.SendNetworkMessage(ack)

	case MsgSend, MsgRequest:
		// Route to local actor
		ref := as.GetActor(nm.Target)
		if ref == nil {
			as.routeDeadLetter(&Message{
				Type:   MsgDeadLetter,
				Payload: nm,
			})
			return
		}
		msg := &Message{
			Type:    nm.Type,
			Sender:  &ActorRef{Name: from.ID, NodeID: from.ID},
			Target:  ref,
			Payload: nm.Payload,
			ID:      nextMsgID(),
		}
		ref.deliver(msg)

	case MsgLeave:
		from.Close()
		as.nodeMu.Lock()
		delete(as.nodes, from.ID)
		as.nodeMu.Unlock()
	}
}

// ============================================================
// Convenience: SimpleActor — callback-based actor
// ============================================================

// SimpleActor is an actor with configurable message handler
type SimpleActor struct {
	Name    string
	handler func(msg *Message) error
	startFn func() error
	stopFn  func() error
}

func NewSimpleActor(name string, handler func(msg *Message) error) *SimpleActor {
	return &SimpleActor{
		Name:    name,
		handler: handler,
		startFn: func() error { return nil },
		stopFn:  func() error { return nil },
	}
}

func (sa *SimpleActor) HandleMessage(msg *Message) error {
	if sa.handler != nil {
		return sa.handler(msg)
	}
	return nil
}

func (sa *SimpleActor) OnStart() error {
	return sa.startFn()
}

func (sa *SimpleActor) OnStop() error {
	return sa.stopFn()
}

// ============================================================
// Typed Message Helpers
// ============================================================

// TypedMessage wraps a payload with type info
type TypedMessage struct {
	TypeName string
	Data     interface{}
}

// StringMessage is a simple string-typed message
type StringMessage struct {
	Content string
}

// IntMessage is a simple int-typed message
type IntMessage struct {
	Value int64
}
