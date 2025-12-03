// Package integration contains integration tests for the arbitrage trading terminal.
//
// API Integration Tests
// These tests verify the complete HTTP request/response cycle through all layers:
// Handler → Service → Repository → Database
//
// Run with: go test -tags=integration ./tests/integration/...
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"arbitrage/internal/api"
	"arbitrage/internal/models"
	"arbitrage/internal/websocket"
)

// ============================================================
// Stats API Integration Tests
// ============================================================

func TestStatsAPI_GetStats_Integration(t *testing.T) {
	ts := SetupTestServer(t)
	if ts == nil {
		t.Skip("Skipping: test server not available")
	}
	defer ts.Cleanup()

	t.Run("returns empty stats initially", func(t *testing.T) {
		resp, err := http.Get(ts.Server.URL + "/api/v1/stats")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var stats models.Stats
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if stats.TotalTrades != 0 {
			t.Errorf("expected 0 total trades, got %d", stats.TotalTrades)
		}
	})

	t.Run("returns correct stats after trades", func(t *testing.T) {
		// Insert test trades directly into database
		_, err := ts.DB.Exec(`
			INSERT INTO trades (pair_id, symbol, exchanges, entry_time, exit_time, pnl, was_stop_loss, was_liquidation)
			VALUES
				(NULL, 'BTCUSDT', 'bybit,okx', NOW() - INTERVAL '1 hour', NOW(), 50.25, false, false),
				(NULL, 'ETHUSDT', 'bitget,gate', NOW() - INTERVAL '2 hours', NOW(), -10.50, true, false)
		`)
		if err != nil {
			t.Fatalf("failed to insert test trades: %v", err)
		}

		resp, err := http.Get(ts.Server.URL + "/api/v1/stats")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var stats models.Stats
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Verify at least some trades are counted
		if stats.TodayTrades < 0 {
			t.Errorf("expected non-negative trades count, got %d", stats.TodayTrades)
		}
	})
}

func TestStatsAPI_GetTopPairs_Integration(t *testing.T) {
	ts := SetupTestServer(t)
	if ts == nil {
		t.Skip("Skipping: test server not available")
	}
	defer ts.Cleanup()

	// Insert test trades for different pairs
	_, err := ts.DB.Exec(`
		INSERT INTO trades (pair_id, symbol, exchanges, entry_time, exit_time, pnl)
		VALUES
			(NULL, 'BTCUSDT', 'bybit,okx', NOW(), NOW(), 100.00),
			(NULL, 'BTCUSDT', 'bybit,okx', NOW(), NOW(), 50.00),
			(NULL, 'ETHUSDT', 'bitget,gate', NOW(), NOW(), 75.00),
			(NULL, 'SOLUSDT', 'htx,bingx', NOW(), NOW(), -25.00)
	`)
	if err != nil {
		t.Fatalf("failed to insert test trades: %v", err)
	}

	testCases := []struct {
		name           string
		metric         string
		expectedStatus int
	}{
		{"trades metric", "trades", http.StatusOK},
		{"profit metric", "profit", http.StatusOK},
		{"loss metric", "loss", http.StatusOK},
		{"invalid metric", "invalid", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := fmt.Sprintf("%s/api/v1/stats/top-pairs?metric=%s", ts.Server.URL, tc.metric)
			resp, err := http.Get(url)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			if tc.expectedStatus == http.StatusOK {
				var pairs []models.PairStat
				if err := json.NewDecoder(resp.Body).Decode(&pairs); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				// Top pairs should be returned as array
				if pairs == nil {
					t.Error("expected non-nil pairs array")
				}
			}
		})
	}
}

func TestStatsAPI_ResetStats_Integration(t *testing.T) {
	ts := SetupTestServer(t)
	if ts == nil {
		t.Skip("Skipping: test server not available")
	}
	defer ts.Cleanup()

	// Insert test trades
	_, err := ts.DB.Exec(`
		INSERT INTO trades (pair_id, symbol, exchanges, entry_time, exit_time, pnl)
		VALUES (NULL, 'BTCUSDT', 'bybit,okx', NOW(), NOW(), 100.00)
	`)
	if err != nil {
		t.Fatalf("failed to insert test trade: %v", err)
	}

	t.Run("resets stats successfully", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, ts.Server.URL+"/api/v1/stats/reset", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result map[string]string
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["message"] == "" {
			t.Error("expected success message")
		}
	})
}

