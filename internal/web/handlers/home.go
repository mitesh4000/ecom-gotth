// Package webhandlers contains HTTP handlers for server-rendered pages.
package webhandlers

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/response"
	"github.com/mitesh4000/pushti_sangar/server/internal/service"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/render"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/views/pages"
)

// Handler serves the server-rendered pages. It receives the same database and
// configuration used by the JSON API so pages reuse the existing data access
// without a second application stack.
type Handler struct {
	DB       *sql.DB
	Cfg      *config.Config
	Renderer *render.Renderer
	Catalog  *service.Catalog
	Cart     *service.CartService
}

// New creates a Handler. Renderer must not be nil; RegisterWebRoutes supplies
// a default when the caller does not inject one.
func New(db *sql.DB, cfg *config.Config, renderer *render.Renderer) *Handler {
	return &Handler{
		DB:       db,
		Cfg:      cfg,
		Renderer: renderer,
		Catalog:  service.NewCatalog(db),
		Cart:     service.NewCart(db),
	}
}

// Home renders the storefront landing page from the real catalogue and CMS
// tables. A database error degrades to an empty section rather than inventing
// products.
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := pages.HomeData{}

	banners, err := h.Catalog.Banners(ctx, 1)
	if err != nil {
		slog.Error("web: home banners", "error", err)
	}
	data.Banners = banners

	categories, err := h.Catalog.Categories(ctx)
	if err != nil {
		slog.Error("web: home categories", "error", err)
	}
	data.Categories = categories

	featured, err := h.Catalog.Featured(ctx, 4)
	if err != nil {
		slog.Error("web: home featured", "error", err)
	}
	data.Featured = featured

	arrivals, err := h.Catalog.NewArrivals(ctx, 4)
	if err != nil {
		slog.Error("web: home new arrivals", "error", err)
	}
	data.NewArrivals = arrivals

	h.Renderer.Page(w, r, http.StatusOK, "Home", pages.Home(data))
}

// ServerMessage demonstrates a small HTMX interaction.
//
// HTMX requests receive an HTML fragment. A normal browser request is
// redirected instead of rendering a fragment as a standalone document.
func (h *Handler) ServerMessage(w http.ResponseWriter, r *http.Request) {
	if !render.IsHTMX(r) {
		h.Renderer.Redirect(w, r, "/#htmx-demo")
		return
	}
	message := "Templ and HTMX are working. The existing Chi API remains available."
	h.Renderer.Fragment(w, r, http.StatusOK, pages.ServerMessage(message, time.Now().UTC()))
}

// Health reports that the web process is running. It intentionally avoids
// touching the database so it can be used as a liveness probe.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "pushti-sangar",
	})
}
