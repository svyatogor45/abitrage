package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	gateBaseURL   = "https://api.gateio.ws/api/v4"
	gateWSURL     = "wss://fx-ws.gateio.ws/v4/ws/usdt"
)

type Gate struct {
	apiKey    string
	secretKey string

	httpClient *http.Client

	// WebSocket manager с автоматическим переподключением
	wsManager *WSReconnectManager
	wsMu      sync.Mutex // защита инициализации WebSocket manager

	tickerCallbacks  map[string]func(*Ticker)
	positionCallback func(*Position)
	callbackMu       sync.RWMutex

	connected bool
	closeChan chan struct{}
}

// NewGate создаёт новый экземпляр Gate.io
// Использует глобальный HTTP клиент с connection pooling и оптимизированными таймаутами
func NewGate() *Gate {
	return &Gate{
		httpClient:      GetGlobalHTTPClient().GetClient(),
		tickerCallbacks: make(map[string]func(*Ticker)),
		closeChan:       make(chan struct{}),
	}
}

// sign создает подпись для Gate.io API
func (g *Gate) sign(method, url, queryString, body string, timestamp int64) string {
	// Hash body with SHA512
	bodyHash := sha512.Sum512([]byte(body))
	bodyHashHex := hex.EncodeToString(bodyHash[:])

	// Create signing string
	signStr := fmt.Sprintf("%s\n%s\n%s\n%s\n%d", method, url, queryString, bodyHashHex, timestamp)

	h := hmac.New(sha512.New, []byte(g.secretKey))
	h.Write([]byte(signStr))
	return hex.EncodeToString(h.Sum(nil))
}

// parseFloat парсит строку в float64 с логированием ошибок
func (g *Gate) parseFloat(value, field string) float64 {
	result, err := strconv.ParseFloat(value, 64)
	if err != nil && value != "" {
		log.Printf("[gate] failed to parse %s %q: %v", field, value, err)
	}
	return result
}

