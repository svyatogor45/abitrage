package exchange

import (
	"context"
	"fmt"
)

type OKX struct {
	apiKey     string
	secretKey  string
	passphrase string
}

func NewOKX() *OKX {
	return &OKX{}
}

func (o *OKX) Connect(apiKey, secret, passphrase string) error {
	o.apiKey = apiKey
	o.secretKey = secret
	o.passphrase = passphrase
	return nil
}

func (o *OKX) GetName() string {
	return "okx"
}

func (o *OKX) GetBalance(ctx context.Context) (float64, error) {
	return 0, fmt.Errorf("not implemented")
}

func (o *OKX) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	return nil, fmt.Errorf("not implemented")
}

func (o *OKX) GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error) {
	return nil, fmt.Errorf("not implemented")
}

func (o *OKX) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error) {
	return nil, fmt.Errorf("not implemented")
}

func (o *OKX) GetOpenPositions(ctx context.Context) ([]*Position, error) {
	return nil, fmt.Errorf("not implemented")
}

func (o *OKX) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	return fmt.Errorf("not implemented")
}

func (o *OKX) SubscribeTicker(symbol string, callback func(*Ticker)) error {
	return fmt.Errorf("not implemented")
}

func (o *OKX) SubscribePositions(callback func(*Position)) error {
	return fmt.Errorf("not implemented")
}

func (o *OKX) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	return 0.0005, nil // 0.05% тейкер
}

func (o *OKX) GetLimits(ctx context.Context, symbol string) (*Limits, error) {
	return nil, fmt.Errorf("not implemented")
}

func (o *OKX) Close() error {
	return nil
}
