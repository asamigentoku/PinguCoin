package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/asamigentoku/PinguCoin/pkg/orderflow"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/model"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/paymentmethod"
	"github.com/asamigentoku/PinguCoin/services/payment-api/internal/repository"
)

var (
	ErrInvalid             = errors.New("invalid payment")
	ErrInsufficientPoints  = errors.New("insufficient points")
	ErrUnsupportedMethod   = errors.New("unsupported payment method")
	ErrProviderUnavailable = errors.New("payment provider unavailable")
)

// ここでコマンドを受け取る
type CreatePaymentCommand struct {
	UserID, ProductID uint
	Amount            int64
	Currency          string
	PaymentMethod     string
	IdempotencyKey    string
	Order             *orderflow.OrderCreated // present for Kafka-triggered payments
	Purpose           string
}

type CreatePaymentResult struct {
	Payment   *model.Payment
	Replayed  bool
	Rejected  error
	ActionURL string
}

// PaymentUseCaseは決済に関する業務処理の中心です。
// gRPC、Kafka Consumer、Webhookのどの入口から呼ばれても、
// 入力検証・冪等性確認・決済処理・結果イベント作成を同じルールで実行します。
type PaymentUseCase struct {
	db        *gorm.DB
	//それぞれモデル自体ではなくリポジトリ層を叩く
	payments  *repository.PaymentRepository
	points    *repository.PointRepository
	//支払い方法の設定
	sources   *repository.PaymentSourceRepository
	outbox    *repository.OutboxRepository
	stripe    paymentmethod.Charger
	returnURL string
}

// NewPaymentUseCaseは決済処理に必要なDBリポジトリと外部決済クライアントを組み立てます。
func NewPaymentUseCase(db *gorm.DB, stripe paymentmethod.Charger, returnURLs ...string) *PaymentUseCase {
	returnURL := "http://localhost:3000/purchases"
	if len(returnURLs) > 0 && strings.TrimSpace(returnURLs[0]) != "" {
		returnURL = returnURLs[0]
	}
	return &PaymentUseCase{db: db, payments: repository.NewPaymentRepository(db), points: repository.NewPointRepository(db), sources: repository.NewPaymentSourceRepository(db), outbox: repository.NewOutboxRepository(db), stripe: stripe, returnURL: returnURL}
}

