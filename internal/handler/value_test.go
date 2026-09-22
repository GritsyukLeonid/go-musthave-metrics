package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

func newValueRequest(mType, name string) *http.Request {
	return newRequest(http.MethodGet, "/value/"+mType+"/"+name, map[string]string{
		"type": mType,
		"name": name,
	})
}

func TestValueServeHTTP(t *testing.T) {
	// Хранилище одно на все случаи: хендлер только читает.
	store := newStubStore()
	store.UpdateGauge("Alloc", 123.45)
	store.UpdateGauge("HeapSys", 100)
	store.UpdateGauge("TotalAlloc", 12345678901)
	store.UpdateGauge("RandomValue", -0.0015)
	store.UpdateCounter("PollCount", 12)

	tests := []struct {
		name     string
		mType    string
		metric   string
		wantCode int
		wantBody string
	}{
		{
			name:     "дробный gauge",
			mType:    models.Gauge,
			metric:   "Alloc",
			wantCode: http.StatusOK,
			wantBody: "123.45",
		},
		{
			name:     "целый gauge отдаётся без дробной части",
			mType:    models.Gauge,
			metric:   "HeapSys",
			wantCode: http.StatusOK,
			wantBody: "100",
		},
		{
			name:     "большой gauge отдаётся без экспоненты",
			mType:    models.Gauge,
			metric:   "TotalAlloc",
			wantCode: http.StatusOK,
			wantBody: "12345678901",
		},
		{
			name:     "отрицательный gauge",
			mType:    models.Gauge,
			metric:   "RandomValue",
			wantCode: http.StatusOK,
			wantBody: "-0.0015",
		},
		{
			name:     "counter",
			mType:    models.Counter,
			metric:   "PollCount",
			wantCode: http.StatusOK,
			wantBody: "12",
		},
		{
			name:     "неизвестный gauge",
			mType:    models.Gauge,
			metric:   "Unknown",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "неизвестный counter",
			mType:    models.Counter,
			metric:   "Unknown",
			wantCode: http.StatusNotFound,
		},
		{
			// Типы независимы: gauge с таким именем есть, counter — нет.
			name:     "метрика другого типа",
			mType:    models.Counter,
			metric:   "Alloc",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "неизвестный тип",
			mType:    "histogram",
			metric:   "Alloc",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "пустое имя метрики",
			mType:    models.Gauge,
			metric:   "",
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			NewValue(store).ServeHTTP(rec, newValueRequest(tt.mType, tt.metric))

			res := rec.Result()
			defer res.Body.Close()

			if res.StatusCode != tt.wantCode {
				t.Fatalf("код ответа = %d; ожидался %d", res.StatusCode, tt.wantCode)
			}

			if tt.wantCode != http.StatusOK {
				return
			}

			if got := res.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
				t.Errorf("Content-Type = %q; ожидался %q", got, "text/plain; charset=utf-8")
			}
			// Тело сравнивается целиком: клиент сверяет ответ с тем,
			// что отправлял, и лишний перевод строки сломает сравнение.
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("тело ответа = %q; ожидалось %q", got, tt.wantBody)
			}
		})
	}
}

// TestValueReturnsAccumulatedCounter проверяет то, ради чего инкремент
// и добавлен: сервер отдаёт накопленное значение counter, а не последнее
// присланное приращение.
func TestValueReturnsAccumulatedCounter(t *testing.T) {
	store := repository.NewMemStorage()
	update := NewUpdate(store)

	for _, delta := range []string{"5", "7"} {
		rec := httptest.NewRecorder()
		update.ServeHTTP(rec, newUpdateRequest(models.Counter, "PollCount", delta))

		if rec.Code != http.StatusOK {
			t.Fatalf("код ответа на приращение %s = %d; ожидался 200", delta, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	NewValue(store).ServeHTTP(rec, newValueRequest(models.Counter, "PollCount"))

	if rec.Code != http.StatusOK {
		t.Fatalf("код ответа = %d; ожидался 200", rec.Code)
	}
	if got := rec.Body.String(); got != "12" {
		t.Errorf("значение PollCount = %q; ожидалось %q", got, "12")
	}
}
