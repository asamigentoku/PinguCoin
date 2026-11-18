package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestExchangeRatePublishHandlerWritesOnlyVersionedSnapshot(t *testing.T) {
	directory := t.TempDir()
	legacyManifest := filepath.Join(directory, "latest.json")
	if err := os.WriteFile(legacyManifest, []byte(`{"file":"old.json"}`), 0644); err != nil {
		t.Fatal(err)
	}
	handler := NewExchangeRatePublishHandler(directory, "secret")
	request := httptest.NewRequest(http.MethodPut, "/internal/exchange-rates", bytes.NewBufferString(
		`{"version":"20261118120005","snapshot":{"base":"JPY","rates":{"USD":155.5},"updatedAt":"2026-11-18T12:00:00Z"}}`,
	))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()

	handler.Put(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", response.Code, response.Body.String())
	}
	if _, err := os.Stat(filepath.Join(directory, "20261118", "20261118120005.json")); err != nil {
		t.Fatalf("versioned snapshot was not written: %v", err)
	}
	if _, err := os.Stat(legacyManifest); !os.IsNotExist(err) {
		t.Fatalf("legacy latest.json still exists: %v", err)
	}
}