// Createは決済処理の共通入口です。
// 入力を検証し、冪等性を確認した後、決済方法に応じた処理へ分岐します。
// 決済結果はoutbox_eventsへ保存し、Publisherが後でKafkaへ通知します。
func (u *PaymentUseCase) Create(ctx context.Context, cmd CreatePaymentCommand) (*CreatePaymentResult, error) {
	// 入力値はKafkaイベント、gRPCリクエスト、Webhookなど複数の入口から来ます。
	// ここで共通の形式にそろえてから、検証と決済処理を行います。
	//ここでkafkaを読み取って最終的にたどり着く
	cmd.Currency, cmd.PaymentMethod, cmd.IdempotencyKey = strings.TrimSpace(cmd.Currency), strings.TrimSpace(cmd.PaymentMethod), strings.TrimSpace(cmd.IdempotencyKey)
	if cmd.Purpose == "" {
		// Purposeが省略された場合は、通常の商品購入として扱います。
		cmd.Purpose = model.PaymentPurposePurchase
	}
	if err := validate(cmd); err != nil {
		// 注文由来の決済は失敗イベントを保存し、order-apiへ失敗を通知します。
		result := &CreatePaymentResult{Rejected: err}
		// Kafka起点の注文は、入力不備でも失敗イベントを返して注文を
		// payment_pendingのまま放置しない。最低限永続化できる項目がある場合だけ記録する。
		if u.db != nil && cmd.Order != nil && cmd.UserID != 0 && cmd.ProductID != 0 && cmd.Amount > 0 && cmd.IdempotencyKey != "" {
			var recordErr error
			result.Payment, recordErr = u.recordRejected(u.db.WithContext(ctx), cmd, err)
			if recordErr != nil {
				return nil, recordErr
			}
		}
		return result, nil
	}
	db := u.db.WithContext(ctx)
	// 同じ冪等性キーの決済が存在する場合、外部決済を再実行せず既存結果を返します。
	if existing, err := u.payments.WithTx(db).FindByIdempotencyKey(cmd.IdempotencyKey); err == nil {
		return u.replay(db, existing), nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	switch cmd.PaymentMethod {
	// 決済方法ごとに、ポイント決済と外部プロバイダー決済へ分岐します。
	//ポイント払いの時
	case model.PaymentMethodPoint:
	//stripeなどの外部経由払いの場合
	case model.PaymentMethodCard, model.PaymentMethodCreditCard, model.PaymentMethodBankTransfer:
		return u.createStripe(ctx, db, cmd)
	default:
		result := &CreatePaymentResult{Rejected: fmt.Errorf("%w: %s", ErrUnsupportedMethod, cmd.PaymentMethod)}
		if cmd.Order !=		return u.createPoint(db, cmd)
 nil {
			result.Payment, _ = u.recordRejected(db, cmd, result.Rejected)
		}
		return result, nil
	}
}

// ここでvalidationをする
// validateは必須項目と、決済方法・通貨の組み合わせを検証します。
func validate(c CreatePaymentCommand) error {
	switch {
	case c.UserID == 0:
		return fmt.Errorf("%w: user_id is required", ErrInvalid)
	case c.ProductID == 0 && c.Purpose != model.PaymentPurposePointTopUp:
		return fmt.Errorf("%w: product_id is required", ErrInvalid)
	case c.Amount <= 0:
		return fmt.Errorf("%w: amount must be greater than zero", ErrInvalid)
	case c.Currency == "":
		return fmt.Errorf("%w: currency is required", ErrInvalid)
	case c.PaymentMethod == "":
		return fmt.Errorf("%w: payment_method is required", ErrInvalid)
	case c.IdempotencyKey == "":
		return fmt.Errorf("%w: idempotency_key is required", ErrInvalid)
	case c.Purpose != model.PaymentPurposePurchase && c.Purpose != model.PaymentPurposePointTopUp:
		return fmt.Errorf("%w: invalid purpose", ErrInvalid)
	case c.PaymentMethod == model.PaymentMethodPoint && (c.Currency != "POINT" || c.Purpose != model.PaymentPurposePurchase):
		return fmt.Errorf("%w: point payments require currency POINT and purpose purchase", ErrInvalid)
	case (c.PaymentMethod == model.PaymentMethodCard || c.PaymentMethod == model.PaymentMethodCreditCard || c.PaymentMethod == model.PaymentMethodBankTransfer) && c.Currency != "JPY":
		return fmt.Errorf("%w: external payments require currency JPY", ErrInvalid)
	default:
		return nil
	}
}

// newPaymentはコマンドからDB保存用の決済モデルを作成します。
func newPayment(c CreatePaymentCommand, status string) *model.Payment {
	return &model.Payment{UserID: c.UserID, ProductID: c.ProductID, Amount: c.Amount, Currency: c.Currency, PaymentMethod: c.PaymentMethod, Purpose: c.Purpose, Status: status, IdempotencyKey: c.IdempotencyKey}
}

// replayは同じ冪等性キーで既存決済が見つかった場合の再送結果を作ります。
func replay(payment *model.Payment) *CreatePaymentResult {
	r := &CreatePaymentResult{Payment: payment, Replayed: true}
	if payment.Status == model.PaymentStatusFailed {
		r.Rejected = errors.New("payment previously failed")
	}
	return r
}

func (u *PaymentUseCase) replay(db *gorm.DB, payment *model.Payment) *CreatePaymentResult {
	r := replay(payment)
	if source, err := u.sources.WithTx(db).FindByPaymentID(payment.ID); err == nil {
		r.ActionURL = source.ActionURL
	}
	return r
}

// createPointはポイント残高を減算し、決済を成功として確定します。
// 決済レコード、ポイント台帳、結果イベントを同じDBトランザクションで保存します。
func (u *PaymentUseCase) createPoint(db *gorm.DB, cmd CreatePaymentCommand) (*CreatePaymentResult, error) {
	// ポイント決済は外部サービスを呼ばず、payment-api内の残高を即時に確定します。
	//ここで実際にpaymentに成功ステータスで書き込む
	payment := newPayment(cmd, model.PaymentStatusSucceeded)
	// 決済レコード、ポイント減算、決済結果イベントを同時に確定します。
	// 途中で失敗した場合は、これらの処理をすべてロールバックします。
	//WithTx(tx)がトランザクション付きモデル操作
	err := db.Transaction(func(tx *gorm.DB) error {

		if err := u.payments.WithTx(tx).Create(payment); err != nil {
			return err
		}
		// point_accountsの残高を減らし、point_transactionsへ減算履歴を記録します。
		paymentID := payment.ID
		if _, err := u.points.WithTx(tx).Adjust(payment.UserID, -payment.Amount, model.PointTransactionTypePayment, fmt.Sprintf("payment #%d", payment.ID), &paymentID); err != nil {
			return err
		}
		// 注文から呼ばれた場合は、決済成功イベントをoutbox_eventsへ保存します。
		return u.enqueue(tx, payment, cmd, "")
	})
	if errors.Is(err, repository.ErrInsufficientPoints) {
		// 残高不足の場合は決済を失敗として記録し、注文にも失敗イベントを返します。
		rejected := ErrInsufficientPoints
		if cmd.Order != nil {
			payment, _ = u.recordRejected(db, cmd, rejected)
		} else {
			payment = nil
		}
		return &CreatePaymentResult{Payment: payment, Rejected: rejected}, nil
	}
	if err != nil {
		// DB一意制約などで競合した場合、既に作られた決済を再利用します。
		if existing, findErr := u.payments.WithTx(db).FindByIdempotencyKey(cmd.IdempotencyKey); findErr == nil {
			return u.replay(db, existing), nil
		}
		return nil, err
	}
	return &CreatePaymentResult{Payment: payment}, nil
}

// createStripeは外部決済を開始し、成功・失敗・追加認証の結果を処理します。
func (u *PaymentUseCase) createStripe(ctx context.Context, db *gorm.DB, cmd CreatePaymentCommand) (*CreatePaymentResult, error) {
	// カード・クレジットカード・銀行振込など、外部決済プロバイダーを使う決済です。
	// 最初はprocessingで保存し、外部プロバイダーの結果を受けて最終状態へ更新します。
	payment := newPayment(cmd, model.PaymentStatusProcessing)
	source := &model.PaymentSource{Provider: "stripe", Status: model.PaymentSourceStatusPending}
	if cmd.Order != nil {
		// 後でWebhookを受け取ったときに、注文向けPaymentResultを再構成できるよう保存します。
		payload, err := json.Marshal(cmd.Order)
		if err != nil {
			return nil, err
		}
		source.OrderPayload = string(payload)
	} else {
		source.OrderPayload = "{}"
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		// paymentsとpayment_sourcesを同時に作成し、決済と外部決済情報を関連付けます。
		if err := u.payments.WithTx(tx).Create(payment); err != nil {
			return err
		}
		source.PaymentID = payment.ID
		return u.sources.WithTx(tx).Create(source)
	}); err != nil {
		if existing, findErr := u.payments.WithTx(db).FindByIdempotencyKey(cmd.IdempotencyKey); findErr == nil {
			return u.replay(db, existing), nil
		}
		return nil, err
	}

	returnURL := u.returnURL
	if cmd.Purpose == model.PaymentPurposePointTopUp {
		// ポイント購入の場合は、決済後の戻り先をポイント画面に変更します。
		returnURL = strings.TrimSuffix(returnURL, "/purchases") + "/points"
	}
	// 外部決済プロバイダーへ決済を依頼します。
	// IdempotencyKeyを渡すことで、同じリクエストの再送による二重決済を防ぎます。
	charged, chargeErr := u.stripe.Charge(ctx, paymentmethod.ChargeRequest{Amount: cmd.Amount, Currency: cmd.Currency, IdempotencyKey: cmd.IdempotencyKey, ReturnURL: returnURL, PaymentMethod: cmd.PaymentMethod})
	if chargeErr != nil {
		if !paymentmethod.IsPermanent(chargeErr) {
			// 外部サービスの応答が失われた可能性があるため、失敗確定せずprocessingのまま再照合します。
			// The provider may have completed the charge even though the response was
			// lost. Keep processing and let the idempotent reconciler converge it.
			return nil, fmt.Errorf("%w: %v", ErrProviderUnavailable, chargeErr)
		}
		rejected := fmt.Errorf("%w: %v", ErrProviderUnavailable, chargeErr)
		if err := db.Transaction(func(tx *gorm.DB) error {
			// 恒久的な失敗はpayment-api側にも保存し、注文へ失敗イベントを送ります。
			payment.Status, source.Status, source.FailureReason = model.PaymentStatusFailed, model.PaymentSourceStatusFailed, rejected.Error()
			if err := u.payments.WithTx(tx).Update(payment); err != nil {
				return err
			}
			if err := u.sources.WithTx(tx).Update(source); err != nil {
				return err
			}
			return u.enqueue(tx, payment, cmd, rejected.Error())
		}); err != nil {
			return nil, err
		}
		return &CreatePaymentResult{Payment: payment, Rejected: rejected}, nil
	}

	if charged.Status == "requires_action" {
		// 3DSなどユーザー操作が必要な場合は、認証URLを保存して注文へ通知します。
		if err := db.Transaction(func(tx *gorm.DB) error {
			source.Status, source.ProviderReference, source.ActionURL = model.PaymentSourceStatusAuthorized, charged.ID, charged.ActionURL
			if err := u.sources.WithTx(tx).Update(source); err != nil {
				return err
			}
			return u.enqueueActionRequired(tx, payment, cmd, charged.ActionURL)
		}); err != nil {
			return nil, err
		}
		return &CreatePaymentResult{Payment: payment, ActionURL: charged.ActionURL}, nil
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		// 外部決済成功をローカルDBへ反映します。
		payment.Status = model.PaymentStatusSucceeded
		source.Status, source.ProviderReference = model.PaymentSourceStatusCaptured, charged.ID
		if err := u.payments.WithTx(tx).Update(payment); err != nil {
			return err
		}
		if err := u.sources.WithTx(tx).Update(source); err != nil {
			return err
		}
		if payment.Purpose == model.PaymentPurposePointTopUp {
			// 外部決済によるポイント購入の場合だけ、成功後にポイントを加算します。
			paymentID := payment.ID
			if _, err := u.points.WithTx(tx).Adjust(payment.UserID, payment.Amount, model.PointTransactionTypeCredit, fmt.Sprintf("point top-up #%d", payment.ID), &paymentID); err != nil {
				return err
			}
		}
		return u.enqueue(tx, payment, cmd, "")
	}); err != nil {
		// ローカル更新に失敗した場合は、外部決済をキャンセルして不整合を抑えます。
		_ = u.stripe.Cancel(ctx, charged.ID) // compensation; reconciliation retries unresolved cases
		return nil, err
	}
	return &CreatePaymentResult{Payment: payment}, nil
}

