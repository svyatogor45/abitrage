package handlers

import (
	"errors"
	"sync"
	"time"

	"arbitrage/internal/models"
	"arbitrage/internal/service"
)

// ============ Mock Blacklist Service ============

// MockBlacklistService мок для BlacklistServiceInterface
type MockBlacklistService struct {
	entries   map[string]*models.BlacklistEntry
	addErr    error
	getErr    error
	removeErr error
	searchErr error
	nextID    int
	mu        sync.RWMutex
}

// NewMockBlacklistService создает новый мок сервиса черного списка
func NewMockBlacklistService() *MockBlacklistService {
	return &MockBlacklistService{
		entries: make(map[string]*models.BlacklistEntry),
		nextID:  1,
	}
}

func (m *MockBlacklistService) AddToBlacklist(symbol, reason string) (*models.BlacklistEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.addErr != nil {
		return nil, m.addErr
	}

	if _, exists := m.entries[symbol]; exists {
		return nil, service.ErrBlacklistSymbolExists
	}

	entry := &models.BlacklistEntry{
		ID:        m.nextID,
		Symbol:    symbol,
		Reason:    reason,
		CreatedAt: time.Now(),
	}
	m.nextID++
	m.entries[symbol] = entry
	return entry, nil
}

func (m *MockBlacklistService) GetBlacklist() ([]*models.BlacklistEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return nil, m.getErr
	}

	result := make([]*models.BlacklistEntry, 0, len(m.entries))
	for _, e := range m.entries {
		result = append(result, e)
	}
	return result, nil
}

func (m *MockBlacklistService) RemoveFromBlacklist(symbol string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.removeErr != nil {
		return m.removeErr
	}

	if _, exists := m.entries[symbol]; !exists {
		return service.ErrBlacklistEntryNotFound
	}

	delete(m.entries, symbol)
	return nil
}

func (m *MockBlacklistService) GetBySymbol(symbol string) (*models.BlacklistEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return nil, m.getErr
	}

	if entry, exists := m.entries[symbol]; exists {
		return entry, nil
	}
	return nil, service.ErrBlacklistEntryNotFound
}

func (m *MockBlacklistService) IsBlacklisted(symbol string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return false, m.getErr
	}

	_, exists := m.entries[symbol]
	return exists, nil
}

func (m *MockBlacklistService) UpdateReason(symbol, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, exists := m.entries[symbol]; exists {
		entry.Reason = reason
		return nil
	}
	return service.ErrBlacklistEntryNotFound
}

func (m *MockBlacklistService) Search(query string) ([]*models.BlacklistEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.searchErr != nil {
		return nil, m.searchErr
	}

	result := make([]*models.BlacklistEntry, 0, len(m.entries))
	for _, e := range m.entries {
		result = append(result, e)
	}
	return result, nil
}

func (m *MockBlacklistService) GetCount() (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return 0, m.getErr
	}
	return len(m.entries), nil
}

func (m *MockBlacklistService) ClearAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries = make(map[string]*models.BlacklistEntry)
	return nil
}

// SetError устанавливает ошибку для указанной операции
func (m *MockBlacklistService) SetError(operation string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch operation {
	case "add":
		m.addErr = err
	case "get":
		m.getErr = err
	case "remove":
		m.removeErr = err
	case "search":
		m.searchErr = err
	}
}

// AddEntry добавляет запись напрямую (для настройки тестов)
func (m *MockBlacklistService) AddEntry(symbol, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries[symbol] = &models.BlacklistEntry{
		ID:        m.nextID,
		Symbol:    symbol,
		Reason:    reason,
		CreatedAt: time.Now(),
	}
	m.nextID++
}

// ============ Mock Settings Service ============

// MockSettingsService мок для SettingsServiceInterface
type MockSettingsService struct {
	settings  *models.Settings
	getErr    error
	updateErr error
	mu        sync.RWMutex
}

// NewMockSettingsService создает новый мок сервиса настроек
func NewMockSettingsService() *MockSettingsService {
	return &MockSettingsService{
		settings: &models.Settings{
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
		},
	}
}

func (m *MockSettingsService) GetSettings() (*models.Settings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.settings, nil
}

func (m *MockSettingsService) UpdateSettings(req *service.UpdateSettingsRequest) (*models.Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.updateErr != nil {
		return nil, m.updateErr
	}

	if req.ConsiderFunding != nil {
		m.settings.ConsiderFunding = *req.ConsiderFunding
	}
	if req.MaxConcurrentTrades != nil {
		m.settings.MaxConcurrentTrades = req.MaxConcurrentTrades
	}
	if req.ClearMaxConcurrentTrades {
		m.settings.MaxConcurrentTrades = nil
	}
	if req.NotificationPrefs != nil {
		m.settings.NotificationPrefs = *req.NotificationPrefs
	}
	m.settings.UpdatedAt = time.Now()

	return m.settings, nil
}

func (m *MockSettingsService) GetNotificationPrefs() (*models.NotificationPreferences, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return nil, m.getErr
	}
	return &m.settings.NotificationPrefs, nil
}

