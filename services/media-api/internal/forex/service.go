package forex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"
)

const RefreshInterval = 3 * time.Minute

var currencies = []string{"USD", "EUR", "GBP", "CNY"}

type Snapshot struct {
	Rates     map[string]float64
	UpdatedAt time.Time
}

type Service struct {
	apiKey  string
	baseURL string
	client  *http.Client
	logger  *slog.Logger
	mu      sync.RWMutex
	latest  Snapshot
}

func New(apiKey, baseURL string, logger *slog.Logger) *Service {
	return &Service{apiKey: apiKey, baseURL: baseURL, client: &http.Client{Timeout: 10 * time.Second}, logger: logger}
}

func (s *Service) Run(ctx context.Context) {
	if s.apiKey == "" {
		s.logger.Warn("forex refresh disabled: TWELVE_DATA_API_KEY is empty")
		return
	}
	s.refresh(ctx)
	ticker := time.NewTicker(RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh(ctx)
		}
	}
}

func (s *Service) Snapshot() (Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.latest.UpdatedAt.IsZero() {
		return Snapshot{}, errors.New("exchange rates are not available yet")
	}
	rates := make(map[string]float64, len(s.latest.Rates))
	for currency, rate := range s.latest.Rates {
		rates[currency] = rate
	}
	return Snapshot{Rates: rates, UpdatedAt: s.latest.UpdatedAt}, nil
}

func (s *Service) refresh(ctx context.Context) {
	rates := make(map[string]float64, len(currencies))
	for _, currency := range currencies {
		rate, err := s.fetch(ctx, currency+"/JPY")
		if err != nil {
			s.logger.Warn("failed to refresh exchange rates; keeping last good snapshot", slog.String("symbol", currency+"/JPY"), slog.Any("error", err))
			return
		}
		rates[currency] = rate
	}
	s.mu.Lock()
	s.latest = Snapshot{Rates: rates, UpdatedAt: time.Now().UTC()}
	s.mu.Unlock()
	keys := make([]string, 0, len(rates))
	for key := range rates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	s.logger.Info("exchange rates refreshed", slog.Any("currencies", keys))
}

func (s *Service) fetch(ctx context.Context, symbol string) (float64, error) {
	endpoint, err := url.Parse(s.baseURL + "/price")
	if err != nil {
		return 0, err
	}
	query := endpoint.Query()
	query.Set("symbol", symbol)
	query.Set("apikey", s.apiKey)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return 0, err
	}
	response, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Twelve Data returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		Price   string `json:"price"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return 0, err
	}
	if payload.Status == "error" {
		return 0, fmt.Errorf("Twelve Data: %s", payload.Message)
	}
	rate, err := strconv.ParseFloat(payload.Price, 64)
	if err != nil || rate <= 0 || math.IsInf(rate, 0) || math.IsNaN(rate) {
		return 0, fmt.Errorf("invalid rate %q", payload.Price)
	}
	return rate, nil
}
