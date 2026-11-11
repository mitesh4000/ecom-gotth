package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/auth"
	"github.com/mitesh4000/pushti_sangar/server/internal/middleware"
	"github.com/mitesh4000/pushti_sangar/server/internal/models"
	"github.com/mitesh4000/pushti_sangar/server/internal/response"
)

func (a *API) serializeCustomer(id string, withCart bool) map[string]any {
	var username, email, address, paymentMethods, preferences string
	var phone sql.NullString
	var active, deleted int
	var created, updated sql.NullString
	err := a.DB.QueryRow(`
		SELECT username, email, address, phone, paymentMethods, preferences, active, deleted, createdAt, updatedAt
		FROM "Customer" WHERE id = ?`, id).
		Scan(&username, &email, &address, &phone, &paymentMethods, &preferences, &active, &deleted, &created, &updated)
	if err != nil {
		return nil
	}
	m := withID(id, map[string]any{
		"username":       username,
		"email":          email,
		"address":        response.ParseJSONField(address, []any{}),
		"phone":          nullStr(nullStringPtr(phone)),
		"paymentMethods": response.ParseJSONField(paymentMethods, []any{}),
		"preferences":    response.ParseJSONField(preferences, map[string]any{}),
		"active":         active != 0,
		"deleted":        deleted != 0,
		"createdAt":      scanTime(created),
		"updatedAt":      scanTime(updated),
	})
	if withCart {
		m["cartItems"] = a.customerCartItems(id)
		m["wishlist"] = a.customerWishlist(id)
	}
	return m
}

func nullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

func (a *API) customerCartItems(customerID string) []map[string]any {
	rows, err := a.DB.Query(`SELECT id, productId, quantity, tax FROM "CartItem" WHERE customerId = ?`, customerID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	var items []map[string]any
	for rows.Next() {
		var id, pid string
		var qty int
		var tax sql.NullFloat64
		if err := rows.Scan(&id, &pid, &qty, &tax); err != nil {
			continue
		}
		item := withID(id, map[string]any{
			"productId": pid,
			"quantity":  qty,
			"product":   a.serializeProduct(pid),
		})
		if tax.Valid {
			item["tax"] = tax.Float64
		}
		items = append(items, item)
	}
	if items == nil {
		items = []map[string]any{}
	}
	return items
}

func (a *API) customerWishlist(customerID string) []map[string]any {
	rows, err := a.DB.Query(`SELECT id, productId FROM "WishlistItem" WHERE customerId = ?`, customerID)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()
	var items []map[string]any
	for rows.Next() {
		var id, pid string
		if err := rows.Scan(&id, &pid); err != nil {
			continue
		}
		items = append(items, withID(id, map[string]any{
			"productId": pid,
			"product":   a.serializeProduct(pid),
		}))
	}
	if items == nil {
		items = []map[string]any{}
	}
	return items
}

func (a *API) RegisterCustomer(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	if str(body, "password") != str(body, "confirmPassword") {
		response.Fail(w, "Passwords do not match")
		return
	}
	hash, err := auth.HashPassword(str(body, "password"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	id := models.NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = a.DB.Exec(`INSERT INTO "Customer"
		(id, username, email, address, phone, password, paymentMethods, preferences, active, deleted, createdAt, updatedAt)
		VALUES (?, ?, ?, '[]', ?, ?, '[]', '{}', ?, 0, ?, ?)`,
		id, str(body, "username"), str(body, "email"), nullStr(strPtr(body, "phone")), hash,
		boolVal(body, "active", true), now, now)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Success(w, "Customer added successfully", map[string]any{"customer": a.serializeCustomer(id, true)})
}

func (a *API) LoginCustomer(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	email, password := str(body, "email"), str(body, "password")
	if email == "" || password == "" {
		response.Fail(w, "Please fill all required fields")
		return
	}
	var id, hash string
	err := a.DB.QueryRow(`SELECT id, password FROM "Customer" WHERE email = ? AND deleted = 0`, email).Scan(&id, &hash)
	if err != nil || !auth.CheckPassword(hash, password) {
		response.Fail(w, "Invalid Credentials")
		return
	}
	token, _ := auth.GenerateToken(a.Cfg.JWTSecret, id, a.Cfg.TokenTTL)
	http.SetCookie(w, &http.Cookie{
		Name: "token", Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteNoneMode, Secure: true, Expires: time.Now().Add(a.Cfg.TokenTTL),
	})
	response.OK(w, map[string]any{
		"success":  true,
		"msg":      "Successfully LoggedIn",
		"token":    token,
		"customer": a.serializeCustomer(id, true),
	})
}

func (a *API) LogoutCustomer(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "token", Value: "", Path: "/", HttpOnly: true, Expires: time.Unix(0, 0)})
	response.OK(w, map[string]any{"message": "Successfully logged out"})
}

func (a *API) GetCustomers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`SELECT id FROM "Customer" WHERE deleted = 0 ORDER BY createdAt DESC`)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			if c := a.serializeCustomer(id, true); c != nil {
				list = append(list, c)
			}
		}
	}
	if list == nil {
		list = []map[string]any{}
	}
	response.OK(w, map[string]any{"customers": list})
}

