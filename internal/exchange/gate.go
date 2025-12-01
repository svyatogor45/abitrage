package exchange

import (
	"context"
	"fmt"
)

type Gate struct {
	apiKey    string
	secretKey string
}

func NewGate() *Gate {
	return &Gate{}
}

func (g *Gate) Connect(apiKey, secret, passphrase string) error {
	g.apiKey = apiKey
	g.secretKey = secret
	return nil
}

func (g *Gate) GetName() string {
	return "gate"
}

func (g *Gate) GetBalance(ctx context.Context) (float64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (g *Gate) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *Gate) GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *Gate) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *Gate) GetOpenPositions(ctx context.Context) ([]*Position, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *Gate) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	return fmt.Errorf("not implemented")
}

func (g *Gate) SubscribeTicker(symbol string, callback func(*Ticker)) error {
	return fmt.Errorf("not implemented")
}

func (g *Gate) SubscribePositions(callback func(*Position)) error {
	return fmt.Errorf("not implemented")
}

func (g *Gate) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	return 0.0005, nil // 0.05% тейкер
}

func (g *Gate) GetLimits(ctx context.Context, symbol string) (*Limits, error) {
	return nil, fmt.Errorf("not implemented")
}

func (g *Gate) Close() error {
	return nil
}