// enqueueActionRequiredは3DSなどの追加操作が必要な場合に、認証URLを含むイベントを保存します。
func (u *PaymentUseCase) enqueueActionRequired(tx *gorm.DB, payment *model.Payment, cmd CreatePaymentCommand, actionURL string) error {
	// 注文から呼ばれた決済だけ、注文サービスへ結果を返します。
	if cmd.Order == nil {
		return nil
	}
	event := orderflow.PaymentResult{OrderCreated: *cmd.Order, PaymentID: payment.ID, ActionURL: actionURL}
	// Kafkaへ直接送らず、まずoutbox_eventsへ保存します。
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return u.outbox.WithTx(tx).Create(&model.OutboxEvent{AggregateID: payment.ID, Topic: orderflow.TopicPaymentActionRequired, Key: cmd.IdempotencyKey, Payload: string(payload)})
}

// recordRejectedは決済を失敗として記録し、注文があれば失敗イベントも保存します。
func (u *PaymentUseCase) recordRejected(db *gorm.DB, cmd CreatePaymentCommand, reason error) (*model.Payment, error) {
	// 入力不正や残高不足でも、注文由来なら失敗決済をDBに残します。
	payment := newPayment(cmd, model.PaymentStatusFailed)
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := u.payments.WithTx(tx).Create(payment); err != nil {
			return err
		}
		return u.enqueue(tx, payment, cmd, reason.Error())
	})
	return payment, err
}