func (m *MockSettingsService) GetMaxConcurrentTrades() (*int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.settings.MaxConcurrentTrades, nil
}

func (m *MockSettingsService) ResetToDefaults() error {
	m.mu.Lock()
	defer m.mu.Unlock()

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

// SetError устанавливает ошибку для указанной операции
func (m *MockSettingsService) SetError(operation string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch operation {
	case "get":
		m.getErr = err
	case "update":
		m.updateErr = err
	}
}

// ============ Mock Notification Service ============

// MockNotificationService мок для NotificationServiceInterface
type MockNotificationService struct {
	notifications []*models.Notification
	createErr     error
	getErr        error
	clearErr      error
	nextID        int
	mu            sync.RWMutex
}

// NewMockNotificationService создает новый мок сервиса уведомлений
func NewMockNotificationService() *MockNotificationService {
	return &MockNotificationService{
		notifications: make([]*models.Notification, 0),
		nextID:        1,
	}
}

func (m *MockNotificationService) GetNotifications(types []string, limit int) ([]*models.Notification, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return nil, m.getErr
	}

	result := make([]*models.Notification, 0, len(m.notifications))

	if len(types) == 0 {
		result = append(result, m.notifications...)
	} else {
		typeSet := make(map[string]bool)
		for _, t := range types {
			typeSet[t] = true
		}
		for _, n := range m.notifications {
			if typeSet[n.Type] {
				result = append(result, n)
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

func (m *MockNotificationService) ClearNotifications() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clearErr != nil {
		return m.clearErr
	}

	m.notifications = make([]*models.Notification, 0)
	return nil
}

func (m *MockNotificationService) CreateNotification(notif *models.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.createErr != nil {
		return m.createErr
	}

	notif.ID = m.nextID
	m.nextID++
	notif.Timestamp = time.Now()
	m.notifications = append(m.notifications, notif)
	return nil
}

func (m *MockNotificationService) GetNotificationCount() (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return 0, m.getErr
	}
	return len(m.notifications), nil
}

// SetError устанавливает ошибку для указанной операции
func (m *MockNotificationService) SetError(operation string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch operation {
	case "create":
		m.createErr = err
	case "get":
		m.getErr = err
	case "clear":
		m.clearErr = err
	}
}

// AddNotification добавляет уведомление напрямую (для настройки тестов)
func (m *MockNotificationService) AddNotification(notifType, severity, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.notifications = append(m.notifications, &models.Notification{
		ID:        m.nextID,
		Type:      notifType,
		Severity:  severity,
		Message:   message,
		Timestamp: time.Now(),
	})
	m.nextID++
}

// ============ Mock Stats Service ============

// MockStatsService мок для StatsServiceInterface
type MockStatsService struct {
	stats      *models.Stats
	topPairs   map[string][]models.PairStat
	getErr     error
	topPairErr error
	resetErr   error
	mu         sync.RWMutex
}

// NewMockStatsService создает новый мок сервиса статистики
func NewMockStatsService() *MockStatsService {
	return &MockStatsService{
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
		topPairs: make(map[string][]models.PairStat),
	}
}

func (m *MockStatsService) GetStats() (*models.Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.stats, nil
}

func (m *MockStatsService) GetTopPairs(metric string, limit int) ([]models.PairStat, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.topPairErr != nil {
		return nil, m.topPairErr
	}

	pairs, exists := m.topPairs[metric]
	if !exists {
		return []models.PairStat{}, nil
	}

	if limit > 0 && len(pairs) > limit {
		return pairs[:limit], nil
	}
	return pairs, nil
}

func (m *MockStatsService) ResetStats() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.resetErr != nil {
		return m.resetErr
	}

	m.stats = &models.Stats{}
	m.topPairs = make(map[string][]models.PairStat)
	return nil
}

// SetError устанавливает ошибку для указанной операции
func (m *MockStatsService) SetError(operation string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch operation {
	case "get":
		m.getErr = err
	case "topPairs":
		m.topPairErr = err
	case "reset":
		m.resetErr = err
	}
}

// SetStats устанавливает статистику напрямую (для настройки тестов)
func (m *MockStatsService) SetStats(stats *models.Stats) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats = stats
}

// SetTopPairs устанавливает топ пары для метрики
func (m *MockStatsService) SetTopPairs(metric string, pairs []models.PairStat) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.topPairs[metric] = pairs
}

// ============ Helper errors for tests ============

var (
	ErrMockDatabase = errors.New("mock database error")
	ErrMockService  = errors.New("mock service error")
)

// ============ Проверяем, что моки реализуют интерфейсы ============

var _ service.BlacklistServiceInterface = (*MockBlacklistService)(nil)
var _ service.SettingsServiceInterface = (*MockSettingsService)(nil)
var _ service.NotificationServiceInterface = (*MockNotificationService)(nil)
var _ service.StatsServiceInterface = (*MockStatsService)(nil)
