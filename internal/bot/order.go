package bot

import (
	"context"
	"fmt"
	"sync"

	"arbitrage/internal/config"
	"arbitrage/internal/exchange"
	"arbitrage/internal/models"
)

// OrderExecutor - исполнитель ордеров с ПАРАЛЛЕЛЬНОЙ отправкой
//
// Ключевая оптимизация:
// - Ордера на обе биржи отправляются ОДНОВРЕМЕННО (goroutines)
// - Общее время = max(время_биржи_A, время_биржи_B), а не сумма
// - Типичное ускорение: 300ms вместо 600ms
type OrderExecutor struct {
	exchanges map[string]exchange.Exchange
	cfg       config.BotConfig
	mu        sync.RWMutex
}

// ExecuteParams - параметры для исполнения арбитража
type ExecuteParams struct {
	Symbol        string
	Volume        float64 // объём в базовой валюте
	LongExchange  string  // биржа для лонга
	ShortExchange string  // биржа для шорта
	NOrders       int     // на сколько частей разбить
}

// ExecuteResult - результат исполнения
type ExecuteResult struct {
	Success bool
	Error   error

	// Открытые позиции
	Legs []models.Leg

	// Детали исполнения
	LongOrder  *exchange.Order
	ShortOrder *exchange.Order
}

// LegResult - результат одной ноги
type LegResult struct {
	Order *exchange.Order
	Error error
}

// NewOrderExecutor создаёт исполнитель
func NewOrderExecutor(exchanges map[string]exchange.Exchange, cfg config.BotConfig) *OrderExecutor {
	return &OrderExecutor{
		exchanges: exchanges,
		cfg:       cfg,
	}
}

// ExecuteParallel выполняет вход в арбитраж ПАРАЛЛЕЛЬНО на обеих биржах
//
// Тайминги:
// - Отправка ордеров: ~0ms (мгновенный запуск goroutines)
// - Ожидание ответа: max(latency_A, latency_B) ≈ 150-300ms
// - БЕЗ параллелизма было бы: latency_A + latency_B ≈ 300-600ms
func (oe *OrderExecutor) ExecuteParallel(ctx context.Context, params ExecuteParams) *ExecuteResult {
	oe.mu.RLock()
	longExch, longOk := oe.exchanges[params.LongExchange]
	shortExch, shortOk := oe.exchanges[params.ShortExchange]
	oe.mu.RUnlock()

	if !longOk || !shortOk {
		return &ExecuteResult{
			Success: false,
			Error:   fmt.Errorf("exchange not found: long=%s(%v) short=%s(%v)",
				params.LongExchange, longOk, params.ShortExchange, shortOk),
		}
	}

	// Каналы для результатов
	longCh := make(chan LegResult, 1)
	shortCh := make(chan LegResult, 1)

	// Объём для одной части (если разбиваем на N ордеров)
	partVolume := params.Volume
	if params.NOrders > 1 {
		partVolume = params.Volume / float64(params.NOrders)
	}

	// ПАРАЛЛЕЛЬНАЯ отправка ордеров
	go func() {
		order, err := longExch.PlaceMarketOrder(ctx, params.Symbol, exchange.SideBuy, partVolume)
		longCh <- LegResult{Order: order, Error: err}
	}()

	go func() {
		order, err := shortExch.PlaceMarketOrder(ctx, params.Symbol, exchange.SideSell, partVolume)
		shortCh <- LegResult{Order: order, Error: err}
	}()

	// ОПТИМИЗАЦИЯ: параллельное ожидание обоих результатов
	// Было: ждём long → потом ждём short (если long медленный, не слушаем short)
	// Стало: слушаем оба канала одновременно
	var longResult, shortResult LegResult
	var longReceived, shortReceived bool

	for !longReceived || !shortReceived {
		select {
		case longResult = <-longCh:
			longReceived = true
		case shortResult = <-shortCh:
			shortReceived = true
		case <-ctx.Done():
			// Timeout - откатываем то, что успело исполниться
			if longReceived && longResult.Error == nil {
				oe.rollbackLong(params.Symbol, longExch, longResult.Order)
			}
			if shortReceived && shortResult.Error == nil {
				oe.rollbackShort(params.Symbol, shortExch, shortResult.Order)
			}
			return &ExecuteResult{Success: false, Error: ctx.Err()}
		}
	}

	// Обработка результатов
	return oe.handleResults(ctx, params, longExch, shortExch, longResult, shortResult)
}

