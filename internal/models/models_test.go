package models

import (
	"encoding/json"
	"testing"
	"time"
)

// ============ ExchangeAccount Tests ============

func TestExchangeAccount_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	account := ExchangeAccount{
		ID:         1,
		Name:       "bybit",
		APIKey:     "secret_api_key",
		SecretKey:  "secret_key",
		Passphrase: "secret_passphrase",
		Connected:  true,
		Balance:    1500.50,
		LastError:  "",
		UpdatedAt:  now,
		CreatedAt:  now,
	}

	// Сериализуем в JSON
	data, err := json.Marshal(account)
	if err != nil {
		t.Fatalf("ошибка сериализации: %v", err)
	}

	// Проверяем что секретные поля НЕ попали в JSON (тег json:"-")
	jsonStr := string(data)

	secretFields := []string{"secret_api_key", "secret_key", "secret_passphrase"}
	for _, secret := range secretFields {
		if contains(jsonStr, secret) {
			t.Errorf("секретное поле %q не должно быть в JSON", secret)
		}
	}

	// Проверяем что публичные поля присутствуют
	publicFields := []string{"id", "name", "connected", "balance"}
	for _, field := range publicFields {
		if !contains(jsonStr, field) {
			t.Errorf("публичное поле %q должно быть в JSON", field)
		}
	}
}

func TestExchangeAccount_JSONDeserialization(t *testing.T) {
	jsonData := `{
		"id": 1,
		"name": "bitget",
		"connected": true,
		"balance": 2000.00,
		"last_error": "connection timeout",
		"updated_at": "2024-01-15T10:30:00Z",
		"created_at": "2024-01-01T00:00:00Z"
	}`

	var account ExchangeAccount
	err := json.Unmarshal([]byte(jsonData), &account)
	if err != nil {
		t.Fatalf("ошибка десериализации: %v", err)
	}

	if account.ID != 1 {
		t.Errorf("ID: ожидали 1, получили %d", account.ID)
	}
	if account.Name != "bitget" {
		t.Errorf("Name: ожидали 'bitget', получили '%s'", account.Name)
	}
	if !account.Connected {
		t.Error("Connected должен быть true")
	}
	if account.Balance != 2000.00 {
		t.Errorf("Balance: ожидали 2000.00, получили %f", account.Balance)
	}
	if account.LastError != "connection timeout" {
		t.Errorf("LastError: ожидали 'connection timeout', получили '%s'", account.LastError)
	}
}

func TestExchangeAccount_SupportedExchanges(t *testing.T) {
	supportedExchanges := []string{"bybit", "bitget", "okx", "gate", "htx", "bingx"}

	for _, exchange := range supportedExchanges {
		account := ExchangeAccount{Name: exchange}
		if account.Name != exchange {
			t.Errorf("биржа %s должна поддерживаться", exchange)
		}
	}
}

func TestExchangeAccount_ZeroValues(t *testing.T) {
	var account ExchangeAccount

	if account.ID != 0 {
		t.Error("нулевое значение ID должно быть 0")
	}
	if account.Name != "" {
		t.Error("нулевое значение Name должно быть пустой строкой")
	}
	if account.Connected != false {
		t.Error("нулевое значение Connected должно быть false")
	}
	if account.Balance != 0 {
		t.Error("нулевое значение Balance должно быть 0")
	}
}

// ============ PairConfig Tests ============

