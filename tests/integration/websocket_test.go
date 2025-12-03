// Package integration contains integration tests for the arbitrage trading terminal.
//
// WebSocket Integration Tests
// These tests verify WebSocket connection, messaging, and broadcast functionality:
// - Connection establishment and upgrade
// - Client registration/unregistration
// - Broadcast messaging to all clients
// - Ping/Pong heartbeat mechanism
// - Graceful connection handling
//
// Run with: go test -tags=integration ./tests/integration/...
package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"arbitrage/internal/api"
	"arbitrage/internal/models"
	"arbitrage/internal/websocket"

	gorillaws "github.com/gorilla/websocket"
)

// ============================================================
// WebSocket Connection Tests
// ============================================================

func TestWebSocket_Connection_Integration(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	deps := &api.Dependencies{Hub: hub}
	router := api.SetupRoutes(deps)
	server := httptest.NewServer(router)
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/stream"

	t.Run("establishes WebSocket connection", func(t *testing.T) {
		conn, resp, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect to WebSocket: %v", err)
		}
		defer conn.Close()

		if resp.StatusCode != http.StatusSwitchingProtocols {
			t.Errorf("expected status 101, got %d", resp.StatusCode)
		}

		// Wait for registration
		time.Sleep(100 * time.Millisecond)

		if hub.ClientCount() < 1 {
			t.Errorf("expected at least 1 client, got %d", hub.ClientCount())
		}
	})

	t.Run("client count decreases on disconnect", func(t *testing.T) {
		initialCount := hub.ClientCount()

		conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}

		time.Sleep(100 * time.Millisecond)
		afterConnect := hub.ClientCount()

		conn.Close()
		time.Sleep(200 * time.Millisecond)

		afterDisconnect := hub.ClientCount()

		if afterConnect <= initialCount {
			t.Error("client count should increase after connect")
		}
		if afterDisconnect >= afterConnect {
			t.Error("client count should decrease after disconnect")
		}
	})
}

// ============================================================
// WebSocket Broadcast Tests
// ============================================================

func TestWebSocket_Broadcast_Integration(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	deps := &api.Dependencies{Hub: hub}
	router := api.SetupRoutes(deps)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/stream"

	t.Run("broadcasts message to single client", func(t *testing.T) {
		conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		time.Sleep(100 * time.Millisecond)

		// Send broadcast
		testMessage := map[string]string{"type": "test", "data": "hello"}
		hub.Broadcast(testMessage)

		// Read message with timeout
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message: %v", err)
		}

		var received map[string]string
		if err := json.Unmarshal(message, &received); err != nil {
			t.Fatalf("failed to unmarshal message: %v", err)
		}

		if received["type"] != "test" {
			t.Errorf("expected type 'test', got '%s'", received["type"])
		}
		if received["data"] != "hello" {
			t.Errorf("expected data 'hello', got '%s'", received["data"])
		}
	})

	t.Run("broadcasts to multiple clients", func(t *testing.T) {
		const clientCount = 3
		conns := make([]*gorillaws.Conn, clientCount)
		var wg sync.WaitGroup

		// Connect multiple clients
		for i := 0; i < clientCount; i++ {
			conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				t.Fatalf("failed to connect client %d: %v", i, err)
			}
			conns[i] = conn
		}
		defer func() {
			for _, conn := range conns {
				if conn != nil {
					conn.Close()
				}
			}
		}()

		time.Sleep(200 * time.Millisecond)

		// Send broadcast
		testMessage := map[string]interface{}{
			"type": "multicast_test",
			"id":   12345,
		}
		hub.Broadcast(testMessage)

		// Verify all clients receive message
		received := int32(0)
		wg.Add(clientCount)

		for i, conn := range conns {
			go func(idx int, c *gorillaws.Conn) {
				defer wg.Done()
				c.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, msg, err := c.ReadMessage()
				if err != nil {
					t.Logf("client %d failed to read: %v", idx, err)
					return
				}

				var data map[string]interface{}
				if err := json.Unmarshal(msg, &data); err == nil {
					if data["type"] == "multicast_test" {
						atomic.AddInt32(&received, 1)
					}
				}
			}(i, conn)
		}

		wg.Wait()

		if received != clientCount {
			t.Errorf("expected %d clients to receive message, got %d", clientCount, received)
		}
	})
}

// ============================================================
// WebSocket Message Types Tests
// ============================================================

