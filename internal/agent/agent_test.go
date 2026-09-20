package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
)

var errSendFailed = errors.New("сервер недоступен")

// stubSender заменяет HTTP-клиент: запоминает отправленное и умеет
// «ломаться» по требованию теста. Run работает в своей горутине,
// поэтому доступ к полям закрыт мьютексом — иначе -race поймает гонку.
type stubSender struct {
	mu   sync.Mutex
	sent []models.Metrics
	err  error

	// reports получает сигнал после каждого полного отчёта: PollCount
	// идёт в снимке последним, значит, дошло всё остальное.
	reports chan struct{}
}

func newStubSender() *stubSender {
	return &stubSender{reports: make(chan struct{}, 1)}
}

func (s *stubSender) Send(_ context.Context, m models.Metrics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err != nil {
		return s.err
	}

	s.sent = append(s.sent, m)
	if m.ID == PollCount {
		// Неблокирующая отправка: тест может ещё не ждать сигнал,
		// а агент не должен на нём вставать.
		select {
		case s.reports <- struct{}{}:
		default:
		}
	}

	return nil
}

func (s *stubSender) setErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.err = err
}

func (s *stubSender) snapshot() []models.Metrics {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]models.Metrics(nil), s.sent...)
}

func TestNewAppliesDefaults(t *testing.T) {
	a := New(Config{})

	if a.pollInterval != DefaultPollInterval {
		t.Errorf("pollInterval = %s; ожидался %s", a.pollInterval, DefaultPollInterval)
	}
	if a.reportInterval != DefaultReportInterval {
		t.Errorf("reportInterval = %s; ожидался %s", a.reportInterval, DefaultReportInterval)
	}

	custom := New(Config{ServerURL: "http://example.com", PollInterval: time.Second, ReportInterval: time.Minute})
	if custom.pollInterval != time.Second || custom.reportInterval != time.Minute {
		t.Errorf("заданные интервалы (%s, %s) не сохранились", custom.pollInterval, custom.reportInterval)
	}
}

// TestAgentRunReportsAllMetrics — сквозная проверка цикла: за отведённое
// время агент успевает опросить рантайм и отправить весь набор метрик.
func TestAgentRunReportsAllMetrics(t *testing.T) {
	sender := newStubSender()
	a := newAgent(newTestCollector(), sender, time.Millisecond, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	select {
	case <-sender.reports:
	case <-time.After(5 * time.Second):
		t.Fatal("агент не отправил ни одного отчёта за 5 секунд")
	}

	cancel()

	select {
	case err := <-done:
		// Отмена контекста — штатная остановка, и Run обязан о ней сообщить,
		// а не вернуть nil: main отличает её от настоящей ошибки.
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run() вернул %v; ожидалось context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() не завершился после отмены контекста")
	}

	sent := sender.snapshot()
	seen := make(map[string]models.Metrics, len(sent))
	for _, m := range sent {
		seen[m.ID] = m
	}

	// Список из задания: 27 метрик runtime плюс RandomValue и PollCount.
	want := []string{
		"Alloc", "BuckHashSys", "Frees", "GCCPUFraction", "GCSys", "HeapAlloc",
		"HeapIdle", "HeapInuse", "HeapObjects", "HeapReleased", "HeapSys",
		"LastGC", "Lookups", "MCacheInuse", "MCacheSys", "MSpanInuse", "MSpanSys",
		"Mallocs", "NextGC", "NumForcedGC", "NumGC", "OtherSys", "PauseTotalNs",
		"StackInuse", "StackSys", "Sys", "TotalAlloc", RandomValue, PollCount,
	}
	for _, name := range want {
		if _, ok := seen[name]; !ok {
			t.Errorf("метрика %s не отправлена", name)
		}
	}

	if poll := seen[PollCount]; poll.Delta == nil || *poll.Delta < 1 {
		t.Errorf("%s = %v; ожидалось значение не меньше 1", PollCount, poll.Delta)
	}
}

func TestAgentRunStopsOnCanceledContext(t *testing.T) {
	// Интервалы большие: единственная причина выйти из цикла — контекст.
	a := newAgent(newTestCollector(), newStubSender(), time.Hour, time.Hour)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run() вернул %v; ожидалось context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run() не завершился с отменённым контекстом")
	}
}

// TestAgentReportAcksOnlySuccessfulReports описывает поведение при
// недоступном сервере: упавший отчёт не подтверждается, опросы
// не теряются и уезжают следующим отчётом.
func TestAgentReportAcksOnlySuccessfulReports(t *testing.T) {
	collector := newTestCollector()
	sender := newStubSender()
	sender.setErr(errSendFailed)

	a := newAgent(collector, sender, time.Hour, time.Hour)

	collector.Poll()
	collector.Poll()
	a.report(t.Context())

	if n := len(sender.snapshot()); n != 0 {
		t.Errorf("при недоступном сервере запомнено %d метрик; ожидалось 0", n)
	}

	sender.setErr(nil)
	collector.Poll()
	a.report(t.Context())

	if got := pollCountOf(t, sender.snapshot()); got != 3 {
		t.Errorf("%s в успешном отчёте = %d; ожидалось 3 (два опроса до сбоя и один после)", PollCount, got)
	}

	// После подтверждения счётчик обнулён: сервер уже сложил эти три опроса.
	if got := pollCountOf(t, collector.Collect()); got != 0 {
		t.Errorf("%s после успешного отчёта = %d; ожидалось 0", PollCount, got)
	}
}

// TestAgentReportStopsAfterFirstError: если сервер не ответил на первую
// метрику, он не ответит и на остальные, а таймаут на каждой из двадцати
// девяти занял бы больше, чем весь reportInterval.
func TestAgentReportStopsAfterFirstError(t *testing.T) {
	var attempts int
	failing := senderFunc(func(context.Context, models.Metrics) error {
		attempts++
		return errSendFailed
	})

	a := newAgent(newTestCollector(), failing, time.Hour, time.Hour)
	a.collector.Poll()
	a.report(t.Context())

	if attempts != 1 {
		t.Errorf("после ошибки выполнено %d попыток отправки; ожидалась 1", attempts)
	}
}

// TestAgentRunKeepsGoingAfterError: недоступный сервер не должен ронять
// агент — метрики продолжат уходить, когда сервер поднимется.
func TestAgentRunKeepsGoingAfterError(t *testing.T) {
	sender := newStubSender()
	sender.setErr(errSendFailed)

	a := newAgent(newTestCollector(), sender, time.Millisecond, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	// Сервер «поднимается» после нескольких неудачных отчётов.
	time.Sleep(20 * time.Millisecond)
	sender.setErr(nil)

	select {
	case <-sender.reports:
	case <-time.After(5 * time.Second):
		t.Fatal("агент не отправил отчёт после восстановления сервера")
	}

	cancel()
	<-done
}

// senderFunc позволяет подставить отправщика одной функцией,
// когда полноценная заглушка тесту не нужна.
type senderFunc func(ctx context.Context, m models.Metrics) error

func (f senderFunc) Send(ctx context.Context, m models.Metrics) error { return f(ctx, m) }

var _ Sender = senderFunc(nil)
