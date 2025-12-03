package service

import (
	"context"
	"time"

	"arbitrage/internal/models"
	"arbitrage/internal/repository"
)

// ============ Mock BlacklistRepository ============

type MockBlacklistRepository struct {
	entries     map[string]*models.BlacklistEntry
	createErr   error
	getErr      error
	deleteErr   error
	existsErr   error
	updateErr   error
	searchErr   error
	nextID      int
}

func NewMockBlacklistRepository() *MockBlacklistRepository {
	return &MockBlacklistRepository{
		entries: make(map[string]*models.BlacklistEntry),
		nextID:  1,
	}
}

func (m *MockBlacklistRepository) Create(entry *models.BlacklistEntry) error {
	if m.createErr != nil {
		return m.createErr
	}
	if _, exists := m.entries[entry.Symbol]; exists {
		return repository.ErrBlacklistEntryExists
	}
	entry.ID = m.nextID
	m.nextID++
	entry.CreatedAt = time.Now()
	m.entries[entry.Symbol] = entry
	return nil
}

func (m *MockBlacklistRepository) GetAll() ([]*models.BlacklistEntry, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.BlacklistEntry, 0, len(m.entries))
	for _, e := range m.entries {
		result = append(result, e)
	}
	return result, nil
}

func (m *MockBlacklistRepository) GetBySymbol(symbol string) (*models.BlacklistEntry, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if entry, exists := m.entries[symbol]; exists {
		return entry, nil
	}
	return nil, repository.ErrBlacklistEntryNotFound
}

func (m *MockBlacklistRepository) Delete(symbol string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, exists := m.entries[symbol]; !exists {
		return repository.ErrBlacklistEntryNotFound
	}
	delete(m.entries, symbol)
	return nil
}

func (m *MockBlacklistRepository) Exists(symbol string) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	_, exists := m.entries[symbol]
	return exists, nil
}

func (m *MockBlacklistRepository) UpdateReason(symbol, reason string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if entry, exists := m.entries[symbol]; exists {
		entry.Reason = reason
		return nil
	}
	return repository.ErrBlacklistEntryNotFound
}

func (m *MockBlacklistRepository) Count() (int, error) {
	if m.getErr != nil {
		return 0, m.getErr
	}
	return len(m.entries), nil
}

func (m *MockBlacklistRepository) DeleteAll() error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.entries = make(map[string]*models.BlacklistEntry)
	return nil
}

func (m *MockBlacklistRepository) Search(query string) ([]*models.BlacklistEntry, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	var result []*models.BlacklistEntry
	for symbol, entry := range m.entries {
		if containsIgnoreCase(symbol, query) {
			result = append(result, entry)
		}
	}
	return result, nil
}

// ============ Mock SettingsRepository ============

type MockSettingsRepository struct {
	settings  *models.Settings
	getErr    error
	updateErr error
}

func NewMockSettingsRepository() *MockSettingsRepository {
	return &MockSettingsRepository{
		settings: &models.Settings{
			ID:                  1,
			ConsiderFunding:     false,
			MaxConcurrentTrades: nil,
			NotificationPrefs: models.NotificationPreferences{
				Open:          true,
				Close:         true,
				StopLoss:      true,
				Liquidation:   true,
				APIError:      true,
				Margin:        true,
				Pause:         true,
				SecondLegFail: true,
			},
			UpdatedAt: time.Now(),
		},
	}
}

func (m *MockSettingsRepository) Get() (*models.Settings, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.settings, nil
}

func (m *MockSettingsRepository) Update(settings *models.Settings) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.settings = settings
	m.settings.UpdatedAt = time.Now()
	return nil
}

func (m *MockSettingsRepository) UpdateNotificationPrefs(prefs models.NotificationPreferences) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.settings.NotificationPrefs = prefs
	m.settings.UpdatedAt = time.Now()
	return nil
}

func (m *MockSettingsRepository) UpdateConsiderFunding(consider bool) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.settings.ConsiderFunding = consider
	m.settings.UpdatedAt = time.Now()
	return nil
}

func (m *MockSettingsRepository) UpdateMaxConcurrentTrades(maxTrades *int) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.settings.MaxConcurrentTrades = maxTrades
	m.settings.UpdatedAt = time.Now()
	return nil
}

func (m *MockSettingsRepository) GetNotificationPrefs() (*models.NotificationPreferences, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return &m.settings.NotificationPrefs, nil
}

func (m *MockSettingsRepository) GetMaxConcurrentTrades() (*int, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.settings.MaxConcurrentTrades, nil
}

