package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExchangeRateStaticHandler(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "20261114120300.json"), []byte(`{"base":"JPY"}`), 0644); err != nil {
		t.Fatal(err)
	}
	handler := NewExchangeRateStaticHandler(directory)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/exchange-rates/20261114120300.json", nil)
	req.SetPathValue("file", "20261114120300.json")
	res := httptest.NewRecorder()
	handler.Get(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if got := res.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestExchangeRateStaticHandlerRejectsUnknownFile(t *testing.T) {
	handler := NewExchangeRateStaticHandler(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/exchange-rates/not-static.json", nil)
	req.SetPathValue("file", "not-static.json")
	res := httptest.NewRecorder()
	handler.Get(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Code)
	}
}
