package bot

import (
	"arbitrage/internal/models"
	"arbitrage/pkg/utils"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// validTransitions определяет допустимые переходы между состояниями
// Не экспортируется для защиты от внешней модификации
var validTransitions = map[string][]string{
	models.StatePaused:   {models.StateReady},
	models.StateReady:    {models.StatePaused, models.StateEntering},
	models.StateEntering: {models.StateHolding, models.StateReady, models.StateError},  // Ready при откате
	models.StateHolding:  {models.StateExiting, models.StatePaused, models.StateError}, // PAUSED при SL/ликвидации
	models.StateExiting:  {models.StateReady, models.StatePaused, models.StateError},
	models.StateError:    {models.StatePaused}, // Только ручной сброс
}

// validTransitionsSet - O(1) lookup версия для hot path
// Инициализируется один раз при загрузке пакета
var validTransitionsSet = func() map[string]map[string]struct{} {
	result := make(map[string]map[string]struct{}, len(validTransitions))
	for from, toList := range validTransitions {
		result[from] = make(map[string]struct{}, len(toList))
		for _, to := range toList {
			result[from][to] = struct{}{}
		}
	}
	return result
}()

// GetValidTransitions возвращает копию допустимых переходов (для тестов и отладки)
func GetValidTransitions() map[string][]string {
	result := make(map[string][]string, len(validTransitions))
	for k, v := range validTransitions {
		result[k] = append([]string{}, v...) // deep copy слайса
	}
	return result
}

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
// Возвращает nil если переход выполнен, error если невалиден
// ВАЖНО: вызывающий код должен держать lock на PairState.mu
func TryTransition(runtime *models.PairRuntime, pairID int, newState string) error {
	if runtime == nil {
		return fmt.Errorf("runtime is nil for pair %d", pairID)
	}
	oldState := runtime.State
	if !CanTransition(oldState, newState) {
		return &StateTransitionError{
			PairID: pairID,
			From:   oldState,
			To:     newState,
		}
	}
	runtime.State = newState
	// Записываем метрику перехода и логируем смену состояния
	logTransition(pairID, oldState, newState, false)
	return nil
}

// ForceTransition принудительно устанавливает состояние без проверки
// Используется ТОЛЬКО для аварийных ситуаций (ликвидации, критические ошибки)
// ВАЖНО: вызывающий код должен держать lock на PairState.mu
func ForceTransition(runtime *models.PairRuntime, newState string) {
	runtime.State = newState
}

// ForceTransitionWithLog устанавливает состояние без проверки и логирует переход
// Используется для аварийных ситуаций (ликвидации, критические ошибки)
// ВАЖНО: вызывающий код должен держать lock на PairState.mu
func ForceTransitionWithLog(runtime *models.PairRuntime, pairID int, newState string) {
	oldState := runtime.State
	runtime.State = newState
	logTransition(pairID, oldState, newState, true)
}

// TransitionCounter отслеживает все переходы для метрик
var (
	transitionCounter     = make(map[string]uint64)
	transitionCounterLock sync.RWMutex
)

// RecordTransition записывает переход для метрик
// Вызывается автоматически в TryTransition, можно вызвать вручную для ForceTransition
func RecordTransition(from, to string) {
	recordTransitionCounts(from, to, false)
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

// HasOpenPosition возвращает true если есть открытая позиция или ордера в процессе исполнения
// Включает ENTERING т.к. ордера уже отправлены и есть exposure
// Консистентно с Engine.HasOpenPosition
func HasOpenPosition(s string) bool {
	return s == models.StateHolding || s == models.StateEntering || s == models.StateExiting
}

// HasFilledPosition возвращает true если позиция полностью исполнена
// Не включает ENTERING - только HOLDING (позиция открыта) и EXITING (закрывается)
func HasFilledPosition(s string) bool {
	return s == models.StateHolding || s == models.StateExiting
}

// logTransition записывает метрики/логи переходов централизованно
func logTransition(pairID int, from, to string, forced bool) {
	recordTransitionCounts(from, to, forced)

	if logger := utils.GetGlobalLogger(); logger != nil {
		logger.WithComponent("state_machine").With(
			zap.Int("pair_id", pairID),
			zap.String("from_state", from),
			zap.String("to_state", to),
			zap.Bool("forced", forced),
		).Info("state transition")
	}
}

// recordTransitionCounts инкрементирует локальные и Prometheus метрики
func recordTransitionCounts(from, to string, forced bool) {
	key := from + "→" + to
	transitionCounterLock.Lock()
	transitionCounter[key]++
	transitionCounterLock.Unlock()

	forcedLabel := "no"
	if forced {
		forcedLabel = "yes"
	}
	StateTransitions.WithLabelValues(from, to, forcedLabel).Inc()
}
