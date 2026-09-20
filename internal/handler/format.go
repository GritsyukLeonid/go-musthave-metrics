package handler

import "strconv"

// formatGauge переводит gauge в текст для ответа сервера.
//
// 'f' и точность -1 дают минимум цифр, по которым ParseFloat вернёт ровно
// то же число, и без экспоненты: агент отправил «123.45» — сервер отдаёт
// «123.45», а не «1.2345e+02».
func formatGauge(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// formatCounter переводит counter в текст для ответа сервера.
func formatCounter(delta int64) string {
	return strconv.FormatInt(delta, 10)
}
