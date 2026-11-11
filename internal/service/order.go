// Package service contains business logic that is independent of HTTP. It is
// intentionally small and uses database/sql directly: no ORM is introduced.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/models"
)

var (
	ErrEmptyOrder            = errors.New("order must contain at least one item")
	ErrInvalidQuantity       = errors.New("quantity must be a positive integer")
	ErrProductUnavailable    = errors.New("product is unavailable")
	ErrInsufficientStock     = errors.New("insufficient stock")
	ErrCustomerNotFound      = errors.New("customer not found")
	ErrInvalidCoupon         = errors.New("coupon is invalid or expired")
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderNotCancellable   = errors.New("order cannot be cancelled in its current state")
	ErrIdempotencyConflict   = errors.New("duplicate order request")
	maxOrderLineQuantity     = 10000
	maxOrderLines            = 200
	orderStatusesCancellable = map[string]bool{
		"pending": true, "processing": true, "confirmed": true,
	}
)

// ItemInput is a single requested line. Prices and totals are never accepted
// from the client; they are always resolved from the database.
type ItemInput struct {
	ProductID string
	Quantity  int
}

// PlaceInput describes a checkout request.
type PlaceInput struct {
	CustomerID      string
	FirstName       string
	LastName        string
	BillingAddress  string
	ShippingAddress string
	Country         string
	State           string
	City            string
	PostCode        string
	PaymentMethod   string
	CouponID        string
	IdempotencyKey  string
	Items           []ItemInput
}

// PlacedOrder is the result of a successful checkout.
type PlacedOrder struct {
	ID            string
	Subtotal      float64
	Shipping      float64
	Discount      float64
	Total         float64
	Status        string
	PaymentStatus string
	Reused        bool
}

type normalizedItem struct {
	ProductID string
	Quantity  int
}

