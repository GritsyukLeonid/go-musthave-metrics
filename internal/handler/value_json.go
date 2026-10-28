package handler

import (
	"encoding/json"
	"net/http"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

type ValueJSON struct {
	store repository.Repository
}

func NewValueJSON(store repository.Repository) *ValueJSON {
	return &ValueJSON{store: store}
}

func (h *ValueJSON) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var metric models.Metrics
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if metric.ID == "" {
		http.Error(w, "metric name is required", http.StatusNotFound)
		return
	}

	switch metric.MType {
	case models.Gauge:
		value, ok := h.store.Gauge(metric.ID)
		if !ok {
			http.Error(w, "unknown gauge: "+metric.ID, http.StatusNotFound)
			return
		}
		metric.Value = &value
		metric.Delta = nil
	case models.Counter:
		delta, ok := h.store.Counter(metric.ID)
		if !ok {
			http.Error(w, "unknown counter: "+metric.ID, http.StatusNotFound)
			return
		}
		metric.Delta = &delta
		metric.Value = nil
	default:
		http.Error(w, "unknown metric type: "+metric.MType, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metric); err != nil {
		return
	}
}
