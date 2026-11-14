// Package render contains the small rendering layer shared by the HTML
// handlers. It deliberately does not know anything about business logic.
package render

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/session"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/views/layouts"
	"github.com/mitesh4000/pushti_sangar/server/internal/web/views/pages"
)

// Renderer writes Templ components to an HTTP response.
//
// Components are rendered into a buffer first so a template error can never
// leave a partially written page (or a misleading 200 response) behind.
type Renderer struct {
	logger *log.Logger
}

// New returns a Renderer that logs rendering failures to logger. A nil logger
// falls back to the standard library logger.
func New(logger *log.Logger) *Renderer {
	if logger == nil {
		logger = log.Default()
	}
	return &Renderer{logger: logger}
}

// Page renders a complete HTML document inside the shared layout, using the
// request-scoped session for the CSRF token and cart count.
func (rr *Renderer) Page(w http.ResponseWriter, r *http.Request, status int, title string, component templ.Component) {
	data := session.FromContext(r.Context())
	chrome := layouts.Chrome{
		CSRFToken: data.CSRFToken,
		CartCount: data.CartCount,
		Path:      r.URL.Path,
	}
	rr.render(w, r, status, layouts.Base(title, chrome, component), false)
}

// Fragment renders an HTML fragment intended for an HTMX swap.
func (rr *Renderer) Fragment(w http.ResponseWriter, r *http.Request, status int, component templ.Component) {
	rr.render(w, r, status, component, true)
}

// ValidationError renders a fragment with a 4xx status. It is a semantic
// wrapper around Fragment so form handlers have one obvious place to return
// field-level errors.
func (rr *Renderer) ValidationError(w http.ResponseWriter, r *http.Request, status int, component templ.Component) {
	if status < 400 || status > 499 {
		status = http.StatusUnprocessableEntity
	}
	rr.Fragment(w, r, status, component)
}

// Error renders a complete, user-safe error page. Internal error details must
// be logged by the caller; only message is shown to the user.
func (rr *Renderer) Error(w http.ResponseWriter, r *http.Request, status int, message string) {
	if status < 400 || status > 599 {
		status = http.StatusInternalServerError
	}
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	data := session.FromContext(r.Context())
	chrome := layouts.Chrome{
		CSRFToken: data.CSRFToken,
		CartCount: data.CartCount,
		Path:      r.URL.Path,
	}
	component := layouts.Base(
		fmt.Sprintf("%d %s", status, http.StatusText(status)),
		chrome,
		pages.Error(status, message),
	)
	rr.render(w, r, status, component, false)
}

// Redirect sends a normal browser redirect, or an HX-Redirect header for an
// HTMX request. It is safe to call after a successful form submission.
func (rr *Renderer) Redirect(w http.ResponseWriter, r *http.Request, location string) {
	if IsHTMX(r) {
		w.Header().Set("HX-Redirect", location)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
}

// IsHTMX reports whether the request was initiated by HTMX.
//
// This only describes the transport. It is not an authentication or
// authorisation signal: a client can set HX-Request itself, so protected
// handlers must still run the normal auth middleware.
func IsHTMX(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("HX-Request")), "true")
}

func (rr *Renderer) render(w http.ResponseWriter, r *http.Request, status int, component templ.Component, fragment bool) {
	if component == nil {
		rr.logger.Printf("web: nil component for %s %s", r.Method, r.URL.Path)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if status < 100 || status > 599 {
		status = http.StatusOK
	}

	var buf bytes.Buffer
	if err := component.Render(r.Context(), &buf); err != nil {
		rr.logger.Printf("web: render %s %s: %v", r.Method, r.URL.Path, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("X-Content-Type-Options", "nosniff")
	// Every page embeds a per-session CSRF token and cart state, so no HTML
	// response may be stored by a browser or shared cache.
	header.Set("Cache-Control", "no-store")
	if fragment {
		header.Set("Vary", "HX-Request")
	}
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		rr.logger.Printf("web: write response %s %s: %v", r.Method, r.URL.Path, err)
	}
}
