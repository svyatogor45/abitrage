package bot

import (
	"errors"
	"testing"

	"arbitrage/internal/models"
)

// TestCanTransition_ValidTransitions проверяет все валидные переходы между состояниями
func TestCanTransition_ValidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		// PAUSED → READY (start)
		{
			name: "PAUSED → READY (start pair)",
			from: models.StatePaused,
			to:   models.StateReady,
			want: true,
		},

		// READY → PAUSED (pause pair without position)
		{
			name: "READY → PAUSED (pause pair)",
			from: models.StateReady,
			to:   models.StatePaused,
			want: true,
		},
		// READY → ENTERING (entry conditions met)
		{
			name: "READY → ENTERING (entry conditions met)",
			from: models.StateReady,
			to:   models.StateEntering,
			want: true,
		},

		// ENTERING → HOLDING (both legs opened)
		{
			name: "ENTERING → HOLDING (both legs opened)",
			from: models.StateEntering,
			to:   models.StateHolding,
			want: true,
		},
		// ENTERING → READY (rollback on failure)
		{
			name: "ENTERING → READY (rollback on entry failure)",
			from: models.StateEntering,
			to:   models.StateReady,
			want: true,
		},
		// ENTERING → ERROR (critical error during entry)
		{
			name: "ENTERING → ERROR (critical error)",
			from: models.StateEntering,
			to:   models.StateError,
			want: true,
		},

		// HOLDING → EXITING (exit conditions met)
		{
			name: "HOLDING → EXITING (exit conditions met)",
			from: models.StateHolding,
			to:   models.StateExiting,
			want: true,
		},
		// HOLDING → ERROR (liquidation detected)
		{
			name: "HOLDING → ERROR (liquidation detected)",
			from: models.StateHolding,
			to:   models.StateError,
			want: true,
		},

		// EXITING → READY (successful close)
		{
			name: "EXITING → READY (successful close)",
			from: models.StateExiting,
			to:   models.StateReady,
			want: true,
		},
		// EXITING → PAUSED (close with SL/liquidation)
		{
			name: "EXITING → PAUSED (close with SL)",
			from: models.StateExiting,
			to:   models.StatePaused,
			want: true,
		},
		// EXITING → ERROR (error during close)
		{
			name: "EXITING → ERROR (close error)",
			from: models.StateExiting,
			to:   models.StateError,
			want: true,
		},

		// ERROR → PAUSED (manual reset)
		{
			name: "ERROR → PAUSED (manual reset)",
			from: models.StateError,
			to:   models.StatePaused,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// TestCanTransition_InvalidTransitions проверяет, что невалидные переходы отклоняются
func TestCanTransition_InvalidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
	}{
		// Из PAUSED можно только в READY
		{name: "PAUSED → ENTERING (invalid)", from: models.StatePaused, to: models.StateEntering},
		{name: "PAUSED → HOLDING (invalid)", from: models.StatePaused, to: models.StateHolding},
		{name: "PAUSED → EXITING (invalid)", from: models.StatePaused, to: models.StateExiting},
		{name: "PAUSED → ERROR (invalid)", from: models.StatePaused, to: models.StateError},
		{name: "PAUSED → PAUSED (invalid)", from: models.StatePaused, to: models.StatePaused},

		// Из READY нельзя напрямую в HOLDING/EXITING/ERROR
		{name: "READY → HOLDING (invalid, skip ENTERING)", from: models.StateReady, to: models.StateHolding},
		{name: "READY → EXITING (invalid)", from: models.StateReady, to: models.StateExiting},
		{name: "READY → ERROR (invalid)", from: models.StateReady, to: models.StateError},
		{name: "READY → READY (invalid)", from: models.StateReady, to: models.StateReady},

		// Из ENTERING нельзя напрямую в PAUSED/EXITING
		{name: "ENTERING → PAUSED (invalid)", from: models.StateEntering, to: models.StatePaused},
		{name: "ENTERING → EXITING (invalid, skip HOLDING)", from: models.StateEntering, to: models.StateExiting},
		{name: "ENTERING → ENTERING (invalid)", from: models.StateEntering, to: models.StateEntering},

		// Из HOLDING нельзя напрямую в READY/ENTERING (но можно в PAUSED при SL/ликвидации)
		// HOLDING → PAUSED разрешён для аварийных случаев (SL, ликвидация)
		{name: "HOLDING → READY (invalid, must go through EXITING)", from: models.StateHolding, to: models.StateReady},
		{name: "HOLDING → ENTERING (invalid)", from: models.StateHolding, to: models.StateEntering},
		{name: "HOLDING → HOLDING (invalid)", from: models.StateHolding, to: models.StateHolding},

		// Из EXITING нельзя напрямую в ENTERING/HOLDING
		{name: "EXITING → ENTERING (invalid)", from: models.StateExiting, to: models.StateEntering},
		{name: "EXITING → HOLDING (invalid)", from: models.StateExiting, to: models.StateHolding},
		{name: "EXITING → EXITING (invalid)", from: models.StateExiting, to: models.StateExiting},

		// Из ERROR можно только в PAUSED (ручной сброс)
		{name: "ERROR → READY (invalid)", from: models.StateError, to: models.StateReady},
		{name: "ERROR → ENTERING (invalid)", from: models.StateError, to: models.StateEntering},
		{name: "ERROR → HOLDING (invalid)", from: models.StateError, to: models.StateHolding},
		{name: "ERROR → EXITING (invalid)", from: models.StateError, to: models.StateExiting},
		{name: "ERROR → ERROR (invalid)", from: models.StateError, to: models.StateError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransition(tt.from, tt.to)
			if got != false {
				t.Errorf("CanTransition(%s, %s) = %v, want false (invalid transition)", tt.from, tt.to, got)
			}
		})
	}
}

