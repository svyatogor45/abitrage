package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
	bitgetBaseURL   = "https://api.bitget.com"
	bitgetWSPublic  = "wss://ws.bitget.com/v2/ws/public"
	bitgetWSPrivate = "wss://ws.bitget.com/v2/ws/private"
	bitgetProductType = "USDT-FUTURES"
)

type Bitget struct {
	apiKey     string
	secretKey  string
	passphrase string

	httpClient *http.Client

	// WebSocket managers с автоматическим переподключением
	wsPublicManager  *WSReconnectManager
	wsPrivateManager *WSReconnectManager

	tickerCallbacks  map[string]func(*Ticker)
	positionCallback func(*Position)
	callbackMu       sync.RWMutex

	connected bool
	closeChan chan struct{}
}

// NewBitget создаёт новый экземпляр Bitget
// Использует глобальный HTTP клиент с connection pooling и оптимизированными таймаутами
func NewBitget() *Bitget {
	return &Bitget{
		httpClient:      GetGlobalHTTPClient().GetClient(),
		tickerCallbacks: make(map[string]func(*Ticker)),
		closeChan:       make(chan struct{}),
	}
}

// sign создает подпись для Bitget API
func (b *Bitget) sign(timestamp, method, requestPath, body string) string {
	message := timestamp + method + requestPath + body
	h := hmac.New(sha256.New, []byte(b.secretKey))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (b *Bitget) doRequest(ctx context.Context, method, endpoint string, params map[string]string, signed bool) ([]byte, error) {
	var reqBody string
	var reqURL string

	if method == http.MethodGet {
		query := url.Values{}
		for k, v := range params {
			query.Set(k, v)
		}
		queryStr := query.Encode()
		if queryStr != "" {
			reqURL = bitgetBaseURL + endpoint + "?" + queryStr
		} else {
			reqURL = bitgetBaseURL + endpoint
		}
	} else {
		reqURL = bitgetBaseURL + endpoint
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
		var signPath string
		if method == http.MethodGet && len(params) > 0 {
			query := url.Values{}
			for k, v := range params {
				query.Set(k, v)
			}
			signPath = endpoint + "?" + query.Encode()
		} else {
			signPath = endpoint
		}
		signature := b.sign(timestamp, method, signPath, reqBody)

		req.Header.Set("ACCESS-KEY", b.apiKey)
		req.Header.Set("ACCESS-SIGN", signature)
		req.Header.Set("ACCESS-TIMESTAMP", timestamp)
		req.Header.Set("ACCESS-PASSPHRASE", b.passphrase)
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

	var baseResp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &baseResp); err != nil {
		return nil, err
	}

	if baseResp.Code != "00000" {
		return nil, &ExchangeError{
			Exchange: "bitget",
			Code:     baseResp.Code,
			Message:  baseResp.Msg,
		}
	}

	return body, nil
}

func (b *Bitget) Connect(apiKey, secret, passphrase string) error {
	b.apiKey = apiKey
	b.secretKey = secret
	b.passphrase = passphrase

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := b.GetBalance(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Bitget: %w", err)
	}

	b.connected = true
	return nil
}

func (b *Bitget) GetName() string {
	return "bitget"
}

func (b *Bitget) GetBalance(ctx context.Context) (float64, error) {
	params := map[string]string{
		"productType": bitgetProductType,
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/api/v2/mix/account/accounts", params, true)
	if err != nil {
		return 0, err
	}

	var resp struct {
		Data []struct {
			MarginCoin     string `json:"marginCoin"`
			AccountEquity  string `json:"accountEquity"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}

	for _, acc := range resp.Data {
		if acc.MarginCoin == "USDT" {
			equity, _ := strconv.ParseFloat(acc.AccountEquity, 64)
			return equity, nil
		}
	}

	return 0, nil
}

func (b *Bitget) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	params := map[string]string{
		"productType": bitgetProductType,
		"symbol":      symbol,
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/api/v2/mix/market/ticker", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			Symbol    string `json:"symbol"`
			BidPr     string `json:"bidPr"`
			AskPr     string `json:"askPr"`
			LastPr    string `json:"lastPr"`
			Timestamp string `json:"ts"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("ticker not found for %s", symbol)
	}

	t := resp.Data[0]
	bidPrice, _ := strconv.ParseFloat(t.BidPr, 64)
	askPrice, _ := strconv.ParseFloat(t.AskPr, 64)
	lastPrice, _ := strconv.ParseFloat(t.LastPr, 64)
	ts, _ := strconv.ParseInt(t.Timestamp, 10, 64)

	return &Ticker{
		Symbol:    t.Symbol,
		BidPrice:  bidPrice,
		AskPrice:  askPrice,
		LastPrice: lastPrice,
		Timestamp: time.UnixMilli(ts),
	}, nil
}

func (b *Bitget) GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error) {
	if depth > 100 {
		depth = 100
	}

	params := map[string]string{
		"productType": bitgetProductType,
		"symbol":      symbol,
		"limit":       strconv.Itoa(depth),
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/api/v2/mix/market/merge-depth", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Bids [][]string `json:"bids"`
			Asks [][]string `json:"asks"`
			Ts   string     `json:"ts"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	ts, _ := strconv.ParseInt(resp.Data.Ts, 10, 64)
	orderBook := &OrderBook{
		Symbol:    symbol,
		Bids:      make([]PriceLevel, len(resp.Data.Bids)),
		Asks:      make([]PriceLevel, len(resp.Data.Asks)),
		Timestamp: time.UnixMilli(ts),
	}

	for i, bid := range resp.Data.Bids {
		price, _ := strconv.ParseFloat(bid[0], 64)
		volume, _ := strconv.ParseFloat(bid[1], 64)
		orderBook.Bids[i] = PriceLevel{Price: price, Volume: volume}
	}

	for i, ask := range resp.Data.Asks {
		price, _ := strconv.ParseFloat(ask[0], 64)
		volume, _ := strconv.ParseFloat(ask[1], 64)
		orderBook.Asks[i] = PriceLevel{Price: price, Volume: volume}
	}

	sort.Slice(orderBook.Bids, func(i, j int) bool {
		return orderBook.Bids[i].Price > orderBook.Bids[j].Price
	})
	sort.Slice(orderBook.Asks, func(i, j int) bool {
		return orderBook.Asks[i].Price < orderBook.Asks[j].Price
	})

	return orderBook, nil
}

func (b *Bitget) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error) {
	bitgetSide := "buy"
	tradeSide := "open"
	if side == SideSell || side == SideShort {
		bitgetSide = "sell"
	}

	params := map[string]string{
		"productType": bitgetProductType,
		"symbol":      symbol,
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        bitgetSide,
		"tradeSide":   tradeSide,
		"orderType":   "market",
		"size":        strconv.FormatFloat(qty, 'f', -1, 64),
	}

	body, err := b.doRequest(ctx, http.MethodPost, "/api/v2/mix/order/place-order", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			OrderId   string `json:"orderId"`
			ClientOid string `json:"clientOid"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	order := &Order{
		ID:        resp.Data.OrderId,
		Symbol:    symbol,
		Side:      side,
		Type:      "market",
		Quantity:  qty,
		FilledQty: qty,
		Status:    OrderStatusFilled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Получаем цену исполнения
	execInfo, err := b.getOrderDetail(ctx, symbol, resp.Data.OrderId)
	if err == nil && execInfo != nil {
		order.AvgFillPrice = execInfo.AvgPrice
		order.FilledQty = execInfo.FilledQty
	}

	return order, nil
}

func (b *Bitget) getOrderDetail(ctx context.Context, symbol, orderId string) (*struct {
	FilledQty float64
	AvgPrice  float64
}, error) {
	params := map[string]string{
		"productType": bitgetProductType,
		"symbol":      symbol,
		"orderId":     orderId,
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/api/v2/mix/order/detail", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			BaseVolume string `json:"baseVolume"`
			PriceAvg   string `json:"priceAvg"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	filledQty, _ := strconv.ParseFloat(resp.Data.BaseVolume, 64)
	avgPrice, _ := strconv.ParseFloat(resp.Data.PriceAvg, 64)

	return &struct {
		FilledQty float64
		AvgPrice  float64
	}{
		FilledQty: filledQty,
		AvgPrice:  avgPrice,
	}, nil
}

func (b *Bitget) GetOpenPositions(ctx context.Context) ([]*Position, error) {
	params := map[string]string{
		"productType": bitgetProductType,
		"marginCoin":  "USDT",
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/api/v2/mix/position/all-position", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			Symbol        string `json:"symbol"`
			HoldSide      string `json:"holdSide"`
			Total         string `json:"total"`
			OpenPriceAvg  string `json:"openPriceAvg"`
			MarkPrice     string `json:"markPrice"`
			Leverage      string `json:"leverage"`
			UnrealizedPL  string `json:"unrealizedPL"`
			LiquidationPrice string `json:"liquidationPrice"`
			UTime         string `json:"uTime"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	positions := make([]*Position, 0)
	for _, p := range resp.Data {
		size, _ := strconv.ParseFloat(p.Total, 64)
		if size == 0 {
			continue
		}

		entryPrice, _ := strconv.ParseFloat(p.OpenPriceAvg, 64)
		markPrice, _ := strconv.ParseFloat(p.MarkPrice, 64)
		leverage, _ := strconv.Atoi(p.Leverage)
		unrealizedPnl, _ := strconv.ParseFloat(p.UnrealizedPL, 64)
		uTime, _ := strconv.ParseInt(p.UTime, 10, 64)

		side := SideLong
		if p.HoldSide == "short" {
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
			Liquidation:   false,
			UpdatedAt:     time.UnixMilli(uTime),
		})
	}

	return positions, nil
}

func (b *Bitget) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	closeSide := SideBuy
	if side == SideLong || side == SideBuy {
		closeSide = SideSell
	}

	params := map[string]string{
		"productType": bitgetProductType,
		"symbol":      symbol,
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        closeSide,
		"tradeSide":   "close",
		"orderType":   "market",
		"size":        strconv.FormatFloat(qty, 'f', -1, 64),
	}

	_, err := b.doRequest(ctx, http.MethodPost, "/api/v2/mix/order/place-order", params, true)
	return err
}

func (b *Bitget) SubscribeTicker(symbol string, callback func(*Ticker)) error {
	b.callbackMu.Lock()
	b.tickerCallbacks[symbol] = callback
	b.callbackMu.Unlock()

	// Создаём WSReconnectManager если ещё не создан
	if b.wsPublicManager == nil {
		config := DefaultWSReconnectConfig()
		b.wsPublicManager = NewWSReconnectManager("bitget-public", bitgetWSPublic, config)

		b.wsPublicManager.SetOnMessage(b.handlePublicMessage)
		b.wsPublicManager.SetOnConnect(func() {
			log.Printf("[bitget] Public WebSocket connected")
		})
		b.wsPublicManager.SetOnDisconnect(func(err error) {
			if err != nil {
				log.Printf("[bitget] Public WebSocket disconnected: %v", err)
			}
		})

		if err := b.wsPublicManager.Connect(); err != nil {
			return fmt.Errorf("failed to connect to WebSocket: %w", err)
		}
	}

	subMsg := map[string]interface{}{
		"op": "subscribe",
		"args": []map[string]string{
			{
				"instType": "USDT-FUTURES",
				"channel":  "ticker",
				"instId":   symbol,
			},
		},
	}

	b.wsPublicManager.AddSubscription(subMsg)
	return b.wsPublicManager.Send(subMsg)
}

// handlePublicMessage обрабатывает одно сообщение из публичного WebSocket
func (b *Bitget) handlePublicMessage(message []byte) {
	var msg struct {
		Action string `json:"action"`
		Arg    struct {
			Channel string `json:"channel"`
			InstId  string `json:"instId"`
		} `json:"arg"`
		Data []struct {
			BidPr  string `json:"bidPr"`
			AskPr  string `json:"askPr"`
			LastPr string `json:"lastPr"`
			Ts     string `json:"ts"`
		} `json:"data"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	if msg.Arg.Channel == "ticker" && len(msg.Data) > 0 {
		symbol := msg.Arg.InstId

		b.callbackMu.RLock()
		callback, ok := b.tickerCallbacks[symbol]
		b.callbackMu.RUnlock()

		if ok && callback != nil {
			d := msg.Data[0]
			bidPrice, _ := strconv.ParseFloat(d.BidPr, 64)
			askPrice, _ := strconv.ParseFloat(d.AskPr, 64)
			lastPrice, _ := strconv.ParseFloat(d.LastPr, 64)
			ts, _ := strconv.ParseInt(d.Ts, 10, 64)

			callback(&Ticker{
				Symbol:    symbol,
				BidPrice:  bidPrice,
				AskPrice:  askPrice,
				LastPrice: lastPrice,
				Timestamp: time.UnixMilli(ts),
			})
		}
	}
}

func (b *Bitget) SubscribePositions(callback func(*Position)) error {
	b.callbackMu.Lock()
	b.positionCallback = callback
	b.callbackMu.Unlock()

	// Создаём WSReconnectManager если ещё не создан
	if b.wsPrivateManager == nil {
		config := DefaultWSReconnectConfig()
		b.wsPrivateManager = NewWSReconnectManager("bitget-private", bitgetWSPrivate, config)

		b.wsPrivateManager.SetAuthFunc(b.authenticateWebSocket)
		b.wsPrivateManager.SetOnMessage(b.handlePrivateMessage)
		b.wsPrivateManager.SetOnConnect(func() {
			log.Printf("[bitget] Private WebSocket connected")
		})
		b.wsPrivateManager.SetOnDisconnect(func(err error) {
			if err != nil {
				log.Printf("[bitget] Private WebSocket disconnected: %v", err)
			}
		})

		if err := b.wsPrivateManager.Connect(); err != nil {
			return fmt.Errorf("failed to connect to private WebSocket: %w", err)
		}
	}

	subMsg := map[string]interface{}{
		"op": "subscribe",
		"args": []map[string]string{
			{
				"instType": "USDT-FUTURES",
				"channel":  "positions",
				"instId":   "default",
			},
		},
	}

	b.wsPrivateManager.AddSubscription(subMsg)
	return b.wsPrivateManager.Send(subMsg)
}

func (b *Bitget) authenticateWebSocket(conn *websocket.Conn) error {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	message := timestamp + "GET" + "/user/verify"
	h := hmac.New(sha256.New, []byte(b.secretKey))
	h.Write([]byte(message))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	authMsg := map[string]interface{}{
		"op": "login",
		"args": []map[string]string{
			{
				"apiKey":     b.apiKey,
				"passphrase": b.passphrase,
				"timestamp":  timestamp,
				"sign":       signature,
			},
		},
	}

	if err := conn.WriteJSON(authMsg); err != nil {
		return err
	}

	// Ждем подтверждения
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	var resp struct {
		Event string `json:"event"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(msg, &resp); err != nil {
		return err
	}

	if resp.Event != "login" || resp.Code != "0" {
		return fmt.Errorf("authentication failed")
	}

	return nil
}

// handlePrivateMessage обрабатывает одно сообщение из приватного WebSocket
func (b *Bitget) handlePrivateMessage(message []byte) {
	var msg struct {
		Arg struct {
			Channel string `json:"channel"`
		} `json:"arg"`
		Data []struct {
			InstId       string `json:"instId"`
			HoldSide     string `json:"holdSide"`
			Total        string `json:"total"`
			OpenPriceAvg string `json:"openPriceAvg"`
			MarkPrice    string `json:"markPrice"`
			Leverage     string `json:"leverage"`
			UnrealizedPL string `json:"unrealizedPL"`
			UTime        string `json:"uTime"`
		} `json:"data"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	if msg.Arg.Channel == "positions" {
		b.callbackMu.RLock()
		callback := b.positionCallback
		b.callbackMu.RUnlock()

		if callback != nil {
			for _, p := range msg.Data {
				size, _ := strconv.ParseFloat(p.Total, 64)
				entryPrice, _ := strconv.ParseFloat(p.OpenPriceAvg, 64)
				markPrice, _ := strconv.ParseFloat(p.MarkPrice, 64)
				leverage, _ := strconv.Atoi(p.Leverage)
				unrealizedPnl, _ := strconv.ParseFloat(p.UnrealizedPL, 64)
				uTime, _ := strconv.ParseInt(p.UTime, 10, 64)

				side := SideLong
				if p.HoldSide == "short" {
					side = SideShort
				}

				callback(&Position{
					Symbol:        p.InstId,
					Side:          side,
					Size:          size,
					EntryPrice:    entryPrice,
					MarkPrice:     markPrice,
					Leverage:      leverage,
					UnrealizedPnl: unrealizedPnl,
					Liquidation:   false,
					UpdatedAt:     time.UnixMilli(uTime),
				})
			}
		}
	}
}

func (b *Bitget) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	// Bitget стандартная комиссия тейкера 0.04%
	return 0.0004, nil
}

func (b *Bitget) GetLimits(ctx context.Context, symbol string) (*Limits, error) {
	params := map[string]string{
		"productType": bitgetProductType,
		"symbol":      symbol,
	}

	body, err := b.doRequest(ctx, http.MethodGet, "/api/v2/mix/market/contracts", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			Symbol        string `json:"symbol"`
			MinTradeNum   string `json:"minTradeNum"`
			MaxPositionNum string `json:"maxPositionNum"`
			SizeMultiplier string `json:"sizeMultiplier"`
			PricePlace    string `json:"pricePlace"`
			VolPlace      string `json:"volPlace"`
			MaxLever      string `json:"maxLever"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("contract info not found for %s", symbol)
	}

	info := resp.Data[0]
	minOrderQty, _ := strconv.ParseFloat(info.MinTradeNum, 64)
	maxOrderQty, _ := strconv.ParseFloat(info.MaxPositionNum, 64)
	sizeMultiplier, _ := strconv.ParseFloat(info.SizeMultiplier, 64)
	pricePlace, _ := strconv.Atoi(info.PricePlace)
	volPlace, _ := strconv.Atoi(info.VolPlace)
	maxLeverage, _ := strconv.Atoi(info.MaxLever)

	qtyStep := 1.0
	for i := 0; i < volPlace; i++ {
		qtyStep /= 10
	}

	priceStep := 1.0
	for i := 0; i < pricePlace; i++ {
		priceStep /= 10
	}

	return &Limits{
		Symbol:      symbol,
		MinOrderQty: minOrderQty * sizeMultiplier,
		MaxOrderQty: maxOrderQty * sizeMultiplier,
		QtyStep:     qtyStep,
		MinNotional: 5.0,
		PriceStep:   priceStep,
		MaxLeverage: maxLeverage,
	}, nil
}

func (b *Bitget) Close() error {
	select {
	case <-b.closeChan:
	default:
		close(b.closeChan)
	}

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
