package main

import (
	"log"
	"net/http"

	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/server"
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

	log.Printf("metrics server is listening on %s", serverAddr)
	return http.ListenAndServe(serverAddr, server.NewRouter(store))
}
