package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"
)

// NewPing возвращает обработчик проверки соединения с базой данных.
func NewPing(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "database is not configured", http.StatusInternalServerError)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "database is unavailable", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}