// TestCanTransition_UnknownState проверяет поведение при неизвестном состоянии
func TestCanTransition_UnknownState(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
	}{
		{name: "unknown → READY", from: "UNKNOWN", to: models.StateReady},
		{name: "READY → unknown", from: models.StateReady, to: "UNKNOWN"},
		{name: "unknown → unknown", from: "UNKNOWN", to: "UNKNOWN2"},
		{name: "empty → READY", from: "", to: models.StateReady},
		{name: "READY → empty", from: models.StateReady, to: ""},
		{name: "lowercase paused → READY", from: "paused", to: models.StateReady},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransition(tt.from, tt.to)
			if got != false {
				t.Errorf("CanTransition(%s, %s) = %v, want false for unknown states", tt.from, tt.to, got)
			}
		})
	}
}

// TestStateInfo_AllStates проверяет, что все состояния имеют корректное описание
func TestStateInfo_AllStates(t *testing.T) {
	tests := []struct {
		state    string
		expected string
	}{
		{
			state:    models.StatePaused,
			expected: "Работа торговой пары приостановлена",
		},
		{
			state:    models.StateReady,
			expected: "Торговая пара запущена (ожидание условий)",
		},
		{
			state:    models.StateEntering,
			expected: "Открытие позиций...",
		},
		{
			state:    models.StateHolding,
			expected: "Позиция открыта",
		},
		{
			state:    models.StateExiting,
			expected: "Закрытие позиций...",
		},
		{
			state:    models.StateError,
			expected: "Ошибка! Требуется вмешательство",
		},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := StateInfo(tt.state)
			if got != tt.expected {
				t.Errorf("StateInfo(%s) = %q, want %q", tt.state, got, tt.expected)
			}
		})
	}
}

// TestStateInfo_UnknownState проверяет обработку неизвестного состояния
func TestStateInfo_UnknownState(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{name: "unknown state", state: "UNKNOWN"},
		{name: "empty state", state: ""},
		{name: "lowercase ready", state: "ready"},
		{name: "random string", state: "some_random_state"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StateInfo(tt.state)
			expected := "Неизвестное состояние"
			if got != expected {
				t.Errorf("StateInfo(%q) = %q, want %q", tt.state, got, expected)
			}
		})
	}
}