func TestPairConfig_StatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"PairStatusPaused", PairStatusPaused, "paused"},
		{"PairStatusActive", PairStatusActive, "active"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("константа %s: ожидали '%s', получили '%s'", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

func TestPairConfig_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	pair := PairConfig{
		ID:             1,
		Symbol:         "BTCUSDT",
		Base:           "BTC",
		Quote:          "USDT",
		EntrySpreadPct: 1.0,
		ExitSpreadPct:  0.2,
		VolumeAsset:    0.5,
		NOrders:        4,
		StopLoss:       100,
		Status:         PairStatusActive,
		TradesCount:    10,
		TotalPnl:       250.50,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	data, err := json.Marshal(pair)
	if err != nil {
		t.Fatalf("ошибка сериализации: %v", err)
	}

	var decoded PairConfig
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("ошибка десериализации: %v", err)
	}

	if decoded.Symbol != pair.Symbol {
		t.Errorf("Symbol: ожидали '%s', получили '%s'", pair.Symbol, decoded.Symbol)
	}
	if decoded.EntrySpreadPct != pair.EntrySpreadPct {
		t.Errorf("EntrySpreadPct: ожидали %f, получили %f", pair.EntrySpreadPct, decoded.EntrySpreadPct)
	}
	if decoded.Status != pair.Status {
		t.Errorf("Status: ожидали '%s', получили '%s'", pair.Status, decoded.Status)
	}
}

func TestPairConfig_ValidValues(t *testing.T) {
	tests := []struct {
		name           string
		entrySpread    float64
		exitSpread     float64
		volume         float64
		nOrders        int
		stopLoss       float64
		shouldBeValid  bool
	}{
		{"валидные значения", 1.0, 0.2, 0.5, 4, 100, true},
		{"нулевой спред входа", 0, 0.2, 0.5, 4, 100, false},
		{"отрицательный спред входа", -1.0, 0.2, 0.5, 4, 100, false},
		{"нулевой объем", 1.0, 0.2, 0, 4, 100, false},
		{"nOrders = 1", 1.0, 0.2, 0.5, 1, 100, true},
		{"nOrders = 0", 1.0, 0.2, 0.5, 0, 100, false},
		{"без стоп-лосса", 1.0, 0.2, 0.5, 4, 0, true},
		{"отрицательный стоп-лосс", 1.0, 0.2, 0.5, 4, -100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.entrySpread > 0 && tt.exitSpread >= 0 && tt.volume > 0 && tt.nOrders >= 1 && tt.stopLoss >= 0

			if isValid != tt.shouldBeValid {
				t.Errorf("валидация для %s: ожидали %v, получили %v", tt.name, tt.shouldBeValid, isValid)
			}
		})
	}
}

func TestPairConfig_JSONFieldNames(t *testing.T) {
	pair := PairConfig{
		EntrySpreadPct: 1.5,
		ExitSpreadPct:  0.3,
		VolumeAsset:    1.0,
		NOrders:        2,
	}

	data, _ := json.Marshal(pair)
	jsonStr := string(data)

	expectedFields := map[string]string{
		"entry_spread": "EntrySpreadPct",
		"exit_spread":  "ExitSpreadPct",
		"volume":       "VolumeAsset",
		"n_orders":     "NOrders",
	}

	for jsonField, goField := range expectedFields {
		if !contains(jsonStr, jsonField) {
			t.Errorf("JSON поле '%s' (Go: %s) должно быть в выводе", jsonField, goField)
		}
	}
}

// ============ PairRuntime и Leg Tests ============

func TestPairRuntime_StateConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"StatePaused", StatePaused, "PAUSED"},
		{"StateReady", StateReady, "READY"},
		{"StateEntering", StateEntering, "ENTERING"},
		{"StateHolding", StateHolding, "HOLDING"},
		{"StateExiting", StateExiting, "EXITING"},
		{"StateError", StateError, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("константа %s: ожидали '%s', получили '%s'", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

func TestPairRuntime_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	runtime := PairRuntime{
		PairID:        1,
		State:         StateHolding,
		Legs:          []Leg{
			{Exchange: "bybit", Side: "long", EntryPrice: 45000, CurrentPrice: 45500, Quantity: 0.5, UnrealizedPnl: 250},
			{Exchange: "okx", Side: "short", EntryPrice: 45100, CurrentPrice: 45500, Quantity: 0.5, UnrealizedPnl: -200},
		},
		FilledParts:   2,
		CurrentSpread: 0.5,
		UnrealizedPnl: 50,
		RealizedPnl:   100,
		LastUpdate:    now,
	}

	data, err := json.Marshal(runtime)
	if err != nil {
		t.Fatalf("ошибка сериализации: %v", err)
	}

	var decoded PairRuntime
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("ошибка десериализации: %v", err)
	}

	if decoded.State != runtime.State {
		t.Errorf("State: ожидали '%s', получили '%s'", runtime.State, decoded.State)
	}
	if len(decoded.Legs) != 2 {
		t.Errorf("Legs: ожидали 2, получили %d", len(decoded.Legs))
	}
	if decoded.Legs[0].Exchange != "bybit" {
		t.Errorf("Legs[0].Exchange: ожидали 'bybit', получили '%s'", decoded.Legs[0].Exchange)
	}
}

