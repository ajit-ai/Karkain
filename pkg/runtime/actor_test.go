package runtime

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================
// TestActor_LocalMessagePassing
// ============================================================

func TestActor_LocalMessagePassing(t *testing.T) {
	system := NewActorSystem("test_system", SystemConfig{
		MaxMailboxSize:   64,
		HeartbeatInterval: 1 * time.Second,
		NodeTimeout:       5 * time.Second,
	})
	defer system.Shutdown()

	// Spawn an actor that echoes messages
	echoActor := NewSimpleActor("echo", func(msg *Message) error {
		if sm, ok := msg.Payload.(StringMessage); ok {
			// Echo back via reply if available
			if msg.ReplyTo != nil {
				msg.ReplyTo.deliver(&Message{
					Type:    MsgReply,
					Payload: StringMessage{Content: "echo:" + sm.Content},
				})
			}
		}
		return nil
	})

	ref := system.Spawn("echo", echoActor)
	if ref == nil {
		t.Fatal("failed to spawn echo actor")
	}

	// Test async send
	err := ref.Send(StringMessage{Content: "hello"})
	if err != nil {
		t.Fatalf("async send failed: %v", err)
	}

	// Give time for message delivery
	time.Sleep(50 * time.Millisecond)

	// Test sync request-reply
	reply, err := ref.Request(StringMessage{Content: "world"}, 2*time.Second)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if reply == nil {
		t.Fatal("expected reply, got nil")
	}

	sm, ok := reply.Payload.(StringMessage)
	if !ok {
		t.Fatalf("expected StringMessage, got %T", reply.Payload)
	}
	if sm.Content != "echo:world" {
		t.Errorf("expected 'echo:world', got '%s'", sm.Content)
	}
}

func TestActor_MultipleActors(t *testing.T) {
	system := NewActorSystem("multi_test", DefaultSystemConfig())
	defer system.Shutdown()

	received := make([]string, 0)
	var mu sync.Mutex

	handler := func(msg *Message) error {
		if sm, ok := msg.Payload.(StringMessage); ok {
			mu.Lock()
			received = append(received, sm.Content)
			mu.Unlock()
		}
		return nil
	}

	system.Spawn("a1", NewSimpleActor("a1", handler))
	system.Spawn("a2", NewSimpleActor("a2", handler))
	system.Spawn("a3", NewSimpleActor("a3", handler))

	// Send to each
	system.Send("a1", StringMessage{Content: "msg1"})
	system.Send("a2", StringMessage{Content: "msg2"})
	system.Send("a3", StringMessage{Content: "msg3"})

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 3 {
		t.Errorf("expected 3 messages, got %d", len(received))
	}
}

func TestActor_SendByName(t *testing.T) {
	system := NewActorSystem("send_test", DefaultSystemConfig())
	defer system.Shutdown()

	var count int32
	system.Spawn("counter", NewSimpleActor("counter", func(msg *Message) error {
		atomic.AddInt32(&count, 1)
		return nil
	}))

	system.Send("counter", IntMessage{Value: 1})
	system.Send("counter", IntMessage{Value: 2})
	system.Send("counter", IntMessage{Value: 3})

	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&count) != 3 {
		t.Errorf("expected count 3, got %d", atomic.LoadInt32(&count))
	}
}

