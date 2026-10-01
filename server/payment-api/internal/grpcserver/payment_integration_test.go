package grpcserver

import (
	"context"
	"sync"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/database"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/model"
	pb "github.com/asamigentoku/PinguCoin/server/payment-api/internal/pb/payment/v1"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/repository"
	"github.com/asamigentoku/PinguCoin/server/payment-api/internal/testutil"
)

// これらは実際のPostgresが必要(TEST_DATABASE_URL)。決済とポイントの増減が1つのトランザクションで
// 動くこと、二重決済にならないことなど、お金に関わる振る舞いを確かめる。

type fixture struct {
	db       *gorm.DB
	payments *PaymentServer
	points   *PointServer
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := testutil.NewDB(t, database.AutoMigrate)
	pointRepo := repository.NewPointRepository(db)
	return &fixture{
		db:       db,
		payments: NewPaymentServer(db, repository.NewPaymentRepository(db), repository.NewRefundRepository(db), pointRepo),
		points:   NewPointServer(pointRepo),
	}
}

func (f *fixture) credit(t *testing.T, userID uint32, amount int64) {
	t.Helper()
	if _, err := f.points.CreditPoints(context.Background(), &pb.CreditPointsRequest{UserId: userID, Amount: amount, Reason: "test"}); err != nil {
		t.Fatalf("credit: %v", err)
	}
}

func (f *fixture) balance(t *testing.T, userID uint32) int64 {
	t.Helper()
	response, err := f.points.GetPointAccount(context.Background(), &pb.GetPointAccountRequest{UserId: userID})
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	return response.GetAccount().GetBalance()
}

