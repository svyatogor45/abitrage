package bot

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ============================================================
// Prometheus метрики для торгового ядра
// ============================================================
//
// Согласно требованиям производительности из Архитектуры:
// - Мониторинг латентности Tick → Order
// - Счётчики обработанных событий
// - Алерты на переполнение буферов
//
// Использование:
// - Grafana дашборды для визуализации
// - Alertmanager для уведомлений о проблемах
// - Анализ производительности в production

// ============ Метрики латентности ============

// TickToOrderLatency - время от получения цены до отправки ордера
// Buckets оптимизированы для low-latency торговли (0.1ms - 100ms)
var TickToOrderLatency = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "tick_to_order_latency_ms",
		Help:      "Latency from price tick to order submission in milliseconds",
		Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 25, 50, 100},
	},
	[]string{"symbol", "stage"},
)

// PriceUpdateLatency - время обработки ценового обновления
var PriceUpdateLatency = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "price_update_latency_ms",
		Help:      "Time to process a price update in milliseconds",
		Buckets:   []float64{0.1, 0.25, 0.5, 1, 2, 5, 10},
	},
	[]string{"symbol"},
)

// SpreadCalculationLatency - время расчёта спреда
var SpreadCalculationLatency = promauto.NewHistogram(
	prometheus.HistogramOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "spread_calculation_latency_ms",
		Help:      "Time to calculate spread in milliseconds",
		Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2},
	},
)

// OrderExecutionLatency - время исполнения ордера на бирже
var OrderExecutionLatency = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "order_execution_latency_ms",
		Help:      "Time to execute order on exchange in milliseconds",
		Buckets:   []float64{50, 100, 200, 300, 500, 1000, 2000, 5000},
	},
	[]string{"exchange", "side"},
)

// ============ Счётчики событий ============

// EventsProcessed - количество обработанных событий по типам
var EventsProcessed = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "events_processed_total",
		Help:      "Total number of processed events",
	},
	[]string{"type"}, // price_update, arbitrage_check, entry, exit
)

// TradesTotal - общее количество сделок
var TradesTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "trades_total",
		Help:      "Total number of trades",
	},
	[]string{"symbol", "result"}, // result: success, failed, rollback
)

// PnlTotal - суммарный PNL в USDT
var PnlTotal = promauto.NewCounter(
	prometheus.CounterOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "pnl_total_usdt",
		Help:      "Total realized PnL in USDT",
	},
)

// ============ Метрики состояния ============

// ActiveArbitrages - текущее количество активных арбитражей
var ActiveArbitrages = promauto.NewGauge(
	prometheus.GaugeOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "active_arbitrages",
		Help:      "Current number of active arbitrage positions",
	},
)

// ActivePairs - количество активных торговых пар
var ActivePairs = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "active_pairs",
		Help:      "Number of active trading pairs by state",
	},
	[]string{"state"}, // ready, holding, entering, exiting, paused, error
)

// ExchangeConnections - статус подключений к биржам
var ExchangeConnections = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "arbitrage",
		Subsystem: "exchange",
		Name:      "connection_status",
		Help:      "Exchange connection status (1=connected, 0=disconnected)",
	},
	[]string{"exchange"},
)

// ExchangeBalance - баланс на биржах
var ExchangeBalance = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "arbitrage",
		Subsystem: "exchange",
		Name:      "balance_usdt",
		Help:      "Exchange balance in USDT",
	},
	[]string{"exchange"},
)

// ============ Метрики производительности ============

// BufferOverflows - переполнения буферов каналов
var BufferOverflows = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "buffer_overflows_total",
		Help:      "Number of channel buffer overflows (events dropped)",
	},
	[]string{"buffer"}, // price_shard, notification, position
)

// GoroutineCount - количество горутин
var GoroutineCount = promauto.NewGauge(
	prometheus.GaugeOpts{
		Namespace: "arbitrage",
		Subsystem: "runtime",
		Name:      "goroutines",
		Help:      "Current number of goroutines",
	},
)

// ShardQueueSize - размер очереди в шардах
var ShardQueueSize = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "shard_queue_size",
		Help:      "Current size of shard event queue",
	},
	[]string{"shard"},
)

// ============ Метрики арбитража ============

// SpreadObserved - наблюдаемые спреды
var SpreadObserved = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "spread_observed_percent",
		Help:      "Observed spread values in percent",
		Buckets:   []float64{-1, -0.5, 0, 0.1, 0.2, 0.3, 0.5, 1, 2, 5},
	},
	[]string{"symbol"},
)

// OpportunitiesDetected - обнаруженные возможности
var OpportunitiesDetected = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "arbitrage",
		Subsystem: "trading",
		Name:      "opportunities_detected_total",
		Help:      "Number of arbitrage opportunities detected",
	},
	[]string{"symbol", "triggered"}, // triggered: yes, no (rejected by conditions)
)

// StopLossTriggered - срабатывания стоп-лосса
var StopLossTriggered = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "arbitrage",
		Subsystem: "risk",
		Name:      "stop_loss_triggered_total",
		Help:      "Number of stop loss triggers",
	},
	[]string{"symbol"},
)

// LiquidationsDetected - обнаруженные ликвидации
var LiquidationsDetected = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: "arbitrage",
		Subsystem: "risk",
		Name:      "liquidations_detected_total",
		Help:      "Number of liquidations detected",
	},
	[]string{"exchange", "symbol"},
)

// ============ Вспомогательные функции ============

// RecordPriceUpdateLatency записывает латентность обработки цены
func RecordPriceUpdateLatency(symbol string, latencyMs float64) {
	PriceUpdateLatency.WithLabelValues(symbol).Observe(latencyMs)
	EventsProcessed.WithLabelValues("price_update").Inc()
}

// RecordTickToOrder записывает полную латентность tick-to-order
func RecordTickToOrder(symbol, stage string, latencyMs float64) {
	TickToOrderLatency.WithLabelValues(symbol, stage).Observe(latencyMs)
}

// RecordTrade записывает информацию о сделке
func RecordTrade(symbol, result string, pnl float64) {
	TradesTotal.WithLabelValues(symbol, result).Inc()
	if result == "success" && pnl != 0 {
		PnlTotal.Add(pnl)
	}
}

// RecordBufferOverflow записывает переполнение буфера
func RecordBufferOverflow(bufferName string) {
	BufferOverflows.WithLabelValues(bufferName).Inc()
}

// UpdateActiveArbitrages обновляет счётчик активных арбитражей
func UpdateActiveArbitrages(count int64) {
	ActiveArbitrages.Set(float64(count))
}

// UpdateExchangeStatus обновляет статус биржи
func UpdateExchangeStatus(exchange string, connected bool, balance float64) {
	if connected {
		ExchangeConnections.WithLabelValues(exchange).Set(1)
	} else {
		ExchangeConnections.WithLabelValues(exchange).Set(0)
	}
	ExchangeBalance.WithLabelValues(exchange).Set(balance)
}

// RecordOpportunity записывает обнаруженную возможность
func RecordOpportunity(symbol string, triggered bool) {
	triggeredStr := "no"
	if triggered {
		triggeredStr = "yes"
	}
	OpportunitiesDetected.WithLabelValues(symbol, triggeredStr).Inc()
}

// RecordSpread записывает наблюдаемый спред
func RecordSpread(symbol string, spreadPercent float64) {
	SpreadObserved.WithLabelValues(symbol).Observe(spreadPercent)
}
