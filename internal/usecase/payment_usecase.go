package usecase

import (
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	mt "github.com/midtrans/midtrans-go"
	coreapiClient "github.com/midtrans/midtrans-go/coreapi"
	snapClient "github.com/midtrans/midtrans-go/snap"

	"jaga-warung-go/internal/domain"
	"jaga-warung-go/pkg/kafka"
)

// snapTTL must match the Snap request's expiry below: a pending row younger than
// this is still payable, so a retry reuses it instead of opening a second order.
const snapTTL = 15 * time.Minute

type paymentUsecase struct {
	repo      domain.PaymentRepository
	debtRepo  domain.DebtRepository
	txRepo    domain.TransactionRepository
	debtPay   domain.DebtPaymentUseCase
	pub       *kafka.Producer
	serverKey string
	snap      *snapClient.Client
	core      *coreapiClient.Client
	enabled   bool
}

func NewPaymentUsecase(repo domain.PaymentRepository, debtRepo domain.DebtRepository, txRepo domain.TransactionRepository, debtPay domain.DebtPaymentUseCase, pub *kafka.Producer, serverKey, environment string) domain.PaymentUseCase {
	uc := &paymentUsecase{
		repo:      repo,
		debtRepo:  debtRepo,
		txRepo:    txRepo,
		debtPay:   debtPay,
		pub:       pub,
		serverKey: serverKey,
	}

	env := mt.Sandbox
	if environment == "production" {
		env = mt.Production
	}

	// Explicit clients: the package-level helpers read midtrans.ServerKey, a
	// global nobody in this repo sets, so they would always 401.
	if serverKey != "" {
		uc.snap = &snapClient.Client{}
		uc.snap.New(serverKey, env)
		uc.core = &coreapiClient.Client{}
		uc.core.New(serverKey, env)
		uc.enabled = true
	}
	return uc
}

func (uc *paymentUsecase) CreateSnap(ctx context.Context, req *domain.CreatePaymentRequest) (*domain.PaymentResult, error) {
	if !uc.enabled {
		return nil, domain.NewDomainError("midtrans_not_configured", 503)
	}

	// Resolve the target first: an already-paid sale must never be charged again,
	// and an expired one is reopened by this new attempt.
	amount, wasExpired, err := uc.resolveTarget(ctx, req)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, domain.ErrBadRequest
	}

	// A retry while the QR is still payable must return the same order, not a
	// second Midtrans order for the same sale.
	reused, err := uc.reusePending(ctx, req.TargetType, req.TargetID)
	if err != nil {
		return nil, err
	}
	if reused != nil {
		slog.Default().Info("payment reused",
			slog.String("order_id", reused.OrderID.String()),
			slog.String("target_type", reused.TargetType))
		return &domain.PaymentResult{Payment: reused, RedirectURL: reused.RedirectURL, SnapToken: reused.SnapToken}, nil
	}

	if wasExpired {
		if err := uc.reopenTransaction(ctx, req.TargetID); err != nil {
			return nil, err
		}
	}

	orderID := uuid.New()
	snapReq := &snapClient.Request{
		TransactionDetails: mt.TransactionDetails{
			OrderID:  orderID.String(),
			GrossAmt: amount,
		},
		// Midtrans Snap accepts "gopay", "shopeepay", and "other_qris" for QRIS.
		// Passing invalid strings like "qris" causes "No payment channels available".
		// EnabledPayments: []snapClient.SnapPaymentType{"qris"},
		Expiry: &snapClient.ExpiryDetails{
			StartTime: time.Now().UTC().Format("2006-01-02 15:04:05 -0700"),
			Unit:      "minute",
			Duration:  15,
		},
	}
	resp, midErr := uc.snap.CreateTransaction(snapReq)
	if midErr != nil {
		return nil, fmt.Errorf("create snap transaction: %w", midErr)
	}

	payment := &domain.Payment{
		OrderID:     orderID,
		TargetType:  req.TargetType,
		TargetID:    req.TargetID,
		Amount:      amount,
		Status:      domain.PaymentStatusPending,
		SnapToken:   resp.Token,
		RedirectURL: resp.RedirectURL,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := uc.repo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("persist payment: %w", err)
	}

	slog.Default().Info("payment created",
		slog.String("id", payment.ID.String()),
		slog.String("order_id", orderID.String()),
		slog.Int64("amount", amount))
	return &domain.PaymentResult{Payment: payment, RedirectURL: resp.RedirectURL, SnapToken: resp.Token}, nil
}