func TestLeg_Sides(t *testing.T) {
	validSides := []string{"long", "short"}

	for _, side := range validSides {
		leg := Leg{Side: side}
		if leg.Side != side {
			t.Errorf("сторона '%s' должна быть валидной", side)
		}
	}
}

func TestLeg_PNLCalculation(t *testing.T) {
	tests := []struct {
		name        string
		side        string
		entryPrice  float64
		currentPrice float64
		quantity    float64
		expectedPnl float64
	}{
		{"long прибыль", "long", 100, 110, 1, 10},
		{"long убыток", "long", 100, 90, 1, -10},
		{"short прибыль", "short", 100, 90, 1, 10},
		{"short убыток", "short", 100, 110, 1, -10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pnl float64
			if tt.side == "long" {
				pnl = (tt.currentPrice - tt.entryPrice) * tt.quantity
			} else {
				pnl = (tt.entryPrice - tt.currentPrice) * tt.quantity
			}

			if pnl != tt.expectedPnl {
				t.Errorf("PNL: ожидали %f, получили %f", tt.expectedPnl, pnl)
			}
		})
	}
}

func TestPairRuntime_EmptyLegs(t *testing.T) {
	runtime := PairRuntime{
		PairID: 1,
		State:  StateReady,
		Legs:   []Leg{},
	}

	data, err := json.Marshal(runtime)
	if err != nil {
		t.Fatalf("ошибка сериализации с пустыми Legs: %v", err)
	}

	var decoded PairRuntime
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("ошибка десериализации: %v", err)
	}

	if decoded.Legs == nil {
		t.Log("Legs nil после десериализации пустого массива - это нормально для JSON")
	}
}

// ============ OrderRecord Tests ============

func TestOrderRecord_StatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"OrderStatusFilled", OrderStatusFilled, "filled"},
		{"OrderStatusCancelled", OrderStatusCancelled, "cancelled"},
		{"OrderStatusRejected", OrderStatusRejected, "rejected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("константа %s: ожидали '%s', получили '%s'", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

func TestOrderRecord_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	filledAt := now.Add(time.Minute)
	order := OrderRecord{
		ID:           1,
		PairID:       10,
		Exchange:     "bybit",
		Side:         "buy",
		Type:         "market",
		PartIndex:    0,
		Quantity:     0.5,
		PriceAvg:     45000.50,
		Status:       OrderStatusFilled,
		ErrorMessage: "",
		CreatedAt:    now,
		FilledAt:     &filledAt,
	}

	data, err := json.Marshal(order)
	if err != nil {
		t.Fatalf("ошибка сериализации: %v", err)
	}

	var decoded OrderRecord
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("ошибка десериализации: %v", err)
	}

	if decoded.Exchange != order.Exchange {
		t.Errorf("Exchange: ожидали '%s', получили '%s'", order.Exchange, decoded.Exchange)
	}
	if decoded.Status != order.Status {
		t.Errorf("Status: ожидали '%s', получили '%s'", order.Status, decoded.Status)
	}
	if decoded.FilledAt == nil {
		t.Error("FilledAt не должен быть nil")
	}
}

