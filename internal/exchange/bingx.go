package exchange

import (
	"context"
	"fmt"
)

type BingX struct {
	apiKey    string
	secretKey string
}

func NewBingX() *BingX {
	return &BingX{}
}

func (b *BingX) Connect(apiKey, secret, passphrase string) error {
	b.apiKey = apiKey
	b.secretKey = secret
	return nil
}

func (b *BingX) GetName() string {
	return "bingx"
}

func (b *BingX) GetBalance(ctx context.Context) (float64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (b *BingX) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	return nil, fmt.Errorf("not implemented")
}

func (b *BingX) GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error) {
	return nil, fmt.Errorf("not implemented")
}

func (b *BingX) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error) {
	return nil, fmt.Errorf("not implemented")
}

func (b *BingX) GetOpenPositions(ctx context.Context) ([]*Position, error) {
	return nil, fmt.Errorf("not implemented")
}

func (b *BingX) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	return fmt.Errorf("not implemented")
}

func (b *BingX) SubscribeTicker(symbol string, callback func(*Ticker)) error {
	return fmt.Errorf("not implemented")
}

func (b *BingX) SubscribePositions(callback func(*Position)) error {
	return fmt.Errorf("not implemented")
}

func (b *BingX) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	return 0.0005, nil // 0.05% тейкер
}

func (b *BingX) GetLimits(ctx context.Context, symbol string) (*Limits, error) {
	return nil, fmt.Errorf("not implemented")
}

func (b *BingX) Close() error {
	return nil
}