// TestIsActive проверяет определение активных состояний
func TestIsActive(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		// Активные состояния (пара торгуется)
		{state: models.StateReady, want: true},
		{state: models.StateEntering, want: true},
		{state: models.StateHolding, want: true},
		{state: models.StateExiting, want: true},

		// Неактивные состояния
		{state: models.StatePaused, want: false},
		{state: models.StateError, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := IsActive(tt.state)
			if got != tt.want {
				t.Errorf("IsActive(%s) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

// TestIsActive_UnknownState проверяет поведение при неизвестном состоянии
func TestIsActive_UnknownState(t *testing.T) {
	tests := []struct {
		state string
	}{
		{state: "UNKNOWN"},
		{state: ""},
		{state: "active"}, // lowercase
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := IsActive(tt.state)
			if got != false {
				t.Errorf("IsActive(%q) = %v, want false for unknown state", tt.state, got)
			}
		})
	}
}

// TestHasOpenPosition проверяет определение состояний с открытой позицией
func TestHasOpenPosition(t *testing.T) {
	tests := []struct {
		state string
		want  bool
	}{
		// Состояния с открытой позицией
		{state: models.StateHolding, want: true},
		{state: models.StateExiting, want: true},

		// Состояния без открытой позиции
		{state: models.StatePaused, want: false},
		{state: models.StateReady, want: false},
		{state: models.StateEntering, want: false}, // позиция ещё не полностью открыта
		{state: models.StateError, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := HasOpenPosition(tt.state)
			if got != tt.want {
				t.Errorf("HasOpenPosition(%s) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

// TestHasOpenPosition_UnknownState проверяет поведение при неизвестном состоянии
func TestHasOpenPosition_UnknownState(t *testing.T) {
	tests := []struct {
		state string
	}{
		{state: "UNKNOWN"},
		{state: ""},
		{state: "holding"}, // lowercase
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			got := HasOpenPosition(tt.state)
			if got != false {
				t.Errorf("HasOpenPosition(%q) = %v, want false for unknown state", tt.state, got)
			}
		})
	}
}

// TestValidTransitions_Completeness проверяет полноту таблицы переходов
func TestValidTransitions_Completeness(t *testing.T) {
	allStates := []string{
		models.StatePaused,
		models.StateReady,
		models.StateEntering,
		models.StateHolding,
		models.StateExiting,
		models.StateError,
	}

	// Проверяем, что все состояния есть в ValidTransitions
	for _, state := range allStates {
		if _, ok := ValidTransitions[state]; !ok {
			t.Errorf("State %s is not defined in ValidTransitions", state)
		}
	}

	// Проверяем, что нет лишних состояний в ValidTransitions
	for state := range ValidTransitions {
		found := false
		for _, s := range allStates {
			if s == state {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Unknown state %s in ValidTransitions", state)
		}
	}
}

// TestValidTransitions_NoSelfLoops проверяет отсутствие переходов в себя
func TestValidTransitions_NoSelfLoops(t *testing.T) {
	for from, tos := range ValidTransitions {
		for _, to := range tos {
			if from == to {
				t.Errorf("Self-loop detected: %s → %s", from, to)
			}
		}
	}
}

// TestValidTransitions_AllTargetsAreValid проверяет, что все целевые состояния валидны
func TestValidTransitions_AllTargetsAreValid(t *testing.T) {
	allStates := map[string]bool{
		models.StatePaused:   true,
		models.StateReady:    true,
		models.StateEntering: true,
		models.StateHolding:  true,
		models.StateExiting:  true,
		models.StateError:    true,
	}

	for from, tos := range ValidTransitions {
		for _, to := range tos {
			if !allStates[to] {
				t.Errorf("Invalid target state %s in transition from %s", to, from)
			}
		}
	}
}

// TestStateFlow_NormalArbitrageCycle проверяет полный цикл арбитража
func TestStateFlow_NormalArbitrageCycle(t *testing.T) {
	// Нормальный цикл: PAUSED → READY → ENTERING → HOLDING → EXITING → READY
	cycle := []string{
		models.StatePaused,
		models.StateReady,
		models.StateEntering,
		models.StateHolding,
		models.StateExiting,
		models.StateReady,
	}

	for i := 0; i < len(cycle)-1; i++ {
		from := cycle[i]
		to := cycle[i+1]
		if !CanTransition(from, to) {
			t.Errorf("Normal arbitrage cycle broken: cannot transition from %s to %s", from, to)
		}
	}
}

// TestStateFlow_StopLossCycle проверяет цикл с срабатыванием Stop Loss
func TestStateFlow_StopLossCycle(t *testing.T) {
	// Цикл со SL: PAUSED → READY → ENTERING → HOLDING → EXITING → PAUSED
	cycle := []string{
		models.StatePaused,
		models.StateReady,
		models.StateEntering,
		models.StateHolding,
		models.StateExiting,
		models.StatePaused, // SL приводит к паузе
	}

	for i := 0; i < len(cycle)-1; i++ {
		from := cycle[i]
		to := cycle[i+1]
		if !CanTransition(from, to) {
			t.Errorf("Stop Loss cycle broken: cannot transition from %s to %s", from, to)
		}
	}
}

// TestStateFlow_EntryRollback проверяет откат при неудачном входе
func TestStateFlow_EntryRollback(t *testing.T) {
	// Откат: PAUSED → READY → ENTERING → READY (вторая нога не открылась)
	cycle := []string{
		models.StatePaused,
		models.StateReady,
		models.StateEntering,
		models.StateReady, // откат
	}

	for i := 0; i < len(cycle)-1; i++ {
		from := cycle[i]
		to := cycle[i+1]
		if !CanTransition(from, to) {
			t.Errorf("Entry rollback cycle broken: cannot transition from %s to %s", from, to)
		}
	}
}

// TestStateFlow_ErrorRecovery проверяет восстановление после ошибки
func TestStateFlow_ErrorRecovery(t *testing.T) {
	// Восстановление: HOLDING → ERROR → PAUSED
	cycle := []string{
		models.StateHolding,
		models.StateError,
		models.StatePaused,
	}

	for i := 0; i < len(cycle)-1; i++ {
		from := cycle[i]
		to := cycle[i+1]
		if !CanTransition(from, to) {
			t.Errorf("Error recovery cycle broken: cannot transition from %s to %s", from, to)
		}
	}
}

// BenchmarkCanTransition измеряет производительность проверки переходов
func BenchmarkCanTransition(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CanTransition(models.StateReady, models.StateEntering)
	}
}

// BenchmarkStateInfo измеряет производительность получения описания
func BenchmarkStateInfo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		StateInfo(models.StateHolding)
	}
}

// BenchmarkIsActive измеряет производительность проверки активности
func BenchmarkIsActive(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IsActive(models.StateReady)
	}
}

// BenchmarkHasOpenPosition измеряет производительность проверки позиции
func BenchmarkHasOpenPosition(b *testing.B) {
	for i := 0; i < b.N; i++ {
		HasOpenPosition(models.StateHolding)
	}
}

// TestTryTransition проверяет атомарный переход состояния
func TestTryTransition(t *testing.T) {
	tests := []struct {
		name      string
		from      string
		to        string
		wantErr   bool
		wantState string
	}{
		{
			name:      "valid READY → ENTERING",
			from:      models.StateReady,
			to:        models.StateEntering,
			wantErr:   false,
			wantState: models.StateEntering,
		},
		{
			name:      "valid ENTERING → HOLDING",
			from:      models.StateEntering,
			to:        models.StateHolding,
			wantErr:   false,
			wantState: models.StateHolding,
		},
		{
			name:      "invalid PAUSED → HOLDING",
			from:      models.StatePaused,
			to:        models.StateHolding,
			wantErr:   true,
			wantState: models.StatePaused, // состояние не должно измениться
		},
		{
			name:      "invalid READY → EXITING",
			from:      models.StateReady,
			to:        models.StateExiting,
			wantErr:   true,
			wantState: models.StateReady, // состояние не должно измениться
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := &models.PairRuntime{State: tt.from}
			err := TryTransition(runtime, 1, tt.to)

			if (err != nil) != tt.wantErr {
				t.Errorf("TryTransition() error = %v, wantErr %v", err, tt.wantErr)
			}
			if runtime.State != tt.wantState {
				t.Errorf("TryTransition() state = %s, want %s", runtime.State, tt.wantState)
			}
			if tt.wantErr {
				var transErr *StateTransitionError
				if !errors.As(err, &transErr) {
					t.Errorf("TryTransition() error should be StateTransitionError, got %T", err)
				}
			}
		})
	}
}

// TestForceTransition проверяет принудительный переход
func TestForceTransition(t *testing.T) {
	runtime := &models.PairRuntime{State: models.StateReady}

	// ForceTransition должен работать даже для невалидных переходов
	ForceTransition(runtime, models.StateHolding) // READY → HOLDING невалиден

	if runtime.State != models.StateHolding {
		t.Errorf("ForceTransition() state = %s, want %s", runtime.State, models.StateHolding)
	}
}

// BenchmarkTryTransition измеряет производительность атомарного перехода
func BenchmarkTryTransition(b *testing.B) {
	runtime := &models.PairRuntime{State: models.StateReady}
	for i := 0; i < b.N; i++ {
		runtime.State = models.StateReady
		TryTransition(runtime, 1, models.StateEntering)
	}
}