func (m *MockSettingsRepository) ResetToDefaults() error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.settings = &models.Settings{
		ID:              1,
		ConsiderFunding: false,
		NotificationPrefs: models.NotificationPreferences{
			Open:          true,
			Close:         true,
			StopLoss:      true,
			Liquidation:   true,
			APIError:      true,
			Margin:        true,
			Pause:         true,
			SecondLegFail: true,
		},
		UpdatedAt: time.Now(),
	}
	return nil
}

// ============ Mock NotificationRepository ============

type MockNotificationRepository struct {
	notifications []*models.Notification
	createErr     error
	getErr        error
	deleteErr     error
	nextID        int
}

func NewMockNotificationRepository() *MockNotificationRepository {
	return &MockNotificationRepository{
		notifications: make([]*models.Notification, 0),
		nextID:        1,
	}
}

func (m *MockNotificationRepository) Create(notif *models.Notification) error {
	if m.createErr != nil {
		return m.createErr
	}
	notif.ID = m.nextID
	m.nextID++
	notif.Timestamp = time.Now()
	m.notifications = append(m.notifications, notif)
	return nil
}

func (m *MockNotificationRepository) GetRecent(limit int) ([]*models.Notification, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if limit <= 0 || limit > len(m.notifications) {
		limit = len(m.notifications)
	}
	// Возвращаем последние limit записей
	start := len(m.notifications) - limit
	if start < 0 {
		start = 0
	}
	return m.notifications[start:], nil
}

