package web_test

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/auth"
	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/database"
	"github.com/mitesh4000/pushti_sangar/server/internal/web"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/session"
	"github.com/mitesh4000/pushti_sangar/server/migrations"
)

const storefrontJWTSecret = "storefront-test-secret-1234567890"

type storefrontEnv struct {
	t       *testing.T
	db      *sql.DB
	cfg     *config.Config
	handler http.Handler
}

func newStorefrontEnv(t *testing.T) *storefrontEnv {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "storefront.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Apply(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{
		Env:          "test",
		JWTSecret:    storefrontJWTSecret,
		UploadsDir:   t.TempDir(),
		CookieSecure: false,
		TokenTTL:     time.Hour,
	}
	router := chi.NewRouter()
	web.RegisterWebRoutes(router, web.WebDependencies{DB: db, Cfg: cfg})
	return &storefrontEnv{t: t, db: db, cfg: cfg, handler: router}
}

func (e *storefrontEnv) client() *storefrontClient {
	return &storefrontClient{t: e.t, handler: e.handler, cookies: map[string]string{}}
}

func (e *storefrontEnv) insertCategory(id, name string) {
	e.t.Helper()
	if _, err := e.db.Exec(`INSERT INTO "Category" (id, name, isActive, isDeleted) VALUES (?, ?, 1, 0)`, id, name); err != nil {
		e.t.Fatalf("insert category: %v", err)
	}
}

type storefrontProduct struct {
	ID         string
	Slug       string
	Name       string
	CategoryID string
	Price      float64
	Original   float64
	Stock      int
	Shipping   float64
	Popular    bool
	New        bool
	Active     bool
	Image      string
}

