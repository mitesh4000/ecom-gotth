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

const (
	// MaxCartQuantity bounds a single line in both the JSON API and storefront.
	MaxCartQuantity = 1000
)

var (
	// ErrCartOwnerInvalid means neither a customer nor a guest session was
	// supplied, or both were. Ownership is always resolved on the server.
	ErrCartOwnerInvalid = errors.New("cart owner is invalid")
	// ErrCartProductNotFound is returned when a requested product does not
	// exist at all.
	ErrCartProductNotFound = errors.New("cart product not found")
	// ErrCartItemNotFound is returned when an update targets a line that is not
	// in the current owner's cart. Another user's line is indistinguishable
	// from a missing line, which prevents cross-cart probing.
	ErrCartItemNotFound = errors.New("cart item not found")
	// ErrCartQuantityLimit is returned when an add would exceed MaxCartQuantity.
	ErrCartQuantityLimit = errors.New("cart quantity limit exceeded")
)

// CartOwner identifies the cart being read or changed. Exactly one field must
// be set. GuestKey is the SHA-256 hash of an opaque server-issued session
// cookie, never a browser-supplied id.
type CartOwner struct {
	CustomerID string
	GuestKey   string
}

// Valid reports whether the owner is a single, server-resolved identity.
func (o CartOwner) Valid() bool {
	hasCustomer := strings.TrimSpace(o.CustomerID) != ""
	hasGuest := strings.TrimSpace(o.GuestKey) != ""
	return hasCustomer != hasGuest
}

// IsGuest reports whether the cart belongs to a guest session.
func (o CartOwner) IsGuest() bool {
	return strings.TrimSpace(o.CustomerID) == "" && strings.TrimSpace(o.GuestKey) != ""
}

// CartLine is one resolved cart row. Prices, stock and totals come from the
// database; the browser only ever supplies a product id and quantity.
type CartLine struct {
	ProductID     string
	Slug          string
	Name          string
	Image         string
	CategoryName  string
	Quantity      int
	UnitPrice     float64
	OriginalPrice float64
	LineTotal     float64
	Shipping      float64
	TaxRate       float64
	TaxAmount     float64
	Stock         int
	StockTracked  bool
	Available     bool
	StockWarning  bool
}

// URL returns the canonical product detail path.
func (l CartLine) URL() string {
	if strings.TrimSpace(l.Slug) != "" {
		return "/products/" + l.Slug
	}
	return "/products/" + l.ProductID
}

// HasDiscount reports whether the catalogue price is below the original price.
func (l CartLine) HasDiscount() bool {
	return l.OriginalPrice > 0 && l.UnitPrice > 0 && l.OriginalPrice > l.UnitPrice
}

// CartView is a fully recalculated cart. Financial totals mirror the order
// service: line total is unit price × quantity, shipping is summed per line,
// and total is subtotal + shipping. Tax is shown for information only.
type CartView struct {
	Items       []CartLine
	LineCount   int
	ItemCount   int
	Subtotal    float64
	Shipping    float64
	Tax         float64
	Total       float64
	Unavailable int
}

// Empty reports whether the cart has no rows.
func (c CartView) Empty() bool { return len(c.Items) == 0 }

// HasUnavailable reports whether any line cannot currently be purchased.
func (c CartView) HasUnavailable() bool { return c.Unavailable > 0 }

// HasStockWarning reports whether any line exceeds currently available stock.
func (c CartView) HasStockWarning() bool {
	for _, item := range c.Items {
		if item.StockWarning {
			return true
		}
	}
	return false
}

// CartService owns cart business logic for both signed-in customers and guest
// sessions. Signed-in customers keep using the existing CartItem table; guest
// carts use GuestCartItem, keyed by a hashed opaque session cookie.
type CartService struct {
	DB *sql.DB
}

// NewCart returns a cart service backed by db.
func NewCart(db *sql.DB) *CartService { return &CartService{DB: db} }

