// Package web registers the server-rendered presentation layer on the
// existing Chi router. It does not own or replace the JSON API routes.
package web

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/handlers"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/render"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/session"
	staticassets "github.com/mitesh4000/pushti_sangar/server/static"
)

// WebDependencies is the dependency set needed by HTML handlers.
type WebDependencies struct {
	DB       *sql.DB
	Cfg      *config.Config
	Renderer *render.Renderer
}

// RegisterWebRoutes adds the HTML pages and embedded static assets to r.
//
// API route registration stays in the existing handlers package. Keeping this
// function separate makes it obvious which routes render HTML and which routes
// preserve the original JSON contract.
func RegisterWebRoutes(r chi.Router, deps WebDependencies) {
	renderer := deps.Renderer
	if renderer == nil {
		renderer = render.New(nil)
	}

	h := webhandlers.New(deps.DB, deps.Cfg, renderer)

	secureCookies := false
	jwtSecret := ""
	if deps.Cfg != nil {
		secureCookies = deps.Cfg.CookieSecure
		jwtSecret = deps.Cfg.JWTSecret
	}
	sessions := session.New(deps.DB, jwtSecret, secureCookies)

	// Static assets and liveness do not need session cookies.
	r.Handle("/static/*", cacheStatic(http.StripPrefix("/static/", http.FileServer(http.FS(staticassets.FS)))))
	r.Get("/health", h.Health)

	r.Group(func(r chi.Router) {
		r.Use(sessions.Session)

		r.Get("/", h.Home)
		r.Get("/products", h.Products)
		r.Get("/products/", h.Products)
		r.Get("/products/{slug}", h.Product)
		r.Get("/cart", h.CartPage)
		r.Get("/cart/", h.CartPage)
		r.Get("/fragments/server-message", h.ServerMessage)

		// Cart writes use the correct HTTP verb and a web-specific
		// double-submit check. HX-Request is never an authentication signal.
		r.With(sessions.ProtectCSRF).Post("/cart/items", h.AddCartItem)
		r.With(sessions.ProtectCSRF).Patch("/cart/items/{productID}", h.UpdateCartItem)
		r.With(sessions.ProtectCSRF).Put("/cart/items/{productID}", h.UpdateCartItem)
		r.With(sessions.ProtectCSRF).Delete("/cart/items/{productID}", h.RemoveCartItem)
	})
}

func cacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		next.ServeHTTP(w, r)
	})
}