// enqueueは決済結果をPaymentResultイベントに変換してoutboxへ保存します。
// Publisherがpayments.succeeded.v1またはpayments.failed.v1へ発行します。
func (u *PaymentUseCase) enqueue(tx *gorm.DB, payment *model.Payment, cmd CreatePaymentCommand, reason string) error {
	// ポイント購入など、注文と関係しない決済では注文イベントを作りません。
	if cmd.Order == nil {
		return nil
	}
	event := orderflow.PaymentResult{OrderCreated: *cmd.Order, PaymentID: payment.ID, Reason: reason}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	topic := orderflow.TopicPaymentSucceeded
	if reason != "" || payment.Status == model.PaymentStatusFailed {
		// reasonがある、または決済がfailedなら失敗イベントにします。
		topic = orderflow.TopicPaymentFailed
	}
	// outbox publisherが後でこのレコードをKafkaへ発行します。
	//outboxにおくことでワーカーが障害も考慮してkafkaに書き込んでいる
	return u.outbox.WithTx(tx).Create(&model.OutboxEvent{AggregateID: payment.ID, Topic: topic, Key: cmd.IdempotencyKey, Payload: string(payload)})
}

// ReconcileProcessingは、外部決済成功直後にプロセスが停止して、
// payment-api側の最終更新だけが未完了になった決済を再確認します。
// 元の冪等性キーで外部プロバイダーへ問い合わせ、二重決済を避けます。
func (u *PaymentUseCase) ReconcileProcessing(ctx context.Context, olderThan time.Duration) error {
	db := u.db.WithContext(ctx)
	sources, err := u.sources.WithTx(db).PendingBefore(time.Now().Add(-olderThan), 100)
	if err != nil {
		return err
	}
	for i := range sources {
		source := &sources[i]
		payment, err := u.payments.WithTx(db).FindByID(source.PaymentID)
		if err != nil {
			return err
		}
		if payment.Status != model.PaymentStatusProcessing {
			continue
		}
		var order *orderflow.OrderCreated
		if source.OrderPayload != "" && source.OrderPayload != "{}" {
			var decoded orderflow.OrderCreated
			if json.Unmarshal([]byte(source.OrderPayload), &decoded) == nil {
				order = &decoded
			}
		}
		cmd := CreatePaymentCommand{UserID: payment.UserID, ProductID: payment.ProductID, Amount: payment.Amount, Currency: payment.Currency, PaymentMethod: payment.PaymentMethod, IdempotencyKey: payment.IdempotencyKey, Order: order}
		charged, err := u.stripe.Charge(ctx, paymentmethod.ChargeRequest{Amount: payment.Amount, Currency: payment.Currency, IdempotencyKey: payment.IdempotencyKey})
		if err != nil {
			if !paymentmethod.IsPermanent(err) {
				continue
			}
			reason := fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
			if err := db.Transaction(func(tx *gorm.DB) error {
				payment.Status, source.Status, source.FailureReason = model.PaymentStatusFailed, model.PaymentSourceStatusFailed, reason.Error()
				if err := u.payments.WithTx(tx).Update(payment); err != nil {
					return err
				}
				if err := u.sources.WithTx(tx).Update(source); err != nil {
					return err
				}
				return u.enqueue(tx, payment, cmd, reason.Error())
			}); err != nil {
				return err
			}
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			payment.Status = model.PaymentStatusSucceeded
			source.Status, source.ProviderReference = model.PaymentSourceStatusCaptured, charged.ID
			if err := u.payments.WithTx(tx).Update(payment); err != nil {
				return err
			}
			if err := u.sources.WithTx(tx).Update(source); err != nil {
				return err
			}
			if payment.Purpose == model.PaymentPurposePointTopUp {
				paymentID := payment.ID
				if _, err := u.points.WithTx(tx).Adjust(payment.UserID, payment.Amount, model.PointTransactionTypeCredit, fmt.Sprintf("point top-up #%d", payment.ID), &paymentID); err != nil {
					return err
				}
			}
			return u.enqueue(tx, payment, cmd, "")
		}); err != nil {
			_ = u.stripe.Cancel(ctx, charged.ID)
			return err
		}
	}
	return nil
}

