package service_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/service"
)

type testProduct struct {
	ID            string
	Slug          string
	Name          string
	Description   string
	CategoryID    string
	Tags          string
	Gallery       string
	Original      *float64
	Discounted    *float64
	Calculated    *float64
	Stock         *int
	Shipping      *float64
	Material      string
	Color         string
	Size          string
	SKU           string
	Active        bool
	Popular       bool
	New           bool
	Variant       bool
	MainProductID string
	CreatedAt     time.Time
}

func floatPtr(v float64) *float64 { return &v }

func insertTestCategory(t *testing.T, db *sql.DB, id, name string, active bool) {
	t.Helper()
	flag := 0
	if active {
		flag = 1
	}
	if _, err := db.Exec(`INSERT INTO "Category" (id, name, isActive, isDeleted) VALUES (?, ?, ?, 0)`,
		id, name, flag); err != nil {
		t.Fatalf("insert category %s: %v", id, err)
	}
}

func insertCatalogProduct(t *testing.T, db *sql.DB, p testProduct) {
	t.Helper()
	if p.Name == "" {
		p.Name = p.ID
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	active, popular, isNew, variant := 0, 0, 0, 0
	if p.Active {
		active = 1
	}
	if p.Popular {
		popular = 1
	}
	if p.New {
		isNew = 1
	}
	if p.Variant {
		variant = 1
	}
	_, err := db.Exec(`INSERT INTO "Product"
		(id, slug, name, description, categoryId, tags, pricesOriginal, pricesDiscounted,
		 pricesCalculatedPrice, imageGallery, stockQuantity, shippingCharge, material, color,
		 size, sku, isActive, isProductPopular, isProductNew, isVariant, mainProductId,
		 createdAt, updatedAt)
		VALUES (?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)`,
		p.ID, p.Slug, p.Name, p.Description, p.CategoryID, p.Tags, p.Original, p.Discounted,
		p.Calculated, p.Gallery, p.Stock, p.Shipping, p.Material, p.Color, p.Size, p.SKU,
		active, popular, isNew, variant, p.MainProductID,
		p.CreatedAt.Format(time.RFC3339Nano), p.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("insert product %s: %v", p.ID, err)
	}
}

func TestCatalogListFiltersSortsAndPaginates(t *testing.T) {
	db := newDB(t)
	insertTestCategory(t, db, "cat-rings", "Rings", true)
	insertTestCategory(t, db, "cat-necklaces", "Necklaces", true)

	now := time.Now().UTC()
	insertCatalogProduct(t, db, testProduct{
		ID: "p1", Slug: "silver-ring", Name: "Silver Ring", Description: "A hand-finished band",
		CategoryID: "cat-rings", Tags: `["silver","gift"]`, Calculated: floatPtr(100),
		Original: floatPtr(150), Stock: intPtr(5), Popular: true, New: true, Active: true,
		CreatedAt: now.Add(-2 * time.Hour),
	})
	insertCatalogProduct(t, db, testProduct{
		ID: "p2", Slug: "gold-necklace", Name: "Gold Necklace", CategoryID: "cat-necklaces",
		Calculated: floatPtr(200), Stock: intPtr(10), Active: true, CreatedAt: now.Add(-time.Hour),
	})
	insertCatalogProduct(t, db, testProduct{
		ID: "p3", Name: "Hidden Ring", CategoryID: "cat-rings", Calculated: floatPtr(1),
		Active: false, CreatedAt: now,
	})
	insertCatalogProduct(t, db, testProduct{
		ID: "p4", Name: "Ring Variant", CategoryID: "cat-rings", Calculated: floatPtr(1),
		Variant: true, MainProductID: "p1", Active: true, CreatedAt: now,
	})
	insertCatalogProduct(t, db, testProduct{
		ID: "p5", Name: "Unpriced Ring", CategoryID: "cat-rings", Active: true, CreatedAt: now,
	})

	catalog := service.NewCatalog(db)
	ctx := context.Background()

	result, err := catalog.List(ctx, service.ProductQuery{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if result.Total != 2 || len(result.Products) != 2 {
		t.Fatalf("expected two visible products, got total=%d rows=%d", result.Total, len(result.Products))
	}
	if result.Products[0].ID != "p2" || result.Products[1].ID != "p1" {
		t.Fatalf("default sort should be newest first, got %s then %s", result.Products[0].ID, result.Products[1].ID)
	}
	if result.Products[1].Price != 100 || !result.Products[1].HasDiscount || result.Products[1].DiscountPct != 33 {
		t.Fatalf("unexpected price/discount: %+v", result.Products[1])
	}
	if result.Products[1].URL() != "/products/silver-ring" {
		t.Fatalf("unexpected product URL %q", result.Products[1].URL())
	}

	search, err := catalog.List(ctx, service.ProductQuery{Q: "gift"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if search.Total != 1 || search.Products[0].ID != "p1" {
		t.Fatalf("tag search failed: %+v", search.Products)
	}

	byName, err := catalog.List(ctx, service.ProductQuery{Category: "Necklaces"})
	if err != nil {
		t.Fatalf("category by name: %v", err)
	}
	if byName.Total != 1 || byName.Products[0].ID != "p2" {
		t.Fatalf("category by name failed: %+v", byName.Products)
	}

	bySlug, err := catalog.List(ctx, service.ProductQuery{Category: "cat-rings"})
	if err != nil {
		t.Fatalf("category by id: %v", err)
	}
	if bySlug.Total != 1 || bySlug.Products[0].ID != "p1" {
		t.Fatalf("category by id failed: %+v", bySlug.Products)
	}

	unknown, err := catalog.List(ctx, service.ProductQuery{Category: "missing"})
	if err != nil {
		t.Fatalf("unknown category must not error: %v", err)
	}
	if unknown.Total != 0 {
		t.Fatalf("unknown category should be empty, got %d", unknown.Total)
	}

	asc, err := catalog.List(ctx, service.ProductQuery{Sort: "price-asc"})
	if err != nil {
		t.Fatalf("sort: %v", err)
	}
	if asc.Products[0].ID != "p1" || asc.Products[1].ID != "p2" {
		t.Fatalf("price-asc order wrong: %s, %s", asc.Products[0].ID, asc.Products[1].ID)
	}

	page, err := catalog.List(ctx, service.ProductQuery{Sort: "price-asc", Limit: 1, Page: 2})
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if page.Page != 2 || page.PageCount != 2 || page.Total != 2 {
		t.Fatalf("unexpected pagination: %+v", page)
	}
	if len(page.Products) != 1 || page.Products[0].ID != "p2" {
		t.Fatalf("page two wrong: %+v", page.Products)
	}
	if page.RangeStart() != 2 || page.RangeEnd() != 2 {
		t.Fatalf("range = %d..%d, want 2..2", page.RangeStart(), page.RangeEnd())
	}
	if got := page.PageURL(1); got != "/products?limit=1&sort=price-asc" {
		t.Fatalf("page URL = %q", got)
	}

	over, err := catalog.List(ctx, service.ProductQuery{Limit: 1, Page: 99})
	if err != nil {
		t.Fatalf("clamp page: %v", err)
	}
	if over.Page != 2 {
		t.Fatalf("page should clamp to 2, got %d", over.Page)
	}
}

func TestCatalogDetailVariantsRelatedAndSections(t *testing.T) {
	db := newDB(t)
	insertTestCategory(t, db, "cat-1", "Rings", true)
	now := time.Now().UTC()
	insertCatalogProduct(t, db, testProduct{
		ID: "p1", Slug: "silver-ring", Name: "Silver Ring", CategoryID: "cat-1",
		Calculated: floatPtr(100), Popular: true, New: true, Active: true, CreatedAt: now.Add(-2 * time.Hour),
	})
	insertCatalogProduct(t, db, testProduct{
		ID: "p2", Slug: "gold-ring", Name: "Gold Ring", CategoryID: "cat-1",
		Calculated: floatPtr(250), Active: true, CreatedAt: now.Add(-time.Hour),
	})
	insertCatalogProduct(t, db, testProduct{
		ID: "p3", Name: "Matching Band", CategoryID: "cat-1", Calculated: floatPtr(80),
		Variant: true, MainProductID: "p1", Active: true, CreatedAt: now,
	})

	catalog := service.NewCatalog(db)
	ctx := context.Background()

	bySlug, err := catalog.BySlugOrID(ctx, "silver-ring")
	if err != nil || bySlug.ID != "p1" {
		t.Fatalf("by slug: %+v, %v", bySlug, err)
	}
	byID, err := catalog.BySlugOrID(ctx, "p2")
	if err != nil || byID.ID != "p2" {
		t.Fatalf("by id: %+v, %v", byID, err)
	}
	if _, err := catalog.BySlugOrID(ctx, "nope"); !errors.Is(err, service.ErrProductNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}

	variants, err := catalog.Variants(ctx, "p1")
	if err != nil {
		t.Fatalf("variants: %v", err)
	}
	if len(variants) != 1 || variants[0].ID != "p3" {
		t.Fatalf("variants = %+v", variants)
	}

	related, err := catalog.Related(ctx, *bySlug, 4)
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	if len(related) != 1 || related[0].ID != "p2" {
		t.Fatalf("related = %+v", related)
	}

	featured, err := catalog.Featured(ctx, 4)
	if err != nil {
		t.Fatalf("featured: %v", err)
	}
	if len(featured) != 1 || featured[0].ID != "p1" {
		t.Fatalf("featured = %+v", featured)
	}

	arrivals, err := catalog.NewArrivals(ctx, 4)
	if err != nil {
		t.Fatalf("new arrivals: %v", err)
	}
	if len(arrivals) != 1 || arrivals[0].ID != "p1" {
		t.Fatalf("arrivals = %+v", arrivals)
	}

	categories, err := catalog.Categories(ctx)
	if err != nil {
		t.Fatalf("categories: %v", err)
	}
	if len(categories) != 1 || categories[0].ProductCount != 2 {
		t.Fatalf("categories = %+v", categories)
	}
	if categories[0].Image != service.CategoryPlaceholder {
		t.Fatalf("category image = %q, want placeholder", categories[0].Image)
	}
}

func TestCatalogBannersAndImageNormalisation(t *testing.T) {
	db := newDB(t)
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO "Banner"
		(id, bannerTitle, image, description, bannerType, bannerText, isActive, isDeleted, createdAt, updatedAt)
		VALUES ('b1', 'Festive edit', 'uploads/Banner/festive.jpg', 'Celebrate', 'hero', 'Festive', 1, 0, ?, ?)`,
		now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("insert banner: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO "Banner"
		(id, bannerTitle, image, isActive, isDeleted, createdAt, updatedAt)
		VALUES ('b2', 'Empty image', '', 1, 0, ?, ?)`,
		now.Add(-time.Hour).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("insert banner: %v", err)
	}

	banners, err := service.NewCatalog(db).Banners(context.Background(), 2)
	if err != nil {
		t.Fatalf("banners: %v", err)
	}
	if len(banners) != 2 || banners[0].ID != "b1" {
		t.Fatalf("banners = %+v", banners)
	}
	if banners[0].Image != "/uploads/Banner/festive.jpg" {
		t.Fatalf("banner image = %q", banners[0].Image)
	}
	if banners[1].Image != service.BannerPlaceholder {
		t.Fatalf("empty banner image = %q", banners[1].Image)
	}

	tests := map[string]string{
		"":                              service.ProductPlaceholder,
		"silver-ring.jpg":               "/uploads/Products/silver-ring.jpg",
		"uploads/Products/silver.jpg":   "/uploads/Products/silver.jpg",
		"../private/Products/x.png":     "/uploads/Products/x.png",
		"https://cdn.example.com/a.jpg": "https://cdn.example.com/a.jpg",
		"javascript:alert(1)":           service.ProductPlaceholder,
		"//evil.example.com/a.jpg":      service.ProductPlaceholder,
	}
	for input, want := range tests {
		if got := service.ImageURL(input); got != want {
			t.Errorf("ImageURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFormatMoney(t *testing.T) {
	cases := map[float64]string{
		0:       "₹0.00",
		12.5:    "₹12.50",
		1234.5:  "₹1,234.50",
		1234567: "₹1,234,567.00",
		-5:      "₹0.00",
	}
	for input, want := range cases {
		if got := service.FormatMoney(input); got != want {
			t.Errorf("FormatMoney(%v) = %q, want %q", input, got, want)
		}
	}
	if !strings.Contains(service.FormatMoney(1234.5), ",") {
		t.Fatal("expected grouped thousands")
	}
}
