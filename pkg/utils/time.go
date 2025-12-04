package utils

import (
	"time"
)

// time.go - утилиты для работы со временем
//
// Назначение:
// Вспомогательные функции для временных операций, используемые
// для агрегации статистики по периодам и фильтрации данных.
//
// Функции:
// - GetDayStart: начало текущего дня (00:00:00)
// - GetWeekStart: начало текущей недели (понедельник 00:00:00)
// - GetMonthStart: начало текущего месяца (1-е число 00:00:00)
// - GetYearStart: начало года (1 января 00:00:00)
//
// Использование:
// - Агрегация статистики по периодам (day/week/month)
// - Фильтрация данных по временным диапазонам
// - Очистка старых записей из БД

// ============================================================
// Основные функции получения границ периодов
// ============================================================

// GetDayStart возвращает начало текущего дня (00:00:00) в UTC
//
// Пример:
//
//	// Сейчас: 2024-01-15 14:30:45 UTC
//	start := GetDayStart()
//	// start: 2024-01-15 00:00:00 UTC
func GetDayStart() time.Time {
	return GetDayStartFrom(time.Now().UTC())
}

// GetDayStartFrom возвращает начало дня для указанного времени в UTC
//
// Параметры:
//   - t: исходное время
//
// Возвращает: начало дня (00:00:00 UTC) для указанной даты
func GetDayStartFrom(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// GetDayEnd возвращает конец текущего дня (23:59:59.999999999) в UTC
//
// Пример:
//
//	// Сейчас: 2024-01-15 14:30:45 UTC
//	end := GetDayEnd()
//	// end: 2024-01-15 23:59:59.999999999 UTC
func GetDayEnd() time.Time {
	return GetDayEndFrom(time.Now().UTC())
}

// GetDayEndFrom возвращает конец дня для указанного времени
func GetDayEndFrom(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, time.UTC)
}

// GetWeekStart возвращает начало текущей недели (понедельник 00:00:00) в UTC
//
// Неделя начинается с понедельника (ISO 8601)
//
// Пример:
//
//	// Сейчас: Среда 2024-01-17 14:30:45 UTC
//	start := GetWeekStart()
//	// start: Понедельник 2024-01-15 00:00:00 UTC
func GetWeekStart() time.Time {
	return GetWeekStartFrom(time.Now().UTC())
}

// GetWeekStartFrom возвращает начало недели для указанного времени
//
// Параметры:
//   - t: исходное время
//
// Возвращает: понедельник 00:00:00 UTC недели, содержащей указанную дату
func GetWeekStartFrom(t time.Time) time.Time {
	t = t.UTC()

	// Получаем день недели (0=Sunday, 1=Monday, ..., 6=Saturday)
	weekday := int(t.Weekday())

	// Преобразуем к ISO 8601 (1=Monday, ..., 7=Sunday)
	if weekday == 0 {
		weekday = 7
	}

	// Вычисляем количество дней назад до понедельника
	daysBack := weekday - 1

	// Возвращаем начало понедельника
	monday := t.AddDate(0, 0, -daysBack)
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}

// GetWeekEnd возвращает конец текущей недели (воскресенье 23:59:59.999999999) в UTC
func GetWeekEnd() time.Time {
	return GetWeekEndFrom(time.Now().UTC())
}

// GetWeekEndFrom возвращает конец недели для указанного времени
func GetWeekEndFrom(t time.Time) time.Time {
	// Находим начало недели и добавляем 6 дней
	weekStart := GetWeekStartFrom(t)
	sunday := weekStart.AddDate(0, 0, 6)
	return time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 23, 59, 59, 999999999, time.UTC)
}

// GetMonthStart возвращает начало текущего месяца (1-е число 00:00:00) в UTC
//
// Пример:
//
//	// Сейчас: 2024-01-15 14:30:45 UTC
//	start := GetMonthStart()
//	// start: 2024-01-01 00:00:00 UTC
func GetMonthStart() time.Time {
	return GetMonthStartFrom(time.Now().UTC())
}

// GetMonthStartFrom возвращает начало месяца для указанного времени
//
// Параметры:
//   - t: исходное время
//
// Возвращает: 1-е число месяца 00:00:00 UTC
func GetMonthStartFrom(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// GetMonthEnd возвращает конец текущего месяца в UTC
func GetMonthEnd() time.Time {
	return GetMonthEndFrom(time.Now().UTC())
}

// GetMonthEndFrom возвращает конец месяца для указанного времени
func GetMonthEndFrom(t time.Time) time.Time {
	t = t.UTC()
	// Переходим к первому числу следующего месяца и отнимаем наносекунду
	firstOfNextMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	return firstOfNextMonth.Add(-time.Nanosecond)
}

// GetYearStart возвращает начало текущего года (1 января 00:00:00) в UTC
//
// Пример:
//
//	// Сейчас: 2024-01-15 14:30:45 UTC
//	start := GetYearStart()
//	// start: 2024-01-01 00:00:00 UTC
func GetYearStart() time.Time {
	return GetYearStartFrom(time.Now().UTC())
}

// GetYearStartFrom возвращает начало года для указанного времени
func GetYearStartFrom(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
}

// GetYearEnd возвращает конец текущего года в UTC
func GetYearEnd() time.Time {
	return GetYearEndFrom(time.Now().UTC())
}

// GetYearEndFrom возвращает конец года для указанного времени
func GetYearEndFrom(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), time.December, 31, 23, 59, 59, 999999999, time.UTC)
}

// ============================================================
// Вспомогательные функции
// ============================================================

// GetPreviousDayStart возвращает начало предыдущего дня
func GetPreviousDayStart() time.Time {
	return GetDayStartFrom(time.Now().UTC().AddDate(0, 0, -1))
}

