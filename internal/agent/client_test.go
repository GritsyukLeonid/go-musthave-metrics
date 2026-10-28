package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
)

func gauge(name string, value float64) models.Metrics {
	return models.Metrics{ID: name, MType: models.Gauge, Value: &value}
}

func counter(name string, delta int64) models.Metrics {
	return models.Metrics{ID: name, MType: models.Counter, Delta: &delta}
}

// capturedRequest — то, что сервер увидел от агента.
type capturedRequest struct {
	method          string
	path            string
	query           string
	contentType     string
	contentEncoding string
	acceptEncoding  string
	body            string
}

// newCapturingServer поднимает сервер, запоминающий последний запрос.
// Send возвращается только после ответа, поэтому читать capturedRequest
// из теста безопасно.
func newCapturingServer(t *testing.T, status int) (*httptest.Server, *capturedRequest) {
	t.Helper()

	var got capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("Content-Encoding") == "gzip" {
			zr, err := gzip.NewReader(bytes.NewReader(body))
			if err == nil {
				body, _ = io.ReadAll(zr)
				_ = zr.Close()
			}
		}
		got = capturedRequest{
			method:          r.Method,
			path:            r.URL.Path,
			query:           r.URL.RawQuery,
			contentType:     r.Header.Get("Content-Type"),
			contentEncoding: r.Header.Get("Content-Encoding"),
			acceptEncoding:  r.Header.Get("Accept-Encoding"),
			body:            string(body),
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)

	return srv, &got
}

func TestClientSend(t *testing.T) {
	tests := []struct {
		name     string
		metric   models.Metrics
		wantPath string
	}{
		{
			name:     "gauge с дробным значением",
			metric:   gauge("Alloc", 123.45),
			wantPath: "/update/",
		},
		{
			name:     "нулевой gauge",
			metric:   gauge("HeapIdle", 0),
			wantPath: "/update/",
		},
		{
			name:     "отрицательный gauge",
			metric:   gauge("RandomValue", -1.5),
			wantPath: "/update/",
		},
		{
			name: "большой gauge уходит без экспоненты",
			// Значения MemStats — это байты, у них легко получается
			// порядок, на котором %v перешёл бы на запись вида 1.2e+10.
			metric:   gauge("TotalAlloc", 12345678901),
			wantPath: "/update/",
		},
		{
			name:     "counter",
			metric:   counter(PollCount, 5),
			wantPath: "/update/",
		},
		{
			name:     "отрицательный counter",
			metric:   counter(PollCount, -3),
			wantPath: "/update/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, got := newCapturingServer(t, http.StatusOK)

			if err := NewClient(srv.URL).Send(t.Context(), tt.metric); err != nil {
				t.Fatalf("Send() вернул ошибку: %v", err)
			}

			if got.method != http.MethodPost {
				t.Errorf("метод запроса = %s; ожидался POST", got.method)
			}
			if got.path != tt.wantPath {
				t.Errorf("путь запроса = %s; ожидался %s", got.path, tt.wantPath)
			}
			if got.query != "" {
				t.Errorf("в запросе появилась строка запроса %q; ожидалась пустая", got.query)
			}
			if got.contentType != "application/json" {
				t.Errorf("Content-Type = %q; ожидался %q", got.contentType, "application/json")
			}
			if got.contentEncoding != "gzip" {
				t.Errorf("Content-Encoding = %q; ожидался gzip", got.contentEncoding)
			}
			if !strings.Contains(got.acceptEncoding, "gzip") {
				t.Errorf("Accept-Encoding = %q; ожидалось gzip", got.acceptEncoding)
			}
			var sent models.Metrics
			if err := json.Unmarshal([]byte(got.body), &sent); err != nil {
				t.Fatalf("тело запроса не является JSON: %v", err)
			}
			if sent.ID != tt.metric.ID || sent.MType != tt.metric.MType {
				t.Errorf("отправлена метрика %+v; ожидалась %+v", sent, tt.metric)
			}
		})
	}
}

// TestClientSendTrimsTrailingSlash: хвостовой слэш в адресе сервера дал бы
// путь "//update/...", который сервер отправит в редирект вместо обработки.
func TestClientSendTrimsTrailingSlash(t *testing.T) {
	srv, got := newCapturingServer(t, http.StatusOK)

	if err := NewClient(srv.URL + "/").Send(t.Context(), gauge("Alloc", 1)); err != nil {
		t.Fatalf("Send() вернул ошибку: %v", err)
	}

	if got.path != "/update/" {
		t.Errorf("путь запроса = %s; ожидался /update/", got.path)
	}
}

func TestClientSendUnexpectedStatus(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError} {
		srv, _ := newCapturingServer(t, status)

		if err := NewClient(srv.URL).Send(t.Context(), gauge("Alloc", 1)); err == nil {
			t.Errorf("Send() при ответе %d вернул nil; ожидалась ошибка", status)
		}
	}
}

// TestClientSendInvalidMetric проверяет, что негодная метрика отсекается
// до запроса: слать серверу "/update/gauge/Alloc/" бессмысленно.
func TestClientSendInvalidMetric(t *testing.T) {
	tests := []struct {
		name   string
		metric models.Metrics
	}{
		{
			name:   "gauge без значения",
			metric: models.Metrics{ID: "Alloc", MType: models.Gauge},
		},
		{
			name:   "counter без приращения",
			metric: models.Metrics{ID: PollCount, MType: models.Counter},
		},
		{
			name:   "неизвестный тип",
			metric: models.Metrics{ID: "Alloc", MType: "histogram"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
			}))
			defer srv.Close()

			if err := NewClient(srv.URL).Send(t.Context(), tt.metric); err == nil {
				t.Error("Send() вернул nil; ожидалась ошибка")
			}
			if n := requests.Load(); n != 0 {
				t.Errorf("на сервер ушло %d запросов; ожидалось 0", n)
			}
		})
	}
}

func TestClientSendServerUnavailable(t *testing.T) {
	srv, _ := newCapturingServer(t, http.StatusOK)
	url := srv.URL
	srv.Close()

	if err := NewClient(url).Send(t.Context(), gauge("Alloc", 1)); err == nil {
		t.Error("Send() на закрытый сервер вернул nil; ожидалась ошибка")
	}
}

// TestClientSendRespectsContext: контекст агента отменяется при остановке,
// и зависшая отправка не должна держать процесс.
func TestClientSendRespectsContext(t *testing.T) {
	srv, _ := newCapturingServer(t, http.StatusOK)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := NewClient(srv.URL).Send(ctx, gauge("Alloc", 1)); err == nil {
		t.Error("Send() с отменённым контекстом вернул nil; ожидалась ошибка")
	}
}
