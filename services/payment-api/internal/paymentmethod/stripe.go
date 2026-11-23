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

// ChargeRequestは外部決済プロバイダーへ決済を依頼するための情報です。
// IdempotencyKeyは同じ決済リクエストを再送しても二重決済にならないように使います。
type ChargeRequest struct {
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
	IdempotencyKey string `json:"idempotency_key"`
	ReturnURL      string `json:"return_url,omitempty"`
	PaymentMethod  string `json:"payment_method"`
}

// ChargeResultは外部決済プロバイダーから返される決済状態です。
// requires_actionの場合は、ActionURLへユーザーを誘導して追加認証を行います。
type ChargeResult struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	ActionURL string `json:"action_url,omitempty"`
}

// RefundRequestは外部決済プロバイダーへ返金を依頼するための情報です。
type RefundRequest struct {
	PaymentIntentID string `json:"payment_intent_id"`
	Amount          int64  `json:"amount"`
	IdempotencyKey  string `json:"idempotency_key"`
}

// RefundResultは外部決済プロバイダーから返される返金結果です。
type RefundResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// Chargerはpayment-apiが必要とする外部決済操作の抽象化です。
// 本番相当のStripe実装と、テスト用のInMemoryStripeを差し替えられます。
type Charger interface {
	Charge(context.Context, ChargeRequest) (ChargeResult, error)
	Cancel(context.Context, string) error
	Refund(context.Context, RefundRequest) (RefundResult, error)
}

// PermanentErrorは、リクエストを修正しない限り再試行しても成功しないエラーです。
// 例として、決済拒否などの4xxエラーを表します。
type PermanentError struct{ Err error }

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }
func IsPermanent(err error) bool        { var target *PermanentError; return errors.As(err, &target) }

// StripeはStripe互換のHTTP APIを呼び出す外部決済クライアントです。
// 現在のMinikube環境ではmock Stripeに接続し、本番では同じChargerインターフェースを
// 実際のStripeクライアントへ差し替えられる構造になっています。
type Stripe struct {
	baseURL string
	client  *http.Client
}

func NewStripe(baseURL string) *Stripe {
	// URL末尾のスラッシュを除去し、各APIパスと安全に連結できるようにします。
	// 外部サービスが応答しない場合に処理が無期限で待たないよう、5秒のタイムアウトを設定します。
	return &Stripe{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 5 * time.Second}}
}

func (s *Stripe) Charge(ctx context.Context, request ChargeRequest) (ChargeResult, error) {
	// まずPaymentIntentを作成し、プロバイダー側で決済状態を確認します。
	var intent ChargeResult
	if err := s.post(ctx, "/v1/payment_intents", request, &intent); err != nil {
		return intent, err
	}
	if intent.Status == "requires_action" || intent.Status == "succeeded" {
		// 追加認証が必要な場合や、作成時点ですでに成功した場合はcaptureを行いません。
		return intent, nil
	}
	// 作成直後に確定していない決済はcapture APIで確定します。
	if err := s.post(ctx, "/v1/payment_intents/"+intent.ID+"/capture", struct{}{}, &intent); err != nil {
		// captureの応答が失敗した場合は、未確定のPaymentIntentをキャンセルします。
		_ = s.Cancel(ctx, intent.ID)
		return intent, err
	}
	if intent.Status != "succeeded" {
		// HTTP通信は成功していても、決済状態が成功でなければ異常として扱います。
		return intent, fmt.Errorf("unexpected Stripe status %q", intent.Status)
	}
	return intent, nil
}

func (s *Stripe) Refund(ctx context.Context, request RefundRequest) (RefundResult, error) {
	// 返金は外部プロバイダーのrefund APIへ委譲します。
	var result RefundResult
	err := s.post(ctx, "/v1/refunds", request, &result)
	return result, err
}

func (s *Stripe) Cancel(ctx context.Context, id string) error {
	// PaymentIntentをキャンセルし、未確定の外部決済を無効化します。
	return s.post(ctx, "/v1/payment_intents/"+id+"/cancel", struct{}{}, &ChargeResult{})
}

func (s *Stripe) post(ctx context.Context, path string, body, output any) error {
	// リクエストボディをJSONへ変換します。
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// contextのキャンセルとHTTPクライアントのタイムアウトを有効にして送信します。
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("mock stripe request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		// 4xxは入力・決済内容に起因する恒久的失敗として扱います。
		// ただし429は一時的なレート制限のため、再試行可能なエラーにします。
		err := fmt.Errorf("mock stripe returned HTTP %d", resp.StatusCode)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return &PermanentError{Err: err}
		}
		return err
	}
	if err := json.NewDecoder(resp.Body).Decode(output); err != nil {
		// HTTP成功でもレスポンスを解釈できなければ、決済結果を確定できません。
		return fmt.Errorf("decode mock stripe response: %w", err)
	}
	return nil
}

// InMemoryStripeは外部HTTP通信を行わず、常に決定的な成功結果を返すテスト用実装です。
type InMemoryStripe struct{}

func (InMemoryStripe) Charge(_ context.Context, r ChargeRequest) (ChargeResult, error) {
	// テストでは入力の冪等性キーから固定IDを作り、結果を再現可能にします。
	return ChargeResult{ID: "pi_" + r.IdempotencyKey, Status: "succeeded"}, nil
}
func (InMemoryStripe) Cancel(context.Context, string) error { return nil }
func (InMemoryStripe) Refund(_ context.Context, r RefundRequest) (RefundResult, error) {
	return RefundResult{ID: "re_" + r.IdempotencyKey, Status: "succeeded"}, nil
}
