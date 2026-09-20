package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/GritsyukLeonid/go-musthave-metrics/internal/agent"
	"github.com/GritsyukLeonid/go-musthave-metrics/internal/config"
)

// exitUsage — код возврата при неверных аргументах, такой же,
// как у flag.ExitOnError.
const exitUsage = 2

func main() {
	cfg, err := config.ParseAgent(os.Args[1:])
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
func run(cfg config.Agent) error {
	// По Ctrl+C и SIGTERM контекст отменяется, Run выходит из цикла,
	// а процесс завершается с нулевым кодом, а не умирает по сигналу.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Флаг -a задаётся адресом host:port, а клиенту нужен базовый URL.
	serverURL := cfg.ServerURL()

	log.Printf("metrics agent reports to %s every %s (poll every %s)",
		serverURL, cfg.ReportInterval, cfg.PollInterval)

	a := agent.New(agent.Config{
		ServerURL:      serverURL,
		PollInterval:   cfg.PollInterval,
		ReportInterval: cfg.ReportInterval,
	})

	// Отмена контекста — штатное завершение, а не ошибка запуска.
	if err := a.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
