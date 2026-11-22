package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGzipDecompress(t *testing.T) {
	var got string
	handler := GzipDecompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		got = string(body)
		w.WriteHeader(http.StatusOK)
	}))

	var body bytes.Buffer
	zw := gzip.NewWriter(&body)
	_, _ = zw.Write([]byte("payload"))
	_ = zw.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got != "payload" {
		t.Fatalf("body = %q, want payload", got)
	}
}

func TestGzipCompress(t *testing.T) {
	tests := []struct {
		name             string
		acceptEncoding   string
		contentType      string
		wantContentCoding string
	}{
		{name: "gzip json", acceptEncoding: "gzip", contentType: "application/json", wantContentCoding: "gzip"},
		{name: "gzip html", acceptEncoding: "gzip", contentType: "text/html; charset=utf-8", wantContentCoding: "gzip"},
		{name: "gzip disabled by quality", acceptEncoding: "gzip;q=0", contentType: "application/json", wantContentCoding: ""},
		{name: "unsupported type", acceptEncoding: "gzip", contentType: "text/plain", wantContentCoding: ""},
		{name: "client does not accept gzip", acceptEncoding: "identity", contentType: "application/json", wantContentCoding: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := GzipCompress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				_, _ = io.WriteString(w, "payload")
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if got := rec.Header().Get("Content-Encoding"); got != tt.wantContentCoding {
				t.Fatalf("Content-Encoding = %q, want %q", got, tt.wantContentCoding)
			}
			if tt.wantContentCoding == "gzip" {
				zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
				if err != nil {
					t.Fatalf("gzip.NewReader: %v", err)
				}
				defer zr.Close()
				body, err := io.ReadAll(zr)
				if err != nil {
					t.Fatalf("read compressed response: %v", err)
				}
				if string(body) != "payload" {
					t.Fatalf("decompressed body = %q, want payload", body)
				}
				if !strings.Contains(rec.Header().Get("Vary"), "Accept-Encoding") {
					t.Fatalf("Vary = %q, want Accept-Encoding", rec.Header().Get("Vary"))
				}
			} else if rec.Body.String() != "payload" {
				t.Fatalf("body = %q, want payload", rec.Body.String())
			}
		})
	}
}

func TestGzipDecompressInvalidBody(t *testing.T) {
	handler := GzipDecompress(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler should not be called for invalid gzip")
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not gzip"))
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
