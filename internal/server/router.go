// Package server собирает HTTP-маршруты сервиса.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/GritsyukLeonid/go-musthave-metrics/internal/handler"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

// NewRouter возвращает роутер со всеми маршрутами сервера.
//
// Маршрутизация вынесена из main, чтобы тесты поднимали ровно тот же
// роутер, что и прод: метод и форма пути заданы маршрутом, а не проверками
// внутри хендлера, и проверять их надо вместе с ним.
//
// chi выбран из-за именованных сегментов пути и того, что его Router —
// это http.Handler, а хендлеры остаются обычными http.Handler:
// переход с net/http не потребовал переписывать их сигнатуры.
func NewRouter(store repository.Repository) http.Handler {
	r := chi.NewRouter()

	// Метод задан маршрутом: GET на /update получит 405,
	// а не «молча» принятую метрику.
	r.Method(http.MethodPost, "/update/{type}/{name}/{value}", handler.NewUpdate(store))
	r.Method(http.MethodGet, "/value/{type}/{name}", handler.NewValue(store))
	r.Method(http.MethodGet, "/", handler.NewList(store))

	return r
}