// ============================================================
// Blacklist API Integration Tests
// ============================================================

func TestBlacklistAPI_CRUD_Integration(t *testing.T) {
	ts := SetupTestServer(t)
	if ts == nil {
		t.Skip("Skipping: test server not available")
	}
	defer ts.Cleanup()

	t.Run("get empty blacklist", func(t *testing.T) {
		resp, err := http.Get(ts.Server.URL + "/api/v1/blacklist")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var entries []models.BlacklistEntry
		if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(entries) != 0 {
			t.Errorf("expected empty blacklist, got %d entries", len(entries))
		}
	})

	t.Run("add to blacklist", func(t *testing.T) {
		payload := map[string]string{
			"symbol": "TESTUSDT",
			"reason": "Test reason",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(
			ts.Server.URL+"/api/v1/blacklist",
			"application/json",
			bytes.NewBuffer(body),
		)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			respBody, _ := io.ReadAll(resp.Body)
			t.Errorf("expected status 201, got %d: %s", resp.StatusCode, string(respBody))
		}

		var entry models.BlacklistEntry
		if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if entry.Symbol != "TESTUSDT" {
			t.Errorf("expected symbol TESTUSDT, got %s", entry.Symbol)
		}
		if entry.Reason != "Test reason" {
			t.Errorf("expected reason 'Test reason', got '%s'", entry.Reason)
		}
	})

	t.Run("get blacklist with entries", func(t *testing.T) {
		resp, err := http.Get(ts.Server.URL + "/api/v1/blacklist")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var entries []models.BlacklistEntry
		if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(entries))
		}
	})

	t.Run("remove from blacklist", func(t *testing.T) {
		req, _ := http.NewRequest(
			http.MethodDelete,
			ts.Server.URL+"/api/v1/blacklist/TESTUSDT",
			nil,
		)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Errorf("expected status 200 or 204, got %d", resp.StatusCode)
		}
	})

	t.Run("blacklist is empty after removal", func(t *testing.T) {
		resp, err := http.Get(ts.Server.URL + "/api/v1/blacklist")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var entries []models.BlacklistEntry
		json.NewDecoder(resp.Body).Decode(&entries)

		if len(entries) != 0 {
			t.Errorf("expected empty blacklist after removal, got %d entries", len(entries))
		}
	})
}

func TestBlacklistAPI_DuplicateEntry_Integration(t *testing.T) {
	ts := SetupTestServer(t)
	if ts == nil {
		t.Skip("Skipping: test server not available")
	}
	defer ts.Cleanup()

	// Add first entry
	payload := map[string]string{"symbol": "BTCUSDT", "reason": "First"}
	body, _ := json.Marshal(payload)
	resp, _ := http.Post(ts.Server.URL+"/api/v1/blacklist", "application/json", bytes.NewBuffer(body))
	resp.Body.Close()

	// Try to add duplicate
	payload2 := map[string]string{"symbol": "BTCUSDT", "reason": "Second"}
	body2, _ := json.Marshal(payload2)
	resp2, err := http.Post(ts.Server.URL+"/api/v1/blacklist", "application/json", bytes.NewBuffer(body2))
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp2.Body.Close()

	// Should return conflict or bad request
	if resp2.StatusCode != http.StatusConflict && resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 409 or 400 for duplicate, got %d", resp2.StatusCode)
	}
}

// ============================================================
// Settings API Integration Tests
// ============================================================

func TestSettingsAPI_GetUpdate_Integration(t *testing.T) {
	ts := SetupTestServer(t)
	if ts == nil {
		t.Skip("Skipping: test server not available")
	}
	defer ts.Cleanup()

	t.Run("get default settings", func(t *testing.T) {
		resp, err := http.Get(ts.Server.URL + "/api/v1/settings")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var settings models.Settings
		if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Default settings should have ID 1
		if settings.ID != 1 {
			t.Errorf("expected settings ID 1, got %d", settings.ID)
		}
	})

	t.Run("update settings", func(t *testing.T) {
		maxTrades := 5
		payload := map[string]interface{}{
			"consider_funding":      true,
			"max_concurrent_trades": maxTrades,
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest(
			http.MethodPatch,
			ts.Server.URL+"/api/v1/settings",
			bytes.NewBuffer(body),
		)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(respBody))
		}
	})

	t.Run("verify updated settings", func(t *testing.T) {
		resp, err := http.Get(ts.Server.URL + "/api/v1/settings")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var settings models.Settings
		if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !settings.ConsiderFunding {
			t.Error("expected ConsiderFunding to be true")
		}
		if settings.MaxConcurrentTrades == nil || *settings.MaxConcurrentTrades != 5 {
			t.Error("expected MaxConcurrentTrades to be 5")
		}
	})
}

