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
)

const (
	bingxBaseURL = "https://open-api.bingx.com"
	bingxWSURL   = "wss://open-api-swap.bingx.com/swap-market"
)

type BingX struct {
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

// parseFloat парсит строку в float64 с логированием ошибок
func (b *BingX) parseFloat(value, field string) float64 {
	result, err := strconv.ParseFloat(value, 64)
	if err != nil && value != "" {
		log.Printf("[bingx] failed to parse %s %q: %v", field, value, err)
	}
	return result
}

// NewBingX создаёт новый экземпляр BingX
// Использует глобальный HTTP клиент с connection pooling и оптимизированными таймаутами
func NewBingX() *BingX {
	return &BingX{
		httpClient:      GetGlobalHTTPClient().GetClient(),
		tickerCallbacks: make(map[string]func(*Ticker)),
		closeChan:       make(chan struct{}),
	}
}

// sign создает подпись для BingX API
func (b *BingX) sign(params string) string {
	h := hmac.New(sha256.New, []byte(b.secretKey))
	h.Write([]byte(params))
	return hex.EncodeToString(h.Sum(nil))
}

func (b *BingX) doRequest(ctx context.Context, method, endpoint string, params map[string]string, signed bool) ([]byte, error) {
	var reqBody string
	reqURL := bingxBaseURL + endpoint

	query := url.Values{}
	for k, v := range params {
		query.Set(k, v)
	}

	if signed {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		query.Set("timestamp", timestamp)

		// Сортируем и создаем строку для подписи
		queryStr := query.Encode()
		signature := b.sign(queryStr)
		query.Set("signature", signature)
	}

	if method == http.MethodGet {
		if len(query) > 0 {
			reqURL += "?" + query.Encode()
		}
	} else {
		reqBody = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, strings.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	req.Header.Set("X-BX-APIKEY", b.apiKey)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var baseResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &baseResp); err != nil {
		return nil, err
	}

	if baseResp.Code != 0 {
		return nil, &ExchangeError{
			Exchange: "bingx",
			Code:     strconv.Itoa(baseResp.Code),
			Message:  baseResp.Msg,
		}
	}

	return body, nil
}

func (b *BingX) Connect(apiKey, secret, passphrase string) error {
	b.apiKey = apiKey
	b.secretKey = secret

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := b.GetBalance(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to BingX: %w", err)
	}

	b.connected = true
	return nil
}

func (b *BingX) GetName() string {
	return "bingx"
}

func (b *BingX) GetBalance(ctx context.Context) (float64, error) {
	body, err := b.doRequest(ctx, http.MethodGet, "/openApi/swap/v2/user/balance", nil, true)
	if err != nil {
		return 0, err
	}

	var resp struct {
		Data struct {
			Balance struct {
				Equity string `json:"equity"`
			} `json:"balance"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}

	equity := b.parseFloat(resp.Data.Balance.Equity, "equity")
	return equity, nil
}

func (b *BingX) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	bingxSymbol := b.toBingXSymbol(symbol)

	params := map[string]string{
		"symbol": bingxSymbol,
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/openApi/swap/v2/quote/ticker", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Symbol    string `json:"symbol"`
			LastPrice string `json:"lastPrice"`
			BidPrice  string `json:"bidPrice"`
			AskPrice  string `json:"askPrice"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	bidPrice := b.parseFloat(resp.Data.BidPrice, "bidPrice")
	askPrice := b.parseFloat(resp.Data.AskPrice, "askPrice")
	lastPrice := b.parseFloat(resp.Data.LastPrice, "lastPrice")

	return &Ticker{
		Symbol:    symbol,
		BidPrice:  bidPrice,
		AskPrice:  askPrice,
		LastPrice: lastPrice,
		Timestamp: time.Now(),
	}, nil
}

func (b *BingX) GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error) {
	if depth > 1000 {
		depth = 1000
	}

	bingxSymbol := b.toBingXSymbol(symbol)

	params := map[string]string{
		"symbol": bingxSymbol,
		"limit":  strconv.Itoa(depth),
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/openApi/swap/v2/quote/depth", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Bids [][]string `json:"bids"`
			Asks [][]string `json:"asks"`
			T    int64      `json:"T"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	orderBook := &OrderBook{
		Symbol:    symbol,
		Bids:      make([]PriceLevel, len(resp.Data.Bids)),
		Asks:      make([]PriceLevel, len(resp.Data.Asks)),
		Timestamp: time.UnixMilli(resp.Data.T),
	}

	for i, bid := range resp.Data.Bids {
		if len(bid) >= 2 {
			price := b.parseFloat(bid[0], "bid_price")
			volume := b.parseFloat(bid[1], "bid_volume")
			orderBook.Bids[i] = PriceLevel{Price: price, Volume: volume}
		}
	}

	for i, ask := range resp.Data.Asks {
		if len(ask) >= 2 {
			price := b.parseFloat(ask[0], "ask_price")
			volume := b.parseFloat(ask[1], "ask_volume")
			orderBook.Asks[i] = PriceLevel{Price: price, Volume: volume}
		}
	}

	sort.Slice(orderBook.Bids, func(i, j int) bool {
		return orderBook.Bids[i].Price > orderBook.Bids[j].Price
	})
	sort.Slice(orderBook.Asks, func(i, j int) bool {
		return orderBook.Asks[i].Price < orderBook.Asks[j].Price
	})

	return orderBook, nil
}

func (b *BingX) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error) {
	bingxSymbol := b.toBingXSymbol(symbol)

	bingxSide := "BUY"
	positionSide := "LONG"
	if side == SideSell || side == SideShort {
		bingxSide = "SELL"
		positionSide = "SHORT"
	}

	params := map[string]string{
		"symbol":       bingxSymbol,
		"side":         bingxSide,
		"positionSide": positionSide,
		"type":         "MARKET",
		"quantity":     strconv.FormatFloat(qty, 'f', -1, 64),
	}

	body, err := b.doRequest(ctx, http.MethodPost, "/openApi/swap/v2/trade/order", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Order struct {
				OrderId     string `json:"orderId"`
				Symbol      string `json:"symbol"`
				Side        string `json:"side"`
				ExecutedQty string `json:"executedQty"`
				AvgPrice    string `json:"avgPrice"`
				Status      string `json:"status"`
			} `json:"order"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	filledQty := b.parseFloat(resp.Data.Order.ExecutedQty, "executedQty")
	avgPrice := b.parseFloat(resp.Data.Order.AvgPrice, "avgPrice")

	return &Order{
		ID:           resp.Data.Order.OrderId,
		Symbol:       symbol,
		Side:         side,
		Type:         "market",
		Quantity:     qty,
		FilledQty:    filledQty,
		AvgFillPrice: avgPrice,
		Status:       OrderStatusFilled,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}, nil
}

func (b *BingX) GetOpenPositions(ctx context.Context) ([]*Position, error) {
	body, err := b.doRequest(ctx, http.MethodGet, "/openApi/swap/v2/user/positions", nil, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			Symbol          string `json:"symbol"`
			PositionSide    string `json:"positionSide"`
			PositionAmt     string `json:"positionAmt"`
			AvgPrice        string `json:"avgPrice"`
			MarkPrice       string `json:"markPrice"`
			Leverage        int    `json:"leverage"`
			UnrealizedProfit string `json:"unrealizedProfit"`
			LiquidationPrice string `json:"liquidationPrice"`
			UpdateTime      int64  `json:"updateTime"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	positions := make([]*Position, 0)
	for _, p := range resp.Data {
		posAmt := b.parseFloat(p.PositionAmt, "positionAmt")
		if posAmt == 0 {
			continue
		}

		entryPrice := b.parseFloat(p.AvgPrice, "avgPrice")
		markPrice := b.parseFloat(p.MarkPrice, "markPrice")
		unrealizedPnl := b.parseFloat(p.UnrealizedProfit, "unrealizedProfit")

		side := SideLong
		size := posAmt
		if p.PositionSide == "SHORT" || posAmt < 0 {
			side = SideShort
			if size < 0 {
				size = -size
			}
		}

		positions = append(positions, &Position{
			Symbol:        b.fromBingXSymbol(p.Symbol),
			Side:          side,
			Size:          size,
			EntryPrice:    entryPrice,
			MarkPrice:     markPrice,
			Leverage:      p.Leverage,
			UnrealizedPnl: unrealizedPnl,
			Liquidation:   false,
			UpdatedAt:     time.UnixMilli(p.UpdateTime),
		})
	}

	return positions, nil
}

func (b *BingX) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	bingxSymbol := b.toBingXSymbol(symbol)

	closeSide := "SELL"
	positionSide := "LONG"
	if side == SideShort {
		closeSide = "BUY"
		positionSide = "SHORT"
	}

	params := map[string]string{
		"symbol":       bingxSymbol,
		"side":         closeSide,
		"positionSide": positionSide,
		"type":         "MARKET",
		"quantity":     strconv.FormatFloat(qty, 'f', -1, 64),
	}

	_, err := b.doRequest(ctx, http.MethodPost, "/openApi/swap/v2/trade/order", params, true)
	return err
}

func (b *BingX) SubscribeTicker(symbol string, callback func(*Ticker)) error {
	b.callbackMu.Lock()
	b.tickerCallbacks[symbol] = callback
	b.callbackMu.Unlock()

	b.wsMu.Lock()
	if b.wsManager == nil {
		config := DefaultWSReconnectConfig()
		b.wsManager = NewWSReconnectManager("bingx", bingxWSURL, config)

		b.wsManager.SetOnMessage(b.handleMessage)
		b.wsManager.SetOnConnect(func() {
			log.Printf("[bingx] WebSocket connected")
		})
		b.wsManager.SetOnDisconnect(func(err error) {
			if err != nil {
				log.Printf("[bingx] WebSocket disconnected: %v", err)
			}
		})

		if err := b.wsManager.Connect(); err != nil {
			b.wsMu.Unlock()
			return fmt.Errorf("failed to connect to WebSocket: %w", err)
		}
	}
	wsManager := b.wsManager
	b.wsMu.Unlock()

	bingxSymbol := b.toBingXSymbol(symbol)
	subMsg := map[string]interface{}{
		"id":       fmt.Sprintf("ticker_%s", symbol),
		"reqType":  "sub",
		"dataType": fmt.Sprintf("%s@ticker", bingxSymbol),
	}

	wsManager.AddSubscription(subMsg)
	return wsManager.Send(subMsg)
}

// handleMessage обрабатывает одно сообщение из WebSocket
func (b *BingX) handleMessage(message []byte) {
	var msg struct {
		DataType string `json:"dataType"`
		Data     struct {
			Symbol    string `json:"s"`
			LastPrice string `json:"c"`
			BidPrice  string `json:"b"`
			AskPrice  string `json:"a"`
		} `json:"data"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	if strings.Contains(msg.DataType, "@ticker") {
		symbol := b.fromBingXSymbol(msg.Data.Symbol)

		b.callbackMu.RLock()
		callback, ok := b.tickerCallbacks[symbol]
		b.callbackMu.RUnlock()

		if ok && callback != nil {
			bidPrice := b.parseFloat(msg.Data.BidPrice, "ws_bidPrice")
			askPrice := b.parseFloat(msg.Data.AskPrice, "ws_askPrice")
			lastPrice := b.parseFloat(msg.Data.LastPrice, "ws_lastPrice")

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

func (b *BingX) SubscribePositions(callback func(*Position)) error {
	b.callbackMu.Lock()
	b.positionCallback = callback
	b.callbackMu.Unlock()

	// BingX требует отдельного WebSocket для приватных подписок
	return nil
}

func (b *BingX) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	return 0.0005, nil // 0.05% стандартная комиссия
}

func (b *BingX) GetLimits(ctx context.Context, symbol string) (*Limits, error) {
	bingxSymbol := b.toBingXSymbol(symbol)

	params := map[string]string{
		"symbol": bingxSymbol,
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/openApi/swap/v2/quote/contracts", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			Symbol       string `json:"symbol"`
			Size         string `json:"size"`
			TickSize     string `json:"tickSize"`
			MaxLongLeverage int `json:"maxLongLeverage"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("contract info not found for %s", symbol)
	}

	info := resp.Data[0]
	minSize := b.parseFloat(info.Size, "size")
	tickSize := b.parseFloat(info.TickSize, "tickSize")

	return &Limits{
		Symbol:      symbol,
		MinOrderQty: minSize,
		MaxOrderQty: 1000000,
		QtyStep:     minSize,
		MinNotional: 5.0,
		PriceStep:   tickSize,
		MaxLeverage: info.MaxLongLeverage,
	}, nil
}

func (b *BingX) Close() error {
	select {
	case <-b.closeChan:
	default:
		close(b.closeChan)
	}

	b.wsMu.Lock()
	if b.wsManager != nil {
		b.wsManager.Close()
		b.wsManager = nil
	}
	b.wsMu.Unlock()

	b.connected = false
	return nil
}

// toBingXSymbol конвертирует символ в формат BingX (BTCUSDT -> BTC-USDT)
func (b *BingX) toBingXSymbol(symbol string) string {
	base := strings.TrimSuffix(symbol, "USDT")
	return base + "-USDT"
}

// fromBingXSymbol конвертирует формат BingX обратно (BTC-USDT -> BTCUSDT)
func (b *BingX) fromBingXSymbol(contract string) string {
	return strings.ReplaceAll(contract, "-", "")
}
