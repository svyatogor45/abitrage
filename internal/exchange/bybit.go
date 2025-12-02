package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	bybitBaseURL     = "https://api.bybit.com"
	bybitWSPublic    = "wss://stream.bybit.com/v5/public/linear"
	bybitWSPrivate   = "wss://stream.bybit.com/v5/private"
	bybitRecvWindow  = "5000"
)

// Bybit реализует интерфейс Exchange для биржи Bybit
type Bybit struct {
	apiKey    string
	secretKey string

	httpClient *http.Client

	// WebSocket managers с автоматическим переподключением
	wsPublicManager  *WSReconnectManager
	wsPrivateManager *WSReconnectManager

	// Callbacks
	tickerCallbacks   map[string]func(*Ticker)
	positionCallback  func(*Position)
	callbackMu        sync.RWMutex

	// State
	connected bool
	closeChan chan struct{}
}

// NewBybit создает новый экземпляр Bybit
// Использует глобальный HTTP клиент с connection pooling и оптимизированными таймаутами
func NewBybit() *Bybit {
	return &Bybit{
		httpClient:      GetGlobalHTTPClient().GetClient(),
		tickerCallbacks: make(map[string]func(*Ticker)),
		closeChan:       make(chan struct{}),
	}
}

// sign создает подпись для запроса к Bybit API v5
func (b *Bybit) sign(timestamp string, params string) string {
	message := timestamp + b.apiKey + bybitRecvWindow + params
	h := hmac.New(sha256.New, []byte(b.secretKey))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// doRequest выполняет HTTP запрос к Bybit API
func (b *Bybit) doRequest(ctx context.Context, method, endpoint string, params map[string]string, signed bool) ([]byte, error) {
	var reqBody string
	var reqURL string

	if method == http.MethodGet {
		query := url.Values{}
		for k, v := range params {
			query.Set(k, v)
		}
		reqBody = query.Encode()
		if reqBody != "" {
			reqURL = bybitBaseURL + endpoint + "?" + reqBody
		} else {
			reqURL = bybitBaseURL + endpoint
		}
	} else {
		reqURL = bybitBaseURL + endpoint
		if len(params) > 0 {
			jsonBytes, _ := json.Marshal(params)
			reqBody = string(jsonBytes)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, strings.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	if signed {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		var signPayload string
		if method == http.MethodGet {
			signPayload = reqBody
		} else {
			signPayload = reqBody
		}
		signature := b.sign(timestamp, signPayload)

		req.Header.Set("X-BAPI-API-KEY", b.apiKey)
		req.Header.Set("X-BAPI-SIGN", signature)
		req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
		req.Header.Set("X-BAPI-RECV-WINDOW", bybitRecvWindow)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Проверяем базовый ответ
	var baseResp struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
	}
	if err := json.Unmarshal(body, &baseResp); err != nil {
		return nil, err
	}

	if baseResp.RetCode != 0 {
		return nil, &ExchangeError{
			Exchange: "bybit",
			Code:     strconv.Itoa(baseResp.RetCode),
			Message:  baseResp.RetMsg,
		}
	}

	return body, nil
}

func (b *Bybit) Connect(apiKey, secret, passphrase string) error {
	b.apiKey = apiKey
	b.secretKey = secret

	// Проверяем подключение через получение баланса
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := b.GetBalance(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Bybit: %w", err)
	}

	b.connected = true
	return nil
}

func (b *Bybit) GetName() string {
	return "bybit"
}

func (b *Bybit) GetBalance(ctx context.Context) (float64, error) {
	params := map[string]string{
		"accountType": "UNIFIED",
		"coin":        "USDT",
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/v5/account/wallet-balance", params, true)
	if err != nil {
		return 0, err
	}

	var resp struct {
		Result struct {
			List []struct {
				Coin []struct {
					Coin   string `json:"coin"`
					Equity string `json:"equity"`
				} `json:"coin"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}

	if len(resp.Result.List) > 0 && len(resp.Result.List[0].Coin) > 0 {
		for _, coin := range resp.Result.List[0].Coin {
			if coin.Coin == "USDT" {
				equity, _ := strconv.ParseFloat(coin.Equity, 64)
				return equity, nil
			}
		}
	}

	return 0, nil
}

func (b *Bybit) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	params := map[string]string{
		"category": "linear",
		"symbol":   symbol,
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/v5/market/tickers", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			List []struct {
				Symbol    string `json:"symbol"`
				Bid1Price string `json:"bid1Price"`
				Ask1Price string `json:"ask1Price"`
				LastPrice string `json:"lastPrice"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Result.List) == 0 {
		return nil, fmt.Errorf("ticker not found for %s", symbol)
	}

	t := resp.Result.List[0]
	bidPrice, _ := strconv.ParseFloat(t.Bid1Price, 64)
	askPrice, _ := strconv.ParseFloat(t.Ask1Price, 64)
	lastPrice, _ := strconv.ParseFloat(t.LastPrice, 64)

	return &Ticker{
		Symbol:    t.Symbol,
		BidPrice:  bidPrice,
		AskPrice:  askPrice,
		LastPrice: lastPrice,
		Timestamp: time.Now(),
	}, nil
}

func (b *Bybit) GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error) {
	if depth > 500 {
		depth = 500
	}

	params := map[string]string{
		"category": "linear",
		"symbol":   symbol,
		"limit":    strconv.Itoa(depth),
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/v5/market/orderbook", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			Symbol string     `json:"s"`
			Bids   [][]string `json:"b"`
			Asks   [][]string `json:"a"`
			Ts     int64      `json:"ts"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	orderBook := &OrderBook{
		Symbol:    symbol,
		Bids:      make([]PriceLevel, len(resp.Result.Bids)),
		Asks:      make([]PriceLevel, len(resp.Result.Asks)),
		Timestamp: time.UnixMilli(resp.Result.Ts),
	}

	for i, bid := range resp.Result.Bids {
		price, _ := strconv.ParseFloat(bid[0], 64)
		volume, _ := strconv.ParseFloat(bid[1], 64)
		orderBook.Bids[i] = PriceLevel{Price: price, Volume: volume}
	}

	for i, ask := range resp.Result.Asks {
		price, _ := strconv.ParseFloat(ask[0], 64)
		volume, _ := strconv.ParseFloat(ask[1], 64)
		orderBook.Asks[i] = PriceLevel{Price: price, Volume: volume}
	}

	// Сортируем: bids по убыванию, asks по возрастанию
	sort.Slice(orderBook.Bids, func(i, j int) bool {
		return orderBook.Bids[i].Price > orderBook.Bids[j].Price
	})
	sort.Slice(orderBook.Asks, func(i, j int) bool {
		return orderBook.Asks[i].Price < orderBook.Asks[j].Price
	})

	return orderBook, nil
}

func (b *Bybit) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error) {
	// Конвертируем side в формат Bybit
	bybitSide := "Buy"
	if side == SideSell || side == SideShort {
		bybitSide = "Sell"
	}

	params := map[string]string{
		"category":    "linear",
		"symbol":      symbol,
		"side":        bybitSide,
		"orderType":   "Market",
		"qty":         strconv.FormatFloat(qty, 'f', -1, 64),
		"timeInForce": "IOC",
	}

	body, err := b.doRequest(ctx, http.MethodPost, "/v5/order/create", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			OrderId     string `json:"orderId"`
			OrderLinkId string `json:"orderLinkId"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	// Получаем детали ордера
	order := &Order{
		ID:        resp.Result.OrderId,
		Symbol:    symbol,
		Side:      side,
		Type:      "market",
		Quantity:  qty,
		Status:    OrderStatusFilled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Получаем информацию об исполнении
	execInfo, err := b.getOrderExecution(ctx, symbol, resp.Result.OrderId)
	if err == nil && execInfo != nil {
		order.FilledQty = execInfo.FilledQty
		order.AvgFillPrice = execInfo.AvgPrice
	} else {
		order.FilledQty = qty
	}

	return order, nil
}

// getOrderExecution получает информацию об исполнении ордера
func (b *Bybit) getOrderExecution(ctx context.Context, symbol, orderId string) (*struct {
	FilledQty float64
	AvgPrice  float64
}, error) {
	params := map[string]string{
		"category": "linear",
		"symbol":   symbol,
		"orderId":  orderId,
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/v5/order/realtime", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			List []struct {
				CumExecQty  string `json:"cumExecQty"`
				AvgPrice    string `json:"avgPrice"`
				OrderStatus string `json:"orderStatus"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Result.List) == 0 {
		return nil, fmt.Errorf("order not found")
	}

	o := resp.Result.List[0]
	filledQty, _ := strconv.ParseFloat(o.CumExecQty, 64)
	avgPrice, _ := strconv.ParseFloat(o.AvgPrice, 64)

	return &struct {
		FilledQty float64
		AvgPrice  float64
	}{
		FilledQty: filledQty,
		AvgPrice:  avgPrice,
	}, nil
}

func (b *Bybit) GetOpenPositions(ctx context.Context) ([]*Position, error) {
	params := map[string]string{
		"category":   "linear",
		"settleCoin": "USDT",
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/v5/position/list", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			List []struct {
				Symbol         string `json:"symbol"`
				Side           string `json:"side"`
				Size           string `json:"size"`
				AvgPrice       string `json:"avgPrice"`
				MarkPrice      string `json:"markPrice"`
				Leverage       string `json:"leverage"`
				UnrealisedPnl  string `json:"unrealisedPnl"`
				UpdatedTime    string `json:"updatedTime"`
				PositionStatus string `json:"positionStatus"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	positions := make([]*Position, 0)
	for _, p := range resp.Result.List {
		size, _ := strconv.ParseFloat(p.Size, 64)
		if size == 0 {
			continue
		}

		entryPrice, _ := strconv.ParseFloat(p.AvgPrice, 64)
		markPrice, _ := strconv.ParseFloat(p.MarkPrice, 64)
		leverage, _ := strconv.Atoi(p.Leverage)
		unrealizedPnl, _ := strconv.ParseFloat(p.UnrealisedPnl, 64)
		updatedTime, _ := strconv.ParseInt(p.UpdatedTime, 10, 64)

		side := SideLong
		if p.Side == "Sell" {
			side = SideShort
		}

		positions = append(positions, &Position{
			Symbol:        p.Symbol,
			Side:          side,
			Size:          size,
			EntryPrice:    entryPrice,
			MarkPrice:     markPrice,
			Leverage:      leverage,
			UnrealizedPnl: unrealizedPnl,
			Liquidation:   p.PositionStatus == "Liq",
			UpdatedAt:     time.UnixMilli(updatedTime),
		})
	}

	return positions, nil
}

func (b *Bybit) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	// Для закрытия позиции открываем противоположную
	closeSide := SideBuy
	if side == SideLong || side == SideBuy {
		closeSide = SideSell
	}

	_, err := b.PlaceMarketOrder(ctx, symbol, closeSide, qty)
	return err
}

func (b *Bybit) SubscribeTicker(symbol string, callback func(*Ticker)) error {
	b.callbackMu.Lock()
	b.tickerCallbacks[symbol] = callback
	b.callbackMu.Unlock()

	// Создаём WSReconnectManager если ещё не создан
	if b.wsPublicManager == nil {
		config := DefaultWSReconnectConfig()
		b.wsPublicManager = NewWSReconnectManager("bybit-public", bybitWSPublic, config)

		// Устанавливаем обработчик сообщений
		b.wsPublicManager.SetOnMessage(b.handlePublicMessage)

		// Устанавливаем callback на подключение для логирования
		b.wsPublicManager.SetOnConnect(func() {
			log.Printf("[bybit] Public WebSocket connected")
		})

		// Устанавливаем callback на отключение
		b.wsPublicManager.SetOnDisconnect(func(err error) {
			if err != nil {
				log.Printf("[bybit] Public WebSocket disconnected: %v", err)
			}
		})

		// Подключаемся
		if err := b.wsPublicManager.Connect(); err != nil {
			return fmt.Errorf("failed to connect to WebSocket: %w", err)
		}
	}

	// Формируем сообщение подписки
	subMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": []string{"tickers." + symbol},
	}

	// Добавляем подписку для восстановления после переподключения
	b.wsPublicManager.AddSubscription(subMsg)

	// Отправляем подписку
	return b.wsPublicManager.Send(subMsg)
}

// handlePublicMessage обрабатывает одно сообщение из публичного WebSocket
func (b *Bybit) handlePublicMessage(message []byte) {
	var msg struct {
		Topic string `json:"topic"`
		Data  struct {
			Symbol    string `json:"symbol"`
			Bid1Price string `json:"bid1Price"`
			Ask1Price string `json:"ask1Price"`
			LastPrice string `json:"lastPrice"`
		} `json:"data"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	if strings.HasPrefix(msg.Topic, "tickers.") {
		symbol := msg.Data.Symbol

		b.callbackMu.RLock()
		callback, ok := b.tickerCallbacks[symbol]
		b.callbackMu.RUnlock()

		if ok && callback != nil {
			bidPrice, _ := strconv.ParseFloat(msg.Data.Bid1Price, 64)
			askPrice, _ := strconv.ParseFloat(msg.Data.Ask1Price, 64)
			lastPrice, _ := strconv.ParseFloat(msg.Data.LastPrice, 64)

			callback(&Ticker{
				Symbol:    symbol,
				BidPrice:  bidPrice,
				AskPrice:  askPrice,
				LastPrice: lastPrice,
				Timestamp: time.Now(),
			})
		}
	}
}

func (b *Bybit) SubscribePositions(callback func(*Position)) error {
	b.callbackMu.Lock()
	b.positionCallback = callback
	b.callbackMu.Unlock()

	// Создаём WSReconnectManager если ещё не создан
	if b.wsPrivateManager == nil {
		config := DefaultWSReconnectConfig()
		b.wsPrivateManager = NewWSReconnectManager("bybit-private", bybitWSPrivate, config)

		// Устанавливаем функцию аутентификации
		b.wsPrivateManager.SetAuthFunc(b.authenticateWebSocket)

		// Устанавливаем обработчик сообщений
		b.wsPrivateManager.SetOnMessage(b.handlePrivateMessage)

		// Устанавливаем callback на подключение
		b.wsPrivateManager.SetOnConnect(func() {
			log.Printf("[bybit] Private WebSocket connected")
		})

		// Устанавливаем callback на отключение
		b.wsPrivateManager.SetOnDisconnect(func(err error) {
			if err != nil {
				log.Printf("[bybit] Private WebSocket disconnected: %v", err)
			}
		})

		// Подключаемся
		if err := b.wsPrivateManager.Connect(); err != nil {
			return fmt.Errorf("failed to connect to private WebSocket: %w", err)
		}
	}

	// Формируем сообщение подписки
	subMsg := map[string]interface{}{
		"op":   "subscribe",
		"args": []string{"position"},
	}

	// Добавляем подписку для восстановления после переподключения
	b.wsPrivateManager.AddSubscription(subMsg)

	// Отправляем подписку
	return b.wsPrivateManager.Send(subMsg)
}

func (b *Bybit) authenticateWebSocket(conn *websocket.Conn) error {
	expires := time.Now().UnixMilli() + 10000

	message := fmt.Sprintf("GET/realtime%d", expires)
	h := hmac.New(sha256.New, []byte(b.secretKey))
	h.Write([]byte(message))
	signature := hex.EncodeToString(h.Sum(nil))

	authMsg := map[string]interface{}{
		"op":   "auth",
		"args": []interface{}{b.apiKey, expires, signature},
	}

	return conn.WriteJSON(authMsg)
}

// handlePrivateMessage обрабатывает одно сообщение из приватного WebSocket
func (b *Bybit) handlePrivateMessage(message []byte) {
	var msg struct {
		Topic string `json:"topic"`
		Data  []struct {
			Symbol         string `json:"symbol"`
			Side           string `json:"side"`
			Size           string `json:"size"`
			EntryPrice     string `json:"entryPrice"`
			MarkPrice      string `json:"markPrice"`
			Leverage       string `json:"leverage"`
			UnrealisedPnl  string `json:"unrealisedPnl"`
			LiqPrice       string `json:"liqPrice"`
			PositionStatus string `json:"positionStatus"`
		} `json:"data"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	if msg.Topic == "position" {
		b.callbackMu.RLock()
		callback := b.positionCallback
		b.callbackMu.RUnlock()

		if callback != nil {
			for _, p := range msg.Data {
				size, _ := strconv.ParseFloat(p.Size, 64)
				entryPrice, _ := strconv.ParseFloat(p.EntryPrice, 64)
				markPrice, _ := strconv.ParseFloat(p.MarkPrice, 64)
				leverage, _ := strconv.Atoi(p.Leverage)
				unrealizedPnl, _ := strconv.ParseFloat(p.UnrealisedPnl, 64)

				side := SideLong
				if p.Side == "Sell" {
					side = SideShort
				}

				callback(&Position{
					Symbol:        p.Symbol,
					Side:          side,
					Size:          size,
					EntryPrice:    entryPrice,
					MarkPrice:     markPrice,
					Leverage:      leverage,
					UnrealizedPnl: unrealizedPnl,
					Liquidation:   p.PositionStatus == "Liq",
					UpdatedAt:     time.Now(),
				})
			}
		}
	}
}

func (b *Bybit) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	params := map[string]string{
		"category": "linear",
		"symbol":   symbol,
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/v5/account/fee-rate", params, true)
	if err != nil {
		// Возвращаем стандартную комиссию если не удалось получить
		return 0.00055, nil
	}

	var resp struct {
		Result struct {
			List []struct {
				TakerFeeRate string `json:"takerFeeRate"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return 0.00055, nil
	}

	if len(resp.Result.List) > 0 {
		fee, _ := strconv.ParseFloat(resp.Result.List[0].TakerFeeRate, 64)
		return fee, nil
	}

	return 0.00055, nil // 0.055% стандартная комиссия тейкера Bybit
}

func (b *Bybit) GetLimits(ctx context.Context, symbol string) (*Limits, error) {
	params := map[string]string{
		"category": "linear",
		"symbol":   symbol,
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/v5/market/instruments-info", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			List []struct {
				Symbol       string `json:"symbol"`
				LotSizeFilter struct {
					MinOrderQty string `json:"minOrderQty"`
					MaxOrderQty string `json:"maxOrderQty"`
					QtyStep     string `json:"qtyStep"`
				} `json:"lotSizeFilter"`
				PriceFilter struct {
					TickSize string `json:"tickSize"`
				} `json:"priceFilter"`
				LeverageFilter struct {
					MaxLeverage string `json:"maxLeverage"`
				} `json:"leverageFilter"`
			} `json:"list"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Result.List) == 0 {
		return nil, fmt.Errorf("instrument info not found for %s", symbol)
	}

	info := resp.Result.List[0]
	minOrderQty, _ := strconv.ParseFloat(info.LotSizeFilter.MinOrderQty, 64)
	maxOrderQty, _ := strconv.ParseFloat(info.LotSizeFilter.MaxOrderQty, 64)
	qtyStep, _ := strconv.ParseFloat(info.LotSizeFilter.QtyStep, 64)
	priceStep, _ := strconv.ParseFloat(info.PriceFilter.TickSize, 64)
	maxLeverage, _ := strconv.Atoi(info.LeverageFilter.MaxLeverage)

	return &Limits{
		Symbol:      symbol,
		MinOrderQty: minOrderQty,
		MaxOrderQty: maxOrderQty,
		QtyStep:     qtyStep,
		MinNotional: 5.0, // Bybit минимум 5 USDT
		PriceStep:   priceStep,
		MaxLeverage: maxLeverage,
	}, nil
}

func (b *Bybit) Close() error {
	// Закрываем closeChan только если он ещё не закрыт
	select {
	case <-b.closeChan:
		// Уже закрыт
	default:
		close(b.closeChan)
	}

	// Закрываем WebSocket managers
	if b.wsPublicManager != nil {
		b.wsPublicManager.Close()
		b.wsPublicManager = nil
	}

	if b.wsPrivateManager != nil {
		b.wsPrivateManager.Close()
		b.wsPrivateManager = nil
	}

	b.connected = false
	return nil
}
