package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
)

// FileStorage добавляет сохранение и восстановление к существующему хранилищу.
type FileStorage struct {
	store         Repository
	path          string
	storeInterval time.Duration
	saveMu        sync.Mutex
}

// NewFileStorage создаёт хранилище, использующее файл для персистентности.
func NewFileStorage(store Repository, path string, storeInterval time.Duration) *FileStorage {
	return &FileStorage{store: store, path: path, storeInterval: storeInterval}
}

// Restore загружает метрики из файла. Отсутствующий файл считается первым запуском.
func (s *FileStorage) Restore() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read metrics file: %w", err)
	}

	var metrics []models.Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return fmt.Errorf("decode metrics file: %w", err)
	}

	for _, metric := range metrics {
		switch metric.MType {
		case models.Gauge:
			if metric.ID == "" || metric.Value == nil {
				return fmt.Errorf("invalid gauge metric in storage file: %q", metric.ID)
			}
			s.store.UpdateGauge(metric.ID, *metric.Value)
		case models.Counter:
			if metric.ID == "" || metric.Delta == nil {
				return fmt.Errorf("invalid counter metric in storage file: %q", metric.ID)
			}
			s.store.UpdateCounter(metric.ID, *metric.Delta)
		default:
			return fmt.Errorf("unknown metric type %q in storage file", metric.MType)
		}
	}
	return nil
}

// Save атомарно записывает текущие значения метрик в JSON-файл.
func (s *FileStorage) Save() error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	metrics := make([]models.Metrics, 0)
	for name, value := range s.store.Gauges() {
		v := value
		metrics = append(metrics, models.Metrics{ID: name, MType: models.Gauge, Value: &v})
	}
	for name, delta := range s.store.Counters() {
		d := delta
		metrics = append(metrics, models.Metrics{ID: name, MType: models.Counter, Delta: &d})
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("encode metrics: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create metrics directory: %w", err)
	}

	file, err := os.CreateTemp(dir, ".metrics-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary metrics file: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)

	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write metrics file: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync metrics file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close metrics file: %w", err)
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		return fmt.Errorf("replace metrics file: %w", err)
	}
	return nil
}

// Run периодически сохраняет метрики, пока не будет отменён контекст.
// При нулевом интервале запись выполняется синхронно после каждого обновления.
func (s *FileStorage) Run(ctx context.Context) {
	if s.storeInterval <= 0 {
		return
	}

	ticker := time.NewTicker(s.storeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Save(); err != nil {
				log.Printf("save metrics: %v", err)
			}
		}
	}
}

func (s *FileStorage) saveSynchronously() {
	if s.storeInterval != 0 {
		return
	}
	if err := s.Save(); err != nil {
		log.Printf("save metrics: %v", err)
	}
}

func (s *FileStorage) UpdateGauge(name string, value float64) {
	s.store.UpdateGauge(name, value)
	s.saveSynchronously()
}

func (s *FileStorage) UpdateCounter(name string, delta int64) {
	s.store.UpdateCounter(name, delta)
	s.saveSynchronously()
}

func (s *FileStorage) Gauge(name string) (float64, bool) {
	return s.store.Gauge(name)
}

func (s *FileStorage) Counter(name string) (int64, bool) {
	return s.store.Counter(name)
}

func (s *FileStorage) Gauges() map[string]float64 {
	return s.store.Gauges()
}

func (s *FileStorage) Counters() map[string]int64 {
	return s.store.Counters()
}

var _ Repository = (*FileStorage)(nil)
