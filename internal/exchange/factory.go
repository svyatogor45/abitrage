package exchange

import (
	"fmt"
	"strings"
)

// SupportedExchanges - список поддерживаемых бирж
var SupportedExchanges = []string{
	"bybit",
	"bitget",
	"okx",
	"gate",
	"htx",
	"bingx",
}

// NewExchange создает новый экземпляр биржи по имени
func NewExchange(name string) (Exchange, error) {
	name = strings.ToLower(name)

	switch name {
	case "bybit":
		return NewBybit(), nil
	case "bitget":
		return NewBitget(), nil
	case "okx":
		return NewOKX(), nil
	case "gate":
		return NewGate(), nil
	case "htx":
		return NewHTX(), nil
	case "bingx":
		return NewBingX(), nil
	default:
		return nil, fmt.Errorf("unsupported exchange: %s", name)
	}
}

// IsSupported проверяет, поддерживается ли биржа
func IsSupported(name string) bool {
	name = strings.ToLower(name)
	for _, supported := range SupportedExchanges {
		if name == supported {
			return true
		}
	}
	return false
}
