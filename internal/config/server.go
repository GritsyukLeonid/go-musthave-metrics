package config

import (
	"io"
	"os"
	"strconv"
	"time"
)

// Server — настройки HTTP-сервера.
type Server struct {
	// Address — адрес эндпоинта в формате host:port, флаг -a.
	Address string
	// StoreInterval — интервал сохранения метрик; ноль означает синхронную запись.
	StoreInterval time.Duration
	// FileStoragePath — путь к файлу сохранения метрик.
	FileStoragePath string
	// Restore определяет, нужно ли восстанавливать метрики при запуске.
	Restore bool
}

// ParseServer разбирает аргументы сервера, обычно os.Args[1:].
func ParseServer(args []string) (Server, error) {
	return parseServer(args, os.Stderr)
}

// parseServer — версия для тестов: вывод задаётся явно.
func parseServer(args []string, out io.Writer) (Server, error) {
	fs := newFlagSet("server", out)

	var cfg Server
	var storeInterval int
	fs.StringVar(&cfg.Address, "a", DefaultAddress, "адрес эндпоинта HTTP-сервера в формате host:port")
	fs.IntVar(&storeInterval, "i", 300, "интервал сохранения метрик в секундах; 0 — синхронно")
	fs.StringVar(&cfg.FileStoragePath, "f", "/tmp/metrics-db.json", "путь к файлу сохранения метрик")
	fs.BoolVar(&cfg.Restore, "r", true, "восстанавливать метрики из файла при запуске")

	if err := parseArgs(fs, args); err != nil {
		return Server{}, err
	}

	if value, exists := os.LookupEnv("ADDRESS"); exists {
		cfg.Address = value
	}
	if value, exists := os.LookupEnv("STORE_INTERVAL"); exists {
		number, err := strconv.Atoi(value)
		if err != nil {
			return Server{}, fail(fs, "STORE_INTERVAL must be an integer number of seconds: %v", err)
		}
		storeInterval = number
	}
	if value, exists := os.LookupEnv("FILE_STORAGE_PATH"); exists {
		cfg.FileStoragePath = value
	}
	if value, exists := os.LookupEnv("RESTORE"); exists {
		restore, err := strconv.ParseBool(value)
		if err != nil {
			return Server{}, fail(fs, "RESTORE must be a boolean: %v", err)
		}
		cfg.Restore = restore
	}

	if cfg.Address == "" {
		return Server{}, fail(fs, "flag -a: address must not be empty")
	}
	if storeInterval < 0 {
		return Server{}, fail(fs, "flag -i: store interval must not be negative, got %d", storeInterval)
	}
	if cfg.FileStoragePath == "" {
		return Server{}, fail(fs, "flag -f: file storage path must not be empty")
	}

	cfg.StoreInterval = time.Duration(storeInterval) * time.Second
	return cfg, nil
}
