package utils

import (
	"testing"
)

func TestValidateSymbol(t *testing.T) {
	tests := []struct {
		name    string
		symbol  string
		wantErr bool
	}{
		// Valid symbols
		{"valid BTCUSDT", "BTCUSDT", false},
		{"valid ETHUSDT", "ETHUSDT", false},
		{"valid lowercase", "btcusdt", false},
		{"valid with hyphen", "BTC-USDT", false},
		{"valid with underscore", "BTC_USDT", false},
		{"valid with slash", "BTC/USDT", false},
		{"valid short", "XY", false},
		{"valid with numbers", "1INCH", false},

		// Invalid symbols
		{"empty", "", true},
		{"single char", "B", true},
		{"too long", "BTCUSDTBTCUSDTBTCUSDTBTCUSDTXXX", true},
		{"special chars", "BTC@USDT", true},
		{"spaces", "BTC USDT", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSymbol(tt.symbol)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSymbol(%q) error = %v, wantErr %v", tt.symbol, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeSymbol(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase", "btcusdt", "BTCUSDT"},
		{"with hyphen", "btc-usdt", "BTCUSDT"},
		{"with underscore", "BTC_USDT", "BTCUSDT"},
		{"with slash", "btc/usdt", "BTCUSDT"},
		{"already normalized", "BTCUSDT", "BTCUSDT"},
		{"mixed case with hyphen", "Btc-Usdt", "BTCUSDT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeSymbol(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeSymbol(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractBaseCurrency(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		expected string
	}{
		{"BTCUSDT", "BTCUSDT", "BTC"},
		{"ETHUSDT", "ETHUSDT", "ETH"},
		{"SOLUSDT", "SOLUSDT", "SOL"},
		{"with hyphen", "BTC-USDT", "BTC"},
		{"with underscore", "ETH_USDT", "ETH"},
		{"with slash", "SOL/USDT", "SOL"},
		{"USDC pair", "BTCUSDC", "BTC"},
		{"BTC quote", "ETHBTC", "ETH"},
		{"lowercase", "btcusdt", "BTC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractBaseCurrency(tt.symbol)
			if result != tt.expected {
				t.Errorf("ExtractBaseCurrency(%q) = %q, want %q", tt.symbol, result, tt.expected)
			}
		})
	}
}

func TestExtractQuoteCurrency(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		expected string
	}{
		{"BTCUSDT", "BTCUSDT", "USDT"},
		{"ETHUSDC", "ETHUSDC", "USDC"},
		{"with hyphen", "BTC-USDT", "USDT"},
		{"with underscore", "ETH_BTC", "BTC"},
		{"with slash", "SOL/ETH", "ETH"},
		{"BTC quote", "ETHBTC", "BTC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractQuoteCurrency(tt.symbol)
			if result != tt.expected {
				t.Errorf("ExtractQuoteCurrency(%q) = %q, want %q", tt.symbol, result, tt.expected)
			}
		})
	}
}

func TestValidateSpread(t *testing.T) {
	tests := []struct {
		name    string
		spread  float64
		wantErr bool
	}{
		{"valid small", 0.1, false},
		{"valid normal", 1.0, false},
		{"valid large", 50.0, false},
		{"valid max", 100.0, false},
		{"zero", 0, true},
		{"negative", -1.0, true},
		{"too large", 101.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpread(tt.spread)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSpread(%v) error = %v, wantErr %v", tt.spread, err, tt.wantErr)
			}
		})
	}
}

func TestValidateVolume(t *testing.T) {
	tests := []struct {
		name    string
		volume  float64
		wantErr bool
	}{
		{"valid small", 0.001, false},
		{"valid normal", 100.0, false},
		{"valid large", 1000000.0, false},
		{"min volume", 1e-8, false},
		{"zero", 0, true},
		{"negative", -100.0, true},
		{"too large", 1e10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVolume(tt.volume)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVolume(%v) error = %v, wantErr %v", tt.volume, err, tt.wantErr)
			}
		})
	}
}

func TestValidateNOrders(t *testing.T) {
	tests := []struct {
		name    string
		n       int
		wantErr bool
	}{
		{"valid 1", 1, false},
		{"valid 5", 5, false},
		{"valid 100", 100, false},
		{"zero", 0, true},
		{"negative", -1, true},
		{"too large", 101, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNOrders(tt.n)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNOrders(%v) error = %v, wantErr %v", tt.n, err, tt.wantErr)
			}
		})
	}
}

func TestValidateStopLoss(t *testing.T) {
	tests := []struct {
		name    string
		sl      float64
		wantErr bool
	}{
		{"valid small", 0.5, false},
		{"valid normal", 5.0, false},
		{"valid large", 50.0, false},
		{"valid max", 100.0, false},
		{"zero", 0, true},
		{"negative", -1.0, true},
		{"too large", 101.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStopLoss(tt.sl)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStopLoss(%v) error = %v, wantErr %v", tt.sl, err, tt.wantErr)
			}
		})
	}
}

