package exchange

import (
	"context"
	"time"
)

// Exchange определяет унифицированный интерфейс для работы с любой биржей
type Exchange interface {
	// Connect устанавливает соединение с биржей
	Connect(apiKey, secret, passphrase string) error

	// GetName возвращает имя биржи
	GetName() string

	// GetBalance получает баланс фьючерсного аккаунта в USDT
	GetBalance(ctx context.Context) (float64, error)

	// GetTicker получает текущую цену актива
	GetTicker(ctx context.Context, symbol string) (*Ticker, error)

	// GetOrderBook получает стакан ордеров с заданной глубиной
	GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error)

	// PlaceMarketOrder размещает рыночный ордер
	PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error)

	// GetOpenPositions получает список открытых позиций
	GetOpenPositions(ctx context.Context) ([]*Position, error)

	// ClosePosition закрывает позицию
	ClosePosition(ctx context.Context, symbol, side string, qty float64) error

	// SubscribeTicker подписывается на обновления цен через WebSocket
	SubscribeTicker(symbol string, callback func(*Ticker)) error

	// SubscribePositions подписывается на обновления позиций (для обнаружения ликвидаций)
	SubscribePositions(callback func(*Position)) error

	// GetTradingFee получает комиссию тейкера для символа
	GetTradingFee(ctx context.Context, symbol string) (float64, error)

	// GetLimits получает торговые лимиты биржи для символа
	GetLimits(ctx context.Context, symbol string) (*Limits, error)

	// Close закрывает соединения с биржей
	Close() error
}

// Ticker содержит информацию о текущей цене
type Ticker struct {
	Symbol    string    `json:"symbol"`
	BidPrice  float64   `json:"bid_price"`  // лучшая цена покупки
	AskPrice  float64   `json:"ask_price"`  // лучшая цена продажи
	LastPrice float64   `json:"last_price"` // последняя сделка
	Timestamp time.Time `json:"timestamp"`
}

// OrderBook представляет стакан ордеров
type OrderBook struct {
	Symbol    string          `json:"symbol"`
	Bids      []PriceLevel    `json:"bids"` // заявки на покупку
	Asks      []PriceLevel    `json:"asks"` // заявки на продажу
	Timestamp time.Time       `json:"timestamp"`
}

// PriceLevel представляет уровень цены в стакане
type PriceLevel struct {
	Price  float64 `json:"price"`
	Volume float64 `json:"volume"`
}

// Order представляет ордер
type Order struct {
	ID            string    `json:"id"`
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side"`          // "buy" или "sell"
	Type          string    `json:"type"`          // "market" или "limit"
	Quantity      float64   `json:"quantity"`
	FilledQty     float64   `json:"filled_qty"`
	AvgFillPrice  float64   `json:"avg_fill_price"`
	Status        string    `json:"status"`        // "filled", "partial", "cancelled"
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Position представляет открытую позицию
type Position struct {
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side"`          // "long" или "short"
	Size          float64   `json:"size"`          // размер позиции
	EntryPrice    float64   `json:"entry_price"`   // средняя цена входа
	MarkPrice     float64   `json:"mark_price"`    // текущая маркет цена
	Leverage      int       `json:"leverage"`
	UnrealizedPnl float64   `json:"unrealized_pnl"`
	Liquidation   bool      `json:"liquidation"`   // была ли ликвидирована
	UpdatedAt     time.Time `json:"updated_at"`
}

// Limits содержит торговые ограничения биржи
type Limits struct {
	Symbol         string  `json:"symbol"`
	MinOrderQty    float64 `json:"min_order_qty"`    // минимальный размер ордера
	MaxOrderQty    float64 `json:"max_order_qty"`    // максимальный размер ордера
	QtyStep        float64 `json:"qty_step"`         // шаг изменения количества (lot size)
	MinNotional    float64 `json:"min_notional"`     // минимальная сумма сделки в USDT
	PriceStep      float64 `json:"price_step"`       // шаг изменения цены (tick size)
	MaxLeverage    int     `json:"max_leverage"`     // максимальное плечо
}

// ExchangeError представляет ошибку от биржи
type ExchangeError struct {
	Exchange string
	Code     string
	Message  string
	Original error
}

func (e *ExchangeError) Error() string {
	return e.Exchange + ": " + e.Message
}

// Unwrap возвращает оригинальную ошибку для поддержки errors.Is() и errors.As()
func (e *ExchangeError) Unwrap() error {
	return e.Original
}

// Side constants for orders (используются при размещении ордеров)
const (
	SideBuy  = "buy"  // покупка (открытие long или закрытие short)
	SideSell = "sell" // продажа (открытие short или закрытие long)
)

// Side constants for positions (используются для описания направления позиции)
const (
	SideLong  = "long"  // длинная позиция (ставка на рост)
	SideShort = "short" // короткая позиция (ставка на падение)
)

// Order status constants
const (
	OrderStatusFilled    = "filled"
	OrderStatusPartial   = "partial"
	OrderStatusCancelled = "cancelled"
	OrderStatusRejected  = "rejected"
)
