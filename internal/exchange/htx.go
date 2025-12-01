package exchange

import (
	"context"
	"fmt"
)

type HTX struct {
	apiKey    string
	secretKey string
}

func NewHTX() *HTX {
	return &HTX{}
}

func (h *HTX) Connect(apiKey, secret, passphrase string) error {
	h.apiKey = apiKey
	h.secretKey = secret
	return nil
}

func (h *HTX) GetName() string {
	return "htx"
}

func (h *HTX) GetBalance(ctx context.Context) (float64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (h *HTX) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *HTX) GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *HTX) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *HTX) GetOpenPositions(ctx context.Context) ([]*Position, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *HTX) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	return fmt.Errorf("not implemented")
}

func (h *HTX) SubscribeTicker(symbol string, callback func(*Ticker)) error {
	return fmt.Errorf("not implemented")
}

func (h *HTX) SubscribePositions(callback func(*Position)) error {
	return fmt.Errorf("not implemented")
}

func (h *HTX) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	return 0.0004, nil // 0.04% тейкер
}

func (h *HTX) GetLimits(ctx context.Context, symbol string) (*Limits, error) {
	return nil, fmt.Errorf("not implemented")
}

func (h *HTX) Close() error {
	return nil
}
