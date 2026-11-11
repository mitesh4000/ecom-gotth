package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mitesh4000/pushti_sangar/server/internal/config"
)

// doWith sends a request with optional cookies and headers. It exists so the
// CSRF and idempotency tests can exercise the cookie-authenticated path.
func (e *testEnv) doWith(method, path string, body any, headers map[string]string, cookies ...*http.Cookie) (*httptest.ResponseRecorder, map[string]any) {
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
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec, out
}

func TestCanonicalReadRoutesAcceptGET(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	token := env.userToken("a1", "admin")

	public := []string{
		"/product/getallproducts",
		"/category/getcategories",
		"/subcategory/getsubcategories",
		"/subsubcategory/getsubsubcategories",
		"/customer/loggedin",
		"/api/loggedin",
	}
	for _, path := range public {
		rec, _ := env.do(http.MethodGet, path, "", nil)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: expected 200, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}

	protected := []string{
		"/api/getloggedinuser",
		"/api/getusers",
		"/api/getroles",
		"/api/getspecificuser/a1",
		"/product/get-products-for-table",
		"/orders/getallorders",
		"/dashboard/getdashboard",
	}
	for _, path := range protected {
		rec, _ := env.do(http.MethodGet, path, token, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s: expected 200, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestGETReadsQueryParameters(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	token := env.userToken("a1", "admin")

	// Pagination used to arrive in the POST body; on GET it must come from the
	// query string.
	rec, body := env.do(http.MethodGet, "/orders/getallorders?limit=5&offset=0", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET orders: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, ok := body["orders"]; !ok {
		t.Fatalf("expected orders field, got %v", body)
	}
}

func TestUnsupportedMethodReturns405(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	token := env.userToken("a1", "admin")

	cases := []struct{ method, path string }{
		{http.MethodPatch, "/api/loggedin"},
		{http.MethodPut, "/category/getcategories"},
		{http.MethodDelete, "/api/login"},
	}
	for _, c := range cases {
		rec, _ := env.do(c.method, c.path, token, nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s %s: expected 405, got %d: %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
}

func TestGETDoesNotMutateState(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	env.insertProduct("p1", 100, 5)
	token := env.userToken("a1", "admin")

	var usersBefore, ordersBefore int
	if err := env.db.QueryRow(`SELECT COUNT(1) FROM "User"`).Scan(&usersBefore); err != nil {
		t.Fatal(err)
	}
	if err := env.db.QueryRow(`SELECT COUNT(1) FROM "Order"`).Scan(&ordersBefore); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/api/getusers", "/orders/getallorders", "/product/getallproducts"} {
		rec, _ := env.do(http.MethodGet, path, token, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s failed: %d %s", path, rec.Code, rec.Body.String())
		}
	}

	var usersAfter, ordersAfter int
	if err := env.db.QueryRow(`SELECT COUNT(1) FROM "User"`).Scan(&usersAfter); err != nil {
		t.Fatal(err)
	}
	if err := env.db.QueryRow(`SELECT COUNT(1) FROM "Order"`).Scan(&ordersAfter); err != nil {
		t.Fatal(err)
	}
	if usersBefore != usersAfter || ordersBefore != ordersAfter {
		t.Fatalf("GET changed state: users %d->%d orders %d->%d", usersBefore, usersAfter, ordersBefore, ordersAfter)
	}
}

func TestPATCHPartiallyUpdatesProduct(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	env.insertProduct("p1", 100, 5)
	token := env.userToken("a1", "admin")

	rec, body := env.do(http.MethodPatch, "/product/updateproduct/p1", token, map[string]any{
		"description": "patched description",
	})
	if rec.Code != http.StatusOK || body["success"] != true {
		t.Fatalf("PATCH failed: %d %s", rec.Code, rec.Body.String())
	}

	var name, description string
	var price float64
	var stock int
	if err := env.db.QueryRow(`SELECT name, description, pricesCalculatedPrice, stockQuantity FROM "Product" WHERE id = 'p1'`).
		Scan(&name, &description, &price, &stock); err != nil {
		t.Fatal(err)
	}
	if name != "p1" || price != 100 || stock != 5 {
		t.Fatalf("PATCH must leave omitted fields untouched, got name=%q price=%v stock=%d", name, price, stock)
	}
	if description != "patched description" {
		t.Fatalf("expected description to change, got %q", description)
	}
}

func TestPUTReplacesOmittedProductFields(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	env.insertProduct("p1", 100, 5)
	token := env.userToken("a1", "admin")

	rec, body := env.do(http.MethodPut, "/product/updateproduct/p1", token, map[string]any{
		"name": "replaced",
	})
	if rec.Code != http.StatusOK || body["success"] != true {
		t.Fatalf("PUT failed: %d %s", rec.Code, rec.Body.String())
	}

	var name string
	var price, stock interface{}
	if err := env.db.QueryRow(`SELECT name, pricesCalculatedPrice, stockQuantity FROM "Product" WHERE id = 'p1'`).
		Scan(&name, &price, &stock); err != nil {
		t.Fatal(err)
	}
	if name != "replaced" {
		t.Fatalf("expected replaced name, got %q", name)
	}
	if price != nil || stock != nil {
		t.Fatalf("PUT must clear omitted fields, got price=%v stock=%v", price, stock)
	}
}

func TestPUTRequiresCompleteRepresentation(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	env.insertProduct("p1", 100, 5)
	token := env.userToken("a1", "admin")

	rec, body := env.do(http.MethodPut, "/product/updateproduct/p1", token, map[string]any{
		"description": "missing name",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	if body["success"] != false {
		t.Fatal("PUT without the required name must be rejected")
	}

	var name string
	if err := env.db.QueryRow(`SELECT name FROM "Product" WHERE id = 'p1'`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "p1" {
		t.Fatalf("rejected PUT must not modify the row, got %q", name)
	}
}

func TestDELETESoftDeletesProduct(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	env.insertProduct("p1", 100, 5)
	token := env.userToken("a1", "admin")

	rec, body := env.do(http.MethodDelete, "/product/deleteproduct/p1", token, nil)
	if rec.Code != http.StatusOK || body["success"] != true {
		t.Fatalf("DELETE failed: %d %s", rec.Code, rec.Body.String())
	}

	var active int
	if err := env.db.QueryRow(`SELECT isActive FROM "Product" WHERE id = 'p1'`).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("expected soft-deleted product, isActive=%d", active)
	}
}

func TestDELETEWithQueryID(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	env.insertCustomer("c1", "c1@example.com")
	token := env.userToken("a1", "admin")

	rec, body := env.do(http.MethodDelete, "/customer/deletecustomer?id=c1", token, nil)
	if rec.Code != http.StatusOK || body["success"] != true {
		t.Fatalf("DELETE by query failed: %d %s", rec.Code, rec.Body.String())
	}
	var deleted int
	if err := env.db.QueryRow(`SELECT deleted FROM "Customer" WHERE id = 'c1'`).Scan(&deleted); err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("expected customer to be soft deleted, got %d", deleted)
	}
}

func TestNewMethodsKeepAuthorization(t *testing.T) {
	env := newTestEnv(t)
	env.insertCustomer("c1", "c1@example.com")
	env.insertProduct("p1", 100, 5)

	// Reads that require a staff token must still reject anonymous callers.
	rec, _ := env.do(http.MethodGet, "/product/get-products-for-table", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous GET, got %d", rec.Code)
	}

	// A customer token must not be able to delete catalogue data.
	rec, _ = env.do(http.MethodDelete, "/product/deleteproduct/p1", env.customerToken("c1"), nil)
	if rec.Code < 400 {
		t.Fatalf("expected customer DELETE to be rejected, got %d", rec.Code)
	}
	var active int
	if err := env.db.QueryRow(`SELECT isActive FROM "Product" WHERE id = 'p1'`).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 1 {
		t.Fatal("rejected DELETE must not modify the product")
	}
}

func TestLegacyPOSTEndpointsStillWork(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	env.insertProduct("p1", 100, 5)
	token := env.userToken("a1", "admin")

	// Reads, updates and deletes keep their original POST form.
	if rec, _ := env.do(http.MethodPost, "/api/getloggedinuser", token, nil); rec.Code != http.StatusOK {
		t.Fatalf("legacy read failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec, body := env.do(http.MethodPost, "/product/updateproduct/p1", token, map[string]any{
		"description": "legacy update",
	}); rec.Code != http.StatusOK || body["success"] != true {
		t.Fatalf("legacy update failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec, body := env.do(http.MethodPost, "/customer/deletecustomer", token, map[string]any{
		"id": "missing",
	}); rec.Code != http.StatusOK || body["success"] != false {
		t.Fatalf("legacy delete should still accept a body id: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCheckoutIdempotencyKeyPreventsDuplicateOrders(t *testing.T) {
	env := newTestEnv(t)
	env.insertCustomer("c1", "c1@example.com")
	env.insertProduct("p1", 100, 10)
	token := env.customerToken("c1")

	body := map[string]any{
		"products": []any{map[string]any{"product": "p1", "quantity": 2}},
	}
	headers := map[string]string{"Idempotency-Key": "checkout-1", "Authorization": "Bearer " + token}

	_, first := env.doWith(http.MethodPost, "/orders/addorder", body, headers)
	firstOrder, _ := first["order"].(map[string]any)
	firstID, _ := firstOrder["id"].(string)
	if firstID == "" {
		t.Fatalf("first checkout failed: %v", first)
	}

	_, second := env.doWith(http.MethodPost, "/orders/addorder", body, headers)
	secondOrder, _ := second["order"].(map[string]any)
	secondID, _ := secondOrder["id"].(string)
	if secondID != firstID {
		t.Fatalf("retry must return the same order, got %s and %s", firstID, secondID)
	}

	var orders int
	if err := env.db.QueryRow(`SELECT COUNT(1) FROM "Order"`).Scan(&orders); err != nil {
		t.Fatal(err)
	}
	if orders != 1 {
		t.Fatalf("expected one order, got %d", orders)
	}
	var stock int
	if err := env.db.QueryRow(`SELECT stockQuantity FROM "Product" WHERE id = 'p1'`).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if stock != 8 {
		t.Fatalf("stock must only be decremented once, got %d", stock)
	}
}

func TestCSRFStrictRequiresDoubleSubmitToken(t *testing.T) {
	env := newTestEnvWith(t, func(cfg *config.Config) { cfg.CSRFMode = "strict" })
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	env.insertUser("u1", "u1@example.com", "User@12345", "user")
	token := env.userToken("a1", "admin")
	cookies := []*http.Cookie{{Name: "token", Value: token}}

	// A cookie-authenticated unsafe request without the header is rejected.
	rec, _ := env.doWith(http.MethodPatch, "/api/updateuser", map[string]any{
		"id": "u1", "name": "changed",
	}, nil, cookies...)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without CSRF token, got %d: %s", rec.Code, rec.Body.String())
	}

	// With a matching double-submit token it succeeds.
	csrf := []*http.Cookie{
		{Name: "token", Value: token},
		{Name: "csrf_token", Value: "csrf-value"},
	}
	rec, body := env.doWith(http.MethodPatch, "/api/updateuser", map[string]any{
		"id": "u1", "name": "changed",
	}, map[string]string{"X-CSRF-Token": "csrf-value"}, csrf...)
	if rec.Code != http.StatusOK || body["success"] != true {
		t.Fatalf("expected CSRF-authenticated PATCH to succeed, got %d: %s", rec.Code, rec.Body.String())
	}

	// Bearer-token requests are not vulnerable to CSRF and are exempt.
	rec, body = env.do(http.MethodPatch, "/api/updateuser", token, map[string]any{
		"id": "u1", "name": "bearer",
	})
	if rec.Code != http.StatusOK || body["success"] != true {
		t.Fatalf("expected bearer PATCH to bypass CSRF, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCSRFSameOriginRejectsCrossSiteRequests(t *testing.T) {
	env := newTestEnv(t)
	env.insertUser("a1", "a1@example.com", "Admin@12345", "admin")
	env.insertUser("u1", "u1@example.com", "User@12345", "user")
	token := env.userToken("a1", "admin")
	cookies := []*http.Cookie{{Name: "token", Value: token}}

	rec, _ := env.doWith(http.MethodPatch, "/api/updateuser", map[string]any{
		"id": "u1", "name": "evil",
	}, map[string]string{"Origin": "https://evil.example"}, cookies...)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected cross-site request to be rejected, got %d: %s", rec.Code, rec.Body.String())
	}

	// An allowed frontend still needs the double-submit token.
	rec, _ = env.doWith(http.MethodPatch, "/api/updateuser", map[string]any{
		"id": "u1", "name": "allowed",
	}, map[string]string{"Origin": "http://localhost:3000"}, cookies...)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected allowed origin without CSRF token to be rejected, got %d", rec.Code)
	}

	// Same-origin requests with a cookie session are accepted.
	rec, body := env.doWith(http.MethodPatch, "/api/updateuser", map[string]any{
		"id": "u1", "name": "same-origin",
	}, nil, cookies...)
	if rec.Code != http.StatusOK || body["success"] != true {
		t.Fatalf("expected same-origin cookie PATCH to succeed, got %d: %s", rec.Code, rec.Body.String())
	}
}
