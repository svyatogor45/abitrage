package utils

import (
	"testing"
	"time"
)

func TestGetDayStartFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "middle of day",
			input:    time.Date(2024, 1, 15, 14, 30, 45, 123456789, time.UTC),
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "start of day",
			input:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "end of day",
			input:    time.Date(2024, 1, 15, 23, 59, 59, 999999999, time.UTC),
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "leap year",
			input:    time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetDayStartFrom(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("GetDayStartFrom(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetDayEndFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "middle of day",
			input:    time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
			expected: time.Date(2024, 1, 15, 23, 59, 59, 999999999, time.UTC),
		},
		{
			name:     "start of day",
			input:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 1, 15, 23, 59, 59, 999999999, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetDayEndFrom(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("GetDayEndFrom(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetWeekStartFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "wednesday",
			input:    time.Date(2024, 1, 17, 14, 30, 45, 0, time.UTC), // Wednesday
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),    // Monday
		},
		{
			name:     "monday",
			input:    time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC), // Monday
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),    // Monday
		},
		{
			name:     "sunday",
			input:    time.Date(2024, 1, 21, 14, 30, 45, 0, time.UTC), // Sunday
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),    // Monday of same week
		},
		{
			name:     "saturday",
			input:    time.Date(2024, 1, 20, 14, 30, 45, 0, time.UTC), // Saturday
			expected: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),    // Monday
		},
		{
			name:     "week spanning months",
			input:    time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC), // Thursday Feb 1
			expected: time.Date(2024, 1, 29, 0, 0, 0, 0, time.UTC), // Monday Jan 29
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetWeekStartFrom(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("GetWeekStartFrom(%v) = %v (weekday: %v), want %v", tt.input, result, result.Weekday(), tt.expected)
			}
			// Verify it's Monday
			if result.Weekday() != time.Monday {
				t.Errorf("GetWeekStartFrom(%v) returned %v which is %v, expected Monday", tt.input, result, result.Weekday())
			}
		})
	}
}

func TestGetWeekEndFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "wednesday",
			input:    time.Date(2024, 1, 17, 14, 30, 45, 0, time.UTC), // Wednesday
			expected: time.Date(2024, 1, 21, 23, 59, 59, 999999999, time.UTC), // Sunday
		},
		{
			name:     "monday",
			input:    time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), // Monday
			expected: time.Date(2024, 1, 21, 23, 59, 59, 999999999, time.UTC), // Sunday
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetWeekEndFrom(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("GetWeekEndFrom(%v) = %v, want %v", tt.input, result, tt.expected)
			}
			// Verify it's Sunday
			if result.Weekday() != time.Sunday {
				t.Errorf("GetWeekEndFrom(%v) returned %v which is %v, expected Sunday", tt.input, result, result.Weekday())
			}
		})
	}
}

func TestGetMonthStartFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "middle of month",
			input:    time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "first day of month",
			input:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "last day of month",
			input:    time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC),
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "february leap year",
			input:    time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMonthStartFrom(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("GetMonthStartFrom(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetMonthEndFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "january",
			input:    time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC),
			expected: time.Date(2024, 1, 31, 23, 59, 59, 999999999, time.UTC),
		},
		{
			name:     "february leap year",
			input:    time.Date(2024, 2, 15, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 2, 29, 23, 59, 59, 999999999, time.UTC),
		},
		{
			name:     "february non-leap year",
			input:    time.Date(2023, 2, 15, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2023, 2, 28, 23, 59, 59, 999999999, time.UTC),
		},
		{
			name:     "december",
			input:    time.Date(2024, 12, 15, 12, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 12, 31, 23, 59, 59, 999999999, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMonthEndFrom(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("GetMonthEndFrom(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetYearStartFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "middle of year",
			input:    time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC),
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "first day of year",
			input:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "last day of year",
			input:    time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetYearStartFrom(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("GetYearStartFrom(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetYearEndFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "middle of year",
			input:    time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC),
			expected: time.Date(2024, 12, 31, 23, 59, 59, 999999999, time.UTC),
		},
		{
			name:     "first day of year",
			input:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 12, 31, 23, 59, 59, 999999999, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetYearEndFrom(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("GetYearEndFrom(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTimeRangeContains(t *testing.T) {
	tr := TimeRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 31, 23, 59, 59, 999999999, time.UTC),
	}

	tests := []struct {
		name     string
		time     time.Time
		expected bool
	}{
		{
			name:     "within range",
			time:     time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "at start",
			time:     time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "at end",
			time:     time.Date(2024, 1, 31, 23, 59, 59, 999999999, time.UTC),
			expected: true,
		},
		{
			name:     "before range",
			time:     time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: false,
		},
		{
			name:     "after range",
			time:     time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.Contains(tt.time)
			if result != tt.expected {
				t.Errorf("TimeRange.Contains(%v) = %v, want %v", tt.time, result, tt.expected)
			}
		})
	}
}

func TestTimeRangeDuration(t *testing.T) {
	tr := TimeRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
	}

	expected := 24 * time.Hour
	result := tr.Duration()

	if result != expected {
		t.Errorf("TimeRange.Duration() = %v, want %v", result, expected)
	}
}

func TestGetLastNDays(t *testing.T) {
	tr := GetLastNDays(7)

	// Should span 7 days
	duration := tr.Duration()
	expectedDays := 7
	actualDays := int(duration.Hours()/24) + 1 // +1 because includes both start and end days

	if actualDays != expectedDays {
		t.Errorf("GetLastNDays(7) spans %d days, want %d", actualDays, expectedDays)
	}
}

func TestGetLastNHours(t *testing.T) {
	tr := GetLastNHours(24)

	// Should span approximately 24 hours
	duration := tr.Duration()
	expectedHours := 24 * time.Hour

	// Allow small tolerance for time passing during test
	if duration < expectedHours-time.Minute || duration > expectedHours+time.Minute {
		t.Errorf("GetLastNHours(24) spans %v, want approximately %v", duration, expectedHours)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Duration
		expected string
	}{
		{
			name:     "seconds",
			input:    45 * time.Second,
			expected: "45s",
		},
		{
			name:     "minutes",
			input:    5 * time.Minute,
			expected: "5m0s",
		},
		{
			name:     "minutes and seconds",
			input:    5*time.Minute + 30*time.Second,
			expected: "5m30s",
		},
		{
			name:     "hours",
			input:    2 * time.Hour,
			expected: "2h0m0s",
		},
		{
			name:     "hours and minutes",
			input:    2*time.Hour + 15*time.Minute,
			expected: "2h15m0s",
		},
		{
			name:     "days",
			input:    72 * time.Hour,
			expected: "72h0m0s",
		},
		{
			name:     "zero",
			input:    0,
			expected: "0s",
		},
		{
			name:     "negative",
			input:    -5 * time.Minute,
			expected: "5m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.input)
			if result != tt.expected {
				t.Errorf("FormatDuration(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestUnixMillis(t *testing.T) {
	before := time.Now().UnixMilli()
	result := UnixMillis()
	after := time.Now().UnixMilli()

	if result < before || result > after {
		t.Errorf("UnixMillis() = %d, expected between %d and %d", result, before, after)
	}
}

func TestFromUnixMillis(t *testing.T) {
	now := time.Now().UTC()
	ms := now.UnixMilli()

	result := FromUnixMillis(ms)

	// Should be within 1 millisecond
	diff := now.Sub(result)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Millisecond {
		t.Errorf("FromUnixMillis(%d) = %v, expected close to %v", ms, result, now)
	}
}

func TestGetPeriodStart(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name   string
		period PeriodType
	}{
		{"day", PeriodDay},
		{"week", PeriodWeek},
		{"month", PeriodMonth},
		{"year", PeriodYear},
		{"all", PeriodAll},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPeriodStart(tt.period)

			if tt.period == PeriodAll {
				if !result.IsZero() {
					t.Errorf("GetPeriodStart(PeriodAll) should return zero time, got %v", result)
				}
			} else {
				if result.After(now) {
					t.Errorf("GetPeriodStart(%s) = %v, should be before now (%v)", tt.period, result, now)
				}
			}
		})
	}
}

func TestIsInPeriod(t *testing.T) {
	now := time.Now().UTC()
	yesterday := now.Add(-24 * time.Hour)
	lastMonth := now.AddDate(0, -1, -5)
	lastYear := now.AddDate(-1, -1, 0)

	tests := []struct {
		name     string
		time     time.Time
		period   PeriodType
		expected bool
	}{
		{"now in day", now, PeriodDay, true},
		{"yesterday in day", yesterday, PeriodDay, false},
		{"now in week", now, PeriodWeek, true},
		{"now in month", now, PeriodMonth, true},
		{"last month in month", lastMonth, PeriodMonth, false},
		{"now in year", now, PeriodYear, true},
		{"last year in year", lastYear, PeriodYear, false},
		{"now in all", now, PeriodAll, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsInPeriod(tt.time, tt.period)
			if result != tt.expected {
				t.Errorf("IsInPeriod(%v, %s) = %v, want %v", tt.time, tt.period, result, tt.expected)
			}
		})
	}
}

func TestToUTC(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	eastern := time.Date(2024, 1, 15, 12, 0, 0, 0, loc)

	result := ToUTC(eastern)

	if result.Location() != time.UTC {
		t.Errorf("ToUTC() location = %v, want UTC", result.Location())
	}
}

// Benchmarks

func BenchmarkGetDayStartFrom(b *testing.B) {
	t := time.Now().UTC()
	for i := 0; i < b.N; i++ {
		GetDayStartFrom(t)
	}
}

func BenchmarkGetWeekStartFrom(b *testing.B) {
	t := time.Now().UTC()
	for i := 0; i < b.N; i++ {
		GetWeekStartFrom(t)
	}
}

func BenchmarkGetMonthStartFrom(b *testing.B) {
	t := time.Now().UTC()
	for i := 0; i < b.N; i++ {
		GetMonthStartFrom(t)
	}
}

func BenchmarkTimeRangeContains(b *testing.B) {
	tr := GetMonthRange()
	t := time.Now().UTC()
	for i := 0; i < b.N; i++ {
		tr.Contains(t)
	}
}
