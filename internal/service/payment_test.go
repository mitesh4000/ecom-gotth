package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mitesh4000/pushti_sangar/server/internal/service"
)

const paymentSecret = "test-payment-secret-123456"

func TestVerifyPaymentMarksOrderPaid(t *testing.T) {
	db := newDB(t)
	insertCustomer(t, db, "cust-1", "cust@example.com")
	insertProduct(t, db, "p1", 100, intPtr(5))
	placed, err := service.Place(context.Background(), db, service.PlaceInput{
		CustomerID: "cust-1",
		Items:      []service.ItemInput{{ProductID: "p1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}

	ref := "provider-ref-1"
	result, err := service.VerifyPayment(context.Background(), db, service.PaymentInput{
		OrderID:           placed.ID,
		CustomerID:        "cust-1",
		Provider:          "testpay",
		Secret:            paymentSecret,
		ProviderReference: ref,
		Signature:         service.SignPayment(paymentSecret, placed.ID, ref, placed.Total),
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Status != "succeeded" || result.Amount != 200 {
		t.Fatalf("unexpected result: %+v", result)
	}

	var paymentStatus string
	if err := db.QueryRow(`SELECT paymentStatus FROM "Order" WHERE id = ?`, placed.ID).Scan(&paymentStatus); err != nil {
		t.Fatal(err)
	}
	if paymentStatus != "paid" {
		t.Fatalf("expected paid, got %s", paymentStatus)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM "Payment" WHERE orderId = ?`, placed.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" {
		t.Fatalf("expected succeeded payment row, got %s", status)
	}
}

func TestVerifyPaymentRejectsInvalidSignature(t *testing.T) {
	db := newDB(t)
	insertCustomer(t, db, "cust-1", "cust@example.com")
	insertProduct(t, db, "p1", 100, intPtr(5))
	placed, _ := service.Place(context.Background(), db, service.PlaceInput{
		CustomerID: "cust-1",
		Items:      []service.ItemInput{{ProductID: "p1", Quantity: 1}},
	})

	_, err := service.VerifyPayment(context.Background(), db, service.PaymentInput{
		OrderID:           placed.ID,
		CustomerID:        "cust-1",
		Provider:          "testpay",
		Secret:            paymentSecret,
		ProviderReference: "ref",
		Signature:         "deadbeef",
	})
	if !errors.Is(err, service.ErrPaymentInvalid) {
		t.Fatalf("expected invalid signature, got %v", err)
	}
	var paymentStatus string
	if err := db.QueryRow(`SELECT paymentStatus FROM "Order" WHERE id = ?`, placed.ID).Scan(&paymentStatus); err != nil {
		t.Fatal(err)
	}
	if paymentStatus != "unpaid" {
		t.Fatalf("order must remain unpaid, got %s", paymentStatus)
	}
}

func TestVerifyPaymentFailsClosedWhenNotConfigured(t *testing.T) {
	db := newDB(t)
	_, err := service.VerifyPayment(context.Background(), db, service.PaymentInput{
		OrderID:           "order-1",
		ProviderReference: "ref",
		Signature:         "sig",
	})
	if !errors.Is(err, service.ErrPaymentNotConfigured) {
		t.Fatalf("expected not configured, got %v", err)
	}
}

func TestVerifyPaymentRejectsAnotherCustomersOrder(t *testing.T) {
	db := newDB(t)
	insertCustomer(t, db, "cust-1", "cust1@example.com")
	insertCustomer(t, db, "cust-2", "cust2@example.com")
	insertProduct(t, db, "p1", 100, intPtr(5))
	placed, _ := service.Place(context.Background(), db, service.PlaceInput{
		CustomerID: "cust-1",
		Items:      []service.ItemInput{{ProductID: "p1", Quantity: 1}},
	})

	ref := "ref"
	_, err := service.VerifyPayment(context.Background(), db, service.PaymentInput{
		OrderID:           placed.ID,
		CustomerID:        "cust-2",
		Provider:          "testpay",
		Secret:            paymentSecret,
		ProviderReference: ref,
		Signature:         service.SignPayment(paymentSecret, placed.ID, ref, placed.Total),
	})
	if !errors.Is(err, service.ErrPaymentOrderNotFound) {
		t.Fatalf("expected order not found, got %v", err)
	}
}

func TestVerifyPaymentIsIdempotent(t *testing.T) {
	db := newDB(t)
	insertCustomer(t, db, "cust-1", "cust@example.com")
	insertProduct(t, db, "p1", 100, intPtr(5))
	placed, _ := service.Place(context.Background(), db, service.PlaceInput{
		CustomerID: "cust-1",
		Items:      []service.ItemInput{{ProductID: "p1", Quantity: 1}},
	})

	ref := "ref"
	in := service.PaymentInput{
		OrderID:           placed.ID,
		CustomerID:        "cust-1",
		Provider:          "testpay",
		Secret:            paymentSecret,
		ProviderReference: ref,
		Signature:         service.SignPayment(paymentSecret, placed.ID, ref, placed.Total),
		IdempotencyKey:    "payment-key-1",
	}
	if _, err := service.VerifyPayment(context.Background(), db, in); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	result, err := service.VerifyPayment(context.Background(), db, in)
	if err != nil {
		t.Fatalf("second verify: %v", err)
	}
	if result.Status != "succeeded" {
		t.Fatalf("unexpected idempotent result: %+v", result)
	}
	if n := countRows(t, db, "Payment"); n != 1 {
		t.Fatalf("expected one payment row, got %d", n)
	}
}

func TestVerifyPaymentRejectsAlreadyPaidOrder(t *testing.T) {
	db := newDB(t)
	insertCustomer(t, db, "cust-1", "cust@example.com")
	insertProduct(t, db, "p1", 100, intPtr(5))
	placed, _ := service.Place(context.Background(), db, service.PlaceInput{
		CustomerID: "cust-1",
		Items:      []service.ItemInput{{ProductID: "p1", Quantity: 1}},
	})

	ref := "ref"
	sig := service.SignPayment(paymentSecret, placed.ID, ref, placed.Total)
	in := service.PaymentInput{
		OrderID:           placed.ID,
		CustomerID:        "cust-1",
		Provider:          "testpay",
		Secret:            paymentSecret,
		ProviderReference: ref,
		Signature:         sig,
	}
	if _, err := service.VerifyPayment(context.Background(), db, in); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	in.IdempotencyKey = ""
	if _, err := service.VerifyPayment(context.Background(), db, in); !errors.Is(err, service.ErrPaymentOrderPaid) {
		t.Fatalf("expected already paid, got %v", err)
	}
}
