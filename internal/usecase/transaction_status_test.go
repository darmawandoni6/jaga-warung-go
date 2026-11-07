package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"jaga-warung-go/internal/domain"
)

func TestDeriveTransactionStatus(t *testing.T) {
	cases := []struct {
		method string
		total  int64
		paid   int64
		want   string
	}{
		{domain.PaymentMethodCash, 4000, 5000, domain.TransactionStatusPaid},
		{domain.PaymentMethodCash, 4000, 4000, domain.TransactionStatusPaid},
		{domain.PaymentMethodNonCash, 4000, 0, domain.TransactionStatusPending},
		{domain.PaymentMethodNonCash, 4000, 4000, domain.TransactionStatusPaid},
		{domain.PaymentMethodNonCash, 0, 0, domain.TransactionStatusPaid},
	}
	for _, tc := range cases {
		if got := domain.DeriveTransactionStatus(tc.method, tc.total, tc.paid); got != tc.want {
			t.Errorf("DeriveTransactionStatus(%q, %d, %d) = %q, want %q",
				tc.method, tc.total, tc.paid, got, tc.want)
		}
	}
}

func TestTransactionCreate_CashStartsPaid(t *testing.T) {
	repo := &fakeTransactionRepo{}
	uc := NewTransactionUsecase(repo, nil)

	_, err := uc.Create(context.Background(), &domain.CreateTransactionRequest{
		TotalAmount:   2000,
		PaymentAmount: 2000,
		Items: []domain.CreateTransactionItemRequest{
			{ProductBarcode: "A", SellPrice: 2000, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.created.Status != domain.TransactionStatusPaid {
		t.Fatalf("cash sale must be paid at create, got %q", repo.created.Status)
	}
	if repo.created.PaidAt == nil {
		t.Fatal("paid_at must be set for a paid sale")
	}
}

func TestTransactionCreate_NonCashStartsPending(t *testing.T) {
	repo := &fakeTransactionRepo{}
	uc := NewTransactionUsecase(repo, nil)

	_, err := uc.Create(context.Background(), &domain.CreateTransactionRequest{
		TotalAmount: 2000,
		Method:      domain.PaymentMethodNonCash,
		Items: []domain.CreateTransactionItemRequest{
			{ProductBarcode: "A", SellPrice: 2000, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.created.Status != domain.TransactionStatusPending {
		t.Fatalf("non_cash sale must start pending, got %q", repo.created.Status)
	}
	if repo.created.PaidAt != nil {
		t.Fatal("paid_at must stay nil for an unpaid sale")
	}
}

func TestTransactionPatch_SwitchesNonCashToCash(t *testing.T) {
	existing := &domain.Transaction{
		ID:          uuid.New(),
		Method:      domain.PaymentMethodNonCash,
		Status:      domain.TransactionStatusPending,
		TotalAmount: 4000,
	}
	repo := &fakeTransactionRepo{getResult: existing}
	uc := NewTransactionUsecase(repo, nil)

	got, err := uc.Patch(context.Background(), existing.ID, &domain.PatchTransactionRequest{
		Method:        strPtr(domain.PaymentMethodCash),
		PaymentAmount: int64Ptr(5000),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Method != domain.PaymentMethodCash {
		t.Fatalf("method not switched, got %q", got.Method)
	}
	if got.PaymentAmount != 5000 || got.ChangeAmount != 1000 {
		t.Fatalf("tender not recorded: %+v", got)
	}
	if got.Status != domain.TransactionStatusPaid || got.PaidAt == nil {
		t.Fatalf("switch must settle the sale: status=%q paid_at=%v", got.Status, got.PaidAt)
	}
}

func TestTransactionPatch_SwitchRejectsUnderpayment(t *testing.T) {
	existing := &domain.Transaction{
		ID:          uuid.New(),
		Method:      domain.PaymentMethodNonCash,
		Status:      domain.TransactionStatusExpired,
		TotalAmount: 4000,
	}
	uc := NewTransactionUsecase(&fakeTransactionRepo{getResult: existing}, nil)

	_, err := uc.Patch(context.Background(), existing.ID, &domain.PatchTransactionRequest{
		Method:        strPtr(domain.PaymentMethodCash),
		PaymentAmount: int64Ptr(3000),
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestTransactionPatch_SwitchRequiresTender(t *testing.T) {
	existing := &domain.Transaction{
		ID:          uuid.New(),
		Method:      domain.PaymentMethodNonCash,
		Status:      domain.TransactionStatusPending,
		TotalAmount: 4000,
	}
	uc := NewTransactionUsecase(&fakeTransactionRepo{getResult: existing}, nil)

	_, err := uc.Patch(context.Background(), existing.ID, &domain.PatchTransactionRequest{
		Method: strPtr(domain.PaymentMethodCash),
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestTransactionPatch_RejectsCashToNonCash(t *testing.T) {
	existing := &domain.Transaction{
		ID:            uuid.New(),
		Method:        domain.PaymentMethodCash,
		Status:        domain.TransactionStatusPaid,
		TotalAmount:   4000,
		PaymentAmount: 4000,
	}
	uc := NewTransactionUsecase(&fakeTransactionRepo{getResult: existing}, nil)

	// Paid sales reject method changes before the direction check even runs.
	_, err := uc.Patch(context.Background(), existing.ID, &domain.PatchTransactionRequest{
		Method:        strPtr(domain.PaymentMethodNonCash),
		PaymentAmount: int64Ptr(4000),
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestTransactionPatch_PaidIsTerminal(t *testing.T) {
	existing := &domain.Transaction{
		ID:            uuid.New(),
		Method:        domain.PaymentMethodCash,
		Status:        domain.TransactionStatusPaid,
		TotalAmount:   4000,
		PaymentAmount: 4000,
	}
	repo := &fakeTransactionRepo{getResult: existing}
	uc := NewTransactionUsecase(repo, nil)

	_, err := uc.Patch(context.Background(), existing.ID, &domain.PatchTransactionRequest{
		PaymentAmount: int64Ptr(1000),
	})
	if !errors.Is(err, domain.ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
	if repo.updated != nil {
		t.Fatal("paid sale must not be written")
	}
}

func TestSettleTransaction_MarksPaidOnce(t *testing.T) {
	txID := uuid.New()
	tx := &domain.Transaction{
		ID:          txID,
		Method:      domain.PaymentMethodNonCash,
		Status:      domain.TransactionStatusPending,
		TotalAmount: 4000,
	}
	repo := &fakeTransactionRepo{getResult: tx}
	uc := &paymentUsecase{txRepo: repo}

	if err := uc.settleTransaction(context.Background(), &domain.Payment{
		OrderID: uuid.New(), TargetID: txID, Amount: 4000,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updated.Status != domain.TransactionStatusPaid {
		t.Fatalf("expected paid, got %q", repo.updated.Status)
	}
	if repo.updated.PaymentAmount != 4000 || repo.updated.ChangeAmount != 0 {
		t.Fatalf("settlement amounts wrong: %+v", repo.updated)
	}
	if repo.updated.PaidAt == nil {
		t.Fatal("paid_at must be set")
	}
}

func TestSettleTransaction_DoesNotDoubleCountPaidSale(t *testing.T) {
	txID := uuid.New()
	paidAt := time.Now().UTC().Add(-time.Hour)
	tx := &domain.Transaction{
		ID:            txID,
		Method:        domain.PaymentMethodCash,
		Status:        domain.TransactionStatusPaid,
		TotalAmount:   4000,
		PaymentAmount: 4000,
		PaidAt:        &paidAt,
	}
	repo := &fakeTransactionRepo{getResult: tx}
	uc := &paymentUsecase{txRepo: repo}

	if err := uc.settleTransaction(context.Background(), &domain.Payment{
		OrderID: uuid.New(), TargetID: txID, Amount: 4000,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updated != nil {
		t.Fatal("an already-paid sale must not be rewritten by a late settlement")
	}
	if tx.PaymentAmount != 4000 {
		t.Fatalf("payment_amount was double-counted: %d", tx.PaymentAmount)
	}
	if !tx.PaidAt.Equal(paidAt) {
		t.Fatal("paid_at was overwritten")
	}
}

func TestExpireTarget_OnlyWhenNoNewerAttemptPending(t *testing.T) {
	txID := uuid.New()
	attempt := &domain.Payment{
		ID:         uuid.New(),
		TargetType: "transaction",
		TargetID:   txID,
		Status:     domain.PaymentStatusPending,
	}
	tx := &domain.Transaction{
		ID:          txID,
		Method:      domain.PaymentMethodNonCash,
		Status:      domain.TransactionStatusPending,
		TotalAmount: 4000,
	}
	repo := &fakePaymentRepo{pending: attempt}
	txRepo := &fakeTransactionRepo{getResult: tx}
	uc := &paymentUsecase{repo: repo, txRepo: txRepo}

	if err := uc.expireTarget(context.Background(), attempt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txRepo.updated == nil || txRepo.updated.Status != domain.TransactionStatusExpired {
		t.Fatalf("expected transaction expired, got %+v", txRepo.updated)
	}

	// A newer attempt is pending: expiring the old one must not touch the sale.
	other := &domain.Payment{ID: uuid.New(), TargetType: "transaction", TargetID: txID}
	fresh := &domain.Transaction{ID: txID, Status: domain.TransactionStatusPending}
	txRepo2 := &fakeTransactionRepo{getResult: fresh}
	uc2 := &paymentUsecase{
		repo:   &fakePaymentRepo{pending: other},
		txRepo: txRepo2,
	}
	if err := uc2.expireTarget(context.Background(), attempt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txRepo2.updated != nil {
		t.Fatal("a stale attempt expired a sale that has a newer pending attempt")
	}
}

func TestCreateSnap_RejectsPaidTarget(t *testing.T) {
	txID := uuid.New()
	repo := &fakePaymentRepo{}
	txRepo := &fakeTransactionRepo{getResult: &domain.Transaction{
		ID:          txID,
		Method:      domain.PaymentMethodCash,
		Status:      domain.TransactionStatusPaid,
		TotalAmount: 4000,
	}}
	uc := NewPaymentUsecase(repo, nil, txRepo, nil, nil, "server-key", "sandbox")

	_, err := uc.CreateSnap(context.Background(), &domain.CreatePaymentRequest{
		TargetType: "transaction",
		TargetID:   txID,
	})
	if err == nil || err.Error() != "already_paid" {
		t.Fatalf("expected already_paid, got %v", err)
	}
	if repo.created != nil {
		t.Fatal("a paid sale must not open a new gateway order")
	}
}
