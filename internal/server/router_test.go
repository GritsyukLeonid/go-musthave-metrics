package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
		{
			name:     "главная страница",
			method:   http.MethodGet,
			target:   "/",
			wantCode: http.StatusOK,
		},
		{
			name:     "POST на главную страницу",
			method:   http.MethodPost,
			target:   "/",
			wantCode: http.StatusMethodNotAllowed,
		},
		{
			name:     "значение неизвестной метрики",
			method:   http.MethodGet,
			target:   "/value/gauge/Alloc",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "значение без имени метрики",
			method:   http.MethodGet,
			target:   "/value/gauge",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "лишний сегмент в пути значения",
			method:   http.MethodGet,
			target:   "/value/gauge/Alloc/1",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "POST вместо GET за значением",
			method:   http.MethodPost,
			target:   "/value/gauge/Alloc",
			wantCode: http.StatusMethodNotAllowed,
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

// TestRouterReturnsStoredMetrics — сквозная проверка чтения: принятые
// метрики видны и поимённо в /value, и списком на главной странице.
func TestRouterReturnsStoredMetrics(t *testing.T) {
	store := repository.NewMemStorage()
	srv := httptest.NewServer(NewRouter(store))
	defer srv.Close()

	for _, target := range []string{
		"/update/gauge/Alloc/1.5",
		"/update/gauge/Alloc/2.5",
		"/update/counter/PollCount/5",
		"/update/counter/PollCount/7",
	} {
		res, err := srv.Client().Post(srv.URL+target, "text/plain", http.NoBody)
		if err != nil {
			t.Fatalf("запрос %s не выполнен: %v", target, err)
		}
		res.Body.Close()
	}

	// gauge отдаётся последним значением, counter — суммой приращений.
	for target, want := range map[string]string{
		"/value/gauge/Alloc":       "2.5",
		"/value/counter/PollCount": "12",
	} {
		if got := getBody(t, srv, target, http.StatusOK); got != want {
			t.Errorf("GET %s вернул %q; ожидалось %q", target, got, want)
		}
	}

	page := getBody(t, srv, "/", http.StatusOK)
	for _, want := range []string{"Alloc", "2.5", "PollCount", "12"} {
		if !strings.Contains(page, want) {
			t.Errorf("на главной странице нет %q:\n%s", want, page)
		}
	}
}

// getBody выполняет GET и возвращает тело ответа, проверив код.
func getBody(t *testing.T, srv *httptest.Server, target string, wantCode int) string {
	t.Helper()

	res, err := srv.Client().Get(srv.URL + target)
	if err != nil {
		t.Fatalf("запрос %s не выполнен: %v", target, err)
	}
	defer res.Body.Close()

	if res.StatusCode != wantCode {
		t.Fatalf("GET %s: код ответа = %d; ожидался %d", target, res.StatusCode, wantCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("не удалось прочитать тело ответа на %s: %v", target, err)
	}

	return string(body)
}