func (e *storefrontEnv) insertProduct(p storefrontProduct) {
	e.t.Helper()
	popular, isNew, active := 0, 0, 0
	if p.Popular {
		popular = 1
	}
	if p.New {
		isNew = 1
	}
	if p.Active {
		active = 1
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	gallery := "[]"
	if p.Image != "" {
		gallery = fmt.Sprintf(`[%q]`, p.Image)
	}
	var original any
	if p.Original > 0 {
		original = p.Original
	}
	if _, err := e.db.Exec(`INSERT INTO "Product"
		(id, slug, name, description, categoryId, tags, pricesOriginal, pricesCalculatedPrice,
		 imageGallery, stockQuantity, shippingCharge, isActive, isProductPopular, isProductNew,
		 createdAt, updatedAt)
		VALUES (?, NULLIF(?, ''), ?, 'A test product description.', NULLIF(?, ''), '[]', ?, ?,
		 ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Slug, p.Name, p.CategoryID, original, p.Price, gallery, p.Stock, p.Shipping,
		active, popular, isNew, now, now); err != nil {
		e.t.Fatalf("insert product: %v", err)
	}
}

func (e *storefrontEnv) insertCustomer(id, email string) {
	e.t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := e.db.Exec(`INSERT INTO "Customer"
		(id, username, email, address, password, paymentMethods, preferences, active, deleted, createdAt, updatedAt)
		VALUES (?, ?, ?, '[]', 'x', '[]', '{}', 1, 0, ?, ?)`, id, id, email, now, now); err != nil {
		e.t.Fatalf("insert customer: %v", err)
	}
}

type storefrontClient struct {
	t       *testing.T
	handler http.Handler
	cookies map[string]string
}

func (c *storefrontClient) setCookie(name, value string) {
	c.cookies[name] = value
}

func (c *storefrontClient) cookie(name string) string {
	return c.cookies[name]
}

func (c *storefrontClient) request(method, path string, form url.Values, headers map[string]string) *httptest.ResponseRecorder {
	c.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	for name, value := range c.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	for _, cookie := range rec.Result().Cookies() {
		if cookie.MaxAge < 0 {
			delete(c.cookies, cookie.Name)
			continue
		}
		c.cookies[cookie.Name] = cookie.Value
	}
	return rec
}

func (c *storefrontClient) get(path string) *httptest.ResponseRecorder {
	return c.request(http.MethodGet, path, nil, nil)
}

func (c *storefrontClient) getHTMX(path string) *httptest.ResponseRecorder {
	return c.request(http.MethodGet, path, nil, map[string]string{"HX-Request": "true"})
}

func (c *storefrontClient) mutate(method, path string, form url.Values, htmx bool) *httptest.ResponseRecorder {
	headers := map[string]string{"X-CSRF-Token": c.cookie(session.CSRFCookieName)}
	if htmx {
		headers["HX-Request"] = "true"
	}
	return c.request(method, path, form, headers)
}

func (c *storefrontClient) bootstrap() {
	c.t.Helper()
	rec := c.get("/")
	if rec.Code != http.StatusOK {
		c.t.Fatalf("bootstrap status = %d", rec.Code)
	}
	if c.cookie(session.GuestCookieName) == "" {
		c.t.Fatal("guest session cookie was not issued")
	}
	if c.cookie(session.CSRFCookieName) == "" {
		c.t.Fatal("csrf cookie was not issued")
	}
}

func assertContains(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("response is missing %q", want)
		}
	}
}

func assertNotContains(t *testing.T, body string, unwanted ...string) {
	t.Helper()
	for _, value := range unwanted {
		if strings.Contains(body, value) {
			t.Errorf("response should not contain %q", value)
		}
	}
}

func TestStorefrontPagesRenderRealCatalogueData(t *testing.T) {
	env := newStorefrontEnv(t)
	env.insertCategory("cat-rings", "Rings")
	env.insertProduct(storefrontProduct{
		ID: "p1", Slug: "silver-ring", Name: "Silver Ring", CategoryID: "cat-rings",
		Price: 100, Original: 150, Stock: 5, Shipping: 10, Popular: true, New: true, Active: true,
		Image: "/uploads/Products/silver-ring.jpg",
	})
	client := env.client()

	home := client.get("/")
	if home.Code != http.StatusOK {
		t.Fatalf("home status = %d", home.Code)
	}
	homeHTML := home.Body.String()
	assertContains(t, homeHTML, "<!doctype html>", "<title>Home · Pusti Shangar</title>",
		"Silver Ring", "Rings", "₹100.00", "Popular right now", "New arrivals", `name="csrf-token"`)
	assertNotContains(t, homeHTML, "<div id=\"server-message\"")

	listing := client.get("/products")
	if listing.Code != http.StatusOK {
		t.Fatalf("listing status = %d", listing.Code)
	}
	listingHTML := listing.Body.String()
	assertContains(t, listingHTML, "<title>Shop · Pusti Shangar</title>", "Shop all",
		"Silver Ring", `name="q"`, `name="category"`, `name="sort"`, `id="product-results"`)

	detail := client.get("/products/silver-ring")
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status = %d", detail.Code)
	}
	detailHTML := detail.Body.String()
	assertContains(t, detailHTML, "<title>Silver Ring · Pusti Shangar</title>",
		"₹100.00", "₹150.00", "Only 5 left", "Add to cart", `name="productId" value="p1"`)

	byID := client.get("/products/p1")
	if byID.Code != http.StatusOK || !strings.Contains(byID.Body.String(), "Silver Ring") {
		t.Fatalf("id detail status = %d", byID.Code)
	}

	cart := client.get("/cart")
	if cart.Code != http.StatusOK {
		t.Fatalf("cart status = %d", cart.Code)
	}
	assertContains(t, cart.Body.String(), "<title>Cart · Pusti Shangar</title>", "Your cart is empty",
		"Stored on the server")

	missing := client.get("/products/does-not-exist")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing product status = %d, want 404", missing.Code)
	}
	assertContains(t, missing.Body.String(), "404 Not Found", "could not be found", "<!doctype html>")

	placeholder := client.get("/static/images/product-placeholder.svg")
	if placeholder.Code != http.StatusOK || !strings.Contains(placeholder.Body.String(), "<svg") {
		t.Fatalf("placeholder status = %d", placeholder.Code)
	}
}

func TestProductsHTMXFragmentAppliesFiltersSortAndPagination(t *testing.T) {
	env := newStorefrontEnv(t)
	env.insertCategory("cat-a", "Alpha collection")
	env.insertCategory("cat-b", "Beta collection")
	env.insertProduct(storefrontProduct{ID: "p1", Slug: "alpha-one", Name: "Alpha One", CategoryID: "cat-a", Price: 300, Stock: 10, Active: true})
	env.insertProduct(storefrontProduct{ID: "p2", Slug: "alpha-two", Name: "Alpha Two", CategoryID: "cat-a", Price: 100, Stock: 10, Active: true})
	env.insertProduct(storefrontProduct{ID: "p3", Slug: "beta-one", Name: "Beta One", CategoryID: "cat-b", Price: 200, Stock: 10, Active: true})
	client := env.client()

	fragment := client.getHTMX("/products?q=alpha&sort=price-asc")
	if fragment.Code != http.StatusOK {
		t.Fatalf("fragment status = %d", fragment.Code)
	}
	fragmentHTML := fragment.Body.String()
	assertNotContains(t, fragmentHTML, "<!doctype html>", "<title>")
	assertContains(t, fragmentHTML, "Alpha Two", "Alpha One")
	assertNotContains(t, fragmentHTML, "Beta One")
	if strings.Index(fragmentHTML, "Alpha Two") > strings.Index(fragmentHTML, "Alpha One") {
		t.Fatal("price-asc sort was not applied by the server")
	}

	category := client.getHTMX("/products?category=cat-b")
	assertContains(t, category.Body.String(), "Beta One")
	assertNotContains(t, category.Body.String(), "Alpha One")

	page := client.getHTMX("/products?sort=price-asc&limit=1&page=2")
	if page.Code != http.StatusOK {
		t.Fatalf("pagination status = %d", page.Code)
	}
	pageHTML := page.Body.String()
	assertContains(t, pageHTML, "Page 2 of 3", "Beta One", `hx-get="/products?limit=1&amp;sort=price-asc`)

	empty := client.getHTMX("/products?q=zzzz")
	assertContains(t, empty.Body.String(), "No products match those filters", "Clear filters")
}

func TestGuestCartHTMLLifecycleAndServerTotals(t *testing.T) {
	env := newStorefrontEnv(t)
	env.insertProduct(storefrontProduct{
		ID: "p1", Slug: "silver-ring", Name: "Silver Ring", Price: 100, Original: 150,
		Stock: 5, Shipping: 10, Active: true,
	})
	client := env.client()
	client.bootstrap()

	added := client.mutate(http.MethodPost, "/cart/items", url.Values{
		"productId": {"p1"},
		"quantity":  {"2"},
	}, true)
	if added.Code != http.StatusOK {
		t.Fatalf("add status = %d: %s", added.Code, added.Body.String())
	}
	addedHTML := added.Body.String()
	assertContains(t, addedHTML, "Added", "Silver Ring", `id="cart-badge"`, `hx-swap-oob="outerHTML"`,
		`id="cart-items"`, `id="cart-summary"`, "₹200.00", "₹10.00", "₹210.00", `value="2"`)
	assertNotContains(t, addedHTML, "<!doctype html>")

	var quantity int
	var storedKey string
	if err := env.db.QueryRow(`SELECT quantity, sessionId FROM "GuestCartItem" WHERE productId = 'p1'`).
		Scan(&quantity, &storedKey); err != nil {
		t.Fatalf("read guest cart: %v", err)
	}
	if quantity != 2 {
		t.Fatalf("guest cart quantity = %d", quantity)
	}
	if storedKey != session.HashToken(client.cookie(session.GuestCookieName)) {
		t.Fatal("guest cart must be keyed by a hash of the opaque cookie")
	}

	cart := client.get("/cart")
	if cart.Code != http.StatusOK {
		t.Fatalf("cart status = %d", cart.Code)
	}
	assertContains(t, cart.Body.String(), "Silver Ring", "₹200.00", "₹210.00", `value="2"`, "2 items")

	// The browser cannot send a price. Changing the catalogue re-prices the
	// existing line on the next request.
	if _, err := env.db.Exec(`UPDATE "Product" SET pricesCalculatedPrice = 125 WHERE id = 'p1'`); err != nil {
		t.Fatal(err)
	}
	repriced := client.get("/cart")
	assertContains(t, repriced.Body.String(), "₹250.00", "₹260.00")
	assertNotContains(t, repriced.Body.String(), "₹200.00")

	updated := client.mutate(http.MethodPatch, "/cart/items/p1", url.Values{"quantity": {"3"}}, true)
	if updated.Code != http.StatusOK {
		t.Fatalf("patch status = %d: %s", updated.Code, updated.Body.String())
	}
	assertContains(t, updated.Body.String(), "Quantity updated", `value="3"`, "₹375.00")

	replaced := client.mutate(http.MethodPut, "/cart/items/p1", url.Values{"quantity": {"1"}}, true)
	if replaced.Code != http.StatusOK {
		t.Fatalf("put status = %d: %s", replaced.Code, replaced.Body.String())
	}
	assertContains(t, replaced.Body.String(), `value="1"`, "₹125.00")

	removed := client.mutate(http.MethodDelete, "/cart/items/p1", nil, true)
	if removed.Code != http.StatusOK {
		t.Fatalf("delete status = %d: %s", removed.Code, removed.Body.String())
	}
	assertContains(t, removed.Body.String(), "Item removed", "Your cart is empty", "₹0.00")
	if quantity, _ := env.cartQuantity("p1"); quantity != 0 {
		t.Fatalf("guest cart row was not removed, quantity=%d", quantity)
	}
}

func (e *storefrontEnv) cartQuantity(productID string) (int, error) {
	var quantity sql.NullInt64
	err := e.db.QueryRow(`SELECT SUM(quantity) FROM "GuestCartItem" WHERE productId = ?`, productID).Scan(&quantity)
	return int(quantity.Int64), err
}

func TestCartRequiresCSRFAndHXRequestIsNotAuthentication(t *testing.T) {
	env := newStorefrontEnv(t)
	env.insertProduct(storefrontProduct{ID: "p1", Name: "Ring", Price: 100, Stock: 5, Active: true})
	client := env.client()
	client.bootstrap()

	noToken := client.request(http.MethodPost, "/cart/items", url.Values{
		"productId": {"p1"}, "quantity": {"1"},
	}, map[string]string{"HX-Request": "true"})
	if noToken.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status = %d, want 403", noToken.Code)
	}

	wrongToken := client.request(http.MethodPost, "/cart/items", url.Values{
		"productId": {"p1"}, "quantity": {"1"},
	}, map[string]string{"HX-Request": "true", "X-CSRF-Token": "wrong-token"})
	if wrongToken.Code != http.StatusForbidden {
		t.Fatalf("wrong csrf status = %d, want 403", wrongToken.Code)
	}

	// A normal browser form can submit the same double-submit token in the
	// body. It is a redirect, not an HTMX fragment.
	formToken := client.cookie(session.CSRFCookieName)
	formSubmit := client.request(http.MethodPost, "/cart/items", url.Values{
		"productId": {"p1"}, "quantity": {"1"}, "csrf_token": {formToken},
	}, nil)
	if formSubmit.Code != http.StatusSeeOther {
		t.Fatalf("form status = %d, want 303: %s", formSubmit.Code, formSubmit.Body.String())
	}
	if got := formSubmit.Header().Get("Location"); got != "/cart" {
		t.Fatalf("redirect location = %q", got)
	}

	// A client can set HX-Request on any request. It only selects rendering,
	// so it cannot be treated as authentication or as a CSRF substitute.
	page := client.getHTMX("/cart")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "<!doctype html>") {
		t.Fatalf("cart page status = %d; HX-Request must not bypass the page renderer", page.Code)
	}
}

