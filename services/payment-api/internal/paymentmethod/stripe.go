package paymentmethod

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ChargeRequest struct {
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
	IdempotencyKey string `json:"idempotency_key"`
	ReturnURL      string `json:"return_url,omitempty"`
	PaymentMethod  string `json:"payment_method"`
}

type ChargeResult struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	ActionURL string `json:"action_url,omitempty"`
}

type RefundRequest struct {
	PaymentIntentID string `json:"payment_intent_id"`
	Amount          int64  `json:"amount"`
	IdempotencyKey  string `json:"idempotency_key"`
}

type RefundResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type Charger interface {
	Charge(context.Context, ChargeRequest) (ChargeResult, error)
	Cancel(context.Context, string) error
	Refund(context.Context, RefundRequest) (RefundResult, error)
}

type PermanentError struct{ Err error }

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }
func IsPermanent(err error) bool        { var target *PermanentError; return errors.As(err, &target) }

// Stripe calls the mock Stripe service using provider-side idempotency.
type Stripe struct {
	baseURL string
	client  *http.Client
}

func NewStripe(baseURL string) *Stripe {
	return &Stripe{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 5 * time.Second}}
}

func (s *Stripe) Charge(ctx context.Context, request ChargeRequest) (ChargeResult, error) {
	var intent ChargeResult
	if err := s.post(ctx, "/v1/payment_intents", request, &intent); err != nil {
		return intent, err
	}
	if intent.Status == "requires_action" || intent.Status == "succeeded" {
		return intent, nil
	}
	if err := s.post(ctx, "/v1/payment_intents/"+intent.ID+"/capture", struct{}{}, &intent); err != nil {
		_ = s.Cancel(ctx, intent.ID)
		return intent, err
	}
	if intent.Status != "succeeded" {
		return intent, fmt.Errorf("unexpected Stripe status %q", intent.Status)
	}
	return intent, nil
}

func (s *Stripe) Refund(ctx context.Context, request RefundRequest) (RefundResult, error) {
	var result RefundResult
	err := s.post(ctx, "/v1/refunds", request, &result)
	return result, err
}

func (s *Stripe) Cancel(ctx context.Context, id string) error {
	return s.post(ctx, "/v1/payment_intents/"+id+"/cancel", struct{}{}, &ChargeResult{})
}

func (s *Stripe) post(ctx context.Context, path string, body, output any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("mock stripe request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		err := fmt.Errorf("mock stripe returned HTTP %d", resp.StatusCode)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return &PermanentError{Err: err}
		}
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(output); err != nil {
		return fmt.Errorf("decode mock stripe response: %w", err)
	}
	return nil
}

// InMemoryStripe is deterministic and is used by unit/integration tests.
type InMemoryStripe struct{}

func (InMemoryStripe) Charge(_ context.Context, r ChargeRequest) (ChargeResult, error) {
	return ChargeResult{ID: "pi_" + r.IdempotencyKey, Status: "succeeded"}, nil
}
func (InMemoryStripe) Cancel(context.Context, string) error { return nil }
func (InMemoryStripe) Refund(_ context.Context, r RefundRequest) (RefundResult, error) {
	return RefundResult{ID: "re_" + r.IdempotencyKey, Status: "succeeded"}, nil
}
