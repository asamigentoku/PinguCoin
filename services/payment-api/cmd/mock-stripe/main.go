package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/paymentmethod"
)

type store struct {
	mu            sync.Mutex
	byKey         map[string]*paymentmethod.ChargeResult
	byID          map[string]*paymentmethod.ChargeResult
	nextID        uint64
	amounts       map[string]int64
	refunded      map[string]int64
	refundsByKey  map[string]paymentmethod.RefundResult
	returnURLs    map[string]string
	methods       map[string]string
	webhookURL    string
	webhookSecret string
	publicURL     string
}

func main() {
	s := &store{byKey: map[string]*paymentmethod.ChargeResult{}, byID: map[string]*paymentmethod.ChargeResult{}, amounts: map[string]int64{}, refunded: map[string]int64{}, refundsByKey: map[string]paymentmethod.RefundResult{}, returnURLs: map[string]string{}, methods: map[string]string{}, webhookURL: os.Getenv("PAYMENT_WEBHOOK_URL"), webhookSecret: os.Getenv("WEBHOOK_SECRET"), publicURL: strings.TrimRight(os.Getenv("PUBLIC_URL"), "/")}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/payment_intents", s.create)
	mux.HandleFunc("POST /v1/payment_intents/{id}/capture", s.capture)
	mux.HandleFunc("POST /v1/payment_intents/{id}/cancel", s.cancel)
	mux.HandleFunc("POST /v1/refunds", s.refund)
	mux.HandleFunc("GET /3ds/{id}", s.threeDS)
	mux.HandleFunc("POST /3ds/{id}/confirm", s.confirmThreeDS)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	log.Printf("mock-stripe listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func (s *store) create(w http.ResponseWriter, r *http.Request) {
	var request paymentmethod.ChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Amount <= 0 || strings.TrimSpace(request.IdempotencyKey) == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing := s.byKey[request.IdempotencyKey]; existing != nil {
		writeJSON(w, existing)
		return
	}
	s.nextID++
	intent := &paymentmethod.ChargeResult{ID: fmt.Sprintf("pi_mock_%d", s.nextID), Status: "requires_action"}
	base := s.publicURL
	if base == "" {
		base = "http://" + r.Host
	}
	intent.ActionURL = base + "/3ds/" + intent.ID
	s.byKey[request.IdempotencyKey], s.byID[intent.ID] = intent, intent
	s.amounts[intent.ID], s.returnURLs[intent.ID] = request.Amount, request.ReturnURL
	s.methods[intent.ID] = request.PaymentMethod
	writeJSON(w, intent)
}

func (s *store) capture(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r.PathValue("id"), "succeeded")
}
func (s *store) cancel(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r.PathValue("id"), "canceled")
}
func (s *store) transition(w http.ResponseWriter, id, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	intent := s.byID[id]
	if intent == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if intent.Status != "succeeded" || status == "succeeded" {
		intent.Status = status
	}
	writeJSON(w, intent)
}

func (s *store) threeDS(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	intent := s.byID[r.PathValue("id")]
	method := s.methods[r.PathValue("id")]
	s.mu.Unlock()
	if intent == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	title, description := "3Dセキュア認証", "テスト用カード決済を承認します。"
	if method == "bank_transfer" {
		title, description = "銀行振込の確認", "テスト用銀行振込を完了します。"
	}
	_, _ = fmt.Fprintf(w, `<!doctype html><html lang="ja"><meta name="viewport" content="width=device-width"><title>%s</title><style>body{font-family:system-ui;background:#eef8ff;display:grid;place-items:center;min-height:100vh}.box{background:white;padding:32px;border-radius:20px;box-shadow:0 12px 40px #1232;max-width:420px}button{width:100%%;padding:14px;border:0;border-radius:999px;background:#1264e8;color:white;font-weight:700}</style><div class="box"><h1>%s</h1><p>%s 決済ID: %s</p><form method="post" action="/3ds/%s/confirm"><button>認証して支払う</button></form></div></html>`, title, title, description, intent.ID, intent.ID)
}

func (s *store) confirmThreeDS(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	intent := s.byID[id]
	if intent == nil {
		s.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	intent.Status, intent.ActionURL = "succeeded", ""
	returnURL := s.returnURLs[id]
	s.mu.Unlock()
	if err := s.notify(r.Context(), "payment_intent.succeeded", intent); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if returnURL == "" {
		returnURL = "/"
	}
	http.Redirect(w, r, returnURL, http.StatusSeeOther)
}

func (s *store) refund(w http.ResponseWriter, r *http.Request) {
	var request paymentmethod.RefundRequest
	if json.NewDecoder(r.Body).Decode(&request) != nil || request.Amount <= 0 || request.IdempotencyKey == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	if existing, ok := s.refundsByKey[request.IdempotencyKey]; ok {
		s.mu.Unlock()
		writeJSON(w, existing)
		return
	}
	intent := s.byID[request.PaymentIntentID]
	if intent == nil {
		s.mu.Unlock()
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if intent.Status != "succeeded" || s.refunded[intent.ID]+request.Amount > s.amounts[intent.ID] {
		s.mu.Unlock()
		http.Error(w, "not refundable", http.StatusConflict)
		return
	}
	s.refunded[intent.ID] += request.Amount
	result := paymentmethod.RefundResult{ID: "re_" + request.IdempotencyKey, Status: "succeeded"}
	s.refundsByKey[request.IdempotencyKey] = result
	s.mu.Unlock()
	writeJSON(w, result)
}

func (s *store) notify(ctx context.Context, eventType string, intent *paymentmethod.ChargeResult) error {
	if s.webhookURL == "" {
		return nil
	}
	payload, _ := json.Marshal(map[string]any{"id": "evt_" + intent.ID + "_" + eventType, "type": eventType, "data": intent})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	_, _ = mac.Write(payload)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mock-Stripe-Signature", fmt.Sprintf("%x", mac.Sum(nil)))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("webhook HTTP %d: %s", resp.StatusCode, body)
	}
	return nil
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