func TestCartRejectsInvalidQuantityAndStock(t *testing.T) {
	env := newStorefrontEnv(t)
	env.insertProduct(storefrontProduct{ID: "p1", Name: "Limited", Price: 100, Stock: 2, Active: true})
	client := env.client()
	client.bootstrap()

	tooMany := client.mutate(http.MethodPost, "/cart/items", url.Values{"productId": {"p1"}, "quantity": {"3"}}, true)
	if tooMany.Code != http.StatusConflict {
		t.Fatalf("stock status = %d, want 409", tooMany.Code)
	}
	assertContains(t, tooMany.Body.String(), `role="alert"`, "not enough stock")

	invalid := client.mutate(http.MethodPost, "/cart/items", url.Values{"productId": {"p1"}, "quantity": {"0"}}, true)
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("quantity status = %d, want 422", invalid.Code)
	}
	assertContains(t, invalid.Body.String(), "Choose a quantity")

	missing := client.mutate(http.MethodPost, "/cart/items", url.Values{"productId": {"nope"}, "quantity": {"1"}}, true)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing product status = %d, want 404", missing.Code)
	}

	if count, _ := env.cartQuantity("p1"); count != 0 {
		t.Fatalf("failed mutations must not write rows, got %d", count)
	}

	if ok := client.mutate(http.MethodPost, "/cart/items", url.Values{"productId": {"p1"}, "quantity": {"2"}}, true); ok.Code != http.StatusOK {
		t.Fatalf("valid add status = %d", ok.Code)
	}
	if tooManyNow := client.mutate(http.MethodPost, "/cart/items", url.Values{"productId": {"p1"}, "quantity": {"1"}}, true); tooManyNow.Code != http.StatusConflict {
		t.Fatalf("incremental stock status = %d, want 409", tooManyNow.Code)
	}
	if count, _ := env.cartQuantity("p1"); count != 2 {
		t.Fatalf("quantity must stay at stock limit, got %d", count)
	}
}

