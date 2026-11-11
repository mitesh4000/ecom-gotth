package render_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mitesh4000/pushti_sangar/server/internal/web/render"
)

func TestRedirectForHTMX(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	render.New(nil).Redirect(rec, req, "/done")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("HX-Redirect"); got != "/done" {
		t.Fatalf("HX-Redirect = %q, want /done", got)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Fatalf("unexpected Location header %q", got)
	}
}

func TestRedirectForNormalBrowser(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	rec := httptest.NewRecorder()

	render.New(nil).Redirect(rec, req, "/done")

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/done" {
		t.Fatalf("Location = %q, want /done", got)
	}
}

func TestIsHTMXIsCaseInsensitive(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("HX-Request", " TRUE ")
	if !render.IsHTMX(req) {
		t.Fatal("IsHTMX should accept a case-insensitive true value")
	}
}
