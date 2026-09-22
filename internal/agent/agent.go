// Package agent собирает метрики рантайма и отправляет их на сервер.
package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
)

// Sender отправляет метрику на сервер.
//
// Интерфейс объявлен здесь, у потребителя, а не рядом с Client: агенту
// достаточно одного метода, и в тестах на его место встаёт заглушка,
// не поднимающая HTTP.
type Sender interface {
	Send(ctx context.Context, m models.Metrics) error
}

// Config описывает настройки агента. Готовый Config собирает пакет
// config: он же разбирает флаги и подставляет значения по умолчанию.
type Config struct {
	ServerURL      string
	PollInterval   time.Duration
	ReportInterval time.Duration
}

// Agent связывает опрос метрик и их отправку.
type Agent struct {
	collector      *Collector
	sender         Sender
	pollInterval   time.Duration
	reportInterval time.Duration
}

// New собирает агент с HTTP-клиентом в качестве отправщика.
func New(cfg Config) *Agent {
	return newAgent(NewCollector(), NewClient(cfg.ServerURL), cfg.PollInterval, cfg.ReportInterval)
}

// newAgent — конструктор для тестов: позволяет подставить свой Sender
// и интервалы, не поднимая HTTP-клиент.
func newAgent(collector *Collector, sender Sender, poll, report time.Duration) *Agent {
	return &Agent{
		collector:      collector,
		sender:         sender,
		pollInterval:   poll,
		reportInterval: report,
	}
}

// Run опрашивает метрики и отправляет их, пока не отменят контекст.
//
// Оба таймера крутятся в одном select, поэтому опрос и отправка никогда
// не выполняются одновременно — коллектору не нужна блокировка.
func (a *Agent) Run(ctx context.Context) error {
	// Интервалы проверяет пакет config, но нулевой Config уронил бы агент
	// паникой внутри time.NewTicker — ошибка понятнее паники.
	if a.pollInterval <= 0 || a.reportInterval <= 0 {
		return fmt.Errorf("agent: intervals must be positive: poll=%s, report=%s", a.pollInterval, a.reportInterval)
	}

	poll := time.NewTicker(a.pollInterval)
	defer poll.Stop()

	report := time.NewTicker(a.reportInterval)
	defer report.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-poll.C:
			a.collector.Poll()

		case <-report.C:
			a.report(ctx)
		}
	}
}

// report отправляет снимок метрик.
//
// Ошибка отправки не останавливает агент: сервер может ещё не подняться
// или перезапускаться. Снимок не подтверждается, и накопленный PollCount
// уедет со следующим отчётом.
func (a *Agent) report(ctx context.Context) {
	metrics := a.collector.Collect()

	for _, m := range metrics {
		if err := a.sender.Send(ctx, m); err != nil {
			// Если сервер недоступен, недоступен он и для остальных метрик:
			// нет смысла ждать таймаут на каждой из двадцати девяти.
			log.Printf("agent: report failed: %v", err)
			return
		}
	}

	a.collector.Ack(metrics)
}