// GetPreviousWeekStart возвращает начало предыдущей недели
func GetPreviousWeekStart() time.Time {
	return GetWeekStartFrom(time.Now().UTC().AddDate(0, 0, -7))
}

// GetPreviousMonthStart возвращает начало предыдущего месяца
func GetPreviousMonthStart() time.Time {
	return GetMonthStartFrom(time.Now().UTC().AddDate(0, -1, 0))
}

// ============================================================
// Функции для работы с диапазонами
// ============================================================

// TimeRange представляет временной диапазон
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// Contains проверяет, попадает ли время в диапазон
func (tr TimeRange) Contains(t time.Time) bool {
	return !t.Before(tr.Start) && !t.After(tr.End)
}

// Duration возвращает продолжительность диапазона
func (tr TimeRange) Duration() time.Duration {
	return tr.End.Sub(tr.Start)
}

// GetDayRange возвращает диапазон текущего дня
func GetDayRange() TimeRange {
	return TimeRange{
		Start: GetDayStart(),
		End:   GetDayEnd(),
	}
}

// GetWeekRange возвращает диапазон текущей недели
func GetWeekRange() TimeRange {
	return TimeRange{
		Start: GetWeekStart(),
		End:   GetWeekEnd(),
	}
}

// GetMonthRange возвращает диапазон текущего месяца
func GetMonthRange() TimeRange {
	return TimeRange{
		Start: GetMonthStart(),
		End:   GetMonthEnd(),
	}
}

// GetYearRange возвращает диапазон текущего года
func GetYearRange() TimeRange {
	return TimeRange{
		Start: GetYearStart(),
		End:   GetYearEnd(),
	}
}

// GetLastNDays возвращает диапазон последних n дней (включая сегодня)
func GetLastNDays(n int) TimeRange {
	if n <= 0 {
		n = 1
	}
	now := time.Now().UTC()
	return TimeRange{
		Start: GetDayStartFrom(now.AddDate(0, 0, -(n - 1))),
		End:   GetDayEndFrom(now),
	}
}

// GetLastNHours возвращает диапазон последних n часов
func GetLastNHours(n int) TimeRange {
	if n <= 0 {
		n = 1
	}
	now := time.Now().UTC()
	return TimeRange{
		Start: now.Add(-time.Duration(n) * time.Hour),
		End:   now,
	}
}

// ============================================================
// Форматирование времени
// ============================================================

// FormatDuration форматирует продолжительность в человекочитаемый формат
//
// Примеры:
//   - "45s"
//   - "5m30s"
//   - "2h15m"
//   - "3d5h"
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		if hours > 0 {
			return (time.Duration(days*24+hours) * time.Hour).String()
		}
		return (time.Duration(days*24) * time.Hour).String()
	}

	if hours > 0 {
		if minutes > 0 {
			return (time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute).String()
		}
		return (time.Duration(hours) * time.Hour).String()
	}

	if minutes > 0 {
		if seconds > 0 {
			return (time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second).String()
		}
		return (time.Duration(minutes) * time.Minute).String()
	}

	return (time.Duration(seconds) * time.Second).String()
}

// ============================================================
// Утилиты для timestamp
// ============================================================

// UnixMillis возвращает текущее время в миллисекундах Unix
func UnixMillis() int64 {
	return time.Now().UnixMilli()
}

// FromUnixMillis конвертирует миллисекунды Unix в time.Time
func FromUnixMillis(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}

// UnixMicros возвращает текущее время в микросекундах Unix
func UnixMicros() int64 {
	return time.Now().UnixMicro()
}

// FromUnixMicros конвертирует микросекунды Unix в time.Time
func FromUnixMicros(us int64) time.Time {
	return time.UnixMicro(us).UTC()
}

// ============================================================
// Функции для работы с периодами статистики
// ============================================================

// PeriodType тип периода для статистики
type PeriodType string

const (
	PeriodDay   PeriodType = "day"
	PeriodWeek  PeriodType = "week"
	PeriodMonth PeriodType = "month"
	PeriodYear  PeriodType = "year"
	PeriodAll   PeriodType = "all"
)

// GetPeriodStart возвращает начало периода указанного типа
func GetPeriodStart(period PeriodType) time.Time {
	switch period {
	case PeriodDay:
		return GetDayStart()
	case PeriodWeek:
		return GetWeekStart()
	case PeriodMonth:
		return GetMonthStart()
	case PeriodYear:
		return GetYearStart()
	case PeriodAll:
		return time.Time{} // zero time
	default:
		return GetDayStart()
	}
}

// GetPeriodRange возвращает диапазон для указанного периода
func GetPeriodRange(period PeriodType) TimeRange {
	switch period {
	case PeriodDay:
		return GetDayRange()
	case PeriodWeek:
		return GetWeekRange()
	case PeriodMonth:
		return GetMonthRange()
	case PeriodYear:
		return GetYearRange()
	case PeriodAll:
		return TimeRange{
			Start: time.Time{},
			End:   time.Now().UTC(),
		}
	default:
		return GetDayRange()
	}
}

// IsInPeriod проверяет, попадает ли время в указанный период
func IsInPeriod(t time.Time, period PeriodType) bool {
	return GetPeriodRange(period).Contains(t)
}

// ============================================================
// Функции для timezone
// ============================================================

// ToUTC конвертирует время в UTC
func ToUTC(t time.Time) time.Time {
	return t.UTC()
}

// ToLocation конвертирует время в указанную timezone
func ToLocation(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		return t
	}
	return t.In(loc)
}

// ParseInLocation парсит время в указанной timezone
func ParseInLocation(layout, value string, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.UTC
	}
	return time.ParseInLocation(layout, value, loc)
}
