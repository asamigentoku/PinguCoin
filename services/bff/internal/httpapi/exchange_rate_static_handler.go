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

//v2側
func (h *ExchangeRateStaticHandler) Get(w http.ResponseWriter, r *http.Request) {
	// URLパラメータを取得し、任意のファイルを読めないように形式を検証します。
	date := r.PathValue("date")
	file := r.PathValue("file")
	if len(date) != 8 || !exchangeRateFilePattern.MatchString(file) || file[:8] != date {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(h.directory, date, file)
	// 公開対象のスナップショットが存在する場合だけファイルを返します。
	//ここでは存在しない場合は500系エラーレベルで起きないこととする
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=1")
	// BFFから直接ファイルを返し、media-apiや外部レートAPIへはアクセスしません。
	http.ServeFile(w, r, path)
}