func (a *API) GetLoggedInCustomer(w http.ResponseWriter, r *http.Request) {
	id := middleware.UserIDFromContext(r.Context())
	if id == "" {
		body, _ := a.decodeBody(r)
		if t := tokenFromBody(body); t != "" {
			if c, err := auth.ParseToken(a.Cfg.JWTSecret, t); err == nil {
				id = c.ID
			}
		}
	}
	c := a.serializeCustomer(id, true)
	if c == nil {
		response.Fail(w, "User not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "customer": c})
}

func (a *API) GetSpecificCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	c := a.serializeCustomer(id, true)
	if c == nil {
		response.Fail(w, "User not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "customer": c})
}

func (a *API) CustomerLoginStatus(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	token := tokenFromBody(body)
	if token == "" {
		response.OK(w, map[string]any{"success": false})
		return
	}
	_, err := auth.ParseToken(a.Cfg.JWTSecret, token)
	response.OK(w, map[string]any{"success": err == nil})
}

func (a *API) UpdateCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	info, _ := body["customerInfo"].(map[string]any)
	if info == nil {
		info = body
	}
	sets := []string{"updatedAt = ?"}
	args := []any{time.Now().UTC().Format(time.RFC3339Nano)}
	if v := str(info, "username"); v != "" {
		sets = append(sets, "username = ?")
		args = append(args, v)
	}
	if _, ok := info["phone"]; ok {
		sets = append(sets, "phone = ?")
		args = append(args, nullStr(strPtr(info, "phone")))
	}
	if _, ok := info["address"]; ok {
		sets = append(sets, "address = ?")
		args = append(args, response.ToJSONString(info["address"]))
	}
	args = append(args, id)
	_, err := a.DB.Exec(`UPDATE "Customer" SET `+joinComma(sets)+` WHERE id = ?`, args...)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{
		"success": true, "message": "Customer updated successfully",
		"customer": a.serializeCustomer(id, true),
	})
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}

func (a *API) UpdateCustomerPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	var hash string
	if err := a.DB.QueryRow(`SELECT password FROM "Customer" WHERE id = ?`, id).Scan(&hash); err != nil {
		response.Fail(w, "User not found , Please signup")
		return
	}
	oldP, newP := str(body, "oldPassword"), str(body, "newPassword")
	if oldP == "" || newP == "" {
		response.Fail(w, "Please add old and new password")
		return
	}
	if !auth.CheckPassword(hash, oldP) {
		response.Fail(w, "Old password is incorrect")
		return
	}
	if oldP == newP {
		response.Fail(w, "New Password cannot be same as Old password")
		return
	}
	nh, _ := auth.HashPassword(newP)
	_, _ = a.DB.Exec(`UPDATE "Customer" SET password = ?, updatedAt = ? WHERE id = ?`, nh, time.Now().UTC().Format(time.RFC3339Nano), id)
	response.Success(w, "Password changed successfully", nil)
}

func (a *API) ForgotCustomerPassword(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	email := str(body, "email")
	var id, username string
	if err := a.DB.QueryRow(`SELECT id, username FROM "Customer" WHERE email = ?`, email).Scan(&id, &username); err != nil {
		response.Fail(w, "Customer not found")
		return
	}
	_, _ = a.DB.Exec(`DELETE FROM "Token" WHERE customerId = ?`, id)
	raw := randomHex(32) + id
	h := sha256.Sum256([]byte(raw))
	hashed := hex.EncodeToString(h[:])
	now := time.Now().UTC()
	_, err := a.DB.Exec(`INSERT INTO "Token" (id, customerId, token, createdAt, expiresAt) VALUES (?, ?, ?, ?, ?)`,
		models.NewID(), id, hashed, now.Format(time.RFC3339Nano), now.Add(30*time.Minute).Format(time.RFC3339Nano))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	resetURL := fmt.Sprintf("%s/resetpassword/%s", a.Cfg.FrontendURL, raw)
	_ = a.sendEmail("Password reset request",
		fmt.Sprintf("<h2>Hello %s</h2><p>Please use this URL below to reset your password</p><a href=%s>%s</a>", username, resetURL, resetURL),
		email)
	response.Success(w, "Reset Email Sent", nil)
}