// Place creates an order and decrements inventory in one transaction. The
// caller must not pass client-calculated totals; only product IDs and
// quantities are trusted.
func Place(ctx context.Context, db *sql.DB, in PlaceInput) (*PlacedOrder, error) {
	items, err := normalizeItems(in.Items)
	if err != nil {
		return nil, err
	}
	if in.CustomerID != "" {
		var active, deleted int
		err := db.QueryRowContext(ctx, `SELECT active, deleted FROM "Customer" WHERE id = ?`, in.CustomerID).
			Scan(&active, &deleted)
		if err == sql.ErrNoRows {
			return nil, ErrCustomerNotFound
		}
		if err != nil {
			return nil, err
		}
		if active == 0 || deleted != 0 {
			return nil, ErrCustomerNotFound
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := placeTx(ctx, tx, in, items)
	if err != nil {
		_ = tx.Rollback()
		if in.IdempotencyKey != "" && isUniqueError(err) {
			// A concurrent request won the race. Return the existing order.
			if existing, findErr := findByIdempotencyKey(ctx, db, in.IdempotencyKey); findErr == nil {
				return existing, nil
			}
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return res, nil
}

func placeTx(ctx context.Context, tx *sql.Tx, in PlaceInput, items []normalizedItem) (*PlacedOrder, error) {
	if in.IdempotencyKey != "" {
		if existing, err := findByIdempotencyKeyTx(ctx, tx, in.IdempotencyKey); err == nil {
			return existing, nil
		} else if err != sql.ErrNoRows {
			return nil, err
		}
	}

	type lineItem struct {
		item normalizedItem
		line *resolvedLine
	}

	subtotal := 0.0
	shippingTotal := 0.0
	resolved := make([]lineItem, 0, len(items))
	for _, item := range items {
		line, err := resolveLine(ctx, tx, item)
		if err != nil {
			return nil, err
		}
		subtotal += line.unitPrice * float64(item.Quantity)
		shippingTotal += line.shipping
		resolved = append(resolved, lineItem{item: item, line: line})
	}

	discount, couponID, err := resolveCoupon(ctx, tx, in.CouponID, subtotal+shippingTotal)
	if err != nil {
		return nil, err
	}
	total := round2(math.Max(0, subtotal+shippingTotal-discount))

	paymentMethod := strings.TrimSpace(in.PaymentMethod)
	if paymentMethod == "" {
		paymentMethod = "COD"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	orderID := models.NewID()

	// The order row must exist before its line items because OrderItem has a
	// foreign key to Order.
	_, err = tx.ExecContext(ctx, `INSERT INTO "Order" (
		id, customerId, FirstName, LastName, status, totalAmount, billingAddress, shippingAddress,
		country, isInvoiceGenrated, invoiceNumber, state, city, postCode, paymentMethod, couponId,
		paymentStatus, idempotencyKey, createdAt, updatedAt
	) VALUES (?,?,?,?,?,?,?,?,?,0,?,?,?,?,?,?,?,?,?,?)`,
		orderID, nullableString(in.CustomerID), nullableString(in.FirstName), nullableString(in.LastName),
		"pending", total, nullableString(in.BillingAddress), nullableString(in.ShippingAddress),
		nullableString(in.Country), nil,
		nullableString(in.State), nullableString(in.City), nullableString(in.PostCode),
		paymentMethod, nullableString(couponID), "unpaid", nullableString(in.IdempotencyKey), now, now)
	if err != nil {
		return nil, err
	}

	for _, entry := range resolved {
		line := entry.line
		if _, err := tx.ExecContext(ctx, `INSERT INTO "OrderItem"
			(id, orderId, productId, quantity, productName, sku, unitPrice, lineTotal, tax, stockSource)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			models.NewID(), orderID, entry.item.ProductID, entry.item.Quantity,
			nullableString(line.name), nullableString(line.sku),
			line.unitPrice, round2(line.unitPrice*float64(entry.item.Quantity)),
			line.taxRate, line.stockSource); err != nil {
			return nil, err
		}
	}

	// Decrement inventory after the order and line rows exist. Everything is
	// rolled back if any product no longer has enough stock.
	for _, item := range items {
		if err := decrementStock(ctx, tx, item.ProductID, item.Quantity); err != nil {
			return nil, err
		}
	}

	clearCart(ctx, tx, in.CustomerID, items)

	return &PlacedOrder{
		ID:            orderID,
		Subtotal:      round2(subtotal),
		Shipping:      round2(shippingTotal),
		Discount:      round2(discount),
		Total:         total,
		Status:        "pending",
		PaymentStatus: "unpaid",
	}, nil
}

type resolvedLine struct {
	name        string
	sku         string
	unitPrice   float64
	shipping    float64
	taxRate     float64
	stockSource string
}

func resolveLine(ctx context.Context, tx *sql.Tx, item normalizedItem) (*resolvedLine, error) {
	var name, sku, gstID sql.NullString
	var original, discounted, calculated, shipping sql.NullFloat64
	var stockQuantity sql.NullInt64
	var active int
	err := tx.QueryRowContext(ctx, `SELECT name, sku, pricesOriginal, pricesDiscounted, pricesCalculatedPrice,
		shippingCharge, gstId, stockQuantity, isActive FROM "Product" WHERE id = ?`, item.ProductID).
		Scan(&name, &sku, &original, &discounted, &calculated, &shipping, &gstID, &stockQuantity, &active)
	if err == sql.ErrNoRows {
		return nil, ErrProductUnavailable
	}
	if err != nil {
		return nil, err
	}
	if active == 0 {
		return nil, ErrProductUnavailable
	}

	price, ok := firstPositive(calculated, discounted, original)
	if !ok {
		return nil, fmt.Errorf("%w: product %s has no price", ErrProductUnavailable, item.ProductID)
	}

	line := &resolvedLine{
		name:      name.String,
		sku:       sku.String,
		unitPrice: price,
		shipping:  math.Max(0, shipping.Float64),
	}
	if gstID.Valid {
		_ = tx.QueryRowContext(ctx, `SELECT gst FROM "Gst" WHERE id = ?`, gstID.String).Scan(&line.taxRate)
	}

	// Stock table rows are authoritative when they exist. Otherwise the
	// Product.stockQuantity field is used. A product with neither is treated
	// as untracked/backorderable for backwards compatibility.
	var stockRows int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM "Stock" WHERE productId = ?`, item.ProductID).Scan(&stockRows); err != nil {
		return nil, err
	}
	switch {
	case stockRows > 0:
		var available int
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(quantity), 0) FROM "Stock" WHERE productId = ?`, item.ProductID).Scan(&available); err != nil {
			return nil, err
		}
		if available < item.Quantity {
			return nil, fmt.Errorf("%w: product %s", ErrInsufficientStock, item.ProductID)
		}
		line.stockSource = "stock"
	case stockQuantity.Valid:
		if int(stockQuantity.Int64) < item.Quantity {
			return nil, fmt.Errorf("%w: product %s", ErrInsufficientStock, item.ProductID)
		}
		line.stockSource = "product"
	default:
		line.stockSource = "untracked"
	}
	return line, nil
}

func decrementStock(ctx context.Context, tx *sql.Tx, productID string, quantity int) error {
	var stockRows int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM "Stock" WHERE productId = ?`, productID).Scan(&stockRows); err != nil {
		return err
	}
	if stockRows > 0 {
		remaining := quantity
		rows, err := tx.QueryContext(ctx, `SELECT id, quantity FROM "Stock"
			WHERE productId = ? AND quantity > 0 ORDER BY date, id`, productID)
		if err != nil {
			return err
		}
		defer rows.Close()
		type stockRow struct {
			id  string
			qty int
		}
		var all []stockRow
		for rows.Next() {
			var r stockRow
			if err := rows.Scan(&r.id, &r.qty); err != nil {
				return err
			}
			all = append(all, r)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, r := range all {
			if remaining == 0 {
				break
			}
			take := r.qty
			if take > remaining {
				take = remaining
			}
			res, err := tx.ExecContext(ctx, `UPDATE "Stock" SET quantity = quantity - ? WHERE id = ? AND quantity >= ?`,
				take, r.id, take)
			if err != nil {
				return err
			}
			affected, _ := res.RowsAffected()
			if affected != 1 {
				return fmt.Errorf("%w: product %s", ErrInsufficientStock, productID)
			}
			remaining -= take
		}
		if remaining != 0 {
			return fmt.Errorf("%w: product %s", ErrInsufficientStock, productID)
		}
		return nil
	}

	var hasProductStock int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM "Product" WHERE id = ? AND stockQuantity IS NOT NULL`, productID).Scan(&hasProductStock); err != nil {
		return err
	}
	if hasProductStock == 0 {
		return nil // untracked
	}
	res, err := tx.ExecContext(ctx, `UPDATE "Product" SET stockQuantity = stockQuantity - ?
		WHERE id = ? AND stockQuantity >= ?`, quantity, productID, quantity)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		return fmt.Errorf("%w: product %s", ErrInsufficientStock, productID)
	}
	return nil
}

func clearCart(ctx context.Context, tx *sql.Tx, customerID string, items []normalizedItem) {
	if customerID == "" || len(items) == 0 {
		return
	}
	args := make([]any, 0, len(items)+1)
	args = append(args, customerID)
	placeholders := make([]string, 0, len(items))
	for _, item := range items {
		placeholders = append(placeholders, "?")
		args = append(args, item.ProductID)
	}
	_, _ = tx.ExecContext(ctx, `DELETE FROM "CartItem" WHERE customerId = ? AND productId IN (`+
		strings.Join(placeholders, ",")+`)`, args...)
}

func resolveCoupon(ctx context.Context, tx *sql.Tx, couponID string, total float64) (float64, string, error) {
	if strings.TrimSpace(couponID) == "" {
		return 0, "", nil
	}
	var typ string
	var discount sql.NullFloat64
	var start, expiry sql.NullString
	var active int
	err := tx.QueryRowContext(ctx, `SELECT type, discount, start, expiry, active FROM "Coupon" WHERE id = ?`, couponID).
		Scan(&typ, &discount, &start, &expiry, &active)
	if err == sql.ErrNoRows {
		return 0, "", ErrInvalidCoupon
	}
	if err != nil {
		return 0, "", err
	}
	if active == 0 || !discount.Valid || discount.Float64 < 0 {
		return 0, "", ErrInvalidCoupon
	}
	now := time.Now().UTC()
	if t, ok := parseDBTime(start); ok && now.Before(t) {
		return 0, "", ErrInvalidCoupon
	}
	if t, ok := parseDBTime(expiry); ok && now.After(t) {
		return 0, "", ErrInvalidCoupon
	}

	var value float64
	switch {
	case strings.Contains(typ, "%"):
		value = total * discount.Float64 / 100
	default:
		value = discount.Float64
	}
	if value < 0 {
		value = 0
	}
	if value > total {
		value = total
	}
	return value, couponID, nil
}

// Cancel marks an order cancelled and restores exactly the inventory source
// that was decremented at checkout. Repeated cancellation is a no-op.
func Cancel(ctx context.Context, db *sql.DB, orderID string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM "Order" WHERE id = ? AND deletedAt IS NULL`, orderID).Scan(&status)
	if err == sql.ErrNoRows {
		return ErrOrderNotFound
	}
	if err != nil {
		return err
	}
	if status == "cancelled" {
		return tx.Commit()
	}
	if !orderStatusesCancellable[status] {
		return ErrOrderNotCancellable
	}

	rows, err := tx.QueryContext(ctx, `SELECT productId, quantity, stockSource FROM "OrderItem" WHERE orderId = ?`, orderID)
	if err != nil {
		return err
	}
	type item struct {
		productID string
		quantity  int
		source    sql.NullString
	}
	var items []item
	for rows.Next() {
		var pid sql.NullString
		var qty sql.NullInt64
		var source sql.NullString
		if err := rows.Scan(&pid, &qty, &source); err != nil {
			rows.Close()
			return err
		}
		if pid.Valid && qty.Valid {
			items = append(items, item{productID: pid.String, quantity: int(qty.Int64), source: source})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, it := range items {
		if err := restock(ctx, tx, it.productID, it.quantity, it.source); err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `UPDATE "Order" SET status = 'cancelled', updatedAt = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), orderID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func restock(ctx context.Context, tx *sql.Tx, productID string, quantity int, source sql.NullString) error {
	if quantity <= 0 {
		return nil
	}
	switch source.String {
	case "stock":
		return restockStockTable(ctx, tx, productID, quantity)
	case "product":
		_, err := tx.ExecContext(ctx, `UPDATE "Product" SET stockQuantity = COALESCE(stockQuantity, 0) + ? WHERE id = ?`,
			quantity, productID)
		return err
	case "untracked":
		return nil
	default:
		// Legacy order placed before stockSource was recorded. Restore the
		// source that currently exists, preferring the Stock table.
		var stockRows int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM "Stock" WHERE productId = ?`, productID).Scan(&stockRows); err != nil {
			return err
		}
		if stockRows > 0 {
			return restockStockTable(ctx, tx, productID, quantity)
		}
		_, err := tx.ExecContext(ctx, `UPDATE "Product" SET stockQuantity = COALESCE(stockQuantity, 0) + ? WHERE id = ?`,
			quantity, productID)
		return err
	}
}

func restockStockTable(ctx context.Context, tx *sql.Tx, productID string, quantity int) error {
	var stockID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM "Stock" WHERE productId = ? ORDER BY date, id LIMIT 1`, productID).Scan(&stockID)
	if err == sql.ErrNoRows {
		_, err = tx.ExecContext(ctx, `INSERT INTO "Stock" (id, productId, quantity, currentPricePerUnit, date)
			VALUES (?, ?, ?, 0, ?)`, models.NewID(), productID, quantity, time.Now().UTC().Format(time.RFC3339Nano))
		return err
	}
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE "Stock" SET quantity = quantity + ? WHERE id = ?`, quantity, stockID)
	return err
}

// SoftDelete cancels/restocks the order and then hides it from normal reads.
// The row is retained for reporting and audit.
func SoftDelete(ctx context.Context, db *sql.DB, orderID string) error {
	if err := Cancel(ctx, db, orderID); err != nil && err != ErrOrderNotFound {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := db.ExecContext(ctx, `UPDATE "Order" SET deletedAt = ?, updatedAt = ? WHERE id = ?`, now, now, orderID)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrOrderNotFound
	}
	return nil
}

func findByIdempotencyKey(ctx context.Context, db *sql.DB, key string) (*PlacedOrder, error) {
	var id, status, paymentStatus string
	var total sql.NullFloat64
	err := db.QueryRowContext(ctx, `SELECT id, status, paymentStatus, totalAmount FROM "Order" WHERE idempotencyKey = ?`, key).
		Scan(&id, &status, &paymentStatus, &total)
	if err != nil {
		return nil, err
	}
	return &PlacedOrder{ID: id, Total: total.Float64, Status: status, PaymentStatus: paymentStatus, Reused: true}, nil
}

func findByIdempotencyKeyTx(ctx context.Context, tx *sql.Tx, key string) (*PlacedOrder, error) {
	var id, status, paymentStatus string
	var total sql.NullFloat64
	err := tx.QueryRowContext(ctx, `SELECT id, status, paymentStatus, totalAmount FROM "Order" WHERE idempotencyKey = ?`, key).
		Scan(&id, &status, &paymentStatus, &total)
	if err != nil {
		return nil, err
	}
	return &PlacedOrder{ID: id, Total: total.Float64, Status: status, PaymentStatus: paymentStatus, Reused: true}, nil
}

func normalizeItems(items []ItemInput) ([]normalizedItem, error) {
	if len(items) == 0 {
		return nil, ErrEmptyOrder
	}
	if len(items) > maxOrderLines {
		return nil, ErrEmptyOrder
	}
	merged := map[string]int{}
	order := make([]string, 0, len(items))
	for _, item := range items {
		productID := strings.TrimSpace(item.ProductID)
		if productID == "" {
			return nil, ErrProductUnavailable
		}
		if item.Quantity <= 0 || item.Quantity > maxOrderLineQuantity {
			return nil, ErrInvalidQuantity
		}
		if _, seen := merged[productID]; !seen {
			order = append(order, productID)
		}
		merged[productID] += item.Quantity
		if merged[productID] > maxOrderLineQuantity {
			return nil, ErrInvalidQuantity
		}
	}
	out := make([]normalizedItem, 0, len(order))
	for _, id := range order {
		out = append(out, normalizedItem{ProductID: id, Quantity: merged[id]})
	}
	return out, nil
}

func firstPositive(values ...sql.NullFloat64) (float64, bool) {
	for _, v := range values {
		if v.Valid && v.Float64 > 0 {
			return v.Float64, true
		}
	}
	return 0, false
}

func nullableString(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func isUniqueError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || strings.Contains(msg, "constraint failed")
}

func parseDBTime(ns sql.NullString) (time.Time, bool) {
	if !ns.Valid || ns.String == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, ns.String); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