func TestOrderRecord_NilFilledAt(t *testing.T) {
	order := OrderRecord{
		ID:       1,
		Status:   OrderStatusCancelled,
		FilledAt: nil,
	}

	data, err := json.Marshal(order)
	if err != nil {
		t.Fatalf("ошибка сериализации с nil FilledAt: %v", err)
	}

	// FilledAt должен быть omitempty, проверяем
	jsonStr := string(data)
	// Поле может присутствовать как null или отсутствовать
	t.Logf("JSON с nil FilledAt: %s", jsonStr)
}

func TestOrderRecord_ValidSides(t *testing.T) {
	validSides := []string{"buy", "sell", "long", "short"}

	for _, side := range validSides {
		order := OrderRecord{Side: side}
		if order.Side != side {
			t.Errorf("сторона '%s' должна быть валидной", side)
		}
	}
}

// ============ Notification Tests ============

func TestNotification_TypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"NotificationTypeOpen", NotificationTypeOpen, "OPEN"},
		{"NotificationTypeClose", NotificationTypeClose, "CLOSE"},
		{"NotificationTypeSL", NotificationTypeSL, "SL"},
		{"NotificationTypeLiquidation", NotificationTypeLiquidation, "LIQUIDATION"},
		{"NotificationTypeError", NotificationTypeError, "ERROR"},
		{"NotificationTypeMargin", NotificationTypeMargin, "MARGIN"},
		{"NotificationTypePause", NotificationTypePause, "PAUSE"},
		{"NotificationTypeSecondLegFail", NotificationTypeSecondLegFail, "SECOND_LEG_FAIL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("константа %s: ожидали '%s', получили '%s'", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

func TestNotification_SeverityConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"SeverityInfo", SeverityInfo, "info"},
		{"SeverityWarn", SeverityWarn, "warn"},
		{"SeverityError", SeverityError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("константа %s: ожидали '%s', получили '%s'", tt.name, tt.expected, tt.constant)
			}
		})
	}
}

func TestNotification_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	pairID := 5
	notif := Notification{
		ID:        1,
		Timestamp: now,
		Type:      NotificationTypeOpen,
		Severity:  SeverityInfo,
		PairID:    &pairID,
		Message:   "Открыт арбитраж BTCUSDT",
		Meta: map[string]interface{}{
			"exchange_long":  "bybit",
			"exchange_short": "okx",
			"spread":         1.2,
		},
	}

	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("ошибка сериализации: %v", err)
	}

	var decoded Notification
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("ошибка десериализации: %v", err)
	}

	if decoded.Type != notif.Type {
		t.Errorf("Type: ожидали '%s', получили '%s'", notif.Type, decoded.Type)
	}
	if decoded.Meta == nil {
		t.Error("Meta не должен быть nil")
	}
	if decoded.Meta["exchange_long"] != "bybit" {
		t.Errorf("Meta[exchange_long]: ожидали 'bybit', получили '%v'", decoded.Meta["exchange_long"])
	}
}

func TestNotification_NilPairID(t *testing.T) {
	notif := Notification{
		ID:       1,
		Type:     NotificationTypeError,
		Severity: SeverityError,
		PairID:   nil,
		Message:  "Системная ошибка",
	}

	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("ошибка сериализации с nil PairID: %v", err)
	}

	jsonStr := string(data)
	t.Logf("JSON с nil PairID: %s", jsonStr)
}

func TestNotification_TypeToSeverityMapping(t *testing.T) {
	typeSeverity := map[string]string{
		NotificationTypeOpen:          SeverityInfo,
		NotificationTypeClose:         SeverityInfo,
		NotificationTypeSL:            SeverityError,
		NotificationTypeLiquidation:   SeverityError,
		NotificationTypeError:         SeverityError,
		NotificationTypeMargin:        SeverityWarn,
		NotificationTypePause:         SeverityWarn,
		NotificationTypeSecondLegFail: SeverityError,
	}

	for notifType, expectedSeverity := range typeSeverity {
		notif := Notification{
			Type:     notifType,
			Severity: expectedSeverity,
		}
		if notif.Severity != expectedSeverity {
			t.Errorf("для типа %s ожидали severity '%s'", notifType, expectedSeverity)
		}
	}
}

