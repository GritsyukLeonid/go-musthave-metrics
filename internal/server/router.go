// Package server собирает HTTP-маршруты сервиса.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/GritsyukLeonid/go-musthave-metrics/internal/handler"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

func NewRouter(store repository.Repository, logger *zap.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(Logging(logger))
	r.Use(GzipDecompress)
	r.Use(GzipCompress)

	// Старый текстовый API сохраняется для совместимости с предыдущим инкрементом.
	r.Method(http.MethodPost, "/update/{type}/{name}/{value}", handler.NewUpdate(store))
	r.Method(http.MethodGet, "/value/{type}/{name}", handler.NewValue(store))

	// JSON API принимает пути со слешем в конце и без него.
	updateJSON := handler.NewUpdateJSON(store)
	valueJSON := handler.NewValueJSON(store)
	r.Method(http.MethodPost, "/update", updateJSON)
	r.Method(http.MethodPost, "/update/", updateJSON)
	r.Method(http.MethodPost, "/value", valueJSON)
	r.Method(http.MethodPost, "/value/", valueJSON)

	r.Method(http.MethodGet, "/", handler.NewList(store))
	return r
}
