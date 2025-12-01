package service

import (
	"errors"
	"strings"

	"arbitrage/internal/models"
	"arbitrage/internal/repository"
)

// Ошибки сервиса черного списка
var (
	ErrBlacklistSymbolEmpty   = errors.New("symbol cannot be empty")
	ErrBlacklistSymbolExists  = errors.New("symbol already in blacklist")
	ErrBlacklistEntryNotFound = errors.New("blacklist entry not found")
)

// BlacklistService предоставляет бизнес-логику для управления черным списком.
//
// Черный список носит ИНФОРМАТИВНЫЙ характер - это заметки пользователя
// о нежелательных парах. Бот НЕ фильтрует автоматически на основе этого списка.
//
// Отвечает за:
// - Добавление пар в черный список с причиной
// - Получение списка заблокированных пар
// - Удаление пар из черного списка
// - Поиск по символу
type BlacklistService struct {
	blacklistRepo *repository.BlacklistRepository
}

// NewBlacklistService создает новый экземпляр BlacklistService.
func NewBlacklistService(blacklistRepo *repository.BlacklistRepository) *BlacklistService {
	return &BlacklistService{
		blacklistRepo: blacklistRepo,
	}
}

// AddToBlacklist добавляет пару в черный список.
//
// Параметры:
// - symbol: торговый символ (например, "BTCUSDT")
// - reason: причина добавления (опционально, пользовательская заметка)
//
// Символ автоматически приводится к верхнему регистру.
//
// Возвращает:
// - *models.BlacklistEntry: созданная запись
// - error: ErrBlacklistSymbolEmpty если символ пустой,
//          ErrBlacklistSymbolExists если символ уже в списке
func (s *BlacklistService) AddToBlacklist(symbol, reason string) (*models.BlacklistEntry, error) {
	// Валидация символа
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, ErrBlacklistSymbolEmpty
	}

	// Нормализуем символ
	symbol = strings.ToUpper(symbol)

	// Проверяем, не существует ли уже
	exists, err := s.blacklistRepo.Exists(symbol)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrBlacklistSymbolExists
	}

	// Создаем запись
	entry := &models.BlacklistEntry{
		Symbol: symbol,
		Reason: strings.TrimSpace(reason),
	}

	if err := s.blacklistRepo.Create(entry); err != nil {
		// Дополнительная проверка на unique violation (race condition)
		if errors.Is(err, repository.ErrBlacklistEntryExists) {
			return nil, ErrBlacklistSymbolExists
		}
		return nil, err
	}

	return entry, nil
}

// GetBlacklist возвращает весь черный список.
//
// Записи отсортированы по дате добавления (новые сверху).
func (s *BlacklistService) GetBlacklist() ([]*models.BlacklistEntry, error) {
	entries, err := s.blacklistRepo.GetAll()
	if err != nil {
		return nil, err
	}

	// Гарантируем возврат пустого массива вместо nil
	if entries == nil {
		entries = []*models.BlacklistEntry{}
	}

	return entries, nil
}

// RemoveFromBlacklist удаляет пару из черного списка по символу.
//
// Символ автоматически приводится к верхнему регистру.
//
// Возвращает:
// - error: ErrBlacklistEntryNotFound если символ не найден
func (s *BlacklistService) RemoveFromBlacklist(symbol string) error {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ErrBlacklistSymbolEmpty
	}

	err := s.blacklistRepo.Delete(symbol)
	if err != nil {
		if errors.Is(err, repository.ErrBlacklistEntryNotFound) {
			return ErrBlacklistEntryNotFound
		}
		return err
	}

	return nil
}

// GetBySymbol возвращает запись черного списка по символу.
//
// Символ автоматически приводится к верхнему регистру.
func (s *BlacklistService) GetBySymbol(symbol string) (*models.BlacklistEntry, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, ErrBlacklistSymbolEmpty
	}

	entry, err := s.blacklistRepo.GetBySymbol(symbol)
	if err != nil {
		if errors.Is(err, repository.ErrBlacklistEntryNotFound) {
			return nil, ErrBlacklistEntryNotFound
		}
		return nil, err
	}

	return entry, nil
}

// IsBlacklisted проверяет, находится ли символ в черном списке.
//
// Этот метод может использоваться для информирования пользователя,
// но НЕ должен использоваться для автоматической фильтрации ботом.
func (s *BlacklistService) IsBlacklisted(symbol string) (bool, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return false, ErrBlacklistSymbolEmpty
	}

	return s.blacklistRepo.Exists(symbol)
}

// UpdateReason обновляет причину добавления в черный список.
func (s *BlacklistService) UpdateReason(symbol, reason string) error {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ErrBlacklistSymbolEmpty
	}

	err := s.blacklistRepo.UpdateReason(symbol, strings.TrimSpace(reason))
	if err != nil {
		if errors.Is(err, repository.ErrBlacklistEntryNotFound) {
			return ErrBlacklistEntryNotFound
		}
		return err
	}

	return nil
}

// Search ищет записи по части символа.
//
// Поиск регистронезависимый.
func (s *BlacklistService) Search(query string) ([]*models.BlacklistEntry, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return s.GetBlacklist()
	}

	entries, err := s.blacklistRepo.Search(query)
	if err != nil {
		return nil, err
	}

	if entries == nil {
		entries = []*models.BlacklistEntry{}
	}

	return entries, nil
}

// GetCount возвращает количество записей в черном списке.
func (s *BlacklistService) GetCount() (int, error) {
	return s.blacklistRepo.Count()
}

// ClearAll очищает весь черный список.
//
// Используйте с осторожностью - удаляет все записи без возможности восстановления.
func (s *BlacklistService) ClearAll() error {
	return s.blacklistRepo.DeleteAll()
}
