package agent

import (
	"runtime"
	"slices"
	"testing"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
)

// testMemStats — MemStats, у которого все нужные поля различаются.
// Так тест ловит перепутанные поля в memStatsGauges: при копипасте
// MSpanSys вместо MSpanInuse значения разъедутся, а не совпадут.
var testMemStats = runtime.MemStats{
	Alloc:         1,
	BuckHashSys:   2,
	Frees:         3,
	GCCPUFraction: 0.25,
	GCSys:         5,
	HeapAlloc:     6,
	HeapIdle:      7,
	HeapInuse:     8,
	HeapObjects:   9,
	HeapReleased:  10,
	HeapSys:       11,
	LastGC:        12,
	Lookups:       13,
	MCacheInuse:   14,
	MCacheSys:     15,
	MSpanInuse:    16,
	MSpanSys:      17,
	Mallocs:       18,
	NextGC:        19,
	NumForcedGC:   20,
	NumGC:         21,
	OtherSys:      22,
	PauseTotalNs:  23,
	StackInuse:    24,
	StackSys:      25,
	Sys:           26,
	TotalAlloc:    27,
}

const testRandomValue = 0.42

// newTestCollector возвращает коллектор с предсказуемыми источниками:
// настоящие runtime.ReadMemStats и rand дают значения, которые нельзя
// сравнить с ожидаемыми.
func newTestCollector() *Collector {
	c := NewCollector()
	c.readMemStats = func(ms *runtime.MemStats) { *ms = testMemStats }
	c.random = func() float64 { return testRandomValue }
	return c
}

func TestCollectorPollCollectsRequiredMetrics(t *testing.T) {
	wantGauges := map[string]float64{
		"Alloc":         1,
		"BuckHashSys":   2,
		"Frees":         3,
		"GCCPUFraction": 0.25,
		"GCSys":         5,
		"HeapAlloc":     6,
		"HeapIdle":      7,
		"HeapInuse":     8,
		"HeapObjects":   9,
		"HeapReleased":  10,
		"HeapSys":       11,
		"LastGC":        12,
		"Lookups":       13,
		"MCacheInuse":   14,
		"MCacheSys":     15,
		"MSpanInuse":    16,
		"MSpanSys":      17,
		"Mallocs":       18,
		"NextGC":        19,
		"NumForcedGC":   20,
		"NumGC":         21,
		"OtherSys":      22,
		"PauseTotalNs":  23,
		"StackInuse":    24,
		"StackSys":      25,
		"Sys":           26,
		"TotalAlloc":    27,
		RandomValue:     testRandomValue,
	}

	c := newTestCollector()
	c.Poll()
	got := c.Collect()

	if len(got) != len(wantGauges)+1 {
		t.Errorf("собрано %d метрик; ожидалось %d", len(got), len(wantGauges)+1)
	}

	seen := make(map[string]models.Metrics, len(got))
	for _, m := range got {
		if _, dup := seen[m.ID]; dup {
			t.Errorf("метрика %s встретилась дважды", m.ID)
		}
		seen[m.ID] = m
	}

	for name, want := range wantGauges {
		m, ok := seen[name]
		if !ok {
			t.Errorf("метрика %s не собрана", name)
			continue
		}
		if m.MType != models.Gauge {
			t.Errorf("метрика %s имеет тип %q; ожидался %q", name, m.MType, models.Gauge)
		}
		if m.Delta != nil {
			t.Errorf("у gauge %s заполнено поле Delta", name)
		}
		if m.Value == nil {
			t.Errorf("у gauge %s не заполнено поле Value", name)
			continue
		}
		if *m.Value != want {
			t.Errorf("метрика %s = %v; ожидалось %v", name, *m.Value, want)
		}
	}

	poll, ok := seen[PollCount]
	if !ok {
		t.Fatalf("метрика %s не собрана", PollCount)
	}
	if poll.MType != models.Counter {
		t.Errorf("%s имеет тип %q; ожидался %q", PollCount, poll.MType, models.Counter)
	}
	if poll.Value != nil {
		t.Errorf("у counter %s заполнено поле Value", PollCount)
	}
	if poll.Delta == nil || *poll.Delta != 1 {
		t.Errorf("%s = %v; ожидалось 1", PollCount, poll.Delta)
	}
}

