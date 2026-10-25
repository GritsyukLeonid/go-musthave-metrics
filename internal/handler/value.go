package handler

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

type Value struct {
	store repository.Repository
}

func NewValue(store repository.Repository) *Value {
	return &Value{store: store}
}

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

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, value)
}