func (g *Gate) doRequest(ctx context.Context, method, endpoint string, params map[string]string, signed bool) ([]byte, error) {
	var reqBody string
	var queryString string
	reqURL := gateBaseURL + endpoint

	if method == http.MethodGet {
		if len(params) > 0 {
			query := make([]string, 0, len(params))
			for k, v := range params {
				query = append(query, k+"="+v)
			}
			queryString = strings.Join(query, "&")
			reqURL += "?" + queryString
		}
	} else {
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
	req.Header.Set("Accept", "application/json")

	if signed {
		timestamp := time.Now().Unix()
		signature := g.sign(method, endpoint, queryString, reqBody, timestamp)

		req.Header.Set("KEY", g.apiKey)
		req.Header.Set("SIGN", signature)
		req.Header.Set("Timestamp", strconv.FormatInt(timestamp, 10))
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Label   string `json:"label"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, &ExchangeError{
				Exchange: "gate",
				Code:     errResp.Label,
				Message:  errResp.Message,
			}
		}
		return nil, fmt.Errorf("gate API error: %s", string(body))
	}

	return body, nil
}

func (g *Gate) Connect(apiKey, secret, passphrase string) error {
	g.apiKey = apiKey
	g.secretKey = secret

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := g.GetBalance(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Gate.io: %w", err)
	}

	g.connected = true
	return nil
}

func (g *Gate) GetName() string {
	return "gate"
}

func (g *Gate) GetBalance(ctx context.Context) (float64, error) {
	body, err := g.doRequest(ctx, http.MethodGet, "/futures/usdt/accounts", nil, true)
	if err != nil {
		return 0, err
	}

	var resp struct {
		Total string `json:"total"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}

	total := g.parseFloat(resp.Total, "accountTotal")
	return total, nil
}

func (g *Gate) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	contract := g.toGateSymbol(symbol)

	params := map[string]string{
		"contract": contract,
	}

	body, err := g.doRequest(ctx, http.MethodGet, "/futures/usdt/tickers", params, false)
	if err != nil {
		return nil, err
	}

	var resp []struct {
		Contract    string `json:"contract"`
		Last        string `json:"last"`
		LowestAsk   string `json:"lowest_ask"`
		HighestBid  string `json:"highest_bid"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp) == 0 {
		return nil, fmt.Errorf("ticker not found for %s", symbol)
	}

	t := resp[0]
	bidPrice := g.parseFloat(t.HighestBid, "highestBid")
	askPrice := g.parseFloat(t.LowestAsk, "lowestAsk")
	lastPrice := g.parseFloat(t.Last, "last")

	return &Ticker{
		Symbol:    symbol,
		BidPrice:  bidPrice,
		AskPrice:  askPrice,
		LastPrice: lastPrice,
		Timestamp: time.Now(),
	}, nil
}

func (g *Gate) GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error) {
	if depth > 100 {
		depth = 100
	}

	contract := g.toGateSymbol(symbol)

	params := map[string]string{
		"contract": contract,
		"limit":    strconv.Itoa(depth),
	}

	body, err := g.doRequest(ctx, http.MethodGet, "/futures/usdt/order_book", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Asks []struct {
			P string `json:"p"`
			S int64  `json:"s"`
		} `json:"asks"`
		Bids []struct {
			P string `json:"p"`
			S int64  `json:"s"`
		} `json:"bids"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	orderBook := &OrderBook{
		Symbol:    symbol,
		Bids:      make([]PriceLevel, len(resp.Bids)),
		Asks:      make([]PriceLevel, len(resp.Asks)),
		Timestamp: time.Now(),
	}

	for i, bid := range resp.Bids {
		price := g.parseFloat(bid.P, "bid.price")
		orderBook.Bids[i] = PriceLevel{Price: price, Volume: float64(bid.S)}
	}

	for i, ask := range resp.Asks {
		price := g.parseFloat(ask.P, "ask.price")
		orderBook.Asks[i] = PriceLevel{Price: price, Volume: float64(ask.S)}
	}

	sort.Slice(orderBook.Bids, func(i, j int) bool {
		return orderBook.Bids[i].Price > orderBook.Bids[j].Price
	})
	sort.Slice(orderBook.Asks, func(i, j int) bool {
		return orderBook.Asks[i].Price < orderBook.Asks[j].Price
	})

	return orderBook, nil
}

func (g *Gate) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error) {
	contract := g.toGateSymbol(symbol)

	size := int64(qty)
	if side == SideSell || side == SideShort {
		size = -size // Отрицательное значение для продажи
	}

	params := map[string]string{
		"contract": contract,
		"size":     strconv.FormatInt(size, 10),
		"price":    "0", // Market order
		"tif":      "ioc",
	}

	body, err := g.doRequest(ctx, http.MethodPost, "/futures/usdt/orders", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Id        int64  `json:"id"`
		Contract  string `json:"contract"`
		Size      int64  `json:"size"`
		FillPrice string `json:"fill_price"`
		Left      int64  `json:"left"`
		Status    string `json:"status"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	fillPrice := g.parseFloat(resp.FillPrice, "fillPrice")
	filledSize := qty - float64(resp.Left)
	if filledSize < 0 {
		filledSize = -filledSize
	}

	return &Order{
		ID:           strconv.FormatInt(resp.Id, 10),
		Symbol:       symbol,
		Side:         side,
		Type:         "market",
		Quantity:     qty,
		FilledQty:    filledSize,
		AvgFillPrice: fillPrice,
		Status:       OrderStatusFilled,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}, nil
}

func (g *Gate) GetOpenPositions(ctx context.Context) ([]*Position, error) {
	body, err := g.doRequest(ctx, http.MethodGet, "/futures/usdt/positions", nil, true)
	if err != nil {
		return nil, err
	}

	var resp []struct {
		Contract      string `json:"contract"`
		Size          int64  `json:"size"`
		EntryPrice    string `json:"entry_price"`
		MarkPrice     string `json:"mark_price"`
		Leverage      int    `json:"leverage"`
		UnrealisedPnl string `json:"unrealised_pnl"`
		LiqPrice      string `json:"liq_price"`
		UpdateTime    int64  `json:"update_time"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	positions := make([]*Position, 0)
	for _, p := range resp {
		if p.Size == 0 {
			continue
		}

		entryPrice := g.parseFloat(p.EntryPrice, "position.entryPrice")
		markPrice := g.parseFloat(p.MarkPrice, "position.markPrice")
		unrealizedPnl := g.parseFloat(p.UnrealisedPnl, "position.unrealisedPnl")

		side := SideLong
		size := float64(p.Size)
		if p.Size < 0 {
			side = SideShort
			size = -size
		}

		positions = append(positions, &Position{
			Symbol:        g.fromGateSymbol(p.Contract),
			Side:          side,
			Size:          size,
			EntryPrice:    entryPrice,
			MarkPrice:     markPrice,
			Leverage:      p.Leverage,
			UnrealizedPnl: unrealizedPnl,
			Liquidation:   false,
			UpdatedAt:     time.Unix(p.UpdateTime, 0),
		})
	}

	return positions, nil
}

func (g *Gate) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	contract := g.toGateSymbol(symbol)

	size := int64(qty)
	if side == SideLong || side == SideBuy {
		size = -size // Закрываем лонг продажей
	}

	params := map[string]string{
		"contract": contract,
		"size":     strconv.FormatInt(size, 10),
		"price":    "0",
		"tif":      "ioc",
	}

	_, err := g.doRequest(ctx, http.MethodPost, "/futures/usdt/orders", params, true)
	return err
}