func (a *API) ResetCustomerPassword(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	resetToken := chi.URLParam(r, "resetToken")
	h := sha256.Sum256([]byte(resetToken))
	hashed := hex.EncodeToString(h[:])
	var customerID sql.NullString
	err := a.DB.QueryRow(`SELECT customerId FROM "Token" WHERE token = ? AND expiresAt > ?`,
		hashed, time.Now().UTC().Format(time.RFC3339Nano)).Scan(&customerID)
	if err != nil || !customerID.Valid {
		response.OK(w, "Invalid or Expired Token")
		return
	}
	nh, _ := auth.HashPassword(str(body, "password"))
	_, _ = a.DB.Exec(`UPDATE "Customer" SET password = ?, updatedAt = ? WHERE id = ?`,
		nh, time.Now().UTC().Format(time.RFC3339Nano), customerID.String)
	response.Success(w, "Password Reset Successful, Please Login", nil)
}

func (a *API) DeleteCustomer(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	id := str(body, "id")
	_, err := a.DB.Exec(`DELETE FROM "Customer" WHERE id = ?`, id)
	if err != nil {
		response.OK(w, map[string]any{"success": true, "message": "record not found"})
		return
	}
	response.OK(w, map[string]any{"success": true, "message": "Successfully Deleted"})
}

func (a *API) AddToCart(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	productID := str(body, "productId")
	qty := intVal(body, "quantity", 1)

	var exists int
	if err := a.DB.QueryRow(`SELECT COUNT(1) FROM "Customer" WHERE id = ?`, customerID).Scan(&exists); err != nil || exists == 0 {
		response.FailStatus(w, 404, "Customer not found")
		return
	}
	var gstID sql.NullString
	if err := a.DB.QueryRow(`SELECT gstId FROM "Product" WHERE id = ?`, productID).Scan(&gstID); err != nil {
		response.FailStatus(w, 404, "Product not found")
		return
	}
	var cartID string
	var curQty int
	err := a.DB.QueryRow(`SELECT id, quantity FROM "CartItem" WHERE customerId = ? AND productId = ?`, customerID, productID).
		Scan(&cartID, &curQty)
	if err == nil {
		_, _ = a.DB.Exec(`UPDATE "CartItem" SET quantity = ? WHERE id = ?`, curQty+qty, cartID)
	} else {
		var tax any
		if gstID.Valid {
			var g float64
			if a.DB.QueryRow(`SELECT gst FROM "Gst" WHERE id = ?`, gstID.String).Scan(&g) == nil {
				tax = g
			}
		}
		_, err = a.DB.Exec(`INSERT INTO "CartItem" (id, customerId, productId, quantity, tax) VALUES (?, ?, ?, ?, ?)`,
			models.NewID(), customerID, productID, qty, tax)
		if err != nil {
			response.FailStatus(w, 500, "Internal Server Error")
			return
		}
	}
	response.OK(w, map[string]any{
		"success": true, "msg": "Cart updated successfully",
		"updatedCart": a.serializeCustomer(customerID, true),
	})
}

func (a *API) GetCustomerCartItems(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	items := a.customerCartItems(customerID)
	// Enrich with stock
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		pid, _ := it["productId"].(string)
		var stock map[string]any
		var sid string
		var qty int
		var price float64
		err := a.DB.QueryRow(`SELECT id, quantity, currentPricePerUnit FROM "Stock" WHERE productId = ? LIMIT 1`, pid).
			Scan(&sid, &qty, &price)
		if err == nil {
			stock = withID(sid, map[string]any{"quantity": qty, "currentPricePerUnit": price, "ProductId": pid})
		}
		out = append(out, map[string]any{
			"product":  it["product"],
			"quantity": it["quantity"],
			"tax":      it["tax"],
			"stock":    stock,
		})
	}
	response.OK(w, map[string]any{"success": true, "cartItems": out})
}

func (a *API) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	_, _ = a.DB.Exec(`DELETE FROM "CartItem" WHERE customerId = ? AND productId = ?`, customerID, str(body, "productId"))
	response.OK(w, map[string]any{
		"success": true, "msg": "Product removed from the cart successfully",
		"updatedCart": a.serializeCustomer(customerID, true),
	})
}

func (a *API) AddToWishlist(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	productID := str(body, "productId")
	_, err := a.DB.Exec(`INSERT OR IGNORE INTO "WishlistItem" (id, customerId, productId) VALUES (?, ?, ?)`,
		models.NewID(), customerID, productID)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{
		"success": true, "msg": "Added to wishlist",
		"customer": a.serializeCustomer(customerID, true),
	})
}

func (a *API) RemoveFromWishlist(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	body, _ := a.decodeBody(r)
	_, _ = a.DB.Exec(`DELETE FROM "WishlistItem" WHERE customerId = ? AND productId = ?`, customerID, str(body, "productId"))
	response.OK(w, map[string]any{
		"success": true, "msg": "Removed from wishlist",
		"customer": a.serializeCustomer(customerID, true),
	})
}