func TestGuestCartIsolationAtHTTPLayer(t *testing.T) {
	env := newStorefrontEnv(t)
	env.insertProduct(storefrontProduct{ID: "p1", Name: "Ring", Price: 100, Stock: 10, Active: true})
	env.insertProduct(storefrontProduct{ID: "p2", Name: "Necklace", Price: 200, Stock: 10, Active: true})

	clientA := env.client()
	clientB := env.client()
	clientA.bootstrap()
	clientB.bootstrap()
	if clientA.cookie(session.GuestCookieName) == clientB.cookie(session.GuestCookieName) {
		t.Fatal("separate browsers must receive separate guest sessions")
	}

	if rec := clientA.mutate(http.MethodPost, "/cart/items", url.Values{"productId": {"p1"}, "quantity": {"1"}}, true); rec.Code != http.StatusOK {
		t.Fatalf("A add: %d", rec.Code)
	}
	if rec := clientB.mutate(http.MethodPost, "/cart/items", url.Values{"productId": {"p2"}, "quantity": {"2"}}, true); rec.Code != http.StatusOK {
		t.Fatalf("B add: %d", rec.Code)
	}

	aCart := clientA.get("/cart").Body.String()
	assertContains(t, aCart, "Ring")
	assertNotContains(t, aCart, "Necklace")

	bCart := clientB.get("/cart").Body.String()
	assertContains(t, bCart, "Necklace")
	assertNotContains(t, bCart, ">Ring<")

	crossUpdate := clientA.mutate(http.MethodPatch, "/cart/items/p2", url.Values{"quantity": {"9"}}, true)
	if crossUpdate.Code != http.StatusNotFound {
		t.Fatalf("cross-cart update status = %d, want 404", crossUpdate.Code)
	}
	_ = clientA.mutate(http.MethodDelete, "/cart/items/p2", nil, true)

	bCart = clientB.get("/cart").Body.String()
	assertContains(t, bCart, "Necklace", `value="2"`)
	if count, _ := env.cartQuantity("p2"); count != 2 {
		t.Fatalf("cross-cart delete changed B's cart, quantity=%d", count)
	}
}

