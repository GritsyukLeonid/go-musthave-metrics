package main

import (
	"log"
	"net/http"

	"github.com/GritsyukLeonid/go-musthave-metrics/internal/handler"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
)

// serverAddr пока зашит константой: флаги и переменные окружения
// появятся в следующих инкрементах.
const serverAddr = ":8080"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run отделён от main, чтобы ошибку можно было вернуть, а не гасить
// процесс из глубины вызовов.
func run() error {
	store := repository.NewMemStorage()

	mux := http.NewServeMux()
	// Метод прямо в шаблоне маршрута: GET на этот путь получит 405,
	// а не «молча» принятую метрику.
	mux.Handle("POST /update/{type}/{name}/{value}", handler.NewUpdate(store))

	log.Printf("metrics server is listening on %s", serverAddr)
	return http.ListenAndServe(serverAddr, mux)
}