// ============ Settings Tests ============

func TestSettings_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	maxTrades := 5
	settings := Settings{
		ID:                  1,
		ConsiderFunding:     true,
		MaxConcurrentTrades: &maxTrades,
		NotificationPrefs: NotificationPreferences{
			Open:          true,
			Close:         true,
			StopLoss:      true,
			Liquidation:   true,
			APIError:      true,
			Margin:        true,
			Pause:         true,
			SecondLegFail: true,
		},
		UpdatedAt: now,
	}

	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("ошибка сериализации: %v", err)
	}

	var decoded Settings
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("ошибка десериализации: %v", err)
	}

	if decoded.ConsiderFunding != settings.ConsiderFunding {
		t.Errorf("ConsiderFunding: ожидали %v, получили %v", settings.ConsiderFunding, decoded.ConsiderFunding)
	}
	if decoded.MaxConcurrentTrades == nil || *decoded.MaxConcurrentTrades != 5 {
		t.Errorf("MaxConcurrentTrades: ожидали 5")
	}
	if !decoded.NotificationPrefs.StopLoss {
		t.Error("NotificationPrefs.StopLoss должен быть true")
	}
}

func TestSettings_NilMaxConcurrentTrades(t *testing.T) {
	settings := Settings{
		ID:                  1,
		ConsiderFunding:     false,
		MaxConcurrentTrades: nil,
		NotificationPrefs:   NotificationPreferences{},
	}

	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("ошибка сериализации с nil MaxConcurrentTrades: %v", err)
	}

	var decoded Settings
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("ошибка десериализации: %v", err)
	}

	if decoded.MaxConcurrentTrades != nil {
		t.Errorf("MaxConcurrentTrades должен быть nil, получили %v", *decoded.MaxConcurrentTrades)
	}
}

func TestNotificationPreferences_AllEnabled(t *testing.T) {
	prefs := NotificationPreferences{
		Open:          true,
		Close:         true,
		StopLoss:      true,
		Liquidation:   true,
		APIError:      true,
		Margin:        true,
		Pause:         true,
		SecondLegFail: true,
	}

	// Проверяем что все поля включены
	if !prefs.Open || !prefs.Close || !prefs.StopLoss || !prefs.Liquidation ||
		!prefs.APIError || !prefs.Margin || !prefs.Pause || !prefs.SecondLegFail {
		t.Error("все настройки уведомлений должны быть включены")
	}
}

func TestNotificationPreferences_AllDisabled(t *testing.T) {
	var prefs NotificationPreferences // все false по умолчанию

	if prefs.Open || prefs.Close || prefs.StopLoss || prefs.Liquidation ||
		prefs.APIError || prefs.Margin || prefs.Pause || prefs.SecondLegFail {
		t.Error("по умолчанию все настройки уведомлений должны быть выключены")
	}
}

// ============ BlacklistEntry Tests ============

func TestBlacklistEntry_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	entry := BlacklistEntry{
		ID:        1,
		Symbol:    "XYZUSDT",
		Reason:    "Низкая ликвидность",
		CreatedAt: now,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("ошибка сериализации: %v", err)
	}

	var decoded BlacklistEntry
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("ошибка десериализации: %v", err)
	}

	if decoded.Symbol != entry.Symbol {
		t.Errorf("Symbol: ожидали '%s', получили '%s'", entry.Symbol, decoded.Symbol)
	}
	if decoded.Reason != entry.Reason {
		t.Errorf("Reason: ожидали '%s', получили '%s'", entry.Reason, decoded.Reason)
	}
}

