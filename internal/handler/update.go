package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

type Update struct {
	store repository.Repository
}

func NewUpdate(store repository.Repository) *Update {
	return &Update{store: store}
}

func (h *Update) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "type")
	name := chi.URLParam(r, "name")
	rawValue := chi.URLParam(r, "value")

	if name == "" {
		http.Error(w, "metric name is required", http.StatusNotFound)
		return
	}

	switch mType {
	case models.Gauge:
		value, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			http.Error(w, "gauge value must be a float64: "+rawValue, http.StatusBadRequest)
			return
		}
		h.store.UpdateGauge(name, value)
	case models.Counter:
		delta, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			http.Error(w, "counter value must be an int64: "+rawValue, http.StatusBadRequest)
			return
		}
		h.store.UpdateCounter(name, delta)
	default:
		http.Error(w, "unknown metric type: "+mType, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}