type ProviderEvent struct {
	ID   string                     `json:"id"`
	Type string                     `json:"type"`
	Data paymentmethod.ChargeResult `json:"data"`
}

// HandleProviderEventは外部決済プロバイダーのWebhookを処理します。
// Webhook受信記録、決済状態更新、ポイント反映、結果イベント保存を
// 1トランザクションで行い、同じWebhookの再送を一度だけ処理します。
func (u *PaymentUseCase) HandleProviderEvent(ctx context.Context, event ProviderEvent) error {
	if event.ID == "" || event.Data.ID == "" {
		return fmt.Errorf("%w: invalid provider event", ErrInvalid)
	}
	db := u.db.WithContext(ctx)
	source, err := u.sources.WithTx(db).FindByProviderReference(event.Data.ID)
	if err != nil {
		return err
	}
	payment, err := u.payments.WithTx(db).FindByID(source.PaymentID)
	if err != nil {
		return err
	}
	var order *orderflow.OrderCreated
	if source.OrderPayload != "" && source.OrderPayload != "{}" {
		var decoded orderflow.OrderCreated
		if json.Unmarshal([]byte(source.OrderPayload), &decoded) == nil {
			order = &decoded
		}
	}
	return db.Transaction(func(tx *gorm.DB) error {
		receipt := &model.ProviderEvent{Provider: source.Provider, EventID: event.ID}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(receipt)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			return nil
		}
		if payment.Status != model.PaymentStatusProcessing {
			return nil
		}
		cmd := CreatePaymentCommand{UserID: payment.UserID, ProductID: payment.ProductID, Amount: payment.Amount, Currency: payment.Currency, PaymentMethod: payment.PaymentMethod, IdempotencyKey: payment.IdempotencyKey, Purpose: payment.Purpose, Order: order}
		switch event.Type {
		case "payment_intent.succeeded":
			payment.Status, source.Status, source.ActionURL = model.PaymentStatusSucceeded, model.PaymentSourceStatusCaptured, ""
			if err := u.payments.WithTx(tx).Update(payment); err != nil {
				return err
			}
			if err := u.sources.WithTx(tx).Update(source); err != nil {
				return err
			}
			if payment.Purpose == model.PaymentPurposePointTopUp {
				paymentID := payment.ID
				if _, err := u.points.WithTx(tx).Adjust(payment.UserID, payment.Amount, model.PointTransactionTypeCredit, fmt.Sprintf("point top-up #%d", payment.ID), &paymentID); err != nil {
					return err
				}
			}
			return u.enqueue(tx, payment, cmd, "")
		case "payment_intent.failed", "payment_intent.canceled":
			payment.Status, source.Status, source.ActionURL = model.PaymentStatusFailed, model.PaymentSourceStatusFailed, ""
			source.FailureReason = event.Type
			if event.Type == "payment_intent.canceled" {
				payment.Status, source.Status = model.PaymentStatusCanceled, model.PaymentSourceStatusCanceled
			}
			if err := u.payments.WithTx(tx).Update(payment); err != nil {
				return err
			}
			if err := u.sources.WithTx(tx).Update(source); err != nil {
				return err
			}
			return u.enqueue(tx, payment, cmd, event.Type)
		default:
			return fmt.Errorf("%w: unsupported provider event %s", ErrInvalid, event.Type)
		}
	})
}

