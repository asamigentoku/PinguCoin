package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

var exchangeRateFilePattern = regexp.MustCompile(`^(latest|[0-9]{14})\.json$`)

type ExchangeRateStaticHandler struct {
	directory string
}

func NewExchangeRateStaticHandler(directory string) *ExchangeRateStaticHandler {
	return &ExchangeRateStaticHandler{directory: directory}
}

func (h *ExchangeRateStaticHandler) Get(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	if !exchangeRateFilePattern.MatchString(file) {
		http.NotFound(w, r)
		return
	}

	path := filepath.Join(h.directory, file)
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if file == "latest.json" {
		w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeFile(w, r, path)
}
