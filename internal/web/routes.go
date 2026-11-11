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

	r.Get("/", h.Home)
	r.Get("/health", h.Health)
	r.Get("/fragments/server-message", h.ServerMessage)
	r.Handle("/static/*", cacheStatic(http.StripPrefix("/static/", http.FileServer(http.FS(staticassets.FS)))))
}

func cacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/static/") {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		next.ServeHTTP(w, r)
	})
}
