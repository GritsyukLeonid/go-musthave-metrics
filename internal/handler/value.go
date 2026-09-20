package handler

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

// Value отдаёт накопленное значение метрики по URL вида
// /value/<тип>/<имя> в текстовом виде.
type Value struct {
	store repository.Repository
}

func NewValue(store repository.Repository) *Value {
	return &Value{store: store}
}

// ServeHTTP реализует http.Handler.
//
// Коды ответа заданы инкрементом:
//   - 200 — метрика известна, в теле её значение;
//   - 404 — метрика неизвестна.
//
// Неизвестный тип — это тоже 404, а не 400, как в Update: значений такого
// типа сервер не хранит, значит запрошенной метрики он не знает.
func (h *Value) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "type")
	name := chi.URLParam(r, "name")

	var value string

	switch mType {
	case models.Gauge:
		v, ok := h.store.Gauge(name)
		if !ok {
			http.Error(w, "unknown gauge: "+name, http.StatusNotFound)
			return
		}
		value = formatGauge(v)

	case models.Counter:
		v, ok := h.store.Counter(name)
		if !ok {
			http.Error(w, "unknown counter: "+name, http.StatusNotFound)
			return
		}
		value = formatCounter(v)

	default:
		http.Error(w, "unknown metric type: "+mType, http.StatusNotFound)
		return
	}

	// Заголовок выставляем до первой записи в тело — после неё статус
	// и заголовки уже отправлены клиенту.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// Ответ — ровно значение, без перевода строки: клиент сравнивает его
	// с тем, что отправлял, а не разбирает построчно.
	_, _ = io.WriteString(w, value)
}
