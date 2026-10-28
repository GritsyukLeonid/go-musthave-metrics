package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/GritsyukLeonid/go-musthave-metrics/internal/config"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/repository"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/server"
)

// exitUsage — код возврата при неверных аргументах.
const exitUsage = 2

func main() {
	cfg, err := config.ParseServer(os.Args[1:])
	if err != nil {
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
	fileStore := repository.NewFileStorage(store, cfg.FileStoragePath, cfg.StoreInterval)
	if cfg.Restore {
		if err := fileStore.Restore(); err != nil {
			return err
		}
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	defer logger.Sync()

	httpServer := &http.Server{
		Addr:    cfg.Address,
		Handler: server.NewRouter(fileStore, logger),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go fileStore.Run(ctx)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}

	// Сохраняем актуальное состояние после остановки обработчиков, не полагаясь
	// на то, успел ли наступить очередной тик интервального сохранения.
	if err := fileStore.Save(); err != nil {
		return err
	}
	return nil
}
