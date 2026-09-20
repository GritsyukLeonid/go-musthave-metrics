package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

// stubStore — подменное хранилище. Хендлер зависит от интерфейса
// repository.Repository, поэтому тест подставляет заглушку и проверяет,
// что именно хендлер положил в хранилище, а не как оно устроено внутри.
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

var _ repository.Repository = (*stubStore)(nil)

// newUpdateRequest собирает запрос так, как его передаёт роутер.
//
// Хендлер читает сегменты через r.PathValue, а они появляются только при
// совпадении с шаблоном маршрута. В юнит-тесте роутера нет, поэтому
// значения проставляются вручную; связка «шаблон + хендлер» проверяется
// в тестах пакета server.
func newUpdateRequest(mType, name, value string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/update/"+mType+"/"+name+"/"+value, http.NoBody)
	req.SetPathValue("type", mType)
	req.SetPathValue("name", name)
	req.SetPathValue("value", value)
	return req
}

func TestUpdateServeHTTP(t *testing.T) {
	tests := []struct {
		name        string
		mType       string
		metric      string
		value       string
		wantCode    int
		wantGauge   map[string]float64
		wantCounter map[string]int64
	}{
		{
			name:      "gauge принимается",
			mType:     models.Gauge,
			metric:    "Alloc",
			value:     "123.45",
			wantCode:  http.StatusOK,
			wantGauge: map[string]float64{"Alloc": 123.45},
		},
		{
			name:      "gauge в экспоненциальной записи",
			mType:     models.Gauge,
			metric:    "RandomValue",
			value:     "-1.5e-3",
			wantCode:  http.StatusOK,
			wantGauge: map[string]float64{"RandomValue": -0.0015},
		},
		{
			name:      "целое значение gauge",
			mType:     models.Gauge,
			metric:    "HeapSys",
			value:     "100",
			wantCode:  http.StatusOK,
			wantGauge: map[string]float64{"HeapSys": 100},
		},
		{
			name:        "counter принимается",
			mType:       models.Counter,
			metric:      "PollCount",
			value:       "5",
			wantCode:    http.StatusOK,
			wantCounter: map[string]int64{"PollCount": 5},
		},
		{
			name:        "отрицательный counter принимается",
			mType:       models.Counter,
			metric:      "PollCount",
			value:       "-3",
			wantCode:    http.StatusOK,
			wantCounter: map[string]int64{"PollCount": -3},
		},
		{
			name:     "нечисловой gauge отклоняется",
			mType:    models.Gauge,
			metric:   "Alloc",
			value:    "not-a-number",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "пустое значение отклоняется",
			mType:    models.Gauge,
			metric:   "Alloc",
			value:    "",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "дробный counter отклоняется",
			mType:    models.Counter,
			metric:   "PollCount",
			value:    "1.5",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "неизвестный тип отклоняется",
			mType:    "histogram",
			metric:   "Alloc",
			value:    "1",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "пустой тип отклоняется",
			mType:    "",
			metric:   "Alloc",
			value:    "1",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "запрос без имени метрики",
			mType:    models.Gauge,
			metric:   "",
			value:    "1",
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newStubStore()
			rec := httptest.NewRecorder()

			NewUpdate(store).ServeHTTP(rec, newUpdateRequest(tt.mType, tt.metric, tt.value))

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.wantCode {
				t.Errorf("код ответа = %d; ожидался %d", res.StatusCode, tt.wantCode)
			}

			if res.StatusCode == http.StatusOK {
				if got := res.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
					t.Errorf("Content-Type = %q; ожидался %q", got, "text/plain; charset=utf-8")
				}
			}

			// Отклонённый запрос не должен ничего записать в хранилище.
			assertStored(t, "gauge", store.gauges, tt.wantGauge)
			assertStored(t, "counter", store.counters, tt.wantCounter)
		})
	}
}

// TestUpdateAccumulatesCounter проверяет, что хендлер отдаёт приращение
// хранилищу, а не замещает им прежнее значение.
func TestUpdateAccumulatesCounter(t *testing.T) {
	store := repository.NewMemStorage()
	h := NewUpdate(store)

	for _, value := range []string{"5", "7"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, newUpdateRequest(models.Counter, "PollCount", value))

		if rec.Code != http.StatusOK {
			t.Fatalf("код ответа на значение %s = %d; ожидался 200", value, rec.Code)
		}
	}

	if v, ok := store.Counter("PollCount"); !ok || v != 12 {
		t.Errorf("PollCount = %v, %v; ожидалось 12, true", v, ok)
	}
}

func assertStored[V comparable](t *testing.T, kind string, got, want map[string]V) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("в хранилище %d метрик типа %s: %v; ожидалось %d: %v", len(got), kind, got, len(want), want)
		return
	}
	for name, wantValue := range want {
		if gotValue, ok := got[name]; !ok || gotValue != wantValue {
			t.Errorf("%s %q = %v, %v; ожидалось %v, true", kind, name, gotValue, ok, wantValue)
		}
	}
}