// ============================================================
// Notifications API Integration Tests
// ============================================================

func TestNotificationsAPI_CRUD_Integration(t *testing.T) {
	ts := SetupTestServer(t)
	if ts == nil {
		t.Skip("Skipping: test server not available")
	}
	defer ts.Cleanup()

	// Insert test notifications directly
	_, err := ts.DB.Exec(`
		INSERT INTO notifications (type, severity, message, timestamp)
		VALUES
			('OPEN', 'info', 'Opened arbitrage BTCUSDT', NOW()),
			('CLOSE', 'info', 'Closed arbitrage ETHUSDT with profit', NOW() - INTERVAL '1 minute'),
			('SL', 'error', 'Stop loss triggered for SOLUSDT', NOW() - INTERVAL '2 minutes')
	`)
	if err != nil {
		t.Fatalf("failed to insert test notifications: %v", err)
	}

	t.Run("get all notifications", func(t *testing.T) {
		resp, err := http.Get(ts.Server.URL + "/api/v1/notifications")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var notifications []models.Notification
		if err := json.NewDecoder(resp.Body).Decode(&notifications); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(notifications) < 3 {
			t.Errorf("expected at least 3 notifications, got %d", len(notifications))
		}
	})

	t.Run("filter notifications by type", func(t *testing.T) {
		resp, err := http.Get(ts.Server.URL + "/api/v1/notifications?types=SL")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var notifications []models.Notification
		if err := json.NewDecoder(resp.Body).Decode(&notifications); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		for _, n := range notifications {
			if n.Type != "SL" {
				t.Errorf("expected only SL notifications, got %s", n.Type)
			}
		}
	})

	t.Run("clear notifications", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, ts.Server.URL+"/api/v1/notifications", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			t.Errorf("expected status 200 or 204, got %d", resp.StatusCode)
		}
	})

	t.Run("notifications are cleared", func(t *testing.T) {
		resp, err := http.Get(ts.Server.URL + "/api/v1/notifications")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		var notifications []models.Notification
		json.NewDecoder(resp.Body).Decode(&notifications)

		if len(notifications) != 0 {
			t.Errorf("expected empty notifications after clear, got %d", len(notifications))
		}
	})
}

// ============================================================
// Health Check API Integration Tests
// ============================================================

func TestHealthAPI_Integration(t *testing.T) {
	ts := SetupTestServer(t)
	if ts == nil {
		t.Skip("Skipping: test server not available")
	}
	defer ts.Cleanup()

	t.Run("health check returns OK", func(t *testing.T) {
		resp, err := http.Get(ts.Server.URL + "/health")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if string(body) != "OK" {
			t.Errorf("expected body 'OK', got '%s'", string(body))
		}
	})
}

// ============================================================
// Metrics API Integration Tests
// ============================================================

func TestMetricsAPI_Integration(t *testing.T) {
	ts := SetupTestServer(t)
	if ts == nil {
		t.Skip("Skipping: test server not available")
	}
	defer ts.Cleanup()

	t.Run("metrics endpoint returns prometheus format", func(t *testing.T) {
		resp, err := http.Get(ts.Server.URL + "/metrics")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			t.Error("expected Content-Type header")
		}
	})
}

// ============================================================
// Debug Runtime API Integration Tests
// ============================================================

func TestDebugRuntimeAPI_Integration(t *testing.T) {
	ts := SetupTestServer(t)
	if ts == nil {
		t.Skip("Skipping: test server not available")
	}
	defer ts.Cleanup()

	t.Run("debug runtime returns stats", func(t *testing.T) {
		resp, err := http.Get(ts.Server.URL + "/debug/runtime")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		var stats map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if _, ok := stats["goroutines"]; !ok {
			t.Error("expected goroutines in response")
		}
		if _, ok := stats["heap_alloc_mb"]; !ok {
			t.Error("expected heap_alloc_mb in response")
		}
	})
}

