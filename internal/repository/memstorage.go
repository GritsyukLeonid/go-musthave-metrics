package repository

import "sync"

// Repository описывает операции над хранилищем метрик.
// Хендлеру не нужно знать, лежат метрики в памяти, в файле или в БД, —
// в следующих инкрементах реализация сменится, а сигнатуры останутся.
type Repository interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, delta int64)
	Gauge(name string) (float64, bool)
	Counter(name string) (int64, bool)
}

// MemStorage хранит метрики в памяти процесса.
//
// Два типа метрик лежат в разных картах, а не в одной с interface{}:
// так тип значения проверяет компилятор, а не рантайм.
//
// mu защищает обе карты. http.Server обрабатывает каждый запрос в своей
// горутине, а конкурентная запись в map — это паника рантайма, а не гонка,
// которую можно не заметить.
type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

// NewMemStorage возвращает готовое к работе хранилище.
// Конструктор обязателен: запись в nil-карту паникует,
// а MemStorage{} создаёт именно nil-карты.
func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

// UpdateGauge замещает предыдущее значение метрики.
func (s *MemStorage) UpdateGauge(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gauges[name] = value
}

// UpdateCounter прибавляет delta к накопленному значению.
// Для отсутствующего ключа map вернёт нулевое значение int64,
// поэтому отдельная ветка «метрика ещё не известна» не нужна.
func (s *MemStorage) UpdateCounter(name string, delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counters[name] += delta
}

// Gauge возвращает значение метрики и признак того, что она известна.
func (s *MemStorage) Gauge(name string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.gauges[name]
	return v, ok
}

// Counter возвращает значение метрики и признак того, что она известна.
func (s *MemStorage) Counter(name string) (int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	v, ok := s.counters[name]
	return v, ok
}

// Проверка на этапе компиляции, что MemStorage реализует Repository.
// Переменная не занимает места в бинаре, но сломает сборку,
// если из интерфейса или из методов что-то разъедется.
var _ Repository = (*MemStorage)(nil)