func (m *MockNotificationRepository) GetByTypes(types []string, limit int) ([]*models.Notification, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	var result []*models.Notification
	typeSet := make(map[string]bool)
	for _, t := range types {
		typeSet[t] = true
	}
	for _, n := range m.notifications {
		if typeSet[n.Type] {
			result = append(result, n)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *MockNotificationRepository) DeleteAll() error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.notifications = make([]*models.Notification, 0)
	return nil
}

func (m *MockNotificationRepository) Count() (int, error) {
	if m.getErr != nil {
		return 0, m.getErr
	}
	return len(m.notifications), nil
}

func (m *MockNotificationRepository) CountByType(notifType string) (int, error) {
	if m.getErr != nil {
		return 0, m.getErr
	}
	count := 0
	for _, n := range m.notifications {
		if n.Type == notifType {
			count++
		}
	}
	return count, nil
}

func (m *MockNotificationRepository) KeepRecent(keepCount int) (int64, error) {
	if m.deleteErr != nil {
		return 0, m.deleteErr
	}
	if len(m.notifications) <= keepCount {
		return 0, nil
	}
	deleted := int64(len(m.notifications) - keepCount)
	m.notifications = m.notifications[len(m.notifications)-keepCount:]
	return deleted, nil
}

// ============ Mock StatsRepository ============

type MockStatsRepository struct {
	trades    []*repository.Trade
	stats     *models.Stats
	createErr error
	getErr    error
	deleteErr error
	nextID    int
}

func NewMockStatsRepository() *MockStatsRepository {
	return &MockStatsRepository{
		trades: make([]*repository.Trade, 0),
		stats: &models.Stats{
			TotalTrades: 0,
			TotalPnl:    0,
			TodayTrades: 0,
			TodayPnl:    0,
			WeekTrades:  0,
			WeekPnl:     0,
			MonthTrades: 0,
			MonthPnl:    0,
		},
		nextID: 1,
	}
}

func (m *MockStatsRepository) GetStats() (*models.Stats, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	// Пересчитываем статистику из trades
	m.recalculateStats()
	return m.stats, nil
}

func (m *MockStatsRepository) recalculateStats() {
	m.stats.TotalTrades = len(m.trades)
	m.stats.TotalPnl = 0
	for _, t := range m.trades {
		m.stats.TotalPnl += t.PNL
	}
	// Упрощенная логика для тестов
	m.stats.TodayTrades = len(m.trades)
	m.stats.TodayPnl = m.stats.TotalPnl
	m.stats.WeekTrades = len(m.trades)
	m.stats.WeekPnl = m.stats.TotalPnl
	m.stats.MonthTrades = len(m.trades)
	m.stats.MonthPnl = m.stats.TotalPnl
}

func (m *MockStatsRepository) RecordTrade(
	pairID int,
	symbol string,
	exchanges [2]string,
	entryTime, exitTime time.Time,
	pnl float64,
	wasStopLoss, wasLiquidation bool,
) error {
	if m.createErr != nil {
		return m.createErr
	}
	trade := &repository.Trade{
		ID:             m.nextID,
		PairID:         pairID,
		Symbol:         symbol,
		Exchanges:      exchanges[0] + "," + exchanges[1],
		EntryTime:      entryTime,
		ExitTime:       exitTime,
		PNL:            pnl,
		WasStopLoss:    wasStopLoss,
		WasLiquidation: wasLiquidation,
		CreatedAt:      time.Now(),
	}
	m.nextID++
	m.trades = append(m.trades, trade)
	return nil
}

func (m *MockStatsRepository) GetTopPairsByTrades(limit int) ([]models.PairStat, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	// Упрощенная реализация для тестов
	counts := make(map[string]int)
	for _, t := range m.trades {
		counts[t.Symbol]++
	}
	var result []models.PairStat
	for symbol, count := range counts {
		result = append(result, models.PairStat{Symbol: symbol, Value: float64(count)})
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *MockStatsRepository) GetTopPairsByProfit(limit int) ([]models.PairStat, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	pnls := make(map[string]float64)
	for _, t := range m.trades {
		pnls[t.Symbol] += t.PNL
	}
	var result []models.PairStat
	for symbol, pnl := range pnls {
		if pnl > 0 {
			result = append(result, models.PairStat{Symbol: symbol, Value: pnl})
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *MockStatsRepository) GetTopPairsByLoss(limit int) ([]models.PairStat, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	pnls := make(map[string]float64)
	for _, t := range m.trades {
		pnls[t.Symbol] += t.PNL
	}
	var result []models.PairStat
	for symbol, pnl := range pnls {
		if pnl < 0 {
			result = append(result, models.PairStat{Symbol: symbol, Value: pnl})
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *MockStatsRepository) ResetCounters() error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.trades = make([]*repository.Trade, 0)
	m.stats = &models.Stats{}
	return nil
}

func (m *MockStatsRepository) GetTradesByPairID(pairID int, limit int) ([]*repository.Trade, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	var result []*repository.Trade
	for _, t := range m.trades {
		if t.PairID == pairID {
			result = append(result, t)
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *MockStatsRepository) GetTradesInTimeRange(from, to time.Time, limit int) ([]*repository.Trade, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	var result []*repository.Trade
	for _, t := range m.trades {
		if t.ExitTime.After(from) && t.ExitTime.Before(to) {
			result = append(result, t)
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *MockStatsRepository) Count() (int, error) {
	if m.getErr != nil {
		return 0, m.getErr
	}
	return len(m.trades), nil
}

func (m *MockStatsRepository) GetPNLBySymbol(symbol string) (float64, error) {
	if m.getErr != nil {
		return 0, m.getErr
	}
	var pnl float64
	for _, t := range m.trades {
		if t.Symbol == symbol {
			pnl += t.PNL
		}
	}
	return pnl, nil
}

func (m *MockStatsRepository) DeleteOlderThan(olderThan time.Time) (int64, error) {
	if m.deleteErr != nil {
		return 0, m.deleteErr
	}
	var newTrades []*repository.Trade
	var deleted int64
	for _, t := range m.trades {
		if t.ExitTime.After(olderThan) {
			newTrades = append(newTrades, t)
		} else {
			deleted++
		}
	}
	m.trades = newTrades
	return deleted, nil
}

// ============ Mock PairRepository ============

type MockPairRepository struct {
	pairs       map[int]*models.PairConfig
	createErr   error
	getErr      error
	updateErr   error
	deleteErr   error
	nextID      int
}

func NewMockPairRepository() *MockPairRepository {
	return &MockPairRepository{
		pairs:  make(map[int]*models.PairConfig),
		nextID: 1,
	}
}

func (m *MockPairRepository) Create(pair *models.PairConfig) error {
	if m.createErr != nil {
		return m.createErr
	}
	for _, p := range m.pairs {
		if p.Symbol == pair.Symbol {
			return repository.ErrPairExists
		}
	}
	pair.ID = m.nextID
	m.nextID++
	pair.CreatedAt = time.Now()
	pair.UpdatedAt = time.Now()
	m.pairs[pair.ID] = pair
	return nil
}

func (m *MockPairRepository) GetByID(id int) (*models.PairConfig, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if pair, exists := m.pairs[id]; exists {
		return pair, nil
	}
	return nil, repository.ErrPairNotFound
}

func (m *MockPairRepository) GetAll() ([]*models.PairConfig, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.PairConfig, 0, len(m.pairs))
	for _, p := range m.pairs {
		result = append(result, p)
	}
	return result, nil
}

func (m *MockPairRepository) GetActive() ([]*models.PairConfig, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	var result []*models.PairConfig
	for _, p := range m.pairs {
		if p.Status == models.PairStatusActive {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *MockPairRepository) Update(pair *models.PairConfig) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, exists := m.pairs[pair.ID]; !exists {
		return repository.ErrPairNotFound
	}
	pair.UpdatedAt = time.Now()
	m.pairs[pair.ID] = pair
	return nil
}

func (m *MockPairRepository) Delete(id int) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, exists := m.pairs[id]; !exists {
		return repository.ErrPairNotFound
	}
	delete(m.pairs, id)
	return nil
}

func (m *MockPairRepository) UpdateStatus(id int, status string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if pair, exists := m.pairs[id]; exists {
		pair.Status = status
		pair.UpdatedAt = time.Now()
		return nil
	}
	return repository.ErrPairNotFound
}

func (m *MockPairRepository) UpdateParams(id int, entrySpread, exitSpread, volume float64, nOrders int, stopLoss float64) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if pair, exists := m.pairs[id]; exists {
		pair.EntrySpreadPct = entrySpread
		pair.ExitSpreadPct = exitSpread
		pair.VolumeAsset = volume
		pair.NOrders = nOrders
		pair.StopLoss = stopLoss
		pair.UpdatedAt = time.Now()
		return nil
	}
	return repository.ErrPairNotFound
}

func (m *MockPairRepository) Count() (int, error) {
	if m.getErr != nil {
		return 0, m.getErr
	}
	return len(m.pairs), nil
}

func (m *MockPairRepository) CountActive() (int, error) {
	if m.getErr != nil {
		return 0, m.getErr
	}
	count := 0
	for _, p := range m.pairs {
		if p.Status == models.PairStatusActive {
			count++
		}
	}
	return count, nil
}

func (m *MockPairRepository) ExistsBySymbol(symbol string) (bool, error) {
	if m.getErr != nil {
		return false, m.getErr
	}
	for _, p := range m.pairs {
		if p.Symbol == symbol {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockPairRepository) IncrementTrades(id int) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if pair, exists := m.pairs[id]; exists {
		pair.TradesCount++
		return nil
	}
	return repository.ErrPairNotFound
}

func (m *MockPairRepository) UpdatePnl(id int, pnl float64) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if pair, exists := m.pairs[id]; exists {
		pair.TotalPnl += pnl
		return nil
	}
	return repository.ErrPairNotFound
}

func (m *MockPairRepository) Search(query string) ([]*models.PairConfig, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	var result []*models.PairConfig
	for _, p := range m.pairs {
		if containsIgnoreCase(p.Symbol, query) {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *MockPairRepository) ResetStats(id int) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if pair, exists := m.pairs[id]; exists {
		pair.TradesCount = 0
		pair.TotalPnl = 0
		return nil
	}
	return repository.ErrPairNotFound
}

// ============ Mock WebSocket Broadcaster ============

type MockWebSocketBroadcaster struct {
	notifications []*models.Notification
}

func NewMockWebSocketBroadcaster() *MockWebSocketBroadcaster {
	return &MockWebSocketBroadcaster{
		notifications: make([]*models.Notification, 0),
	}
}

func (m *MockWebSocketBroadcaster) BroadcastNotification(notif *models.Notification) {
	m.notifications = append(m.notifications, notif)
}

// ============ Mock Stats Broadcaster ============

type MockStatsBroadcaster struct {
	updates []*models.Stats
}

func NewMockStatsBroadcaster() *MockStatsBroadcaster {
	return &MockStatsBroadcaster{
		updates: make([]*models.Stats, 0),
	}
}

func (m *MockStatsBroadcaster) BroadcastStatsUpdate(stats *models.Stats) {
	m.updates = append(m.updates, stats)
}

// ============ Helper functions ============

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && contains(toLower(s), toLower(substr))))
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============ Mock ExchangeRepository ============

type MockExchangeRepository struct {
	accounts    map[string]*models.ExchangeAccount
	createErr   error
	getErr      error
	updateErr   error
	deleteErr   error
	nextID      int
}

func NewMockExchangeRepository() *MockExchangeRepository {
	return &MockExchangeRepository{
		accounts: make(map[string]*models.ExchangeAccount),
		nextID:   1,
	}
}

func (m *MockExchangeRepository) Create(account *models.ExchangeAccount) error {
	if m.createErr != nil {
		return m.createErr
	}
	if _, exists := m.accounts[account.Name]; exists {
		return repository.ErrExchangeExists
	}
	account.ID = m.nextID
	m.nextID++
	account.CreatedAt = time.Now()
	account.UpdatedAt = time.Now()
	m.accounts[account.Name] = account
	return nil
}

func (m *MockExchangeRepository) GetByName(name string) (*models.ExchangeAccount, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if account, exists := m.accounts[name]; exists {
		return account, nil
	}
	return nil, repository.ErrExchangeNotFound
}

func (m *MockExchangeRepository) GetByID(id int) (*models.ExchangeAccount, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, account := range m.accounts {
		if account.ID == id {
			return account, nil
		}
	}
	return nil, repository.ErrExchangeNotFound
}

func (m *MockExchangeRepository) GetAll() ([]*models.ExchangeAccount, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	result := make([]*models.ExchangeAccount, 0, len(m.accounts))
	for _, a := range m.accounts {
		result = append(result, a)
	}
	return result, nil
}

func (m *MockExchangeRepository) GetConnected() ([]*models.ExchangeAccount, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	var result []*models.ExchangeAccount
	for _, a := range m.accounts {
		if a.Connected {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *MockExchangeRepository) Update(account *models.ExchangeAccount) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, exists := m.accounts[account.Name]; !exists {
		return repository.ErrExchangeNotFound
	}
	account.UpdatedAt = time.Now()
	m.accounts[account.Name] = account
	return nil
}

func (m *MockExchangeRepository) Delete(id int) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for name, account := range m.accounts {
		if account.ID == id {
			delete(m.accounts, name)
			return nil
		}
	}
	return repository.ErrExchangeNotFound
}

func (m *MockExchangeRepository) UpdateBalance(id int, balance float64) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for _, account := range m.accounts {
		if account.ID == id {
			account.Balance = balance
			account.UpdatedAt = time.Now()
			return nil
		}
	}
	return repository.ErrExchangeNotFound
}

func (m *MockExchangeRepository) SetLastError(id int, errMsg string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	for _, account := range m.accounts {
		if account.ID == id {
			account.LastError = errMsg
			account.UpdatedAt = time.Now()
			return nil
		}
	}
	return repository.ErrExchangeNotFound
}

func (m *MockExchangeRepository) CountConnected() (int, error) {
	if m.getErr != nil {
		return 0, m.getErr
	}
	count := 0
	for _, a := range m.accounts {
		if a.Connected {
			count++
		}
	}
	return count, nil
}

// ============ Mock BotEngine ============

type MockBotEngine struct {
	pairs          map[int]*models.PairConfig
	runtimes       map[int]*models.PairRuntime
	openPositions  map[int]bool
	startErr       error
	pauseErr       error
	forceCloseErr  error
}

func NewMockBotEngine() *MockBotEngine {
	return &MockBotEngine{
		pairs:         make(map[int]*models.PairConfig),
		runtimes:      make(map[int]*models.PairRuntime),
		openPositions: make(map[int]bool),
	}
}

func (m *MockBotEngine) AddPair(cfg *models.PairConfig) {
	m.pairs[cfg.ID] = cfg
	m.runtimes[cfg.ID] = &models.PairRuntime{
		PairID: cfg.ID,
		State:  models.StatePaused,
	}
}

func (m *MockBotEngine) RemovePair(pairID int) {
	delete(m.pairs, pairID)
	delete(m.runtimes, pairID)
	delete(m.openPositions, pairID)
}

func (m *MockBotEngine) StartPair(pairID int) error {
	if m.startErr != nil {
		return m.startErr
	}
	if runtime, exists := m.runtimes[pairID]; exists {
		runtime.State = models.StateReady
	}
	return nil
}

func (m *MockBotEngine) PausePair(pairID int) error {
	if m.pauseErr != nil {
		return m.pauseErr
	}
	if runtime, exists := m.runtimes[pairID]; exists {
		runtime.State = models.StatePaused
	}
	return nil
}

func (m *MockBotEngine) GetPairRuntime(pairID int) *models.PairRuntime {
	return m.runtimes[pairID]
}

func (m *MockBotEngine) ForceClosePair(ctx context.Context, pairID int) error {
	if m.forceCloseErr != nil {
		return m.forceCloseErr
	}
	m.openPositions[pairID] = false
	if runtime, exists := m.runtimes[pairID]; exists {
		runtime.State = models.StatePaused
		runtime.Legs = nil
	}
	return nil
}

func (m *MockBotEngine) UpdatePairConfig(pairID int, cfg *models.PairConfig) {
	m.pairs[pairID] = cfg
}

func (m *MockBotEngine) HasOpenPosition(pairID int) bool {
	return m.openPositions[pairID]
}

// SetOpenPosition устанавливает флаг открытой позиции для тестов
func (m *MockBotEngine) SetOpenPosition(pairID int, hasPosition bool) {
	m.openPositions[pairID] = hasPosition
	if runtime, exists := m.runtimes[pairID]; exists {
		if hasPosition {
			runtime.State = models.StateHolding
		}
	}
}
