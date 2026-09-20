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
