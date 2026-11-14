package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// ErrProductNotFound is returned when a storefront product cannot be resolved.
var ErrProductNotFound = errors.New("product not found")

// Placeholder images are bundled with the binary so a product with no image,
// or with a stale upload path, never renders a broken image element.
const (
	ProductPlaceholder  = "/static/images/product-placeholder.svg"
	CategoryPlaceholder = "/static/images/category-placeholder.svg"
	BannerPlaceholder   = "/static/images/banner-placeholder.svg"
)

// StoreProduct is the presentation-safe product shape used by the storefront.
// Monetary values are always resolved from the database.
type StoreProduct struct {
	ID             string
	Slug           string
	Name           string
	Description    string
	CategoryID     string
	CategoryName   string
	CategorySlug   string
	Tags           []string
	OriginalPrice  float64
	Price          float64
	HasDiscount    bool
	DiscountPct    int
	Image          string
	Images         []string
	ShippingCharge float64
	Stock          int
	StockTracked   bool
	InStock        bool
	IsNew          bool
	IsPopular      bool
	Material       string
	Color          string
	Size           string
	SKU            string
	CreatedAt      time.Time
}

// URL returns the canonical detail path for the product.
func (p StoreProduct) URL() string {
	slug := strings.TrimSpace(p.Slug)
	if slug == "" {
		slug = p.ID
	}
	return "/products/" + url.PathEscape(slug)
}

// StoreCategory is a category link with a product count.
type StoreCategory struct {
	ID           string
	Name         string
	Slug         string
	Image        string
	Description  string
	ProductCount int
}

// URL returns the listing path filtered to this category.
func (c StoreCategory) URL() string {
	return "/products?category=" + url.QueryEscape(c.ID)
}

// StoreBanner is an active CMS banner.
type StoreBanner struct {
	ID          string
	Title       string
	Image       string
	Description string
	Type        string
	Text        string
}

// ProductQuery describes a server-side catalogue request. Only allow-listed
// values are applied; unknown sort values fall back to newest.
type ProductQuery struct {
	Q         string
	Category  string
	Sort      string
	Page      int
	Limit     int
	Featured  bool
	NewOnly   bool
	ExcludeID string
}

