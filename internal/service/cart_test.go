package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/service"
)

func insertCustomerCartProduct(t *testing.T, db *sql.DB, id string, price float64, stock int) {
	t.Helper()
	insertCatalogProduct(t, db, testProduct{
		ID: id, Name: id, Calculated: floatPtr(price), Stock: intPtr(stock), Active: true,
		CreatedAt: time.Now().UTC(),
	})
}

func TestGuestCartLifecycleRecalculatesFromDatabase(t *testing.T) {
	db := newDB(t)
	insertCatalogProduct(t, db, testProduct{
		ID: "p1", Name: "Silver Ring", Slug: "silver-ring", Calculated: floatPtr(100),
		Original: floatPtr(150), Stock: intPtr(5), Shipping: floatPtr(10), Active: true,
	})
	cart := service.NewCart(db)
	ctx := context.Background()
	owner := service.CartOwner{GuestKey: "guest-session-a"}

	if err := cart.Add(ctx, owner, "p1", 2); err != nil {
		t.Fatalf("add: %v", err)
	}
	view, err := cart.Get(ctx, owner)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if view.ItemCount != 2 || view.LineCount != 1 || view.Empty() {
		t.Fatalf("unexpected counts: %+v", view)
	}
	if view.Subtotal != 200 || view.Shipping != 10 || view.Total != 210 {
		t.Fatalf("unexpected totals: subtotal=%v shipping=%v total=%v", view.Subtotal, view.Shipping, view.Total)
	}
	if view.Items[0].UnitPrice != 100 || view.Items[0].LineTotal != 200 {
		t.Fatalf("unexpected line: %+v", view.Items[0])
	}
	if !view.Items[0].HasDiscount() || view.Items[0].OriginalPrice != 150 {
		t.Fatalf("discount not resolved: %+v", view.Items[0])
	}
	if count, err := cart.Count(ctx, owner); err != nil || count != 2 {
		t.Fatalf("count = %d, %v", count, err)
	}

	// The catalogue price changes after the item was added. The cart must show
	// the new server price, not a snapshot from the client or old request.
	if _, err := db.Exec(`UPDATE "Product" SET pricesCalculatedPrice = 125 WHERE id = 'p1'`); err != nil {
		t.Fatal(err)
	}
	view, err = cart.Get(ctx, owner)
	if err != nil {
		t.Fatalf("get after price change: %v", err)
	}
	if view.Items[0].UnitPrice != 125 || view.Subtotal != 250 || view.Total != 260 {
		t.Fatalf("price was not recalculated: %+v", view)
	}

	if err := cart.SetQuantity(ctx, owner, "p1", 3); err != nil {
		t.Fatalf("set quantity: %v", err)
	}
	if count, _ := cart.Count(ctx, owner); count != 3 {
		t.Fatalf("count after set = %d", count)
	}

	if err := cart.Remove(ctx, owner, "p1"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	view, err = cart.Get(ctx, owner)
	if err != nil {
		t.Fatalf("get after remove: %v", err)
	}
	if !view.Empty() || view.Total != 0 {
		t.Fatalf("cart should be empty: %+v", view)
	}
	// Removal is idempotent.
	if err := cart.Remove(ctx, owner, "p1"); err != nil {
		t.Fatalf("second remove: %v", err)
	}
}

func TestCartValidatesQuantityStockAndAvailability(t *testing.T) {
	db := newDB(t)
	insertCatalogProduct(t, db, testProduct{
		ID: "limited", Name: "Limited", Calculated: floatPtr(50), Stock: intPtr(2), Active: true,
	})
	insertCatalogProduct(t, db, testProduct{
		ID: "unpriced", Name: "Unpriced", Active: true,
	})
	cart := service.NewCart(db)
	ctx := context.Background()
	owner := service.CartOwner{GuestKey: "guest-stock"}

	for _, qty := range []int{0, -1, service.MaxCartQuantity + 1} {
		if err := cart.Add(ctx, owner, "limited", qty); !errors.Is(err, service.ErrInvalidQuantity) {
			t.Fatalf("qty %d: expected invalid quantity, got %v", qty, err)
		}
	}
	if err := cart.Add(ctx, owner, "missing", 1); !errors.Is(err, service.ErrCartProductNotFound) {
		t.Fatalf("expected product not found, got %v", err)
	}
	if err := cart.Add(ctx, owner, "unpriced", 1); !errors.Is(err, service.ErrProductUnavailable) {
		t.Fatalf("expected unavailable, got %v", err)
	}
	if err := cart.Add(ctx, owner, "limited", 2); err != nil {
		t.Fatalf("add stock limit: %v", err)
	}
	if err := cart.Add(ctx, owner, "limited", 1); !errors.Is(err, service.ErrInsufficientStock) {
		t.Fatalf("expected insufficient stock, got %v", err)
	}
	if count, _ := cart.Count(ctx, owner); count != 2 {
		t.Fatalf("failed add must not change quantity, got %d", count)
	}
	if err := cart.SetQuantity(ctx, owner, "limited", 3); !errors.Is(err, service.ErrInsufficientStock) {
		t.Fatalf("expected insufficient stock on update, got %v", err)
	}
	if err := cart.SetQuantity(ctx, owner, "limited", 0); !errors.Is(err, service.ErrInvalidQuantity) {
		t.Fatalf("expected invalid quantity on update, got %v", err)
	}
	if err := cart.SetQuantity(ctx, owner, "missing", 1); !errors.Is(err, service.ErrCartProductNotFound) {
		t.Fatalf("expected missing product on update, got %v", err)
	}

	// Deactivating a product leaves the line visible but excluded from totals.
	if _, err := db.Exec(`UPDATE "Product" SET isActive = 0 WHERE id = 'limited'`); err != nil {
		t.Fatal(err)
	}
	view, err := cart.Get(ctx, owner)
	if err != nil {
		t.Fatalf("get unavailable: %v", err)
	}
	if len(view.Items) != 1 || view.Items[0].Available || !view.HasUnavailable() {
		t.Fatalf("expected unavailable line: %+v", view.Items)
	}
	if view.Subtotal != 0 || view.Total != 0 || view.Unavailable != 1 {
		t.Fatalf("unavailable line must be excluded from totals: %+v", view)
	}
	if err := cart.Add(ctx, owner, "limited", 1); !errors.Is(err, service.ErrProductUnavailable) {
		t.Fatalf("expected unavailable add, got %v", err)
	}
}

func TestCartUsesStockTableWhenPresent(t *testing.T) {
	db := newDB(t)
	insertCatalogProduct(t, db, testProduct{
		ID: "p1", Name: "Ring", Calculated: floatPtr(100), Stock: intPtr(999), Active: true,
	})
	insertStockRow(t, db, "p1", 3)
	cart := service.NewCart(db)
	owner := service.CartOwner{GuestKey: "guest-stock-table"}

	if err := cart.Add(context.Background(), owner, "p1", 3); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := cart.Add(context.Background(), owner, "p1", 1); !errors.Is(err, service.ErrInsufficientStock) {
		t.Fatalf("expected stock table limit, got %v", err)
	}
	view, err := cart.Get(context.Background(), owner)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !view.Items[0].StockTracked || view.Items[0].Stock != 3 {
		t.Fatalf("stock table not used: %+v", view.Items[0])
	}
}

func TestCartOwnersAreIsolated(t *testing.T) {
	db := newDB(t)
	insertCustomerCartProduct(t, db, "p1", 100, 10)
	insertCustomerCartProduct(t, db, "p2", 200, 10)
	insertCustomer(t, db, "c1", "one@example.com")
	cart := service.NewCart(db)
	ctx := context.Background()

	guestA := service.CartOwner{GuestKey: "guest-a"}
	guestB := service.CartOwner{GuestKey: "guest-b"}
	customer := service.CartOwner{CustomerID: "c1"}

	if err := cart.Add(ctx, guestA, "p1", 1); err != nil {
		t.Fatalf("guest A add: %v", err)
	}
	if err := cart.Add(ctx, guestB, "p2", 2); err != nil {
		t.Fatalf("guest B add: %v", err)
	}
	if err := cart.Add(ctx, customer, "p1", 3); err != nil {
		t.Fatalf("customer add: %v", err)
	}

	a, _ := cart.Get(ctx, guestA)
	b, _ := cart.Get(ctx, guestB)
	c, _ := cart.Get(ctx, customer)
	if a.ItemCount != 1 || b.ItemCount != 2 || c.ItemCount != 3 {
		t.Fatalf("owners not isolated: a=%d b=%d c=%d", a.ItemCount, b.ItemCount, c.ItemCount)
	}
	if a.Items[0].ProductID != "p1" || b.Items[0].ProductID != "p2" || c.Items[0].ProductID != "p1" {
		t.Fatalf("unexpected owner rows: %+v %+v %+v", a.Items, b.Items, c.Items)
	}

	// Guest A cannot update or delete guest B's line.
	if err := cart.SetQuantity(ctx, guestA, "p2", 9); !errors.Is(err, service.ErrCartItemNotFound) {
		t.Fatalf("cross-cart update should be not found, got %v", err)
	}
	if err := cart.Remove(ctx, guestA, "p2"); err != nil {
		t.Fatalf("cross-cart remove should be harmless: %v", err)
	}
	b, _ = cart.Get(ctx, guestB)
	if b.ItemCount != 2 {
		t.Fatalf("cross-cart remove changed another cart: %+v", b)
	}

	// The customer uses the existing CartItem table, not GuestCartItem.
	var customerRows, guestRows int
	if err := db.QueryRow(`SELECT COUNT(1) FROM "CartItem" WHERE customerId = 'c1'`).Scan(&customerRows); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(1) FROM "GuestCartItem" WHERE sessionId IN ('guest-a','guest-b')`).Scan(&guestRows); err != nil {
		t.Fatal(err)
	}
	if customerRows != 1 || guestRows != 2 {
		t.Fatalf("unexpected table split: customer=%d guest=%d", customerRows, guestRows)
	}
}

func TestCartRejectsInvalidOwnerAndInactiveCustomer(t *testing.T) {
	db := newDB(t)
	insertCustomerCartProduct(t, db, "p1", 100, 10)
	insertCustomer(t, db, "c1", "one@example.com")
	cart := service.NewCart(db)
	ctx := context.Background()

	for _, owner := range []service.CartOwner{
		{},
		{CustomerID: "c1", GuestKey: "guest-a"},
	} {
		if _, err := cart.Get(ctx, owner); !errors.Is(err, service.ErrCartOwnerInvalid) {
			t.Fatalf("expected invalid owner, got %v", err)
		}
		if err := cart.Add(ctx, owner, "p1", 1); !errors.Is(err, service.ErrCartOwnerInvalid) {
			t.Fatalf("expected invalid owner on add, got %v", err)
		}
	}

	if _, err := db.Exec(`UPDATE "Customer" SET active = 0 WHERE id = 'c1'`); err != nil {
		t.Fatal(err)
	}
	if err := cart.Add(ctx, service.CartOwner{CustomerID: "c1"}, "p1", 1); !errors.Is(err, service.ErrCustomerNotFound) {
		t.Fatalf("expected inactive customer rejection, got %v", err)
	}
	if err := cart.Add(ctx, service.CartOwner{CustomerID: "missing"}, "p1", 1); !errors.Is(err, service.ErrCustomerNotFound) {
		t.Fatalf("expected missing customer rejection, got %v", err)
	}
}

func TestCartQuantityLimit(t *testing.T) {
	db := newDB(t)
	insertCatalogProduct(t, db, testProduct{
		ID: "p1", Name: "Bulk", Calculated: floatPtr(1), Stock: intPtr(10000), Active: true,
	})
	cart := service.NewCart(db)
	owner := service.CartOwner{GuestKey: "guest-limit"}
	if err := cart.Add(context.Background(), owner, "p1", service.MaxCartQuantity); err != nil {
		t.Fatalf("add max: %v", err)
	}
	if err := cart.Add(context.Background(), owner, "p1", 1); !errors.Is(err, service.ErrCartQuantityLimit) {
		t.Fatalf("expected quantity limit, got %v", err)
	}
}

func TestCartCountIgnoresOtherOwners(t *testing.T) {
	db := newDB(t)
	insertCustomerCartProduct(t, db, "p1", 100, 10)
	cart := service.NewCart(db)
	ctx := context.Background()
	if err := cart.Add(ctx, service.CartOwner{GuestKey: "a"}, "p1", 4); err != nil {
		t.Fatal(err)
	}
	if err := cart.Add(ctx, service.CartOwner{GuestKey: "b"}, "p1", 1); err != nil {
		t.Fatal(err)
	}
	if count, _ := cart.Count(ctx, service.CartOwner{GuestKey: "a"}); count != 4 {
		t.Fatalf("count a = %d", count)
	}
	if count, _ := cart.Count(ctx, service.CartOwner{GuestKey: "b"}); count != 1 {
		t.Fatalf("count b = %d", count)
	}
	if count, _ := cart.Count(ctx, service.CartOwner{GuestKey: "missing"}); count != 0 {
		t.Fatalf("count missing = %d", count)
	}
}

func TestGuestCartItemsUseServerHashedKey(t *testing.T) {
	db := newDB(t)
	insertCustomerCartProduct(t, db, "p1", 100, 10)
	cart := service.NewCart(db)
	raw := "opaque-cookie-value"
	key := "hashed-" + raw
	if err := cart.Add(context.Background(), service.CartOwner{GuestKey: key}, "p1", 1); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := db.QueryRow(`SELECT sessionId FROM "GuestCartItem" LIMIT 1`).Scan(&stored); err != nil {
		t.Fatalf("read guest cart: %v", err)
	}
	if stored != key {
		t.Fatalf("stored key = %q, want %q", stored, key)
	}
}
