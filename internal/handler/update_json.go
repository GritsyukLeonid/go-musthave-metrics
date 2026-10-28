package handler

import (
	"encoding/json"
	"net/http"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

type UpdateJSON struct {
	store repository.Repository
}

func NewUpdateJSON(store repository.Repository) *UpdateJSON {
	return &UpdateJSON{store: store}
}

func (h *UpdateJSON) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		if metric.Value == nil {
			http.Error(w, "gauge value is required", http.StatusBadRequest)
			return
		}
		h.store.UpdateGauge(metric.ID, *metric.Value)
	case models.Counter:
		if metric.Delta == nil {
			http.Error(w, "counter delta is required", http.StatusBadRequest)
			return
		}
		h.store.UpdateCounter(metric.ID, *metric.Delta)
	default:
		http.Error(w, "unknown metric type: "+metric.MType, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
