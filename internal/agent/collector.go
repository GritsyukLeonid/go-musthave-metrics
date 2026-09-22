package agent

import (
	"maps"
	"math/rand/v2"
	"runtime"
	"slices"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
)

// Имена метрик, которых нет в runtime.MemStats: агент считает их сам.
const (
	PollCount   = "PollCount"
	RandomValue = "RandomValue"
)

// Collector снимает метрики рантайма и хранит последний снимок.
//
// Все методы вызываются из одной горутины — из цикла Agent.Run, поэтому
// внутренней синхронизации здесь нет. Когда опрос и отправка разъедутся
// по разным горутинам, сюда добавится mutex, а сигнатуры не изменятся.
type Collector struct {
	gauges    map[string]float64
	pollCount int64

	// Обе зависимости подменяются в тестах: runtime.ReadMemStats возвращает
	// значения, которые невозможно предсказать, а rand — воспроизвести.
	readMemStats func(*runtime.MemStats)
	random       func() float64
}

// NewCollector возвращает коллектор, читающий метрики настоящего процесса.
func NewCollector() *Collector {
	return &Collector{
		gauges:       make(map[string]float64),
		readMemStats: runtime.ReadMemStats,
		random:       rand.Float64,
	}
}

// Poll обновляет снимок метрик и увеличивает счётчик опросов.
func (c *Collector) Poll() {
	var ms runtime.MemStats
	c.readMemStats(&ms)

	// Карта пересоздаётся целиком: так в снимке не останется метрики,
	// которую перестали собирать.
	c.gauges = memStatsGauges(&ms)
	c.gauges[RandomValue] = c.random()
	c.pollCount++
}

// Collect возвращает снимок метрик для отправки на сервер.
//
// Порядок фиксирован (по имени): так запросы агента воспроизводимы,
// а тесты не зависят от случайного порядка обхода карты.
func (c *Collector) Collect() []models.Metrics {
	metrics := make([]models.Metrics, 0, len(c.gauges)+1)

	for _, name := range slices.Sorted(maps.Keys(c.gauges)) {
		// Значение копируется в локальную переменную: в Metrics лежит
		// указатель, и снимок не должен ссылаться на внутренности карты.
		value := c.gauges[name]
		metrics = append(metrics, models.Metrics{
			ID:    name,
			MType: models.Gauge,
			Value: &value,
		})
	}

	delta := c.pollCount
	metrics = append(metrics, models.Metrics{
		ID:    PollCount,
		MType: models.Counter,
		Delta: &delta,
	})

	return metrics
}

// Ack сообщает коллектору, что снимок доставлен: счётчик опросов
// уменьшается на уже отправленную величину.
//
// Сервер складывает counter, поэтому отправлять нарастающий итог нельзя —
// его значение утроится за три отчёта. Агент отдаёт приращение с прошлого
// успешного отчёта, а при ошибке Ack не вызывается, и неотправленные
// опросы уедут вместе со следующим снимком.
func (c *Collector) Ack(sent []models.Metrics) {
	for _, m := range sent {
		if m.ID == PollCount && m.MType == models.Counter && m.Delta != nil {
			c.pollCount -= *m.Delta
		}
	}
}

// memStatsGauges переводит поля runtime.MemStats в метрики типа gauge.
// Список задан инкрементом, поэтому он перечислен явно, а не собран
// рефлексией по структуре: лишние поля MemStats серверу не нужны.
func memStatsGauges(ms *runtime.MemStats) map[string]float64 {
	return map[string]float64{
		"Alloc":         float64(ms.Alloc),
		"BuckHashSys":   float64(ms.BuckHashSys),
		"Frees":         float64(ms.Frees),
		"GCCPUFraction": ms.GCCPUFraction,
		"GCSys":         float64(ms.GCSys),
		"HeapAlloc":     float64(ms.HeapAlloc),
		"HeapIdle":      float64(ms.HeapIdle),
		"HeapInuse":     float64(ms.HeapInuse),
		"HeapObjects":   float64(ms.HeapObjects),
		"HeapReleased":  float64(ms.HeapReleased),
		"HeapSys":       float64(ms.HeapSys),
		"LastGC":        float64(ms.LastGC),
		"Lookups":       float64(ms.Lookups),
		"MCacheInuse":   float64(ms.MCacheInuse),
		"MCacheSys":     float64(ms.MCacheSys),
		"MSpanInuse":    float64(ms.MSpanInuse),
		"MSpanSys":      float64(ms.MSpanSys),
		"Mallocs":       float64(ms.Mallocs),
		"NextGC":        float64(ms.NextGC),
		"NumForcedGC":   float64(ms.NumForcedGC),
		"NumGC":         float64(ms.NumGC),
		"OtherSys":      float64(ms.OtherSys),
		"PauseTotalNs":  float64(ms.PauseTotalNs),
		"StackInuse":    float64(ms.StackInuse),
		"StackSys":      float64(ms.StackSys),
		"Sys":           float64(ms.Sys),
		"TotalAlloc":    float64(ms.TotalAlloc),
	}
}
