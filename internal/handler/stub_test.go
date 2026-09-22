package handler

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"

	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

// stubStore — подменное хранилище. Хендлеры зависят от интерфейса
// repository.Repository, поэтому тесты подставляют заглушку и проверяют,
// что именно хендлер положил в хранилище или прочитал из него,
// а не как оно устроено внутри.
type stubStore struct {
	gauges   map[string]float64
	counters map[string]int64
}

func newStubStore() *stubStore {
	return &stubStore{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (s *stubStore) UpdateGauge(name string, value float64) { s.gauges[name] = value }
func (s *stubStore) UpdateCounter(name string, delta int64) { s.counters[name] += delta }

func (s *stubStore) Gauge(name string) (float64, bool) {
	v, ok := s.gauges[name]
	return v, ok
}

func (s *stubStore) Counter(name string) (int64, bool) {
	v, ok := s.counters[name]
	return v, ok
}

func (s *stubStore) Gauges() map[string]float64 { return maps.Clone(s.gauges) }
func (s *stubStore) Counters() map[string]int64 { return maps.Clone(s.counters) }

var _ repository.Repository = (*stubStore)(nil)

// newRequest собирает запрос так, как его передаёт роутер.
//
// Хендлеры читают сегменты через chi.URLParam, а тот берёт их из контекста
// запроса, куда их кладёт chi при совпадении с маршрутом. В юнит-тесте
// роутера нет, поэтому контекст заполняется вручную; связка
// «маршрут + хендлер» проверяется в тестах пакета server.
func newRequest(method, target string, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, target, http.NoBody)

	rctx := chi.NewRouteContext()
	for name, value := range params {
		rctx.URLParams.Add(name, value)
	}

	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}
