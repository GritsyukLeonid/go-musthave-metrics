package repository

import (
	"sync"
	"testing"
)

func TestMemStorageUpdateGauge(t *testing.T) {
	s := NewMemStorage()

	if _, ok := s.Gauge("Alloc"); ok {
		t.Fatal("Gauge() вернул ok=true для метрики, которую ещё не записывали")
	}

	s.UpdateGauge("Alloc", 12.5)
	if v, ok := s.Gauge("Alloc"); !ok || v != 12.5 {
		t.Errorf("Gauge() = %v, %v; ожидалось 12.5, true", v, ok)
	}

	// gauge замещается, а не накапливается.
	s.UpdateGauge("Alloc", -0.5)
	if v, ok := s.Gauge("Alloc"); !ok || v != -0.5 {
		t.Errorf("Gauge() после перезаписи = %v, %v; ожидалось -0.5, true", v, ok)
	}
}

func TestMemStorageUpdateCounter(t *testing.T) {
	s := NewMemStorage()

	if _, ok := s.Counter("PollCount"); ok {
		t.Fatal("Counter() вернул ok=true для метрики, которую ещё не записывали")
	}

	// counter накапливается: три отчёта агента дают сумму приращений.
	s.UpdateCounter("PollCount", 5)
	s.UpdateCounter("PollCount", 7)
	s.UpdateCounter("PollCount", -2)

	if v, ok := s.Counter("PollCount"); !ok || v != 10 {
		t.Errorf("Counter() = %v, %v; ожидалось 10, true", v, ok)
	}
}

func TestMemStorageTypesAreIndependent(t *testing.T) {
	s := NewMemStorage()

	// Одно имя в разных типах — разные ячейки, gauge не должен
	// подменить counter и наоборот.
	s.UpdateGauge("Metric", 1.5)
	s.UpdateCounter("Metric", 3)

	if v, ok := s.Gauge("Metric"); !ok || v != 1.5 {
		t.Errorf("Gauge() = %v, %v; ожидалось 1.5, true", v, ok)
	}
	if v, ok := s.Counter("Metric"); !ok || v != 3 {
		t.Errorf("Counter() = %v, %v; ожидалось 3, true", v, ok)
	}
}

// TestMemStorageSnapshots проверяет то, ради чего Gauges и Counters
// отдают копию: страница со списком метрик рендерится уже без блокировки,
// и снимок не должен меняться под руками — ни от записи в хранилище,
// ни от правок самого снимка.
func TestMemStorageSnapshots(t *testing.T) {
	s := NewMemStorage()

	if got := len(s.Gauges()); got != 0 {
		t.Errorf("в пустом хранилище %d метрик типа gauge; ожидалось 0", got)
	}
	if got := len(s.Counters()); got != 0 {
		t.Errorf("в пустом хранилище %d метрик типа counter; ожидалось 0", got)
	}

	s.UpdateGauge("Alloc", 1.5)
	s.UpdateCounter("PollCount", 5)

	gauges, counters := s.Gauges(), s.Counters()

	if v, ok := gauges["Alloc"]; !ok || v != 1.5 {
		t.Errorf("Gauges()[\"Alloc\"] = %v, %v; ожидалось 1.5, true", v, ok)
	}
	if v, ok := counters["PollCount"]; !ok || v != 5 {
		t.Errorf("Counters()[\"PollCount\"] = %v, %v; ожидалось 5, true", v, ok)
	}

	// Правки снимка не доезжают до хранилища.
	gauges["Alloc"] = 100
	delete(counters, "PollCount")

	if v, _ := s.Gauge("Alloc"); v != 1.5 {
		t.Errorf("Gauge() после правки снимка = %v; ожидалось 1.5", v)
	}
	if _, ok := s.Counter("PollCount"); !ok {
		t.Error("Counter() после удаления из снимка вернул ok=false; ожидалось true")
	}

	// И наоборот: запись в хранилище не попадает в уже отданный снимок.
	s.UpdateGauge("HeapSys", 2)
	if _, ok := s.Gauges()["HeapSys"]; !ok {
		t.Error("новая метрика не попала в свежий снимок")
	}
	if _, ok := gauges["HeapSys"]; ok {
		t.Error("новая метрика попала в снимок, сделанный до неё")
	}
}

// TestMemStorageConcurrentUpdates проверяет то, ради чего в MemStorage
// добавлен mutex: http.Server обрабатывает каждый запрос в своей горутине,
// а конкурентная запись в map — паника рантайма. Под -race тест ловит
// и гонку, и потерянные приращения.
func TestMemStorageConcurrentUpdates(t *testing.T) {
	const (
		writers        = 50
		updatesPerGoro = 100
	)

	s := NewMemStorage()

	var wg sync.WaitGroup
	wg.Add(writers)
	for i := range writers {
		go func() {
			defer wg.Done()
			for range updatesPerGoro {
				s.UpdateCounter("PollCount", 1)
				s.UpdateGauge("RandomValue", float64(i))
				s.Counter("PollCount")
				s.Gauge("RandomValue")
			}
		}()
	}
	wg.Wait()

	if v, _ := s.Counter("PollCount"); v != writers*updatesPerGoro {
		t.Errorf("Counter() = %d; ожидалось %d", v, writers*updatesPerGoro)
	}
}
