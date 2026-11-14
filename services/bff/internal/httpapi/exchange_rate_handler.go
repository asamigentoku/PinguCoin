package httpapi

import (
	"net/http"

	"github.com/asamigentoku/PinguCoin/services/bff/internal/apperr"
	"github.com/asamigentoku/PinguCoin/services/bff/internal/orcanclient"
	pb "github.com/asamigentoku/PinguCoin/services/media-api/proto/orcan/v1"
)

type ExchangeRateHandler struct{ media *orcanclient.Client }

func NewExchangeRateHandler(media *orcanclient.Client) *ExchangeRateHandler {
	return &ExchangeRateHandler{media: media}
}

func (h *ExchangeRateHandler) Get(w http.ResponseWriter, r *http.Request) {
	response, err := h.media.ExchangeRate.GetExchangeRates(r.Context(), &pb.GetExchangeRatesRequest{})
	if err != nil {
		writeError(w, r, apperr.FromGRPC(err))
		return
	}
	rates := make(map[string]float64, len(response.GetRates()))
	for _, rate := range response.GetRates() {
		rates[rate.GetCurrency()] = rate.GetJpyPerUnit()
	}
	w.Header().Set("Cache-Control", "public, max-age=180")
	writeJSON(w, http.StatusOK, map[string]any{"base": "JPY", "rates": rates, "updatedAt": response.GetUpdatedAt().AsTime()})
}
