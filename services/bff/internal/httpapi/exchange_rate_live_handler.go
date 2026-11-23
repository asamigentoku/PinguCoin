package httpapi

import (
	"context"
	"net/http"

	"google.golang.org/grpc"

	pb "github.com/asamigentoku/PinguCoin/services/media-api/proto/orcan/v1"
)

type exchangeRateClient interface {
	GetExchangeRates(context.Context, *pb.GetExchangeRatesRequest, ...grpc.CallOption) (*pb.GetExchangeRatesResponse, error)
}

type ExchangeRateLiveHandler struct {
	client exchangeRateClient
}

func NewExchangeRateLiveHandler(client exchangeRateClient) *ExchangeRateLiveHandler {
	return &ExchangeRateLiveHandler{client: client}
}

//v1側
func (h *ExchangeRateLiveHandler) Get(w http.ResponseWriter, r *http.Request) {
	// BFFからmedia-apiのExchangeRateServiceへgRPCで最新Snapshotを問い合わせます。
	response, err := h.client.GetExchangeRates(r.Context(), &pb.GetExchangeRatesRequest{})
	if err != nil {
		// media-apiに最新値がない、または接続できない場合はHTTP 502を返します。
		writeJSON(w, http.StatusBadGateway, errorResponse{Reason: "exchange_rate_provider", Message: "exchange rates are unavailable"})
		return
	}
	// protobufのrepeated ratesを、HTTP APIで扱いやすい通貨名をキーにしたMapへ変換します。
	rates := make(map[string]float64, len(response.GetRates()))
	for _, rate := range response.GetRates() {
		rates[rate.GetCurrency()] = rate.GetJpyPerUnit()
	}
	w.Header().Set("Cache-Control", "public, max-age=1")
	// upstreamの更新間隔とは別に、クライアントとNginxには1秒間のキャッシュを許可します。
	writeJSON(w, http.StatusOK, map[string]any{
		"base":      "JPY",
		"rates":     rates,
		"updatedAt": response.GetUpdatedAt().AsTime(),
	})
}
