package websocket

import (
	"sync"
	"testing"
	"time"

	"arbitrage/internal/models"
)

// ============================================================
// Unit Tests
// ============================================================

func TestNewHub(t *testing.T) {
	hub := NewHub()

	if hub == nil {
		t.Fatal("NewHub returned nil")
	}

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", hub.ClientCount())
	}

	if hub.DroppedMessages() != 0 {
		t.Errorf("expected 0 dropped messages, got %d", hub.DroppedMessages())
	}
}

func TestOriginChecker_Check(t *testing.T) {
	checker := &OriginChecker{
		allowedOrigins: map[string]struct{}{
			"http://localhost:3000": {},
			"https://example.com":   {},
		},
		allowAll: false,
	}

	tests := []struct {
		origin string
		want   bool
	}{
		{"", true},                       // empty origin allowed
		{"http://localhost:3000", true},  // allowed
		{"https://example.com", true},    // allowed
		{"http://evil.com", false},       // not allowed
		{"http://localhost:8080", false}, // not in list
	}

	for _, tt := range tests {
		got := checker.Check(tt.origin)
		if got != tt.want {
			t.Errorf("Check(%q) = %v, want %v", tt.origin, got, tt.want)
		}
	}
}

func TestOriginChecker_AllowAll(t *testing.T) {
	checker := &OriginChecker{
		allowAll: true,
	}

	origins := []string{
		"http://localhost:3000",
		"https://evil.com",
		"http://anything.example.org",
	}

	for _, origin := range origins {
		if !checker.Check(origin) {
			t.Errorf("allowAll=true but Check(%q) = false", origin)
		}
	}
}

func TestHub_BroadcastNonBlocking(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	// Fill the broadcast channel
	for i := 0; i < 10000; i++ {
		hub.Broadcast(map[string]int{"i": i})
	}

	// Should not block, messages should be dropped
	time.Sleep(10 * time.Millisecond)

	// Some messages should be dropped
	if hub.DroppedMessages() == 0 {
		t.Log("No messages dropped (channel was not full)")
	}
}

func TestHub_Stop(t *testing.T) {
	hub := NewHub()

	done := make(chan struct{})
	go func() {
		hub.Run()
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	hub.Stop()

	select {
	case <-done:
		// OK - Run() exited
	case <-time.After(1 * time.Second):
		t.Error("Hub.Run() did not exit after Stop()")
	}
}

// ============================================================
// Benchmarks
// ============================================================

// BenchmarkHub_Broadcast тестирует скорость broadcast
func BenchmarkHub_Broadcast(b *testing.B) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	msg := map[string]interface{}{
		"type": "test",
		"data": "benchmark message",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.Broadcast(msg)
	}
}

// BenchmarkHub_BroadcastRaw тестирует скорость broadcast уже сериализованных данных
func BenchmarkHub_BroadcastRaw(b *testing.B) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	data := []byte(`{"type":"test","data":"benchmark message"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.BroadcastRaw(data)
	}
}

// BenchmarkHub_BroadcastPairUpdate тестирует реальный use case
func BenchmarkHub_BroadcastPairUpdate(b *testing.B) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	runtime := &models.PairRuntime{
		PairID:        1,
		State:         "HOLDING",
		CurrentSpread: 0.5,
		UnrealizedPnl: 25.50,
		RealizedPnl:   100.00,
		FilledParts:   2,
		Legs: []models.Leg{
			{Exchange: "bybit", Side: "long", EntryPrice: 50000, CurrentPrice: 50100, Quantity: 0.1},
			{Exchange: "okx", Side: "short", EntryPrice: 50050, CurrentPrice: 50100, Quantity: 0.1},
		},
		LastUpdate: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.BroadcastPairUpdate(1, runtime)
	}
}

// BenchmarkOriginChecker_Check тестирует скорость проверки origin
func BenchmarkOriginChecker_Check(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		originChecker.Check("http://localhost:3000")
	}
}

// BenchmarkHub_ClientCount тестирует lock-free чтение
func BenchmarkHub_ClientCount(b *testing.B) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hub.ClientCount()
	}
}

// BenchmarkHub_ConcurrentBroadcast тестирует конкурентный broadcast
func BenchmarkHub_ConcurrentBroadcast(b *testing.B) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	msg := map[string]string{"type": "test"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hub.Broadcast(msg)
		}
	})
}

// BenchmarkNewPairUpdateMessage тестирует создание сообщения
func BenchmarkNewPairUpdateMessage(b *testing.B) {
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewPairUpdateMessage(1, runtime)
	}
}

// BenchmarkClientPool тестирует sync.Pool для клиентов
func BenchmarkClientPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client := clientPool.Get().(*Client)
		clientPool.Put(client)
	}
}

// BenchmarkByteSlicePool тестирует sync.Pool для byte slices
func BenchmarkByteSlicePool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := byteSlicePool.Get().(*[]byte)
		*buf = (*buf)[:0]
		byteSlicePool.Put(buf)
	}
}

// BenchmarkHub_ManyClients симулирует много клиентов
func BenchmarkHub_ManyClients(b *testing.B) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	// Симулируем 100 клиентов
	var clients []*Client
	for i := 0; i < 100; i++ {
		client := &Client{
			hub:  hub,
			send: make(chan []byte, clientSendBufferSize),
		}
		hub.register <- client
		clients = append(clients, client)

		// Запускаем горутину которая читает сообщения
		go func(c *Client) {
			for range c.send {
				// discard
			}
		}(client)
	}

	time.Sleep(50 * time.Millisecond)

	msg := map[string]string{"type": "test", "data": "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.Broadcast(msg)
	}
	b.StopTimer()

	// Cleanup
	for _, c := range clients {
		hub.unregister <- c
	}
}

// ============================================================
// Parallel Stress Test
// ============================================================

func TestHub_ConcurrentOperations(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	var wg sync.WaitGroup
	const goroutines = 10
	const operations = 1000

	// Concurrent broadcasts
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				hub.Broadcast(map[string]int{"goroutine": id, "op": j})
			}
		}(i)
	}

	// Concurrent ClientCount reads
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				_ = hub.ClientCount()
			}
		}()
	}

	wg.Wait()
}
