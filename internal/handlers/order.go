package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/models"
	"github.com/mitesh4000/pushti_sangar/server/internal/response"
)

func (a *API) serializeOrder(id string) map[string]any {
	var customerID, first, last, status, bill, ship, country, inv, state, city, post, pay, coupon sql.NullString
	var total sql.NullFloat64
	var invGen int
	var invDate, created, updated sql.NullString
	err := a.DB.QueryRow(`SELECT customerId, FirstName, LastName, status, totalAmount, billingAddress, shippingAddress,
		country, isInvoiceGenrated, invoiceNumber, invoiceGenrationDate, state, city, postCode, paymentMethod, couponId,
		createdAt, updatedAt FROM "Order" WHERE id = ?`, id).
		Scan(&customerID, &first, &last, &status, &total, &bill, &ship, &country, &invGen, &inv, &invDate,
			&state, &city, &post, &pay, &coupon, &created, &updated)
	if err != nil {
		return nil
	}
	items := []map[string]any{}
	rows, err := a.DB.Query(`SELECT id, productId, quantity FROM "OrderItem" WHERE orderId = ?`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var iid string
			var pid sql.NullString
			var qty sql.NullInt64
			if rows.Scan(&iid, &pid, &qty) == nil {
				item := withID(iid, map[string]any{
					"product":  nullStr(nullStringPtr(pid)),
					"quantity": nullInt(nullIntPtr(qty)),
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

func (a *API) CreateOrder(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := models.NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := a.DB.Exec(`INSERT INTO "Order" (
		id, customerId, FirstName, LastName, status, totalAmount, billingAddress, shippingAddress,
		country, isInvoiceGenrated, invoiceNumber, state, city, postCode, paymentMethod, couponId, createdAt, updatedAt
	) VALUES (?,?,?,?,?,?,?,?,?,0,?,?,?,?,?,?,?,?)`,
		id, nullStr(strPtr(body, "customerId")), nullStr(strPtr(body, "FirstName")), nullStr(strPtr(body, "LastName")),
		strDefault(body, "status", "pending"), nullFloat(floatPtr(body, "totalAmount")),
		nullStr(strPtr(body, "billingAddress")), nullStr(strPtr(body, "shippingAddress")),
		nullStr(strPtr(body, "country")), nullStr(strPtr(body, "invoiceNumber")),
		nullStr(strPtr(body, "state")), nullStr(strPtr(body, "city")), nullStr(strPtr(body, "postCode")),
		strDefault(body, "paymentMethod", "COD"), nullStr(strPtr(body, "couponId")), now, now)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	if products, ok := body["products"].([]any); ok {
		for _, p := range products {
			pm, _ := p.(map[string]any)
			if pm == nil {
				continue
			}
			_, _ = a.DB.Exec(`INSERT INTO "OrderItem" (id, orderId, productId, quantity) VALUES (?, ?, ?, ?)`,
				models.NewID(), id, nullStr(strPtr(pm, "product")), nullInt(intPtr(pm, "quantity")))
		}
	}
	response.OK(w, map[string]any{"success": true, "msg": "Order created", "order": a.serializeOrder(id)})
}

func strDefault(m map[string]any, key, def string) string {
	if s := str(m, key); s != "" {
		return s
	}
	return def
}

func (a *API) GetOrders(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`SELECT id FROM "Order" ORDER BY createdAt DESC`)
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
	o := a.serializeOrder(chi.URLParam(r, "id"))
	if o == nil {
		response.Fail(w, "Order not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "order": o})
}

func (a *API) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	sets := []string{"updatedAt = ?"}
	args := []any{time.Now().UTC().Format(time.RFC3339Nano)}
	if _, ok := body["status"]; ok {
		sets = append(sets, "status = ?")
		args = append(args, str(body, "status"))
	}
	if _, ok := body["totalAmount"]; ok {
		sets = append(sets, "totalAmount = ?")
		args = append(args, nullFloat(floatPtr(body, "totalAmount")))
	}
	if response.ToBool(body["isInvoiceGenrated"]) {
		sets = append(sets, "isInvoiceGenrated = 1", "invoiceGenrationDate = ?")
		args = append(args, time.Now().UTC().Format(time.RFC3339Nano))
		// bump invoice counter
		var inv int
		_ = a.DB.QueryRow(`SELECT lattestInvoice FROM "LatestInvoice" WHERE id = '1'`).Scan(&inv)
		inv++
		_, _ = a.DB.Exec(`INSERT INTO "LatestInvoice" (id, lattestInvoice, updatedAt) VALUES ('1', ?, ?)
			ON CONFLICT(id) DO UPDATE SET lattestInvoice = excluded.lattestInvoice, updatedAt = excluded.updatedAt`,
			inv, time.Now().UTC().Format(time.RFC3339Nano))
		sets = append(sets, "invoiceNumber = ?")
		args = append(args, fmt.Sprintf("INV-%d", inv))
	}
	args = append(args, id)
	_, err := a.DB.Exec(`UPDATE "Order" SET `+joinComma(sets)+` WHERE id = ?`, args...)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{"success": true, "order": a.serializeOrder(id)})
}

func (a *API) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, _ = a.DB.Exec(`DELETE FROM "Order" WHERE id = ?`, id)
	response.OK(w, map[string]any{"success": true, "msg": "Order deleted"})
}

func (a *API) GetHighValueCustomers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`
		SELECT customerId, SUM(totalAmount) as total FROM "Order"
		WHERE customerId IS NOT NULL GROUP BY customerId HAVING total >= 50000 ORDER BY total DESC`)
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
		WHERE customerId IS NOT NULL GROUP BY customerId HAVING total >= 10000 AND total < 50000 ORDER BY total DESC`)
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
		SELECT productId, SUM(quantity) as qty FROM "OrderItem"
		WHERE productId IS NOT NULL GROUP BY productId ORDER BY qty DESC LIMIT 20`)
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
	_ = a.DB.QueryRow(`SELECT COUNT(1) FROM "Order"`).Scan(&orders)
	_ = a.DB.QueryRow(`SELECT COUNT(1) FROM "Order" WHERE status = 'pending'`).Scan(&pending)
	var revenue sql.NullFloat64
	_ = a.DB.QueryRow(`SELECT SUM(totalAmount) FROM "Order" WHERE status != 'cancelled'`).Scan(&revenue)
	response.OK(w, map[string]any{
		"success": true,
		"stats": map[string]any{
			"users": users, "customers": customers, "products": products,
			"orders": orders, "pendingOrders": pending,
			"revenue": nullFloat(nullFloatPtr(revenue)),
		},
	})
}