func TestValidateLeverage(t *testing.T) {
	tests := []struct {
		name     string
		leverage int
		wantErr  bool
	}{
		{"valid 1x", 1, false},
		{"valid 10x", 10, false},
		{"valid 100x", 100, false},
		{"zero", 0, true},
		{"negative", -1, true},
		{"too large", 101, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLeverage(tt.leverage)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLeverage(%v) error = %v, wantErr %v", tt.leverage, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePercentage(t *testing.T) {
	tests := []struct {
		name    string
		pct     float64
		wantErr bool
	}{
		{"valid 0", 0, false},
		{"valid 50", 50.0, false},
		{"valid 100", 100.0, false},
		{"negative", -1.0, true},
		{"too large", 101.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePercentage(tt.pct)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePercentage(%v) error = %v, wantErr %v", tt.pct, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid simple", "user@example.com", false},
		{"valid with subdomain", "user@mail.example.com", false},
		{"valid with plus", "user+tag@example.com", false},
		{"valid with dots", "first.last@example.com", false},
		{"empty", "", true},
		{"no at", "userexample.com", true},
		{"no domain", "user@", true},
		{"no user", "@example.com", true},
		{"double at", "user@@example.com", true},
		{"no tld", "user@example", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{"valid 16 chars", "1234567890123456", false},
		{"valid 32 chars", "12345678901234567890123456789012", false},
		{"valid with letters", "AbCdEfGhIjKlMnOp", false},
		{"valid with dashes", "abcd-1234-5678-efgh", false},
		{"valid with underscores", "abcd_1234_5678_efgh", false},
		{"empty", "", true},
		{"too short", "123456789012345", true},
		{"special chars", "abcd!@#$efgh1234", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAPIKey(tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAPIKey(%q) error = %v, wantErr %v", tt.apiKey, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAPISecret(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{"valid 16 chars", "1234567890123456", false},
		{"valid 64 chars", "1234567890123456789012345678901234567890123456789012345678901234", false},
		{"valid with special", "abcd1234!@#$%^&*", false},
		{"empty", "", true},
		{"too short", "123456789012345", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAPISecret(tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAPISecret(%q) error = %v, wantErr %v", tt.secret, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAPIPassphrase(t *testing.T) {
	tests := []struct {
		name       string
		passphrase string
		wantErr    bool
	}{
		{"empty allowed", "", false},
		{"valid short", "pass123", false},
		{"valid with special", "P@ssw0rd!", false},
		{"too long", string(make([]byte, 100)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAPIPassphrase(tt.passphrase)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAPIPassphrase(%q) error = %v, wantErr %v", tt.passphrase, err, tt.wantErr)
			}
		})
	}
}

func TestValidateExchange(t *testing.T) {
	tests := []struct {
		name     string
		exchange string
		wantErr  bool
	}{
		{"valid bybit", "bybit", false},
		{"valid bitget", "bitget", false},
		{"valid okx", "okx", false},
		{"valid gate", "gate", false},
		{"valid htx", "htx", false},
		{"valid bingx", "bingx", false},
		{"valid uppercase", "BYBIT", false},
		{"valid mixed case", "Bybit", false},
		{"empty", "", true},
		{"unsupported", "binance", true},
		{"unsupported kraken", "kraken", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExchange(tt.exchange)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExchange(%q) error = %v, wantErr %v", tt.exchange, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeExchange(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase", "bybit", "bybit"},
		{"uppercase", "BYBIT", "bybit"},
		{"mixed case", "ByBit", "bybit"},
		{"with spaces", "  bybit  ", "bybit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeExchange(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeExchange(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidatePairConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  PairConfigValidation
		wantErr bool
	}{
		{
			name: "valid config",
			config: PairConfigValidation{
				Symbol:      "BTCUSDT",
				EntrySpread: 0.5,
				ExitSpread:  0.2,
				Volume:      100.0,
				NOrders:     5,
				StopLoss:    5.0,
				ExchangeA:   "bybit",
				ExchangeB:   "okx",
			},
			wantErr: false,
		},
		{
			name: "invalid symbol",
			config: PairConfigValidation{
				Symbol:      "",
				EntrySpread: 0.5,
				ExitSpread:  0.2,
				Volume:      100.0,
				NOrders:     5,
			},
			wantErr: true,
		},
		{
			name: "invalid entry spread",
			config: PairConfigValidation{
				Symbol:      "BTCUSDT",
				EntrySpread: -1.0,
				ExitSpread:  0.2,
				Volume:      100.0,
				NOrders:     5,
			},
			wantErr: true,
		},
		{
			name: "invalid exit spread",
			config: PairConfigValidation{
				Symbol:      "BTCUSDT",
				EntrySpread: 0.5,
				ExitSpread:  -1.0,
				Volume:      100.0,
				NOrders:     5,
			},
			wantErr: true,
		},
		{
			name: "invalid volume",
			config: PairConfigValidation{
				Symbol:      "BTCUSDT",
				EntrySpread: 0.5,
				ExitSpread:  0.2,
				Volume:      0,
				NOrders:     5,
			},
			wantErr: true,
		},
		{
			name: "invalid n_orders",
			config: PairConfigValidation{
				Symbol:      "BTCUSDT",
				EntrySpread: 0.5,
				ExitSpread:  0.2,
				Volume:      100.0,
				NOrders:     0,
			},
			wantErr: true,
		},
		{
			name: "same exchanges",
			config: PairConfigValidation{
				Symbol:      "BTCUSDT",
				EntrySpread: 0.5,
				ExitSpread:  0.2,
				Volume:      100.0,
				NOrders:     5,
				ExchangeA:   "bybit",
				ExchangeB:   "bybit",
			},
			wantErr: true,
		},
		{
			name: "entry spread less than exit spread",
			config: PairConfigValidation{
				Symbol:      "BTCUSDT",
				EntrySpread: 0.2,
				ExitSpread:  0.5,
				Volume:      100.0,
				NOrders:     5,
			},
			wantErr: true,
		},
		{
			name: "invalid exchange",
			config: PairConfigValidation{
				Symbol:      "BTCUSDT",
				EntrySpread: 0.5,
				ExitSpread:  0.2,
				Volume:      100.0,
				NOrders:     5,
				ExchangeA:   "invalid",
				ExchangeB:   "bybit",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePairConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePairConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidationErrors(t *testing.T) {
	var errs ValidationErrors

	errs.Add("field1", "error1")
	errs.Add("field2", "error2")

	if !errs.HasErrors() {
		t.Error("ValidationErrors.HasErrors() = false, want true")
	}

	errStr := errs.Error()
	if errStr == "" {
		t.Error("ValidationErrors.Error() should not be empty")
	}

	// Should contain both errors
	if len(errs) != 2 {
		t.Errorf("ValidationErrors length = %d, want 2", len(errs))
	}
}

func TestValidationErrorsAddError(t *testing.T) {
	var errs ValidationErrors

	// Should not add nil error
	errs.AddError("field1", nil)
	if errs.HasErrors() {
		t.Error("ValidationErrors.AddError(nil) should not add error")
	}

	// Should add non-nil error
	errs.AddError("field2", ErrInvalidSymbol)
	if !errs.HasErrors() {
		t.Error("ValidationErrors.AddError(err) should add error")
	}
}

func TestIsValidSymbol(t *testing.T) {
	if !IsValidSymbol("BTCUSDT") {
		t.Error("IsValidSymbol(BTCUSDT) = false, want true")
	}
	if IsValidSymbol("") {
		t.Error("IsValidSymbol('') = true, want false")
	}
}

func TestIsValidEmail(t *testing.T) {
	if !IsValidEmail("user@example.com") {
		t.Error("IsValidEmail(user@example.com) = false, want true")
	}
	if IsValidEmail("invalid") {
		t.Error("IsValidEmail(invalid) = true, want false")
	}
}

func TestIsValidAPIKey(t *testing.T) {
	if !IsValidAPIKey("1234567890123456") {
		t.Error("IsValidAPIKey(1234567890123456) = false, want true")
	}
	if IsValidAPIKey("short") {
		t.Error("IsValidAPIKey(short) = true, want false")
	}
}

func TestIsValidExchange(t *testing.T) {
	if !IsValidExchange("bybit") {
		t.Error("IsValidExchange(bybit) = false, want true")
	}
	if IsValidExchange("invalid") {
		t.Error("IsValidExchange(invalid) = true, want false")
	}
}

func TestGetSupportedExchanges(t *testing.T) {
	exchanges := GetSupportedExchanges()

	if len(exchanges) != len(SupportedExchanges) {
		t.Errorf("GetSupportedExchanges() length = %d, want %d", len(exchanges), len(SupportedExchanges))
	}

	// Verify it's a copy
	exchanges[0] = "modified"
	if SupportedExchanges[0] == "modified" {
		t.Error("GetSupportedExchanges() should return a copy, not the original")
	}
}

// Benchmarks

func BenchmarkValidateSymbol(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ValidateSymbol("BTCUSDT")
	}
}

func BenchmarkNormalizeSymbol(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NormalizeSymbol("btc-usdt")
	}
}

func BenchmarkValidateSpread(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ValidateSpread(0.5)
	}
}

func BenchmarkValidateEmail(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ValidateEmail("user@example.com")
	}
}

func BenchmarkValidatePairConfig(b *testing.B) {
	cfg := PairConfigValidation{
		Symbol:      "BTCUSDT",
		EntrySpread: 0.5,
		ExitSpread:  0.2,
		Volume:      100.0,
		NOrders:     5,
		StopLoss:    5.0,
		ExchangeA:   "bybit",
		ExchangeB:   "okx",
	}
	for i := 0; i < b.N; i++ {
		ValidatePairConfig(cfg)
	}
}
