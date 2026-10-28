package config

import (
	"io"
	"os"
	"strconv"
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
// Причину ошибки и список доступных флагов печатает fail-подход, как в ParseServer.
func ParseAgent(args []string) (Agent, error) {
	return parseAgent(args, os.Stderr)
}

// parseAgent — версия для тестов: вывод задаётся явно, чтобы разбор
// заведомо неверных аргументов не сорил в stderr самого теста.
func parseAgent(args []string, out io.Writer) (Agent, error) {
	fs := newFlagSet("agent", out)

	var cfg Agent
	// Интервалы задаются целыми секундами.
	var report, poll int

	fs.StringVar(&cfg.Address, "a", DefaultAddress, "адрес эндпоинта HTTP-сервера в формате host:port")
	fs.IntVar(&report, "r", int(DefaultReportInterval.Seconds()), "частота отправки метрик на сервер, секунды")
	fs.IntVar(&poll, "p", int(DefaultPollInterval.Seconds()), "частота опроса метрик из пакета runtime, секунды")

	if err := parseArgs(fs, args); err != nil {
		return Agent{}, err
	}

	if value, exists := os.LookupEnv("ADDRESS"); exists {
		cfg.Address = value
	}
	if value, exists := os.LookupEnv("REPORT_INTERVAL"); exists {
		number, err := strconv.Atoi(value)
		if err != nil {
			return Agent{}, fail(fs, "REPORT_INTERVAL must be an integer number of seconds: %v", err)
		}
		report = number
	}
	if value, exists := os.LookupEnv("POLL_INTERVAL"); exists {
		number, err := strconv.Atoi(value)
		if err != nil {
			return Agent{}, fail(fs, "POLL_INTERVAL must be an integer number of seconds: %v", err)
		}
		poll = number
	}

	if cfg.Address == "" {
		return Agent{}, fail(fs, "flag -a: address must not be empty")
	}
	// Нулевой и отрицательный интервалы уронили бы агент в time.NewTicker.
	if report <= 0 {
		return Agent{}, fail(fs, "flag -r / REPORT_INTERVAL: report interval must be positive, got %d", report)
	}
	if poll <= 0 {
		return Agent{}, fail(fs, "flag -p / POLL_INTERVAL: poll interval must be positive, got %d", poll)
	}

	cfg.ReportInterval = time.Duration(report) * time.Second
	cfg.PollInterval = time.Duration(poll) * time.Second

	return cfg, nil
}

// ServerURL переводит адрес из -a в базовый URL для HTTP-клиента.
func (c Agent) ServerURL() string {
	if strings.Contains(c.Address, "://") {
		return c.Address
	}

	return "http://" + c.Address
}
