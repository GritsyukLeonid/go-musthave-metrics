package config

import (
	"io"
	"os"
	"strings"
	"time"
)

// Agent — настройки агента.
type Agent struct {
	// Address — адрес эндпоинта HTTP-сервера в формате host:port, флаг -a.
	Address string
	// ReportInterval — частота отправки метрик на сервер, флаг -r.
	ReportInterval time.Duration
	// PollInterval — частота опроса метрик из пакета runtime, флаг -p.
	PollInterval time.Duration
}

// ParseAgent разбирает аргументы агента, обычно os.Args[1:].
//
// Причину ошибки и список доступных флагов печатает в os.Stderr сам
// flag.FlagSet, поэтому вызывающему остаётся завершить процесс.
func ParseAgent(args []string) (Agent, error) {
	return parseAgent(args, os.Stderr)
}

// parseAgent — версия для тестов: вывод задаётся явно, чтобы разбор
// заведомо неверных аргументов не сорил в stderr самого теста.
func parseAgent(args []string, out io.Writer) (Agent, error) {
	fs := newFlagSet("agent", out)

	var cfg Agent
	// Интервалы по заданию задаются целыми секундами, поэтому флаги
	// читаются как int: time.Duration принял бы «-r=10» за десять
	// наносекунд, а «10s» задание не предполагает.
	var report, poll int

	fs.StringVar(&cfg.Address, "a", DefaultAddress, "адрес эндпоинта HTTP-сервера в формате host:port")
	fs.IntVar(&report, "r", int(DefaultReportInterval.Seconds()), "частота отправки метрик на сервер, секунды")
	fs.IntVar(&poll, "p", int(DefaultPollInterval.Seconds()), "частота опроса метрик из пакета runtime, секунды")

	if err := parseArgs(fs, args); err != nil {
		return Agent{}, err
	}

	if cfg.Address == "" {
		return Agent{}, fail(fs, "flag -a: address must not be empty")
	}
	// Нулевой и отрицательный интервалы уронили бы агент в time.NewTicker,
	// поэтому они отсекаются здесь, а не в рабочем цикле.
	if report <= 0 {
		return Agent{}, fail(fs, "flag -r: report interval must be positive, got %d", report)
	}
	if poll <= 0 {
		return Agent{}, fail(fs, "flag -p: poll interval must be positive, got %d", poll)
	}

	cfg.ReportInterval = time.Duration(report) * time.Second
	cfg.PollInterval = time.Duration(poll) * time.Second

	return cfg, nil
}

// ServerURL переводит адрес из -a в базовый URL для HTTP-клиента.
//
// Флаг задаётся без схемы («localhost:8080»), а клиенту нужен абсолютный
// адрес, иначе запрос уйдёт по относительному пути в никуда. Схему в -a
// передавать не обязательно, но если её передали — она сохраняется.
func (c Agent) ServerURL() string {
	if strings.Contains(c.Address, "://") {
		return c.Address
	}

	return "http://" + c.Address
}
