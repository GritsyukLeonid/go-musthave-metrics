package main

import (
	"log"
	"net/http"
	"os"

	"github.com/GritsyukLeonid/go-musthave-metrics/internal/config"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/server"
)

// exitUsage — код возврата при неверных аргументах, такой же,
// как у flag.ExitOnError.
const exitUsage = 2

func main() {
	cfg, err := config.ParseServer(os.Args[1:])
	if err != nil {
		// Причину и список флагов уже напечатал flag.FlagSet: повторять
		// сообщение незачем, остаётся выйти с ненулевым кодом.
		os.Exit(exitUsage)
	}

	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

// run отделён от main, чтобы ошибку можно было вернуть, а не гасить
// процесс из глубины вызовов.
func run(cfg config.Server) error {
	store := repository.NewMemStorage()

	log.Printf("metrics server is listening on %s", cfg.Address)
	return http.ListenAndServe(cfg.Address, server.NewRouter(store))
}