func (f *fixture) countPayments(t *testing.T) int64 {
	t.Helper()
	var count int64
	if err := f.db.Model(&model.Payment{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func pay(userID uint32, amount int64, method, key string) *pb.CreatePaymentRequest {
	return &pb.CreatePaymentRequest{UserId: userID, ProductId: 10, Amount: amount, Currency: "POINT", PaymentMethod: method, IdempotencyKey: key}
}

func TestPointPaymentChargesTheBalance(t *testing.T) {
	f := newFixture(t)
	f.credit(t, 1, 1000)

	response, err := f.payments.CreatePayment(context.Background(), pay(1, 300, "point", "order-1"))
	if err != nil {
		t.Fatal(err)
	}

	if response.GetPayment().GetStatus() != model.PaymentStatusSucceeded {
		t.Errorf("status = %q, want succeeded", response.GetPayment().GetStatus())
	}
	if got := f.balance(t, 1); got != 700 {
		t.Errorf("balance = %d, want 700", got)
	}

	// 取引履歴には、決済への紐づき付きで消費が残る。
	transactions, _ := f.points.ListPointTransactions(context.Background(), &pb.ListPointTransactionsRequest{})
	var charge *pb.PointTransaction
	for _, transaction := range transactions.GetTransactions() {
		if transaction.GetType() == model.PointTransactionTypePayment {
			charge = transaction
		}
	}
	if charge == nil || charge.GetAmount() != -300 || charge.GetBalanceAfter() != 700 || charge.PaymentId == nil || charge.GetPaymentId() != response.GetPayment().GetId() {
		t.Errorf("unexpected point transaction: %+v", charge)
	}
}

// ポイントが足りないときは、決済も作られず、残高も変わらない(トランザクションごと巻き戻る)。
func TestInsufficientPointsRollsEverythingBack(t *testing.T) {
	f := newFixture(t)
	f.credit(t, 1, 100)

	_, err := f.payments.CreatePayment(context.Background(), pay(1, 300, "point", "order-1"))

	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", status.Code(err))
	}
	if got := f.countPayments(t); got != 0 {
		t.Errorf("%d payment row(s) were left behind", got)
	}
	if got := f.balance(t, 1); got != 100 {
		t.Errorf("balance = %d, want 100", got)
	}
}

func TestUserWithNoPointsCannotPay(t *testing.T) {
	f := newFixture(t)
	_, err := f.payments.CreatePayment(context.Background(), pay(42, 1, "point", "order-1"))
	if status.Code(err) != codes.FailedPrecondition {
		t.Errorf("code = %v, want FailedPrecondition", status.Code(err))
	}
}

// 同じ冪等性キーの再送(ネットワークのリトライ・二重クリック)では、二重に課金しない。
func TestRepeatedIdempotencyKeyChargesOnce(t *testing.T) {
	f := newFixture(t)
	f.credit(t, 1, 1000)
	ctx := context.Background()

	first, err := f.payments.CreatePayment(ctx, pay(1, 300, "point", "order-1"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.payments.CreatePayment(ctx, pay(1, 300, "point", "order-1"))
	if err != nil {
		t.Fatal(err)
	}

	if first.GetPayment().GetId() != second.GetPayment().GetId() {
		t.Errorf("a repeated key created a new payment: %d vs %d", first.GetPayment().GetId(), second.GetPayment().GetId())
	}
	if got := f.balance(t, 1); got != 700 {
		t.Errorf("balance = %d, want 700 (charged once)", got)
	}
	if got := f.countPayments(t); got != 1 {
		t.Errorf("payments = %d, want 1", got)
	}
}

// 同時に同じキーで来ても、課金は1回だけ。
func TestConcurrentRequestsWithTheSameKeyChargeOnce(t *testing.T) {
	f := newFixture(t)
	f.credit(t, 1, 1000)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.payments.CreatePayment(context.Background(), pay(1, 300, "point", "same-key")); err != nil {
				t.Logf("a concurrent duplicate request returned: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := f.countPayments(t); got != 1 {
		t.Errorf("payments = %d, want 1", got)
	}
	if got := f.balance(t, 1); got != 700 {
		t.Errorf("balance = %d, want 700 (charged once)", got)
	}
}

func TestOtherPaymentMethodsDoNotTouchPoints(t *testing.T) {
	f := newFixture(t)
	f.credit(t, 1, 1000)

	response, err := f.payments.CreatePayment(context.Background(), pay(1, 300, "card", "order-card"))
	if err != nil {
		t.Fatal(err)
	}
	if response.GetPayment().GetPaymentMethod() != "card" || response.GetPayment().GetStatus() != model.PaymentStatusSucceeded {
		t.Errorf("unexpected payment: %+v", response.GetPayment())
	}
	if got := f.balance(t, 1); got != 1000 {
		t.Errorf("a card payment changed the point balance to %d", got)
	}
}

func TestRefundReturnsPointsAndTracksTheStatus(t *testing.T) {
	f := newFixture(t)
	f.credit(t, 1, 1000)
	ctx := context.Background()
	created, err := f.payments.CreatePayment(ctx, pay(1, 300, "point", "order-1"))
	if err != nil {
		t.Fatal(err)
	}
	paymentID := created.GetPayment().GetId()

	partial, err := f.payments.RefundPayment(ctx, &pb.RefundPaymentRequest{PaymentId: paymentID, Amount: 100, Reason: "partial"})
	if err != nil {
		t.Fatal(err)
	}
	if partial.GetPayment().GetStatus() != model.PaymentStatusPartiallyRefunded {
		t.Errorf("status = %q, want partially_refunded", partial.GetPayment().GetStatus())
	}
	if got := f.balance(t, 1); got != 800 {
		t.Errorf("balance after a partial refund = %d, want 800", got)
	}

	// 残りより多くは返金できない。
	if _, err := f.payments.RefundPayment(ctx, &pb.RefundPaymentRequest{PaymentId: paymentID, Amount: 201}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("over-refund: code = %v, want InvalidArgument", status.Code(err))
	}
	if got := f.balance(t, 1); got != 800 {
		t.Errorf("a rejected refund changed the balance to %d", got)
	}

	full, err := f.payments.RefundPayment(ctx, &pb.RefundPaymentRequest{PaymentId: paymentID, Amount: 200})
	if err != nil {
		t.Fatal(err)
	}
	if full.GetPayment().GetStatus() != model.PaymentStatusRefunded {
		t.Errorf("status = %q, want refunded", full.GetPayment().GetStatus())
	}
	if got := f.balance(t, 1); got != 1000 {
		t.Errorf("balance after a full refund = %d, want 1000", got)
	}

	// 返金しきった決済は、もう返金できない。
	if _, err := f.payments.RefundPayment(ctx, &pb.RefundPaymentRequest{PaymentId: paymentID, Amount: 1}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("refund of a fully refunded payment: code = %v, want FailedPrecondition", status.Code(err))
	}

	refunds, _ := f.payments.ListRefunds(ctx, &pb.ListRefundsRequest{PaymentId: &paymentID})
	if len(refunds.GetRefunds()) != 2 {
		t.Errorf("refund history has %d entries, want 2", len(refunds.GetRefunds()))
	}
}

func TestCancelOnlyWorksOnPendingPayments(t *testing.T) {
	f := newFixture(t)
	f.credit(t, 1, 1000)
	ctx := context.Background()

	// 現状の決済はその場で確定(succeeded)するので、キャンセルはできない。
	created, _ := f.payments.CreatePayment(ctx, pay(1, 300, "point", "order-1"))
	if _, err := f.payments.CancelPayment(ctx, &pb.CancelPaymentRequest{Id: created.GetPayment().GetId()}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("cancel of a succeeded payment: code = %v, want FailedPrecondition", status.Code(err))
	}

	pending := &model.Payment{UserID: 1, ProductID: 10, Amount: 50, Currency: "JPY", PaymentMethod: "card", Status: model.PaymentStatusPending, IdempotencyKey: "pending-1"}
	if err := f.db.Create(pending).Error; err != nil {
		t.Fatal(err)
	}
	canceled, err := f.payments.CancelPayment(ctx, &pb.CancelPaymentRequest{Id: uint32(pending.ID)})
	if err != nil || canceled.GetPayment().GetStatus() != model.PaymentStatusCanceled {
		t.Errorf("cancel of a pending payment: %v / %+v", err, canceled)
	}

	if _, err := f.payments.CancelPayment(ctx, &pb.CancelPaymentRequest{Id: 99999}); status.Code(err) != codes.NotFound {
		t.Errorf("cancel of a missing payment: code = %v, want NotFound", status.Code(err))
	}
}

func TestListPaymentsFiltersByUser(t *testing.T) {
	f := newFixture(t)
	f.credit(t, 1, 1000)
	f.credit(t, 2, 1000)
	ctx := context.Background()
	f.payments.CreatePayment(ctx, pay(1, 10, "point", "a"))
	f.payments.CreatePayment(ctx, pay(1, 10, "point", "b"))
	f.payments.CreatePayment(ctx, pay(2, 10, "point", "c"))

	userOne := uint32(1)
	mine, err := f.payments.ListPayments(ctx, &pb.ListPaymentsRequest{UserId: &userOne})
	if err != nil || len(mine.GetPayments()) != 2 {
		t.Fatalf("user 1 has %d payments, err=%v; want 2", len(mine.GetPayments()), err)
	}
	all, _ := f.payments.ListPayments(ctx, &pb.ListPaymentsRequest{})
	if len(all.GetPayments()) != 3 {
		t.Errorf("all payments = %d, want 3", len(all.GetPayments()))
	}
}

func TestPointAccountAndManualAdjustments(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// 一度も取引がないユーザーはエラーではなく、残高0として返る。
	if got := f.balance(t, 7); got != 0 {
		t.Errorf("new user balance = %d, want 0", got)
	}

	credited, err := f.points.CreditPoints(ctx, &pb.CreditPointsRequest{UserId: 7, Amount: 1600, Reason: "welcome"})
	if err != nil || credited.GetAccount().GetBalance() != 1600 || credited.GetTransaction().GetType() != model.PointTransactionTypeCredit {
		t.Fatalf("credit: %v / %+v", err, credited)
	}

	debited, err := f.points.DebitPoints(ctx, &pb.DebitPointsRequest{UserId: 7, Amount: 600, Reason: "adjust"})
	if err != nil || debited.GetAccount().GetBalance() != 1000 {
		t.Fatalf("debit: %v / %+v", err, debited)
	}

	// 残高を超える消費は拒否し、残高は変えない。
	if _, err := f.points.DebitPoints(ctx, &pb.DebitPointsRequest{UserId: 7, Amount: 1001, Reason: "too much"}); status.Code(err) != codes.FailedPrecondition {
		t.Errorf("overdraw: code = %v, want FailedPrecondition", status.Code(err))
	}
	if got := f.balance(t, 7); got != 1000 {
		t.Errorf("balance = %d, want 1000", got)
	}

	// 履歴は新しい順で、ユーザーごとに絞り込める。
	f.credit(t, 8, 5)
	userSeven := uint32(7)
	history, _ := f.points.ListPointTransactions(ctx, &pb.ListPointTransactionsRequest{UserId: &userSeven})
	if len(history.GetTransactions()) != 2 || history.GetTransactions()[0].GetType() != model.PointTransactionTypeDebit {
		t.Errorf("unexpected history: %+v", history.GetTransactions())
	}
}

// 同時に消費されても、残高より多くは使えない(行ロックが効いている)。
func TestConcurrentSpendingNeverOverdraws(t *testing.T) {
	f := newFixture(t)
	f.credit(t, 1, 500)

	var wg sync.WaitGroup
	var mu sync.Mutex
	succeeded := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := f.points.DebitPoints(context.Background(), &pb.DebitPointsRequest{UserId: 1, Amount: 100, Reason: "spend"})
			if err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			} else if status.Code(err) != codes.FailedPrecondition {
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if succeeded != 5 {
		t.Errorf("%d debits succeeded, want 5", succeeded)
	}
	if got := f.balance(t, 1); got != 0 {
		t.Errorf("final balance = %d, want 0", got)
	}
}