func (g *Gate) SubscribeTicker(symbol string, callback func(*Ticker)) error {
	g.callbackMu.Lock()
	g.tickerCallbacks[symbol] = callback
	g.callbackMu.Unlock()

	// Защита от race condition при инициализации WebSocket manager
	g.wsMu.Lock()
	if g.wsManager == nil {
		config := DefaultWSReconnectConfig()
		g.wsManager = NewWSReconnectManager("gate", gateWSURL, config)

		g.wsManager.SetOnMessage(g.handleMessage)
		g.wsManager.SetOnConnect(func() {
			log.Printf("[gate] WebSocket connected")
		})
		g.wsManager.SetOnDisconnect(func(err error) {
			if err != nil {
				log.Printf("[gate] WebSocket disconnected: %v", err)
			}
		})

		if err := g.wsManager.Connect(); err != nil {
			g.wsMu.Unlock()
			return fmt.Errorf("failed to connect to WebSocket: %w", err)
		}
	}
	wsManager := g.wsManager
	g.wsMu.Unlock()

	contract := g.toGateSymbol(symbol)
	subMsg := map[string]interface{}{
		"time":    time.Now().Unix(),
		"channel": "futures.tickers",
		"event":   "subscribe",
		"payload": []string{contract},
	}

	wsManager.AddSubscription(subMsg)
	return wsManager.Send(subMsg)
}

// handleMessage обрабатывает одно сообщение из WebSocket
func (g *Gate) handleMessage(message []byte) {
	// Сначала определяем тип канала
	var baseMsg struct {
		Channel string          `json:"channel"`
		Event   string          `json:"event"`
		Result  json.RawMessage `json:"result"`
	}

	if err := json.Unmarshal(message, &baseMsg); err != nil {
		return
	}

	switch baseMsg.Channel {
	case "futures.tickers":
		if baseMsg.Event == "update" {
			g.handleTickerUpdate(baseMsg.Result)
		}
	case "futures.positions":
		if baseMsg.Event == "update" {
			g.handlePositionUpdate(baseMsg.Result)
		}
	}
}

