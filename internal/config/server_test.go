package config

import (
	"io"
	"os"
	"testing"
	"time"
)

func clearServerEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"ADDRESS", "STORE_INTERVAL", "FILE_STORAGE_PATH", "RESTORE"} {
		value, existed := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(key, value)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func setServerDefaults(t *testing.T) {
	t.Helper()
	t.Setenv("ADDRESS", DefaultAddress)
	t.Setenv("STORE_INTERVAL", "300")
	t.Setenv("FILE_STORAGE_PATH", "/tmp/metrics-db.json")
	t.Setenv("RESTORE", "true")
}

func TestParseServer(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    Server
		wantErr bool
	}{
		{
			name: "без аргументов берутся значения по умолчанию",
			want: Server{
				Address: DefaultAddress, StoreInterval: 300 * time.Second,
				FileStoragePath: "/tmp/metrics-db.json", Restore: true,
			},
		},
		{
			name: "параметры через флаги",
			args: []string{"-a=localhost:9090", "-i=0", "-f=metrics.json", "-r=false"},
			want: Server{
				Address: "localhost:9090", StoreInterval: 0,
				FileStoragePath: "metrics.json", Restore: false,
			},
		},
		{
			name: "адрес отдельным аргументом",
			args: []string{"-a", "127.0.0.1:9090"},
			want: Server{
				Address: "127.0.0.1:9090", StoreInterval: 300 * time.Second,
				FileStoragePath: "/tmp/metrics-db.json", Restore: true,
			},
		},
		{name: "неизвестный флаг", args: []string{"-x=1"}, wantErr: true},
		{name: "лишний аргумент за флагами", args: []string{"-a=localhost:9090", "run"}, wantErr: true},
		{name: "пустой адрес", args: []string{"-a="}, wantErr: true},
		{name: "отрицательный интервал", args: []string{"-i=-1"}, wantErr: true},
		{name: "пустой путь", args: []string{"-f="}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearServerEnvironment(t)
			got, err := parseServer(tt.args, io.Discard)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseServer(%q) вернул %+v; ожидалась ошибка", tt.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseServer(%q) вернул ошибку %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("parseServer(%q) = %+v; ожидалось %+v", tt.args, got, tt.want)
			}
		})
	}
}

func TestParseServerEnvironmentOverridesFlags(t *testing.T) {
	clearServerEnvironment(t)
	setServerDefaults(t)
	t.Setenv("STORE_INTERVAL", "7")
	t.Setenv("FILE_STORAGE_PATH", "/var/tmp/metrics.json")
	t.Setenv("RESTORE", "false")
	t.Setenv("ADDRESS", "127.0.0.1:7777")

	got, err := parseServer([]string{
		"-a=localhost:9090", "-i=10", "-f=flags.json", "-r=true",
	}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	want := Server{
		Address: "127.0.0.1:7777", StoreInterval: 7 * time.Second,
		FileStoragePath: "/var/tmp/metrics.json", Restore: false,
	}
	if got != want {
		t.Fatalf("parseServer() = %+v; ожидалось %+v", got, want)
	}
}

func TestParseServerInvalidEnvironment(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  string
	}{
		{name: "invalid interval", key: "STORE_INTERVAL", val: "abc"},
		{name: "invalid restore", key: "RESTORE", val: "sometimes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearServerEnvironment(t)
			setServerDefaults(t)
			t.Setenv(tt.key, tt.val)
			if _, err := parseServer(nil, io.Discard); err == nil {
				t.Fatal("ожидалась ошибка конфигурации")
			}
		})
	}
}