func (uc *paymentUsecase) Get(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	return uc.repo.Get(ctx, id)
}

func (uc *paymentUsecase) List(ctx context.Context, page, limit int, targetType string, targetID *uuid.UUID) (*domain.PaymentListResult, error) {
	page, limit = normalizePage(page, limit)
	data, total, err := uc.repo.List(ctx, page, limit, targetType, targetID)
	if err != nil {
		return nil, fmt.Errorf("payment list: %w", err)
	}
	return &domain.PaymentListResult{
		Data: data,
		Meta: domain.ProductListMeta{Page: page, Limit: limit, Total: total},
	}, nil
}

// Verify asks Midtrans about this order right now and returns the response
// verbatim. This — not the stored row — is the proof that an order_id belongs to
// this payment and that it settled for this amount.
func (uc *paymentUsecase) Verify(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	p, err := uc.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !uc.enabled {
		return nil, domain.NewDomainError("midtrans_not_configured", 503)
	}

	statusResp, midErr := uc.core.CheckTransaction(p.OrderID.String())
	if midErr != nil {
		return nil, fmt.Errorf("check transaction: %w", midErr)
	}
	if statusResp == nil {
		return nil, domain.ErrNotFound
	}

	uc.recordMidtrans(p, statusResp)
	if err := uc.repo.Update(ctx, p); err != nil {
		return nil, fmt.Errorf("persist verification: %w", err)
	}
	out, err := statusToMap(statusResp)
	if err != nil {
		return nil, err
	}
	// The signature is derived from the server key; keep it in the audit column,
	// not in an API response.
	delete(out, "signature_key")
	return out, nil
}

func (uc *paymentUsecase) HandleNotification(ctx context.Context, raw map[string]interface{}) error {
	orderIDRaw, _ := raw["order_id"].(string)
	if orderIDRaw == "" {
		return domain.ErrBadRequest
	}
	orderID, err := uuid.Parse(orderIDRaw)
	if err != nil {
		// Deterministic: Midtrans dashboard "Test notification" sends a fake
		// order_id (payment_notif_test_...) and non-UUID ids can never match a
		// row. Treat as bad request so the handler answers 200 and no retry.
		return domain.ErrBadRequest
	}

	existing, err := uc.repo.GetByOrderID(ctx, orderID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("lookup payment: %w", err)
	}

	// The signature proves the body came from Midtrans, but it is keyed to the
	// server key that created the order. A rotated key makes old attempts
	// unverifiable, so a mismatch is logged and the CheckTransaction re-query
	// below stays the authority: it is a server-to-server call under our current
	// key, and nothing settles unless Midtrans itself reports it for this exact
	// order and amount.
	statusCode, _ := raw["status_code"].(string)
	grossAmount, _ := raw["gross_amount"].(string)
	signature, _ := raw["signature_key"].(string)
	if !uc.validSignature(orderIDRaw, statusCode, grossAmount, signature) {
		slog.Default().Warn("notification signature mismatch",
			slog.String("order_id", orderIDRaw))
	}
	if !uc.enabled {
		return errors.New("midtrans not configured")
	}

	statusResp, midErr := uc.core.CheckTransaction(orderIDRaw)
	if midErr != nil {
		return fmt.Errorf("check transaction: %w", midErr)
	}
	if statusResp == nil {
		return domain.ErrNotFound
	}
	// The response must be about the order we hold, for the amount we asked.
	if statusResp.OrderID != orderIDRaw {
		return domain.ErrBadRequest
	}
	gross, err := parseGrossAmount(statusResp.GrossAmount)
	if err != nil || gross != existing.Amount {
		return domain.ErrBadRequest
	}

	next := translateStatus(statusResp.TransactionStatus, statusResp.FraudStatus)
	// Side effect first, status last: if the process dies in between, Midtrans
	// retries and the side effect runs again — and it is idempotent. The other
	// order loses the settlement forever.
	switch next {
	case domain.PaymentStatusPaid:
		if err := uc.applySettlement(ctx, existing); err != nil {
			return err
		}
	case domain.PaymentStatusExpired, domain.PaymentStatusCanceled:
		if err := uc.expireTarget(ctx, existing); err != nil {
			return err
		}
	}
	existing.Status = next
	uc.recordMidtrans(existing, statusResp)
	if err := uc.repo.Update(ctx, existing); err != nil {
		return fmt.Errorf("update payment status: %w", err)
	}
	slog.Default().Info("payment settled", slog.String("order_id", orderIDRaw), slog.String("status", next))
	return nil
}