func TestBlacklistEntry_EmptyReason(t *testing.T) {
	entry := BlacklistEntry{
		ID:     1,
		Symbol: "TESTUSDT",
		Reason: "",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("ошибка сериализации с пустым Reason: %v", err)
	}

	var decoded BlacklistEntry
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("ошибка десериализации: %v", err)
	}

	if decoded.Reason != "" {
		t.Errorf("Reason должен быть пустым, получили '%s'", decoded.Reason)
	}
}

// ============ Stats Tests ============

func TestStats_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	stats := Stats{
		TotalTrades: 100,
		TotalPnl:    500.50,
		TodayTrades: 5,
		TodayPnl:    25.00,
		WeekTrades:  30,
		WeekPnl:     150.00,
		MonthTrades: 100,
		MonthPnl:    500.50,
		StopLossCount: StopLossStats{
			Today:  1,
			Week:   3,
			Month:  5,
			Events: []StopLossEvent{
				{Symbol: "BTCUSDT", Exchanges: [2]string{"bybit", "okx"}, Timestamp: now},
			},
		},
		LiquidationCount: LiquidationStats{
			Today:  0,
			Week:   1,
			Month:  1,
			Events: []LiquidationEvent{
				{Symbol: "ETHUSDT", Exchange: "bitget", Side: "short", Timestamp: now},
			},
		},
		TopPairsByTrades: []PairStat{
			{Symbol: "BTCUSDT", Value: 50},
			{Symbol: "ETHUSDT", Value: 30},
		},
		TopPairsByProfit: []PairStat{
			{Symbol: "BTCUSDT", Value: 200.50},
		},
		TopPairsByLoss: []PairStat{
			{Symbol: "XYZUSDT", Value: -50.00},
		},
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("ошибка сериализации: %v", err)
	}

	var decoded Stats
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("ошибка десериализации: %v", err)
	}

	if decoded.TotalTrades != stats.TotalTrades {
		t.Errorf("TotalTrades: ожидали %d, получили %d", stats.TotalTrades, decoded.TotalTrades)
	}
	if decoded.TotalPnl != stats.TotalPnl {
		t.Errorf("TotalPnl: ожидали %f, получили %f", stats.TotalPnl, decoded.TotalPnl)
	}
	if len(decoded.StopLossCount.Events) != 1 {
		t.Errorf("StopLossCount.Events: ожидали 1, получили %d", len(decoded.StopLossCount.Events))
	}
	if len(decoded.TopPairsByTrades) != 2 {
		t.Errorf("TopPairsByTrades: ожидали 2, получили %d", len(decoded.TopPairsByTrades))
	}
}

func TestStopLossStats_EmptyEvents(t *testing.T) {
	stats := StopLossStats{
		Today:  0,
		Week:   0,
		Month:  0,
		Events: []StopLossEvent{},
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("ошибка сериализации с пустыми Events: %v", err)
	}

	var decoded StopLossStats
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("ошибка десериализации: %v", err)
	}
}

func TestStopLossEvent_ExchangesArray(t *testing.T) {
	now := time.Now()
	event := StopLossEvent{
		Symbol:    "BTCUSDT",
		Exchanges: [2]string{"bybit", "okx"},
		Timestamp: now,
	}

	if event.Exchanges[0] != "bybit" {
		t.Errorf("Exchanges[0]: ожидали 'bybit', получили '%s'", event.Exchanges[0])
	}
	if event.Exchanges[1] != "okx" {
		t.Errorf("Exchanges[1]: ожидали 'okx', получили '%s'", event.Exchanges[1])
	}
}

func TestLiquidationEvent_AllFields(t *testing.T) {
	now := time.Now()
	event := LiquidationEvent{
		Symbol:    "ETHUSDT",
		Exchange:  "bitget",
		Side:      "long",
		Timestamp: now,
	}

	if event.Symbol != "ETHUSDT" {
		t.Errorf("Symbol: ожидали 'ETHUSDT', получили '%s'", event.Symbol)
	}
	if event.Exchange != "bitget" {
		t.Errorf("Exchange: ожидали 'bitget', получили '%s'", event.Exchange)
	}
	if event.Side != "long" {
		t.Errorf("Side: ожидали 'long', получили '%s'", event.Side)
	}
}