// CancelExternalは外部プロバイダーの決済をキャンセルし、
// payment-apiの状態と注文向け結果イベントを更新します。
func (u *PaymentUseCase) CancelExternal(ctx context.Context, payment *model.Payment) error {
	source, err := u.sources.WithTx(u.db.WithContext(ctx)).FindByPaymentID(payment.ID)
	if err != nil {
		return err
	}
	if source.ProviderReference != "" {
		if err := u.stripe.Cancel(ctx, source.ProviderReference); err != nil {
			return err
		}
	}
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		payment.Status, source.Status, source.ActionURL = model.PaymentStatusCanceled, model.PaymentSourceStatusCanceled, ""
		if err := u.payments.WithTx(tx).Update(payment); err != nil {
			return err
		}
		if err := u.sources.WithTx(tx).Update(source); err != nil {
			return err
		}
		var order *orderflow.OrderCreated
		if source.OrderPayload != "" && source.OrderPayload != "{}" {
			var decoded orderflow.OrderCreated
			if json.Unmarshal([]byte(source.OrderPayload), &decoded) == nil {
				order = &decoded
			}
		}
		return u.enqueue(tx, payment, CreatePaymentCommand{UserID: payment.UserID, ProductID: payment.ProductID, Amount: payment.Amount, Currency: payment.Currency, PaymentMethod: payment.PaymentMethod, IdempotencyKey: payment.IdempotencyKey, Purpose: payment.Purpose, Order: order}, "payment canceled")
	})
}

// RefundExternalは外部プロバイダーへ返金を依頼します。
func (u *PaymentUseCase) RefundExternal(ctx context.Context, payment *model.Payment, amount int64, key string) error {
	source, err := u.sources.WithTx(u.db.WithContext(ctx)).FindByPaymentID(payment.ID)
	if err != nil {
		return err
	}
	_, err = u.stripe.Refund(ctx, paymentmethod.RefundRequest{PaymentIntentID: source.ProviderReference, Amount: amount, IdempotencyKey: key})
	return err
}