// recordMidtrans copies the Midtrans-side identity onto the row. Every field
// comes from CheckTransaction, never from the webhook body.
func (uc *paymentUsecase) recordMidtrans(p *domain.Payment, resp *coreapiClient.TransactionStatusResponse) {
	now := time.Now().UTC()
	if resp.TransactionID != "" {
		p.MidtransTransactionID = &resp.TransactionID
	}
	if resp.PaymentType != "" {
		p.PaymentType = &resp.PaymentType
	}
	if resp.StatusCode != "" {
		p.StatusCode = &resp.StatusCode
	}
	if resp.SettlementTime != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", resp.SettlementTime); err == nil {
			t = t.UTC()
			p.SettledAt = &t
		}
	}
	if raw, err := json.Marshal(resp); err == nil {
		p.RawStatus = raw
	}
	p.VerifiedAt = &now
	p.UpdatedAt = now
}

// reusePending returns the still-payable attempt for a target, or nil after
// closing a stale one so the unique pending index stops blocking the next try.
// Nothing was paid on a stale attempt, so no side effect runs.
func (uc *paymentUsecase) reusePending(ctx context.Context, targetType string, targetID uuid.UUID) (*domain.Payment, error) {
	p, err := uc.repo.FindPendingByTarget(ctx, targetType, targetID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find pending payment: %w", err)
	}
	if time.Since(p.CreatedAt) < snapTTL {
		return p, nil
	}
	p.Status = domain.PaymentStatusExpired
	p.UpdatedAt = time.Now().UTC()
	if err := uc.repo.Update(ctx, p); err != nil {
		return nil, fmt.Errorf("expire stale payment: %w", err)
	}
	return nil, nil
}

// resolveTarget returns the amount to charge and whether the transaction was
// expired (a new attempt reopens it). It rejects a sale/debt that is already
// paid, so the gateway can never collect twice.
func (uc *paymentUsecase) resolveTarget(ctx context.Context, req *domain.CreatePaymentRequest) (amount int64, wasExpired bool, err error) {
	switch req.TargetType {
	case "transaction":
		tx, err := uc.txRepo.Get(ctx, req.TargetID)
		if err != nil {
			return 0, false, err
		}
		if tx.Status == domain.TransactionStatusPaid {
			return 0, false, domain.NewDomainError("already_paid", 409)
		}
		return tx.TotalAmount, tx.Status == domain.TransactionStatusExpired, nil
	case "debt":
		d, err := uc.debtRepo.Get(ctx, req.TargetID)
		if err != nil {
			return 0, false, err
		}
		if d.Status == domain.DebtStatusPaid {
			return 0, false, domain.NewDomainError("already_paid", 409)
		}
		return d.TotalDebt - d.PaidAmount, false, nil
	default:
		return 0, false, domain.ErrBadRequest
	}
}

// reopenTransaction moves an expired sale back to pending when a fresh QRIS
// attempt is opened. paid is guarded earlier and stays terminal.
func (uc *paymentUsecase) reopenTransaction(ctx context.Context, id uuid.UUID) error {
	tx, err := uc.txRepo.Get(ctx, id)
	if err != nil {
		return err
	}
	if tx.Status != domain.TransactionStatusExpired {
		return nil
	}
	tx.Status = domain.TransactionStatusPending
	if err := uc.txRepo.Update(ctx, tx); err != nil {
		return fmt.Errorf("reopen transaction: %w", err)
	}
	uc.publishTxUpdated(ctx, tx)
	return nil
}

func (uc *paymentUsecase) applySettlement(ctx context.Context, p *domain.Payment) error {
	switch p.TargetType {
	case "transaction":
		return uc.settleTransaction(ctx, p)
	case "debt":
		return uc.settleDebt(ctx, p)
	default:
		return domain.ErrBadRequest
	}
}

