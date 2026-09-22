// Package config собирает настройки сервера и агента из аргументов
// командной строки.
//
// Пакет держит значения по умолчанию из задания и отвечает за разбор
// флагов целиком: сервер и агент получают уже проверенную конфигурацию
// и не занимаются аргументами сами.
package config

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

// Значения по умолчанию из задания. Интервалы задаются флагами в секундах,
// но внутри программы живут как time.Duration: так их нельзя перепутать
// с любым другим числом.
const (
	DefaultAddress        = "localhost:8080"
	DefaultPollInterval   = 2 * time.Second
	DefaultReportInterval = 10 * time.Second
)

// newFlagSet создаёт набор флагов, который возвращает ошибку разбора
// вызывающему, а не завершает процесс из недр пакета flag: так разбор
// проверяется тестами, а выход остаётся делом main.
//
// Сообщение об ошибке и список флагов печатает сам FlagSet — в out,
// то есть в os.Stderr для рабочих сборок и в io.Discard для тестов.
func newFlagSet(name string, out io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(out)

	return fs
}

// parseArgs разбирает аргументы и отвергает всё, что осталось за флагами.
//
// Сам flag молча складывает такие аргументы в fs.Args() и продолжает
// работу, а по заданию приложение с неизвестным аргументом должно
// завершаться с ошибкой.
func parseArgs(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() > 0 {
		return fail(fs, "unknown arguments: %s", strings.Join(fs.Args(), " "))
	}

	return nil
}

// fail сообщает об ошибке так же, как это делает flag при разборе
// аргументов: печатает причину и список флагов, а ошибку отдаёт наверх.
func fail(fs *flag.FlagSet, format string, args ...any) error {
	err := fmt.Errorf(format, args...)

	fmt.Fprintln(fs.Output(), err)
	fs.Usage()

	return err
}
