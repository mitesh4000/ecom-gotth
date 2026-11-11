package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/middleware"
	"github.com/mitesh4000/pushti_sangar/server/internal/response"
	"github.com/mitesh4000/pushti_sangar/server/internal/service"
)

var validOrderStatuses = map[string]bool{
	"pending":    true,
	"processing": true,
	"confirmed":  true,
	"shipped":    true,
	"delivered":  true,
	"cancelled":  true,
}

func (a *API) serializeOrder(id string) map[string]any {
	var customerID, first, last, status, bill, ship, country, inv, state, city, post, pay, coupon, paymentStatus sql.NullString
	var total sql.NullFloat64
	var invGen int
	var invDate, created, updated sql.NullString
	err := a.DB.QueryRow(`SELECT customerId, FirstName, LastName, status, totalAmount, billingAddress, shippingAddress,
		country, isInvoiceGenrated, invoiceNumber, invoiceGenrationDate, state, city, postCode, paymentMethod, couponId,
		paymentStatus, createdAt, updatedAt FROM "Order" WHERE id = ? AND deletedAt IS NULL`, id).
		Scan(&customerID, &first, &last, &status, &total, &bill, &ship, &country, &invGen, &inv, &invDate,
			&state, &city, &post, &pay, &coupon, &paymentStatus, &created, &updated)
	if err != nil {
		return nil
	}
	items := []map[string]any{}
	rows, err := a.DB.Query(`SELECT id, productId, quantity, productName, sku, unitPrice, lineTotal FROM "OrderItem" WHERE orderId = ?`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var iid string
			var pid, name, sku sql.NullString
			var qty sql.NullInt64
			var unitPrice, lineTotal sql.NullFloat64
			if rows.Scan(&iid, &pid, &qty, &name, &sku, &unitPrice, &lineTotal) == nil {
				item := withID(iid, map[string]any{
					"product":     nullStr(nullStringPtr(pid)),
					"quantity":    nullInt(nullIntPtr(qty)),
					"productName": nullStr(nullStringPtr(name)),
					"sku":         nullStr(nullStringPtr(sku)),
					"unitPrice":   nullFloat(nullFloatPtr(unitPrice)),
					"lineTotal":   nullFloat(nullFloatPtr(lineTotal)),
				})
				if pid.Valid {
					item["productData"] = a.serializeProduct(pid.String)
				}
				items = append(items, item)
			}
		}
	}
	return withID(id, map[string]any{
		"customer":             nullStr(nullStringPtr(customerID)),
		"customerId":           nullStr(nullStringPtr(customerID)),
		"FirstName":            nullStr(nullStringPtr(first)),
		"LastName":             nullStr(nullStringPtr(last)),
		"status":               status.String,
		"paymentStatus":        paymentStatus.String,
		"totalAmount":          nullFloat(nullFloatPtr(total)),
		"billingAddress":       nullStr(nullStringPtr(bill)),
		"shippingAddress":      nullStr(nullStringPtr(ship)),
		"country":              nullStr(nullStringPtr(country)),
		"isInvoiceGenrated":    invGen != 0,
		"invoiceNumber":        nullStr(nullStringPtr(inv)),
		"invoiceGenrationDate": scanNullTime(invDate),
		"state":                nullStr(nullStringPtr(state)),
		"city":                 nullStr(nullStringPtr(city)),
		"postCode":             nullStr(nullStringPtr(post)),
		"paymentMethod":        pay.String,
		"couponCode":           nullStr(nullStringPtr(coupon)),
		"couponId":             nullStr(nullStringPtr(coupon)),
		"products":             items,
		"createdAt":            scanTime(created),
		"updatedAt":            scanTime(updated),
	})
}

func (a *API) orderBelongsToCustomer(orderID, customerID string) bool {
	var owner sql.NullString
	if err := a.DB.QueryRow(`SELECT customerId FROM "Order" WHERE id = ? AND deletedAt IS NULL`, orderID).Scan(&owner); err != nil {
		return false
	}
	return owner.Valid && owner.String == customerID
}

