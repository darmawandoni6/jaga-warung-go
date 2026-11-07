package usecase

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

func sha512Hex(s string) string {
	sum := sha512.Sum512([]byte(s))
	return hex.EncodeToString(sum[:])
}

// fakePaymentRepo is enough of PaymentRepository to test the flows that must not
// touch Midtrans or Postgres.
type fakePaymentRepo struct {
	pending  *domain.Payment
	pendingE error

	byOrder map[uuid.UUID]*domain.Payment
	created *domain.Payment
	updated *domain.Payment
	listRes []*domain.Payment
	listTot int64
}

func (f *fakePaymentRepo) Create(ctx context.Context, p *domain.Payment) error {
	f.created = p
	return nil
}

func (f *fakePaymentRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	return f.updated, nil
}

func (f *fakePaymentRepo) GetByOrderID(ctx context.Context, orderID uuid.UUID) (*domain.Payment, error) {
	if p, ok := f.byOrder[orderID]; ok {
		return p, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakePaymentRepo) FindPendingByTarget(ctx context.Context, targetType string, targetID uuid.UUID) (*domain.Payment, error) {
	if f.pendingE != nil {
		return nil, f.pendingE
	}
	if f.pending == nil {
		return nil, domain.ErrNotFound
	}
	return f.pending, nil
}

func (f *fakePaymentRepo) List(ctx context.Context, page, limit int, targetType string, targetID *uuid.UUID) ([]*domain.Payment, int64, error) {
	return f.listRes, f.listTot, nil
}

func (f *fakePaymentRepo) Update(ctx context.Context, p *domain.Payment) error {
	f.updated = p
	return nil
}

// fakeDebtPay counts Create calls so double settlement is visible.
type fakeDebtPay struct {
	calls   int
	created *domain.DebtPayment
}

func (f *fakeDebtPay) List(ctx context.Context, page, limit int, debtID *uuid.UUID) (*domain.DebtPaymentListResult, error) {
	return nil, nil
}

func (f *fakeDebtPay) Create(ctx context.Context, req *domain.CreateDebtPaymentRequest) (*domain.DebtPaymentResult, error) {
	f.calls++
	f.created = &domain.DebtPayment{ID: uuid.New(), DebtID: req.DebtID, Amount: req.Amount, Method: req.Method}
	return &domain.DebtPaymentResult{Payment: f.created}, nil
}

func (f *fakeDebtPay) Get(ctx context.Context, id uuid.UUID) (*domain.DebtPayment, error) {
	return nil, nil
}

func (f *fakeDebtPay) Patch(ctx context.Context, id uuid.UUID, req *domain.PatchDebtPaymentRequest) (*domain.DebtPayment, error) {
	return nil, nil
}

func (f *fakeDebtPay) Delete(ctx context.Context, id uuid.UUID) error { return nil }

func TestParseGrossAmount(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"45000.00", 45000, true},
		{"0.00", 0, true},
		{"45000", 45000, true},
		{"45000.50", 0, false},
		{"", 0, false},
	}
	for _, tc := range cases {
		got, err := parseGrossAmount(tc.in)
		if tc.ok && (err != nil || got != tc.want) {
			t.Errorf("parseGrossAmount(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
		if !tc.ok && err == nil {
			t.Errorf("parseGrossAmount(%q) must fail, got %d", tc.in, got)
		}
	}
}

func TestValidSignature(t *testing.T) {
	uc := &paymentUsecase{serverKey: "server-key"}
	order, code, gross := "order-1", "200", "45000.00"

	// Midtrans: sha512(order_id + status_code + gross_amount + server_key).
	good := sha512Hex(order + code + gross + "server-key")

	if !uc.validSignature(order, code, gross, good) {
		t.Fatal("correct signature was rejected")
	}
	if uc.validSignature(order, code, gross, "deadbeef") {
		t.Fatal("wrong signature was accepted")
	}
	if uc.validSignature(order, code, gross, "") {
		t.Fatal("empty signature was accepted")
	}
}

func TestCreateSnapReusesPendingPayment(t *testing.T) {
	targetID := uuid.New()
	pending := &domain.Payment{
		ID:          uuid.New(),
		OrderID:     uuid.New(),
		TargetType:  "transaction",
		TargetID:    targetID,
		Status:      domain.PaymentStatusPending,
		SnapToken:   "token-1",
		RedirectURL: "https://example.test/pay",
		CreatedAt:   time.Now().UTC(),
	}
	repo := &fakePaymentRepo{pending: pending}
	txRepo := &fakeTransactionRepo{getResult: &domain.Transaction{
		ID: targetID, Status: domain.TransactionStatusPending, TotalAmount: 4000,
	}}
	uc := NewPaymentUsecase(repo, nil, txRepo, nil, nil, "server-key", "sandbox")

	res, err := uc.CreateSnap(context.Background(), &domain.CreatePaymentRequest{
		TargetType: "transaction",
		TargetID:   pending.TargetID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Payment.OrderID != pending.OrderID {
		t.Fatalf("expected reuse of %s, got %s", pending.OrderID, res.Payment.OrderID)
	}
	if repo.created != nil {
		t.Fatal("a second Midtrans order must not be created while the first is payable")
	}
}

func TestReusePending_ExpiresStaleRow(t *testing.T) {
	stale := &domain.Payment{
		ID:         uuid.New(),
		TargetType: "transaction",
		TargetID:   uuid.New(),
		Status:     domain.PaymentStatusPending,
		CreatedAt:  time.Now().UTC().Add(-snapTTL - time.Minute),
	}
	repo := &fakePaymentRepo{pending: stale}
	uc := &paymentUsecase{repo: repo}

	got, err := uc.reusePending(context.Background(), "transaction", stale.TargetID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatal("a stale attempt must not be reused")
	}
	if repo.updated == nil || repo.updated.Status != domain.PaymentStatusExpired {
		t.Fatalf("stale pending was not expired, got %+v", repo.updated)
	}
}

func TestReusePending_NoAttempt(t *testing.T) {
	repo := &fakePaymentRepo{}
	uc := &paymentUsecase{repo: repo}

	got, err := uc.reusePending(context.Background(), "transaction", uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestSettleDebtIsIdempotent(t *testing.T) {
	dpID := uuid.New()
	p := &domain.Payment{
		TargetType:    "debt",
		TargetID:      uuid.New(),
		Amount:        5000,
		DebtPaymentID: &dpID,
	}
	fake := &fakeDebtPay{}
	uc := &paymentUsecase{debtPay: fake}

	if err := uc.settleDebt(context.Background(), p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("already-settled payment booked %d extra debt payments", fake.calls)
	}
}

func TestSettleDebtRecordsMethodAndLink(t *testing.T) {
	p := &domain.Payment{TargetType: "debt", TargetID: uuid.New(), Amount: 5000}
	fake := &fakeDebtPay{}
	uc := &paymentUsecase{debtPay: fake}

	if err := uc.settleDebt(context.Background(), p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("expected 1 debt payment, got %d", fake.calls)
	}
	if fake.created.Method != domain.PaymentMethodQRIS {
		t.Fatalf("expected method non_cash, got %q", fake.created.Method)
	}
	if p.DebtPaymentID == nil || *p.DebtPaymentID != fake.created.ID {
		t.Fatal("payment was not linked to the created debt payment")
	}
}

// non_cash sales are paid by the gateway, so the client-supplied amount is ignored
// and the cash tender check does not apply.
func TestTransactionCreate_NonCashForcesZeroPayment(t *testing.T) {
	repo := &fakeTransactionRepo{}
	uc := NewTransactionUsecase(repo, nil)

	_, err := uc.Create(context.Background(), &domain.CreateTransactionRequest{
		TotalAmount:   4000,
		PaymentAmount: 0,
		Method:        domain.PaymentMethodNonCash,
		Items: []domain.CreateTransactionItemRequest{
			{ProductBarcode: "899", SellPrice: 2000, Quantity: 2},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.created.Method != domain.PaymentMethodNonCash {
		t.Fatalf("expected method non_cash, got %q", repo.created.Method)
	}
	if repo.created.PaymentAmount != 0 || repo.created.ChangeAmount != 0 {
		t.Fatalf("non_cash sale must store zero tender, got %+v", repo.created)
	}
}

func TestTransactionCreate_RejectsUnknownMethod(t *testing.T) {
	uc := NewTransactionUsecase(&fakeTransactionRepo{}, nil)

	_, err := uc.Create(context.Background(), &domain.CreateTransactionRequest{
		TotalAmount: 1000,
		Method:      "transfer",
		Items: []domain.CreateTransactionItemRequest{
			{ProductBarcode: "A", SellPrice: 1000, Quantity: 1},
		},
	})
	if err != domain.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestTransactionPatch_RejectsNonCashAmounts(t *testing.T) {
	existing := &domain.Transaction{ID: uuid.New(), Method: domain.PaymentMethodNonCash, TotalAmount: 1000}
	repo := &fakeTransactionRepo{getResult: existing}
	uc := NewTransactionUsecase(repo, nil)

	_, err := uc.Patch(context.Background(), existing.ID, &domain.PatchTransactionRequest{
		PaymentAmount: int64Ptr(1000),
	})
	if err != domain.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestHandleNotification_RejectsNonUUIDOrderID(t *testing.T) {
	uc := &paymentUsecase{}
	// The dashboard "Test notification" button sends this shape. It can never
	// map to a row, so it must be ErrBadRequest (handler → 200 ignored), not a
	// retryable 500.
	err := uc.HandleNotification(context.Background(), map[string]interface{}{
		"order_id": "payment_notif_test_M211170412_e5524d63-9f5f-4a80-bf32-3ae556d5a5b8",
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}
