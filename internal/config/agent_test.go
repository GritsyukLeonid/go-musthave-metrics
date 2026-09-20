package config

import (
	"io"
	"testing"
	"time"
)

func TestParseAgent(t *testing.T) {
	defaults := Agent{
		Address:        DefaultAddress,
		ReportInterval: DefaultReportInterval,
		PollInterval:   DefaultPollInterval,
	}

	tests := []struct {
		name    string
		args    []string
		want    Agent
		wantErr bool
	}{
		{
			name: "без аргументов берутся значения по умолчанию",
			args: nil,
			want: defaults,
		},
		{
			name: "все флаги разом",
			args: []string{"-a=localhost:9090", "-r=5", "-p=1"},
			want: Agent{Address: "localhost:9090", ReportInterval: 5 * time.Second, PollInterval: time.Second},
		},
		{
			name: "флаги отдельными аргументами",
			args: []string{"-a", "localhost:9090", "-r", "5", "-p", "1"},
			want: Agent{Address: "localhost:9090", ReportInterval: 5 * time.Second, PollInterval: time.Second},
		},
		{
			name: "заданное значение не затирает остальные умолчания",
			args: []string{"-p=1"},
			want: Agent{Address: DefaultAddress, ReportInterval: DefaultReportInterval, PollInterval: time.Second},
		},
		{
			name:    "неизвестный флаг",
			args:    []string{"-x=1"},
			wantErr: true,
		},
		{
			name:    "лишний аргумент за флагами",
			args:    []string{"-r=5", "run"},
			wantErr: true,
		},
		{
			name:    "интервал не число",
			args:    []string{"-r=5s"},
			wantErr: true,
		},
		{
			name:    "нулевой интервал опроса",
			args:    []string{"-p=0"},
			wantErr: true,
		},
		{
			name:    "отрицательный интервал отправки",
			args:    []string{"-r=-1"},
			wantErr: true,
		},
		{
			name:    "пустой адрес",
			args:    []string{"-a="},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// io.Discard: пакет flag печатает ошибку и список флагов сам,
			// и в выводе теста этот шум только мешает.
			got, err := parseAgent(tt.args, io.Discard)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseAgent(%q) вернул %+v; ожидалась ошибка", tt.args, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseAgent(%q) вернул ошибку %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("parseAgent(%q) = %+v; ожидалось %+v", tt.args, got, tt.want)
			}
		})
	}
}

func TestAgentServerURL(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
	}{
		{
			name:    "адрес без схемы получает http",
			address: "localhost:8080",
			want:    "http://localhost:8080",
		},
		{
			name:    "адрес по умолчанию",
			address: DefaultAddress,
			want:    "http://" + DefaultAddress,
		},
		{
			name:    "заданная схема сохраняется",
			address: "https://example.com:8080",
			want:    "https://example.com:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Agent{Address: tt.address}).ServerURL(); got != tt.want {
				t.Errorf("ServerURL() = %q; ожидалось %q", got, tt.want)
			}
		})
	}
}