func TestCollectorPollCountGrowsWithEachPoll(t *testing.T) {
	c := newTestCollector()

	for i := range 3 {
		c.Poll()

		if got := pollCountOf(t, c.Collect()); got != int64(i+1) {
			t.Errorf("после %d опросов %s = %d; ожидалось %d", i+1, PollCount, got, i+1)
		}
	}
}

// TestCollectorAck фиксирует договорённость с сервером: тот складывает
// counter, поэтому агент отдаёт приращение с прошлого успешного отчёта.
func TestCollectorAck(t *testing.T) {
	c := newTestCollector()

	c.Poll()
	c.Poll()

	sent := c.Collect()
	if got := pollCountOf(t, sent); got != 2 {
		t.Fatalf("%s в первом отчёте = %d; ожидалось 2", PollCount, got)
	}

	// Отчёт не подтверждён — опросы остаются за агентом.
	c.Poll()
	if got := pollCountOf(t, c.Collect()); got != 3 {
		t.Errorf("%s без подтверждения = %d; ожидалось 3", PollCount, got)
	}

	// Подтверждение снимает ровно отправленные два опроса, третий остаётся.
	c.Ack(sent)
	if got := pollCountOf(t, c.Collect()); got != 1 {
		t.Errorf("%s после Ack = %d; ожидалось 1", PollCount, got)
	}
}

// TestCollectorCollectIsIndependentSnapshot проверяет, что снимок не даёт
// доступа к внутренностям коллектора: Metrics хранит значения указателями,
// и без копирования отправка могла бы менять состояние.
func TestCollectorCollectIsIndependentSnapshot(t *testing.T) {
	c := newTestCollector()
	c.Poll()

	first := c.Collect()
	for _, m := range first {
		if m.Value != nil {
			*m.Value = -1
		}
		if m.Delta != nil {
			*m.Delta = -1
		}
	}

	second := c.Collect()
	for _, m := range second {
		if m.Value != nil && *m.Value == -1 {
			t.Errorf("значение метрики %s изменилось через прошлый снимок", m.ID)
		}
		if m.Delta != nil && *m.Delta == -1 {
			t.Errorf("приращение метрики %s изменилось через прошлый снимок", m.ID)
		}
	}
}

func TestCollectorCollectIsOrdered(t *testing.T) {
	c := newTestCollector()
	c.Poll()

	got := c.Collect()

	names := make([]string, 0, len(got))
	for _, m := range got[:len(got)-1] {
		names = append(names, m.ID)
	}

	if !slices.IsSorted(names) {
		t.Errorf("метрики идут не по алфавиту: %v", names)
	}
	if last := got[len(got)-1].ID; last != PollCount {
		t.Errorf("последняя метрика снимка = %s; ожидался %s", last, PollCount)
	}
}

// TestCollectorUsesRuntime проверяет коллектор, собранный NewCollector,
// то есть настоящие runtime.ReadMemStats и rand.
func TestCollectorUsesRuntime(t *testing.T) {
	c := NewCollector()
	c.Poll()

	for _, m := range c.Collect() {
		switch m.ID {
		case RandomValue:
			if *m.Value < 0 || *m.Value >= 1 {
				t.Errorf("%s = %v; ожидалось значение из [0, 1)", RandomValue, *m.Value)
			}
		case "Sys":
			// Sys — общий объём памяти, полученный процессом от ОС:
			// у работающего процесса он не может быть нулевым.
			if *m.Value <= 0 {
				t.Errorf("Sys = %v; ожидалось значение больше нуля", *m.Value)
			}
		}
	}
}

// pollCountOf достаёт из снимка приращение счётчика опросов.
func pollCountOf(t *testing.T, metrics []models.Metrics) int64 {
	t.Helper()

	for _, m := range metrics {
		if m.ID == PollCount {
			if m.Delta == nil {
				t.Fatalf("у метрики %s не заполнено поле Delta", PollCount)
			}
			return *m.Delta
		}
	}

	t.Fatalf("метрика %s не найдена в снимке", PollCount)
	return 0
}
