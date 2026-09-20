// Package server собирает HTTP-маршруты сервиса.
package server

import (
	"net/http"

	"github.com/GritsyukLeonid/go-musthave-metrics/internal/handler"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

// NewRouter возвращает роутер со всеми маршрутами сервера.
//
// Маршрутизация вынесена из main, чтобы тесты поднимали ровно тот же
// роутер, что и прод: метод и форма пути заданы шаблоном маршрута,
// а не проверками внутри хендлера, и проверять их надо вместе с ним.
func NewRouter(store repository.Repository) http.Handler {
	mux := http.NewServeMux()

	// Метод прямо в шаблоне: GET на этот путь получит 405,
	// а не «молча» принятую метрику.
	mux.Handle("POST /update/{type}/{name}/{value}", handler.NewUpdate(store))

	return mux
}