func (s *CartService) ownerTable(owner CartOwner) (table, column, value string, err error) {
	if !owner.Valid() {
		return "", "", "", ErrCartOwnerInvalid
	}
	if owner.IsGuest() {
		return `"GuestCartItem"`, "sessionId", strings.TrimSpace(owner.GuestKey), nil
	}
	return `"CartItem"`, "customerId", strings.TrimSpace(owner.CustomerID), nil
}

// Get returns the current owner's cart with all monetary values recalculated.
// Inactive or unpriced products remain visible as unavailable lines so the
// user can remove them, but they do not contribute to the totals.
func (s *CartService) Get(ctx context.Context, owner CartOwner) (CartView, error) {
	out := CartView{Items: []CartLine{}}
	table, column, value, err := s.ownerTable(owner)
	if err != nil {
		return out, err
	}
	if s == nil || s.DB == nil {
		return out, nil
	}

	rows, err := s.DB.QueryContext(ctx, `
		SELECT ci."productId", ci."quantity", COALESCE(ci."tax", g."gst", 0),
			p."name", COALESCE(NULLIF(p."slug", ''), p."id"), p."imageGallery",
			p."pricesOriginal", p."pricesDiscounted", p."pricesCalculatedPrice",
			p."shippingCharge", p."isActive", p."stockQuantity", c."name"
		FROM `+table+` ci
		JOIN "Product" p ON p."id" = ci."productId"
		LEFT JOIN "Category" c ON c."id" = p."categoryId"
		LEFT JOIN "Gst" g ON g."id" = p."gstId"
		WHERE ci.`+column+` = ?
		ORDER BY LOWER(COALESCE(p."name", '')) ASC, ci."productId" ASC`, value)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	type scannedLine struct {
		line       CartLine
		original   sql.NullFloat64
		discounted sql.NullFloat64
		calculated sql.NullFloat64
		stockQty   sql.NullInt64
		active     int
		gallery    sql.NullString
		category   sql.NullString
		name       sql.NullString
		slug       sql.NullString
	}
	var scanned []scannedLine
	ids := []string{}
	for rows.Next() {
		var item scannedLine
		var tax sql.NullFloat64
		var shipping sql.NullFloat64
		if err := rows.Scan(&item.line.ProductID, &item.line.Quantity, &tax,
			&item.name, &item.slug, &item.gallery,
			&item.original, &item.discounted, &item.calculated,
			&shipping, &item.active, &item.stockQty, &item.category); err != nil {
			return out, err
		}
		item.line.Name = strings.TrimSpace(item.name.String)
		if item.line.Name == "" {
			item.line.Name = "Untitled product"
		}
		item.line.Slug = strings.TrimSpace(item.slug.String)
		item.line.CategoryName = strings.TrimSpace(item.category.String)
		item.line.TaxRate = math.Max(0, tax.Float64)
		item.line.Shipping = math.Max(0, shipping.Float64)
		item.line.Available = item.active != 0
		scanned = append(scanned, item)
		ids = append(ids, item.line.ProductID)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	stocks, err := stockByProduct(ctx, s.DB, ids)
	if err != nil {
		return out, err
	}

	for _, item := range scanned {
		line := item.line
		price, hasPrice := firstPositive(item.calculated, item.discounted, item.original)
		if !hasPrice {
			line.Available = false
		}
		if item.original.Valid && item.original.Float64 > 0 {
			line.OriginalPrice = round2(item.original.Float64)
		}
		if line.Available {
			line.UnitPrice = round2(price)
		}

		stock := stocks[line.ProductID]
		if !stock.Tracked && item.stockQty.Valid {
			stock = StockInfo{Quantity: int(item.stockQty.Int64), Tracked: true}
		}
		line.Stock = stock.Quantity
		line.StockTracked = stock.Tracked
		if stock.Tracked && line.Quantity > stock.Quantity {
			line.StockWarning = true
		}

		if line.Available {
			line.LineTotal = round2(line.UnitPrice * float64(line.Quantity))
			line.TaxAmount = round2(line.LineTotal * line.TaxRate / 100)
			out.Subtotal += line.LineTotal
			out.Shipping += line.Shipping
			out.Tax += line.TaxAmount
		} else {
			out.Unavailable++
		}
		out.ItemCount += line.Quantity
		out.Items = append(out.Items, line)
	}

	out.LineCount = len(out.Items)
	out.Subtotal = round2(out.Subtotal)
	out.Shipping = round2(out.Shipping)
	out.Tax = round2(out.Tax)
	out.Total = round2(out.Subtotal + out.Shipping)
	return out, nil
}

// Count returns the total quantity in the owner's cart. It is used by the
// header badge and deliberately does not expose another owner's rows.
func (s *CartService) Count(ctx context.Context, owner CartOwner) (int, error) {
	table, column, value, err := s.ownerTable(owner)
	if err != nil {
		return 0, err
	}
	if s == nil || s.DB == nil {
		return 0, nil
	}
	var count int
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM("quantity"), 0) FROM `+table+` WHERE `+column+` = ?`, value).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// Add increases a line by quantity. The product price and stock are resolved
// inside the transaction; client-supplied prices are never accepted.
func (s *CartService) Add(ctx context.Context, owner CartOwner, productID string, quantity int) error {
	productID = strings.TrimSpace(productID)
	table, column, value, err := s.ownerTable(owner)
	if err != nil {
		return err
	}
	if productID == "" {
		return ErrCartProductNotFound
	}
	if quantity <= 0 || quantity > MaxCartQuantity {
		return ErrInvalidQuantity
	}
	if s == nil || s.DB == nil {
		return fmt.Errorf("%w: database unavailable", ErrProductUnavailable)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if !owner.IsGuest() {
		var active, deleted int
		err := tx.QueryRowContext(ctx, `SELECT active, deleted FROM "Customer" WHERE id = ?`, value).
			Scan(&active, &deleted)
		if err == sql.ErrNoRows {
			return ErrCustomerNotFound
		}
		if err != nil {
			return err
		}
		if active == 0 || deleted != 0 {
			return ErrCustomerNotFound
		}
	}

	product, err := resolveCartProduct(ctx, tx, productID)
	if err != nil {
		return err
	}

	var rowID string
	var current int
	err = tx.QueryRowContext(ctx, `SELECT "id", "quantity" FROM `+table+` WHERE `+column+` = ? AND "productId" = ?`,
		value, productID).Scan(&rowID, &current)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	newQuantity := quantity
	if err == nil {
		newQuantity = current + quantity
	}
	if newQuantity > MaxCartQuantity {
		return ErrCartQuantityLimit
	}
	if product.stock.Tracked && newQuantity > product.stock.Quantity {
		return ErrInsufficientStock
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	var tax any
	if product.tax.Valid {
		tax = product.tax.Float64
	}

	if rowID != "" {
		if owner.IsGuest() {
			_, err = tx.ExecContext(ctx,
				`UPDATE "GuestCartItem" SET "quantity" = ?, "tax" = ?, "updatedAt" = ? WHERE "id" = ?`,
				newQuantity, tax, now, rowID)
		} else {
			_, err = tx.ExecContext(ctx,
				`UPDATE "CartItem" SET "quantity" = ?, "tax" = ? WHERE "id" = ?`,
				newQuantity, tax, rowID)
		}
		if err != nil {
			return err
		}
	} else {
		if owner.IsGuest() {
			_, err = tx.ExecContext(ctx, `INSERT INTO "GuestCartItem"
				("id", "sessionId", "productId", "quantity", "tax", "createdAt", "updatedAt")
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				models.NewID(), value, productID, newQuantity, tax, now, now)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO "CartItem"
				("id", "customerId", "productId", "quantity", "tax")
				VALUES (?, ?, ?, ?, ?)`,
				models.NewID(), value, productID, newQuantity, tax)
		}
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// SetQuantity replaces a line's quantity after validating stock and bounds.
// A missing line is an error; use Add to create one.
func (s *CartService) SetQuantity(ctx context.Context, owner CartOwner, productID string, quantity int) error {
	productID = strings.TrimSpace(productID)
	_, column, value, err := s.ownerTable(owner)
	if err != nil {
		return err
	}
	if productID == "" {
		return ErrCartProductNotFound
	}
	if quantity <= 0 || quantity > MaxCartQuantity {
		return ErrInvalidQuantity
	}
	if s == nil || s.DB == nil {
		return fmt.Errorf("%w: database unavailable", ErrProductUnavailable)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	product, err := resolveCartProduct(ctx, tx, productID)
	if err != nil {
		return err
	}
	if product.stock.Tracked && quantity > product.stock.Quantity {
		return ErrInsufficientStock
	}

	var tax any
	if product.tax.Valid {
		tax = product.tax.Float64
	}
	var res sql.Result
	if owner.IsGuest() {
		res, err = tx.ExecContext(ctx,
			`UPDATE "GuestCartItem" SET "quantity" = ?, "tax" = ?, "updatedAt" = ?
			 WHERE `+column+` = ? AND "productId" = ?`,
			quantity, tax, time.Now().UTC().Format(time.RFC3339Nano), value, productID)
	} else {
		res, err = tx.ExecContext(ctx,
			`UPDATE "CartItem" SET "quantity" = ?, "tax" = ?
			 WHERE `+column+` = ? AND "productId" = ?`,
			quantity, tax, value, productID)
	}
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrCartItemNotFound
	}
	return tx.Commit()
}

// Remove deletes a line from the owner's cart. It is idempotent.
func (s *CartService) Remove(ctx context.Context, owner CartOwner, productID string) error {
	productID = strings.TrimSpace(productID)
	table, column, value, err := s.ownerTable(owner)
	if err != nil {
		return err
	}
	if productID == "" {
		return nil
	}
	if s == nil || s.DB == nil {
		return nil
	}
	_, err = s.DB.ExecContext(ctx,
		`DELETE FROM `+table+` WHERE `+column+` = ? AND "productId" = ?`, value, productID)
	return err
}

type resolvedCartProduct struct {
	price float64
	tax   sql.NullFloat64
	stock StockInfo
}

func resolveCartProduct(ctx context.Context, q queryer, productID string) (resolvedCartProduct, error) {
	var active int
	var original, discounted, calculated sql.NullFloat64
	var stockQuantity sql.NullInt64
	var tax sql.NullFloat64
	err := q.QueryRowContext(ctx, `
		SELECT p."isActive", p."pricesOriginal", p."pricesDiscounted", p."pricesCalculatedPrice",
			p."stockQuantity", g."gst"
		FROM "Product" p
		LEFT JOIN "Gst" g ON g."id" = p."gstId"
		WHERE p."id" = ?`, productID).
		Scan(&active, &original, &discounted, &calculated, &stockQuantity, &tax)
	if err == sql.ErrNoRows {
		return resolvedCartProduct{}, ErrCartProductNotFound
	}
	if err != nil {
		return resolvedCartProduct{}, err
	}
	if active == 0 {
		return resolvedCartProduct{}, ErrProductUnavailable
	}
	price, ok := firstPositive(calculated, discounted, original)
	if !ok {
		return resolvedCartProduct{}, ErrProductUnavailable
	}

	stocks, err := stockByProduct(ctx, q, []string{productID})
	if err != nil {
		return resolvedCartProduct{}, err
	}
	stock := stocks[productID]
	if !stock.Tracked && stockQuantity.Valid {
		stock = StockInfo{Quantity: int(stockQuantity.Int64), Tracked: true}
	}
	if stock.Tracked && stock.Quantity <= 0 {
		return resolvedCartProduct{}, ErrInsufficientStock
	}
	return resolvedCartProduct{price: round2(price), tax: tax, stock: stock}, nil
}
