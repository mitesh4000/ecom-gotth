package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mitesh4000/pushti_sangar/server/internal/web"
)

func newWebRouter() http.Handler {
	r := chi.NewRouter()
	web.RegisterWebRoutes(r, web.WebDependencies{})
	return r
}

func get(t *testing.T, router http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func body(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	b, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func TestHomePageRendersFullDocument(t *testing.T) {
	rec := get(t, newWebRouter(), "/", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content-type = %q, want text/html", got)
	}

	html := body(t, rec)
	for _, want := range []string{
		"<!doctype html>",
		"<html lang=\"en\"",
		"<title>Home · Pusti Shangar</title>",
		"/static/css/app.css",
		"/static/js/htmx.min.js",
		"name=\"htmx-config\"",
		"name=\"csrf-token\"",
		"id=\"cart-badge\"",
		"Pusti Shangar",
		"Shop the collection",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("homepage is missing %q", want)
		}
	}
}

func TestServerMessageFragmentForHTMX(t *testing.T) {
	rec := get(t, newWebRouter(), "/fragments/server-message", map[string]string{
		"HX-Request": "true",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("content-type = %q, want text/html", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want no-store", got)
	}

	fragment := body(t, rec)
	if !strings.Contains(fragment, "Templ and HTMX are working") {
		t.Fatalf("fragment does not contain server message: %q", fragment)
	}
	if strings.Contains(fragment, "<!doctype html>") {
		t.Fatal("fragment unexpectedly contains a full document")
	}
}

func TestServerMessageRedirectsNormalBrowser(t *testing.T) {
	rec := get(t, newWebRouter(), "/fragments/server-message", nil)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/#htmx-demo" {
		t.Fatalf("location = %q, want /#htmx-demo", got)
	}
}

func TestStaticAssetsAreServed(t *testing.T) {
	router := newWebRouter()

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{"/static/css/app.css", "text/css", "bg-emerald-600"},
		{"/static/js/htmx.min.js", "javascript", "htmx"},
		{"/static/images/logo.svg", "image/svg+xml", "<svg"},
	}
	for _, tt := range tests {
		rec := get(t, router, tt.path, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d, want %d", tt.path, rec.Code, http.StatusOK)
			continue
		}
		if got := rec.Header().Get("Content-Type"); !strings.Contains(got, tt.contentType) {
			t.Errorf("%s content-type = %q, want %q", tt.path, got, tt.contentType)
		}
		if got := body(t, rec); !strings.Contains(got, tt.contains) {
			t.Errorf("%s does not contain %q", tt.path, tt.contains)
		}
		if got := rec.Header().Get("Cache-Control"); got == "" {
			t.Errorf("%s is missing Cache-Control", tt.path)
		}
	}
}

func TestHealthCheck(t *testing.T) {
	rec := get(t, newWebRouter(), "/health", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	if got := body(t, rec); !strings.Contains(got, `"status":"ok"`) {
		t.Fatalf("health body = %q", got)
	}
}