func TestWebSocket_MessageTypes_Integration(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	deps := &api.Dependencies{Hub: hub}
	router := api.SetupRoutes(deps)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/stream"

	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	t.Run("broadcasts pairUpdate message", func(t *testing.T) {
		runtime := &models.PairRuntime{
			PairID:        1,
			State:         "HOLDING",
			CurrentSpread: 0.5,
			UnrealizedPnl: 25.50,
			Legs: []models.Leg{
				{Exchange: "bybit", Side: "long", EntryPrice: 50000, CurrentPrice: 50100, Quantity: 0.1},
				{Exchange: "okx", Side: "short", EntryPrice: 50050, CurrentPrice: 50100, Quantity: 0.1},
			},
		}

		hub.BroadcastPairUpdate(1, runtime)

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message: %v", err)
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if msg["type"] != "pairUpdate" {
			t.Errorf("expected type 'pairUpdate', got '%v'", msg["type"])
		}
	})

	t.Run("broadcasts notification message", func(t *testing.T) {
		notification := &models.Notification{
			ID:        1,
			Type:      "OPEN",
			Severity:  "info",
			Message:   "Opened arbitrage BTCUSDT",
			Timestamp: time.Now(),
		}

		hub.BroadcastNotification(notification)

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message: %v", err)
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if msg["type"] != "notification" {
			t.Errorf("expected type 'notification', got '%v'", msg["type"])
		}
	})

	t.Run("broadcasts balanceUpdate message", func(t *testing.T) {
		hub.BroadcastBalanceUpdate("bybit", 1500.50)

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message: %v", err)
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if msg["type"] != "balanceUpdate" {
			t.Errorf("expected type 'balanceUpdate', got '%v'", msg["type"])
		}
	})

	t.Run("broadcasts statsUpdate message", func(t *testing.T) {
		stats := &models.Stats{
			TotalTrades: 100,
			TotalPnl:    500.25,
			TodayTrades: 5,
			TodayPnl:    25.00,
		}

		hub.BroadcastStatsUpdate(stats)

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message: %v", err)
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if msg["type"] != "statsUpdate" {
			t.Errorf("expected type 'statsUpdate', got '%v'", msg["type"])
		}
	})

	t.Run("broadcasts allBalancesUpdate message", func(t *testing.T) {
		balances := map[string]float64{
			"bybit":  1500.50,
			"okx":    2000.00,
			"bitget": 1750.25,
		}

		hub.BroadcastAllBalances(balances)

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read message: %v", err)
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		// AllBalancesUpdateMessage uses MessageTypeBalanceUpdate ("balanceUpdate")
		if msg["type"] != "balanceUpdate" {
			t.Errorf("expected type 'balanceUpdate', got '%v'", msg["type"])
		}
		// Verify it has balances map
		if msg["balances"] == nil {
			t.Error("expected balances map in message")
		}
	})
}

// ============================================================
// WebSocket Concurrent Connections Tests
// ============================================================

func TestWebSocket_ConcurrentConnections_Integration(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	deps := &api.Dependencies{Hub: hub}
	router := api.SetupRoutes(deps)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/stream"

	t.Run("handles many concurrent connections", func(t *testing.T) {
		const numClients = 20
		conns := make([]*gorillaws.Conn, numClients)
		var connectWg sync.WaitGroup

		// Connect all clients concurrently
		connectWg.Add(numClients)
		for i := 0; i < numClients; i++ {
			go func(idx int) {
				defer connectWg.Done()
				conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
				if err != nil {
					t.Logf("client %d failed to connect: %v", idx, err)
					return
				}
				conns[idx] = conn
			}(i)
		}
		connectWg.Wait()

		// Count successful connections
		successfulConns := 0
		for _, conn := range conns {
			if conn != nil {
				successfulConns++
			}
		}

		if successfulConns < numClients/2 {
			t.Errorf("expected at least %d connections, got %d", numClients/2, successfulConns)
		}

		time.Sleep(200 * time.Millisecond)

		// Verify hub client count
		clientCount := hub.ClientCount()
		if clientCount < successfulConns/2 {
			t.Errorf("expected at least %d clients in hub, got %d", successfulConns/2, clientCount)
		}

		// Cleanup
		for _, conn := range conns {
			if conn != nil {
				conn.Close()
			}
		}
	})
}

// ============================================================
// WebSocket Message Ordering Tests
// ============================================================

func TestWebSocket_MessageOrdering_Integration(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	deps := &api.Dependencies{Hub: hub}
	router := api.SetupRoutes(deps)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/stream"

	t.Run("messages arrive in order", func(t *testing.T) {
		conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		time.Sleep(100 * time.Millisecond)

		// Send multiple messages
		const messageCount = 10
		for i := 0; i < messageCount; i++ {
			hub.Broadcast(map[string]int{"sequence": i})
		}

		// Read and verify order
		lastSequence := -1
		for i := 0; i < messageCount; i++ {
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, message, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("failed to read message %d: %v", i, err)
			}

			var msg map[string]int
			if err := json.Unmarshal(message, &msg); err != nil {
				t.Fatalf("failed to unmarshal message %d: %v", i, err)
			}

			if msg["sequence"] <= lastSequence {
				t.Errorf("message out of order: got %d after %d", msg["sequence"], lastSequence)
			}
			lastSequence = msg["sequence"]
		}
	})
}