// handleTickerUpdate обрабатывает обновления тикеров
func (g *Gate) handleTickerUpdate(data json.RawMessage) {
	var tickers []struct {
		Contract   string `json:"contract"`
		Last       string `json:"last"`
		LowestAsk  string `json:"lowest_ask"`
		HighestBid string `json:"highest_bid"`
	}

	if err := json.Unmarshal(data, &tickers); err != nil {
		return
	}

	for _, t := range tickers {
		symbol := g.fromGateSymbol(t.Contract)

		g.callbackMu.RLock()
		callback, ok := g.tickerCallbacks[symbol]
		g.callbackMu.RUnlock()

		if ok && callback != nil {
			bidPrice := g.parseFloat(t.HighestBid, "ws.ticker.highestBid")
			askPrice := g.parseFloat(t.LowestAsk, "ws.ticker.lowestAsk")
			lastPrice := g.parseFloat(t.Last, "ws.ticker.last")

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

// handlePositionUpdate обрабатывает обновления позиций (для обнаружения ликвидаций)
func (g *Gate) handlePositionUpdate(data json.RawMessage) {
	var positions []struct {
		Contract      string `json:"contract"`
		Size          int64  `json:"size"`
		EntryPrice    string `json:"entry_price"`
		MarkPrice     string `json:"mark_price"`
		Leverage      int    `json:"leverage"`
		UnrealisedPnl string `json:"unrealised_pnl"`
		LiqPrice      string `json:"liq_price"`
		Mode          string `json:"mode"`
		UpdateTime    int64  `json:"update_time"`
	}

	if err := json.Unmarshal(data, &positions); err != nil {
		log.Printf("[gate] failed to parse position update: %v", err)
		return
	}

	g.callbackMu.RLock()
	callback := g.positionCallback
	g.callbackMu.RUnlock()

	if callback == nil {
		return
	}

	for _, p := range positions {
		side := SideLong
		size := float64(p.Size)
		if p.Size < 0 {
			side = SideShort
			size = -size
		}

		// Проверяем ликвидацию: если размер стал 0 и была позиция
		liquidation := false
		if p.Size == 0 {
			// Позиция закрыта - возможно ликвидация
			// Gate.io не присылает явный флаг ликвидации, определяем по контексту
			liquidation = false // будет определяться через markPrice vs liqPrice в Risk Manager
		}

		callback(&Position{
			Symbol:        g.fromGateSymbol(p.Contract),
			Side:          side,
			Size:          size,
			EntryPrice:    g.parseFloat(p.EntryPrice, "ws.position.entryPrice"),
			MarkPrice:     g.parseFloat(p.MarkPrice, "ws.position.markPrice"),
			Leverage:      p.Leverage,
			UnrealizedPnl: g.parseFloat(p.UnrealisedPnl, "ws.position.unrealisedPnl"),
			Liquidation:   liquidation,
			UpdatedAt:     time.Unix(p.UpdateTime, 0),
		})
	}
}

func (g *Gate) SubscribePositions(callback func(*Position)) error {
	g.callbackMu.Lock()
	g.positionCallback = callback
	g.callbackMu.Unlock()

	// Защита от race condition при инициализации WebSocket manager
	g.wsMu.Lock()
	if g.wsManager == nil {
		config := DefaultWSReconnectConfig()
		g.wsManager = NewWSReconnectManager("gate", gateWSURL, config)

		g.wsManager.SetOnMessage(g.handleMessage)
		g.wsManager.SetOnConnect(func() {
			log.Printf("[gate] WebSocket connected")
		})
		g.wsManager.SetOnDisconnect(func(err error) {
			if err != nil {
				log.Printf("[gate] WebSocket disconnected: %v", err)
			}
		})

		if err := g.wsManager.Connect(); err != nil {
			g.wsMu.Unlock()
			return fmt.Errorf("failed to connect to WebSocket: %w", err)
		}
	}
	wsManager := g.wsManager
	g.wsMu.Unlock()

	subMsg := map[string]interface{}{
		"time":    time.Now().Unix(),
		"channel": "futures.positions",
		"event":   "subscribe",
		"payload": []string{"!all"},
		"auth": map[string]string{
			"method": "api_key",
			"KEY":    g.apiKey,
			"SIGN":   g.signWS("subscribe", "futures.positions"),
		},
	}

	wsManager.AddSubscription(subMsg)
	return wsManager.Send(subMsg)
}

func (g *Gate) authenticateWebSocket(conn *websocket.Conn) error {
	// Gate.io WebSocket authentication
	return nil // Auth happens per-subscription
}

func (g *Gate) signWS(event, channel string) string {
	timestamp := time.Now().Unix()
	message := fmt.Sprintf("channel=%s&event=%s&time=%d", channel, event, timestamp)
	h := hmac.New(sha512.New, []byte(g.secretKey))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

func (g *Gate) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	return 0.0005, nil // 0.05% стандартная комиссия
}

func (g *Gate) GetLimits(ctx context.Context, symbol string) (*Limits, error) {
	contract := g.toGateSymbol(symbol)

	params := map[string]string{
		"contract": contract,
	}

	body, err := g.doRequest(ctx, http.MethodGet, "/futures/usdt/contracts/"+contract, params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Name            string `json:"name"`
		OrderSizeMin    int64  `json:"order_size_min"`
		OrderSizeMax    int64  `json:"order_size_max"`
		QuantoMultiplier string `json:"quanto_multiplier"`
		OrderPriceDeviate string `json:"order_price_deviate"`
		LeverageMin     int    `json:"leverage_min"`
		LeverageMax     int    `json:"leverage_max"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	quantoMultiplier := g.parseFloat(resp.QuantoMultiplier, "limits.quantoMultiplier")

	return &Limits{
		Symbol:      symbol,
		MinOrderQty: float64(resp.OrderSizeMin),
		MaxOrderQty: float64(resp.OrderSizeMax),
		QtyStep:     1.0, // Gate.io использует контракты
		MinNotional: 5.0,
		PriceStep:   quantoMultiplier,
		MaxLeverage: resp.LeverageMax,
	}, nil
}

func (g *Gate) Close() error {
	select {
	case <-g.closeChan:
	default:
		close(g.closeChan)
	}

	if g.wsManager != nil {
		g.wsManager.Close()
		g.wsManager = nil
	}

	g.connected = false
	return nil
}

// toGateSymbol конвертирует символ в формат Gate.io (BTCUSDT -> BTC_USDT)
func (g *Gate) toGateSymbol(symbol string) string {
	base := strings.TrimSuffix(symbol, "USDT")
	return base + "_USDT"
}

// fromGateSymbol конвертирует формат Gate.io обратно (BTC_USDT -> BTCUSDT)
func (g *Gate) fromGateSymbol(contract string) string {
	return strings.ReplaceAll(contract, "_", "")
}
