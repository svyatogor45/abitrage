package service

import (
	"time"

	"arbitrage/internal/models"
	"arbitrage/internal/repository"
)

// BlacklistRepositoryInterface определяет интерфейс репозитория черного списка
type BlacklistRepositoryInterface interface {
	Create(entry *models.BlacklistEntry) error
	GetAll() ([]*models.BlacklistEntry, error)
	GetBySymbol(symbol string) (*models.BlacklistEntry, error)
	Delete(symbol string) error
	Exists(symbol string) (bool, error)
	UpdateReason(symbol, reason string) error
	Count() (int, error)
	DeleteAll() error
	Search(query string) ([]*models.BlacklistEntry, error)
}

// SettingsRepositoryInterface определяет интерфейс репозитория настроек
type SettingsRepositoryInterface interface {
	Get() (*models.Settings, error)
	Update(settings *models.Settings) error
	UpdateNotificationPrefs(prefs models.NotificationPreferences) error
	UpdateConsiderFunding(consider bool) error
	UpdateMaxConcurrentTrades(maxTrades *int) error
	GetNotificationPrefs() (*models.NotificationPreferences, error)
	GetMaxConcurrentTrades() (*int, error)
	ResetToDefaults() error
}

// NotificationRepositoryInterface определяет интерфейс репозитория уведомлений
type NotificationRepositoryInterface interface {
	Create(notif *models.Notification) error
	GetRecent(limit int) ([]*models.Notification, error)
	GetByTypes(types []string, limit int) ([]*models.Notification, error)
	DeleteAll() error
	Count() (int, error)
	CountByType(notifType string) (int, error)
	KeepRecent(keepCount int) (int64, error)
}

// StatsRepositoryInterface определяет интерфейс репозитория статистики
type StatsRepositoryInterface interface {
	GetStats() (*models.Stats, error)
	RecordTrade(pairID int, symbol string, exchanges [2]string, entryTime, exitTime time.Time, pnl float64, wasStopLoss, wasLiquidation bool) error
	GetTopPairsByTrades(limit int) ([]models.PairStat, error)
	GetTopPairsByProfit(limit int) ([]models.PairStat, error)
	GetTopPairsByLoss(limit int) ([]models.PairStat, error)
	ResetCounters() error
	GetTradesByPairID(pairID int, limit int) ([]*repository.Trade, error)
	GetTradesInTimeRange(from, to time.Time, limit int) ([]*repository.Trade, error)
	Count() (int, error)
	GetPNLBySymbol(symbol string) (float64, error)
	DeleteOlderThan(olderThan time.Time) (int64, error)
}

// PairRepositoryInterface определяет интерфейс репозитория пар
type PairRepositoryInterface interface {
	Create(pair *models.PairConfig) error
	GetByID(id int) (*models.PairConfig, error)
	GetAll() ([]*models.PairConfig, error)
	GetActive() ([]*models.PairConfig, error)
	Update(pair *models.PairConfig) error
	Delete(id int) error
	UpdateStatus(id int, status string) error
	UpdateParams(id int, entrySpread, exitSpread, volume float64, nOrders int, stopLoss float64) error
	Count() (int, error)
	CountActive() (int, error)
	ExistsBySymbol(symbol string) (bool, error)
	IncrementTrades(id int) error
	UpdatePnl(id int, pnl float64) error
	Search(query string) ([]*models.PairConfig, error)
	ResetStats(id int) error
}

// ExchangeRepositoryInterface определяет интерфейс репозитория бирж
type ExchangeRepositoryInterface interface {
	Create(account *models.ExchangeAccount) error
	GetByName(name string) (*models.ExchangeAccount, error)
	GetByID(id int) (*models.ExchangeAccount, error)
	GetAll() ([]*models.ExchangeAccount, error)
	GetConnected() ([]*models.ExchangeAccount, error)
	Update(account *models.ExchangeAccount) error
	Delete(id int) error
	UpdateBalance(id int, balance float64) error
	SetLastError(id int, errMsg string) error
	CountConnected() (int, error)
}

// Проверяем, что реальные репозитории реализуют интерфейсы
var _ BlacklistRepositoryInterface = (*repository.BlacklistRepository)(nil)
var _ SettingsRepositoryInterface = (*repository.SettingsRepository)(nil)
var _ NotificationRepositoryInterface = (*repository.NotificationRepository)(nil)
var _ StatsRepositoryInterface = (*repository.StatsRepository)(nil)
var _ PairRepositoryInterface = (*repository.PairRepository)(nil)
var _ ExchangeRepositoryInterface = (*repository.ExchangeRepository)(nil)