func TestActor_RequestTimeout(t *testing.T) {
	system := NewActorSystem("timeout_test", DefaultSystemConfig())
	defer system.Shutdown()

	// Actor that never replies
	system.Spawn("silent", NewSimpleActor("silent", func(msg *Message) error {
		return nil
	}))

	_, err := system.Request("silent", StringMessage{Content: "hello"}, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestActor_NotFound(t *testing.T) {
	system := NewActorSystem("notfound_test", DefaultSystemConfig())
	defer system.Shutdown()

	err := system.Send("nonexistent", StringMessage{Content: "hello"})
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestActor_Shutdown(t *testing.T) {
	system := NewActorSystem("shutdown_test", DefaultSystemConfig())

	var stopped int32
	system.Spawn("stoppable", NewSimpleActor("stoppable", func(msg *Message) error {
		return nil
	}))
	system.actors[system.nextID].actor.(*SimpleActor).stopFn = func() error {
		atomic.StoreInt32(&stopped, 1)
		return nil
	}

	ref := system.GetActor("stoppable")
	if ref == nil {
		t.Fatal("actor not found")
	}

	ref.Shutdown()
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&stopped) != 1 {
		t.Error("actor OnStop was not called")
	}

	// Sending to shut-down actor should fail
	err := ref.Send(StringMessage{Content: "after shutdown"})
	if err == nil {
		t.Error("send to shut-down actor should fail")
	}
}

func TestActor_DeadLetters(t *testing.T) {
	system := NewActorSystem("deadletter_test", DefaultSystemConfig())
	defer system.Shutdown()

	// Spawn actor that always fails
	system.Spawn("failer", NewSimpleActor("failer", func(msg *Message) error {
		return fmt.Errorf("processing failed")
	}))

	system.Send("failer", StringMessage{Content: "doomed"})
	time.Sleep(100 * time.Millisecond)

	dead := system.GetDeadLetters()
	if len(dead) == 0 {
		t.Error("expected dead letters, got none")
	}
}

func TestActor_SystemShutdown(t *testing.T) {
	system := NewActorSystem("sys_shutdown_test", DefaultSystemConfig())

	var count int32
	for i := 0; i < 5; i++ {
		system.Spawn("actor_"+string(rune('0'+i)), NewSimpleActor("actor", func(msg *Message) error {
			atomic.AddInt32(&count, 1)
			return nil
		}))
	}

	system.Shutdown()
	time.Sleep(100 * time.Millisecond)

	if system.IsRunning() {
		t.Error("system should not be running after shutdown")
	}
}

func TestActor_ActorCount(t *testing.T) {
	system := NewActorSystem("count_test", DefaultSystemConfig())
	defer system.Shutdown()

	if system.ActorCount() != 0 {
		t.Errorf("expected 0 actors, got %d", system.ActorCount())
	}

	system.Spawn("a", NewSimpleActor("a", func(msg *Message) error { return nil }))
	system.Spawn("b", NewSimpleActor("b", func(msg *Message) error { return nil }))

	if system.ActorCount() != 2 {
		t.Errorf("expected 2 actors, got %d", system.ActorCount())
	}
}

func TestActor_GetActor(t *testing.T) {
	system := NewActorSystem("lookup_test", DefaultSystemConfig())
	defer system.Shutdown()

	system.Spawn("findme", NewSimpleActor("findme", func(msg *Message) error { return nil }))

	ref := system.GetActor("findme")
	if ref == nil {
		t.Fatal("expected to find actor 'findme'")
	}
	if ref.Name != "findme" {
		t.Errorf("expected name 'findme', got '%s'", ref.Name)
	}

	ref2 := system.GetActor("nothere")
	if ref2 != nil {
		t.Error("expected nil for non-existent actor")
	}
}

// ============================================================
// TestActor_RemoteNetworkDispatch
// ============================================================

func TestActor_RemoteNetworkDispatch(t *testing.T) {
	// Create two systems on different ports
	system1 := NewActorSystem("node1", SystemConfig{
		MaxMailboxSize:   64,
		ListenAddr:       "127.0.0.1:17901",
		HeartbeatInterval: 1 * time.Second,
		NodeTimeout:       5 * time.Second,
	})
	system2 := NewActorSystem("node2", SystemConfig{
		MaxMailboxSize:   64,
		ListenAddr:       "127.0.0.1:17902",
		HeartbeatInterval: 1 * time.Second,
		NodeTimeout:       5 * time.Second,
	})

	// Start listening
	err := system1.StartListening()
	if err != nil {
		t.Fatalf("system1 listen failed: %v", err)
	}
	defer system1.Shutdown()

	err = system2.StartListening()
	if err != nil {
		t.Fatalf("system2 listen failed: %v", err)
	}
	defer system2.Shutdown()

	// Connect node1 to node2
	node, err := system1.ConnectToNode("node2", "127.0.0.1:17902")
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	_ = node

	time.Sleep(100 * time.Millisecond)

	// Verify node is connected
	nodes := system1.GetNodes()
	if len(nodes) == 0 {
		t.Error("expected at least 1 connected node")
	}

	// Send network message
	nm := &NetworkMessage{
		Type:   MsgSend,
		NodeID: "node1",
		Target: "test_actor",
		Payload: []byte("hello from node1"),
	}

	err = node.SendNetworkMessage(nm)
	if err != nil {
		t.Fatalf("network send failed: %v", err)
	}

	// Verify node is alive
	if !node.IsAlive() {
		t.Error("remote node should be alive")
	}
}

func TestActor_NetworkBroadcast(t *testing.T) {
	system := NewActorSystem("broadcaster", SystemConfig{
		MaxMailboxSize:   64,
		ListenAddr:       "127.0.0.1:17903",
		HeartbeatInterval: 1 * time.Second,
		NodeTimeout:       5 * time.Second,
	})

	err := system.StartListening()
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer system.Shutdown()

	// Broadcast without peers should not error
	system.Broadcast(&NetworkMessage{
		Type:   MsgHeartbeat,
		NodeID: "broadcaster",
	})
}

func TestActor_NetworkMessageTypes(t *testing.T) {
	tests := []struct {
		mt   MessageType
		want string
	}{
		{MsgSend, "send"},
		{MsgRequest, "request"},
		{MsgReply, "reply"},
		{MsgSpawn, "spawn"},
		{MsgShutdown, "shutdown"},
		{MsgHeartbeat, "heartbeat"},
		{MsgJoin, "join"},
		{MsgLeave, "leave"},
		{MsgDeadLetter, "dead_letter"},
		{MessageType(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.mt.String()
		if got != tt.want {
			t.Errorf("MessageType(%d).String() = %q, want %q", tt.mt, got, tt.want)
		}
	}
}

func TestActor_ActorRefString(t *testing.T) {
	system := NewActorSystem("test", DefaultSystemConfig())
	defer system.Shutdown()

	ref := system.Spawn("my_actor", NewSimpleActor("my_actor", nil))
	if ref == nil {
		t.Fatal("spawn failed")
	}

	s := ref.String()
	if s != "actor://local/my_actor" {
		t.Errorf("expected 'actor://local/my_actor', got '%s'", s)
	}

	remote := &ActorRef{Name: "remote_actor", NodeID: "192.168.1.1:7900"}
	s = remote.String()
	if s != "actor://192.168.1.1:7900/remote_actor" {
		t.Errorf("expected 'actor://192.168.1.1:7900/remote_actor', got '%s'", s)
	}
}

func TestActor_SimpleActorLifecycle(t *testing.T) {
	system := NewActorSystem("lifecycle_test", DefaultSystemConfig())
	defer system.Shutdown()

	var started, stopped int32

	sa := NewSimpleActor("lifecycle", func(msg *Message) error {
		return nil
	})
	sa.startFn = func() error {
		atomic.StoreInt32(&started, 1)
		return nil
	}
	sa.stopFn = func() error {
		atomic.StoreInt32(&stopped, 1)
		return nil
	}

	system.Spawn("lifecycle", sa)
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&started) != 1 {
		t.Error("OnStart was not called")
	}

	system.Shutdown()
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&stopped) != 1 {
		t.Error("OnStop was not called")
	}
}

// ============================================================
// TestActor_NodeFailureRecovery
// ============================================================

func TestActor_NodeFailureRecovery(t *testing.T) {
	system := NewActorSystem("recovery_test", SystemConfig{
		MaxMailboxSize:    64,
		HeartbeatInterval: 50 * time.Millisecond,
		NodeTimeout:       100 * time.Millisecond,
		ListenAddr:        "127.0.0.1:17910",
	})

	err := system.StartListening()
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer system.Shutdown()

	// Start heartbeat so it detects failed nodes
	system.StartHeartbeat()

	// Create a mock remote node that has a stale LastSeen
	fakeNode := &RemoteNode{
		ID:       "fake_node",
		Addr:     "127.0.0.1:17999",
		System:   system,
		Alive:    1,
		LastSeen: time.Now().Add(-1 * time.Second), // stale
	}

	system.nodeMu.Lock()
	system.nodes["fake_node"] = fakeNode
	system.nodeMu.Unlock()

	// Wait for heartbeat to clean up the stale node
	time.Sleep(300 * time.Millisecond)

	// Check that the node was cleaned up
	system.nodeMu.RLock()
	_, exists := system.nodes["fake_node"]
	system.nodeMu.RUnlock()

	if exists {
		t.Error("stale node should have been removed by heartbeat check")
	}

	// Test dead letter routing
	system.routeDeadLetter(&Message{
		Type:    MsgSend,
		Payload: StringMessage{Content: "orphaned"},
	})

	dead := system.GetDeadLetters()
	if len(dead) == 0 {
		t.Error("expected dead letters after node failure")
	}
}

func TestActor_MailboxPersistence(t *testing.T) {
	system := NewActorSystem("mailbox_test", SystemConfig{
		MaxMailboxSize: 128,
	})
	defer system.Shutdown()

	received := make(chan string, 10)
	system.Spawn("receiver", NewSimpleActor("receiver", func(msg *Message) error {
		if sm, ok := msg.Payload.(StringMessage); ok {
			received <- sm.Content
		}
		return nil
	}))

	// Send multiple messages rapidly
	for i := 0; i < 5; i++ {
		system.Send("receiver", StringMessage{Content: "msg"})
	}

	// Read all messages
	count := 0
	timeout := time.After(2 * time.Second)
	for {
		select {
		case <-received:
			count++
			if count == 5 {
				goto done
			}
		case <-timeout:
			goto done
		}
	}
done:
	if count != 5 {
		t.Errorf("expected 5 messages, got %d", count)
	}
}
