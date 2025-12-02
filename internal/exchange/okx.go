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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	okxBaseURL    = "https://www.okx.com"
	okxWSPublic   = "wss://ws.okx.com:8443/ws/v5/public"
	okxWSPrivate  = "wss://ws.okx.com:8443/ws/v5/private"
)

type OKX struct {
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

// NewOKX создаёт новый экземпляр OKX
// Использует глобальный HTTP клиент с connection pooling и оптимизированными таймаутами
func NewOKX() *OKX {
	return &OKX{
		httpClient:      GetGlobalHTTPClient().GetClient(),
		tickerCallbacks: make(map[string]func(*Ticker)),
		closeChan:       make(chan struct{}),
	}
}

// sign создает подпись для OKX API
func (o *OKX) sign(timestamp, method, requestPath, body string) string {
	message := timestamp + method + requestPath + body
	h := hmac.New(sha256.New, []byte(o.secretKey))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (o *OKX) doRequest(ctx context.Context, method, endpoint string, params map[string]string, signed bool) ([]byte, error) {
	var reqBody string
	var reqURL string

	if method == http.MethodGet {
		reqURL = okxBaseURL + endpoint
		if len(params) > 0 {
			query := make([]string, 0, len(params))
			for k, v := range params {
				query = append(query, k+"="+v)
			}
			reqURL += "?" + strings.Join(query, "&")
		}
	} else {
		reqURL = okxBaseURL + endpoint
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
		timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		var signPath string
		if method == http.MethodGet && len(params) > 0 {
			query := make([]string, 0, len(params))
			for k, v := range params {
				query = append(query, k+"="+v)
			}
			signPath = endpoint + "?" + strings.Join(query, "&")
		} else {
			signPath = endpoint
		}
		signature := o.sign(timestamp, method, signPath, reqBody)

		req.Header.Set("OK-ACCESS-KEY", o.apiKey)
		req.Header.Set("OK-ACCESS-SIGN", signature)
		req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
		req.Header.Set("OK-ACCESS-PASSPHRASE", o.passphrase)
	}

	resp, err := o.httpClient.Do(req)
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

	if baseResp.Code != "0" {
		return nil, &ExchangeError{
			Exchange: "okx",
			Code:     baseResp.Code,
			Message:  baseResp.Msg,
		}
	}

	return body, nil
}

func (o *OKX) Connect(apiKey, secret, passphrase string) error {
	o.apiKey = apiKey
	o.secretKey = secret
	o.passphrase = passphrase

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := o.GetBalance(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to OKX: %w", err)
	}

	o.connected = true
	return nil
}

func (o *OKX) GetName() string {
	return "okx"
}

func (o *OKX) GetBalance(ctx context.Context) (float64, error) {
	params := map[string]string{
		"ccy": "USDT",
	}

	body, err := o.doRequest(ctx, http.MethodGet, "/api/v5/account/balance", params, true)
	if err != nil {
		return 0, err
	}

	var resp struct {
		Data []struct {
			Details []struct {
				Ccy   string `json:"ccy"`
				Eq    string `json:"eq"`
			} `json:"details"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}

	if len(resp.Data) > 0 {
		for _, detail := range resp.Data[0].Details {
			if detail.Ccy == "USDT" {
				equity, _ := strconv.ParseFloat(detail.Eq, 64)
				return equity, nil
			}
		}
	}

	return 0, nil
}

func (o *OKX) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	// OKX использует формат BTC-USDT-SWAP для свопов
	instId := o.toOKXSymbol(symbol)

	params := map[string]string{
		"instId": instId,
	}

	body, err := o.doRequest(ctx, http.MethodGet, "/api/v5/market/ticker", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			InstId  string `json:"instId"`
			BidPx   string `json:"bidPx"`
			AskPx   string `json:"askPx"`
			Last    string `json:"last"`
			Ts      string `json:"ts"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("ticker not found for %s", symbol)
	}

	t := resp.Data[0]
	bidPrice, _ := strconv.ParseFloat(t.BidPx, 64)
	askPrice, _ := strconv.ParseFloat(t.AskPx, 64)
	lastPrice, _ := strconv.ParseFloat(t.Last, 64)
	ts, _ := strconv.ParseInt(t.Ts, 10, 64)

	return &Ticker{
		Symbol:    symbol,
		BidPrice:  bidPrice,
		AskPrice:  askPrice,
		LastPrice: lastPrice,
		Timestamp: time.UnixMilli(ts),
	}, nil
}

func (o *OKX) GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error) {
	if depth > 400 {
		depth = 400
	}

	instId := o.toOKXSymbol(symbol)

	params := map[string]string{
		"instId": instId,
		"sz":     strconv.Itoa(depth),
	}

	body, err := o.doRequest(ctx, http.MethodGet, "/api/v5/market/books", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			Bids [][]string `json:"bids"`
			Asks [][]string `json:"asks"`
			Ts   string     `json:"ts"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("orderbook not found for %s", symbol)
	}

	data := resp.Data[0]
	ts, _ := strconv.ParseInt(data.Ts, 10, 64)

	orderBook := &OrderBook{
		Symbol:    symbol,
		Bids:      make([]PriceLevel, len(data.Bids)),
		Asks:      make([]PriceLevel, len(data.Asks)),
		Timestamp: time.UnixMilli(ts),
	}

	for i, bid := range data.Bids {
		price, _ := strconv.ParseFloat(bid[0], 64)
		volume, _ := strconv.ParseFloat(bid[1], 64)
		orderBook.Bids[i] = PriceLevel{Price: price, Volume: volume}
	}

	for i, ask := range data.Asks {
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

func (o *OKX) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error) {
	instId := o.toOKXSymbol(symbol)

	okxSide := "buy"
	posSide := "long"
	if side == SideSell || side == SideShort {
		okxSide = "sell"
		posSide = "short"
	}

	params := map[string]string{
		"instId":  instId,
		"tdMode":  "cross",
		"side":    okxSide,
		"posSide": posSide,
		"ordType": "market",
		"sz":      strconv.FormatFloat(qty, 'f', -1, 64),
	}

	body, err := o.doRequest(ctx, http.MethodPost, "/api/v5/trade/order", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			OrdId string `json:"ordId"`
			SCode string `json:"sCode"`
			SMsg  string `json:"sMsg"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 || resp.Data[0].SCode != "0" {
		msg := "unknown error"
		if len(resp.Data) > 0 {
			msg = resp.Data[0].SMsg
		}
		return nil, fmt.Errorf("order failed: %s", msg)
	}

	order := &Order{
		ID:        resp.Data[0].OrdId,
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
	execInfo, err := o.getOrderDetail(ctx, instId, resp.Data[0].OrdId)
	if err == nil && execInfo != nil {
		order.AvgFillPrice = execInfo.AvgPrice
		order.FilledQty = execInfo.FilledQty
	}

	return order, nil
}

func (o *OKX) getOrderDetail(ctx context.Context, instId, orderId string) (*struct {
	FilledQty float64
	AvgPrice  float64
}, error) {
	params := map[string]string{
		"instId": instId,
		"ordId":  orderId,
	}

	body, err := o.doRequest(ctx, http.MethodGet, "/api/v5/trade/order", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			AccFillSz string `json:"accFillSz"`
			AvgPx     string `json:"avgPx"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("order not found")
	}

	filledQty, _ := strconv.ParseFloat(resp.Data[0].AccFillSz, 64)
	avgPrice, _ := strconv.ParseFloat(resp.Data[0].AvgPx, 64)

	return &struct {
		FilledQty float64
		AvgPrice  float64
	}{
		FilledQty: filledQty,
		AvgPrice:  avgPrice,
	}, nil
}

func (o *OKX) GetOpenPositions(ctx context.Context) ([]*Position, error) {
	params := map[string]string{
		"instType": "SWAP",
	}

	body, err := o.doRequest(ctx, http.MethodGet, "/api/v5/account/positions", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			InstId     string `json:"instId"`
			PosSide    string `json:"posSide"`
			Pos        string `json:"pos"`
			AvgPx      string `json:"avgPx"`
			MarkPx     string `json:"markPx"`
			Lever      string `json:"lever"`
			Upl        string `json:"upl"`
			LiqPx      string `json:"liqPx"`
			UTime      string `json:"uTime"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	positions := make([]*Position, 0)
	for _, p := range resp.Data {
		pos, _ := strconv.ParseFloat(p.Pos, 64)
		if pos == 0 {
			continue
		}

		entryPrice, _ := strconv.ParseFloat(p.AvgPx, 64)
		markPrice, _ := strconv.ParseFloat(p.MarkPx, 64)
		leverage, _ := strconv.Atoi(p.Lever)
		unrealizedPnl, _ := strconv.ParseFloat(p.Upl, 64)
		uTime, _ := strconv.ParseInt(p.UTime, 10, 64)

		side := SideLong
		if p.PosSide == "short" {
			side = SideShort
			pos = -pos // Делаем положительным
		}

		positions = append(positions, &Position{
			Symbol:        o.fromOKXSymbol(p.InstId),
			Side:          side,
			Size:          pos,
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

func (o *OKX) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	instId := o.toOKXSymbol(symbol)

	closeSide := "sell"
	posSide := "long"
	if side == SideShort {
		closeSide = "buy"
		posSide = "short"
	}

	params := map[string]string{
		"instId":  instId,
		"tdMode":  "cross",
		"side":    closeSide,
		"posSide": posSide,
		"ordType": "market",
		"sz":      strconv.FormatFloat(qty, 'f', -1, 64),
	}

	_, err := o.doRequest(ctx, http.MethodPost, "/api/v5/trade/order", params, true)
	return err
}

func (o *OKX) SubscribeTicker(symbol string, callback func(*Ticker)) error {
	o.callbackMu.Lock()
	o.tickerCallbacks[symbol] = callback
	o.callbackMu.Unlock()

	if o.wsPublicManager == nil {
		config := DefaultWSReconnectConfig()
		o.wsPublicManager = NewWSReconnectManager("okx-public", okxWSPublic, config)

		o.wsPublicManager.SetOnMessage(o.handlePublicMessage)
		o.wsPublicManager.SetOnConnect(func() {
			log.Printf("[okx] Public WebSocket connected")
		})
		o.wsPublicManager.SetOnDisconnect(func(err error) {
			if err != nil {
				log.Printf("[okx] Public WebSocket disconnected: %v", err)
			}
		})

		if err := o.wsPublicManager.Connect(); err != nil {
			return fmt.Errorf("failed to connect to WebSocket: %w", err)
		}
	}

	instId := o.toOKXSymbol(symbol)
	subMsg := map[string]interface{}{
		"op": "subscribe",
		"args": []map[string]string{
			{
				"channel": "tickers",
				"instId":  instId,
			},
		},
	}

	o.wsPublicManager.AddSubscription(subMsg)
	return o.wsPublicManager.Send(subMsg)
}

// handlePublicMessage обрабатывает одно сообщение из публичного WebSocket
func (o *OKX) handlePublicMessage(message []byte) {
	var msg struct {
		Arg struct {
			Channel string `json:"channel"`
			InstId  string `json:"instId"`
		} `json:"arg"`
		Data []struct {
			BidPx string `json:"bidPx"`
			AskPx string `json:"askPx"`
			Last  string `json:"last"`
			Ts    string `json:"ts"`
		} `json:"data"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	if msg.Arg.Channel == "tickers" && len(msg.Data) > 0 {
		symbol := o.fromOKXSymbol(msg.Arg.InstId)

		o.callbackMu.RLock()
		callback, ok := o.tickerCallbacks[symbol]
		o.callbackMu.RUnlock()

		if ok && callback != nil {
			d := msg.Data[0]
			bidPrice, _ := strconv.ParseFloat(d.BidPx, 64)
			askPrice, _ := strconv.ParseFloat(d.AskPx, 64)
			lastPrice, _ := strconv.ParseFloat(d.Last, 64)
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

func (o *OKX) SubscribePositions(callback func(*Position)) error {
	o.callbackMu.Lock()
	o.positionCallback = callback
	o.callbackMu.Unlock()

	if o.wsPrivateManager == nil {
		config := DefaultWSReconnectConfig()
		o.wsPrivateManager = NewWSReconnectManager("okx-private", okxWSPrivate, config)

		o.wsPrivateManager.SetAuthFunc(o.authenticateWebSocket)
		o.wsPrivateManager.SetOnMessage(o.handlePrivateMessage)
		o.wsPrivateManager.SetOnConnect(func() {
			log.Printf("[okx] Private WebSocket connected")
		})
		o.wsPrivateManager.SetOnDisconnect(func(err error) {
			if err != nil {
				log.Printf("[okx] Private WebSocket disconnected: %v", err)
			}
		})

		if err := o.wsPrivateManager.Connect(); err != nil {
			return fmt.Errorf("failed to connect to private WebSocket: %w", err)
		}
	}

	subMsg := map[string]interface{}{
		"op": "subscribe",
		"args": []map[string]string{
			{
				"channel":  "positions",
				"instType": "SWAP",
			},
		},
	}

	o.wsPrivateManager.AddSubscription(subMsg)
	return o.wsPrivateManager.Send(subMsg)
}

func (o *OKX) authenticateWebSocket(conn *websocket.Conn) error {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	message := timestamp + "GET" + "/users/self/verify"
	h := hmac.New(sha256.New, []byte(o.secretKey))
	h.Write([]byte(message))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	authMsg := map[string]interface{}{
		"op": "login",
		"args": []map[string]string{
			{
				"apiKey":     o.apiKey,
				"passphrase": o.passphrase,
				"timestamp":  timestamp,
				"sign":       signature,
			},
		},
	}

	if err := conn.WriteJSON(authMsg); err != nil {
		return err
	}

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
func (o *OKX) handlePrivateMessage(message []byte) {
	var msg struct {
		Arg struct {
			Channel string `json:"channel"`
		} `json:"arg"`
		Data []struct {
			InstId  string `json:"instId"`
			PosSide string `json:"posSide"`
			Pos     string `json:"pos"`
			AvgPx   string `json:"avgPx"`
			MarkPx  string `json:"markPx"`
			Lever   string `json:"lever"`
			Upl     string `json:"upl"`
			UTime   string `json:"uTime"`
		} `json:"data"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	if msg.Arg.Channel == "positions" {
		o.callbackMu.RLock()
		callback := o.positionCallback
		o.callbackMu.RUnlock()

		if callback != nil {
			for _, p := range msg.Data {
				pos, _ := strconv.ParseFloat(p.Pos, 64)
				entryPrice, _ := strconv.ParseFloat(p.AvgPx, 64)
				markPrice, _ := strconv.ParseFloat(p.MarkPx, 64)
				leverage, _ := strconv.Atoi(p.Lever)
				unrealizedPnl, _ := strconv.ParseFloat(p.Upl, 64)
				uTime, _ := strconv.ParseInt(p.UTime, 10, 64)

				side := SideLong
				if p.PosSide == "short" {
					side = SideShort
					if pos < 0 {
						pos = -pos
					}
				}

				callback(&Position{
					Symbol:        o.fromOKXSymbol(p.InstId),
					Side:          side,
					Size:          pos,
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

func (o *OKX) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	// OKX стандартная комиссия тейкера 0.05%
	return 0.0005, nil
}

func (o *OKX) GetLimits(ctx context.Context, symbol string) (*Limits, error) {
	instId := o.toOKXSymbol(symbol)

	params := map[string]string{
		"instType": "SWAP",
		"instId":   instId,
	}

	body, err := o.doRequest(ctx, http.MethodGet, "/api/v5/public/instruments", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			InstId   string `json:"instId"`
			MinSz    string `json:"minSz"`
			MaxLmtSz string `json:"maxLmtSz"`
			LotSz    string `json:"lotSz"`
			TickSz   string `json:"tickSz"`
			Lever    string `json:"lever"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("instrument info not found for %s", symbol)
	}

	info := resp.Data[0]
	minOrderQty, _ := strconv.ParseFloat(info.MinSz, 64)
	maxOrderQty, _ := strconv.ParseFloat(info.MaxLmtSz, 64)
	qtyStep, _ := strconv.ParseFloat(info.LotSz, 64)
	priceStep, _ := strconv.ParseFloat(info.TickSz, 64)
	maxLeverage, _ := strconv.Atoi(info.Lever)

	return &Limits{
		Symbol:      symbol,
		MinOrderQty: minOrderQty,
		MaxOrderQty: maxOrderQty,
		QtyStep:     qtyStep,
		MinNotional: 5.0,
		PriceStep:   priceStep,
		MaxLeverage: maxLeverage,
	}, nil
}

func (o *OKX) Close() error {
	select {
	case <-o.closeChan:
	default:
		close(o.closeChan)
	}

	if o.wsPublicManager != nil {
		o.wsPublicManager.Close()
		o.wsPublicManager = nil
	}

	if o.wsPrivateManager != nil {
		o.wsPrivateManager.Close()
		o.wsPrivateManager = nil
	}

	o.connected = false
	return nil
}

// toOKXSymbol конвертирует символ в формат OKX (BTCUSDT -> BTC-USDT-SWAP)
func (o *OKX) toOKXSymbol(symbol string) string {
	// Убираем USDT и добавляем формат OKX
	base := strings.TrimSuffix(symbol, "USDT")
	return base + "-USDT-SWAP"
}

// fromOKXSymbol конвертирует формат OKX обратно (BTC-USDT-SWAP -> BTCUSDT)
func (o *OKX) fromOKXSymbol(instId string) string {
	// BTC-USDT-SWAP -> BTCUSDT
	parts := strings.Split(instId, "-")
	if len(parts) >= 2 {
		return parts[0] + parts[1]
	}
	return instId
}
