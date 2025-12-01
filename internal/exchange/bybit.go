package exchange

import (
	"context"
	"fmt"
)

// Bybit реализует интерфейс Exchange для биржи Bybit
type Bybit struct {
	apiKey     string
	secretKey  string
	// TODO: добавить HTTP и WebSocket клиенты
}

// NewBybit создает новый экземпляр Bybit
func NewBybit() *Bybit {
	return &Bybit{}
}

func (b *Bybit) Connect(apiKey, secret, passphrase string) error {
	b.apiKey = apiKey
	b.secretKey = secret
	// TODO: инициализация HTTP клиента и проверка подключения
	return nil
}

func (b *Bybit) GetName() string {
	return "bybit"
}

func (b *Bybit) GetBalance(ctx context.Context) (float64, error) {
	// TODO: реализовать запрос баланса через Bybit API v5
	return 0, fmt.Errorf("not implemented")
}

func (b *Bybit) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	// TODO: реализовать получение тикера
	return nil, fmt.Errorf("not implemented")
}

func (b *Bybit) GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error) {
	// TODO: реализовать получение стакана ордеров
	return nil, fmt.Errorf("not implemented")
}

func (b *Bybit) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error) {
	// TODO: реализовать размещение рыночного ордера
	return nil, fmt.Errorf("not implemented")
}

func (b *Bybit) GetOpenPositions(ctx context.Context) ([]*Position, error) {
	// TODO: реализовать получение открытых позиций
	return nil, fmt.Errorf("not implemented")
}

func (b *Bybit) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	// TODO: реализовать закрытие позиции
	return fmt.Errorf("not implemented")
}

func (b *Bybit) SubscribeTicker(symbol string, callback func(*Ticker)) error {
	// TODO: реализовать WebSocket подписку на тикер
	return fmt.Errorf("not implemented")
}

func (b *Bybit) SubscribePositions(callback func(*Position)) error {
	// TODO: реализовать WebSocket подписку на позиции
	return fmt.Errorf("not implemented")
}

func (b *Bybit) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	// TODO: получить комиссию через API или вернуть стандартную 0.055%
	return 0.00055, nil
}

func (b *Bybit) GetLimits(ctx context.Context, symbol string) (*Limits, error) {
	// TODO: реализовать получение лимитов
	return nil, fmt.Errorf("not implemented")
}

func (b *Bybit) Close() error {
	// TODO: закрыть WebSocket соединения
	return nil
}