func TestCustomerCartReusesExistingCartItemTable(t *testing.T) {
	env := newStorefrontEnv(t)
	env.insertCustomer("c1", "customer@example.com")
	env.insertProduct(storefrontProduct{ID: "p1", Name: "Ring", Price: 100, Stock: 5, Active: true})

	token, err := auth.GenerateTokenWithClaims(storefrontJWTSecret, "c1", auth.KindCustomer, "customer", time.Hour)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	client := env.client()
	client.setCookie("token", token)
	client.bootstrap()

	cartPage := client.get("/cart")
	if cartPage.Code != http.StatusOK {
		t.Fatalf("cart status = %d", cartPage.Code)
	}
	assertContains(t, cartPage.Body.String(), "Your cart is empty")

	added := client.mutate(http.MethodPost, "/cart/items", url.Values{"productId": {"p1"}, "quantity": {"2"}}, true)
	if added.Code != http.StatusOK {
		t.Fatalf("add status = %d: %s", added.Code, added.Body.String())
	}
	assertContains(t, added.Body.String(), "Ring", `value="2"`, "₹200.00")

	var cartRows int
	if err := env.db.QueryRow(`SELECT COUNT(1) FROM "CartItem" WHERE customerId = 'c1' AND productId = 'p1'`).Scan(&cartRows); err != nil {
		t.Fatal(err)
	}
	var guestRows int
	if err := env.db.QueryRow(`SELECT COUNT(1) FROM "GuestCartItem"`).Scan(&guestRows); err != nil {
		t.Fatal(err)
	}
	if cartRows != 1 || guestRows != 0 {
		t.Fatalf("customer must use CartItem, got cart=%d guest=%d", cartRows, guestRows)
	}

	// The existing JSON API sees the same row through the shared cart service.
	cartPage = client.get("/cart")
	assertContains(t, cartPage.Body.String(), "Ring", "2 items", "₹200.00")
}

