package bot

import (
	"arbitrage/internal/models"
	"fmt"
	"sync"
)

// ValidTransitions определяет допустимые переходы между состояниями (для совместимости и тестов)
var ValidTransitions = map[string][]string{
	models.StatePaused:   {models.StateReady},
	models.StateReady:    {models.StatePaused, models.StateEntering},
	models.StateEntering: {models.StateHolding, models.StateReady, models.StateError}, // Ready при откате
	models.StateHolding:  {models.StateExiting, models.StatePaused, models.StateError}, // PAUSED при SL/ликвидации
	models.StateExiting:  {models.StateReady, models.StatePaused, models.StateError},
	models.StateError:    {models.StatePaused}, // Только ручной сброс
}

// validTransitionsSet - O(1) lookup версия для hot path
// Инициализируется один раз при загрузке пакета
var validTransitionsSet = func() map[string]map[string]struct{} {
	result := make(map[string]map[string]struct{}, len(ValidTransitions))
	for from, toList := range ValidTransitions {
		result[from] = make(map[string]struct{}, len(toList))
		for _, to := range toList {
			result[from][to] = struct{}{}
		}
	}
	return result
}()

// CanTransition проверяет допустимость перехода - O(1) lookup
func CanTransition(from, to string) bool {
	if allowed, ok := validTransitionsSet[from]; ok {
		_, valid := allowed[to]
		return valid
	}
	return false
}

// StateTransitionError описывает ошибку недопустимого перехода
type StateTransitionError struct {
	PairID int
	From   string
	To     string
}

func (e *StateTransitionError) Error() string {
	return fmt.Sprintf("invalid state transition for pair %d: %s → %s", e.PairID, e.From, e.To)
}

// TryTransition атомарно проверяет и выполняет переход состояния
// Возвращает true если переход выполнен, false если невалиден
// ВАЖНО: вызывающий код должен держать lock на PairState.mu
func TryTransition(runtime *models.PairRuntime, pairID int, newState string) error {
	if !CanTransition(runtime.State, newState) {
		return &StateTransitionError{
			PairID: pairID,
			From:   runtime.State,
			To:     newState,
		}
	}
	runtime.State = newState
	return nil
}

// ForceTransition принудительно устанавливает состояние без проверки
// Используется ТОЛЬКО для аварийных ситуаций (ликвидации, критические ошибки)
// ВАЖНО: вызывающий код должен держать lock на PairState.mu
func ForceTransition(runtime *models.PairRuntime, newState string) {
	runtime.State = newState
}

// TransitionCounter отслеживает все переходы для метрик
var (
	transitionCounter     = make(map[string]uint64)
	transitionCounterLock sync.RWMutex
)

// RecordTransition записывает переход для метрик (вызывается автоматически в TryTransition)
func RecordTransition(from, to string) {
	key := from + "→" + to
	transitionCounterLock.Lock()
	transitionCounter[key]++
	transitionCounterLock.Unlock()
}

// GetTransitionStats возвращает статистику переходов (для отладки)
func GetTransitionStats() map[string]uint64 {
	transitionCounterLock.RLock()
	defer transitionCounterLock.RUnlock()
	result := make(map[string]uint64, len(transitionCounter))
	for k, v := range transitionCounter {
		result[k] = v
	}
	return result
}

// StateInfo возвращает описание состояния для UI
func StateInfo(s string) string {
	switch s {
	case models.StatePaused:
		return "Работа торговой пары приостановлена"
	case models.StateReady:
		return "Торговая пара запущена (ожидание условий)"
	case models.StateEntering:
		return "Открытие позиций..."
	case models.StateHolding:
		return "Позиция открыта"
	case models.StateExiting:
		return "Закрытие позиций..."
	case models.StateError:
		return "Ошибка! Требуется вмешательство"
	default:
		return "Неизвестное состояние"
	}
}

// IsActive возвращает true если пара активно торгуется
func IsActive(s string) bool {
	return s == models.StateReady || s == models.StateEntering || s == models.StateHolding || s == models.StateExiting
}

// HasOpenPosition возвращает true если есть открытая позиция
func HasOpenPosition(s string) bool {
	return s == models.StateHolding || s == models.StateExiting
}
