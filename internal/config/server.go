package config

import (
	"io"
	"os"
)

// Server — настройки HTTP-сервера.
type Server struct {
	// Address — адрес эндпоинта в формате host:port, флаг -a.
	Address string
}

// ParseServer разбирает аргументы сервера, обычно os.Args[1:].
//
// Причину ошибки и список доступных флагов печатает в os.Stderr сам
// flag.FlagSet, поэтому вызывающему остаётся завершить процесс.
func ParseServer(args []string) (Server, error) {
	return parseServer(args, os.Stderr)
}

// parseServer — версия для тестов: вывод задаётся явно, чтобы разбор
// заведомо неверных аргументов не сорил в stderr самого теста.
func parseServer(args []string, out io.Writer) (Server, error) {
	fs := newFlagSet("server", out)

	var cfg Server
	fs.StringVar(&cfg.Address, "a", DefaultAddress, "адрес эндпоинта HTTP-сервера в формате host:port")

	if err := parseArgs(fs, args); err != nil {
		return Server{}, err
	}

	if cfg.Address == "" {
		return Server{}, fail(fs, "flag -a: address must not be empty")
	}

	return cfg, nil
}
