package webhandlers

import (
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/service"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/render"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/views/components"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/views/pages"
)

// Products renders the catalogue listing. HTMX requests receive only the
// results body so filters, sorting and pagination are server-side and the
// full page remains usable without JavaScript.
func (h *Handler) Products(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	limit, _ := strconv.Atoi(query.Get("limit"))

	result, err := h.Catalog.List(r.Context(), service.ProductQuery{
		Q:        query.Get("q"),
		Category: query.Get("category"),
		Sort:     query.Get("sort"),
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		slog.Error("web: product listing", "error", err)
		h.Renderer.Error(w, r, http.StatusInternalServerError, "The catalogue could not be loaded.")
		return
	}

	if render.IsHTMX(r) {
		h.Renderer.Fragment(w, r, http.StatusOK, components.ProductResults(result))
		return
	}

	categories, err := h.Catalog.Categories(r.Context())
	if err != nil {
		slog.Error("web: listing categories", "error", err)
	}
	h.Renderer.Page(w, r, http.StatusOK, "Shop", pages.Products(result, categories))
}

// Product renders one product detail page. The route accepts a slug and falls
// back to the id so old links keep working. The legacy /products/* upload
// alias is registered before this route, so image-looking paths are served
// from UPLOADS_DIR/Products instead of being treated as product slugs.
func (h *Handler) Product(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(chi.URLParam(r, "slug"))
	if slug == "" {
		h.Renderer.Error(w, r, http.StatusNotFound, "That product could not be found.")
		return
	}
	if isProductImagePath(slug) {
		h.serveProductImage(w, r, slug)
		return
	}

	product, err := h.Catalog.BySlugOrID(r.Context(), slug)
	if errors.Is(err, service.ErrProductNotFound) {
		h.Renderer.Error(w, r, http.StatusNotFound, "That product could not be found.")
		return
	}
	if err != nil {
		slog.Error("web: product detail", "error", err)
		h.Renderer.Error(w, r, http.StatusInternalServerError, "This product could not be loaded.")
		return
	}

	variants, err := h.Catalog.Variants(r.Context(), product.ID)
	if err != nil {
		slog.Error("web: product variants", "error", err)
	}
	related, err := h.Catalog.Related(r.Context(), *product, 4)
	if err != nil {
		slog.Error("web: related products", "error", err)
	}

	h.Renderer.Page(w, r, http.StatusOK, product.Name, pages.Product(pages.ProductDetail{
		Product:  *product,
		Variants: variants,
		Related:  related,
	}))
}

func (h *Handler) serveProductImage(w http.ResponseWriter, r *http.Request, name string) {
	if h.Cfg == nil || strings.TrimSpace(h.Cfg.UploadsDir) == "" {
		h.Renderer.Error(w, r, http.StatusNotFound, "Image not found.")
		return
	}
	// filepath.Base prevents traversal; only known image extensions reach here.
	file := filepath.Join(h.Cfg.UploadsDir, "Products", filepath.Base(name))
	http.ServeFile(w, r, file)
}

func isProductImagePath(value string) bool {
	switch strings.ToLower(filepath.Ext(value)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".avif":
		return true
	}
	return false
}