// CreateOrder accepts only product IDs and quantities. Prices, totals, status
// and payment status are always resolved by the service layer.
func (a *API) CreateOrder(w http.ResponseWriter, r *http.Request) {
	body, err := a.decodeBody(r)
	if err != nil {
		response.Fail(w, "invalid body")
		return
	}

	customerID := strings.TrimSpace(str(body, "customerId"))
	if actor := middleware.CustomerIDFromContext(r.Context()); actor != "" {
		// A customer can only place orders for their own account.
		customerID = actor
	}

	products, ok := body["products"].([]any)
	if !ok || len(products) == 0 {
		response.Fail(w, "Order must contain at least one product")
		return
	}
	items := make([]service.ItemInput, 0, len(products))
	for _, raw := range products {
		pm, _ := raw.(map[string]any)
		if pm == nil {
			response.Fail(w, "Invalid order item")
			return
		}
		productID := strings.TrimSpace(str(pm, "product"))
		if productID == "" {
			productID = strings.TrimSpace(str(pm, "productId"))
		}
		items = append(items, service.ItemInput{
			ProductID: productID,
			Quantity:  intVal(pm, "quantity", 0),
		})
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(str(body, "idempotencyKey"))
	}

	placed, err := service.Place(r.Context(), a.DB, service.PlaceInput{
		CustomerID:      customerID,
		FirstName:       str(body, "FirstName"),
		LastName:        str(body, "LastName"),
		BillingAddress:  str(body, "billingAddress"),
		ShippingAddress: str(body, "shippingAddress"),
		Country:         str(body, "country"),
		State:           str(body, "state"),
		City:            str(body, "city"),
		PostCode:        str(body, "postCode"),
		PaymentMethod:   str(body, "paymentMethod"),
		CouponID:        str(body, "couponId"),
		IdempotencyKey:  idempotencyKey,
		Items:           items,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInsufficientStock):
			response.Fail(w, "Not enough stock available for one or more products")
		case errors.Is(err, service.ErrProductUnavailable):
			response.Fail(w, "One or more products are unavailable")
		case errors.Is(err, service.ErrInvalidCoupon):
			response.Fail(w, "Coupon is invalid or expired")
		case errors.Is(err, service.ErrCustomerNotFound):
			response.Fail(w, "Customer not found")
		case errors.Is(err, service.ErrInvalidQuantity):
			response.Fail(w, "Quantity must be a positive integer")
		case errors.Is(err, service.ErrEmptyOrder):
			response.Fail(w, "Order must contain at least one product")
		default:
			response.FailErr(w, err)
		}
		return
	}

	msg := "Order created"
	if placed.Reused {
		msg = "Order already exists"
	}
	response.OK(w, map[string]any{
		"success":       true,
		"msg":           msg,
		"order":         a.serializeOrder(placed.ID),
		"totalAmount":   placed.Total,
		"paymentStatus": placed.PaymentStatus,
	})
}

func (a *API) GetOrders(w http.ResponseWriter, r *http.Request) {
	body, _ := a.requestInput(r)
	limit, offset := pagination(body)
	customerID := middleware.CustomerIDFromContext(r.Context())
	var (
		rows *sql.Rows
		err  error
	)
	if customerID != "" {
		rows, err = a.DB.Query(`SELECT id FROM "Order" WHERE customerId = ? AND deletedAt IS NULL ORDER BY createdAt DESC LIMIT ? OFFSET ?`,
			customerID, limit, offset)
	} else {
		rows, err = a.DB.Query(`SELECT id FROM "Order" WHERE deletedAt IS NULL ORDER BY createdAt DESC LIMIT ? OFFSET ?`, limit, offset)
	}
	if err != nil {
		response.FailErr(w, err)
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			if o := a.serializeOrder(id); o != nil {
				list = append(list, o)
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"orders": list})
}

func (a *API) GetOrderByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if customerID := middleware.CustomerIDFromContext(r.Context()); customerID != "" {
		if !a.orderBelongsToCustomer(id, customerID) {
			response.FailStatus(w, http.StatusForbidden, "Forbidden")
			return
		}
	}
	o := a.serializeOrder(id)
	if o == nil {
		response.Fail(w, "Order not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "order": o})
}

// UpdateOrder is restricted to admins/managers. Clients cannot change totals;
// cancelling goes through the service so inventory is restored atomically.
func (a *API) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)

	if status := strings.ToLower(strings.TrimSpace(str(body, "status"))); status != "" {
		if !validOrderStatuses[status] {
			response.Fail(w, "Invalid order status")
			return
		}
		if status == "cancelled" {
			if err := service.Cancel(r.Context(), a.DB, id); err != nil {
				if errors.Is(err, service.ErrOrderNotFound) {
					response.Fail(w, "Order not found")
					return
				}
				response.FailErr(w, err)
				return
			}
		} else {
			_, err := a.DB.Exec(`UPDATE "Order" SET status = ?, updatedAt = ? WHERE id = ? AND deletedAt IS NULL`,
				status, time.Now().UTC().Format(time.RFC3339Nano), id)
			if err != nil {
				response.FailErr(w, err)
				return
			}
		}
	}

	if response.ToBool(body["isInvoiceGenrated"]) {
		if err := a.generateInvoice(id); err != nil {
			response.FailErr(w, err)
			return
		}
	}
	response.OK(w, map[string]any{"success": true, "order": a.serializeOrder(id)})
}

