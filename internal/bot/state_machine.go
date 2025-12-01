package bot

import "arbitrage/internal/models"

// ValidTransitions определяет допустимые переходы между состояниями
var ValidTransitions = map[string][]string{
	models.StatePaused:   {models.StateReady},
	models.StateReady:    {models.StatePaused, models.StateEntering},
	models.StateEntering: {models.StateHolding, models.StateReady, models.StateError}, // Ready при откате
	models.StateHolding:  {models.StateExiting, models.StateError},                    // Error при ликвидации
	models.StateExiting:  {models.StateReady, models.StatePaused, models.StateError},
	models.StateError:    {models.StatePaused}, // Только ручной сброс
}

// CanTransition проверяет допустимость перехода
func CanTransition(from, to string) bool {
	allowed, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
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
