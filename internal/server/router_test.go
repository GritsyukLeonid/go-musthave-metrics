package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

// TestRouterRouting проверяет то, что задано шаблоном маршрута, а не кодом
// хендлера: чужой метод и путь неправильной формы не должны доходить
// до обработки метрики.
func TestRouterRouting(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		target   string
		wantCode int
	}{
		{
			name:     "корректный запрос",
			method:   http.MethodPost,
			target:   "/update/gauge/Alloc/123.45",
			wantCode: http.StatusOK,
		},
		{
			name:     "GET вместо POST",
			method:   http.MethodGet,
			target:   "/update/gauge/Alloc/123.45",
			wantCode: http.StatusMethodNotAllowed,
		},
		{
			name:     "запрос без значения",
			method:   http.MethodPost,
			target:   "/update/gauge/Alloc",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "запрос без имени метрики",
			method:   http.MethodPost,
			target:   "/update/gauge/",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "лишний сегмент в пути",
			method:   http.MethodPost,
			target:   "/update/gauge/Alloc/1/2",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "неизвестный маршрут",
			method:   http.MethodPost,
			target:   "/updates/gauge/Alloc/1",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "неизвестный тип метрики",
			method:   http.MethodPost,
			target:   "/update/histogram/Alloc/1",
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(repository.NewMemStorage())

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, http.NoBody))

			if rec.Code != tt.wantCode {
				t.Errorf("%s %s: код ответа = %d; ожидался %d", tt.method, tt.target, rec.Code, tt.wantCode)
			}
		})
	}
}

// TestRouterStoresMetrics — сквозная проверка связки роутер → хендлер →
// хранилище: значения из URL доезжают до хранилища с нужной семантикой.
func TestRouterStoresMetrics(t *testing.T) {
	store := repository.NewMemStorage()
	srv := httptest.NewServer(NewRouter(store))
	defer srv.Close()

	targets := []string{
		"/update/gauge/Alloc/1.5",
		"/update/gauge/Alloc/2.5",
		"/update/counter/PollCount/5",
		"/update/counter/PollCount/7",
	}

	for _, target := range targets {
		req, err := http.NewRequest(http.MethodPost, srv.URL+target, http.NoBody)
		if err != nil {
			t.Fatalf("не удалось собрать запрос %s: %v", target, err)
		}
		req.Header.Set("Content-Type", "text/plain")

		res, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("запрос %s не выполнен: %v", target, err)
		}
		res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Fatalf("POST %s: код ответа = %d; ожидался 200", target, res.StatusCode)
		}
	}

	// gauge хранит последнее значение.
	if v, ok := store.Gauge("Alloc"); !ok || v != 2.5 {
		t.Errorf("Alloc = %v, %v; ожидалось 2.5, true", v, ok)
	}
	// counter — сумму приращений.
	if v, ok := store.Counter("PollCount"); !ok || v != 12 {
		t.Errorf("PollCount = %v, %v; ожидалось 12, true", v, ok)
	}
}