func (uc *paymentUsecase) settleTransaction(ctx context.Context, p *domain.Payment) error {
	tx, err := uc.txRepo.Get(ctx, p.TargetID)
	if err != nil {
		return err
	}
	if tx.Status == domain.TransactionStatusPaid {
		// The money arrived, but the sale is already settled — qris→cash switch,
		// or a second gateway notification for an old attempt. Keep the payment
		// row (proof of the extra charge) and leave the amounts alone, otherwise
		// payment_amount would be counted twice.
		slog.Default().Warn("payment settled on an already-paid transaction",
			slog.String("transaction_id", tx.ID.String()),
			slog.String("order_id", p.OrderID.String()),
			slog.Int64("amount", p.Amount))
		return nil
	}
	tx.PaymentAmount = p.Amount
	tx.ChangeAmount = 0
	tx.Status = domain.TransactionStatusPaid
	if tx.PaidAt == nil {
		now := time.Now().UTC()
		tx.PaidAt = &now
	}
	if err := uc.txRepo.Update(ctx, tx); err != nil {
		return err
	}
	uc.publishTxUpdated(ctx, tx)
	return nil
}

// expireTarget marks the sale behind a failed gateway attempt as expired. Debts
// keep their own active/partial/paid status, and a paid sale is never touched.
func (uc *paymentUsecase) expireTarget(ctx context.Context, p *domain.Payment) error {
	if p.TargetType != "transaction" {
		return nil
	}
	// A late notification for an old attempt must not expire a sale that already
	// has a newer pending attempt.
	if other, err := uc.repo.FindPendingByTarget(ctx, p.TargetType, p.TargetID); err == nil {
		if other.ID != p.ID {
			return nil
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}

	tx, err := uc.txRepo.Get(ctx, p.TargetID)
	if err != nil {
		return err
	}
	if tx.Status != domain.TransactionStatusPending {
		return nil
	}
	tx.Status = domain.TransactionStatusExpired
	if err := uc.txRepo.Update(ctx, tx); err != nil {
		return err
	}
	uc.publishTxUpdated(ctx, tx)
	return nil
}

func (uc *paymentUsecase) publishTxUpdated(ctx context.Context, tx *domain.Transaction) {
	row := *tx
	row.Items = make([]domain.TransactionItem, len(tx.Items))
	copy(row.Items, tx.Items)
	uc.pub.Publish(ctx, kafka.TopicEvents, tx.ID.String(),
		domain.NewEvent(domain.EventTypeTransactionUpdated, "transaction", tx.ID.String(), domain.NewTransactionEvent(&row)))
}

func (uc *paymentUsecase) settleDebt(ctx context.Context, p *domain.Payment) error {
	// Already applied: a retry must not book the same debt payment twice.
	if p.DebtPaymentID != nil {
		return nil
	}
	note := "midtrans qris"
	result, err := uc.debtPay.Create(ctx, &domain.CreateDebtPaymentRequest{
		DebtID:         p.TargetID,
		Amount:         p.Amount,
		Method:         domain.PaymentMethodQRIS,
		CreateCashFlow: true,
		Note:           &note,
	})
	if err != nil {
		return err
	}
	p.DebtPaymentID = &result.Payment.ID
	return nil
}

// validSignature is Midtrans' documented webhook check:
// SHA512(order_id + status_code + gross_amount + server_key).
func (uc *paymentUsecase) validSignature(orderID, statusCode, grossAmount, signature string) bool {
	if uc.serverKey == "" || signature == "" {
		return false
	}
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + uc.serverKey))
	want := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(want), []byte(signature)) == 1
}

// parseGrossAmount reads Midtrans' decimal string ("45000.00") as whole rupiah
// without going through float, which would round silently.
func parseGrossAmount(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSuffix(s, ".00"), 10, 64)
}

func statusToMap(resp *coreapiClient.TransactionStatusResponse) (map[string]any, error) {
	raw, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func translateStatus(transaction string, fraud string) string {
	switch transaction {
	case "settlement", "capture":
		if fraud == "challenge" {
			return domain.PaymentStatusPending // needs manual review
		}
		return domain.PaymentStatusPaid
	case "expire":
		return domain.PaymentStatusExpired
	case "deny", "cancel":
		return domain.PaymentStatusCanceled
	case "pending":
		return domain.PaymentStatusPending
	default:
		// Unknown status: treat as still pending so a stray notification never
		// settles a sale.
		return domain.PaymentStatusPending
	}
}
