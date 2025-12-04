package bot

import (
	"testing"

	"arbitrage/internal/models"
)

// TestCheckEntryConditions_BlocksInactiveOrOpen ensures early filters stop entry
// before тяжёлые проверки, сохраняя производительность.
func TestCheckEntryConditions_BlocksInactiveOrOpen(t *testing.T) {
	tracker := NewPriceTracker(16)
	calc := NewSpreadCalculator(tracker)
	detector := NewArbitrageDetector(tracker, calc, nil, nil)

	// Создаём положительный спред
	updatePrice(tracker, "BTCUSDT", "binance", 100.0, 101.0)
	updatePrice(tracker, "BTCUSDT", "okx", 102.0, 103.0)

	// Проставляем маржу, чтобы не падать на margin check
	detector.UpdateMarginCache("binance", 1_000)
	detector.UpdateMarginCache("okx", 1_000)

	baseConfig := &models.PairConfig{
		ID:             1,
		Symbol:         "BTCUSDT",
		EntrySpreadPct: 0.5,
		ExitSpreadPct:  0.1,
		VolumeAsset:    1,
		NOrders:        1,
	}

	tests := []struct {
		name       string
		cfg        *models.PairConfig
		runtime    *models.PairRuntime
		wantReason string
	}{
		{
			name: "inactive status blocks entry",
			cfg: &models.PairConfig{
				ID:             baseConfig.ID,
				Symbol:         baseConfig.Symbol,
				EntrySpreadPct: baseConfig.EntrySpreadPct,
				ExitSpreadPct:  baseConfig.ExitSpreadPct,
				VolumeAsset:    baseConfig.VolumeAsset,
				NOrders:        baseConfig.NOrders,
				Status:         models.PairStatusPaused,
			},
			runtime:    &models.PairRuntime{State: models.StateReady},
			wantReason: "pair status is paused",
		},
		{
			name: "open position blocks entry",
			cfg: &models.PairConfig{
				ID:             baseConfig.ID,
				Symbol:         baseConfig.Symbol,
				EntrySpreadPct: baseConfig.EntrySpreadPct,
				ExitSpreadPct:  baseConfig.ExitSpreadPct,
				VolumeAsset:    baseConfig.VolumeAsset,
				NOrders:        baseConfig.NOrders,
				Status:         models.PairStatusActive,
			},
			runtime:    &models.PairRuntime{State: models.StateHolding},
			wantReason: "pair BTCUSDT already has an open or pending position",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ps := &PairState{Config: tt.cfg, Runtime: tt.runtime}
			conditions := detector.CheckEntryConditions(ps, 0, 10, nil)
			t.Cleanup(func() { ReleaseEntryConditions(conditions) })

			if conditions.CanEnter {
				t.Fatalf("expected CanEnter=false, got true")
			}

			if conditions.Reason != tt.wantReason {
				t.Fatalf("unexpected reason: %s", conditions.Reason)
			}

			if conditions.Opportunity != nil {
				t.Fatalf("opportunity should not be retained when entry is blocked")
			}
		})
	}
}
