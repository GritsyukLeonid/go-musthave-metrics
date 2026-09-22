package config

import (
	"io"
	"testing"
)

func TestParseServer(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    Server
		wantErr bool
	}{
		{
			name: "без аргументов берутся значения по умолчанию",
			args: nil,
			want: Server{Address: DefaultAddress},
		},
		{
			name: "адрес через знак равенства",
			args: []string{"-a=localhost:9090"},
			want: Server{Address: "localhost:9090"},
		},
		{
			name: "адрес отдельным аргументом",
			args: []string{"-a", "127.0.0.1:9090"},
			want: Server{Address: "127.0.0.1:9090"},
		},
		{
			name:    "неизвестный флаг",
			args:    []string{"-x=1"},
			wantErr: true,
		},
		{
			name:    "лишний аргумент за флагами",
			args:    []string{"-a=localhost:9090", "run"},
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
