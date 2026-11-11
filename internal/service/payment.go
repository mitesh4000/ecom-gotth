package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/models"
)

var (
	ErrPaymentNotConfigured = errors.New("payment provider is not configured")
	ErrPaymentInvalid       = errors.New("payment signature is invalid")
	ErrPaymentOrderNotFound = errors.New("order not found")
	ErrPaymentOrderPaid     = errors.New("order is already paid")
)

// PaymentInput is a provider callback. Amount is deliberately absent: it is
// always read from the order row.
type PaymentInput struct {
	OrderID           string
	CustomerID        string
	Provider          string
	Secret            string
	ProviderReference string
	Signature         string
	IdempotencyKey    string
}

type PaymentResult struct {
	OrderID           string
	Status            string
	Amount            float64
	ProviderReference string
}

// VerifyPayment checks an HMAC-SHA256 callback signature over
// orderID|providerReference|amount and, only if it is valid, marks the order
// paid. The provider adapter is responsible for producing the same signature.
// When no provider/secret is configured the call fails closed.
func VerifyPayment(ctx context.Context, db *sql.DB, in PaymentInput) (*PaymentResult, error) {
	if strings.TrimSpace(in.Provider) == "" || strings.TrimSpace(in.Secret) == "" {
		return nil, ErrPaymentNotConfigured
	}
	if strings.TrimSpace(in.OrderID) == "" || strings.TrimSpace(in.ProviderReference) == "" || strings.TrimSpace(in.Signature) == "" {
		return nil, ErrPaymentInvalid
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if in.IdempotencyKey != "" {
		var status, ref string
		var amount float64
		err := tx.QueryRowContext(ctx, `SELECT status, providerReference, amount FROM "Payment" WHERE idempotencyKey = ? AND orderId = ?`, in.IdempotencyKey, in.OrderID).
			Scan(&status, &ref, &amount)
		if err == nil {
			return &PaymentResult{OrderID: in.OrderID, Status: status, Amount: amount, ProviderReference: ref}, tx.Commit()
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	var orderCustomer sql.NullString
	var paymentStatus string
	var total sql.NullFloat64
	err = tx.QueryRowContext(ctx, `SELECT customerId, paymentStatus, totalAmount FROM "Order" WHERE id = ? AND deletedAt IS NULL`, in.OrderID).
		Scan(&orderCustomer, &paymentStatus, &total)
	if err == sql.ErrNoRows {
		return nil, ErrPaymentOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	if in.CustomerID != "" && (!orderCustomer.Valid || orderCustomer.String != in.CustomerID) {
		// Do not reveal whether another customer's order exists.
		return nil, ErrPaymentOrderNotFound
	}
	if paymentStatus == "paid" {
		return &PaymentResult{OrderID: in.OrderID, Status: "succeeded", Amount: total.Float64}, ErrPaymentOrderPaid
	}
	amount := total.Float64

	expected := signPayment(in.Secret, in.OrderID, in.ProviderReference, amount)
	if !hmac.Equal([]byte(strings.ToLower(expected)), []byte(strings.ToLower(strings.TrimSpace(in.Signature)))) {
		_, _ = tx.ExecContext(ctx, `INSERT INTO "Payment"
			(id, orderId, customerId, amount, provider, providerReference, status, failureReason, idempotencyKey, createdAt, updatedAt)
			VALUES (?, ?, ?, ?, ?, ?, 'failed', ?, ?, ?, ?)`,
			models.NewID(), in.OrderID, nullableString(in.CustomerID), amount, in.Provider,
			in.ProviderReference, "invalid signature", nullableString(in.IdempotencyKey),
			time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano))
		_ = tx.Commit()
		return nil, ErrPaymentInvalid
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `INSERT INTO "Payment"
		(id, orderId, customerId, amount, provider, providerReference, status, idempotencyKey, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, ?, ?, 'succeeded', ?, ?, ?)`,
		models.NewID(), in.OrderID, nullableString(in.CustomerID), amount, in.Provider,
		in.ProviderReference, nullableString(in.IdempotencyKey), now, now)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE "Order" SET paymentStatus = 'paid', updatedAt = ? WHERE id = ?`, now, in.OrderID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &PaymentResult{OrderID: in.OrderID, Status: "succeeded", Amount: amount, ProviderReference: in.ProviderReference}, nil
}

// SignPayment exposes the same signature format for provider adapters and
// tests.
func SignPayment(secret, orderID, reference string, amount float64) string {
	return signPayment(secret, orderID, reference, amount)
}

func signPayment(secret, orderID, reference string, amount float64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(orderID + "|" + reference + "|" + strconv.FormatFloat(amount, 'f', 2, 64)))
	return hex.EncodeToString(mac.Sum(nil))
}