// Normalize bounds and canonicalises user-supplied query values.
func (q ProductQuery) Normalize() ProductQuery {
	q.Q = strings.TrimSpace(q.Q)
	if len(q.Q) > 100 {
		q.Q = q.Q[:100]
	}
	q.Category = strings.TrimSpace(q.Category)
	if len(q.Category) > 100 {
		q.Category = q.Category[:100]
	}
	switch q.Sort {
	case "price-asc", "price-desc", "name-asc", "name-desc", "popular", "newest":
	default:
		q.Sort = "newest"
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit <= 0 {
		q.Limit = 12
	}
	if q.Limit > 48 {
		q.Limit = 48
	}
	return q
}

// ProductResult is one page of catalogue results.
type ProductResult struct {
	Products  []StoreProduct
	Total     int
	Page      int
	Limit     int
	PageCount int
	Query     ProductQuery
}

// HasPrevious and HasNext drive pagination controls without template logic.
func (r ProductResult) HasPrevious() bool { return r.Page > 1 }
func (r ProductResult) HasNext() bool     { return r.Page < r.PageCount }

// PageURL builds a pagination link while preserving the active filters.
func (r ProductResult) PageURL(page int) string {
	values := url.Values{}
	if r.Query.Q != "" {
		values.Set("q", r.Query.Q)
	}
	if r.Query.Category != "" {
		values.Set("category", r.Query.Category)
	}
	if r.Query.Sort != "" && r.Query.Sort != "newest" {
		values.Set("sort", r.Query.Sort)
	}
	if r.Query.Limit > 0 && r.Query.Limit != 12 {
		values.Set("limit", strconv.Itoa(r.Query.Limit))
	}
	if page > 1 {
		values.Set("page", strconv.Itoa(page))
	}
	if encoded := values.Encode(); encoded != "" {
		return "/products?" + encoded
	}
	return "/products"
}

// RangeStart is the 1-based index of the first visible result.
func (r ProductResult) RangeStart() int {
	if r.Total == 0 {
		return 0
	}
	return (r.Page-1)*r.Limit + 1
}

// RangeEnd is the 1-based index of the last visible result.
func (r ProductResult) RangeEnd() int {
	end := r.RangeStart() + len(r.Products) - 1
	if end < 0 {
		return 0
	}
	return end
}

// Catalog reads the existing catalogue tables. It does not create a second
// product model or duplicate catalogue data.
type Catalog struct {
	DB *sql.DB
}

// NewCatalog returns a catalogue reader backed by db.
func NewCatalog(db *sql.DB) *Catalog { return &Catalog{DB: db} }

type catalogueRow struct {
	ID           string
	Slug         string
	Name         sql.NullString
	Description  sql.NullString
	CategoryID   sql.NullString
	CategoryName sql.NullString
	Tags         sql.NullString
	Original     sql.NullFloat64
	Discounted   sql.NullFloat64
	Calculated   sql.NullFloat64
	Gallery      sql.NullString
	StockQty     sql.NullInt64
	Shipping     sql.NullFloat64
	Material     sql.NullString
	Color        sql.NullString
	Size         sql.NullString
	SKU          sql.NullString
	IsPopular    int
	IsNew        int
	CreatedAt    sql.NullString
}

const productSelectColumns = `
	p.id,
	COALESCE(NULLIF(p.slug, ''), p.id) AS slug,
	p.name,
	p.description,
	p.categoryId,
	c.name AS categoryName,
	p.tags,
	p.pricesOriginal,
	p.pricesDiscounted,
	p.pricesCalculatedPrice,
	p.imageGallery,
	p.stockQuantity,
	p.shippingCharge,
	p.material,
	p.color,
	p.size,
	p.sku,
	p.isProductPopular,
	p.isProductNew,
	p.createdAt`

const productFromClause = `
	FROM "Product" p
	LEFT JOIN "Category" c ON c.id = p.categoryId`

// List returns active, non-variant products matching q.
func (c *Catalog) List(ctx context.Context, q ProductQuery) (ProductResult, error) {
	q = q.Normalize()
	result := ProductResult{Products: []StoreProduct{}, Page: q.Page, Limit: q.Limit, Query: q}
	if c == nil || c.DB == nil {
		return result, nil
	}

	// Products without any positive price cannot be sold, so they are not
	// advertised or counted in the listing. This matches productsFromRows,
	// which skips them, and keeps Total aligned with the visible rows.
	where := []string{
		"p.isActive = 1",
		"COALESCE(p.isVariant, 0) = 0",
		"(p.pricesCalculatedPrice > 0 OR p.pricesDiscounted > 0 OR p.pricesOriginal > 0)",
	}
	args := []any{}
	if q.Q != "" {
		like := "%" + strings.ToLower(q.Q) + "%"
		where = append(where,
			"(LOWER(COALESCE(p.name, '')) LIKE ? OR LOWER(COALESCE(p.description, '')) LIKE ? OR LOWER(p.tags) LIKE ?)")
		args = append(args, like, like, like)
	}
	if q.Category != "" {
		categoryID, err := c.resolveCategoryID(ctx, q.Category)
		if err != nil {
			return result, err
		}
		if categoryID != "" {
			where = append(where, "p.categoryId = ?")
			args = append(args, categoryID)
		} else {
			// An unknown category is a valid empty result, not an error.
			where = append(where, "1 = 0")
		}
	}
	if q.Featured {
		where = append(where, "p.isProductPopular = 1")
	}
	if q.NewOnly {
		where = append(where, "p.isProductNew = 1")
	}
	if q.ExcludeID != "" {
		where = append(where, "p.id <> ?")
		args = append(args, q.ExcludeID)
	}
	whereSQL := strings.Join(where, " AND ")

	if err := c.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM "Product" p WHERE `+whereSQL, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	if result.Total == 0 {
		return result, nil
	}
	result.PageCount = int(math.Ceil(float64(result.Total) / float64(q.Limit)))
	if result.Page > result.PageCount {
		result.Page = result.PageCount
	}
	offset := (result.Page - 1) * q.Limit

	order := orderClause(q.Sort)
	rows, err := c.DB.QueryContext(ctx,
		`SELECT `+productSelectColumns+productFromClause+` WHERE `+whereSQL+` ORDER BY `+order+` LIMIT ? OFFSET ?`,
		append(args, q.Limit, offset)...)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	items, err := c.scanProducts(ctx, rows)
	if err != nil {
		return result, err
	}
	result.Products = items
	return result, nil
}

// BySlugOrID resolves a product detail page by slug, falling back to the id so
// links created before slugs existed keep working.
func (c *Catalog) BySlugOrID(ctx context.Context, slugOrID string) (*StoreProduct, error) {
	slugOrID = strings.TrimSpace(slugOrID)
	if c == nil || c.DB == nil || slugOrID == "" {
		return nil, ErrProductNotFound
	}
	row := c.DB.QueryRowContext(ctx,
		`SELECT `+productSelectColumns+productFromClause+`
		 WHERE (p.slug = ? OR p.id = ?) AND p.isActive = 1
		 LIMIT 1`, slugOrID, slugOrID)
	p, err := c.scanProduct(ctx, row)
	if err == sql.ErrNoRows {
		return nil, ErrProductNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Variants returns active variants of a main product.
func (c *Catalog) Variants(ctx context.Context, mainProductID string) ([]StoreProduct, error) {
	if c == nil || c.DB == nil || mainProductID == "" {
		return []StoreProduct{}, nil
	}
	rows, err := c.DB.QueryContext(ctx,
		`SELECT `+productSelectColumns+productFromClause+`
		 WHERE p.mainProductId = ? AND p.isActive = 1
		 ORDER BY p.createdAt DESC LIMIT 24`, mainProductID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return c.scanProducts(ctx, rows)
}

// Related returns products from the same category, falling back to popular
// items when the product has no category or too few siblings.
func (c *Catalog) Related(ctx context.Context, product StoreProduct, limit int) ([]StoreProduct, error) {
	if limit <= 0 {
		limit = 4
	}
	q := ProductQuery{Sort: "popular", Page: 1, Limit: limit, ExcludeID: product.ID}
	if product.CategoryID != "" {
		q.Category = product.CategoryID
	}
	result, err := c.List(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(result.Products) > 0 {
		return result.Products, nil
	}
	result, err = c.List(ctx, ProductQuery{Sort: "newest", Page: 1, Limit: limit, ExcludeID: product.ID})
	if err != nil {
		return nil, err
	}
	return result.Products, nil
}

// Featured returns popular products for the homepage.
func (c *Catalog) Featured(ctx context.Context, limit int) ([]StoreProduct, error) {
	result, err := c.List(ctx, ProductQuery{Featured: true, Sort: "popular", Limit: limit})
	return result.Products, err
}

// NewArrivals returns recently created products for the homepage.
func (c *Catalog) NewArrivals(ctx context.Context, limit int) ([]StoreProduct, error) {
	result, err := c.List(ctx, ProductQuery{NewOnly: true, Sort: "newest", Limit: limit})
	if err == nil && len(result.Products) == 0 && result.Total == 0 {
		result, err = c.List(ctx, ProductQuery{Sort: "newest", Limit: limit})
	}
	return result.Products, err
}

// Categories returns active categories with a count of active products.
func (c *Catalog) Categories(ctx context.Context) ([]StoreCategory, error) {
	out := []StoreCategory{}
	if c == nil || c.DB == nil {
		return out, nil
	}
	rows, err := c.DB.QueryContext(ctx, `
		SELECT c.id, c.name, c.image, c.description, COUNT(p.id)
		FROM "Category" c
		LEFT JOIN "Product" p
			ON p.categoryId = c.id AND p.isActive = 1 AND COALESCE(p.isVariant, 0) = 0
			AND (p.pricesCalculatedPrice > 0 OR p.pricesDiscounted > 0 OR p.pricesOriginal > 0)
		WHERE c.isDeleted = 0 AND c.isActive = 1
		GROUP BY c.id, c.name, c.image, c.description
		ORDER BY c.name COLLATE NOCASE`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var name, image, description sql.NullString
		var count int
		if err := rows.Scan(&id, &name, &image, &description, &count); err != nil {
			return out, err
		}
		display := strings.TrimSpace(name.String)
		if display == "" {
			display = "Collection"
		}
		out = append(out, StoreCategory{
			ID:           id,
			Name:         display,
			Slug:         slugify(display),
			Image:        imageFor(image.String, CategoryPlaceholder),
			Description:  strings.TrimSpace(description.String),
			ProductCount: count,
		})
	}
	return out, rows.Err()
}

// Banners returns active CMS banners, newest first.
func (c *Catalog) Banners(ctx context.Context, limit int) ([]StoreBanner, error) {
	out := []StoreBanner{}
	if c == nil || c.DB == nil {
		return out, nil
	}
	if limit <= 0 || limit > 10 {
		limit = 3
	}
	rows, err := c.DB.QueryContext(ctx, `
		SELECT id, bannerTitle, image, description, bannerType, bannerText
		FROM "Banner"
		WHERE isDeleted = 0 AND isActive = 1
		ORDER BY createdAt DESC LIMIT ?`, limit)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var title, image, description, bannerType, bannerText sql.NullString
		if err := rows.Scan(&id, &title, &image, &description, &bannerType, &bannerText); err != nil {
			return out, err
		}
		out = append(out, StoreBanner{
			ID:          id,
			Title:       strings.TrimSpace(title.String),
			Image:       imageFor(image.String, BannerPlaceholder),
			Description: strings.TrimSpace(description.String),
			Type:        strings.TrimSpace(bannerType.String),
			Text:        strings.TrimSpace(bannerText.String),
		})
	}
	return out, rows.Err()
}

// ResolveCategoryID exposes the id/name/slug resolution used by filters.
func (c *Catalog) ResolveCategoryID(ctx context.Context, value string) (string, error) {
	return c.resolveCategoryID(ctx, value)
}

func (c *Catalog) resolveCategoryID(ctx context.Context, value string) (string, error) {
	if c == nil || c.DB == nil || value == "" {
		return "", nil
	}
	var id string
	err := c.DB.QueryRowContext(ctx,
		`SELECT id FROM "Category" WHERE isDeleted = 0 AND isActive = 1 AND id = ? LIMIT 1`, value).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}
	err = c.DB.QueryRowContext(ctx,
		`SELECT id FROM "Category"
		 WHERE isDeleted = 0 AND isActive = 1
		   AND (LOWER(COALESCE(name, '')) = LOWER(?) OR LOWER(REPLACE(COALESCE(name, ''), ' ', '-')) = LOWER(?))
		 LIMIT 1`, value, value).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

type productScanner interface {
	Scan(dest ...any) error
}

func (c *Catalog) scanProduct(ctx context.Context, row productScanner) (StoreProduct, error) {
	var r catalogueRow
	if err := row.Scan(&r.ID, &r.Slug, &r.Name, &r.Description, &r.CategoryID, &r.CategoryName,
		&r.Tags, &r.Original, &r.Discounted, &r.Calculated, &r.Gallery, &r.StockQty,
		&r.Shipping, &r.Material, &r.Color, &r.Size, &r.SKU, &r.IsPopular, &r.IsNew, &r.CreatedAt); err != nil {
		return StoreProduct{}, err
	}
	items, err := c.productsFromRows(ctx, []catalogueRow{r})
	if err != nil {
		return StoreProduct{}, err
	}
	if len(items) == 0 {
		return StoreProduct{}, ErrProductNotFound
	}
	return items[0], nil
}

func (c *Catalog) scanProducts(ctx context.Context, rows *sql.Rows) ([]StoreProduct, error) {
	var scanned []catalogueRow
	for rows.Next() {
		var r catalogueRow
		if err := rows.Scan(&r.ID, &r.Slug, &r.Name, &r.Description, &r.CategoryID, &r.CategoryName,
			&r.Tags, &r.Original, &r.Discounted, &r.Calculated, &r.Gallery, &r.StockQty,
			&r.Shipping, &r.Material, &r.Color, &r.Size, &r.SKU, &r.IsPopular, &r.IsNew, &r.CreatedAt); err != nil {
			return nil, err
		}
		scanned = append(scanned, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return c.productsFromRows(ctx, scanned)
}

func (c *Catalog) productsFromRows(ctx context.Context, rows []catalogueRow) ([]StoreProduct, error) {
	if len(rows) == 0 {
		return []StoreProduct{}, nil
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	stocks, err := stockByProduct(ctx, c.DB, ids)
	if err != nil {
		return nil, err
	}
	out := make([]StoreProduct, 0, len(rows))
	for _, r := range rows {
		price, ok := firstPositive(r.Calculated, r.Discounted, r.Original)
		if !ok {
			continue
		}
		original := 0.0
		if r.Original.Valid && r.Original.Float64 > 0 {
			original = round2(r.Original.Float64)
		}
		images := parseImages(r.Gallery.String, ProductPlaceholder)
		stock := stocks[r.ID]
		if !stock.Tracked && r.StockQty.Valid {
			stock = StockInfo{Quantity: int(r.StockQty.Int64), Tracked: true}
		}
		name := strings.TrimSpace(r.Name.String)
		if name == "" {
			name = "Untitled product"
		}
		categoryName := strings.TrimSpace(r.CategoryName.String)
		p := StoreProduct{
			ID:             r.ID,
			Slug:           r.Slug,
			Name:           name,
			Description:    strings.TrimSpace(r.Description.String),
			CategoryID:     r.CategoryID.String,
			CategoryName:   categoryName,
			CategorySlug:   slugify(categoryName),
			Tags:           parseTags(r.Tags.String),
			OriginalPrice:  original,
			Price:          round2(price),
			Image:          images[0],
			Images:         images,
			ShippingCharge: round2(math.Max(0, r.Shipping.Float64)),
			Stock:          stock.Quantity,
			StockTracked:   stock.Tracked,
			InStock:        !stock.Tracked || stock.Quantity > 0,
			IsNew:          r.IsNew != 0,
			IsPopular:      r.IsPopular != 0,
			Material:       strings.TrimSpace(r.Material.String),
			Color:          strings.TrimSpace(r.Color.String),
			Size:           strings.TrimSpace(r.Size.String),
			SKU:            strings.TrimSpace(r.SKU.String),
		}
		if t, ok := parseDBTime(r.CreatedAt); ok {
			p.CreatedAt = t
		}
		p.HasDiscount = original > 0 && p.Price > 0 && original > p.Price
		if p.HasDiscount {
			p.DiscountPct = int(math.Round((original - p.Price) / original * 100))
		}
		out = append(out, p)
	}
	return out, nil
}

// StockInfo is the authoritative available quantity for a product.
type StockInfo struct {
	Quantity int
	Tracked  bool
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func stockByProduct(ctx context.Context, q queryer, ids []string) (map[string]StockInfo, error) {
	out := map[string]StockInfo{}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := q.QueryContext(ctx,
		`SELECT productId, COALESCE(SUM(quantity), 0) FROM "Stock"
		 WHERE productId IN (`+strings.Join(placeholders, ",")+`) GROUP BY productId`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var qty int
		if err := rows.Scan(&id, &qty); err != nil {
			return nil, err
		}
		out[id] = StockInfo{Quantity: qty, Tracked: true}
	}
	return out, rows.Err()
}

func orderClause(sort string) string {
	priceExpr := `CASE
		WHEN p.pricesCalculatedPrice IS NOT NULL AND p.pricesCalculatedPrice > 0 THEN p.pricesCalculatedPrice
		WHEN p.pricesDiscounted IS NOT NULL AND p.pricesDiscounted > 0 THEN p.pricesDiscounted
		WHEN p.pricesOriginal IS NOT NULL AND p.pricesOriginal > 0 THEN p.pricesOriginal
		ELSE 0 END`
	switch sort {
	case "price-asc":
		return priceExpr + " ASC, p.createdAt DESC"
	case "price-desc":
		return priceExpr + " DESC, p.createdAt DESC"
	case "name-asc":
		return "LOWER(COALESCE(p.name, '')) ASC"
	case "name-desc":
		return "LOWER(COALESCE(p.name, '')) DESC"
	case "popular":
		return "p.isProductPopular DESC, p.createdAt DESC"
	default:
		return "p.createdAt DESC"
	}
}

// ImageURL normalises a stored image reference into a browser-safe local URL.
// It never invents a remote host and falls back to a bundled placeholder.
func ImageURL(raw string) string {
	return imageFor(raw, ProductPlaceholder)
}

func imageFor(raw, fallback string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" {
		return fallback
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return raw
	}
	// Reject protocol-relative and non-web schemes. data: and javascript: are
	// never used as an image source on the storefront.
	if strings.HasPrefix(raw, "//") || (strings.Contains(raw, ":") && !strings.HasPrefix(raw, "/")) {
		return fallback
	}
	if i := strings.Index(lower, "/uploads/"); i >= 0 {
		return raw[i:]
	}
	if strings.HasPrefix(lower, "uploads/") {
		return "/" + raw
	}
	if strings.HasPrefix(lower, "/static/") || strings.HasPrefix(raw, "/") {
		return raw
	}
	if i := strings.Index(lower, "/products/"); i >= 0 {
		return "/uploads/Products/" + path.Base(raw[i:])
	}
	if i := strings.Index(lower, "products/"); i >= 0 {
		return "/uploads/Products/" + path.Base(raw[i:])
	}
	if isImageFile(lower) {
		return "/uploads/Products/" + path.Base(raw)
	}
	return fallback
}

func isImageFile(value string) bool {
	switch strings.ToLower(path.Ext(value)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".avif":
		return true
	}
	return false
}

func parseImages(raw, fallback string) []string {
	raw = strings.TrimSpace(raw)
	var values []string
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			// Legacy rows occasionally contain a single plain path.
			values = []string{raw}
		}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		u := imageFor(value, fallback)
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	if len(out) == 0 {
		out = []string{fallback}
	}
	return out
}

func parseTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		var generic []any
		if json.Unmarshal([]byte(raw), &generic) != nil {
			return []string{}
		}
		for _, v := range generic {
			if s, ok := v.(string); ok {
				values = append(values, s)
			}
		}
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// FormatMoney renders an INR amount with grouped thousands. It lives in the
// service package so API and HTML callers use the same representation.
func FormatMoney(value float64) string {
	if value < 0 {
		value = 0
	}
	rounded := int64(math.Round(value * 100))
	whole := rounded / 100
	frac := rounded % 100
	digits := strconv.FormatInt(whole, 10)
	var grouped strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(r)
	}
	return "₹" + grouped.String() + fmt.Sprintf(".%02d", frac)
}
