package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

	wsConn   *websocket.Conn
	wsMu     sync.RWMutex

	tickerCallbacks  map[string]func(*Ticker)
	positionCallback func(*Position)
	callbackMu       sync.RWMutex

	connected bool
	closeChan chan struct{}
}

func NewGate() *Gate {
	return &Gate{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
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

	total, _ := strconv.ParseFloat(resp.Total, 64)
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
	bidPrice, _ := strconv.ParseFloat(t.HighestBid, 64)
	askPrice, _ := strconv.ParseFloat(t.LowestAsk, 64)
	lastPrice, _ := strconv.ParseFloat(t.Last, 64)

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
		price, _ := strconv.ParseFloat(bid.P, 64)
		orderBook.Bids[i] = PriceLevel{Price: price, Volume: float64(bid.S)}
	}

	for i, ask := range resp.Asks {
		price, _ := strconv.ParseFloat(ask.P, 64)
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

	fillPrice, _ := strconv.ParseFloat(resp.FillPrice, 64)
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

		entryPrice, _ := strconv.ParseFloat(p.EntryPrice, 64)
		markPrice, _ := strconv.ParseFloat(p.MarkPrice, 64)
		unrealizedPnl, _ := strconv.ParseFloat(p.UnrealisedPnl, 64)

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

	g.wsMu.Lock()
	defer g.wsMu.Unlock()

	if g.wsConn == nil {
		conn, _, err := websocket.DefaultDialer.Dial(gateWSURL, nil)
		if err != nil {
			return fmt.Errorf("failed to connect to WebSocket: %w", err)
		}
		g.wsConn = conn
		go g.handleMessages()
	}

	contract := g.toGateSymbol(symbol)
	subMsg := map[string]interface{}{
		"time":    time.Now().Unix(),
		"channel": "futures.tickers",
		"event":   "subscribe",
		"payload": []string{contract},
	}

	return g.wsConn.WriteJSON(subMsg)
}

func (g *Gate) handleMessages() {
	for {
		select {
		case <-g.closeChan:
			return
		default:
		}

		g.wsMu.RLock()
		conn := g.wsConn
		g.wsMu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		var msg struct {
			Channel string `json:"channel"`
			Event   string `json:"event"`
			Result  []struct {
				Contract   string `json:"contract"`
				Last       string `json:"last"`
				LowestAsk  string `json:"lowest_ask"`
				HighestBid string `json:"highest_bid"`
			} `json:"result"`
		}

		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		if msg.Channel == "futures.tickers" && msg.Event == "update" {
			for _, t := range msg.Result {
				symbol := g.fromGateSymbol(t.Contract)

				g.callbackMu.RLock()
				callback, ok := g.tickerCallbacks[symbol]
				g.callbackMu.RUnlock()

				if ok && callback != nil {
					bidPrice, _ := strconv.ParseFloat(t.HighestBid, 64)
					askPrice, _ := strconv.ParseFloat(t.LowestAsk, 64)
					lastPrice, _ := strconv.ParseFloat(t.Last, 64)

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
	}
}

func (g *Gate) SubscribePositions(callback func(*Position)) error {
	g.callbackMu.Lock()
	g.positionCallback = callback
	g.callbackMu.Unlock()

	g.wsMu.Lock()
	defer g.wsMu.Unlock()

	if g.wsConn == nil {
		conn, _, err := websocket.DefaultDialer.Dial(gateWSURL, nil)
		if err != nil {
			return fmt.Errorf("failed to connect to WebSocket: %w", err)
		}
		g.wsConn = conn

		// Authenticate
		if err := g.authenticateWebSocket(conn); err != nil {
			conn.Close()
			g.wsConn = nil
			return err
		}

		go g.handleMessages()
	}

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

	return g.wsConn.WriteJSON(subMsg)
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

	quantoMultiplier, _ := strconv.ParseFloat(resp.QuantoMultiplier, 64)

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
	close(g.closeChan)

	g.wsMu.Lock()
	defer g.wsMu.Unlock()

	if g.wsConn != nil {
		g.wsConn.Close()
		g.wsConn = nil
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
