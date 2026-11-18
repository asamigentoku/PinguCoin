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
	dateDirectory := filepath.Join(directory, "20261114")
	if err := os.MkdirAll(dateDirectory, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dateDirectory, "20261114120305.json"), []byte(`{"version":"requested"}`), 0644); err != nil {
		t.Fatal(err)
	}
	handler := NewExchangeRateStaticHandler(directory)
	req := httptest.NewRequest(http.MethodGet, "/api/v2/exchange-rates/20261114/20261114120305.json", nil)
	req.SetPathValue("date", "20261114")
	req.SetPathValue("file", "20261114120305.json")
	res := httptest.NewRecorder()
	handler.Get(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if got := res.Header().Get("Cache-Control"); got != "public, max-age=1" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := res.Body.String(); got != `{"version":"requested"}` {
		t.Fatalf("body = %s, want requested snapshot", got)
	}
}

func TestExchangeRateStaticHandlerReturnsNotFoundWithoutSnapshots(t *testing.T) {
	handler := NewExchangeRateStaticHandler(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/v2/exchange-rates/20261114/20261114120305.json", nil)
	req.SetPathValue("date", "20261114")
	req.SetPathValue("file", "20261114120305.json")
	res := httptest.NewRecorder()
	handler.Get(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Code)
	}
}

func TestExchangeRateStaticHandlerRejectsLatestManifest(t *testing.T) {
	handler := NewExchangeRateStaticHandler(t.TempDir())
	req := httptest.NewRequest(http.MethodGet, "/api/v2/exchange-rates/20261114/latest.json", nil)
	req.SetPathValue("date", "20261114")
	req.SetPathValue("file", "latest.json")
	res := httptest.NewRecorder()
	handler.Get(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.Code)
	}
}