func (a *API) generateInvoice(id string) error {
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var inv int
	_ = tx.QueryRow(`SELECT lattestInvoice FROM "LatestInvoice" WHERE id = '1'`).Scan(&inv)
	inv++
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.Exec(`INSERT INTO "LatestInvoice" (id, lattestInvoice, updatedAt) VALUES ('1', ?, ?)
		ON CONFLICT(id) DO UPDATE SET lattestInvoice = excluded.lattestInvoice, updatedAt = excluded.updatedAt`,
		inv, now); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE "Order" SET isInvoiceGenrated = 1, invoiceNumber = ?, invoiceGenrationDate = ?, updatedAt = ?
		WHERE id = ? AND deletedAt IS NULL`, fmt.Sprintf("INV-%d", inv), now, now, id); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteOrder performs a soft delete and restocks any not-yet-cancelled order.
func (a *API) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := service.SoftDelete(r.Context(), a.DB, id); err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			response.Fail(w, "Order not found")
			return
		}
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "msg": "Order deleted"})
}

func (a *API) GetHighValueCustomers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`
		SELECT customerId, SUM(totalAmount) as total FROM "Order"
		WHERE customerId IS NOT NULL AND deletedAt IS NULL GROUP BY customerId HAVING total >= 50000 ORDER BY total DESC`)
	if err != nil {
		response.OK(w, map[string]any{"customers": []any{}})
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var cid string
		var total float64
		if rows.Scan(&cid, &total) == nil {
			list = append(list, map[string]any{"customerId": cid, "total": total, "customer": a.serializeCustomer(cid, false)})
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"customers": list})
}

func (a *API) GetMedValueCustomers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`
		SELECT customerId, SUM(totalAmount) as total FROM "Order"
		WHERE customerId IS NOT NULL AND deletedAt IS NULL GROUP BY customerId HAVING total >= 10000 AND total < 50000 ORDER BY total DESC`)
	if err != nil {
		response.OK(w, map[string]any{"customers": []any{}})
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var cid string
		var total float64
		if rows.Scan(&cid, &total) == nil {
			list = append(list, map[string]any{"customerId": cid, "total": total, "customer": a.serializeCustomer(cid, false)})
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"customers": list})
}

func (a *API) GetTopSellingProducts(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`
		SELECT oi.productId, SUM(oi.quantity) as qty FROM "OrderItem" oi
		JOIN "Order" o ON o.id = oi.orderId
		WHERE oi.productId IS NOT NULL AND o.deletedAt IS NULL GROUP BY oi.productId ORDER BY qty DESC LIMIT 20`)
	if err != nil {
		response.OK(w, map[string]any{"products": []any{}})
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var pid string
		var qty int
		if rows.Scan(&pid, &qty) == nil {
			list = append(list, map[string]any{"productId": pid, "quantity": qty, "product": a.serializeProduct(pid)})
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"products": list})
}

func (a *API) Dashboard(w http.ResponseWriter, r *http.Request) {
	var users, customers, products, orders, pending int
	_ = a.DB.QueryRow(`SELECT COUNT(1) FROM "User"`).Scan(&users)
	_ = a.DB.QueryRow(`SELECT COUNT(1) FROM "Customer" WHERE deleted = 0`).Scan(&customers)
	_ = a.DB.QueryRow(`SELECT COUNT(1) FROM "Product" WHERE isActive = 1`).Scan(&products)
	_ = a.DB.QueryRow(`SELECT COUNT(1) FROM "Order" WHERE deletedAt IS NULL`).Scan(&orders)
	_ = a.DB.QueryRow(`SELECT COUNT(1) FROM "Order" WHERE status = 'pending' AND deletedAt IS NULL`).Scan(&pending)
	var revenue sql.NullFloat64
	_ = a.DB.QueryRow(`SELECT SUM(totalAmount) FROM "Order" WHERE status != 'cancelled' AND deletedAt IS NULL`).Scan(&revenue)
	response.OK(w, map[string]any{
		"success": true,
		"stats": map[string]any{
			"users": users, "customers": customers, "products": products,
			"orders": orders, "pendingOrders": pending,
			"revenue": nullFloat(nullFloatPtr(revenue)),
		},
	})
}
