package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	models "github.com/GritsyukLeonid/go-musthave-metrics/internal/model"
)

func TestFileStorageSaveAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	store := NewMemStorage()
	store.UpdateGauge("Alloc", 12.5)
	store.UpdateCounter("PollCount", 42)

	fileStore := NewFileStorage(store, path, time.Second)
	if err := fileStore.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var metrics []models.Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if len(metrics) != 2 {
		t.Fatalf("saved %d metrics, want 2", len(metrics))
	}

	restored := NewMemStorage()
	restoredFileStore := NewFileStorage(restored, path, time.Second)
	if err := restoredFileStore.Restore(); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if got, ok := restored.Gauge("Alloc"); !ok || got != 12.5 {
		t.Errorf("restored gauge = %v, %v; want 12.5, true", got, ok)
	}
	if got, ok := restored.Counter("PollCount"); !ok || got != 42 {
		t.Errorf("restored counter = %v, %v; want 42, true", got, ok)
	}
}

func TestFileStorageRestoreMissingFile(t *testing.T) {
	fileStore := NewFileStorage(NewMemStorage(), filepath.Join(t.TempDir(), "missing.json"), time.Second)
	if err := fileStore.Restore(); err != nil {
		t.Fatalf("Restore() for missing file error = %v, want nil", err)
	}
}

func TestFileStorageSavesSynchronously(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	fileStore := NewFileStorage(NewMemStorage(), path, 0)

	fileStore.UpdateGauge("Alloc", 3.5)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("synchronous update did not create file: %v", err)
	}
	var metrics []models.Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if len(metrics) != 1 || metrics[0].ID != "Alloc" || metrics[0].Value == nil || *metrics[0].Value != 3.5 {
		t.Fatalf("saved metrics = %+v; want gauge Alloc=3.5", metrics)
	}
}

func TestFileStorageRunStopsWithContext(t *testing.T) {
	fileStore := NewFileStorage(NewMemStorage(), filepath.Join(t.TempDir(), "metrics.json"), time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		fileStore.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop after context cancellation")
	}
}
