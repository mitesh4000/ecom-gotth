// Package webhandlers contains HTTP handlers for server-rendered pages.
package webhandlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/response"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/render"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/views/layouts"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/views/pages"
)

// Handler serves the server-rendered pages. It receives the same database and
// configuration used by the JSON API so future pages can reuse the existing
// data access without a second application stack.
type Handler struct {
	DB       *sql.DB
	Cfg      *config.Config
	Renderer *render.Renderer
}

// New creates a Handler. Renderer must not be nil; RegisterWebRoutes supplies
// a default when the caller does not inject one.
func New(db *sql.DB, cfg *config.Config, renderer *render.Renderer) *Handler {
	return &Handler{DB: db, Cfg: cfg, Renderer: renderer}
}

// Home renders the full placeholder homepage.
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	h.Renderer.Page(w, r, http.StatusOK, layouts.Base("Home", pages.Home()))
}

// ServerMessage demonstrates a small HTMX interaction.
//
// HTMX requests receive an HTML fragment. A normal browser request is
// redirected back to the demo section on the homepage instead of rendering a
// fragment as a standalone document.
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
