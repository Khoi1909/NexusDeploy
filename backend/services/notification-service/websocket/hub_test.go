package websocket

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHub_RegisterUnregister(t *testing.T) {
	log := zerolog.New(os.Stdout)
	hub := NewHub(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	// Allow hub to start
	time.Sleep(10 * time.Millisecond)

	// Create mock client
	client := &Client{
		hub:      hub,
		conn:     nil,
		send:     make(chan []byte, 256),
		channels: make(map[string]bool),
	}

	// Register client
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 1, hub.GetActiveConnections())

	// Unregister client
	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 0, hub.GetActiveConnections())
}

func TestHub_SubscribeUnsubscribe(t *testing.T) {
	log := zerolog.New(os.Stdout)
	hub := NewHub(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	client := &Client{
		hub:      hub,
		conn:     nil,
		send:     make(chan []byte, 256),
		channels: make(map[string]bool),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	// Subscribe to channel
	channel := "build_logs:proj-123:build-456"
	hub.Subscribe(client, channel)
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 1, hub.GetChannelSubscribers(channel))

	// Verify client has the subscription
	client.mu.RLock()
	_, hasChannel := client.channels[channel]
	client.mu.RUnlock()
	assert.True(t, hasChannel)

	// Unsubscribe
	hub.Unsubscribe(client, channel)
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 0, hub.GetChannelSubscribers(channel))
}

func TestHub_Broadcast(t *testing.T) {
	log := zerolog.New(os.Stdout)
	hub := NewHub(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	// Create multiple clients
	numClients := 3
	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = &Client{
			hub:      hub,
			conn:     nil,
			send:     make(chan []byte, 256),
			channels: make(map[string]bool),
		}
		hub.Register(clients[i])
	}
	time.Sleep(20 * time.Millisecond)

	channel := "build_logs:proj-123"

	// Subscribe first two clients to channel
	hub.Subscribe(clients[0], channel)
	hub.Subscribe(clients[1], channel)
	time.Sleep(10 * time.Millisecond)

	// Broadcast message
	testMessage := []byte(`{"type":"build_log","message":"Building..."}`)
	hub.Broadcast(channel, testMessage)
	time.Sleep(20 * time.Millisecond)

	// Check clients[0] and clients[1] received the message
	select {
	case msg := <-clients[0].send:
		assert.Equal(t, testMessage, msg)
	case <-time.After(100 * time.Millisecond):
		t.Error("Client 0 did not receive message")
	}

	select {
	case msg := <-clients[1].send:
		assert.Equal(t, testMessage, msg)
	case <-time.After(100 * time.Millisecond):
		t.Error("Client 1 did not receive message")
	}

	// Check client[2] did not receive message (not subscribed)
	select {
	case <-clients[2].send:
		t.Error("Client 2 should not receive message")
	case <-time.After(50 * time.Millisecond):
		// Expected - no message
	}
}

func TestHub_ConcurrentOperations(t *testing.T) {
	log := zerolog.New(os.Stdout)
	hub := NewHub(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup
	numClients := 10
	channel := "test_channel"

	clients := make([]*Client, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = &Client{
			hub:      hub,
			conn:     nil,
			send:     make(chan []byte, 256),
			channels: make(map[string]bool),
		}
	}

	// Concurrent register, subscribe, broadcast
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			hub.Register(clients[idx])
			time.Sleep(5 * time.Millisecond)
			hub.Subscribe(clients[idx], channel)
		}(i)
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	require.Equal(t, numClients, hub.GetActiveConnections())
	require.Equal(t, numClients, hub.GetChannelSubscribers(channel))

	// Broadcast and verify
	testMsg := []byte("concurrent test message")
	hub.Broadcast(channel, testMsg)
	time.Sleep(50 * time.Millisecond)

	receivedCount := 0
	for _, client := range clients {
		select {
		case <-client.send:
			receivedCount++
		case <-time.After(10 * time.Millisecond):
		}
	}

	assert.Equal(t, numClients, receivedCount)
}

func TestHub_ShutdownClosesClients(t *testing.T) {
	log := zerolog.New(os.Stdout)
	hub := NewHub(log)

	ctx, cancel := context.WithCancel(context.Background())

	go hub.Run(ctx)
	time.Sleep(10 * time.Millisecond)

	client := &Client{
		hub:      hub,
		conn:     nil,
		send:     make(chan []byte, 256),
		channels: make(map[string]bool),
	}

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 1, hub.GetActiveConnections())

	// Cancel context to shutdown
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Client's send channel should be closed
	_, ok := <-client.send
	assert.False(t, ok, "Client send channel should be closed")
}

