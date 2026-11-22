package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

var exchangeRateFilePattern = regexp.MustCompile(`^[0-9]{14}\.json$`)

type ExchangeRateStaticHandler struct {
	directory string
}

func NewExchangeRateStaticHandler(directory string) *ExchangeRateStaticHandler {
	return &ExchangeRateStaticHandler{directory: directory}
}

func (h *ExchangeRateStaticHandler) Get(w http.ResponseWriter, r *http.Request) {
	date := r.PathValue("date")
	file := r.PathValue("file")
	if len(date) != 8 || !exchangeRateFilePattern.MatchString(file) || file[:8] != date {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(h.directory, date, file)
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=1")
	http.ServeFile(w, r, path)
}