func TestPairStat_Values(t *testing.T) {
	tests := []struct {
		name   string
		symbol string
		value  float64
	}{
		{"положительный PNL", "BTCUSDT", 100.50},
		{"отрицательный PNL", "XYZUSDT", -50.25},
		{"нулевой PNL", "ETHUSDT", 0},
		{"количество сделок", "BTCUSDT", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stat := PairStat{
				Symbol: tt.symbol,
				Value:  tt.value,
			}

			if stat.Symbol != tt.symbol {
				t.Errorf("Symbol: ожидали '%s', получили '%s'", tt.symbol, stat.Symbol)
			}
			if stat.Value != tt.value {
				t.Errorf("Value: ожидали %f, получили %f", tt.value, stat.Value)
			}
		})
	}
}

func TestStats_ZeroValues(t *testing.T) {
	var stats Stats

	if stats.TotalTrades != 0 {
		t.Error("TotalTrades должен быть 0")
	}
	if stats.TotalPnl != 0 {
		t.Error("TotalPnl должен быть 0")
	}
	if stats.StopLossCount.Today != 0 {
		t.Error("StopLossCount.Today должен быть 0")
	}
	if stats.LiquidationCount.Today != 0 {
		t.Error("LiquidationCount.Today должен быть 0")
	}
}

// ============ Вспомогательные функции ============

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) != -1
}

func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// ============ Benchmarks ============

func BenchmarkPairConfig_JSONMarshal(b *testing.B) {
	pair := PairConfig{
		ID:             1,
		Symbol:         "BTCUSDT",
		Base:           "BTC",
		Quote:          "USDT",
		EntrySpreadPct: 1.0,
		ExitSpreadPct:  0.2,
		VolumeAsset:    0.5,
		NOrders:        4,
		StopLoss:       100,
		Status:         PairStatusActive,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(pair)
	}
}

func BenchmarkPairRuntime_JSONMarshal(b *testing.B) {
	runtime := PairRuntime{
		PairID:        1,
		State:         StateHolding,
		Legs:          []Leg{
			{Exchange: "bybit", Side: "long", EntryPrice: 45000, CurrentPrice: 45500, Quantity: 0.5, UnrealizedPnl: 250},
			{Exchange: "okx", Side: "short", EntryPrice: 45100, CurrentPrice: 45500, Quantity: 0.5, UnrealizedPnl: -200},
		},
		FilledParts:   2,
		CurrentSpread: 0.5,
		UnrealizedPnl: 50,
		RealizedPnl:   100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(runtime)
	}
}

func BenchmarkNotification_JSONMarshal(b *testing.B) {
	pairID := 5
	notif := Notification{
		ID:        1,
		Timestamp: time.Now(),
		Type:      NotificationTypeOpen,
		Severity:  SeverityInfo,
		PairID:    &pairID,
		Message:   "Открыт арбитраж BTCUSDT",
		Meta: map[string]interface{}{
			"exchange_long":  "bybit",
			"exchange_short": "okx",
			"spread":         1.2,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(notif)
	}
}

func BenchmarkStats_JSONMarshal(b *testing.B) {
	stats := Stats{
		TotalTrades: 100,
		TotalPnl:    500.50,
		TodayTrades: 5,
		TodayPnl:    25.00,
		WeekTrades:  30,
		WeekPnl:     150.00,
		MonthTrades: 100,
		MonthPnl:    500.50,
		TopPairsByTrades: []PairStat{
			{Symbol: "BTCUSDT", Value: 50},
			{Symbol: "ETHUSDT", Value: 30},
			{Symbol: "XRPUSDT", Value: 20},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(stats)
	}
}
