package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/auth"
	"github.com/mitesh4000/pushti_sangar/server/internal/middleware"
	"github.com/mitesh4000/pushti_sangar/server/internal/models"
	"github.com/mitesh4000/pushti_sangar/server/internal/response"
	"github.com/mitesh4000/pushti_sangar/server/internal/service"
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

func (a *API) customerEmailTaken(email, exceptID string) bool {
	var id string
	err := a.DB.QueryRow(`SELECT id FROM "Customer" WHERE lower(email) = lower(?)`, email).Scan(&id)
	return err == nil && id != exceptID
}

// RegisterCustomer always creates an active, non-deleted customer. Client
// supplied active/deleted values are ignored.
func (a *API) RegisterCustomer(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	username := strings.TrimSpace(str(body, "username"))
	email := strings.TrimSpace(str(body, "email"))
	password := str(body, "password")
	if username == "" || email == "" || password == "" {
		response.Fail(w, "Please fill all required fields")
		return
	}
	if !validEmail(email) {
		response.Fail(w, "Please provide a valid email address")
		return
	}
	if password != str(body, "confirmPassword") {
		response.Fail(w, "Passwords do not match")
		return
	}
	if err := validatePassword(password); err != nil {
		response.Fail(w, err.Error())
		return
	}
	if a.customerEmailTaken(email, "") {
		response.Fail(w, "Email already registered")
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	id := models.NewID()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = a.DB.Exec(`INSERT INTO "Customer"
		(id, username, email, address, phone, password, paymentMethods, preferences, active, deleted, createdAt, updatedAt)
		VALUES (?, ?, ?, '[]', ?, ?, '[]', '{}', 1, 0, ?, ?)`,
		id, username, email, nullStr(strPtr(body, "phone")), hash, now, now)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Success(w, "Customer added successfully", map[string]any{"customer": a.serializeCustomer(id, true)})
}

func (a *API) LoginCustomer(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	email, password := strings.TrimSpace(str(body, "email")), str(body, "password")
	if email == "" || password == "" {
		response.Fail(w, "Please fill all required fields")
		return
	}
	var id, hash string
	var active int
	err := a.DB.QueryRow(`SELECT id, password, active FROM "Customer" WHERE email = ? AND deleted = 0`, email).
		Scan(&id, &hash, &active)
	if err != nil || !auth.CheckPassword(hash, password) {
		response.Fail(w, "Invalid Credentials")
		return
	}
	if active == 0 {
		response.Fail(w, "Account is disabled")
		return
	}
	token, err := auth.GenerateTokenWithClaims(a.Cfg.JWTSecret, id, auth.KindCustomer, "customer", a.Cfg.TokenTTL)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	a.setAuthCookie(w, token)
	response.OK(w, map[string]any{
		"success":  true,
		"msg":      "Successfully LoggedIn",
		"token":    token,
		"customer": a.serializeCustomer(id, true),
	})
}

func (a *API) LogoutCustomer(w http.ResponseWriter, r *http.Request) {
	clearAuthCookie(w)
	response.OK(w, map[string]any{"message": "Successfully logged out"})
}

func (a *API) GetCustomers(w http.ResponseWriter, r *http.Request) {
	body, _ := a.requestInput(r)
	limit, offset := pagination(body)
	rows, err := a.DB.Query(`SELECT id FROM "Customer" WHERE deleted = 0 ORDER BY createdAt DESC LIMIT ? OFFSET ?`, limit, offset)
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
	id := middleware.CustomerIDFromContext(r.Context())
	if id == "" {
		response.Fail(w, "Not authorized")
		return
	}
	c := a.serializeCustomer(id, true)
	if c == nil {
		response.Fail(w, "User not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "customer": c})
}

// GetSpecificCustomer lets a customer read only their own record. Staff may
// read any customer.
func (a *API) GetSpecificCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !a.canAccessCustomer(r, id) {
		response.FailStatus(w, http.StatusForbidden, "Forbidden")
		return
	}
	c := a.serializeCustomer(id, true)
	if c == nil {
		response.Fail(w, "User not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "customer": c})
}

func (a *API) CustomerLoginStatus(w http.ResponseWriter, r *http.Request) {
	body, _ := a.requestInput(r)
	token := tokenFromBody(body)
	if token == "" {
		token = middleware.TokenFromRequest(r)
	}
	if token == "" {
		response.OK(w, map[string]any{"success": false})
		return
	}
	claims, err := auth.ParseToken(a.Cfg.JWTSecret, token)
	if err != nil || (claims.Kind != "" && claims.Kind != auth.KindCustomer) {
		response.OK(w, map[string]any{"success": false})
		return
	}
	response.OK(w, map[string]any{"success": true})
}

func (a *API) UpdateCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !a.canAccessCustomer(r, id) {
		response.FailStatus(w, http.StatusForbidden, "Forbidden")
		return
	}
	var exists int
	if err := a.DB.QueryRow(`SELECT 1 FROM "Customer" WHERE id = ? AND deleted = 0`, id).Scan(&exists); err != nil {
		response.Fail(w, "Customer not found")
		return
	}
	body, _ := a.decodeBody(r)
	info, _ := body["customerInfo"].(map[string]any)
	if info == nil {
		info = body
	}
	sets := []string{"updatedAt = ?"}
	args := []any{time.Now().UTC().Format(time.RFC3339Nano)}
	if v := strings.TrimSpace(str(info, "username")); v != "" {
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

// ReplaceCustomer implements PUT semantics for the mutable customer profile.
// The email address is immutable here; username is required. Omitted optional
// fields are cleared rather than left untouched. Use PATCH for partial updates.
func (a *API) ReplaceCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !a.canAccessCustomer(r, id) {
		response.FailStatus(w, http.StatusForbidden, "Forbidden")
		return
	}
	body, err := a.decodeBody(r)
	if err != nil {
		response.Fail(w, "invalid body")
		return
	}
	info, _ := body["customerInfo"].(map[string]any)
	if info == nil {
		info = body
	}
	username := strings.TrimSpace(str(info, "username"))
	if username == "" {
		response.Fail(w, "Customer username is required")
		return
	}
	res, err := a.DB.Exec(`UPDATE "Customer" SET username = ?, phone = ?, address = ?, updatedAt = ?
		WHERE id = ? AND deleted = 0`,
		username, nullStr(strPtr(info, "phone")), response.ToJSONString(info["address"]),
		time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		response.Fail(w, "Customer not found")
		return
	}
	response.OK(w, map[string]any{
		"success": true, "message": "Customer replaced",
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
	if !a.canAccessCustomer(r, id) {
		response.FailStatus(w, http.StatusForbidden, "Forbidden")
		return
	}
	body, _ := a.decodeBody(r)
	var hash string
	if err := a.DB.QueryRow(`SELECT password FROM "Customer" WHERE id = ? AND deleted = 0`, id).Scan(&hash); err != nil {
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
	if err := validatePassword(newP); err != nil {
		response.Fail(w, err.Error())
		return
	}
	nh, err := auth.HashPassword(newP)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	_, _ = a.DB.Exec(`UPDATE "Customer" SET password = ?, updatedAt = ? WHERE id = ?`, nh, time.Now().UTC().Format(time.RFC3339Nano), id)
	response.Success(w, "Password changed successfully", nil)
}

func (a *API) ForgotCustomerPassword(w http.ResponseWriter, r *http.Request) {
	body, _ := a.decodeBody(r)
	email := strings.TrimSpace(str(body, "email"))
	if !validEmail(email) {
		response.Fail(w, "Please provide a valid email address")
		return
	}
	var id, username string
	if err := a.DB.QueryRow(`SELECT id, username FROM "Customer" WHERE email = ? AND deleted = 0`, email).Scan(&id, &username); err != nil {
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
	password := str(body, "password")
	if err := validatePassword(password); err != nil {
		response.Fail(w, err.Error())
		return
	}
	resetToken := chi.URLParam(r, "resetToken")
	h := sha256.Sum256([]byte(resetToken))
	hashed := hex.EncodeToString(h[:])
	var customerID sql.NullString
	err := a.DB.QueryRow(`SELECT customerId FROM "Token" WHERE token = ? AND expiresAt > ? AND usedAt IS NULL`,
		hashed, time.Now().UTC().Format(time.RFC3339Nano)).Scan(&customerID)
	if err != nil || !customerID.Valid {
		response.OK(w, "Invalid or Expired Token")
		return
	}
	nh, err := auth.HashPassword(password)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := a.DB.Begin()
	if err != nil {
		response.FailErr(w, err)
		return
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE "Customer" SET password = ?, updatedAt = ? WHERE id = ?`, nh, now, customerID.String); err != nil {
		response.FailErr(w, err)
		return
	}
	if _, err := tx.Exec(`UPDATE "Token" SET usedAt = ? WHERE token = ?`, now, hashed); err != nil {
		response.FailErr(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		response.FailErr(w, err)
		return
	}
	response.Success(w, "Password Reset Successful, Please Login", nil)
}

// DeleteCustomer is a soft delete: the row is retained so historical orders and
// reports keep their customer reference.
func (a *API) DeleteCustomer(w http.ResponseWriter, r *http.Request) {
	body, _ := a.requestInput(r)
	id := str(body, "id")
	if id == "" {
		id = chi.URLParam(r, "id")
	}
	if id == "" {
		response.Fail(w, "Customer id required")
		return
	}
	res, err := a.DB.Exec(`UPDATE "Customer" SET deleted = 1, active = 0, updatedAt = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		response.Fail(w, "Customer not found")
		return
	}
	response.OK(w, map[string]any{"success": true, "message": "Successfully Deleted"})
}

func (a *API) AddToCart(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if !a.canAccessCustomer(r, customerID) {
		response.FailStatus(w, http.StatusForbidden, "Forbidden")
		return
	}
	body, _ := a.decodeBody(r)
	productID := strings.TrimSpace(str(body, "productId"))
	qty := intVal(body, "quantity", 1)
	if productID == "" || qty <= 0 || qty > service.MaxCartQuantity {
		response.Fail(w, "Please provide a valid product and quantity")
		return
	}

	err := service.NewCart(a.DB).Add(r.Context(), service.CartOwner{CustomerID: customerID}, productID, qty)
	switch {
	case err == nil:
	case errors.Is(err, service.ErrCustomerNotFound):
		response.FailStatus(w, http.StatusNotFound, "Customer not found")
		return
	case errors.Is(err, service.ErrCartProductNotFound):
		response.FailStatus(w, http.StatusNotFound, "Product not found")
		return
	case errors.Is(err, service.ErrProductUnavailable):
		response.Fail(w, "Product is not available")
		return
	case errors.Is(err, service.ErrCartQuantityLimit):
		response.Fail(w, "Quantity limit exceeded")
		return
	case errors.Is(err, service.ErrInsufficientStock):
		response.Fail(w, "Not enough stock available")
		return
	case errors.Is(err, service.ErrInvalidQuantity):
		response.Fail(w, "Please provide a valid product and quantity")
		return
	default:
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{
		"success": true, "msg": "Cart updated successfully",
		"updatedCart": a.serializeCustomer(customerID, true),
	})
}

func (a *API) GetCustomerCartItems(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if !a.canAccessCustomer(r, customerID) {
		response.FailStatus(w, http.StatusForbidden, "Forbidden")
		return
	}
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
	if !a.canAccessCustomer(r, customerID) {
		response.FailStatus(w, http.StatusForbidden, "Forbidden")
		return
	}
	body, _ := a.requestInput(r)
	if err := service.NewCart(a.DB).Remove(r.Context(), service.CartOwner{CustomerID: customerID}, str(body, "productId")); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, map[string]any{
		"success": true, "msg": "Product removed from the cart successfully",
		"updatedCart": a.serializeCustomer(customerID, true),
	})
}

func (a *API) AddToWishlist(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if !a.canAccessCustomer(r, customerID) {
		response.FailStatus(w, http.StatusForbidden, "Forbidden")
		return
	}
	body, _ := a.decodeBody(r)
	productID := strings.TrimSpace(str(body, "productId"))
	if productID == "" {
		response.Fail(w, "Product id required")
		return
	}
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
	if !a.canAccessCustomer(r, customerID) {
		response.FailStatus(w, http.StatusForbidden, "Forbidden")
		return
	}
	body, _ := a.requestInput(r)
	_, _ = a.DB.Exec(`DELETE FROM "WishlistItem" WHERE customerId = ? AND productId = ?`, customerID, str(body, "productId"))
	response.OK(w, map[string]any{
		"success": true, "msg": "Removed from wishlist",
		"customer": a.serializeCustomer(customerID, true),
	})
}
