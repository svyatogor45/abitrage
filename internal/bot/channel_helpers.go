package bot

import "arbitrage/internal/models"

// tryEnqueueNotification отправляет уведомление в канал с метриками переполнения.
// Возвращает true, если уведомление поставлено в очередь.
func tryEnqueueNotification(ch chan *models.Notification, notif *models.Notification) bool {
	if ch == nil || notif == nil {
		return false
	}

	select {
	case ch <- notif:
		return true
	default:
		RecordBufferOverflow("notification")
		RecordBufferBacklog("notification", cap(ch), len(ch))
		return false
	}
}
