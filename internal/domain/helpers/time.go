package helpers

import "time"

// GMT7Location возвращает локацию для GMT+7 (UTC+7)
// Используется для работы с датами в базе данных
var GMT7Location *time.Location

func init() {
	// Создаем фиксированную временную зону GMT+7 (7 часов от UTC)
	// 7 * 3600 секунд = 25200 секунд
	GMT7Location = time.FixedZone("GMT+7", 7*3600)
}

// NowGMT7 возвращает текущее время в часовом поясе GMT+7
func NowGMT7() time.Time {
	return time.Now().In(GMT7Location)
}
