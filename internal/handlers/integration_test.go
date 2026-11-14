package handlers_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/auth"
	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/database"
	"github.com/mitesh4000/pushti_sangar/server/internal/handlers"
	"github.com/mitesh4000/pushti_sangar/server/internal/models"
	"github.com/mitesh4000/pushti_sangar/server/internal/service"
	"github.com/mitesh4000/pushti_sangar/server/migrations"
)

const jwtSecret = "integration-test-secret-1234567890"

type testEnv struct {
	t       *testing.T
	db      *sql.DB
	cfg     *config.Config
	handler http.Handler
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	return newTestEnvWith(t, nil)
}

func newTestEnvWith(t *testing.T, configure func(*config.Config)) *testEnv {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Apply(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{
		Port:         "0",
		Env:          "test",
		DatabaseURL:  "test.db",
		JWTSecret:    jwtSecret,
		FrontendURL:  "http://localhost:3000",
		AdminURL:     "http://localhost:3001",
		CORSOrigins:  []string{"http://localhost:3000"},
		UploadsDir:   t.TempDir(),
		TokenTTL:     time.Hour,
		MaxBodyBytes: 4 << 20,
		AuthStatus:   "strict",
	}
	if configure != nil {
		configure(cfg)
	}
	api := &handlers.API{DB: db, Cfg: cfg}
	return &testEnv{t: t, db: db, cfg: cfg, handler: api.Routes()}
}

func (e *testEnv) do(method, path, token string, body any) (*httptest.ResponseRecorder, map[string]any) {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			e.t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec, out
}

func (e *testEnv) userToken(id, role string) string {
	e.t.Helper()
	token, err := auth.GenerateTokenWithClaims(jwtSecret, id, auth.KindUser, role, time.Hour)
	if err != nil {
		e.t.Fatalf("user token: %v", err)
	}
	return token
}

func (e *testEnv) customerToken(id string) string {
	e.t.Helper()
	token, err := auth.GenerateTokenWithClaims(jwtSecret, id, auth.KindCustomer, "customer", time.Hour)
	if err != nil {
		e.t.Fatalf("customer token: %v", err)
	}
	return token
}

func (e *testEnv) insertUser(id, email, password, role string) {
	e.t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		e.t.Fatalf("hash: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := e.db.Exec(`INSERT INTO "User" (id, name, email, password, photo, active, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, '', 1, ?, ?)`, id, id, email, hash, now, now); err != nil {
		e.t.Fatalf("insert user: %v", err)
	}
	var roleID string
	err = e.db.QueryRow(`SELECT id FROM "Role" WHERE role = ?`, role).Scan(&roleID)
	if err == sql.ErrNoRows {
		roleID = models.NewID()
		if _, err := e.db.Exec(`INSERT INTO "Role" (id, role, permissions) VALUES (?, ?, '[]')`, roleID, role); err != nil {
			e.t.Fatalf("insert role: %v", err)
		}
	} else if err != nil {
		e.t.Fatalf("find role: %v", err)
	}
	if _, err := e.db.Exec(`INSERT INTO "UserRole" (id, userId, roleId) VALUES (?, ?, ?)`,
		models.NewID(), id, roleID); err != nil {
		e.t.Fatalf("insert user role: %v", err)
	}
}

func (e *testEnv) insertCustomer(id, email string) {
	e.t.Helper()
	hash, _ := auth.HashPassword("Customer@12345")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := e.db.Exec(`INSERT INTO "Customer"
		(id, username, email, address, password, paymentMethods, preferences, active, deleted, createdAt, updatedAt)
		VALUES (?, ?, ?, '[]', ?, '[]', '{}', 1, 0, ?, ?)`, id, id, email, hash, now, now); err != nil {
		e.t.Fatalf("insert customer: %v", err)
	}
}

func (e *testEnv) insertProduct(id string, price float64, stock int) {
	e.t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := e.db.Exec(`INSERT INTO "Product"
		(id, name, pricesCalculatedPrice, stockQuantity, isActive, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, 1, ?, ?)`, id, id, price, stock, now, now); err != nil {
		e.t.Fatalf("insert product: %v", err)
	}
}

func (e *testEnv) rolesFor(userID string) []string {
	e.t.Helper()
	rows, err := e.db.Query(`SELECT r.role FROM "Role" r JOIN "UserRole" ur ON ur.roleId = r.id WHERE ur.userId = ?`, userID)
	if err != nil {
		e.t.Fatalf("roles: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		if rows.Scan(&r) == nil {
			out = append(out, r)
		}
	}
	return out
}

func TestProtectedRouteRequiresAuthentication(t *testing.T) {
	env := newTestEnv(t)
	rec, body := env.do(http.MethodPost, "/api/getloggedinuser", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if body["success"] != false {
		t.Fatalf("expected success false, got %v", body["success"])
	}
}

func TestLegacyAuthReturnsHTTP200(t *testing.T) {
	env := newTestEnvWith(t, func(cfg *config.Config) { cfg.AuthStatus = "legacy" })
	rec, body := env.do(http.MethodPost, "/api/getloggedinuser", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 in legacy mode, got %d", rec.Code)
	}
	if body["success"] != false {
		t.Fatalf("expected success false, got %v", body["success"])
	}
}

func TestLoginIssuesScopedTokenAndRole(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("admin-1", "admin@example.com", "Admin@12345", "admin")

	rec, body := env.do(http.MethodPost, "/api/login", "", map[string]any{
		"email":    "admin@example.com",
		"password": "Admin@12345",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", rec.Code, rec.Body.String())
	}
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatal("expected token")
	}
	claims, err := auth.ParseToken(jwtSecret, token)
	if err != nil {
		t.Fatalf("parse issued token: %v", err)
	}
	if claims.Kind != auth.KindUser || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestLoginRejectsWrongPasswordAndDisabledAccount(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("u1", "u1@example.com", "User@12345", "user")
	if _, body := env.do(http.MethodPost, "/api/login", "", map[string]any{
		"email": "u1@example.com", "password": "wrong-password",
	}); body["success"] != false {
		t.Fatal("expected wrong password to fail")
	}
	if _, err := env.db.Exec(`UPDATE "User" SET active = 0 WHERE id = 'u1'`); err != nil {
		t.Fatal(err)
	}
	if _, body := env.do(http.MethodPost, "/api/login", "", map[string]any{
		"email": "u1@example.com", "password": "User@12345",
	}); body["success"] != false {
		t.Fatal("expected disabled account to fail")
	}
}

func TestRegistrationCannotInjectRoleOrActiveFlag(t *testing.T) {
	env := newTestEnv(t)
	_, body := env.do(http.MethodPost, "/api/register", "", map[string]any{
		"name":            "Attacker",
		"email":           "attacker@example.com",
		"password":        "Password@123",
		"confirmPassword": "Password@123",
		"roles":           []string{"admin"},
		"active":          false,
	})
	if body["success"] != true {
		t.Fatalf("registration failed: %v", body)
	}
	var id string
	var active int
	if err := env.db.QueryRow(`SELECT id, active FROM "User" WHERE email = 'attacker@example.com'`).Scan(&id, &active); err != nil {
		t.Fatalf("find registered user: %v", err)
	}
	if active != 1 {
		t.Fatal("public registration must force active=true")
	}
	roles := env.rolesFor(id)
	if len(roles) != 1 || roles[0] != "user" {
		t.Fatalf("expected only the user role, got %v", roles)
	}
}

func TestRegistrationRejectsWeakOrMismatchedPassword(t *testing.T) {
	env := newTestEnv(t)
	if _, body := env.do(http.MethodPost, "/api/register", "", map[string]any{
		"name": "Weak", "email": "weak@example.com", "password": "short", "confirmPassword": "short",
	}); body["success"] != false {
		t.Fatal("expected short password to fail")
	}
	if _, body := env.do(http.MethodPost, "/api/register", "", map[string]any{
		"name": "Mismatch", "email": "mismatch@example.com", "password": "Password@123", "confirmPassword": "Password@124",
	}); body["success"] != false {
		t.Fatal("expected mismatched passwords to fail")
	}
}

func TestUserCannotEscalateOwnRole(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("u1", "u1@example.com", "User@12345", "user")
	token := env.userToken("u1", "user")

	rec, _ := env.do(http.MethodPost, "/api/updateuser", token, map[string]any{
		"id":    "u1",
		"roles": []string{"admin"},
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if roles := env.rolesFor("u1"); len(roles) != 1 || roles[0] != "user" {
		t.Fatalf("role must not change, got %v", roles)
	}
}

func TestManagerCannotGrantElevatedRoles(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("m1", "m1@example.com", "Manager@12345", "manager")
	env.insertUser("u1", "u1@example.com", "User@12345", "user")
	managerToken := env.userToken("m1", "manager")

	// A manager may create ordinary users, but the supplied admin role is
	// ignored because only an admin can assign roles.
	_, body := env.do(http.MethodPost, "/api/register", managerToken, map[string]any{
		"name":            "Promoted",
		"email":           "promoted@example.com",
		"password":        "Password@123",
		"confirmPassword": "Password@123",
		"roles":           []string{"admin"},
	})
	if body["success"] != true {
		t.Fatalf("manager registration failed: %v", body)
	}
	var id string
	if err := env.db.QueryRow(`SELECT id FROM "User" WHERE email = 'promoted@example.com'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if roles := env.rolesFor(id); len(roles) != 1 || roles[0] != "user" {
		t.Fatalf("manager must not assign admin at registration, got %v", roles)
	}

	// A manager cannot promote another user or change the active flag.
	rec, _ := env.do(http.MethodPost, "/api/updateuser", managerToken, map[string]any{
		"id": "u1", "roles": []string{"admin"},
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for manager role change, got %d", rec.Code)
	}
	rec, _ = env.do(http.MethodPost, "/api/updateuser", managerToken, map[string]any{
		"id": "u1", "active": false,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for manager active change, got %d", rec.Code)
	}
	if roles := env.rolesFor("u1"); len(roles) != 1 || roles[0] != "user" {
		t.Fatalf("roles must not change, got %v", roles)
	}
}

func TestAdminCanAssignRolesButNotToSelf(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	env.insertUser("u1", "u1@example.com", "User@12345", "user")
	adminToken := env.userToken("a1", "admin")

	rec, body := env.do(http.MethodPost, "/api/updateuser", adminToken, map[string]any{
		"id": "u1", "roles": []string{"manager"},
	})
	if rec.Code != http.StatusOK || body["success"] != true {
		t.Fatalf("admin role change failed: %d %s", rec.Code, rec.Body.String())
	}
	if roles := env.rolesFor("u1"); len(roles) != 1 || roles[0] != "manager" {
		t.Fatalf("expected manager role, got %v", roles)
	}

	rec, _ = env.do(http.MethodPost, "/api/updateuser", adminToken, map[string]any{
		"id": "a1", "roles": []string{"user"},
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected self role change to be forbidden, got %d", rec.Code)
	}
	rec, _ = env.do(http.MethodPost, "/api/updateuser", adminToken, map[string]any{
		"id": "a1", "active": false,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected self deactivation to be forbidden, got %d", rec.Code)
	}
}

func TestCustomerCannotReadAnotherCustomer(t *testing.T) {
	env := newTestEnv(t)
	env.insertCustomer("c1", "c1@example.com")
	env.insertCustomer("c2", "c2@example.com")

	rec, _ := env.do(http.MethodPost, "/customer/getspecificcustomer/c2", env.customerToken("c1"), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for other customer, got %d: %s", rec.Code, rec.Body.String())
	}
	rec, body := env.do(http.MethodPost, "/customer/getspecificcustomer/c1", env.customerToken("c1"), nil)
	if rec.Code != http.StatusOK || body["success"] != true {
		t.Fatalf("expected own customer to be readable, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCartOwnershipAndStockChecks(t *testing.T) {
	env := newTestEnv(t)
	env.insertCustomer("c1", "c1@example.com")
	env.insertCustomer("c2", "c2@example.com")
	env.insertProduct("p1", 100, 2)
	token := env.customerToken("c1")

	rec, _ := env.do(http.MethodPost, "/customer/addtocart/c2", token, map[string]any{"productId": "p1", "quantity": 1})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for other cart, got %d", rec.Code)
	}

	if _, body := env.do(http.MethodPost, "/customer/addtocart/c1", token, map[string]any{"productId": "p1", "quantity": 3}); body["success"] != false {
		t.Fatal("expected stock limit to reject quantity 3 of stock 2")
	}
	if _, body := env.do(http.MethodPost, "/customer/addtocart/c1", token, map[string]any{"productId": "p1", "quantity": 2}); body["success"] != true {
		t.Fatalf("expected valid add to cart: %v", body)
	}
	if _, body := env.do(http.MethodPost, "/customer/addtocart/c1", token, map[string]any{"productId": "p1", "quantity": 1}); body["success"] != false {
		t.Fatal("expected accumulated quantity to exceed stock")
	}
}

func TestOrderUsesServerTotalsAndOwnership(t *testing.T) {
	env := newTestEnv(t)
	env.insertCustomer("c1", "c1@example.com")
	env.insertCustomer("c2", "c2@example.com")
	env.insertProduct("p1", 100, 5)
	token := env.customerToken("c1")

	_, body := env.do(http.MethodPost, "/orders/addorder", token, map[string]any{
		"customerId":  "c2", // must be ignored
		"totalAmount": 1,    // must be ignored
		"status":      "delivered",
		"products":    []any{map[string]any{"product": "p1", "quantity": 2}},
	})
	if body["success"] != true {
		t.Fatalf("order failed: %v", body)
	}
	order, _ := body["order"].(map[string]any)
	orderID, _ := order["id"].(string)
	if orderID == "" {
		t.Fatal("expected order id")
	}
	var customerID string
	var total float64
	var status string
	if err := env.db.QueryRow(`SELECT customerId, totalAmount, status FROM "Order" WHERE id = ?`, orderID).
		Scan(&customerID, &total, &status); err != nil {
		t.Fatalf("read order: %v", err)
	}
	if customerID != "c1" {
		t.Fatalf("customer id must come from the token, got %s", customerID)
	}
	if total != 200 {
		t.Fatalf("total must be computed from the product price, got %v", total)
	}
	if status != "pending" {
		t.Fatalf("client status must be ignored, got %s", status)
	}

	// Another customer must not be able to read the order.
	rec, _ := env.do(http.MethodPost, "/orders/getspecificorder/"+orderID, env.customerToken("c2"), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for other customer's order, got %d", rec.Code)
	}
	// The owner can.
	rec, body = env.do(http.MethodPost, "/orders/getspecificorder/"+orderID, token, nil)
	if rec.Code != http.StatusOK || body["success"] != true {
		t.Fatalf("expected owner to read order, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOrderRejectsInvalidInputAndInsufficientStock(t *testing.T) {
	env := newTestEnv(t)
	env.insertCustomer("c1", "c1@example.com")
	env.insertProduct("p1", 100, 1)
	token := env.customerToken("c1")

	if _, body := env.do(http.MethodPost, "/orders/addorder", token, map[string]any{"products": []any{}}); body["success"] != false {
		t.Fatal("expected empty order to fail")
	}
	if _, body := env.do(http.MethodPost, "/orders/addorder", token, map[string]any{
		"products": []any{map[string]any{"product": "p1", "quantity": 0}},
	}); body["success"] != false {
		t.Fatal("expected zero quantity to fail")
	}
	if _, body := env.do(http.MethodPost, "/orders/addorder", token, map[string]any{
		"products": []any{map[string]any{"product": "p1", "quantity": 2}},
	}); body["success"] != false {
		t.Fatal("expected insufficient stock to fail")
	}
	var n int
	_ = env.db.QueryRow(`SELECT COUNT(1) FROM "Order"`).Scan(&n)
	if n != 0 {
		t.Fatalf("no order should be written, got %d", n)
	}
}

func TestCustomerOnlyListsOwnOrders(t *testing.T) {
	env := newTestEnv(t)
	env.insertCustomer("c1", "c1@example.com")
	env.insertCustomer("c2", "c2@example.com")
	env.insertProduct("p1", 100, 10)

	if _, body := env.do(http.MethodPost, "/orders/addorder", env.customerToken("c1"), map[string]any{
		"products": []any{map[string]any{"product": "p1", "quantity": 1}},
	}); body["success"] != true {
		t.Fatalf("c1 order failed: %v", body)
	}
	if _, body := env.do(http.MethodPost, "/orders/addorder", env.customerToken("c2"), map[string]any{
		"products": []any{map[string]any{"product": "p1", "quantity": 1}},
	}); body["success"] != true {
		t.Fatalf("c2 order failed: %v", body)
	}

	_, body := env.do(http.MethodPost, "/orders/getallorders", env.customerToken("c1"), nil)
	orders, _ := body["orders"].([]any)
	if len(orders) != 1 {
		t.Fatalf("expected one order for c1, got %d", len(orders))
	}
}

func TestProductAddRequiresRoleAndValidatesInput(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("u1", "u1@example.com", "User@12345", "user")
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")

	// A normal user cannot manage products.
	rec, _ := env.do(http.MethodPost, "/product/addproduct", env.userToken("u1", "user"), map[string]any{"name": "x"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for user role, got %d", rec.Code)
	}
	// An admin cannot add a product with a negative price.
	if _, body := env.do(http.MethodPost, "/product/addproduct", env.userToken("a1", "admin"), map[string]any{
		"name": "Bad", "pricesCalculatedPrice": -5,
	}); body["success"] != false {
		t.Fatal("expected negative price to fail")
	}
	// A valid product succeeds.
	if _, body := env.do(http.MethodPost, "/product/addproduct", env.userToken("a1", "admin"), map[string]any{
		"name": "Good", "pricesCalculatedPrice": 50, "stockQuantity": 3,
	}); body["success"] != true {
		t.Fatalf("expected valid product to be added: %v", body)
	}
}

func TestPaymentEndpointFailsClosedWithoutConfiguration(t *testing.T) {
	env := newTestEnv(t)
	env.insertCustomer("c1", "c1@example.com")
	env.insertProduct("p1", 100, 5)
	token := env.customerToken("c1")
	_, body := env.do(http.MethodPost, "/orders/addorder", token, map[string]any{
		"products": []any{map[string]any{"product": "p1", "quantity": 1}},
	})
	order, _ := body["order"].(map[string]any)
	orderID, _ := order["id"].(string)

	rec, _ := env.do(http.MethodPost, "/api/payment", token, map[string]any{
		"orderId": orderID, "providerReference": "ref", "signature": "sig",
	})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when payment is not configured, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPaymentEndpointVerifiesSignature(t *testing.T) {
	env := newTestEnv(t)
	env.cfg.PaymentProvider = "testpay"
	env.cfg.PaymentSecret = "integration-payment-secret"
	env.insertCustomer("c1", "c1@example.com")
	env.insertProduct("p1", 100, 5)
	token := env.customerToken("c1")
	_, body := env.do(http.MethodPost, "/orders/addorder", token, map[string]any{
		"products": []any{map[string]any{"product": "p1", "quantity": 2}},
	})
	order, _ := body["order"].(map[string]any)
	orderID, _ := order["id"].(string)

	// Invalid signature must not mark the order paid.
	rec, _ := env.do(http.MethodPost, "/api/payment", token, map[string]any{
		"orderId": orderID, "providerReference": "ref-1", "signature": "bad",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad signature, got %d", rec.Code)
	}

	ref := "ref-2"
	sig := service.SignPayment(env.cfg.PaymentSecret, orderID, ref, 200)
	rec, body = env.do(http.MethodPost, "/api/payment", token, map[string]any{
		"orderId": orderID, "providerReference": ref, "signature": sig,
	})
	if rec.Code != http.StatusOK || body["success"] != true {
		t.Fatalf("expected verified payment, got %d: %s", rec.Code, rec.Body.String())
	}
	var paymentStatus string
	if err := env.db.QueryRow(`SELECT paymentStatus FROM "Order" WHERE id = ?`, orderID).Scan(&paymentStatus); err != nil {
		t.Fatal(err)
	}
	if paymentStatus != "paid" {
		t.Fatalf("expected paid, got %s", paymentStatus)
	}
}

func TestAdminCanManageOrdersAndCancelRestocks(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	env.insertCustomer("c1", "c1@example.com")
	env.insertProduct("p1", 100, 5)
	customerToken := env.customerToken("c1")
	_, body := env.do(http.MethodPost, "/orders/addorder", customerToken, map[string]any{
		"products": []any{map[string]any{"product": "p1", "quantity": 2}},
	})
	order, _ := body["order"].(map[string]any)
	orderID, _ := order["id"].(string)
	adminToken := env.userToken("a1", "admin")

	rec, _ := env.do(http.MethodPost, "/orders/updateorder/"+orderID, adminToken, map[string]any{"status": "cancelled"})
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel failed: %d %s", rec.Code, rec.Body.String())
	}
	var stock sql.NullInt64
	if err := env.db.QueryRow(`SELECT stockQuantity FROM "Product" WHERE id = 'p1'`).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if int(stock.Int64) != 5 {
		t.Fatalf("expected restocked 5, got %d", stock.Int64)
	}

	// A plain staff user (role "user") lacks the manager/admin role.
	env.insertUser("u1", "u1@example.com", "User@12345", "user")
	rec, _ = env.do(http.MethodPost, "/orders/updateorder/"+orderID, env.userToken("u1", "user"), map[string]any{"status": "delivered"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for user role, got %d", rec.Code)
	}
	// A customer token is not a staff token at all.
	rec, _ = env.do(http.MethodPost, "/orders/updateorder/"+orderID, customerToken, map[string]any{"status": "delivered"})
	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
		t.Fatalf("expected 401/403 for customer update, got %d", rec.Code)
	}
}

func TestDeleteOrderSoftDeletes(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	env.insertCustomer("c1", "c1@example.com")
	env.insertProduct("p1", 100, 5)
	_, body := env.do(http.MethodPost, "/orders/addorder", env.customerToken("c1"), map[string]any{
		"products": []any{map[string]any{"product": "p1", "quantity": 1}},
	})
	order, _ := body["order"].(map[string]any)
	orderID, _ := order["id"].(string)

	rec, _ := env.do(http.MethodPost, "/orders/deleteorder/"+orderID, env.userToken("a1", "admin"), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", rec.Code, rec.Body.String())
	}
	var deletedAt sql.NullString
	if err := env.db.QueryRow(`SELECT deletedAt FROM "Order" WHERE id = ?`, orderID).Scan(&deletedAt); err != nil {
		t.Fatal(err)
	}
	if !deletedAt.Valid {
		t.Fatal("expected soft delete timestamp")
	}
	_, body = env.do(http.MethodPost, "/orders/getspecificorder/"+orderID, env.userToken("a1", "admin"), nil)
	if body["success"] != false {
		t.Fatal("soft-deleted order must not be readable")
	}
}

func TestHealthAndReady(t *testing.T) {
	env := newTestEnv(t)
	rec, body := env.do(http.MethodGet, "/healthz", "", nil)
	if rec.Code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("health failed: %d %s", rec.Code, rec.Body.String())
	}
	rec, body = env.do(http.MethodGet, "/readyz", "", nil)
	if rec.Code != http.StatusOK || body["status"] != "ready" {
		t.Fatalf("ready failed: %d %s", rec.Code, rec.Body.String())
	}
}

func TestUnexpectedErrorsDoNotLeakInternals(t *testing.T) {
	env := newTestEnv(t)
	env.insertCustomer("c1", "c1@example.com")
	token := env.customerToken("c1")
	// Closing the database forces a handler query to fail. Customer routes do
	// not need to load staff roles, so the request reaches the handler.
	if err := env.db.Close(); err != nil {
		t.Fatal(err)
	}
	rec, body := env.do(http.MethodPost, "/orders/getallorders", token, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
	msg, _ := body["msg"].(string)
	if msg != "Internal Server Error" {
		t.Fatalf("expected generic error message, got %q", msg)
	}
	if _, leaked := body["error"]; leaked {
		t.Fatalf("internal error leaked: %v", body["error"])
	}
}

func TestUnknownRouteReturns404(t *testing.T) {
	env := newTestEnv(t)
	rec, _ := env.do(http.MethodPost, "/api/does-not-exist", "", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestOversellingIsPreventedThroughHTTP(t *testing.T) {
	env := newTestEnv(t)
	env.insertCustomer("c1", "c1@example.com")
	env.insertProduct("p1", 100, 1)
	token := env.customerToken("c1")

	results := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, body := env.do(http.MethodPost, "/orders/addorder", token, map[string]any{
				"products": []any{map[string]any{"product": "p1", "quantity": 1}},
			})
			results <- body["success"] == true
		}()
	}
	successes := 0
	for i := 0; i < 2; i++ {
		if <-results {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one successful order, got %d", successes)
	}
	var n int
	if err := env.db.QueryRow(`SELECT COUNT(1) FROM "Order"`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected one order, got %d", n)
	}
}

func TestStorefrontRoutesCoexistWithUploadsAndGlobalCSRF(t *testing.T) {
	env := newTestEnv(t)
	env.insertProduct("p1", 100, 5)
	if _, err := env.db.Exec(`UPDATE "Product" SET slug = 'silver-ring', name = 'Silver Ring' WHERE id = 'p1'`); err != nil {
		t.Fatal(err)
	}

	var sessionCookie, csrfCookie *http.Cookie
	bootstrap := httptest.NewRequest(http.MethodGet, "/", nil)
	bootstrapRec := httptest.NewRecorder()
	env.handler.ServeHTTP(bootstrapRec, bootstrap)
	if bootstrapRec.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d", bootstrapRec.Code)
	}
	if !strings.Contains(bootstrapRec.Body.String(), "<title>Home · Pusti Shangar</title>") {
		t.Fatal("storefront was not rendered by the global router")
	}
	for _, cookie := range bootstrapRec.Result().Cookies() {
		switch cookie.Name {
		case "storefront_session":
			sessionCookie = cookie
		case "csrf_token":
			csrfCookie = cookie
		}
	}
	if sessionCookie == nil || csrfCookie == nil {
		t.Fatal("storefront bootstrap did not issue session and CSRF cookies")
	}

	// The outer /products/* upload alias must not shadow the Templ detail
	// route registered afterwards by web.RegisterWebRoutes.
	detail := httptest.NewRequest(http.MethodGet, "/products/silver-ring", nil)
	detailRec := httptest.NewRecorder()
	env.handler.ServeHTTP(detailRec, detail)
	if detailRec.Code != http.StatusOK || !strings.Contains(detailRec.Body.String(), "Silver Ring") {
		t.Fatalf("detail status = %d, body = %q", detailRec.Code, detailRec.Body.String())
	}

	imageDir := filepath.Join(env.cfg.UploadsDir, "Products")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imageDir, "silver-ring.jpg"), []byte("image-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	image := httptest.NewRequest(http.MethodGet, "/products/silver-ring.jpg", nil)
	imageRec := httptest.NewRecorder()
	env.handler.ServeHTTP(imageRec, image)
	if imageRec.Code != http.StatusOK || imageRec.Body.String() != "image-bytes" {
		t.Fatalf("image status = %d, body = %q", imageRec.Code, imageRec.Body.String())
	}

	// No token cookie exists, so the global CSRF middleware skips this guest
	// request; the storefront double-submit check still has to accept it.
	form := url.Values{"productId": {"p1"}, "quantity": {"2"}}
	add := httptest.NewRequest(http.MethodPost, "/cart/items", strings.NewReader(form.Encode()))
	add.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	add.Header.Set("HX-Request", "true")
	add.Header.Set("X-CSRF-Token", csrfCookie.Value)
	add.AddCookie(sessionCookie)
	add.AddCookie(csrfCookie)
	addRec := httptest.NewRecorder()
	env.handler.ServeHTTP(addRec, add)
	if addRec.Code != http.StatusOK {
		t.Fatalf("cart add status = %d, body = %q", addRec.Code, addRec.Body.String())
	}
	if !strings.Contains(addRec.Body.String(), "Added") || !strings.Contains(addRec.Body.String(), "₹200.00") {
		t.Fatalf("unexpected cart fragment: %q", addRec.Body.String())
	}
}
