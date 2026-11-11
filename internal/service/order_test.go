package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mitesh4000/pushti_sangar/server/internal/service"
)

func intPtr(v int) *int { return &v }

func TestPlaceComputesServerSideTotalAndDecrementsStock(t *testing.T) {
	db := newDB(t)
	insertProduct(t, db, "p1", 100, intPtr(5))

	placed, err := service.Place(context.Background(), db, service.PlaceInput{
		Items: []service.ItemInput{{ProductID: "p1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if placed.Total != 200 {
		t.Fatalf("expected total 200, got %v", placed.Total)
	}
	if placed.Status != "pending" || placed.PaymentStatus != "unpaid" {
		t.Fatalf("unexpected status: %+v", placed)
	}
	if got := productStock(t, db, "p1"); got != 3 {
		t.Fatalf("expected stock 3, got %d", got)
	}

	var unitPrice, lineTotal float64
	if err := db.QueryRow(`SELECT unitPrice, lineTotal FROM "OrderItem" WHERE orderId = ?`, placed.ID).
		Scan(&unitPrice, &lineTotal); err != nil {
		t.Fatalf("read line: %v", err)
	}
	if unitPrice != 100 || lineTotal != 200 {
		t.Fatalf("expected snapshot 100/200, got %v/%v", unitPrice, lineTotal)
	}
}

func TestPlaceIgnoresUnavailableProduct(t *testing.T) {
	db := newDB(t)
	insertProduct(t, db, "p1", 100, intPtr(5))
	if _, err := db.Exec(`UPDATE "Product" SET isActive = 0 WHERE id = 'p1'`); err != nil {
		t.Fatal(err)
	}
	_, err := service.Place(context.Background(), db, service.PlaceInput{
		Items: []service.ItemInput{{ProductID: "p1", Quantity: 1}},
	})
	if !errors.Is(err, service.ErrProductUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}
}

func TestPlaceRejectsInvalidQuantity(t *testing.T) {
	db := newDB(t)
	insertProduct(t, db, "p1", 100, intPtr(5))
	for _, qty := range []int{0, -1} {
		_, err := service.Place(context.Background(), db, service.PlaceInput{
			Items: []service.ItemInput{{ProductID: "p1", Quantity: qty}},
		})
		if !errors.Is(err, service.ErrInvalidQuantity) {
			t.Fatalf("qty %d: expected invalid quantity, got %v", qty, err)
		}
	}
}

func TestPlaceRejectsInsufficientStockWithoutWriting(t *testing.T) {
	db := newDB(t)
	insertProduct(t, db, "p1", 100, intPtr(1))

	_, err := service.Place(context.Background(), db, service.PlaceInput{
		Items: []service.ItemInput{{ProductID: "p1", Quantity: 2}},
	})
	if !errors.Is(err, service.ErrInsufficientStock) {
		t.Fatalf("expected insufficient stock, got %v", err)
	}
	if n := countRows(t, db, "Order"); n != 0 {
		t.Fatalf("expected no orders, got %d", n)
	}
	if got := productStock(t, db, "p1"); got != 1 {
		t.Fatalf("stock must be unchanged, got %d", got)
	}
}

func TestPlaceIsAtomicAcrossMultipleItems(t *testing.T) {
	db := newDB(t)
	insertProduct(t, db, "p1", 100, intPtr(5))
	insertProduct(t, db, "p2", 50, intPtr(1))

	_, err := service.Place(context.Background(), db, service.PlaceInput{
		Items: []service.ItemInput{
			{ProductID: "p1", Quantity: 2},
			{ProductID: "p2", Quantity: 2},
		},
	})
	if !errors.Is(err, service.ErrInsufficientStock) {
		t.Fatalf("expected insufficient stock, got %v", err)
	}
	if n := countRows(t, db, "Order"); n != 0 {
		t.Fatalf("expected no orders, got %d", n)
	}
	if n := countRows(t, db, "OrderItem"); n != 0 {
		t.Fatalf("expected no order items, got %d", n)
	}
	if got := productStock(t, db, "p1"); got != 5 {
		t.Fatalf("first product stock must roll back, got %d", got)
	}
}

func TestPlaceUsesStockTableWhenPresent(t *testing.T) {
	db := newDB(t)
	insertProduct(t, db, "p1", 100, intPtr(999)) // ignored when Stock rows exist
	insertStockRow(t, db, "p1", 3)

	_, err := service.Place(context.Background(), db, service.PlaceInput{
		Items: []service.ItemInput{{ProductID: "p1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if got := stockTableTotal(t, db, "p1"); got != 1 {
		t.Fatalf("expected stock table total 1, got %d", got)
	}
	if got := productStock(t, db, "p1"); got != 999 {
		t.Fatalf("product stock must be untouched, got %d", got)
	}
}

func TestPlaceConcurrentOrdersDoNotOversell(t *testing.T) {
	db := newDB(t)
	insertProduct(t, db, "p1", 100, intPtr(10))

	const attempts = 25
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	failures := 0
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.Place(context.Background(), db, service.PlaceInput{
				Items: []service.ItemInput{{ProductID: "p1", Quantity: 1}},
			})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successes++
			} else if errors.Is(err, service.ErrInsufficientStock) {
				failures++
			} else {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if successes != 10 {
		t.Fatalf("expected exactly 10 successful orders, got %d", successes)
	}
	if failures != attempts-10 {
		t.Fatalf("expected %d stock failures, got %d", attempts-10, failures)
	}
	if got := productStock(t, db, "p1"); got != 0 {
		t.Fatalf("expected stock 0, got %d", got)
	}
	if n := countRows(t, db, "Order"); n != 10 {
		t.Fatalf("expected 10 orders, got %d", n)
	}
}

func TestPlaceIsIdempotent(t *testing.T) {
	db := newDB(t)
	insertProduct(t, db, "p1", 100, intPtr(5))

	in := service.PlaceInput{
		IdempotencyKey: "checkout-key-1",
		Items:          []service.ItemInput{{ProductID: "p1", Quantity: 2}},
	}
	first, err := service.Place(context.Background(), db, in)
	if err != nil {
		t.Fatalf("first place: %v", err)
	}
	second, err := service.Place(context.Background(), db, in)
	if err != nil {
		t.Fatalf("second place: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same order id, got %s and %s", first.ID, second.ID)
	}
	if !second.Reused {
		t.Fatal("expected second response to be marked reused")
	}
	if got := productStock(t, db, "p1"); got != 3 {
		t.Fatalf("stock must only be decremented once, got %d", got)
	}
	if n := countRows(t, db, "Order"); n != 1 {
		t.Fatalf("expected one order, got %d", n)
	}
}

func TestCancelRestocksAndIsIdempotent(t *testing.T) {
	db := newDB(t)
	insertProduct(t, db, "p1", 100, intPtr(5))

	placed, err := service.Place(context.Background(), db, service.PlaceInput{
		Items: []service.ItemInput{{ProductID: "p1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if err := service.Cancel(context.Background(), db, placed.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got := productStock(t, db, "p1"); got != 5 {
		t.Fatalf("expected restocked 5, got %d", got)
	}
	if err := service.Cancel(context.Background(), db, placed.ID); err != nil {
		t.Fatalf("second cancel should be a no-op: %v", err)
	}
	if got := productStock(t, db, "p1"); got != 5 {
		t.Fatalf("second cancel must not double-restock, got %d", got)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM "Order" WHERE id = ?`, placed.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "cancelled" {
		t.Fatalf("expected cancelled, got %s", status)
	}
}

func TestCancelRestocksStockTableSource(t *testing.T) {
	db := newDB(t)
	insertProduct(t, db, "p1", 100, intPtr(999))
	insertStockRow(t, db, "p1", 4)

	placed, err := service.Place(context.Background(), db, service.PlaceInput{
		Items: []service.ItemInput{{ProductID: "p1", Quantity: 3}},
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if got := stockTableTotal(t, db, "p1"); got != 1 {
		t.Fatalf("expected 1 after checkout, got %d", got)
	}
	if err := service.Cancel(context.Background(), db, placed.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got := stockTableTotal(t, db, "p1"); got != 4 {
		t.Fatalf("expected 4 after cancel, got %d", got)
	}
}

func TestCancelUnknownOrder(t *testing.T) {
	db := newDB(t)
	if err := service.Cancel(context.Background(), db, "missing"); !errors.Is(err, service.ErrOrderNotFound) {
		t.Fatalf("expected order not found, got %v", err)
	}
}

func TestSoftDeleteHidesOrderAndRestocks(t *testing.T) {
	db := newDB(t)
	insertProduct(t, db, "p1", 100, intPtr(5))
	placed, err := service.Place(context.Background(), db, service.PlaceInput{
		Items: []service.ItemInput{{ProductID: "p1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if err := service.SoftDelete(context.Background(), db, placed.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	var deletedAt interface{}
	if err := db.QueryRow(`SELECT deletedAt FROM "Order" WHERE id = ?`, placed.ID).Scan(&deletedAt); err != nil {
		t.Fatal(err)
	}
	if deletedAt == nil {
		t.Fatal("expected deletedAt to be set")
	}
	if got := productStock(t, db, "p1"); got != 5 {
		t.Fatalf("expected restocked 5, got %d", got)
	}
}

func TestPlaceAppliesPercentageCouponServerSide(t *testing.T) {
	db := newDB(t)
	insertProduct(t, db, "p1", 100, intPtr(5))
	if _, err := db.Exec(`INSERT INTO "Coupon" (id, name, type, discount, start, expiry, active, createdAt, updatedAt)
		VALUES ('c1', 'ten', '%', 10, NULL, NULL, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert coupon: %v", err)
	}
	placed, err := service.Place(context.Background(), db, service.PlaceInput{
		CouponID: "c1",
		Items:    []service.ItemInput{{ProductID: "p1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if placed.Total != 180 {
		t.Fatalf("expected 180 after 10%% discount, got %v", placed.Total)
	}
}
