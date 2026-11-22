package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/asamigentoku/PinguCoin/services/media-api/proto/orcan/v1"
)

type fakeExchangeRateClient struct {
	response *pb.GetExchangeRatesResponse
	err      error
	calls    int
}

func (f *fakeExchangeRateClient) GetExchangeRates(context.Context, *pb.GetExchangeRatesRequest, ...grpc.CallOption) (*pb.GetExchangeRatesResponse, error) {
	f.calls++
	return f.response, f.err
}

func TestExchangeRateLiveHandlerFetchesForEveryRequest(t *testing.T) {
	updatedAt := time.Date(2026, 11, 18, 3, 0, 0, 0, time.UTC)
	client := &fakeExchangeRateClient{response: &pb.GetExchangeRatesResponse{
		Rates:     []*pb.ExchangeRate{{Currency: "USD", JpyPerUnit: 155.5}},
		UpdatedAt: timestamppb.New(updatedAt),
	}}
	handler := NewExchangeRateLiveHandler(client)

	for range 2 {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/exchange-rates", nil)
		response := httptest.NewRecorder()
		handler.Get(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.Code)
		}
		if got := response.Header().Get("Cache-Control"); got != "public, max-age=1" {
			t.Fatalf("Cache-Control = %q, want public, max-age=1", got)
		}
	}
	if client.calls != 2 {
		t.Fatalf("upstream calls = %d, want 2", client.calls)
	}
}

func TestExchangeRateLiveHandlerReturnsBadGateway(t *testing.T) {
	handler := NewExchangeRateLiveHandler(&fakeExchangeRateClient{err: errors.New("provider unavailable")})
	response := httptest.NewRecorder()
	handler.Get(response, httptest.NewRequest(http.MethodGet, "/api/v1/exchange-rates", nil))
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", response.Code)
	}
}