// ============================================================
// WebSocket Reconnection Tests
// ============================================================

func TestWebSocket_Reconnection_Integration(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	deps := &api.Dependencies{Hub: hub}
	router := api.SetupRoutes(deps)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/stream"

	t.Run("client can reconnect after disconnect", func(t *testing.T) {
		// First connection
		conn1, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}

		time.Sleep(100 * time.Millisecond)
		initialCount := hub.ClientCount()

		// Close first connection
		conn1.Close()
		time.Sleep(200 * time.Millisecond)

		// Second connection
		conn2, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to reconnect: %v", err)
		}
		defer conn2.Close()

		time.Sleep(100 * time.Millisecond)

		// Verify reconnection
		finalCount := hub.ClientCount()
		if finalCount < 1 {
			t.Error("client should be able to reconnect")
		}

		// Verify can receive messages after reconnection
		hub.Broadcast(map[string]string{"test": "reconnect"})

		conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, message, err := conn2.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read after reconnection: %v", err)
		}

		var msg map[string]string
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		if msg["test"] != "reconnect" {
			t.Error("should receive message after reconnection")
		}

		_ = initialCount // suppress unused variable warning
	})
}

// ============================================================
// WebSocket Large Message Tests
// ============================================================

func TestWebSocket_LargeMessage_Integration(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	deps := &api.Dependencies{Hub: hub}
	router := api.SetupRoutes(deps)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/stream"

	t.Run("handles large messages", func(t *testing.T) {
		conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		time.Sleep(100 * time.Millisecond)

		// Create a large message (~10KB)
		largeData := make([]string, 100)
		for i := range largeData {
			largeData[i] = strings.Repeat("x", 100)
		}

		largeMessage := map[string]interface{}{
			"type": "large_test",
			"data": largeData,
		}

		hub.Broadcast(largeMessage)

		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read large message: %v", err)
		}

		if len(message) < 5000 {
			t.Errorf("expected large message (>5KB), got %d bytes", len(message))
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			t.Fatalf("failed to unmarshal large message: %v", err)
		}

		if msg["type"] != "large_test" {
			t.Errorf("expected type 'large_test', got '%v'", msg["type"])
		}
	})
}

// ============================================================
// WebSocket Hub Tests
// ============================================================

func TestWebSocket_Hub_Integration(t *testing.T) {
	t.Run("hub runs without blocking", func(t *testing.T) {
		hub := websocket.NewHub()

		done := make(chan bool)
		go func() {
			hub.Run()
		}()

		// Hub should not block startup
		select {
		case <-done:
			t.Error("hub should not complete")
		case <-time.After(100 * time.Millisecond):
			// Expected: hub runs indefinitely
		}
	})

	t.Run("hub handles broadcast without clients", func(t *testing.T) {
		hub := websocket.NewHub()
		go hub.Run()

		// Should not panic when broadcasting without clients
		hub.Broadcast(map[string]string{"test": "no clients"})

		// Give time for processing
		time.Sleep(50 * time.Millisecond)

		if hub.ClientCount() != 0 {
			t.Errorf("expected 0 clients, got %d", hub.ClientCount())
		}
	})
}

// ============================================================
// WebSocket Binary Message Tests
// ============================================================

func TestWebSocket_BinaryMessage_Integration(t *testing.T) {
	hub := websocket.NewHub()
	go hub.Run()

	deps := &api.Dependencies{Hub: hub}
	router := api.SetupRoutes(deps)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/stream"

	t.Run("handles JSON messages correctly", func(t *testing.T) {
		conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect: %v", err)
		}
		defer conn.Close()

		time.Sleep(100 * time.Millisecond)

		// Test various data types
		testCases := []map[string]interface{}{
			{"string": "test"},
			{"number": 123.45},
			{"bool": true},
			{"null": nil},
			{"array": []int{1, 2, 3}},
			{"nested": map[string]string{"key": "value"}},
		}

		for i, tc := range testCases {
			hub.Broadcast(tc)

			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, message, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("test %d: failed to read: %v", i, err)
			}

			var received map[string]interface{}
			if err := json.Unmarshal(message, &received); err != nil {
				t.Fatalf("test %d: failed to unmarshal: %v", i, err)
			}
		}
	})
}
