package exchange

import (
	"context"
	"fmt"
)

type Bitget struct {
	apiKey    string
	secretKey string
}

func NewBitget() *Bitget {
	return &Bitget{}
}

func (b *Bitget) Connect(apiKey, secret, passphrase string) error {
	b.apiKey = apiKey
	b.secretKey = secret
	return nil
}

func (b *Bitget) GetName() string {
	return "bitget"
}

func (b *Bitget) GetBalance(ctx context.Context) (float64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (b *Bitget) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	return nil, fmt.Errorf("not implemented")
}

func (b *Bitget) GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error) {
	return nil, fmt.Errorf("not implemented")
}

func (b *Bitget) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error) {
	return nil, fmt.Errorf("not implemented")
}

func (b *Bitget) GetOpenPositions(ctx context.Context) ([]*Position, error) {
	return nil, fmt.Errorf("not implemented")
}

func (b *Bitget) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	return fmt.Errorf("not implemented")
}

func (b *Bitget) SubscribeTicker(symbol string, callback func(*Ticker)) error {
	return fmt.Errorf("not implemented")
}

func (b *Bitget) SubscribePositions(callback func(*Position)) error {
	return fmt.Errorf("not implemented")
}

func (b *Bitget) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	return 0.0004, nil // 0.04% тейкер
}

func (b *Bitget) GetLimits(ctx context.Context, symbol string) (*Limits, error) {
	return nil, fmt.Errorf("not implemented")
}

func (b *Bitget) Close() error {
	return nil
}
