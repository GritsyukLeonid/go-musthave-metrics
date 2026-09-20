// Package agent собирает метрики рантайма и отправляет их на сервер.
package agent

import (
	"context"
	"log"
	"time"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
)

// Значения по умолчанию из задания. Флаги и переменные окружения
// появятся в следующих инкрементах и будут перекрывать их.
const (
	DefaultServerURL      = "http://localhost:8080"
	DefaultPollInterval   = 2 * time.Second
	DefaultReportInterval = 10 * time.Second
)

// Sender отправляет метрику на сервер.
//
// Интерфейс объявлен здесь, у потребителя, а не рядом с Client: агенту
// достаточно одного метода, и в тестах на его место встаёт заглушка,
// не поднимающая HTTP.
type Sender interface {
	Send(ctx context.Context, m models.Metrics) error
}

// Config описывает настройки агента. Нулевые поля заменяются значениями
// по умолчанию, поэтому agent.New(agent.Config{}) — рабочий агент.
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
	if cfg.ServerURL == "" {
		cfg.ServerURL = DefaultServerURL
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultPollInterval
	}
	if cfg.ReportInterval <= 0 {
		cfg.ReportInterval = DefaultReportInterval
	}

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
