package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/application"
)

type Handler struct {
	usecase *application.PaymentUseCase
	secret  []byte
}

func New(usecase *application.PaymentUseCase, secret string) *Handler {
	return &Handler{usecase: usecase, secret: []byte(secret)}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	want, err := hex.DecodeString(r.Header.Get("X-Mock-Stripe-Signature"))
	if err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	mac := hmac.New(sha256.New, h.secret)
	_, _ = mac.Write(body)
	if !hmac.Equal(want, mac.Sum(nil)) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	var event application.ProviderEvent
	if json.Unmarshal(body, &event) != nil {
		http.Error(w, "invalid event", http.StatusBadRequest)
		return
	}
	if err := h.usecase.HandleProviderEvent(r.Context(), event); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