func TestProductImagePathDoesNotShadowSlugDetail(t *testing.T) {
	env := newStorefrontEnv(t)
	env.insertProduct(storefrontProduct{ID: "p1", Slug: "silver-ring", Name: "Silver Ring", Price: 100, Stock: 5, Active: true})
	client := env.client()

	// A slug still resolves to the detail page.
	if rec := client.get("/products/silver-ring"); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "<title>Silver Ring · Pusti Shangar</title>") {
		t.Fatalf("slug detail status = %d", rec.Code)
	}

	// An image-looking path is served from UPLOADS_DIR/Products rather than
	// being interpreted as a product slug.
	imagePath := filepath.Join(env.cfg.UploadsDir, "Products", "silver-ring.jpg")
	if err := writeFile(imagePath, []byte("jpeg-bytes")); err != nil {
		t.Fatal(err)
	}
	image := client.get("/products/silver-ring.jpg")
	if image.Code != http.StatusOK || image.Body.String() != "jpeg-bytes" {
		t.Fatalf("image status = %d body=%q", image.Code, image.Body.String())
	}

	if rec := client.get("/products/../../go.mod"); rec.Code == http.StatusOK &&
		strings.Contains(rec.Body.String(), "module ") {
		t.Fatal("path traversal leaked a source file")
	}
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