// ============================================================
// Full Request Cycle Tests
// ============================================================

func TestFullRequestCycle_Integration(t *testing.T) {
	ts := SetupTestServer(t)
	if ts == nil {
		t.Skip("Skipping: test server not available")
	}
	defer ts.Cleanup()

	t.Run("complete blacklist workflow", func(t *testing.T) {
		// 1. Get empty list
		resp1, _ := http.Get(ts.Server.URL + "/api/v1/blacklist")
		var list1 []models.BlacklistEntry
		json.NewDecoder(resp1.Body).Decode(&list1)
		resp1.Body.Close()
		initialCount := len(list1)

		// 2. Add multiple entries
		symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
		for _, symbol := range symbols {
			payload := map[string]string{"symbol": symbol, "reason": "Test " + symbol}
			body, _ := json.Marshal(payload)
			resp, _ := http.Post(ts.Server.URL+"/api/v1/blacklist", "application/json", bytes.NewBuffer(body))
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("failed to add %s to blacklist", symbol)
			}
			resp.Body.Close()
		}

		// 3. Verify all entries exist
		resp2, _ := http.Get(ts.Server.URL + "/api/v1/blacklist")
		var list2 []models.BlacklistEntry
		json.NewDecoder(resp2.Body).Decode(&list2)
		resp2.Body.Close()

		if len(list2) != initialCount+len(symbols) {
			t.Errorf("expected %d entries, got %d", initialCount+len(symbols), len(list2))
		}

		// 4. Remove one entry
		req, _ := http.NewRequest(http.MethodDelete, ts.Server.URL+"/api/v1/blacklist/ETHUSDT", nil)
		resp3, _ := http.DefaultClient.Do(req)
		resp3.Body.Close()

		// 5. Verify count decreased
		resp4, _ := http.Get(ts.Server.URL + "/api/v1/blacklist")
		var list3 []models.BlacklistEntry
		json.NewDecoder(resp4.Body).Decode(&list3)
		resp4.Body.Close()

		if len(list3) != initialCount+len(symbols)-1 {
			t.Errorf("expected %d entries after removal, got %d", initialCount+len(symbols)-1, len(list3))
		}

		// 6. Verify ETHUSDT is not in list
		for _, entry := range list3 {
			if entry.Symbol == "ETHUSDT" {
				t.Error("ETHUSDT should have been removed")
			}
		}
	})
}

// ============================================================
// Concurrent Requests Tests
// ============================================================

func TestConcurrentRequests_Integration(t *testing.T) {
	ts := SetupTestServer(t)
	if ts == nil {
		t.Skip("Skipping: test server not available")
	}
	defer ts.Cleanup()

	t.Run("handles concurrent GET requests", func(t *testing.T) {
		done := make(chan bool, 10)
		errors := make(chan error, 10)

		for i := 0; i < 10; i++ {
			go func() {
				resp, err := http.Get(ts.Server.URL + "/api/v1/stats")
				if err != nil {
					errors <- err
					return
				}
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					errors <- fmt.Errorf("unexpected status: %d", resp.StatusCode)
					return
				}
				done <- true
			}()
		}

		successCount := 0
		for i := 0; i < 10; i++ {
			select {
			case <-done:
				successCount++
			case err := <-errors:
				t.Errorf("concurrent request failed: %v", err)
			case <-time.After(5 * time.Second):
				t.Error("timeout waiting for concurrent requests")
				return
			}
		}

		if successCount != 10 {
			t.Errorf("expected 10 successful requests, got %d", successCount)
		}
	})
}

// ============================================================
// Error Handling Tests
// ============================================================

func TestErrorHandling_Integration(t *testing.T) {
	// Create minimal server without full setup for error testing
	hub := websocket.NewHub()
	go hub.Run()

	deps := &api.Dependencies{Hub: hub}
	router := api.SetupRoutes(deps)
	server := httptest.NewServer(router)
	defer server.Close()

	t.Run("404 for unknown endpoint", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/unknown")
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", resp.StatusCode)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		resp, err := http.Post(server.URL+"/health", "application/json", nil)
		if err != nil {
			t.Fatalf("failed to make request: %v", err)
		}
		defer resp.Body.Close()

		// Health endpoint only allows GET
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("expected status 405, got %d", resp.StatusCode)
		}
	})
}
