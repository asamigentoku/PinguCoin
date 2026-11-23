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
	// このAPIはmedia-api専用です。共有Internal API Tokenで呼び出し元を認証します。
	if h.token == "" || strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ") != h.token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.directory == "" {
		http.Error(w, "exchange-rate storage is not configured", http.StatusServiceUnavailable)
		return
	}
	var payload exchangeRatePublishRequest
	// リクエストサイズを制限し、versionとsnapshotのJSON形式を検証します。
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

	// ルートディレクトリを作成します。
	// 例: /var/lib/pingucoin/exchange-rates
	// MkdirAllは既に存在していても成功するため、起動直後でも安全に呼べます。
	if err := os.MkdirAll(h.directory, 0755); err != nil {
		http.Error(w, "cannot create exchange-rate directory", http.StatusInternalServerError)
		return
	}
	// versionはYYYYMMDDHHMMSS形式です。
	// 先頭8文字のYYYYMMDDを使い、日付ごとにファイルを分けます。
	// 例: version=20261123100500
	//     保存先=/var/lib/pingucoin/exchange-rates/20261123/20261123100500.json
	dateDirectory := filepath.Join(h.directory, payload.Version[:8])
	// versionの先頭8桁を日付ディレクトリにし、同一秒のスナップショットを配置します。
	// 日付ディレクトリを作成します。日付が同じスナップショットは同じディレクトリに入ります。
	if err := os.MkdirAll(dateDirectory, 0755); err != nil {
		http.Error(w, "cannot create dated exchange-rate directory", http.StatusInternalServerError)
		return
	}
	// 完成したスナップショットを、日付ディレクトリ内の時刻ファイルとして保存します。
	// 例: /var/lib/pingucoin/exchange-rates/20261123/20261123100500.json
	if err := writeAtomic(filepath.Join(dateDirectory, payload.Version+".json"), payload.Snapshot, 0644); err != nil {
		http.Error(w, "cannot write exchange-rate snapshot", http.StatusInternalServerError)
		return
	}
	// 旧方式で使っていたlatest.jsonは削除します。
	// 現在は時刻付きURLを使うため、latest.jsonを正規の参照先にはしません。
	if err := os.Remove(filepath.Join(h.directory, "latest.json")); err != nil && !os.IsNotExist(err) {
		http.Error(w, "cannot remove legacy exchange-rate manifest", http.StatusInternalServerError)
		return
	}
	// 一時ファイルからrename済みなので、読み取り側が不完全なJSONを見ることはありません。
	w.WriteHeader(http.StatusCreated)
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	// いきなり目的のJSONファイルへ書くと、読み取り中に不完全なJSONを返す可能性があります。
	// そのため同じディレクトリ内の一時ファイルへ書き切ってからrenameします。
	file, err := os.CreateTemp(filepath.Dir(path), ".exchange-rate-*.tmp")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	// 後続処理で失敗しても、一時ファイルを残さないようにします。
	defer os.Remove(tempPath)
	// 最終ファイルと同じ権限に設定します。
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	// スナップショット本体を書き込みます。まだ利用者が見る正式なパスには存在しません。
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	// OSの書き込みバッファからディスクへ反映します。
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	// closeしてからrenameし、Windowsでもファイルを移動できる状態にします。
	if err := file.Close(); err != nil {
		return err
	}
	// 同じディレクトリ内のrenameは、読み取り側から見ると一瞬で切り替わります。
	// これにより、古い完全なJSONか新しい完全なJSONのどちらかだけが見えます。
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return nil
}
