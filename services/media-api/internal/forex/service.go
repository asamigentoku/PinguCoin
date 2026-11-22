package forex

import (
	"bytes"
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
	"strings"
	"sync"
	"time"
)

const (
	RefreshInterval = 5 * time.Minute
	PublishInterval = time.Second
)

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
	publish func(context.Context, Snapshot) error
}

// SetPublisher registers the periodic static-snapshot publisher. Publishing is
// best effort: a storage outage must not discard the last in-memory snapshot.
func (s *Service) SetPublisher(publish func(context.Context, Snapshot) error) {
	s.publish = publish
}

// PublishSnapshot sends a completed snapshot to BFF under the next UTC-second
// name so the current-second URL already exists when clients request it.
func PublishSnapshot(ctx context.Context, publishURL, token string, snapshot Snapshot) error {
	// The filename identifies when this static file was generated. updatedAt
	// continues to identify when the rates themselves were fetched upstream.
	version := time.Now().UTC().Add(PublishInterval).Truncate(PublishInterval).Format("20060102150405")
	snapshotJSON, err := json.Marshal(map[string]any{"base": "JPY", "rates": snapshot.Rates, "updatedAt": snapshot.UpdatedAt})
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"version": version, "snapshot": json.RawMessage(snapshotJSON)})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, publishURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("BFF snapshot publish returned HTTP %d", response.StatusCode)
	}
	return nil
}

func New(apiKey, baseURL string, logger *slog.Logger) *Service {
	return &Service{apiKey: apiKey, baseURL: baseURL, client: &http.Client{Timeout: 10 * time.Second}, logger: logger}
}

func (s *Service) Run(ctx context.Context) {
	s.run(ctx, RefreshInterval, PublishInterval)
}

func (s *Service) run(ctx context.Context, refreshInterval, publishInterval time.Duration) {
	if s.apiKey == "" {
		s.logger.Warn("forex refresh disabled: TWELVE_DATA_API_KEY is empty")
		return
	}
	s.refresh(ctx)
	s.publishLatest(ctx)
	refreshTicker := time.NewTicker(refreshInterval)
	publishTicker := time.NewTicker(publishInterval)
	defer refreshTicker.Stop()
	defer publishTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-refreshTicker.C:
			s.refresh(ctx)
		case <-publishTicker.C:
			s.publishLatest(ctx)
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
	symbols := make([]string, 0, len(currencies))
	for _, currency := range currencies {
		symbols = append(symbols, currency+"/JPY")
	}
	prices, err := s.fetch(ctx, symbols)
	if err != nil {
		s.logger.Warn("failed to refresh exchange rates; keeping last good snapshot", slog.Any("symbols", symbols), slog.Any("error", err))
		return
	}
	rates := make(map[string]float64, len(currencies))
	for _, currency := range currencies {
		rates[currency] = prices[currency+"/JPY"]
	}
	snapshot := Snapshot{Rates: rates, UpdatedAt: time.Now().UTC()}
	s.mu.Lock()
	s.latest = snapshot
	s.mu.Unlock()
	keys := make([]string, 0, len(rates))
	for key := range rates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	s.logger.Info("exchange rates refreshed", slog.Any("currencies", keys))
}

func (s *Service) publishLatest(ctx context.Context) {
	if s.publish == nil {
		return
	}
	snapshot, err := s.Snapshot()
	if err != nil {
		s.logger.Warn("exchange-rate snapshot is not ready", slog.Any("error", err))
		return
	}
	if err := s.publish(ctx, snapshot); err != nil {
		s.logger.Warn("failed to publish static exchange-rate snapshot", slog.Any("error", err))
	}
}

func (s *Service) fetch(ctx context.Context, symbols []string) (map[string]float64, error) {
	endpoint, err := url.Parse(s.baseURL + "/price")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("symbol", strings.Join(symbols, ","))
	query.Set("apikey", s.apiKey)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Twelve Data returned HTTP %d", response.StatusCode)
	}
	type pricePayload struct {
		Price   string `json:"price"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	var payload map[string]pricePayload
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	rates := make(map[string]float64, len(symbols))
	for _, symbol := range symbols {
		price, ok := payload[symbol]
		if !ok {
			return nil, fmt.Errorf("Twelve Data response is missing %s", symbol)
		}
		if price.Status == "error" {
			return nil, fmt.Errorf("Twelve Data %s: %s", symbol, price.Message)
		}
		rate, err := strconv.ParseFloat(price.Price, 64)
		if err != nil || rate <= 0 || math.IsInf(rate, 0) || math.IsNaN(rate) {
			return nil, fmt.Errorf("invalid %s rate %q", symbol, price.Price)
		}
		rates[symbol] = rate
	}
	return rates, nil
}
