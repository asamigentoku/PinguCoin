package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var snapshotVersionPattern = regexp.MustCompile(`^[0-9]{14}$`)

type exchangeRatePublishRequest struct {
	Snapshot json.RawMessage `json:"snapshot"`
	Version  string          `json:"version"`
}

type ExchangeRatePublishHandler struct {
	directory string
	token     string
}

func NewExchangeRatePublishHandler(directory, token string) *ExchangeRatePublishHandler {
	return &ExchangeRatePublishHandler{directory: directory, token: token}
}

func (h *ExchangeRatePublishHandler) Put(w http.ResponseWriter, r *http.Request) {
	if h.token == "" || strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") != h.token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.directory == "" {
		http.Error(w, "exchange-rate storage is not configured", http.StatusServiceUnavailable)
		return
	}
	var payload exchangeRatePublishRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	if err := decoder.Decode(&payload); err != nil || len(payload.Snapshot) == 0 || !snapshotVersionPattern.MatchString(payload.Version) {
		http.Error(w, "invalid publish payload", http.StatusBadRequest)
		return
	}
	var snapshot map[string]json.RawMessage
	if err := json.Unmarshal(payload.Snapshot, &snapshot); err != nil || len(snapshot["rates"]) == 0 || len(snapshot["updatedAt"]) == 0 {
		http.Error(w, "invalid exchange-rate snapshot", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(h.directory, 0755); err != nil {
		http.Error(w, "cannot create exchange-rate directory", http.StatusInternalServerError)
		return
	}
	dateDirectory := filepath.Join(h.directory, payload.Version[:8])
	if err := os.MkdirAll(dateDirectory, 0755); err != nil {
		http.Error(w, "cannot create dated exchange-rate directory", http.StatusInternalServerError)
		return
	}
	if err := writeAtomic(filepath.Join(dateDirectory, payload.Version+".json"), payload.Snapshot, 0644); err != nil {
		http.Error(w, "cannot write exchange-rate snapshot", http.StatusInternalServerError)
		return
	}
	if err := os.Remove(filepath.Join(h.directory, "latest.json")); err != nil && !os.IsNotExist(err) {
		http.Error(w, "cannot remove legacy exchange-rate manifest", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".exchange-rate-*.tmp")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return nil
}
