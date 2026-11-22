package grpcserver

import (
	"context"
	"sort"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/asamigentoku/PinguCoin/services/media-api/internal/forex"
	pb "github.com/asamigentoku/PinguCoin/services/media-api/proto/orcan/v1"
)

type ExchangeRateServer struct {
	pb.UnimplementedExchangeRateServiceServer
	service *forex.Service
}

func NewExchangeRateServer(service *forex.Service) *ExchangeRateServer {
	return &ExchangeRateServer{service: service}
}

func (s *ExchangeRateServer) GetExchangeRates(context.Context, *pb.GetExchangeRatesRequest) (*pb.GetExchangeRatesResponse, error) {
	snapshot, err := s.service.Snapshot()
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	currencies := make([]string, 0, len(snapshot.Rates))
	for currency := range snapshot.Rates {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	rates := make([]*pb.ExchangeRate, 0, len(currencies))
	for _, currency := range currencies {
		rates = append(rates, &pb.ExchangeRate{Currency: currency, JpyPerUnit: snapshot.Rates[currency]})
	}
	return &pb.GetExchangeRatesResponse{Rates: rates, UpdatedAt: timestamppb.New(snapshot.UpdatedAt)}, nil
}