// handleResults обрабатывает результаты параллельного исполнения
func (oe *OrderExecutor) handleResults(
	ctx context.Context,
	params ExecuteParams,
	longExch, shortExch exchange.Exchange,
	longRes, shortRes LegResult,
) *ExecuteResult {

	// Оба успешны
	if longRes.Error == nil && shortRes.Error == nil {
		return &ExecuteResult{
			Success:    true,
			LongOrder:  longRes.Order,
			ShortOrder: shortRes.Order,
			Legs: []models.Leg{
				{
					Exchange:   params.LongExchange,
					Side:       "long",
					EntryPrice: longRes.Order.AvgFillPrice,
					Quantity:   longRes.Order.FilledQty,
				},
				{
					Exchange:   params.ShortExchange,
					Side:       "short",
					EntryPrice: shortRes.Order.AvgFillPrice,
					Quantity:   shortRes.Order.FilledQty,
				},
			},
		}
	}

	// Лонг успешен, шорт провалился - откат лонга
	if longRes.Error == nil && shortRes.Error != nil {
		oe.rollbackLong(params.Symbol, longExch, longRes.Order)
		return &ExecuteResult{
			Success: false,
			Error:   fmt.Errorf("short failed, long rolled back: %w", shortRes.Error),
		}
	}

	// Шорт успешен, лонг провалился - откат шорта
	if shortRes.Error == nil && longRes.Error != nil {
		oe.rollbackShort(params.Symbol, shortExch, shortRes.Order)
		return &ExecuteResult{
			Success: false,
			Error:   fmt.Errorf("long failed, short rolled back: %w", longRes.Error),
		}
	}

	// Оба провалились
	return &ExecuteResult{
		Success: false,
		Error:   fmt.Errorf("both legs failed: long=%v, short=%v", longRes.Error, shortRes.Error),
	}
}

// rollbackLong закрывает лонг при ошибке шорта
func (oe *OrderExecutor) rollbackLong(symbol string, exch exchange.Exchange, order *exchange.Order) {
	if order == nil || order.FilledQty == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), oe.cfg.OrderTimeout)
	defer cancel()

	// Продаём то, что купили
	_, _ = exch.PlaceMarketOrder(ctx, symbol, exchange.SideSell, order.FilledQty)
}

// rollbackShort закрывает шорт при ошибке лонга
func (oe *OrderExecutor) rollbackShort(symbol string, exch exchange.Exchange, order *exchange.Order) {
	if order == nil || order.FilledQty == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), oe.cfg.OrderTimeout)
	defer cancel()

	// Покупаем то, что продали
	_, _ = exch.PlaceMarketOrder(ctx, symbol, exchange.SideBuy, order.FilledQty)
}

// CloseParallel закрывает обе позиции параллельно
func (oe *OrderExecutor) CloseParallel(ctx context.Context, legs []models.Leg, symbol string) *ExecuteResult {
	if len(legs) != 2 {
		return &ExecuteResult{Success: false, Error: fmt.Errorf("expected 2 legs, got %d", len(legs))}
	}

	oe.mu.RLock()
	exch1, ok1 := oe.exchanges[legs[0].Exchange]
	exch2, ok2 := oe.exchanges[legs[1].Exchange]
	oe.mu.RUnlock()

	if !ok1 || !ok2 {
		return &ExecuteResult{Success: false, Error: fmt.Errorf("exchange not found")}
	}

	// Каналы для результатов
	ch1 := make(chan LegResult, 1)
	ch2 := make(chan LegResult, 1)

	// Параллельное закрытие
	go func() {
		var side string
		if legs[0].Side == "long" {
			side = exchange.SideSell // закрываем лонг продажей
		} else {
			side = exchange.SideBuy // закрываем шорт покупкой
		}
		order, err := exch1.PlaceMarketOrder(ctx, symbol, side, legs[0].Quantity)
		ch1 <- LegResult{Order: order, Error: err}
	}()

	go func() {
		var side string
		if legs[1].Side == "long" {
			side = exchange.SideSell
		} else {
			side = exchange.SideBuy
		}
		order, err := exch2.PlaceMarketOrder(ctx, symbol, side, legs[1].Quantity)
		ch2 <- LegResult{Order: order, Error: err}
	}()

	// ОПТИМИЗАЦИЯ: параллельное ожидание обоих результатов
	var res1, res2 LegResult
	var res1Received, res2Received bool

	for !res1Received || !res2Received {
		select {
		case res1 = <-ch1:
			res1Received = true
		case res2 = <-ch2:
			res2Received = true
		case <-ctx.Done():
			return &ExecuteResult{Success: false, Error: ctx.Err()}
		}
	}

	// Проверяем результаты
	if res1.Error != nil || res2.Error != nil {
		return &ExecuteResult{
			Success: false,
			Error:   fmt.Errorf("close failed: leg1=%v, leg2=%v", res1.Error, res2.Error),
		}
	}

	return &ExecuteResult{
		Success:    true,
		LongOrder:  res1.Order,
		ShortOrder: res2.Order,
	}
}

// UpdateExchanges обновляет карту бирж (потокобезопасно)
func (oe *OrderExecutor) UpdateExchanges(exchanges map[string]exchange.Exchange) {
	oe.mu.Lock()
	oe.exchanges = exchanges
	oe.mu.Unlock()
}
