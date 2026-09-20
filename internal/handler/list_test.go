package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newListRequest() *http.Request {
	return newRequest(http.MethodGet, "/", nil)
}

func TestListRendersMetrics(t *testing.T) {
	store := newStubStore()
	store.UpdateGauge("HeapSys", 100)
	store.UpdateGauge("Alloc", 123.45)
	store.UpdateCounter("PollCount", 12)

	rec := httptest.NewRecorder()
	NewList(store).ServeHTTP(rec, newListRequest())

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("код ответа = %d; ожидался 200", res.StatusCode)
	}
	if got := res.Header.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q; ожидался %q", got, "text/html; charset=utf-8")
	}

	body := rec.Body.String()

	if !strings.HasPrefix(body, "<!DOCTYPE html>") {
		t.Errorf("тело ответа начинается с %q; ожидался HTML-документ", firstLine(body))
	}

	// Имя без значения — бесполезная страница, поэтому проверяется пара.
	for _, want := range []string{"Alloc", "123.45", "HeapSys", "100", "PollCount", "12"} {
		if !strings.Contains(body, want) {
			t.Errorf("на странице нет %q:\n%s", want, body)
		}
	}

	// Порядок фиксирован: страница не должна перетасовываться между
	// обновлениями из-за случайного порядка обхода карты.
	if strings.Index(body, "Alloc") > strings.Index(body, "HeapSys") {
		t.Errorf("метрики выведены не по алфавиту:\n%s", body)
	}
}

func TestListEmptyStorage(t *testing.T) {
	rec := httptest.NewRecorder()
	NewList(newStubStore()).ServeHTTP(rec, newListRequest())

	if rec.Code != http.StatusOK {
		t.Fatalf("код ответа = %d; ожидался 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "ни одной метрики") {
		t.Errorf("пустое хранилище не отражено на странице:\n%s", body)
	}
}

// TestListEscapesMetricName: имя метрики приходит из запроса, а не из
// констант, и не должно доехать до браузера как разметка.
func TestListEscapesMetricName(t *testing.T) {
	store := newStubStore()
	store.UpdateGauge("<script>alert(1)</script>", 1)

	rec := httptest.NewRecorder()
	NewList(store).ServeHTTP(rec, newListRequest())

	if body := rec.Body.String(); strings.Contains(body, "<script>") {
		t.Errorf("имя метрики попало на страницу неэкранированным:\n%s", body)
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
