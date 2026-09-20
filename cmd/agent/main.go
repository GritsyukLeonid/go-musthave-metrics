package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/GritsyukLeonid/go-musthave-metrics/internal/agent"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run отделён от main, чтобы ошибку можно было вернуть, а не гасить
// процесс из глубины вызовов.
func run() error {
	// Адрес и интервалы пока берутся из значений по умолчанию:
	// флаги и переменные окружения появятся в следующих инкрементах.
	cfg := agent.Config{
		ServerURL:      agent.DefaultServerURL,
		PollInterval:   agent.DefaultPollInterval,
		ReportInterval: agent.DefaultReportInterval,
	}

	// По Ctrl+C и SIGTERM контекст отменяется, Run выходит из цикла,
	// а процесс завершается с нулевым кодом, а не умирает по сигналу.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("metrics agent reports to %s every %s (poll every %s)",
		cfg.ServerURL, cfg.ReportInterval, cfg.PollInterval)

	// Отмена контекста — штатное завершение, а не ошибка запуска.
	if err := agent.New(cfg).Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
